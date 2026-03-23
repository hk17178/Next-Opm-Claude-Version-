// Package main 是 svc-log 日志服务的入口包。
// svc-log 负责日志的采集、存储、检索，支持 HTTP 和 gRPC 双协议接入。
// 启动顺序：加载配置 → 连接 Postgres → 连接 Elasticsearch → 初始化 Kafka → 启动 HTTP/gRPC 服务器。
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

	"google.golang.org/grpc"

	"github.com/opsnexus/svc-log/internal/biz"
	"github.com/opsnexus/svc-log/internal/data"
	"github.com/opsnexus/svc-log/internal/service"
	"go.uber.org/zap"
)

// main 是 svc-log 服务的启动入口函数。
// 初始化顺序：日志 → 配置 → 数据库 → ES → Kafka → 业务层 → HTTP/gRPC 服务器 → 等待信号优雅关闭。
func main() {
	// 初始化生产级日志记录器，defer Sync 确保缓冲区刷盘
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// 加载 YAML 配置文件，包含数据库、ES、Kafka 等连接参数
	cfg, err := loadConfig("configs/config.yaml")
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// 初始化 Postgres 连接池，用于日志元数据的持久化存储
	pgPool, err := data.NewPostgresPool(context.Background(), cfg.Postgres.DSN())
	if err != nil {
		logger.Fatal("failed to connect postgres", zap.Error(err))
	}
	defer pgPool.Close()

	// 初始化 Elasticsearch 客户端，用于日志全文检索和聚合分析
	esClient, err := data.NewESClient(cfg.Elasticsearch.Addresses)
	if err != nil {
		logger.Fatal("failed to connect elasticsearch", zap.Error(err))
	}

	// 初始化 Kafka 生产者，用于将已处理的日志事件发布到下游消费者
	kafkaProducer, err := service.NewKafkaProducer(cfg.Kafka.Brokers, logger)
	if err != nil {
		logger.Fatal("failed to create kafka producer", zap.Error(err))
	}
	defer kafkaProducer.Close()

	// 初始化数据仓库层：Postgres 仓库负责元数据，ES 仓库负责索引与检索
	logRepo := data.NewLogRepo(pgPool)
	esRepo := data.NewESRepo(esClient, cfg.Elasticsearch.IndexPrefix)

	// 初始化业务逻辑层：日志摄取服务（批量写入、脱敏）和搜索服务
	ingestSvc := biz.NewIngestService(logRepo, esRepo, kafkaProducer, logger, biz.IngestConfig{
		MaxBatchSize:    cfg.Ingest.MaxBatchSize,
		FlushInterval:   cfg.Ingest.FlushInterval,
		SensitiveFields: cfg.Ingest.SensitiveFields,
	})
	searchSvc := biz.NewSearchService(esRepo, logRepo, logger)

	// 启动 Kafka 消费者，订阅外部日志推送主题，异步消费日志数据
	kafkaConsumer, err := service.NewKafkaConsumer(
		cfg.Kafka.Brokers,
		cfg.Kafka.ConsumerTopic,
		cfg.Kafka.ConsumerGroup,
		ingestSvc,
		logger,
	)
	if err != nil {
		logger.Fatal("failed to create kafka consumer", zap.Error(err))
	}
	go kafkaConsumer.Start(context.Background())

	// 构建 HTTP 路由与处理器，提供 RESTful API 接入能力
	handler := service.NewHandler(ingestSvc, searchSvc, logger)
	router := service.NewRouter(handler)

	// 启动 HTTP 服务器，设置读写超时防止慢客户端占用连接
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("svc-log HTTP starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// 启动 gRPC 服务器，供内部微服务间高效通信
	grpcAddr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Fatal("failed to listen gRPC", zap.String("addr", grpcAddr), zap.Error(err))
	}
	grpcServer := grpc.NewServer()
	logGRPCSrv := service.NewLogGRPCServer(ingestSvc, searchSvc, logger)
	service.RegisterLogService(grpcServer, logGRPCSrv)

	go func() {
		logger.Info("svc-log gRPC starting", zap.String("addr", grpcAddr))
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatal("gRPC server error", zap.Error(err))
		}
	}()

	// 优雅关闭：监听 SIGINT/SIGTERM 信号，收到后按依赖逆序关闭各组件
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down svc-log")
	// 创建 15 秒超时上下文，防止关闭过程无限阻塞
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 关闭顺序：先停止消费者（不再接收新数据）→ 停止 gRPC → 刷盘缓冲区 → 停止 HTTP
	kafkaConsumer.Stop()
	grpcServer.GracefulStop()
	ingestSvc.Flush(ctx) // 刷写内存中未持久化的批量日志
	srv.Shutdown(ctx)
	logger.Info("svc-log stopped")
}

