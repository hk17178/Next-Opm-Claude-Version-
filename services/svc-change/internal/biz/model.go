// Package biz 包含变更管理领域的核心业务逻辑，包括变更单全生命周期管理、审批流引擎和冲突检测。
package biz

import (
	"context"
	"time"
)

// AuditLogger 审计日志记录接口，用于将变更操作记录到 svc-log 审计服务。
// 实现方可以选择同步或异步方式记录日志。
type AuditLogger interface {
	// Log 记录一条审计日志。
	// action: 操作动作（如"创建变更单"）
	// operator: 操作人
	// resource: 资源类型（如"change_ticket"）
	// resourceID: 资源 ID
	// detail: 操作详情
	Log(ctx context.Context, action, operator, resource, resourceID, detail string) error
}

// MaintenanceClient 维护模式管理客户端接口，用于在变更执行期间联动告警维护模式。
type MaintenanceClient interface {
	// EnableMaintenance 为指定资产启用维护模式。
	// resourceIDs: 需要进入维护模式的资产 ID 列表
	// duration: 维护时长
	// reason: 维护原因
	EnableMaintenance(ctx context.Context, resourceIDs []string, duration time.Duration, reason string) error
	// DisableMaintenance 为指定资产解除维护模式。
	DisableMaintenance(ctx context.Context, resourceIDs []string) error
}

// AIRiskAssessor AI 风险评估接口，用于在变更提交审批时自动进行风险评估。
type AIRiskAssessor interface {
	// AssessRisk 对变更单进行 AI 风险评估，返回评估结果。
	AssessRisk(ctx context.Context, ticket *ChangeTicket) (*RiskAssessment, error)
}

// RiskAssessment AI 风险评估结果。
type RiskAssessment struct {
	RiskScore       float64  `json:"risk_score"`       // 风险分数，0-100
	RiskLevel       string   `json:"risk_level"`       // 风险等级：low/medium/high/critical
	ImpactSummary   string   `json:"impact_summary"`   // 影响范围摘要
	HistoricalFails int      `json:"historical_fails"` // 历史同类变更失败次数
	Suggestions     []string `json:"suggestions"`      // 改进建议
}

// ChangeType 变更类型，决定审批流程和风险评估策略。
type ChangeType string

const (
	ChangeTypeStandard  ChangeType = "standard"  // 标准变更：预审批，按既定流程执行
	ChangeTypeNormal    ChangeType = "normal"    // 常规变更：需正常审批流程
	ChangeTypeEmergency ChangeType = "emergency" // 紧急变更：加急审批，事后补审
	ChangeTypeMajor     ChangeType = "major"     // 重大变更：需多级审批
)

// RiskLevel 风险级别，影响审批路由和执行策略。
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"      // 低风险：可自动审批
	RiskMedium   RiskLevel = "medium"   // 中风险：需主管审批
	RiskHigh     RiskLevel = "high"     // 高风险：需总监审批
	RiskCritical RiskLevel = "critical" // 极高风险：需 VP 级审批
)

// ChangeStatus 变更单状态，定义变更从创建到完成的完整生命周期。
type ChangeStatus string

const (
	StatusDraft           ChangeStatus = "draft"            // 草稿：初始创建，可编辑
	StatusPendingApproval ChangeStatus = "pending_approval" // 待审批：已提交，等待审批人决策
	StatusApproved        ChangeStatus = "approved"         // 已批准：审批通过，可开始执行
	StatusInProgress      ChangeStatus = "in_progress"      // 执行中：正在实施变更
	StatusCompleted       ChangeStatus = "completed"        // 已完成：变更执行完毕
	StatusCancelled       ChangeStatus = "cancelled"        // 已取消：主动取消或因冲突取消
	StatusRejected        ChangeStatus = "rejected"         // 已拒绝：审批未通过
)

// validTransitions 定义变更单状态机的合法流转路径。
var validTransitions = map[ChangeStatus][]ChangeStatus{
	StatusDraft:           {StatusPendingApproval, StatusCancelled},
	StatusPendingApproval: {StatusApproved, StatusRejected, StatusCancelled},
	StatusApproved:        {StatusInProgress, StatusCancelled},
	StatusInProgress:      {StatusCompleted, StatusCancelled},
	StatusCompleted:       {},
	StatusCancelled:       {},
	StatusRejected:        {StatusDraft}, // 被拒绝后可退回草稿重新编辑
}

