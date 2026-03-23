// Package main 是 svc-notify 通知服务的入口包。
// svc-notify 负责多渠道消息通知（邮件、企业微信、钉钉、Webhook 等），支持广播规则、去重和渠道健康探测。
// 启动顺序：日志 → 配置 → Postgres → Redis → 业务层 → Kafka 消费 → 健康探测 → gRPC/HTTP 服务器。
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

	notifypb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/notify"
	"github.com/opsnexus/svc-notify/internal/biz"
	"github.com/opsnexus/svc-notify/internal/config"
	"github.com/opsnexus/svc-notify/internal/data"
	"github.com/opsnexus/svc-notify/internal/service"
)

// main 是 svc-notify 服务的启动入口函数。
func main() {
	// 初始化生产级日志记录器
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// 加载 YAML 配置文件
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// 连接 PostgreSQL 数据库，用于存储机器人配置、通知日志和广播规则
	db, err := data.NewPostgres(cfg.Database)
	if err != nil {
		logger.Fatal("failed to connect database", zap.Error(err))
	}
	defer db.Close()

	// 连接 Redis，用于通知去重引擎的滑动窗口计数
	rdb := data.NewRedis(cfg.Redis)
	defer rdb.Close()

	// 初始化数据仓库层
	botRepo := data.NewBotRepo(db)             // 机器人/渠道配置仓库
	logRepo := data.NewNotificationLogRepo(db) // 通知发送日志仓库
	ruleRepo := data.NewBroadcastRuleRepo(db)  // 广播规则仓库

	// 初始化业务层组件：
	// - ChannelManager：管理各通知渠道的发送适配器
	// - DedupEngine：基于 Redis 的通知去重引擎，防止重复通知
	// - Broadcaster：根据广播规则将事件分发到对应渠道
	// - ChannelHealthProbe：定期探测渠道可用性，标记不健康渠道
	channelManager := biz.NewChannelManager(botRepo, logger)
	dedupEngine := biz.NewDedupEngine(rdb, cfg.Dedup, logger)
	broadcaster := biz.NewBroadcaster(botRepo, ruleRepo, channelManager, dedupEngine, logRepo, cfg.Broadcast, logger)
	healthProbe := biz.NewChannelHealthProbe(botRepo, channelManager, cfg.ChannelHealth, logger)

	// 组合通知用例，供 HTTP/gRPC 层调用
	notifyUseCase := biz.NewNotifyUseCase(channelManager, dedupEngine, logRepo, botRepo, logger)

	// 构建 HTTP 路由（传入 channelManager 以支持渠道连通性测试端点）
	router := service.NewRouter(notifyUseCase, botRepo, ruleRepo, channelManager, logger)

	// 启动后台 goroutine：Kafka 消费者、渠道健康探测、定期广播
	kafkaConsumer := service.NewKafkaConsumer(cfg.Kafka, broadcaster, logger)
	go kafkaConsumer.Start(context.Background())
	go healthProbe.Start(context.Background())
	go broadcaster.StartPeriodicBroadcast(context.Background())

	// 启动 gRPC 服务器，供内部微服务（如 svc-incident）调用通知发送接口
	grpcPort := cfg.Server.GRPCPort
	if grpcPort == 0 {
		grpcPort = 9082 // 默认 gRPC 端口
	}
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		logger.Fatal("failed to listen on gRPC port", zap.Int("port", grpcPort), zap.Error(err))
	}
	grpcServer := grpc.NewServer()
	notifyGRPCServer := service.NewNotifyGRPCServer(notifyUseCase, logger)
	notifypb.RegisterNotifyServiceServer(grpcServer, notifyGRPCServer)

	go func() {
		logger.Info("svc-notify gRPC starting", zap.Int("port", grpcPort))
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
		logger.Info("svc-notify HTTP starting", zap.Int("port", cfg.Server.HTTPPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// 优雅关闭：监听 SIGINT/SIGTERM 信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down svc-notify...")

	// 关闭顺序：Kafka 消费者 → 健康探测 → gRPC → HTTP（10 秒超时保护）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	kafkaConsumer.Stop()
	healthProbe.Stop()
	grpcServer.GracefulStop()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}
	logger.Info("svc-notify stopped")
}
