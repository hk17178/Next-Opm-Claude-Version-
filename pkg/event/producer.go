// producer.go 实现 Kafka 事件生产者，负责将 CloudEvent 发布到指定的 Kafka 主题。

package event

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Producer 负责将 CloudEvent 发布到 Kafka 主题，支持钩子（hook）机制实现链路追踪和指标采集。
type Producer struct {
	brokers      string         // Kafka 集群地址
	logger       *zap.Logger    // 结构化日志实例
	mu           sync.Mutex     // 保护 closed 状态的互斥锁
	closed       bool           // 标记生产者是否已关闭
	flushTimeout time.Duration  // 刷新超时时间
	// beforePublish 是发布前的钩子列表，用于注入链路追踪、指标采集等
	beforePublish []PublishHook
}

// PublishHook 是消息发布前的回调函数，可用于注入链路上下文、记录指标等。
type PublishHook func(ctx context.Context, topic string, event *CloudEvent) error

// ProducerConfig 保存 Kafka 生产者配置。
type ProducerConfig struct {
	Brokers      string        // Kafka 集群地址
	FlushTimeout time.Duration // 刷新超时时间，默认 5 秒
}

// ProducerOption 是生产者可选配置的函数式选项类型。
type ProducerOption func(*Producer)

// WithPublishHook 注册一个在每次发布前执行的钩子函数。
func WithPublishHook(hook PublishHook) ProducerOption {
	return func(p *Producer) {
		p.beforePublish = append(p.beforePublish, hook)
	}
}

// NewProducer 创建一个新的 Kafka 事件生产者。
// 生产环境将包装 confluent-kafka-go；当前实现提供接口和日志，无 CGo 依赖。
func NewProducer(cfg ProducerConfig, logger *zap.Logger, opts ...ProducerOption) (*Producer, error) {
	if cfg.Brokers == "" {
		return nil, fmt.Errorf("kafka brokers must not be empty")
	}
	flushTimeout := cfg.FlushTimeout
	if flushTimeout == 0 {
		flushTimeout = 5 * time.Second
	}
	p := &Producer{
		brokers:      cfg.Brokers,
		logger:       logger,
		flushTimeout: flushTimeout,
	}
	for _, opt := range opts {
		opt(p)
	}
	logger.Info("kafka producer initialized",
		zap.String("brokers", cfg.Brokers),
		zap.Duration("flush_timeout", flushTimeout),
	)
	return p, nil
}

// Publish 将一个 CloudEvent 发送到指定的 Kafka 主题。
// 发布前会依次执行所有已注册的钩子函数。
func (p *Producer) Publish(ctx context.Context, topic string, event *CloudEvent) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return fmt.Errorf("producer is closed")
	}
	p.mu.Unlock()

	// Run pre-publish hooks (tracing, metrics, etc.)
	for _, hook := range p.beforePublish {
		if err := hook(ctx, topic, event); err != nil {
			p.logger.Warn("publish hook error", zap.Error(err))
		}
	}

	data, err := event.Marshal()
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	p.logger.Info("publishing event",
		zap.String("topic", topic),
		zap.String("event_type", event.Type),
		zap.String("event_id", event.ID),
		zap.String("partition_key", event.PartitionKey),
		zap.Int("payload_bytes", len(data)),
	)

	// TODO: Replace with actual confluent-kafka-go producer.Produce() call
	// using event.PartitionKey as the message key for partition ordering.
	// The actual Kafka integration will be wired when the infrastructure is ready.
	_ = data
	return nil
}

// PublishBatch 批量发送多个 CloudEvent 到指定主题。任一事件发布失败则立即返回错误。
func (p *Producer) PublishBatch(ctx context.Context, topic string, events []*CloudEvent) error {
	for _, event := range events {
		if err := p.Publish(ctx, topic, event); err != nil {
			return fmt.Errorf("batch publish failed at event %s: %w", event.ID, err)
		}
	}
	return nil
}

// Close 关闭生产者并刷新待发送的消息。
func (p *Producer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.logger.Info("kafka producer closed")
}
