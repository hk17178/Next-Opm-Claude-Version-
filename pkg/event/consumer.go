// consumer.go 实现 Kafka 事件消费者，支持主题订阅、消息分发、失败重试和死信处理。

package event

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Handler 处理单个 CloudEvent 的回调函数。
type Handler func(ctx context.Context, event *CloudEvent) error

// ErrorHandler 是消息处理失败后的错误回调函数，用于死信队列等场景。
type ErrorHandler func(ctx context.Context, topic string, event *CloudEvent, err error)

// Consumer 订阅 Kafka 主题并将事件分发给对应的处理函数，支持重试和错误处理。
type Consumer struct {
	brokers      string              // Kafka 集群地址
	groupID      string              // 消费者组 ID
	logger       *zap.Logger         // 结构化日志实例
	handlers     map[string]Handler  // 主题到处理函数的映射
	closed       chan struct{}        // 关闭信号通道
	errorHandler ErrorHandler        // 重试耗尽后的错误处理函数
	maxRetries   int                 // 最大重试次数
	retryBackoff time.Duration       // 重试退避基础间隔
}

// ConsumerConfig 保存 Kafka 消费者配置。
type ConsumerConfig struct {
	Brokers      string        // Kafka 集群地址
	GroupID      string        // 消费者组 ID
	MaxRetries   int           // 最大重试次数，默认 3
	RetryBackoff time.Duration // 重试退避基础间隔，默认 1 秒
}

// ConsumerOption 是消费者可选配置的函数式选项类型。
type ConsumerOption func(*Consumer)

// WithErrorHandler 设置自定义的错误处理函数，在消息处理重试耗尽后调用。
func WithErrorHandler(h ErrorHandler) ConsumerOption {
	return func(c *Consumer) {
		c.errorHandler = h
	}
}

// NewConsumer 创建一个新的 Kafka 事件消费者。
func NewConsumer(cfg ConsumerConfig, logger *zap.Logger, opts ...ConsumerOption) (*Consumer, error) {
	if cfg.Brokers == "" {
		return nil, fmt.Errorf("kafka brokers must not be empty")
	}
	if cfg.GroupID == "" {
		return nil, fmt.Errorf("consumer group ID must not be empty")
	}
	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}
	retryBackoff := cfg.RetryBackoff
	if retryBackoff == 0 {
		retryBackoff = 1 * time.Second
	}
	c := &Consumer{
		brokers:      cfg.Brokers,
		groupID:      cfg.GroupID,
		logger:       logger,
		handlers:     make(map[string]Handler),
		closed:       make(chan struct{}),
		maxRetries:   maxRetries,
		retryBackoff: retryBackoff,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Subscribe 为指定主题注册事件处理函数。
func (c *Consumer) Subscribe(topic string, handler Handler) {
	c.handlers[topic] = handler
	c.logger.Info("subscribed to topic",
		zap.String("topic", topic),
		zap.String("group_id", c.groupID),
	)
}

// Topics 返回已订阅的主题列表。
func (c *Consumer) Topics() []string {
	topics := make([]string, 0, len(c.handlers))
	for t := range c.handlers {
		topics = append(topics, t)
	}
	return topics
}

// Start 开始消费消息。阻塞直到上下文被取消或调用 Close。
func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("kafka consumer starting",
		zap.String("brokers", c.brokers),
		zap.String("group_id", c.groupID),
		zap.Int("topic_count", len(c.handlers)),
	)

	// TODO: Replace with actual confluent-kafka-go consumer loop.
	// The loop should:
	// 1. Poll for messages
	// 2. Unmarshal CloudEvent
	// 3. Extract trace context from event.TraceParent
	// 4. Call dispatch() with trace-enriched context
	// 5. Commit offset on success
	// 6. Retry with backoff on failure (up to maxRetries)
	// 7. Send to dead-letter topic on exhausted retries
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closed:
		return nil
	}
}

// Close 优雅地停止消费者。
func (c *Consumer) Close() error {
	close(c.closed)
	c.logger.Info("kafka consumer closed")
	return nil
}

// dispatch 将原始消息路由到对应的处理函数，包含指数退避重试逻辑。
// 重试耗尽后调用 errorHandler（如已配置），可用于发送到死信队列。
func (c *Consumer) dispatch(ctx context.Context, topic string, data []byte) error {
	handler, ok := c.handlers[topic]
	if !ok {
		return fmt.Errorf("no handler for topic %s", topic)
	}

	evt, err := Unmarshal(data)
	if err != nil {
		return fmt.Errorf("unmarshal event from topic %s: %w", topic, err)
	}

	// Retry loop with exponential backoff
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.retryBackoff * time.Duration(1<<(attempt-1))
			c.logger.Warn("retrying event handler",
				zap.String("topic", topic),
				zap.String("event_id", evt.ID),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
			)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		lastErr = handler(ctx, evt)
		if lastErr == nil {
			return nil
		}
	}

	// Exhausted retries — invoke error handler if configured
	c.logger.Error("event handler failed after retries",
		zap.String("topic", topic),
		zap.String("event_id", evt.ID),
		zap.Int("max_retries", c.maxRetries),
		zap.Error(lastErr),
	)
	if c.errorHandler != nil {
		c.errorHandler(ctx, topic, evt, lastErr)
	}
	return lastErr
}
