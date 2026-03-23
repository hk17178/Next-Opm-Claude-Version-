// Package service 提供日志服务的传输层实现，包括 gRPC 服务端、HTTP 路由处理和 Kafka 消费者。
package service

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/IBM/sarama"
	"github.com/opsnexus/svc-log/internal/biz"
	"go.uber.org/zap"
)

// KafkaConsumer 从 Kafka 消费日志消息并送入摄入管道处理。
type KafkaConsumer struct {
	client    sarama.ConsumerGroup
	topic     string
	ingestSvc *biz.IngestService
	logger    *zap.Logger
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// KafkaProducer 实现 biz.EventPublisher 接口，将 CloudEvents 事件发布到 Kafka。
type KafkaProducer struct {
	producer sarama.SyncProducer
	logger   *zap.Logger
}

// NewKafkaProducer 创建一个新的 Kafka 同步生产者，用于事件发布。
func NewKafkaProducer(brokers []string, logger *zap.Logger) (*KafkaProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return &KafkaProducer{
		producer: producer,
		logger:   logger,
	}, nil
}

// Publish 将 CloudEvent 事件发送到指定的 Kafka 主题。
func (p *KafkaProducer) Publish(ctx context.Context, topic string, event biz.CloudEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(event.ID),
		Value: sarama.ByteEncoder(data),
	}

	_, _, err = p.producer.SendMessage(msg)
	if err != nil {
		p.logger.Error("failed to publish event", zap.String("topic", topic), zap.Error(err))
		return err
	}

	return nil
}

// Close 关闭 Kafka 生产者连接。
func (p *KafkaProducer) Close() error {
	return p.producer.Close()
}

// NewKafkaConsumer 创建一个新的 Kafka 消费者组，使用 RoundRobin 再平衡策略。
func NewKafkaConsumer(brokers []string, topic, group string, ingestSvc *biz.IngestService, logger *zap.Logger) (*KafkaConsumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Return.Errors = true

	client, err := sarama.NewConsumerGroup(brokers, group, config)
	if err != nil {
		return nil, err
	}

	return &KafkaConsumer{
		client:    client,
		topic:     topic,
		ingestSvc: ingestSvc,
		logger:    logger,
	}, nil
}

// Start 开始消费 Kafka 消息，在后台 goroutine 中运行。调用 Stop() 可优雅关闭。
func (c *KafkaConsumer) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	c.wg.Add(1)

	handler := &consumerGroupHandler{
		ingestSvc: c.ingestSvc,
		logger:    c.logger,
	}

	go func() {
		defer c.wg.Done()
		for {
			if err := c.client.Consume(ctx, []string{c.topic}, handler); err != nil {
				c.logger.Error("kafka consume error", zap.Error(err))
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	go func() {
		for err := range c.client.Errors() {
			c.logger.Error("kafka consumer error", zap.Error(err))
		}
	}()

	c.logger.Info("kafka consumer started", zap.String("topic", c.topic))
}

// Stop 优雅关闭 Kafka 消费者，等待所有消费协程退出。
func (c *KafkaConsumer) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	c.client.Close()
	c.logger.Info("kafka consumer stopped")
}

// consumerGroupHandler 实现 sarama.ConsumerGroupHandler 接口，将每条 Kafka 消息送入日志摄入管道。
type consumerGroupHandler struct {
	ingestSvc *biz.IngestService
	logger    *zap.Logger
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if err := h.ingestSvc.IngestKafka(session.Context(), msg.Key, msg.Value); err != nil {
			h.logger.Error("kafka ingest failed",
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Error(err),
			)
		}
		session.MarkMessage(msg, "")
	}
	return nil
}
