// Package biz 实现编排引擎的核心业务逻辑，包括工作流管理、执行引擎、
// 脚本沙箱和定时调度等功能。
package biz

import (
	"encoding/json"
	"time"
)

// --- 步骤类型枚举 ---

// StepType 表示工作流步骤的类型。
type StepType string

const (
	StepTypeScript   StepType = "script"   // 执行 Shell 脚本
	StepTypeApproval StepType = "approval" // 等待人工确认（Web 或企微）
	StepTypeCondition StepType = "condition" // 条件分支（true/false 路径）
	StepTypeParallel StepType = "parallel" // 并行执行多个子步骤
	StepTypeWait     StepType = "wait"     // 等待指定时间
	StepTypeNotify   StepType = "notify"   // 发送通知
)

// ValidStepTypes 包含所有合法的步骤类型，用于校验。
var ValidStepTypes = map[StepType]bool{
	StepTypeScript:    true,
	StepTypeApproval:  true,
	StepTypeCondition: true,
	StepTypeParallel:  true,
	StepTypeWait:      true,
	StepTypeNotify:    true,
}

// --- 触发类型枚举 ---

// TriggerType 表示工作流的触发方式。
type TriggerType string

const (
	TriggerManual   TriggerType = "manual"   // 手动触发
	TriggerAlert    TriggerType = "alert"    // 告警触发
	TriggerSchedule TriggerType = "schedule" // 定时触发
)

// --- 执行状态枚举 ---

// ExecutionStatus 表示工作流执行的生命周期状态。
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"   // 等待执行
	ExecutionStatusRunning   ExecutionStatus = "running"   // 执行中
	ExecutionStatusPaused    ExecutionStatus = "paused"    // 已暂停（等待审批等）
	ExecutionStatusCompleted ExecutionStatus = "completed" // 执行完成
	ExecutionStatusFailed    ExecutionStatus = "failed"    // 执行失败
	ExecutionStatusCancelled ExecutionStatus = "cancelled" // 已取消
)

// StepStatus 表示单个步骤的执行状态。
type StepStatus string

const (
	StepStatusPending  StepStatus = "pending"  // 等待执行
	StepStatusRunning  StepStatus = "running"  // 执行中
	StepStatusSuccess  StepStatus = "success"  // 执行成功
	StepStatusFailed   StepStatus = "failed"   // 执行失败
	StepStatusSkipped  StepStatus = "skipped"  // 已跳过
	StepStatusApproved StepStatus = "approved" // 已审批通过
	StepStatusRejected StepStatus = "rejected" // 已拒绝
)

// --- 核心实体 ---

// WorkflowStep 定义工作流中单个步骤的配置。
type WorkflowStep struct {
	Name       string          `json:"name"`                  // 步骤名称
	Type       StepType        `json:"type"`                  // 步骤类型
	Config     json.RawMessage `json:"config,omitempty"`      // 步骤配置（脚本内容、通知模板等）
	OnFailure  string          `json:"on_failure,omitempty"`  // 失败时行为：continue/stop（默认 stop）
	Timeout    int             `json:"timeout,omitempty"`     // 超时时间（秒），默认 30
	Conditions json.RawMessage `json:"conditions,omitempty"`  // 条件分支配置
	SubSteps   []WorkflowStep  `json:"sub_steps,omitempty"`   // 并行步骤的子步骤列表
}

// Workflow 表示一个工作流模板，定义了自动化编排的步骤序列。
type Workflow struct {
	ID          string          `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	Description string          `json:"description,omitempty" db:"description"`
	Steps       json.RawMessage `json:"steps" db:"steps"`           // JSON 编码的步骤列表
	Variables   json.RawMessage `json:"variables,omitempty" db:"variables"` // JSON 编码的变量定义
	TriggerType TriggerType     `json:"trigger_type" db:"trigger_type"`
	CronExpr    string          `json:"cron_expr,omitempty" db:"cron_expr"` // 定时触发的 cron 表达式
	CreatedBy   string          `json:"created_by,omitempty" db:"created_by"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
	IsActive    bool            `json:"is_active" db:"is_active"`
}

// WorkflowExecution 表示一次工作流执行记录。
type WorkflowExecution struct {
	ID            string          `json:"id" db:"id"`
	WorkflowID    string          `json:"workflow_id" db:"workflow_id"`
	TriggerType   TriggerType     `json:"trigger_type" db:"trigger_type"`
	TriggerSource string          `json:"trigger_source,omitempty" db:"trigger_source"` // 告警 ID / 事件 ID 等
	Status        ExecutionStatus `json:"status" db:"status"`
	Variables     json.RawMessage `json:"variables,omitempty" db:"variables"`
	StartedAt     *time.Time      `json:"started_at,omitempty" db:"started_at"`
	FinishedAt    *time.Time      `json:"finished_at,omitempty" db:"finished_at"`
	CreatedBy     string          `json:"created_by,omitempty" db:"created_by"`
}

// ExecutionStep 表示工作流执行中单个步骤的执行记录。
type ExecutionStep struct {
	ID          string          `json:"id" db:"id"`
	ExecutionID string          `json:"execution_id" db:"execution_id"`
	StepIndex   int             `json:"step_index" db:"step_index"`
	StepName    string          `json:"step_name" db:"step_name"`
	StepType    StepType        `json:"step_type" db:"step_type"`
	Status      StepStatus      `json:"status" db:"status"`
	Input       json.RawMessage `json:"input,omitempty" db:"input"`
	Output      json.RawMessage `json:"output,omitempty" db:"output"`
	StartedAt   *time.Time      `json:"started_at,omitempty" db:"started_at"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty" db:"finished_at"`
	ErrorMsg    string          `json:"error_msg,omitempty" db:"error_msg"`
}

// WorkflowTemplate 表示预置的工作流模板。
type WorkflowTemplate struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []WorkflowStep `json:"steps"`
	Variables   map[string]string `json:"variables,omitempty"`
}

// --- 仓储接口 ---

// WorkflowRepository 定义工作流模板的持久化操作接口。
type WorkflowRepository interface {
	Create(w *Workflow) error
	Update(w *Workflow) error
	Delete(id string) error
	GetByID(id string) (*Workflow, error)
	List(isActive *bool, pageSize int, offset int) ([]*Workflow, int, error)
}

// ExecutionRepository 定义工作流执行记录的持久化操作接口。
type ExecutionRepository interface {
	CreateExecution(e *WorkflowExecution) error
	UpdateExecution(e *WorkflowExecution) error
	GetExecution(id string) (*WorkflowExecution, error)
	ListExecutions(workflowID string, pageSize int, offset int) ([]*WorkflowExecution, int, error)
	CreateStep(s *ExecutionStep) error
	UpdateStep(s *ExecutionStep) error
	GetStep(executionID string, stepIndex int) (*ExecutionStep, error)
	ListSteps(executionID string) ([]*ExecutionStep, error)
}
