package service

import (
	"context"
	"encoding/json"

	"github.com/opsnexus/svc-notify/internal/biz"
	"github.com/opsnexus/svc-notify/internal/config"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// KafkaConsumer 订阅事件生命周期相关的 Kafka 主题，将消息路由到 Broadcaster 进行广播。
// 订阅的事件包括：告警触发、事件创建、AI 分析完成、事件解决。
type KafkaConsumer struct {
	cfg         config.KafkaConfig  // Kafka 连接和主题配置
	broadcaster *biz.Broadcaster    // 广播器，处理接收到的生命周期事件
	logger      *zap.Logger         // 日志记录器
	readers     []*kafka.Reader     // 各主题的 Kafka Reader 实例列表
	cancelFn    context.CancelFunc  // 用于停止所有消费协程的取消函数
}

// NewKafkaConsumer 创建 Kafka 消费者实例。
func NewKafkaConsumer(cfg config.KafkaConfig, broadcaster *biz.Broadcaster, logger *zap.Logger) *KafkaConsumer {
	return &KafkaConsumer{
		cfg:         cfg,
		broadcaster: broadcaster,
		logger:      logger,
	}
}

// Start 为每个配置的事件主题创建 Kafka Reader 并启动消费协程。
// 跳过配置中 topic 为空的主题。每个主题使用独立的消费者组（consumer_group.topic）。
func (kc *KafkaConsumer) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	kc.cancelFn = cancel

	topics := []struct {
		topic   string
		handler func(ctx context.Context, data json.RawMessage) error
	}{
		{kc.cfg.Topics.AlertFired, kc.broadcaster.HandleAlertFired},
		{kc.cfg.Topics.IncidentCreated, kc.broadcaster.HandleIncidentCreated},
		{kc.cfg.Topics.AIAnalysisDone, kc.broadcaster.HandleAIAnalysisDone},
		{kc.cfg.Topics.IncidentResolved, kc.broadcaster.HandleIncidentResolved},
	}

	for _, t := range topics {
		if t.topic == "" {
			continue
		}
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:  kc.cfg.Brokers,
			Topic:    t.topic,
			GroupID:  kc.cfg.ConsumerGroup + "." + t.topic,
			MinBytes: 1,
			MaxBytes: 10e6,
		})
		kc.readers = append(kc.readers, reader)

		handler := t.handler
		topic := t.topic
		go kc.consumeLoop(ctx, reader, topic, handler)
	}

	kc.logger.Info("kafka consumers started for svc-notify")
}

// consumeLoop 从指定 Kafka Reader 持续读取消息并交给 handler 处理。
// 上下文取消时退出循环，读取错误会记录日志并继续消费。
func (kc *KafkaConsumer) consumeLoop(ctx context.Context, reader *kafka.Reader, topic string, handler func(context.Context, json.RawMessage) error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			kc.logger.Error("kafka read error", zap.String("topic", topic), zap.Error(err))
			continue
		}

		if err := handler(ctx, msg.Value); err != nil {
			kc.logger.Error("event handling error",
				zap.String("topic", topic),
				zap.Int64("offset", msg.Offset),
				zap.Error(err),
			)
		}
	}
}

// Stop 停止所有 Kafka 消费协程并关闭所有 Reader 连接。
func (kc *KafkaConsumer) Stop() {
	if kc.cancelFn != nil {
		kc.cancelFn()
	}
	for _, r := range kc.readers {
		if err := r.Close(); err != nil {
			kc.logger.Warn("error closing kafka reader", zap.Error(err))
		}
	}
	kc.logger.Info("kafka consumers stopped")
}
