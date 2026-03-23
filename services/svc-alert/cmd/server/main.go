// Package main 是 svc-alert 告警服务的入口包。
// svc-alert 负责告警规则管理、告警事件评估与触发，支持基线检测和告警去重。
// 启动顺序：日志 → Postgres → 业务层 → Kafka 消费者 → HTTP/gRPC 服务器 → 优雅关闭。
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	alertpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/alert"
	"github.com/opsnexus/svc-alert/internal/biz"
	"github.com/opsnexus/svc-alert/internal/data"
	"github.com/opsnexus/svc-alert/internal/service"
)

// main 是 svc-alert 服务的启动入口函数。
// 初始化顺序：日志 → 数据库 → 仓库层 → 业务层（基线/去重/引擎）→ HTTP 路由 → Kafka 消费 → HTTP/gRPC 服务器。
func main() {
	// 初始化生产级日志记录器
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	log := logger.Sugar()

	// 连接 PostgreSQL 数据库，用于存储告警规则和告警事件
	pgDSN := envOrDefault("POSTGRES_DSN", "postgres://opm:opm@localhost:5432/opm_alert?sslmode=disable")
	pool, err := data.NewPostgres(context.Background(), pgDSN)
	if err != nil {
		log.Fatalf("failed to connect postgres: %v", err)
	}
	defer pool.Close()

	// 初始化数据仓库层：规则仓库、告警仓库和沉默规则仓库
	ruleRepo := data.NewRuleRepo(pool)
	alertRepo := data.NewAlertRepo(pool)
	// silenceRepo 负责将告警沉默规则持久化到 PostgreSQL silences 表，
	// 避免重启后静默规则丢失（内存方案无法跨重启保留）
	silenceRepo := data.NewSilenceRepo(pool)

	// 初始化业务层组件：
	// - BaselineTracker：维护指标基线，用于动态阈值检测
	// - Deduplicator：5 分钟窗口内相同告警去重，防止告警风暴
	// - AlertEngine：核心告警评估引擎，串联规则匹配、基线检测和去重
	baseline := biz.NewBaselineTracker(100)
	dedup := biz.NewDeduplicator(5 * time.Minute)
	engine := biz.NewAlertEngine(ruleRepo, alertRepo, baseline, dedup, log)
	ruleUseCase := biz.NewRuleUseCase(ruleRepo, log)

	// 构建 HTTP 路由，注册恢复、请求 ID、真实 IP、日志等中间件
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)

	// 初始化沉默规则用例：封装 Create/List/Delete 业务逻辑，读写 PostgreSQL silences 表
	silenceUseCase := biz.NewSilenceUseCase(silenceRepo, log)

	// 注册 HTTP 路由处理器，提供告警规则 CRUD、告警查询和沉默规则管理 API
	// silenceRepo 同时直接传给 handler 供静默查询端点使用，silenceUseCase 用于业务逻辑
	handler := service.NewHandler(ruleUseCase, engine, alertRepo, silenceRepo, silenceUseCase, log)
	handler.RegisterRoutes(r)

	// 启动 Kafka 消费者，订阅日志摄取完成事件，触发告警规则评估
	brokers := []string{envOrDefault("KAFKA_BROKERS", "localhost:9092")}
	consumer := service.NewKafkaConsumer(
		brokers,
		"svc-alert-group",
		envOrDefault("KAFKA_TOPIC_LOG", "opsnexus.log.ingested"),       // 消费：已摄取的日志事件
		envOrDefault("KAFKA_TOPIC_ALERT_FIRED", "opsnexus.alert.fired"), // 生产：已触发的告警事件
		engine,
		log,
	)

	// 创建可取消的上下文，用于协调各组件的生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := consumer.Start(ctx); err != nil {
			log.Errorf("kafka consumer error: %v", err)
		}
	}()

	// 启动 HTTP 服务器，设置读写和空闲超时
	addr := envOrDefault("HTTP_ADDR", ":8084")
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Infof("svc-alert HTTP listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	// 启动 gRPC 服务器，供内部微服务（如 svc-incident）调用告警查询接口
	grpcAddr := envOrDefault("OPSNEXUS_ALERT_GRPC_ADDR", ":9086")
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen on gRPC address %s: %v", grpcAddr, err)
	}
	grpcServer := grpc.NewServer()
	alertpb.RegisterAlertServiceServer(grpcServer, service.NewGRPCServer(ruleUseCase, alertRepo, log))

	go func() {
		log.Infof("svc-alert gRPC starting on %s", grpcAddr)
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Errorf("gRPC server error: %v", err)
		}
	}()

	// 优雅关闭：监听 SIGINT/SIGTERM 信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Infof("received signal %v, shutting down", sig)

	// 关闭顺序：先取消上下文（停止 Kafka 消费）→ 优雅停止 gRPC → 关闭 HTTP 服务器
	cancel()
	grpcServer.GracefulStop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorf("http server shutdown error: %v", err)
	}

	fmt.Println("svc-alert stopped")
}

// envOrDefault 读取环境变量，若为空则返回 fallback 默认值。
// 用于支持容器化部署时通过环境变量覆盖默认配置。
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