// CanTransition 校验从 from 到 to 的状态流转是否合法。
func CanTransition(from, to ChangeStatus) bool {
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

// IsTerminal 判断给定状态是否为终态（completed/cancelled/rejected 不可再变更）。
func IsTerminal(s ChangeStatus) bool {
	return s == StatusCompleted || s == StatusCancelled
}

// ChangeTicket 变更单，记录一次变更请求的完整信息。
type ChangeTicket struct {
	ID               string       `json:"id" db:"id"`                              // 变更单编号，如 CHG-20260322-001
	Title            string       `json:"title" db:"title"`                         // 变更标题
	Type             ChangeType   `json:"type" db:"type"`                           // 变更类型：standard/normal/emergency/major
	RiskLevel        RiskLevel    `json:"risk_level" db:"risk_level"`               // 风险级别：low/medium/high/critical
	Status           ChangeStatus `json:"status" db:"status"`                       // 状态
	Requester        string       `json:"requester" db:"requester"`                 // 申请人
	Approvers        []string     `json:"approvers" db:"approvers"`                 // 审批人列表
	ExecutorID       string       `json:"executor_id" db:"executor_id"`             // 执行人
	AffectedAssets   []string     `json:"affected_assets" db:"affected_assets"`     // 影响的资产 ID 列表
	RollbackPlan     string       `json:"rollback_plan" db:"rollback_plan"`         // 回滚方案
	ScheduledStart   time.Time    `json:"scheduled_start" db:"scheduled_start"`     // 计划开始时间
	ScheduledEnd     time.Time    `json:"scheduled_end" db:"scheduled_end"`         // 计划结束时间
	ActualStart      *time.Time   `json:"actual_start,omitempty" db:"actual_start"` // 实际开始时间
	ActualEnd        *time.Time   `json:"actual_end,omitempty" db:"actual_end"`     // 实际结束时间
	Description      string       `json:"description" db:"description"`             // 变更描述
	AIRiskSummary    string       `json:"ai_risk_summary" db:"ai_risk_summary"`     // AI 风险评估摘要
	RelatedChangeIDs []string     `json:"related_change_ids" db:"related_change_ids"` // 关联变更单（用于冲突检测）
	MaintenanceID    string       `json:"maintenance_id" db:"maintenance_id"`       // 关联的维护模式 ID
	CancelReason     string       `json:"cancel_reason,omitempty" db:"cancel_reason"` // 取消原因
	CreatedAt        time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at" db:"updated_at"`
}

// ApprovalRecord 审批记录，保存每一次审批决策的详细信息。
type ApprovalRecord struct {
	ID         string    `json:"id" db:"id"`
	ChangeID   string    `json:"change_id" db:"change_id"`
	ApproverID string    `json:"approver_id" db:"approver_id"`
	Decision   string    `json:"decision" db:"decision"`     // approved / rejected
	Comment    string    `json:"comment" db:"comment"`
	DecidedAt  time.Time `json:"decided_at" db:"decided_at"`
}

// ListFilter 封装变更单列表查询的过滤条件。
type ListFilter struct {
	Status    *ChangeStatus // 按状态筛选
	Type      *ChangeType   // 按变更类型筛选
	RiskLevel *RiskLevel    // 按风险级别筛选
	Requester *string       // 按申请人筛选
	StartTime *time.Time    // 计划开始时间范围（起）
	EndTime   *time.Time    // 计划开始时间范围（止）
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

// CreateChangeReq 创建变更单的请求参数。
type CreateChangeReq struct {
	Title          string     `json:"title"`
	Type           ChangeType `json:"type"`
	RiskLevel      RiskLevel  `json:"risk_level"`
	Requester      string     `json:"requester"`
	ExecutorID     string     `json:"executor_id"`
	AffectedAssets []string   `json:"affected_assets"`
	RollbackPlan   string     `json:"rollback_plan"`
	ScheduledStart time.Time  `json:"scheduled_start"`
	ScheduledEnd   time.Time  `json:"scheduled_end"`
	Description    string     `json:"description"`
	MaintenanceID  string     `json:"maintenance_id"`
}

// UpdateChangeReq 更新变更单的请求参数（仅允许在 draft 状态下更新）。
type UpdateChangeReq struct {
	Title          *string    `json:"title,omitempty"`
	Type           *ChangeType `json:"type,omitempty"`
	RiskLevel      *RiskLevel `json:"risk_level,omitempty"`
	ExecutorID     *string    `json:"executor_id,omitempty"`
	AffectedAssets []string   `json:"affected_assets,omitempty"`
	RollbackPlan   *string    `json:"rollback_plan,omitempty"`
	ScheduledStart *time.Time `json:"scheduled_start,omitempty"`
	ScheduledEnd   *time.Time `json:"scheduled_end,omitempty"`
	Description    *string    `json:"description,omitempty"`
	MaintenanceID  *string    `json:"maintenance_id,omitempty"`
}
