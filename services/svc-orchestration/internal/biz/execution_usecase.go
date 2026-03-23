package biz

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ExecutionUseCase 封装工作流执行的触发、查询、取消和步骤审批等业务逻辑。
type ExecutionUseCase struct {
	execRepo     ExecutionRepository
	workflowRepo WorkflowRepository
	sandbox      *ScriptSandbox
	log          *zap.SugaredLogger
}

// NewExecutionUseCase 创建执行用例实例。
func NewExecutionUseCase(execRepo ExecutionRepository, workflowRepo WorkflowRepository, log *zap.SugaredLogger) *ExecutionUseCase {
	return &ExecutionUseCase{
		execRepo:     execRepo,
		workflowRepo: workflowRepo,
		sandbox:      NewScriptSandbox(),
		log:          log,
	}
}

// TriggerExecution 触发工作流执行。
// 创建执行记录和各步骤记录，然后异步启动执行引擎。
func (uc *ExecutionUseCase) TriggerExecution(workflowID, triggerSource, createdBy string, variables json.RawMessage) (*WorkflowExecution, error) {
	// 获取工作流定义
	wf, err := uc.workflowRepo.GetByID(workflowID)
	if err != nil {
		return nil, fmt.Errorf("工作流不存在: %w", err)
	}

	// 解析步骤定义
	var steps []WorkflowStep
	if err := json.Unmarshal(wf.Steps, &steps); err != nil {
		return nil, fmt.Errorf("步骤解析失败: %w", err)
	}

	now := time.Now()
	exec := &WorkflowExecution{
		ID:            uuid.New().String(),
		WorkflowID:    workflowID,
		TriggerType:   wf.TriggerType,
		TriggerSource: triggerSource,
		Status:        ExecutionStatusPending,
		Variables:     variables,
		StartedAt:     &now,
		CreatedBy:     createdBy,
	}

	if err := uc.execRepo.CreateExecution(exec); err != nil {
		return nil, fmt.Errorf("创建执行记录失败: %w", err)
	}

	// 为每个步骤创建初始记录
	for i, step := range steps {
		stepRecord := &ExecutionStep{
			ID:          uuid.New().String(),
			ExecutionID: exec.ID,
			StepIndex:   i,
			StepName:    step.Name,
			StepType:    step.Type,
			Status:      StepStatusPending,
			Input:       step.Config,
		}
		if err := uc.execRepo.CreateStep(stepRecord); err != nil {
			uc.log.Errorf("创建步骤记录失败: executionID=%s, step=%d, %v", exec.ID, i, err)
		}
	}

	uc.log.Infof("工作流执行已触发: executionID=%s, workflowID=%s", exec.ID, workflowID)

	// 异步启动执行引擎
	go uc.runExecution(exec.ID)

	return exec, nil
}

// GetExecution 根据 ID 查询执行记录详情。
func (uc *ExecutionUseCase) GetExecution(id string) (*WorkflowExecution, error) {
	return uc.execRepo.GetExecution(id)
}

// ListExecutions 分页查询执行记录列表。
func (uc *ExecutionUseCase) ListExecutions(workflowID string, pageSize int, offset int) ([]*WorkflowExecution, int, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return uc.execRepo.ListExecutions(workflowID, pageSize, offset)
}

// CancelExecution 取消正在执行或等待中的工作流。
func (uc *ExecutionUseCase) CancelExecution(id string) error {
	exec, err := uc.execRepo.GetExecution(id)
	if err != nil {
		return fmt.Errorf("执行记录不存在: %w", err)
	}

	if exec.Status == ExecutionStatusCompleted || exec.Status == ExecutionStatusCancelled {
		return fmt.Errorf("执行已结束，无法取消（当前状态: %s）", exec.Status)
	}

	now := time.Now()
	exec.Status = ExecutionStatusCancelled
	exec.FinishedAt = &now

	if err := uc.execRepo.UpdateExecution(exec); err != nil {
		return fmt.Errorf("更新执行状态失败: %w", err)
	}

	uc.log.Infof("执行已取消: id=%s", id)
	return nil
}

// ApproveStep 审批通过指定步骤，将步骤状态更新为 approved。
func (uc *ExecutionUseCase) ApproveStep(executionID string, stepIndex int) error {
	step, err := uc.execRepo.GetStep(executionID, stepIndex)
	if err != nil {
		return fmt.Errorf("步骤不存在: %w", err)
	}

	if step.StepType != StepTypeApproval {
		return fmt.Errorf("步骤 %d 不是审批类型", stepIndex)
	}

	if step.Status != StepStatusRunning && step.Status != StepStatusPending {
		return fmt.Errorf("步骤 %d 当前状态 %s 不允许审批", stepIndex, step.Status)
	}

	now := time.Now()
	step.Status = StepStatusApproved
	step.FinishedAt = &now
	step.Output = json.RawMessage(`{"action":"approved"}`)

	if err := uc.execRepo.UpdateStep(step); err != nil {
		return fmt.Errorf("更新步骤状态失败: %w", err)
	}

	uc.log.Infof("步骤已审批通过: executionID=%s, step=%d", executionID, stepIndex)
	return nil
}

