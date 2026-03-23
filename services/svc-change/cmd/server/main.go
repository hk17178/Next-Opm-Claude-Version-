// Package main 是 svc-change 变更管理服务的入口包。
// svc-change 负责变更单的全生命周期管理，包括创建、审批、执行、完成，以及冲突检测和变更日历。
// 启动顺序：日志 → 数据库 → 业务层 → 健康检查 → HTTP 服务器。
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/opsnexus/opsnexus/pkg/auth"
	"github.com/opsnexus/opsnexus/pkg/health"
	"github.com/opsnexus/opsnexus/pkg/logger"
	"github.com/opsnexus/opsnexus/pkg/middleware"
	"github.com/opsnexus/svc-change/internal/biz"
	"github.com/opsnexus/svc-change/internal/data"
	service "github.com/opsnexus/svc-change/internal/service"
)

// version 定义当前服务版本号，用于健康检查和日志标识。
const version = "0.1.0"

// main 是 svc-change 服务的启动入口函数。
func main() {
	// 初始化结构化日志记录器
	log := logger.New("svc-change")
	defer log.Sync()

	// 创建可取消的根上下文，用于协调所有 goroutine 的生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 连接 PostgreSQL 数据库，用于存储变更单和审批记录数据
	dbCfg := data.DBConfig{
		Host:         envOrDefault("OPSNEXUS_CHANGE_DB_HOST", "localhost"),
		Port:         5432,
		User:         envOrDefault("OPSNEXUS_CHANGE_DB_USER", "opsnexus"),
		Password:     envOrDefault("OPSNEXUS_CHANGE_DB_PASSWORD", "opsnexus"),
		DBName:       envOrDefault("OPSNEXUS_CHANGE_DB_NAME", "change_db"),
		SSLMode:      "disable",
		MaxOpenConns: 25,
	}
	pool, err := data.NewDB(ctx, dbCfg, log)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	// 初始化数据仓库层
	changeRepo := data.NewChangeRepo(pool)
	approvalRepo := data.NewApprovalRepo(pool)

	// 初始化业务用例层
	changeUC := biz.NewChangeUsecase(changeRepo, approvalRepo, log)
	approvalUC := biz.NewApprovalUsecase(changeRepo, approvalRepo, log)

	// 注册健康检查端点，供 Kubernetes 探针使用
	h := health.New("svc-change", version)
	h.AddReadinessCheck("postgres", health.DatabaseCheck(pool))

	// 初始化 Keycloak 密钥存储，用于 JWT 令牌验证（开发模式下为 nil，跳过签名校验）
	var keyStore *auth.KeyStore
	if keycloakURL := os.Getenv("OPSNEXUS_KEYCLOAK_URL"); keycloakURL != "" {
		keyStore = auth.NewKeyStore(auth.Config{
			KeycloakURL: keycloakURL,
			Realm:       envOrDefault("OPSNEXUS_KEYCLOAK_REALM", "opsnexus"),
		}, log)
	}

	// 构建 HTTP 路由，注册通用中间件
	r := chi.NewRouter()
	r.Use(middleware.Recovery(log))
	r.Use(middleware.RequestID)
	r.Use(middleware.Tracing)
	r.Use(middleware.Logging(log))
	r.Use(middleware.CORS(middleware.DefaultCORSConfig()))

	// 健康检查端点无需认证
	r.Get("/healthz", h.LivenessHandler())
	r.Get("/readyz", h.ReadinessHandler())

	// 业务路由需要 JWT 认证中间件保护
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(keyStore, log))
		handler := service.NewHandler(changeUC, approvalUC, log)
		handler.RegisterRoutes(r)
	})

	// 启动 HTTP 服务器
	httpAddr := envOrDefault("OPSNEXUS_CHANGE_HTTP_ADDR", ":8086")
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
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info("svc-change HTTP starting", zap.String("addr", httpAddr), zap.String("version", version))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("http server error", zap.Error(err))
	}

	log.Info("svc-change stopped")
}

// envOrDefault 读取环境变量，若为空则返回默认值。
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
