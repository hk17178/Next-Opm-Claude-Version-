// Package biz 包含事件管理领域的核心业务逻辑，包括事件生命周期状态机、升级策略、复盘和改进跟踪等功能。
package biz

import (
	"time"
)

// 事件严重等级，从 P0（致命）到 P4（信息），用于确定响应优先级和升级策略。
const (
	SeverityP0 = "P0" // 致命：核心业务完全不可用
	SeverityP1 = "P1" // 严重：核心功能严重受损
	SeverityP2 = "P2" // 重要：部分功能受损但有替代方案
	SeverityP3 = "P3" // 次要：非核心功能异常
	SeverityP4 = "P4" // 信息：无实际影响，仅需关注
)

// Status 表示事件生命周期的当前状态，遵循有限状态机模型。
type Status string

// 事件生命周期状态常量，定义从创建到关闭的完整流程。
// 状态流转：created → triaging → analyzing → assigned → resolving → verifying → resolved → postmortem → closed
const (
	StatusCreated    Status = "created"    // 已创建：事件刚刚录入
	StatusTriaging   Status = "triaging"   // 分诊中：正在评估严重程度和影响范围
	StatusAnalyzing  Status = "analyzing"  // 分析中：正在定位根因
	StatusAssigned   Status = "assigned"   // 已分配：已指派处理人员
	StatusResolving  Status = "resolving"  // 处理中：正在执行修复操作
	StatusVerifying  Status = "verifying"  // 验证中：修复完成，正在验证恢复情况
	StatusResolved   Status = "resolved"   // 已解决：确认问题已修复
	StatusPostmortem Status = "postmortem" // 复盘中：正在进行事后回顾
	StatusClosed     Status = "closed"     // 已关闭：事件处理完成（终态）
)

// RootCauseCategory 对事件根因进行分类，用于复盘统计和趋势分析。
type RootCauseCategory string

// 根因分类常量，覆盖常见的事件根因类型。
const (
	RootCauseHumanAction  RootCauseCategory = "human_action"  // 人为操作失误
	RootCauseChangeCaused RootCauseCategory = "change_caused" // 变更引发
	RootCauseSystemFault  RootCauseCategory = "system_fault"  // 系统自身故障
	RootCauseExternal     RootCauseCategory = "external"      // 外部因素（如第三方服务、网络等）
	RootCauseUnknown      RootCauseCategory = "unknown"       // 原因不明
)

// validTransitions 定义状态机的合法流转路径，键为当前状态，值为允许的目标状态列表。
var validTransitions = map[Status][]Status{
	StatusCreated:    {StatusTriaging, StatusAnalyzing, StatusAssigned},
	StatusTriaging:   {StatusAnalyzing, StatusAssigned},
	StatusAnalyzing:  {StatusAssigned, StatusResolving},
	StatusAssigned:   {StatusResolving, StatusAnalyzing},
	StatusResolving:  {StatusVerifying, StatusResolved},
	StatusVerifying:  {StatusResolved, StatusResolving},
	StatusResolved:   {StatusPostmortem, StatusClosed},
	StatusPostmortem: {StatusClosed},
	StatusClosed:     {},
}

// CanTransition 校验从 from 到 to 的状态流转是否合法。
func CanTransition(from, to Status) bool {
	targets, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

// IsTerminal 判断给定状态是否为终态（closed），终态事件不可再变更。
func IsTerminal(s Status) bool {
	return s == StatusClosed
}

// RequiresPostmortem 判断该严重等级是否要求在关闭前完成复盘（P0/P1 必须复盘）。
func RequiresPostmortem(severity string) bool {
	return severity == SeverityP0 || severity == SeverityP1
}

// RotationType 定义值班轮转的类型。
type RotationType string

const (
	RotationDaily  RotationType = "daily"
	RotationWeekly RotationType = "weekly"
	RotationCustom RotationType = "custom"
)

// RotationConfig 定义值班轮转的配置信息，从 OncallSchedule.Rotation JSON 字段中解析。
type RotationConfig struct {
	Type     RotationType `json:"type"`               // daily/weekly/custom
	Members  []OnCallMember `json:"members"`           // 参与轮转的成员列表
	StartDate string       `json:"start_date"`         // 轮转起始日期，格式 YYYY-MM-DD
	ShiftDays int          `json:"shift_days,omitempty"` // custom 模式下每人值班天数
}

// OnCallMember 表示值班排班中的一个成员。
type OnCallMember struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
}