// RejectStep 拒绝指定步骤，将步骤状态更新为 rejected。
func (uc *ExecutionUseCase) RejectStep(executionID string, stepIndex int) error {
	step, err := uc.execRepo.GetStep(executionID, stepIndex)
	if err != nil {
		return fmt.Errorf("步骤不存在: %w", err)
	}

	if step.StepType != StepTypeApproval {
		return fmt.Errorf("步骤 %d 不是审批类型", stepIndex)
	}

	if step.Status != StepStatusRunning && step.Status != StepStatusPending {
		return fmt.Errorf("步骤 %d 当前状态 %s 不允许拒绝", stepIndex, step.Status)
	}

	now := time.Now()
	step.Status = StepStatusRejected
	step.FinishedAt = &now
	step.Output = json.RawMessage(`{"action":"rejected"}`)

	if err := uc.execRepo.UpdateStep(step); err != nil {
		return fmt.Errorf("更新步骤状态失败: %w", err)
	}

	uc.log.Infof("步骤已拒绝: executionID=%s, step=%d", executionID, stepIndex)
	return nil
}

// GetExecutionLog 获取完整的执行日志，包含所有步骤的输入输出信息。
func (uc *ExecutionUseCase) GetExecutionLog(executionID string) ([]*ExecutionStep, error) {
	return uc.execRepo.ListSteps(executionID)
}

// runExecution 异步执行工作流的所有步骤。
// 按步骤顺序依次执行，遇到失败根据 on_failure 策略决定是否继续。
func (uc *ExecutionUseCase) runExecution(executionID string) {
	exec, err := uc.execRepo.GetExecution(executionID)
	if err != nil {
		uc.log.Errorf("获取执行记录失败: %v", err)
		return
	}

	// 更新为运行中状态
	exec.Status = ExecutionStatusRunning
	_ = uc.execRepo.UpdateExecution(exec)

	// 获取工作流步骤定义
	wf, err := uc.workflowRepo.GetByID(exec.WorkflowID)
	if err != nil {
		uc.log.Errorf("获取工作流定义失败: %v", err)
		uc.failExecution(exec)
		return
	}

	var steps []WorkflowStep
	if err := json.Unmarshal(wf.Steps, &steps); err != nil {
		uc.log.Errorf("步骤解析失败: %v", err)
		uc.failExecution(exec)
		return
	}

	// 解析执行变量
	variables := make(map[string]string)
	if exec.Variables != nil {
		_ = json.Unmarshal(exec.Variables, &variables)
	}

	// 依次执行每个步骤
	for i, stepDef := range steps {
		// 检查是否已取消
		currentExec, _ := uc.execRepo.GetExecution(executionID)
		if currentExec != nil && currentExec.Status == ExecutionStatusCancelled {
			uc.log.Infof("执行已被取消，停止后续步骤: executionID=%s", executionID)
			return
		}

		err := uc.ExecuteStep(executionID, i, stepDef, variables)
		if err != nil {
			uc.log.Errorf("步骤执行失败: executionID=%s, step=%d, %v", executionID, i, err)
			if stepDef.OnFailure != "continue" {
				uc.failExecution(exec)
				return
			}
		}
	}

	// 所有步骤完成
	now := time.Now()
	exec.Status = ExecutionStatusCompleted
	exec.FinishedAt = &now
	_ = uc.execRepo.UpdateExecution(exec)
	uc.log.Infof("工作流执行完成: executionID=%s", executionID)
}

// ExecuteStep 执行单个步骤，根据步骤类型分发到对应的执行器。
func (uc *ExecutionUseCase) ExecuteStep(executionID string, stepIndex int, stepDef WorkflowStep, variables map[string]string) error {
	step, err := uc.execRepo.GetStep(executionID, stepIndex)
	if err != nil {
		return fmt.Errorf("获取步骤记录失败: %w", err)
	}

	// 更新为运行中
	now := time.Now()
	step.Status = StepStatusRunning
	step.StartedAt = &now
	_ = uc.execRepo.UpdateStep(step)

	var stepErr error
	switch stepDef.Type {
	case StepTypeScript:
		stepErr = uc.executeScriptStep(step, stepDef, variables)
	case StepTypeApproval:
		stepErr = uc.executeApprovalStep(step, executionID)
	case StepTypeWait:
		stepErr = uc.executeWaitStep(step, stepDef)
	case StepTypeNotify:
		stepErr = uc.executeNotifyStep(step, stepDef, variables)
	case StepTypeCondition:
		stepErr = uc.executeConditionStep(step, stepDef)
	case StepTypeParallel:
		stepErr = uc.executeParallelStep(step, stepDef)
	default:
		stepErr = fmt.Errorf("未知步骤类型: %s", stepDef.Type)
	}

	if stepErr != nil {
		finishedAt := time.Now()
		step.Status = StepStatusFailed
		step.FinishedAt = &finishedAt
		step.ErrorMsg = stepErr.Error()
		_ = uc.execRepo.UpdateStep(step)
		return stepErr
	}

	// 审批步骤不在此处标记完成（等待外部审批）
	if stepDef.Type != StepTypeApproval {
		finishedAt := time.Now()
		step.Status = StepStatusSuccess
		step.FinishedAt = &finishedAt
		_ = uc.execRepo.UpdateStep(step)
	}

	return nil
}

