package biz

import (
	"context"
	"time"
)

// 误差预算告警阈值常量。
//
// 业务依据：根据 SRE 最佳实践，当误差预算剩余不足 20% 时应发出预警，
// 提醒团队减少变更频率；不足 5% 时为严重告警，应冻结非关键变更。
// 这两个阈值对应了从"注意"到"紧急"的两级升级策略。
const (
	BudgetAlertThresholdWarning  = 20.0 // warning 级别：剩余预算 < 20%，建议减少变更
	BudgetAlertThresholdCritical = 5.0  // critical 级别：剩余预算 < 5%，应冻结非关键变更
)

// BudgetAlertEvent 是发布到 Kafka analytics.error_budget_alert 主题的事件负载。
// 包含完整的预算状态信息，供下游告警系统和通知服务消费。
type BudgetAlertEvent struct {
	ConfigID          string    `json:"config_id"`
	Dimension         string    `json:"dimension"`
	DimensionValue    string    `json:"dimension_value"`
	TargetPercentage  float64   `json:"target_pct"`
	ActualPercentage  float64   `json:"actual_pct"`
	ErrorBudgetTotal  float64   `json:"error_budget_total_seconds"`
	ErrorBudgetUsed   float64   `json:"error_budget_used_seconds"`
	ErrorBudgetRemain float64   `json:"error_budget_remain_pct"`
	Severity          string    `json:"severity"` // "warning" | "critical"
	FiredAt           time.Time `json:"fired_at"`
}

// EventProducer 定义领域事件发布接口，用于将告警事件推送到 Kafka。
type EventProducer interface {
	Publish(ctx context.Context, topic string, data interface{}) error
}

// CheckBudgetAlert 检查 SLA 计算结果的误差预算剩余量，低于阈值时生成告警事件。
// 返回告警事件（warning 或 critical），若预算充足则返回 nil。
// 判断逻辑：剩余 < 5% → critical，剩余 < 20% → warning，否则不告警。
func CheckBudgetAlert(result *SLAResult) *BudgetAlertEvent {
	var severity string
	switch {
	case result.ErrorBudgetRemain < BudgetAlertThresholdCritical:
		severity = "critical"
	case result.ErrorBudgetRemain < BudgetAlertThresholdWarning:
		severity = "warning"
	default:
		return nil
	}

	return &BudgetAlertEvent{
		ConfigID:          result.ConfigID,
		Dimension:         result.Dimension,
		DimensionValue:    result.DimensionValue,
		TargetPercentage:  result.TargetPercentage,
		ActualPercentage:  result.ActualPercentage,
		ErrorBudgetTotal:  result.ErrorBudgetTotal,
		ErrorBudgetUsed:   result.ErrorBudgetUsed,
		ErrorBudgetRemain: result.ErrorBudgetRemain,
		Severity:          severity,
		FiredAt:           time.Now(),
	}
}
