// Package event 提供基于 Kafka 的事件 SDK，用于跨域异步通信。
// 所有服务间的异步消息必须使用此包，以确保符合 CloudEvents 规范并保持
// 统一的主题（topic）和消费者组（consumer group）命名约定。
package event

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CloudEvent 表示符合 CloudEvents v1.0 规范的事件信封，
// 用于所有通过 Kafka 进行的跨域通信。
type CloudEvent struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Type            string          `json:"type"`
	Source          string          `json:"source"`
	Time            time.Time       `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
	// W3C Trace Context 扩展字段，用于分布式链路追踪传播
	TraceParent string `json:"traceparent,omitempty"`
	TraceState  string `json:"tracestate,omitempty"`
	// PartitionKey 用于 Kafka 消息分区，保证同一资源（如 host_id、alert_id）的事件有序
	PartitionKey string `json:"-"`
}

// NewCloudEvent 创建一个新的 CloudEvent，自动生成唯一 ID 和 UTC 时间戳。
func NewCloudEvent(eventType, source string, data interface{}) (*CloudEvent, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal event data: %w", err)
	}
	return &CloudEvent{
		SpecVersion:     "1.0",
		ID:              uuid.New().String(),
		Type:            eventType,
		Source:          source,
		Time:            time.Now().UTC(),
		DataContentType: "application/json",
		Data:            raw,
	}, nil
}

// WithPartitionKey 设置 Kafka 分区键以保证消息顺序。
func (e *CloudEvent) WithPartitionKey(key string) *CloudEvent {
	e.PartitionKey = key
	return e
}

// WithTraceContext 设置 W3C Trace Context 头，用于分布式链路追踪。
func (e *CloudEvent) WithTraceContext(traceParent, traceState string) *CloudEvent {
	e.TraceParent = traceParent
	e.TraceState = traceState
	return e
}

// DecodeData 将事件数据反序列化到目标结构体。
func (e *CloudEvent) DecodeData(target interface{}) error {
	if err := json.Unmarshal(e.Data, target); err != nil {
		return fmt.Errorf("decode event data (type=%s): %w", e.Type, err)
	}
	return nil
}

// Marshal 将事件序列化为 JSON 字节数组。
func (e *CloudEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// Unmarshal 从 JSON 字节数组反序列化一个 CloudEvent。
func Unmarshal(data []byte) (*CloudEvent, error) {
	var e CloudEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("unmarshal cloud event: %w", err)
	}
	return &e, nil
}

// TopicName 生成标准化的 Kafka 主题名称。
// 格式：opsnexus.{域}.{事件类型}
func TopicName(domain, eventType string) string {
	return fmt.Sprintf("opsnexus.%s.%s", domain, eventType)
}

// ConsumerGroupName 生成标准化的消费者组名称。
// 格式：{消费服务名}.{主题简称}
func ConsumerGroupName(service, topicShortName string) string {
	return fmt.Sprintf("%s.%s", service, topicShortName)
}

// 跨域通信事件类型常量
const (
	TypeLogIngested      = "opsnexus.log.ingested"
	TypeOperationLogged  = "opsnexus.operation.logged"
	TypeAlertFired       = "opsnexus.alert.fired"
	TypeAlertResolved    = "opsnexus.alert.resolved"
	TypeIncidentCreated  = "opsnexus.incident.created"
	TypeIncidentUpdated  = "opsnexus.incident.updated"
	TypeIncidentResolved = "opsnexus.incident.resolved"
	TypeAssetChanged     = "opsnexus.asset.changed"
	TypeAssetMaintenance = "opsnexus.asset.maintenance"
	TypeAIAnalysisDone   = "opsnexus.ai.analysis.done"
	TypeNotifySent       = "opsnexus.notify.sent"
)

// Kafka 主题名称，与事件类型一一对应
const (
	TopicLogIngested      = "opsnexus.log.ingested"
	TopicOperationLogged  = "opsnexus.operation.logged"
	TopicAlertFired       = "opsnexus.alert.fired"
	TopicAlertResolved    = "opsnexus.alert.resolved"
	TopicIncidentCreated  = "opsnexus.incident.created"
	TopicIncidentUpdated  = "opsnexus.incident.updated"
	TopicIncidentResolved = "opsnexus.incident.resolved"
	TopicAssetChanged     = "opsnexus.asset.changed"
	TopicAssetMaintenance = "opsnexus.asset.maintenance"
	TopicAIAnalysisDone   = "opsnexus.ai.analysis.done"
	TopicNotifySent       = "opsnexus.notify.sent"
)
