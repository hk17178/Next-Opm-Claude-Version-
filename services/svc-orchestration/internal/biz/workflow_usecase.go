package biz

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// WorkflowUseCase 封装工作流模板的增删改查及校验业务逻辑。
type WorkflowUseCase struct {
	repo WorkflowRepository
	log  *zap.SugaredLogger
}

// NewWorkflowUseCase 创建工作流用例实例。
func NewWorkflowUseCase(repo WorkflowRepository, log *zap.SugaredLogger) *WorkflowUseCase {
	return &WorkflowUseCase{repo: repo, log: log}
}

// CreateWorkflow 创建新的工作流模板。
// 会校验步骤定义的合法性，自动生成 UUID 并设置时间戳。
func (uc *WorkflowUseCase) CreateWorkflow(w *Workflow) error {
	// 校验步骤定义
	if err := uc.ValidateWorkflow(w); err != nil {
		return fmt.Errorf("工作流校验失败: %w", err)
	}

	w.ID = uuid.New().String()
	now := time.Now()
	w.CreatedAt = now
	w.UpdatedAt = now
	if w.TriggerType == "" {
		w.TriggerType = TriggerManual
	}
	w.IsActive = true

	if err := uc.repo.Create(w); err != nil {
		uc.log.Errorf("创建工作流失败: %v", err)
		return err
	}
	uc.log.Infof("工作流已创建: id=%s, name=%s", w.ID, w.Name)
	return nil
}

// GetWorkflow 根据 ID 查询工作流模板详情。
func (uc *WorkflowUseCase) GetWorkflow(id string) (*Workflow, error) {
	return uc.repo.GetByID(id)
}

// ListWorkflows 分页查询工作流模板列表。
func (uc *WorkflowUseCase) ListWorkflows(isActive *bool, pageSize int, offset int) ([]*Workflow, int, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return uc.repo.List(isActive, pageSize, offset)
}

// UpdateWorkflow 更新已有的工作流模板。
// 会重新校验步骤定义的合法性。
func (uc *WorkflowUseCase) UpdateWorkflow(w *Workflow) error {
	if err := uc.ValidateWorkflow(w); err != nil {
		return fmt.Errorf("工作流校验失败: %w", err)
	}
	w.UpdatedAt = time.Now()
	if err := uc.repo.Update(w); err != nil {
		uc.log.Errorf("更新工作流失败: id=%s, %v", w.ID, err)
		return err
	}
	uc.log.Infof("工作流已更新: id=%s", w.ID)
	return nil
}

// DeleteWorkflow 删除指定 ID 的工作流模板。
func (uc *WorkflowUseCase) DeleteWorkflow(id string) error {
	if err := uc.repo.Delete(id); err != nil {
		uc.log.Errorf("删除工作流失败: id=%s, %v", id, err)
		return err
	}
	uc.log.Infof("工作流已删除: id=%s", id)
	return nil
}

// ValidateWorkflow 校验工作流定义的合法性。
// 检查项：名称非空、步骤列表非空、步骤类型合法、无循环引用。
func (uc *WorkflowUseCase) ValidateWorkflow(w *Workflow) error {
	if w.Name == "" {
		return fmt.Errorf("工作流名称不能为空")
	}

	// 解析步骤定义
	var steps []WorkflowStep
	if err := json.Unmarshal(w.Steps, &steps); err != nil {
		return fmt.Errorf("步骤定义解析失败: %w", err)
	}

	if len(steps) == 0 {
		return fmt.Errorf("工作流至少需要一个步骤")
	}

	// 校验每个步骤
	seen := make(map[string]bool)
	for i, step := range steps {
		if err := uc.validateStep(step, i, seen); err != nil {
			return err
		}
	}

	// 校验触发类型
	if w.TriggerType == TriggerSchedule && w.CronExpr == "" {
		return fmt.Errorf("定时触发类型必须设置 cron 表达式")
	}

	return nil
}

// validateStep 校验单个步骤的定义是否合法。
// 检查步骤名称唯一性、类型合法性以及子步骤递归校验。
func (uc *WorkflowUseCase) validateStep(step WorkflowStep, index int, seen map[string]bool) error {
	if step.Name == "" {
		return fmt.Errorf("步骤 %d 名称不能为空", index)
	}

	if seen[step.Name] {
		return fmt.Errorf("步骤名称重复: %s（可能存在循环引用）", step.Name)
	}
	seen[step.Name] = true

	if !ValidStepTypes[step.Type] {
		return fmt.Errorf("步骤 %d (%s) 类型无效: %s", index, step.Name, step.Type)
	}

	// 递归校验并行步骤的子步骤
	if step.Type == StepTypeParallel {
		if len(step.SubSteps) == 0 {
			return fmt.Errorf("并行步骤 %s 必须包含子步骤", step.Name)
		}
		for j, sub := range step.SubSteps {
			if err := uc.validateStep(sub, j, seen); err != nil {
				return fmt.Errorf("并行步骤 %s 的子步骤校验失败: %w", step.Name, err)
			}
		}
	}

	return nil
}

// --- 预置模板 ---

