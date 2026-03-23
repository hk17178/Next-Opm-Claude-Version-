// Package config 提供 svc-notify 服务的配置定义和加载功能。
// 所有配置项通过 YAML 文件加载，支持服务器、数据库、Redis、Kafka、广播规则、去重和渠道健康等模块。
package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 定义 svc-notify 服务的完整配置结构。
type Config struct {
	Server        ServerConfig        `yaml:"server"`         // HTTP/gRPC 服务器配置
	Database      DatabaseConfig      `yaml:"database"`       // PostgreSQL 数据库配置
	Redis         RedisConfig         `yaml:"redis"`          // Redis 缓存配置（用于去重引擎）
	Kafka         KafkaConfig         `yaml:"kafka"`          // Kafka 消息队列配置
	Broadcast     BroadcastConfig     `yaml:"broadcast"`      // 广播规则配置
	Dedup         DedupConfig         `yaml:"dedup"`          // 通知去重配置
	ChannelHealth ChannelHealthConfig `yaml:"channel_health"` // 渠道健康探测配置
	Logging       LoggingConfig       `yaml:"logging"`        // 日志输出配置
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
	ProducerTopic string           `yaml:"producer_topic"` // 通知发送后的发布主题
}

// KafkaTopicConfig 定义 Kafka 消费的事件主题，涵盖告警和事件生命周期的关键节点。
type KafkaTopicConfig struct {
	AlertFired       string `yaml:"alert_fired"`        // 告警触发事件
	AlertResolved    string `yaml:"alert_resolved"`     // 告警恢复事件
	IncidentCreated  string `yaml:"incident_created"`   // 事件创建
	IncidentUpdated  string `yaml:"incident_updated"`   // 事件更新（升级/降级/分配）
	IncidentResolved string `yaml:"incident_resolved"`  // 事件解决
	AIAnalysisDone   string `yaml:"ai_analysis_done"`   // AI 分析完成事件
}

// BroadcastConfig 定义广播规则参数。
type BroadcastConfig struct {
	Intervals      map[string]time.Duration `yaml:"intervals"`       // 各事件类型的广播间隔
	LifecycleNodes []string                 `yaml:"lifecycle_nodes"` // 需要广播的生命周期节点列表
}

// DedupConfig 定义通知去重参数，防止短时间内重复发送相同通知。
type DedupConfig struct {
	Window     time.Duration `yaml:"window"`      // 去重时间窗口
	MergeLimit int           `yaml:"merge_limit"` // 窗口内最大合并通知数
}

// ChannelHealthConfig 定义渠道健康探测参数。
type ChannelHealthConfig struct {
	ProbeInterval    time.Duration `yaml:"probe_interval"`    // 探测间隔
	FailureThreshold int           `yaml:"failure_threshold"` // 连续失败次数阈值，达到后标记渠道不健康
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