// OverrideEntry 表示值班替班（代班）记录。
type OverrideEntry struct {
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"` // YYYY-MM-DD
	EndDate   string `json:"end_date"`   // YYYY-MM-DD
}

// Incident 表示一条事件记录，包含事件的完整生命周期信息、关联告警、受影响资产和复盘数据。
type Incident struct {
	IncidentID       string            `json:"incident_id" db:"incident_id"`
	Title            string            `json:"title" db:"title"`
	Description      string            `json:"description,omitempty" db:"description"`
	Severity         string            `json:"severity" db:"severity"`
	Status           Status            `json:"status" db:"status"`
	RootCauseCategory *string          `json:"root_cause_category,omitempty" db:"root_cause_category"`
	AssigneeID       *string           `json:"assignee_id,omitempty" db:"assignee_id"`
	AssigneeName     *string           `json:"assignee_name,omitempty" db:"assignee_name"`
	DetectedAt       time.Time         `json:"detected_at" db:"detected_at"`
	AcknowledgedAt   *time.Time        `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	ResolvedAt       *time.Time        `json:"resolved_at,omitempty" db:"resolved_at"`
	ClosedAt         *time.Time        `json:"closed_at,omitempty" db:"closed_at"`
	SourceAlerts     []string          `json:"source_alerts" db:"source_alerts"`
	AffectedAssets   []string          `json:"affected_assets" db:"affected_assets"`
	BusinessUnit     string            `json:"business_unit" db:"business_unit"`
	Postmortem       *Postmortem       `json:"postmortem,omitempty" db:"postmortem"`
	ImprovementItems []ImprovementItem `json:"improvement_items" db:"improvement_items"`
	Tags             map[string]string `json:"tags" db:"tags"`
	CreatedAt        time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at" db:"updated_at"`
}

// Postmortem 保存事件复盘（事后回顾）数据，包括根因分析、影响范围、改进措施和经验教训。
type Postmortem struct {
	RootCause    string   `json:"root_cause"`
	Impact       string   `json:"impact"`
	Timeline     string   `json:"timeline"`
	Improvements []string `json:"improvements"`
	Lessons      []string `json:"lessons"`
	AuthorID     string   `json:"author_id"`
	CompletedAt  string   `json:"completed_at"`
}

// ImprovementItem 跟踪复盘产出的改进项，包含负责人、截止日期和完成状态。
type ImprovementItem struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	AssigneeID  *string `json:"assignee_id,omitempty"`
	Status      string  `json:"status"` // 改进项状态：open/in_progress/completed
	DueDate     *string `json:"due_date,omitempty"`
	CompletedAt *string `json:"completed_at,omitempty"`
}