// Config 定义 svc-log 服务的完整配置结构。
type Config struct {
	Server        ServerConfig        `yaml:"server"`        // HTTP/gRPC 服务器配置
	Postgres      PostgresConfig      `yaml:"postgres"`      // PostgreSQL 数据库配置
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"` // Elasticsearch 配置
	Kafka         KafkaConfig         `yaml:"kafka"`         // Kafka 消息队列配置
	Ingest        IngestConfig        `yaml:"ingest"`        // 日志摄取行为配置
}

// ServerConfig 定义服务监听端口和运行模式。
type ServerConfig struct {
	Port     int    `yaml:"port"`      // HTTP 监听端口
	GRPCPort int    `yaml:"grpc_port"` // gRPC 监听端口
	Mode     string `yaml:"mode"`      // 运行模式（如 debug、release）
}

// PostgresConfig 定义 PostgreSQL 连接参数。
type PostgresConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	MaxOpenConns int    `yaml:"max_open_conns"` // 最大打开连接数
	MaxIdleConns int    `yaml:"max_idle_conns"` // 最大空闲连接数
}

// DSN 根据配置字段拼接 PostgreSQL 连接字符串。
func (c PostgresConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

// ElasticsearchConfig 定义 Elasticsearch 连接和批量写入参数。
type ElasticsearchConfig struct {
	Addresses     []string      `yaml:"addresses"`      // ES 节点地址列表
	IndexPrefix   string        `yaml:"index_prefix"`   // 索引名前缀
	BulkSize      int           `yaml:"bulk_size"`      // 批量写入条数阈值
	FlushInterval time.Duration `yaml:"flush_interval"` // 批量写入时间间隔
}

// KafkaConfig 定义 Kafka 生产者和消费者的连接参数。
type KafkaConfig struct {
	Brokers       []string `yaml:"brokers"`        // Kafka Broker 地址列表
	ConsumerTopic string   `yaml:"consumer_topic"` // 消费主题（外部日志推送）
	ProduceTopic  string   `yaml:"produce_topic"`  // 生产主题（已处理日志事件）
	ConsumerGroup string   `yaml:"consumer_group"` // 消费者组 ID
}

// IngestConfig 定义日志摄取行为参数，包括批量大小、刷盘间隔和敏感字段脱敏列表。
type IngestConfig struct {
	MaxBatchSize    int           `yaml:"max_batch_size"`    // 单批次最大日志条数
	FlushInterval   time.Duration `yaml:"flush_interval"`    // 自动刷盘时间间隔
	SensitiveFields []string      `yaml:"sensitive_fields"`  // 需要脱敏的字段名列表
}

// loadConfig 从指定路径读取 YAML 配置文件并解析为 Config 结构体。
// 当前为开发阶段占位实现，使用硬编码默认值；生产环境应使用 gopkg.in/yaml.v3 解析。
func loadConfig(path string) (*Config, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	// Simple YAML parsing placeholder — in production, use gopkg.in/yaml.v3
	_ = f
	// 开发环境默认值
	cfg.Server.Port = 8084
	cfg.Server.GRPCPort = 9085
	cfg.Postgres.Host = "localhost"
	cfg.Postgres.Port = 5432
	cfg.Postgres.User = "opm"
	cfg.Postgres.Password = "opm_secret"
	cfg.Postgres.Database = "opm_log"
	cfg.Postgres.MaxOpenConns = 20
	cfg.Postgres.MaxIdleConns = 5
	cfg.Elasticsearch.Addresses = []string{"http://localhost:9200"}
	cfg.Elasticsearch.IndexPrefix = "opsnexus-log"
	cfg.Elasticsearch.BulkSize = 500
	cfg.Elasticsearch.FlushInterval = 5 * time.Second
	cfg.Kafka.Brokers = []string{"localhost:9092"}
	cfg.Kafka.ConsumerTopic = "opm.log.ingest"
	cfg.Kafka.ProduceTopic = "opsnexus.log.ingested"
	cfg.Kafka.ConsumerGroup = "svc-log-consumer"
	cfg.Ingest.MaxBatchSize = 1000
	cfg.Ingest.FlushInterval = 3 * time.Second
	cfg.Ingest.SensitiveFields = []string{"password", "token", "secret", "authorization"}

	return cfg, nil
}
