// Package main 是 svc-ai 智能分析服务的入口包。
// svc-ai 负责接收告警/事件数据，调用 LLM 进行根因分析和处置建议生成，支持模型管理、预算控制和熔断保护。
// 启动顺序：日志 → 配置 → Postgres → 业务层（脱敏/模型/熔断）→ Kafka 消费 → gRPC/HTTP 服务器。
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

	"go.uber.org/zap"
	"google.golang.org/grpc"

	aipb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/ai"
	"github.com/opsnexus/svc-ai/internal/biz"
	"github.com/opsnexus/svc-ai/internal/config"
	"github.com/opsnexus/svc-ai/internal/data"
	"github.com/opsnexus/svc-ai/internal/service"
)

// main 是 svc-ai 服务的启动入口函数。
func main() {
	// 初始化生产级日志记录器
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// 加载 YAML 配置文件，包含 AI 模型参数、脱敏规则、熔断策略等
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// 连接 PostgreSQL 数据库，用于存储分析记录、模型配置、提示词模板、调用日志和预算
	db, err := data.NewPostgres(cfg.Database)
	if err != nil {
		logger.Fatal("failed to connect database", zap.Error(err))
	}
	defer db.Close()

	// 初始化数据仓库层
	repo := data.NewAnalysisRepo(db)       // 分析结果仓库
	modelRepo := data.NewModelRepo(db)     // LLM 模型配置仓库
	promptRepo := data.NewPromptRepo(db)   // 提示词模板仓库
	callLogRepo := data.NewCallLogRepo(db) // LLM 调用日志仓库（用于审计和计费）
	budgetRepo := data.NewBudgetRepo(db)   // 调用预算仓库（按租户/团队限制 Token 消耗）

	// 初始化数据脱敏器，在发送给 LLM 前移除敏感信息（密码、密钥等）
	desensitizer, err := biz.NewDesensitizer(cfg.AI.Desensitize)
	if err != nil {
		logger.Fatal("failed to init desensitizer", zap.Error(err))
	}

	// 初始化业务层组件：
	// - ModelManager：管理多 LLM 提供商的选择和预算控制
	// - ContextCollector：收集告警/事件上下文并进行 Token 裁剪和脱敏
	// - CircuitBreaker：LLM 调用熔断器，防止下游 API 故障导致雪崩
	modelManager := biz.NewModelManager(modelRepo, budgetRepo, cfg.AI, logger)
	contextCollector := biz.NewContextCollector(cfg.AI, desensitizer, logger)
	circuitBreaker := biz.NewCircuitBreaker(cfg.AI.CircuitBreaker, logger)

	// 组合分析用例，串联上下文收集 → 脱敏 → LLM 调用 → 结果存储
	analysisUseCase := biz.NewAnalysisUseCase(
		repo, modelManager, contextCollector, circuitBreaker, promptRepo, callLogRepo, logger,
	)

	// 构建 HTTP 路由，提供分析触发、模型管理和提示词模板 API
	router := service.NewRouter(analysisUseCase, modelRepo, promptRepo, logger)

	// 启动 Kafka 消费者，订阅告警触发事件，自动触发 AI 分析流程
	kafkaConsumer := service.NewKafkaConsumer(cfg.Kafka, analysisUseCase, logger)
	go kafkaConsumer.Start(context.Background())

	// 启动 gRPC 服务器，供内部微服务（如 svc-incident）同步调用分析接口
	grpcPort := cfg.Server.GRPCPort
	if grpcPort == 0 {
		grpcPort = 9081 // 默认 gRPC 端口
	}
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		logger.Fatal("failed to listen on gRPC port", zap.Int("port", grpcPort), zap.Error(err))
	}
	grpcServer := grpc.NewServer()
	aiGRPCServer := service.NewAIGRPCServer(analysisUseCase, nil, logger)
	aipb.RegisterAIServiceServer(grpcServer, aiGRPCServer)

	go func() {
		logger.Info("svc-ai gRPC starting", zap.Int("port", grpcPort))
		if err := grpcServer.Serve(grpcLis); err != nil {
			logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	// 启动 HTTP 服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler: router,
	}

	go func() {
		logger.Info("svc-ai HTTP starting", zap.Int("port", cfg.Server.HTTPPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// 优雅关闭：监听 SIGINT/SIGTERM 信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down svc-ai...")

	// 关闭顺序：Kafka 消费者 → gRPC → HTTP（10 秒超时保护）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	kafkaConsumer.Stop()
	grpcServer.GracefulStop()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}
	logger.Info("svc-ai stopped")
}
