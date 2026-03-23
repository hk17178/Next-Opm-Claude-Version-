// Package main 是 svc-incident 事件管理服务的入口包。
// svc-incident 负责事件的全生命周期管理，包括创建、确认、升级、解决，以及时间线追踪和值班调度。
// 启动顺序：日志 → 数据库 → Kafka → 业务层 → 升级监控 → Kafka 消费 → 健康检查 → gRPC/HTTP 服务器。
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
	incidentpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/incident"
	// BUG-005 修复：原始导入路径使用了 monorepo 根模块路径
	// github.com/opsnexus/opsnexus/services/svc-incident/internal/...，
	// 与服务自身 go.mod 声明的模块名 github.com/opsnexus/svc-incident 不匹配，
	// 导致编译时无法解析包。修复：统一使用服务自身模块名作为导入前缀。
	"github.com/opsnexus/svc-incident/internal/biz"
	"github.com/opsnexus/svc-incident/internal/data"
	"github.com/opsnexus/svc-incident/internal/service"
)

// version 定义当前服务版本号，用于健康检查和日志标识。
const version = "0.1.0"

// main 是 svc-incident 服务的启动入口函数。
func main() {
	// 初始化结构化日志记录器
	log := logger.New("svc-incident")
	defer log.Sync()

	// 创建可取消的根上下文，用于协调所有 goroutine 的生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 连接 PostgreSQL 数据库，用于存储事件、时间线和值班排期数据
	dbCfg := data.DBConfig{
		Host:         envOrDefault("OPSNEXUS_INCIDENT_DB_HOST", "localhost"),
		Port:         5432,
		User:         envOrDefault("OPSNEXUS_INCIDENT_DB_USER", "opsnexus"),
		Password:     envOrDefault("OPSNEXUS_INCIDENT_DB_PASSWORD", "opsnexus"),
		DBName:       envOrDefault("OPSNEXUS_INCIDENT_DB_NAME", "incident_db"),
		SSLMode:      "disable",
		MaxOpenConns: 25,
	}
	pool, err := data.NewDB(ctx, dbCfg, log)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	// 初始化数据仓库层：事件仓库、时间线仓库、值班排期仓库
	incidentRepo := data.NewIncidentRepo(pool)
	timelineRepo := data.NewTimelineRepo(pool)
	scheduleRepo := data.NewScheduleRepo(pool)

	// 初始化 Kafka 生产者，用于发布事件状态变更消息到下游
	producer, err := event.NewProducer(event.ProducerConfig{
		Brokers: envOrDefault("OPSNEXUS_KAFKA_BROKERS", "localhost:9092"),
	}, log)
	if err != nil {
		log.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer producer.Close()

	// 初始化事件业务用例，聚合所有仓库和生产者依赖
	incidentUC := biz.NewIncidentUsecase(incidentRepo, timelineRepo, scheduleRepo, producer, log)

	// 启动升级监控器：定期扫描超时未确认/未解决的事件，自动触发升级流程
	watcher := biz.NewEscalationWatcher(incidentRepo, incidentUC, log)
	go watcher.Start(ctx)

	// 启动 Kafka 消费者，订阅告警触发和 AI 分析完成事件
	consumer, err := event.NewConsumer(event.ConsumerConfig{
		Brokers: envOrDefault("OPSNEXUS_KAFKA_BROKERS", "localhost:9092"),
		GroupID: "svc-incident.alert-fired",
	}, log)
	if err != nil {
		log.Fatal("failed to create kafka consumer", zap.Error(err))
	}

	// 注册消费者处理函数：告警触发 → 自动创建事件；AI 分析完成 → 补充事件上下文
	alertConsumer := service.NewAlertConsumer(incidentUC, log)
	consumer.Subscribe(event.TopicAlertFired, alertConsumer.HandleAlertFired)
	consumer.Subscribe(event.TopicAIAnalysisDone, alertConsumer.HandleAIAnalysisDone)

	go func() {
		if err := consumer.Start(ctx); err != nil && err != context.Canceled {
			log.Error("kafka consumer error", zap.Error(err))
		}
	}()

	// 注册健康检查端点，供 Kubernetes 探针使用
	h := health.New("svc-incident", version)
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

	// 启动 gRPC 服务器，供内部微服务（如 svc-analytics）查询事件数据
	grpcAddr := envOrDefault("OPSNEXUS_INCIDENT_GRPC_ADDR", ":9083")
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal("failed to listen on gRPC address", zap.String("addr", grpcAddr), zap.Error(err))
	}
	grpcServer := grpc.NewServer()
	incidentpb.RegisterIncidentServiceServer(grpcServer, service.NewGRPCServer(incidentUC, log))

	go func() {
		log.Info("svc-incident gRPC starting", zap.String("addr", grpcAddr))
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
		handler := service.NewHandler(incidentUC, log)
		handler.RegisterRoutes(r)
	})

	// 启动 HTTP 服务器
	httpAddr := envOrDefault("OPSNEXUS_INCIDENT_HTTP_ADDR", ":8083")
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
		// 取消根上下文，通知所有组件（Kafka 消费者、升级监控器等）停止工作
		cancel()

		// 关闭顺序：gRPC → Kafka 消费者 → HTTP（按依赖逆序）
		grpcServer.GracefulStop()
		consumer.Close()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info("svc-incident HTTP starting", zap.String("addr", httpAddr), zap.String("version", version))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("http server error", zap.Error(err))
	}

	log.Info("svc-incident stopped")
}

// envOrDefault 读取环境变量，若为空则返回默认值。
// 用于支持容器化部署时通过环境变量覆盖默认配置。
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
