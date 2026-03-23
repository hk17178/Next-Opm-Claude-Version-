package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/opsnexus/svc-alert/internal/biz"
)

// AlertFiredEvent 是 alert.fired 事件的 CloudEvents 信封结构，
// 符合 opsnexus.alert.fired.schema.json 规范。
type AlertFiredEvent struct {
	SpecVersion     string                 `json:"specversion"`
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`
	Source          string                 `json:"source"`
	Time            string                 `json:"time"`
	DataContentType string                 `json:"datacontenttype"`
	Data            AlertFiredData         `json:"data"`
}

// AlertFiredData 是 alert.fired 事件的数据载荷，包含告警 ID、规则 ID、
// 严重等级、触发值等上下文信息。
type AlertFiredData struct {
	AlertID        string            `json:"alert_id"`
	RuleID         string            `json:"rule_id"`
	Severity       string            `json:"severity"`
	Title          string            `json:"title"`
	Description    string            `json:"description,omitempty"`
	FiredAt        string            `json:"fired_at"`
	HostID         string            `json:"host_id,omitempty"`
	ServiceName    string            `json:"service_name,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	TriggerValues  map[string]interface{} `json:"trigger_values,omitempty"`
	SourceEventIDs []string          `json:"source_event_ids,omitempty"`
}

// KafkaConsumer 从 Kafka 消费日志/指标事件并送入 AlertEngine 评估。
// 当告警触发时，以 CloudEvents 格式发布到 opsnexus.alert.fired 主题。
type KafkaConsumer struct {
	brokers       []string
	groupID       string
	topic         string
	producerTopic string
	engine        *biz.AlertEngine
	log           *zap.SugaredLogger
}

// NewKafkaConsumer 创建 Kafka 消费者实例。
// brokers: Kafka 集群地址；groupID: 消费者组 ID；topic: 消费主题；producerTopic: 告警事件发布主题。
func NewKafkaConsumer(brokers []string, groupID, topic, producerTopic string, engine *biz.AlertEngine, log *zap.SugaredLogger) *KafkaConsumer {
	return &KafkaConsumer{
		brokers:       brokers,
		groupID:       groupID,
		topic:         topic,
		producerTopic: producerTopic,
		engine:        engine,
		log:           log,
	}
}

// Start 启动 Kafka 消费循环，同时创建同步生产者用于发布告警事件。
// 当 ctx 取消时优雅退出。生产者创建失败时降级运行（告警不会发布但评估仍继续）。
func (kc *KafkaConsumer) Start(ctx context.Context) error {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Producer.Return.Successes = true
	config.Version = sarama.V3_6_0_0

	group, err := sarama.NewConsumerGroup(kc.brokers, kc.groupID, config)
	if err != nil {
		return err
	}
	defer group.Close()

	// Create producer for alert.fired events
	producer, err := sarama.NewSyncProducer(kc.brokers, config)
	if err != nil {
		kc.log.Warnw("failed to create kafka producer, alerts won't be published", "error", err)
		producer = nil
	} else {
		defer producer.Close()
	}

	handler := &consumerGroupHandler{
		engine:        kc.engine,
		producer:      producer,
		producerTopic: kc.producerTopic,
		log:           kc.log,
	}

	kc.log.Infow("kafka consumer started", "topic", kc.topic, "group", kc.groupID)

	for {
		if err := group.Consume(ctx, []string{kc.topic}, handler); err != nil {
			kc.log.Errorw("kafka consume error", "error", err)
		}
		if ctx.Err() != nil {
			kc.log.Info("kafka consumer context cancelled, stopping")
			return nil
		}
	}
}

// consumerGroupHandler 实现 sarama.ConsumerGroupHandler 接口，处理 Kafka 消息的消费和告警发布。
type consumerGroupHandler struct {
	engine        *biz.AlertEngine
	producer      sarama.SyncProducer
	producerTopic string
	log           *zap.SugaredLogger
}

func (h *consumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim 逐条处理分区消息，处理完毕后标记 offset。
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.processMessage(msg)
		session.MarkMessage(msg, "")
	}
	return nil
}

// processMessage 解析 Kafka 消息并送入告警引擎评估。
// 优先尝试解析为 LogEvent（来自 opsnexus.log.ingested 主题），
// 失败后尝试解析为 MetricSample，均失败则记录警告日志。
func (h *consumerGroupHandler) processMessage(msg *sarama.ConsumerMessage) {
	// 优先尝试解析为日志事件（来自 opsnexus.log.ingested 主题）
	var logEvent biz.LogEvent
	if err := json.Unmarshal(msg.Value, &logEvent); err == nil && logEvent.Message != "" {
		result, err := h.engine.EvaluateLog(logEvent)
		if err != nil {
			h.log.Errorw("evaluate log event failed", "error", err)
			return
		}
		if result.Fired {
			h.publishAlerts(result.Alerts)
		}
		return
	}

	// 尝试解析为指标采样点
	var metric biz.MetricSample
	if err := json.Unmarshal(msg.Value, &metric); err == nil && metric.MetricName != "" {
		result, err := h.engine.EvaluateMetric(metric)
		if err != nil {
			h.log.Errorw("evaluate metric failed", "error", err)
			return
		}
		if result.Fired {
			h.publishAlerts(result.Alerts)
		}
		return
	}

	h.log.Warnw("unable to parse kafka message", "offset", msg.Offset, "partition", msg.Partition)
}

// publishAlerts 将触发的告警以 CloudEvents 格式发布到 Kafka 的 alert.fired 主题。
// 使用告警 ID 作为消息键，确保同一告警的事件路由到相同分区。
func (h *consumerGroupHandler) publishAlerts(alerts []*biz.Alert) {
	if h.producer == nil {
		return
	}

	for _, alert := range alerts {
		triggerValues := map[string]interface{}{}
		if alert.MetricValue != nil {
			triggerValues["metric_value"] = *alert.MetricValue
		}
		if alert.ThresholdValue != nil {
			triggerValues["threshold_value"] = *alert.ThresholdValue
		}

		event := AlertFiredEvent{
			SpecVersion:     "1.0",
			ID:              uuid.New().String(),
			Type:            "opsnexus.alert.fired",
			Source:          "/services/svc-alert",
			Time:            alert.TriggeredAt.Format(time.RFC3339),
			DataContentType: "application/json",
			Data: AlertFiredData{
				AlertID:       alert.AlertID,
				RuleID:        alert.RuleID,
				Severity:      string(alert.Severity),
				Title:         alert.Title,
				Description:   alert.Description,
				FiredAt:       alert.TriggeredAt.Format(time.RFC3339),
				HostID:        alert.SourceHost,
				ServiceName:   alert.SourceService,
				Labels:        alert.Tags,
				TriggerValues: triggerValues,
			},
		}

		payload, err := json.Marshal(event)
		if err != nil {
			h.log.Errorw("failed to marshal alert.fired event", "error", err)
			continue
		}

		_, _, err = h.producer.SendMessage(&sarama.ProducerMessage{
			Topic: h.producerTopic,
			Key:   sarama.StringEncoder(alert.AlertID),
			Value: sarama.ByteEncoder(payload),
		})
		if err != nil {
			h.log.Errorw("failed to publish alert.fired", "alert_id", alert.AlertID, "error", err)
		} else {
			h.log.Infow("published alert.fired", "alert_id", alert.AlertID, "topic", h.producerTopic)
		}
	}
}
