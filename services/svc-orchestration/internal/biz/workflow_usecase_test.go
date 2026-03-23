package biz

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockWorkflowRepo 是工作流仓储的内存模拟实现，用于单元测试。
type mockWorkflowRepo struct {
	workflows map[string]*Workflow
}

func newMockWorkflowRepo() *mockWorkflowRepo {
	return &mockWorkflowRepo{workflows: make(map[string]*Workflow)}
}

func (m *mockWorkflowRepo) Create(w *Workflow) error {
	m.workflows[w.ID] = w
	return nil
}

func (m *mockWorkflowRepo) Update(w *Workflow) error {
	if _, ok := m.workflows[w.ID]; !ok {
		return fmt.Errorf("not found")
	}
	m.workflows[w.ID] = w
	return nil
}

func (m *mockWorkflowRepo) Delete(id string) error {
	delete(m.workflows, id)
	return nil
}

func (m *mockWorkflowRepo) GetByID(id string) (*Workflow, error) {
	w, ok := m.workflows[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return w, nil
}

func (m *mockWorkflowRepo) List(isActive *bool, pageSize int, offset int) ([]*Workflow, int, error) {
	var result []*Workflow
	for _, w := range m.workflows {
		if isActive != nil && w.IsActive != *isActive {
			continue
		}
		result = append(result, w)
	}
	total := len(result)
	if offset >= len(result) {
		return nil, total, nil
	}
	end := offset + pageSize
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

// newTestLogger 创建测试用的日志记录器。
func newTestLogger() *zap.SugaredLogger {
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}

// TestCreateWorkflow_Success 测试正常创建工作流。
func TestCreateWorkflow_Success(t *testing.T) {
	repo := newMockWorkflowRepo()
	uc := NewWorkflowUseCase(repo, newTestLogger())

	steps := []WorkflowStep{
		{Name: "步骤1", Type: StepTypeScript, Config: json.RawMessage(`{"script":"echo hello"}`)},
		{Name: "步骤2", Type: StepTypeNotify, Config: json.RawMessage(`{"channel":"ops"}`)},
	}
	stepsJSON, _ := json.Marshal(steps)

	wf := &Workflow{
		Name:        "测试工作流",
		Description: "单元测试用工作流",
		Steps:       stepsJSON,
		TriggerType: TriggerManual,
	}

	err := uc.CreateWorkflow(wf)
	require.NoError(t, err)
	assert.NotEmpty(t, wf.ID)
	assert.True(t, wf.IsActive)
	assert.Equal(t, "测试工作流", wf.Name)
}

// TestCreateWorkflow_EmptyName 测试名称为空时应返回错误。
func TestCreateWorkflow_EmptyName(t *testing.T) {
	repo := newMockWorkflowRepo()
	uc := NewWorkflowUseCase(repo, newTestLogger())

	steps := []WorkflowStep{
		{Name: "步骤1", Type: StepTypeScript},
	}
	stepsJSON, _ := json.Marshal(steps)

	wf := &Workflow{
		Name:  "",
		Steps: stepsJSON,
	}

	err := uc.CreateWorkflow(wf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "名称不能为空")
}

// TestCreateWorkflow_EmptySteps 测试步骤列表为空时应返回错误。
func TestCreateWorkflow_EmptySteps(t *testing.T) {
	repo := newMockWorkflowRepo()
	uc := NewWorkflowUseCase(repo, newTestLogger())

	wf := &Workflow{
		Name:  "测试",
		Steps: json.RawMessage(`[]`),
	}

	err := uc.CreateWorkflow(wf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "至少需要一个步骤")
}

// TestValidateWorkflow_InvalidStepType 测试无效步骤类型时应返回错误。
func TestValidateWorkflow_InvalidStepType(t *testing.T) {
	repo := newMockWorkflowRepo()
	uc := NewWorkflowUseCase(repo, newTestLogger())

	steps := []WorkflowStep{
		{Name: "步骤1", Type: StepType("invalid_type")},
	}
	stepsJSON, _ := json.Marshal(steps)

	wf := &Workflow{
		Name:  "测试",
		Steps: stepsJSON,
	}

	err := uc.ValidateWorkflow(wf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "类型无效")
}

// TestValidateWorkflow_DuplicateStepName 测试步骤名称重复时应返回错误。
func TestValidateWorkflow_DuplicateStepName(t *testing.T) {
	repo := newMockWorkflowRepo()
	uc := NewWorkflowUseCase(repo, newTestLogger())

	steps := []WorkflowStep{
		{Name: "重复名称", Type: StepTypeScript},
		{Name: "重复名称", Type: StepTypeNotify},
	}
	stepsJSON, _ := json.Marshal(steps)

	wf := &Workflow{
		Name:  "测试",
		Steps: stepsJSON,
	}

	err := uc.ValidateWorkflow(wf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "名称重复")
}

// TestValidateWorkflow_ScheduleWithoutCron 测试定时触发缺少 cron 表达式时应返回错误。
func TestValidateWorkflow_ScheduleWithoutCron(t *testing.T) {
	repo := newMockWorkflowRepo()
	uc := NewWorkflowUseCase(repo, newTestLogger())

	steps := []WorkflowStep{
		{Name: "步骤1", Type: StepTypeScript},
	}
	stepsJSON, _ := json.Marshal(steps)

	wf := &Workflow{
		Name:        "定时工作流",
		Steps:       stepsJSON,
		TriggerType: TriggerSchedule,
		CronExpr:    "",
	}

	err := uc.ValidateWorkflow(wf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cron 表达式")
}

// TestListTemplates 测试预置模板列表。
func TestListTemplates(t *testing.T) {
	repo := newMockWorkflowRepo()
	uc := NewWorkflowUseCase(repo, newTestLogger())

	templates := uc.ListTemplates()
	assert.Len(t, templates, 4)

	// 验证模板名称
	names := make(map[string]bool)
	for _, tpl := range templates {
		names[tpl.Name] = true
	}
	assert.True(t, names["服务重启"])
	assert.True(t, names["磁盘清理"])
	assert.True(t, names["日志轮转"])
	assert.True(t, names["配置回滚"])
}

// TestCreateFromTemplate_Success 测试从模板创建工作流。
func TestCreateFromTemplate_Success(t *testing.T) {
	repo := newMockWorkflowRepo()
	uc := NewWorkflowUseCase(repo, newTestLogger())

	wf, err := uc.CreateFromTemplate("tpl-service-restart", "我的服务重启", "admin")
	require.NoError(t, err)
	assert.NotNil(t, wf)
	assert.Equal(t, "我的服务重启", wf.Name)
	assert.True(t, wf.IsActive)
}

// TestCreateFromTemplate_NotFound 测试从不存在的模板创建时应返回错误。
func TestCreateFromTemplate_NotFound(t *testing.T) {
	repo := newMockWorkflowRepo()
	uc := NewWorkflowUseCase(repo, newTestLogger())

	_, err := uc.CreateFromTemplate("tpl-nonexistent", "", "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "模板不存在")
}

// TestValidateWorkflow_ParallelWithoutSubSteps 测试并行步骤缺少子步骤时应返回错误。
func TestValidateWorkflow_ParallelWithoutSubSteps(t *testing.T) {
	repo := newMockWorkflowRepo()
	uc := NewWorkflowUseCase(repo, newTestLogger())

	steps := []WorkflowStep{
		{Name: "并行步骤", Type: StepTypeParallel, SubSteps: nil},
	}
	stepsJSON, _ := json.Marshal(steps)

	wf := &Workflow{
		Name:  "测试",
		Steps: stepsJSON,
	}

	err := uc.ValidateWorkflow(wf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "必须包含子步骤")
}
