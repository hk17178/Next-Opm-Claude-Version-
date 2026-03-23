// Package main 是 svc-cmdb 配置管理数据库服务的入口包。
// svc-cmdb 负责 IT 资产（服务器、容器、网络设备等）的全生命周期管理，包括资产发现、关系拓扑和分组维度。
// 启动顺序：日志 → 数据库 → Kafka → 业务层 → 健康检查 → gRPC/HTTP 服务器。
package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/opsnexus/opsnexus/pkg/auth"
	"github.com/opsnexus/opsnexus/pkg/event"
	"github.com/opsnexus/opsnexus/pkg/health"
	"github.com/opsnexus/opsnexus/pkg/logger"
	"github.com/opsnexus/opsnexus/pkg/middleware"
	cmdbpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/cmdb"
	// BUG-006 修复：原始导入路径使用了 monorepo 根模块路径
	// github.com/opsnexus/opsnexus/services/svc-cmdb/internal/...，
	// 与服务自身 go.mod 声明的模块名 github.com/opsnexus/svc-cmdb 不匹配，
	// 导致编译时无法解析包。修复：统一使用服务自身模块名作为导入前缀。
	"github.com/opsnexus/svc-cmdb/internal/biz"
	"github.com/opsnexus/svc-cmdb/internal/data"
	"github.com/opsnexus/svc-cmdb/internal/service"
)

// version 定义当前服务版本号，用于健康检查和日志标识。
const version = "0.1.0"

// main 是 svc-cmdb 服务的启动入口函数。
func main() {
	// 初始化结构化日志记录器
	log := logger.New("svc-cmdb")
	defer log.Sync()

	// 创建可取消的根上下文，用于协调所有 goroutine 的生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 连接 PostgreSQL 数据库，用于存储资产、关系、分组、维度和发现记录
	dbCfg := data.DBConfig{
		Host:         envOrDefault("OPSNEXUS_CMDB_DB_HOST", "localhost"),
		Port:         5432,
		User:         envOrDefault("OPSNEXUS_CMDB_DB_USER", "opsnexus"),
		Password:     envOrDefault("OPSNEXUS_CMDB_DB_PASSWORD", "opsnexus"),
		DBName:       envOrDefault("OPSNEXUS_CMDB_DB_NAME", "cmdb_db"),
		SSLMode:      "disable",
		MaxOpenConns: 25,
	}
	pool, err := data.NewDB(ctx, dbCfg, log)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	// 初始化数据仓库层：资产、关系、分组、维度、发现记录五个仓库
	assetRepo := data.NewAssetRepo(pool)
	relationRepo := data.NewRelationRepo(pool)
	groupRepo := data.NewGroupRepo(pool)
	dimensionRepo := data.NewDimensionRepo(pool)
	discoveryRepo := data.NewDiscoveryRepo(pool)

	// 初始化 Kafka 生产者，用于发布资产变更事件到下游消费者
	producer, err := event.NewProducer(event.ProducerConfig{
		Brokers: envOrDefault("OPSNEXUS_KAFKA_BROKERS", "localhost:9092"),
	}, log)
	if err != nil {
		log.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer producer.Close()

	// 初始化资产业务用例，聚合所有仓库和 Kafka 生产者依赖
	assetUC := biz.NewAssetUsecase(assetRepo, relationRepo, groupRepo, dimensionRepo, discoveryRepo, producer, log)

	// 注册健康检查端点，供 Kubernetes 探针使用
	h := health.New("svc-cmdb", version)
	h.AddReadinessCheck("postgres", health.DatabaseCheck(pool))
	h.AddReadinessCheck("kafka", health.PingCheck(envOrDefault("OPSNEXUS_KAFKA_BROKERS", "localhost:9092"), 3*time.Second))

	// 初始化 Keycloak 密钥存储，用于 JWT 令牌验证（开发模式下为 nil，跳过签名校验）
	var keyStore *auth.KeyStore
	if keycloakURL := os.Getenv("OPSNEXUS_KEYCLOAK_URL"); keycloakURL != "" {
		keyStore = auth.NewKeyStore(auth.Config{
			KeycloakURL: keycloakURL,
			Realm:       envOrDefault("OPSNEXUS_KEYCLOAK_REALM", "opsnexus"),
		}, log)
	}

	// 启动 gRPC 服务器，供内部微服务（如 svc-incident、svc-analytics）查询资产拓扑
	grpcAddr := envOrDefault("OPSNEXUS_CMDB_GRPC_ADDR", ":9084")
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal("failed to listen on gRPC address", zap.String("addr", grpcAddr), zap.Error(err))
	}
	grpcServer := grpc.NewServer()
	cmdbpb.RegisterCMDBServiceServer(grpcServer, service.NewGRPCServer(assetUC, log))

	go func() {
		log.Info("svc-cmdb gRPC starting", zap.String("addr", grpcAddr))
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
		handler := service.NewHandler(assetUC, log)
		handler.RegisterRoutes(r)
	})

	// 启动 HTTP 服务器
	httpAddr := envOrDefault("OPSNEXUS_CMDB_HTTP_ADDR", ":8084")
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

		// 关闭顺序：先停止 gRPC → 再关闭 HTTP（15 秒超时保护）
		grpcServer.GracefulStop()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info("svc-cmdb HTTP starting", zap.String("addr", httpAddr), zap.String("version", version))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("http server error", zap.Error(err))
	}

	log.Info("svc-cmdb stopped")
}

// envOrDefault 读取环境变量，若为空则返回默认值。
// 用于支持容器化部署时通过环境变量覆盖默认配置。
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