// TimelineEntry 表示事件时间线中的一条记录，完整记录事件处置过程中的每个关键动作。
type TimelineEntry struct {
	EntryID    string    `json:"entry_id" db:"entry_id"`
	IncidentID string    `json:"incident_id" db:"incident_id"`
	Timestamp  time.Time `json:"timestamp" db:"timestamp"`
	EntryType  string    `json:"entry_type" db:"entry_type"` // 条目类型：alert/status_change/ai_analysis/human_action/note/notification
	Source     string    `json:"source" db:"source"`         // 来源：system（系统自动）/human（人工操作）/ai（AI 分析）
	Content    any       `json:"content" db:"content"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// ChangeOrder 表示一条变更管理记录，用于跟踪基础设施或应用的计划变更及其审批流程。
type ChangeOrder struct {
	ChangeID         string    `json:"change_id" db:"change_id"`
	Title            string    `json:"title" db:"title"`
	ChangeType       string    `json:"change_type" db:"change_type"` // 变更类型：standard（标准）/normal（常规）/emergency（紧急）/major（重大）
	RiskLevel        string    `json:"risk_level" db:"risk_level"`   // 风险等级：low/medium/high/critical
	Status           string    `json:"status" db:"status"`
	RequesterID      *string   `json:"requester_id,omitempty" db:"requester_id"`
	ApproverID       *string   `json:"approver_id,omitempty" db:"approver_id"`
	ExecutorID       *string   `json:"executor_id,omitempty" db:"executor_id"`
	Plan             any       `json:"plan" db:"plan"`
	Schedule         any       `json:"schedule,omitempty" db:"schedule"`
	Result           any       `json:"result,omitempty" db:"result"`
	RelatedIncidents []string  `json:"related_incidents" db:"related_incidents"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// OncallSchedule 定义值班排班计划，包含轮转规则、升级策略和临时覆盖配置。
type OncallSchedule struct {
	ScheduleID string    `json:"schedule_id" db:"schedule_id"`
	Name       string    `json:"name" db:"name"`
	Scope      any       `json:"scope" db:"scope"`
	Rotation   any       `json:"rotation" db:"rotation"`
	Escalation any       `json:"escalation" db:"escalation"`
	Overrides  any       `json:"overrides,omitempty" db:"overrides"`
	Enabled    bool      `json:"enabled" db:"enabled"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// IncidentMetrics 保存事件关键时效指标的计算结果。
type IncidentMetrics struct {
	MTTASeconds *int64 `json:"mtta_seconds,omitempty"` // 平均确认时间（Mean Time To Acknowledge）
	MTTISeconds *int64 `json:"mtti_seconds,omitempty"` // 平均识别时间（Mean Time To Identify）
	MTTRSeconds *int64 `json:"mttr_seconds,omitempty"` // 平均修复时间（Mean Time To Resolve）
}

// CalculateMetrics 计算单个事件的 MTTA、MTTI、MTTR 时效指标（单位：秒）。
func (inc *Incident) CalculateMetrics() IncidentMetrics {
	m := IncidentMetrics{}
	if inc.AcknowledgedAt != nil {
		v := int64(inc.AcknowledgedAt.Sub(inc.DetectedAt).Seconds())
		m.MTTASeconds = &v
	}
	if inc.Status == StatusAssigned || inc.Status == StatusResolving || inc.Status == StatusVerifying ||
		inc.Status == StatusResolved || inc.Status == StatusPostmortem || inc.Status == StatusClosed {
		if inc.AcknowledgedAt != nil {
			v := int64(inc.AcknowledgedAt.Sub(inc.DetectedAt).Seconds())
			m.MTTISeconds = &v
		}
	}
	if inc.ResolvedAt != nil {
		v := int64(inc.ResolvedAt.Sub(inc.DetectedAt).Seconds())
		m.MTTRSeconds = &v
	}
	return m
}

// ListFilter 封装事件列表查询的过滤条件，支持按状态、严重等级、负责人和业务单元筛选。
type ListFilter struct {
	Status       *Status
	Severity     *string
	AssigneeID   *string
	BusinessUnit *string
	Page         int
	PageSize     int
	SortBy       string
	SortOrder    string
}

// IncidentChange 记录事件与变更工单的关联关系，用于追溯"哪些变更引发或参与了该事件的处置"。
// 对应数据库表 incident_changes。
type IncidentChange struct {
	ID            string    `json:"id" db:"id"`
	IncidentID    string    `json:"incident_id" db:"incident_id"`
	ChangeOrderID string    `json:"change_order_id" db:"change_order_id"`
	Description   string    `json:"description,omitempty" db:"description"`
	OperatorID    string    `json:"operator_id,omitempty" db:"operator_id"`
	OperatorName  string    `json:"operator_name,omitempty" db:"operator_name"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}