// builtinTemplates 定义 4 个预置工作流模板。
var builtinTemplates = []WorkflowTemplate{
	{
		ID:          "tpl-service-restart",
		Name:        "服务重启",
		Description: "重启指定服务并验证恢复状态，适用于服务异常时的快速恢复",
		Steps: []WorkflowStep{
			{Name: "确认重启", Type: StepTypeApproval, Config: json.RawMessage(`{"message":"确认要重启服务 {{.ServiceName}} 吗？"}`)},
			{Name: "停止服务", Type: StepTypeScript, Config: json.RawMessage(`{"script":"systemctl stop {{.ServiceName}}"}`)},
			{Name: "等待停止", Type: StepTypeWait, Config: json.RawMessage(`{"duration":5}`)},
			{Name: "启动服务", Type: StepTypeScript, Config: json.RawMessage(`{"script":"systemctl start {{.ServiceName}}"}`)},
			{Name: "健康检查", Type: StepTypeScript, Config: json.RawMessage(`{"script":"systemctl is-active {{.ServiceName}}"}`)},
			{Name: "通知结果", Type: StepTypeNotify, Config: json.RawMessage(`{"channel":"ops","message":"服务 {{.ServiceName}} 已重启完成"}`)},
		},
		Variables: map[string]string{"ServiceName": ""},
	},
	{
		ID:          "tpl-disk-cleanup",
		Name:        "磁盘清理",
		Description: "清理临时文件和日志以释放磁盘空间，适用于磁盘告警场景",
		Steps: []WorkflowStep{
			{Name: "检查磁盘", Type: StepTypeScript, Config: json.RawMessage(`{"script":"df -h {{.MountPoint}}"}`)},
			{Name: "清理临时文件", Type: StepTypeScript, Config: json.RawMessage(`{"script":"find {{.MountPoint}}/tmp -type f -mtime +7 -delete 2>/dev/null; echo done"}`)},
			{Name: "清理旧日志", Type: StepTypeScript, Config: json.RawMessage(`{"script":"find {{.MountPoint}}/var/log -name '*.gz' -mtime +30 -delete 2>/dev/null; echo done"}`)},
			{Name: "确认结果", Type: StepTypeScript, Config: json.RawMessage(`{"script":"df -h {{.MountPoint}}"}`)},
			{Name: "通知", Type: StepTypeNotify, Config: json.RawMessage(`{"channel":"ops","message":"磁盘清理完成: {{.MountPoint}}"}`)},
		},
		Variables: map[string]string{"MountPoint": "/"},
	},
	{
		ID:          "tpl-log-rotation",
		Name:        "日志轮转",
		Description: "手动触发日志轮转并压缩归档，适用于日志文件过大的场景",
		Steps: []WorkflowStep{
			{Name: "执行轮转", Type: StepTypeScript, Config: json.RawMessage(`{"script":"logrotate -f /etc/logrotate.d/{{.LogConfig}}"}`)},
			{Name: "压缩归档", Type: StepTypeScript, Config: json.RawMessage(`{"script":"find /var/log -name '*.1' -exec gzip {} \\; 2>/dev/null; echo done"}`)},
			{Name: "通知", Type: StepTypeNotify, Config: json.RawMessage(`{"channel":"ops","message":"日志轮转完成: {{.LogConfig}}"}`)},
		},
		Variables: map[string]string{"LogConfig": "syslog"},
	},
	{
		ID:          "tpl-config-rollback",
		Name:        "配置回滚",
		Description: "回滚服务配置到上一版本并重启服务，适用于配置变更导致异常的场景",
		Steps: []WorkflowStep{
			{Name: "审批回滚", Type: StepTypeApproval, Config: json.RawMessage(`{"message":"确认要回滚 {{.ServiceName}} 的配置吗？"}`)},
			{Name: "备份当前配置", Type: StepTypeScript, Config: json.RawMessage(`{"script":"cp {{.ConfigPath}} {{.ConfigPath}}.rollback.$(date +%s)"}`)},
			{Name: "恢复旧配置", Type: StepTypeScript, Config: json.RawMessage(`{"script":"cp {{.BackupPath}} {{.ConfigPath}}"}`)},
			{Name: "重启服务", Type: StepTypeScript, Config: json.RawMessage(`{"script":"systemctl restart {{.ServiceName}}"}`)},
			{Name: "验证", Type: StepTypeScript, Config: json.RawMessage(`{"script":"systemctl is-active {{.ServiceName}}"}`)},
			{Name: "通知", Type: StepTypeNotify, Config: json.RawMessage(`{"channel":"ops","message":"配置回滚完成: {{.ServiceName}}"}`)},
		},
		Variables: map[string]string{"ServiceName": "", "ConfigPath": "", "BackupPath": ""},
	},
}

// ListTemplates 返回所有预置工作流模板列表。
func (uc *WorkflowUseCase) ListTemplates() []WorkflowTemplate {
	return builtinTemplates
}

// CreateFromTemplate 基于预置模板创建新的工作流。
// 根据模板 ID 查找模板，将模板步骤序列化为工作流步骤定义。
func (uc *WorkflowUseCase) CreateFromTemplate(templateID string, name string, createdBy string) (*Workflow, error) {
	// 查找模板
	var tpl *WorkflowTemplate
	for i := range builtinTemplates {
		if builtinTemplates[i].ID == templateID {
			tpl = &builtinTemplates[i]
			break
		}
	}
	if tpl == nil {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}

	// 序列化步骤和变量
	stepsJSON, err := json.Marshal(tpl.Steps)
	if err != nil {
		return nil, fmt.Errorf("序列化模板步骤失败: %w", err)
	}
	varsJSON, err := json.Marshal(tpl.Variables)
	if err != nil {
		return nil, fmt.Errorf("序列化模板变量失败: %w", err)
	}

	if name == "" {
		name = tpl.Name
	}

	w := &Workflow{
		Name:        name,
		Description: tpl.Description,
		Steps:       stepsJSON,
		Variables:   varsJSON,
		TriggerType: TriggerManual,
		CreatedBy:   createdBy,
	}

	if err := uc.CreateWorkflow(w); err != nil {
		return nil, err
	}
	return w, nil
}
