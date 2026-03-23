// Package config 提供 svc-ai 服务的配置定义和加载功能。
// 所有配置项通过 YAML 文件加载，支持服务器、数据库、Redis、Kafka、AI 模型和日志等模块。
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 定义 svc-ai 服务的完整配置结构。
type Config struct {
	Server   ServerConfig   `yaml:"server"`   // HTTP/gRPC 服务器配置
	Database DatabaseConfig `yaml:"database"` // PostgreSQL 数据库配置
	Redis    RedisConfig    `yaml:"redis"`    // Redis 缓存配置
	Kafka    KafkaConfig    `yaml:"kafka"`    // Kafka 消息队列配置
	AI       AIConfig       `yaml:"ai"`       // AI 模型和分析行为配置
	Logging  LoggingConfig  `yaml:"logging"`  // 日志输出配置
}

// ServerConfig 定义服务监听端口和运行模式。
type ServerConfig struct {
	HTTPPort int    `yaml:"http_port"` // HTTP 监听端口
	GRPCPort int    `yaml:"grpc_port"` // gRPC 监听端口
	Mode     string `yaml:"mode"`      // 运行模式（如 debug、release）
}

// DatabaseConfig 定义 PostgreSQL 连接参数。
type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	DBName       string `yaml:"dbname"`
	SSLMode      string `yaml:"sslmode"`
	MaxOpenConns int    `yaml:"max_open_conns"` // 最大打开连接数
	MaxIdleConns int    `yaml:"max_idle_conns"` // 最大空闲连接数
}

// RedisConfig 定义 Redis 连接参数。
type RedisConfig struct {
	Addr     string `yaml:"addr"`     // Redis 地址（host:port）
	Password string `yaml:"password"` // 认证密码
	DB       int    `yaml:"db"`       // 数据库编号
}

// KafkaConfig 定义 Kafka 消费者和生产者配置。
type KafkaConfig struct {
	Brokers       []string         `yaml:"brokers"`        // Kafka Broker 地址列表
	ConsumerGroup string           `yaml:"consumer_group"` // 消费者组 ID
	Topics        KafkaTopicConfig `yaml:"topics"`         // 订阅主题配置
	ProducerTopic string           `yaml:"producer_topic"` // 分析完成后的发布主题
}

// KafkaTopicConfig 定义 Kafka 消费的事件主题。
type KafkaTopicConfig struct {
	AlertFired      string `yaml:"alert_fired"`       // 告警触发事件主题
	IncidentCreated string `yaml:"incident_created"`  // 事件创建主题
	OperationLogged string `yaml:"operation_logged"`  // 运维操作日志主题
}

// AIConfig 定义 AI 分析相关的核心参数。
type AIConfig struct {
	DefaultTimeoutSeconds int                  `yaml:"default_timeout_seconds"` // LLM 调用默认超时（秒）
	MaxContextTokens      int                  `yaml:"max_context_tokens"`      // 上下文最大 Token 数（超出将裁剪）
	MinContextTokens      int                  `yaml:"min_context_tokens"`      // 上下文最小 Token 数（低于此值跳过分析）
	CircuitBreaker        CircuitBreakerConfig `yaml:"circuit_breaker"`         // LLM 调用熔断器配置
	Desensitize           DesensitizeConfig    `yaml:"desensitize"`             // 数据脱敏配置
}

// CircuitBreakerConfig 定义熔断器参数，防止 LLM API 故障导致雪崩。
type CircuitBreakerConfig struct {
	FailureThreshold int `yaml:"failure_threshold"` // 连续失败次数阈值，达到后熔断
	SuccessThreshold int `yaml:"success_threshold"` // 半开状态下连续成功次数，达到后恢复
	TimeoutSeconds   int `yaml:"timeout_seconds"`   // 熔断持续时间（秒），超时后进入半开状态
}

// DesensitizeConfig 定义发送给 LLM 前的数据脱敏策略。
type DesensitizeConfig struct {
	Enabled      bool   `yaml:"enabled"`       // 是否启用脱敏
	PatternsFile string `yaml:"patterns_file"` // 脱敏正则表达式规则文件路径
}

// LoggingConfig 定义日志输出配置。
type LoggingConfig struct {
	Level  string `yaml:"level"`  // 日志级别（debug、info、warn、error）
	Format string `yaml:"format"` // 日志格式（json、console）
}

// Load 从指定路径读取 YAML 配置文件并解析为 Config 结构体。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
