package biz

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockExecutionRepo 是执行记录仓储的内存模拟实现，用于单元测试。
type mockExecutionRepo struct {
	executions map[string]*WorkflowExecution
	steps      map[string][]*ExecutionStep // executionID -> steps
}

func newMockExecutionRepo() *mockExecutionRepo {
	return &mockExecutionRepo{
		executions: make(map[string]*WorkflowExecution),
		steps:      make(map[string][]*ExecutionStep),
	}
}

func (m *mockExecutionRepo) CreateExecution(e *WorkflowExecution) error {
	m.executions[e.ID] = e
	return nil
}

func (m *mockExecutionRepo) UpdateExecution(e *WorkflowExecution) error {
	m.executions[e.ID] = e
	return nil
}

func (m *mockExecutionRepo) GetExecution(id string) (*WorkflowExecution, error) {
	e, ok := m.executions[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return e, nil
}

func (m *mockExecutionRepo) ListExecutions(workflowID string, pageSize int, offset int) ([]*WorkflowExecution, int, error) {
	var result []*WorkflowExecution
	for _, e := range m.executions {
		if workflowID != "" && e.WorkflowID != workflowID {
			continue
		}
		result = append(result, e)
	}
	return result, len(result), nil
}

func (m *mockExecutionRepo) CreateStep(s *ExecutionStep) error {
	m.steps[s.ExecutionID] = append(m.steps[s.ExecutionID], s)
	return nil
}

func (m *mockExecutionRepo) UpdateStep(s *ExecutionStep) error {
	steps := m.steps[s.ExecutionID]
	for i, existing := range steps {
		if existing.ID == s.ID {
			steps[i] = s
			return nil
		}
	}
	return fmt.Errorf("step not found")
}

func (m *mockExecutionRepo) GetStep(executionID string, stepIndex int) (*ExecutionStep, error) {
	steps := m.steps[executionID]
	for _, s := range steps {
		if s.StepIndex == stepIndex {
			return s, nil
		}
	}
	return nil, fmt.Errorf("step not found")
}

func (m *mockExecutionRepo) ListSteps(executionID string) ([]*ExecutionStep, error) {
	return m.steps[executionID], nil
}

// setupTestUseCase 创建用于测试的执行用例和相关依赖。
func setupTestUseCase() (*ExecutionUseCase, *mockWorkflowRepo, *mockExecutionRepo) {
	wfRepo := newMockWorkflowRepo()
	execRepo := newMockExecutionRepo()
	uc := NewExecutionUseCase(execRepo, wfRepo, newTestLogger())
	return uc, wfRepo, execRepo
}

// createTestWorkflow 创建测试用的工作流并存入仓储。
func createTestWorkflow(repo *mockWorkflowRepo) *Workflow {
	steps := []WorkflowStep{
		{Name: "通知步骤", Type: StepTypeNotify, Config: json.RawMessage(`{"channel":"test","message":"hello"}`)},
	}
	stepsJSON, _ := json.Marshal(steps)

	wf := &Workflow{
		ID:          "wf-test-001",
		Name:        "测试工作流",
		Steps:       stepsJSON,
		TriggerType: TriggerManual,
		IsActive:    true,
	}
	repo.workflows[wf.ID] = wf
	return wf
}

// TestTriggerExecution_Success 测试正常触发执行。
func TestTriggerExecution_Success(t *testing.T) {
	uc, wfRepo, _ := setupTestUseCase()
	wf := createTestWorkflow(wfRepo)

	exec, err := uc.TriggerExecution(wf.ID, "manual", "admin", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, exec.ID)
	assert.Equal(t, wf.ID, exec.WorkflowID)
	// 执行是异步的，状态可能已经从 pending 变为 running 或 completed
	assert.Contains(t, []ExecutionStatus{ExecutionStatusPending, ExecutionStatusRunning, ExecutionStatusCompleted}, exec.Status)
}

// TestTriggerExecution_WorkflowNotFound 测试触发不存在的工作流时应返回错误。
func TestTriggerExecution_WorkflowNotFound(t *testing.T) {
	uc, _, _ := setupTestUseCase()

	_, err := uc.TriggerExecution("nonexistent", "", "admin", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "工作流不存在")
}

// TestCancelExecution_Success 测试正常取消执行。
func TestCancelExecution_Success(t *testing.T) {
	uc, _, execRepo := setupTestUseCase()

	now := time.Now()
	exec := &WorkflowExecution{
		ID:        "exec-001",
		Status:    ExecutionStatusRunning,
		StartedAt: &now,
	}
	execRepo.executions[exec.ID] = exec

	err := uc.CancelExecution(exec.ID)
	require.NoError(t, err)

	updated, _ := execRepo.GetExecution(exec.ID)
	assert.Equal(t, ExecutionStatusCancelled, updated.Status)
	assert.NotNil(t, updated.FinishedAt)
}

// TestCancelExecution_AlreadyCompleted 测试取消已完成的执行应返回错误。
func TestCancelExecution_AlreadyCompleted(t *testing.T) {
	uc, _, execRepo := setupTestUseCase()

	exec := &WorkflowExecution{
		ID:     "exec-002",
		Status: ExecutionStatusCompleted,
	}
	execRepo.executions[exec.ID] = exec

	err := uc.CancelExecution(exec.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无法取消")
}

// TestApproveStep_Success 测试正常审批通过步骤。
func TestApproveStep_Success(t *testing.T) {
	uc, _, execRepo := setupTestUseCase()

	step := &ExecutionStep{
		ID:          "step-001",
		ExecutionID: "exec-001",
		StepIndex:   0,
		StepType:    StepTypeApproval,
		Status:      StepStatusRunning,
	}
	execRepo.steps["exec-001"] = []*ExecutionStep{step}

	err := uc.ApproveStep("exec-001", 0)
	require.NoError(t, err)

	updated, _ := execRepo.GetStep("exec-001", 0)
	assert.Equal(t, StepStatusApproved, updated.Status)
}

// TestRejectStep_Success 测试正常拒绝步骤。
func TestRejectStep_Success(t *testing.T) {
	uc, _, execRepo := setupTestUseCase()

	step := &ExecutionStep{
		ID:          "step-002",
		ExecutionID: "exec-002",
		StepIndex:   0,
		StepType:    StepTypeApproval,
		Status:      StepStatusPending,
	}
	execRepo.steps["exec-002"] = []*ExecutionStep{step}

	err := uc.RejectStep("exec-002", 0)
	require.NoError(t, err)

	updated, _ := execRepo.GetStep("exec-002", 0)
	assert.Equal(t, StepStatusRejected, updated.Status)
}

// TestApproveStep_NotApprovalType 测试对非审批类型步骤执行审批应返回错误。
func TestApproveStep_NotApprovalType(t *testing.T) {
	uc, _, execRepo := setupTestUseCase()

	step := &ExecutionStep{
		ID:          "step-003",
		ExecutionID: "exec-003",
		StepIndex:   0,
		StepType:    StepTypeScript,
		Status:      StepStatusRunning,
	}
	execRepo.steps["exec-003"] = []*ExecutionStep{step}

	err := uc.ApproveStep("exec-003", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不是审批类型")
}

// TestGetExecutionLog 测试获取执行日志。
func TestGetExecutionLog(t *testing.T) {
	uc, _, execRepo := setupTestUseCase()

	steps := []*ExecutionStep{
		{ID: "s1", ExecutionID: "exec-log", StepIndex: 0, StepName: "步骤1", StepType: StepTypeScript, Status: StepStatusSuccess},
		{ID: "s2", ExecutionID: "exec-log", StepIndex: 1, StepName: "步骤2", StepType: StepTypeNotify, Status: StepStatusSuccess},
	}
	execRepo.steps["exec-log"] = steps

	result, err := uc.GetExecutionLog("exec-log")
	require.NoError(t, err)
	assert.Len(t, result, 2)
}
