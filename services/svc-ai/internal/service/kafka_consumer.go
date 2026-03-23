package service

import (
	"context"
	"encoding/json"

	"github.com/opsnexus/svc-ai/internal/biz"
	"github.com/opsnexus/svc-ai/internal/config"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// KafkaConsumer subscribes to alert.fired and incident.created topics
// to trigger AI analysis automatically.
type KafkaConsumer struct {
	cfg        config.KafkaConfig
	analysisUC *biz.AnalysisUseCase
	logger     *zap.Logger
	readers    []*kafka.Reader
	cancelFn   context.CancelFunc
}

func NewKafkaConsumer(cfg config.KafkaConfig, analysisUC *biz.AnalysisUseCase, logger *zap.Logger) *KafkaConsumer {
	return &KafkaConsumer{
		cfg:        cfg,
		analysisUC: analysisUC,
		logger:     logger,
	}
}

func (kc *KafkaConsumer) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	kc.cancelFn = cancel

	topics := []struct {
		topic   string
		handler func(ctx context.Context, msg kafka.Message) error
	}{
		{kc.cfg.Topics.AlertFired, kc.handleAlertFired},
		{kc.cfg.Topics.IncidentCreated, kc.handleIncidentCreated},
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

		go kc.consumeLoop(ctx, reader, t.topic, t.handler)
	}

	kc.logger.Info("kafka consumers started",
		zap.Strings("topics", []string{kc.cfg.Topics.AlertFired, kc.cfg.Topics.IncidentCreated}),
	)
}

func (kc *KafkaConsumer) consumeLoop(ctx context.Context, reader *kafka.Reader, topic string, handler func(context.Context, kafka.Message) error) {
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

		if err := handler(ctx, msg); err != nil {
			kc.logger.Error("kafka message handling error",
				zap.String("topic", topic),
				zap.Int64("offset", msg.Offset),
				zap.Error(err),
			)
		}
	}
}

func (kc *KafkaConsumer) handleAlertFired(ctx context.Context, msg kafka.Message) error {
	var event biz.AlertFiredEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		kc.logger.Warn("invalid alert.fired event", zap.Error(err))
		return nil // Don't retry malformed messages
	}

	kc.logger.Info("processing alert.fired",
		zap.String("alert_id", event.Data.AlertID),
		zap.String("severity", event.Data.Severity),
	)

	return kc.analysisUC.HandleAlertFired(ctx, event)
}

func (kc *KafkaConsumer) handleIncidentCreated(ctx context.Context, msg kafka.Message) error {
	var event biz.IncidentCreatedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		kc.logger.Warn("invalid incident.created event", zap.Error(err))
		return nil
	}

	kc.logger.Info("processing incident.created",
		zap.String("incident_id", event.Data.IncidentID),
		zap.String("severity", event.Data.Severity),
	)

	return kc.analysisUC.HandleIncidentCreated(ctx, event)
}

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
