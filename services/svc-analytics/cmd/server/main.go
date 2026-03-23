// Package main 是 svc-analytics 数据分析服务的入口包。
// svc-analytics 负责 SLA 管理、指标查询、报表生成、仪表盘和知识库检索，使用 PostgreSQL + ClickHouse 双存储引擎。
// 启动顺序：日志 → PostgreSQL → ClickHouse → 业务层 → 健康检查 → gRPC/HTTP 服务器。
package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/opsnexus/opsnexus/pkg/auth"
	"github.com/opsnexus/opsnexus/pkg/health"
	"github.com/opsnexus/opsnexus/pkg/logger"
	"github.com/opsnexus/opsnexus/pkg/middleware"
	analyticspb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/analytics"
	"github.com/opsnexus/svc-analytics/internal/biz"
	"github.com/opsnexus/svc-analytics/internal/data"
	"github.com/opsnexus/svc-analytics/internal/service"
)

// version 定义当前服务版本号，用于健康检查和日志标识。
const version = "0.1.0"

// main 是 svc-analytics 服务的启动入口函数。
func main() {
	// 初始化结构化日志记录器
	log := logger.New("svc-analytics")
	defer log.Sync()

	// 创建可取消的根上下文，用于协调所有 goroutine 的生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 连接 PostgreSQL 数据库，用于存储 SLA 定义、报表模板、仪表盘配置等元数据
	pgCfg := data.PGConfig{
		Host:         envOrDefault("OPSNEXUS_ANALYTICS_PG_HOST", "localhost"),
		Port:         5432,
		User:         envOrDefault("OPSNEXUS_ANALYTICS_PG_USER", "opsnexus"),
		Password:     envOrDefault("OPSNEXUS_ANALYTICS_PG_PASSWORD", "opsnexus"),
		DBName:       envOrDefault("OPSNEXUS_ANALYTICS_PG_NAME", "analytics_db"),
		SSLMode:      "disable",
		MaxOpenConns: 25,
	}
	pgPool, err := data.NewPG(ctx, pgCfg, log)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer pgPool.Close()

	// 连接 ClickHouse 列式存储，用于高性能指标查询和 SLA 事件聚合分析
	chAddrs := strings.Split(envOrDefault("OPSNEXUS_CLICKHOUSE_ADDRS", "localhost:9000"), ",")
	chCfg := data.ClickHouseConfig{
		Addrs:    chAddrs,
		Database: envOrDefault("OPSNEXUS_CLICKHOUSE_DB", "opsnexus_metrics"),
		Username: envOrDefault("OPSNEXUS_CLICKHOUSE_USER", "default"),
		Password: envOrDefault("OPSNEXUS_CLICKHOUSE_PASSWORD", ""),
	}
	chConn, err := data.NewClickHouse(ctx, chCfg, log)
	if err != nil {
		log.Fatal("failed to connect to clickhouse", zap.Error(err))
	}
	defer chConn.Close()

	// 初始化数据仓库层：SLA 定义（PG）、SLA 事件记录（ClickHouse）、指标查询（ClickHouse）
	slaRepo := data.NewSLARepo(pgPool)
	slaIncidentRepo := data.NewSLAIncidentRepo(chConn)
	metricsRepo := data.NewMetricsRepo(chConn)

	// 初始化业务层组件：
	// - SLAUsecase：SLA 定义管理和达标率计算
	// - MetricsUsecase：指标查询和聚合
	// - ReportUsecase：定期报表生成
	// - DashboardUsecase：自定义仪表盘管理
	// - KnowledgeUsecase：知识库语义检索（向量相似度匹配）
	slaUC := biz.NewSLAUsecase(slaRepo, slaIncidentRepo, log)
	metricsUC := biz.NewMetricsUsecase(metricsRepo, log)
	reportUC := biz.NewReportUsecase(nil, metricsRepo, log)      // 报表 PG 仓库待接入
	dashboardUC := biz.NewDashboardUsecase(nil, log)              // 仪表盘 PG 仓库待接入
	knowledgeUC := biz.NewKnowledgeUsecase(nil, nil, 0.75, log)  // 知识库仓库和嵌入器待接入
	tokenUC := biz.NewAPITokenUsecase(nil)                       // API Token PG 仓库待接入

	// 注册健康检查端点，供 Kubernetes 探针使用
	h := health.New("svc-analytics", version)
	h.AddReadinessCheck("postgres", health.DatabaseCheck(pgPool))

	// 初始化 Keycloak 密钥存储，用于 JWT 令牌验证（开发模式下为 nil，跳过签名校验）
	var keyStore *auth.KeyStore
	if keycloakURL := os.Getenv("OPSNEXUS_KEYCLOAK_URL"); keycloakURL != "" {
		keyStore = auth.NewKeyStore(auth.Config{
			KeycloakURL: keycloakURL,
			Realm:       envOrDefault("OPSNEXUS_KEYCLOAK_REALM", "opsnexus"),
		}, log)
	}

	// 启动 gRPC 服务器，供内部微服务查询指标、SLA 和知识库数据
	grpcAddr := envOrDefault("OPSNEXUS_ANALYTICS_GRPC_ADDR", ":9087")
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal("failed to listen on gRPC address", zap.String("addr", grpcAddr), zap.Error(err))
	}
	grpcServer := grpc.NewServer()
	analyticspb.RegisterAnalyticsServiceServer(grpcServer, service.NewGRPCServer(metricsUC, slaUC, knowledgeUC, log))

	go func() {
		log.Info("svc-analytics gRPC starting", zap.String("addr", grpcAddr))
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Error("gRPC server error", zap.Error(err))
		}
	}()

	// 构建 HTTP 路由，注册通用中间件（恢复、请求 ID、链路追踪、日志、CORS）
	r := chi.NewRouter()
	r.Use(middleware.Recovery(log))
	r.Use(middleware.RequestID)
	r.Use(middleware.Tracing)
	r.Use(middleware.Logging(log))
	r.Use(middleware.CORS(middleware.DefaultCORSConfig()))

	// 健康检查端点无需认证，直接暴露给 K8s 探针
	r.Get("/healthz", h.LivenessHandler())
	r.Get("/readyz", h.ReadinessHandler())

	// 业务路由需要 JWT 认证中间件保护
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(keyStore, log))
		handler := service.NewHandler(slaUC, metricsUC, reportUC, dashboardUC, knowledgeUC, tokenUC, nil, nil, nil, nil, nil, log)
		handler.RegisterRoutes(r)
	})

	// 启动 HTTP 服务器
	httpAddr := envOrDefault("OPSNEXUS_ANALYTICS_HTTP_ADDR", ":8087")
	srv := &http.Server{
		Addr:         httpAddr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 优雅关闭：在独立 goroutine 中监听系统信号
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info("shutdown signal received")
		// 取消根上下文，通知所有关联组件停止工作
		cancel()

		// 关闭顺序：gRPC → HTTP（15 秒超时保护）
		grpcServer.GracefulStop()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info("svc-analytics starting", zap.String("addr", httpAddr), zap.String("version", version))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("http server error", zap.Error(err))
	}

	log.Info("svc-analytics stopped")
}

// envOrDefault 读取环境变量，若为空则返回默认值。
// 用于支持容器化部署时通过环境变量覆盖默认配置。
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