// executeScriptStep 在沙箱中执行脚本步骤。
func (uc *ExecutionUseCase) executeScriptStep(step *ExecutionStep, stepDef WorkflowStep, variables map[string]string) error {
	// 从步骤配置中提取脚本内容
	var config struct {
		Script string `json:"script"`
	}
	if err := json.Unmarshal(stepDef.Config, &config); err != nil {
		return fmt.Errorf("解析脚本配置失败: %w", err)
	}

	result, err := uc.sandbox.Execute(config.Script, variables)
	if result != nil {
		output, _ := json.Marshal(result)
		step.Output = output
	}

	return err
}

// executeApprovalStep 处理审批步骤：将执行暂停，等待外部审批。
func (uc *ExecutionUseCase) executeApprovalStep(step *ExecutionStep, executionID string) error {
	// 更新执行状态为暂停
	exec, err := uc.execRepo.GetExecution(executionID)
	if err != nil {
		return err
	}
	exec.Status = ExecutionStatusPaused
	_ = uc.execRepo.UpdateExecution(exec)

	uc.log.Infof("执行已暂停等待审批: executionID=%s, step=%s", executionID, step.StepName)

	// 轮询等待审批结果（实际生产环境应使用事件驱动）
	for i := 0; i < 3600; i++ { // 最多等待 1 小时
		time.Sleep(1 * time.Second)

		current, err := uc.execRepo.GetStep(step.ExecutionID, step.StepIndex)
		if err != nil {
			continue
		}

		switch current.Status {
		case StepStatusApproved:
			// 恢复执行
			exec.Status = ExecutionStatusRunning
			_ = uc.execRepo.UpdateExecution(exec)
			return nil
		case StepStatusRejected:
			return fmt.Errorf("审批被拒绝")
		}

		// 检查执行是否被取消
		currentExec, _ := uc.execRepo.GetExecution(executionID)
		if currentExec != nil && currentExec.Status == ExecutionStatusCancelled {
			return fmt.Errorf("执行已被取消")
		}
	}

	return fmt.Errorf("审批超时")
}

// executeWaitStep 执行等待步骤，暂停指定时间。
func (uc *ExecutionUseCase) executeWaitStep(step *ExecutionStep, stepDef WorkflowStep) error {
	var config struct {
		Duration int `json:"duration"` // 等待秒数
	}
	if err := json.Unmarshal(stepDef.Config, &config); err != nil {
		return fmt.Errorf("解析等待配置失败: %w", err)
	}
	if config.Duration <= 0 {
		config.Duration = 5
	}

	uc.log.Infof("等待步骤: %d 秒", config.Duration)
	time.Sleep(time.Duration(config.Duration) * time.Second)
	return nil
}

// executeNotifyStep 执行通知步骤（占位实现）。
// 生产环境应对接企微、邮件或其他通知渠道。
func (uc *ExecutionUseCase) executeNotifyStep(step *ExecutionStep, stepDef WorkflowStep, variables map[string]string) error {
	var config struct {
		Channel string `json:"channel"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(stepDef.Config, &config); err != nil {
		return fmt.Errorf("解析通知配置失败: %w", err)
	}

	uc.log.Infof("发送通知 [%s]: %s（占位实现）", config.Channel, config.Message)

	output, _ := json.Marshal(map[string]string{
		"channel": config.Channel,
		"status":  "sent_placeholder",
	})
	step.Output = output

	return nil
}

// executeConditionStep 执行条件分支步骤（占位实现）。
// 根据条件表达式计算 true/false 路径。
func (uc *ExecutionUseCase) executeConditionStep(step *ExecutionStep, stepDef WorkflowStep) error {
	uc.log.Infof("条件分支步骤: %s（占位实现，默认走 true 路径）", step.StepName)

	output, _ := json.Marshal(map[string]string{
		"branch": "true",
	})
	step.Output = output
	return nil
}

// executeParallelStep 执行并行步骤（占位实现）。
// 并行执行所有子步骤，等待全部完成。
func (uc *ExecutionUseCase) executeParallelStep(step *ExecutionStep, stepDef WorkflowStep) error {
	uc.log.Infof("并行步骤: %s, 子步骤数=%d（占位实现）", step.StepName, len(stepDef.SubSteps))

	output, _ := json.Marshal(map[string]interface{}{
		"sub_steps_count": len(stepDef.SubSteps),
		"status":          "completed_placeholder",
	})
	step.Output = output
	return nil
}

// failExecution 将执行标记为失败状态。
func (uc *ExecutionUseCase) failExecution(exec *WorkflowExecution) {
	now := time.Now()
	exec.Status = ExecutionStatusFailed
	exec.FinishedAt = &now
	_ = uc.execRepo.UpdateExecution(exec)
}
