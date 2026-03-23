package data

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opsnexus/svc-orchestration/internal/biz"
)

// --- 工作流仓储实现 ---

// WorkflowRepo 实现 biz.WorkflowRepository 接口，持久化工作流模板到 PostgreSQL。
type WorkflowRepo struct {
	pool *pgxpool.Pool
}

// NewWorkflowRepo 创建工作流仓储实例。
func NewWorkflowRepo(pool *pgxpool.Pool) *WorkflowRepo {
	return &WorkflowRepo{pool: pool}
}

// Create 创建新的工作流模板记录。
func (r *WorkflowRepo) Create(w *biz.Workflow) error {
	sql := `INSERT INTO workflows (id, name, description, steps, variables, trigger_type, cron_expr, created_by, created_at, updated_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := r.pool.Exec(context.Background(), sql,
		w.ID, w.Name, w.Description, w.Steps, w.Variables,
		string(w.TriggerType), w.CronExpr, w.CreatedBy, w.CreatedAt, w.UpdatedAt, w.IsActive)
	return err
}

// Update 更新已有的工作流模板记录。
func (r *WorkflowRepo) Update(w *biz.Workflow) error {
	sql := `UPDATE workflows SET name=$1, description=$2, steps=$3, variables=$4,
		trigger_type=$5, cron_expr=$6, updated_at=$7, is_active=$8 WHERE id=$9`
	_, err := r.pool.Exec(context.Background(), sql,
		w.Name, w.Description, w.Steps, w.Variables,
		string(w.TriggerType), w.CronExpr, w.UpdatedAt, w.IsActive, w.ID)
	return err
}

// Delete 删除指定 ID 的工作流模板。
func (r *WorkflowRepo) Delete(id string) error {
	_, err := r.pool.Exec(context.Background(), `DELETE FROM workflows WHERE id=$1`, id)
	return err
}

// GetByID 根据 ID 查询工作流模板详情。
func (r *WorkflowRepo) GetByID(id string) (*biz.Workflow, error) {
	w := &biz.Workflow{}
	var triggerType string
	err := r.pool.QueryRow(context.Background(),
		`SELECT id, name, description, steps, variables, trigger_type, cron_expr, created_by, created_at, updated_at, is_active
		FROM workflows WHERE id=$1`, id).Scan(
		&w.ID, &w.Name, &w.Description, &w.Steps, &w.Variables,
		&triggerType, &w.CronExpr, &w.CreatedBy, &w.CreatedAt, &w.UpdatedAt, &w.IsActive)
	if err != nil {
		return nil, err
	}
	w.TriggerType = biz.TriggerType(triggerType)
	return w, nil
}

// List 分页查询工作流模板列表，支持按 is_active 过滤。
func (r *WorkflowRepo) List(isActive *bool, pageSize int, offset int) ([]*biz.Workflow, int, error) {
	// 构建查询条件
	var total int
	baseQuery := "FROM workflows"
	args := []interface{}{}
	argIdx := 1

	if isActive != nil {
		baseQuery += fmt.Sprintf(" WHERE is_active=$%d", argIdx)
		args = append(args, *isActive)
		argIdx++
	}

	// 查询总数
	countSQL := "SELECT COUNT(*) " + baseQuery
	if err := r.pool.QueryRow(context.Background(), countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 查询数据
	dataSQL := "SELECT id, name, description, steps, variables, trigger_type, cron_expr, created_by, created_at, updated_at, is_active " +
		baseQuery + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.pool.Query(context.Background(), dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var workflows []*biz.Workflow
	for rows.Next() {
		w := &biz.Workflow{}
		var triggerType string
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.Steps, &w.Variables,
			&triggerType, &w.CronExpr, &w.CreatedBy, &w.CreatedAt, &w.UpdatedAt, &w.IsActive); err != nil {
			return nil, 0, err
		}
		w.TriggerType = biz.TriggerType(triggerType)
		workflows = append(workflows, w)
	}

	return workflows, total, nil
}

// --- 执行记录仓储实现 ---

// ExecutionRepo 实现 biz.ExecutionRepository 接口，持久化执行记录到 PostgreSQL。
type ExecutionRepo struct {
	pool *pgxpool.Pool
}

// NewExecutionRepo 创建执行记录仓储实例。
func NewExecutionRepo(pool *pgxpool.Pool) *ExecutionRepo {
	return &ExecutionRepo{pool: pool}
}

// CreateExecution 创建新的执行记录。
func (r *ExecutionRepo) CreateExecution(e *biz.WorkflowExecution) error {
	sql := `INSERT INTO workflow_executions (id, workflow_id, trigger_type, trigger_source, status, variables, started_at, finished_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.pool.Exec(context.Background(), sql,
		e.ID, e.WorkflowID, string(e.TriggerType), e.TriggerSource, string(e.Status),
		e.Variables, e.StartedAt, e.FinishedAt, e.CreatedBy)
	return err
}

// UpdateExecution 更新执行记录的状态和时间。
func (r *ExecutionRepo) UpdateExecution(e *biz.WorkflowExecution) error {
	sql := `UPDATE workflow_executions SET status=$1, started_at=$2, finished_at=$3 WHERE id=$4`
	_, err := r.pool.Exec(context.Background(), sql,
		string(e.Status), e.StartedAt, e.FinishedAt, e.ID)
	return err
}

// GetExecution 根据 ID 查询执行记录详情。
func (r *ExecutionRepo) GetExecution(id string) (*biz.WorkflowExecution, error) {
	e := &biz.WorkflowExecution{}
	var triggerType, status string
	err := r.pool.QueryRow(context.Background(),
		`SELECT id, workflow_id, trigger_type, trigger_source, status, variables, started_at, finished_at, created_by
		FROM workflow_executions WHERE id=$1`, id).Scan(
		&e.ID, &e.WorkflowID, &triggerType, &e.TriggerSource, &status,
		&e.Variables, &e.StartedAt, &e.FinishedAt, &e.CreatedBy)
	if err != nil {
		return nil, err
	}
	e.TriggerType = biz.TriggerType(triggerType)
	e.Status = biz.ExecutionStatus(status)
	return e, nil
}

// ListExecutions 分页查询执行记录列表，可按 workflow_id 过滤。
func (r *ExecutionRepo) ListExecutions(workflowID string, pageSize int, offset int) ([]*biz.WorkflowExecution, int, error) {
	var total int
	baseQuery := "FROM workflow_executions"
	args := []interface{}{}
	argIdx := 1

	if workflowID != "" {
		baseQuery += fmt.Sprintf(" WHERE workflow_id=$%d", argIdx)
		args = append(args, workflowID)
		argIdx++
	}

	countSQL := "SELECT COUNT(*) " + baseQuery
	if err := r.pool.QueryRow(context.Background(), countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataSQL := "SELECT id, workflow_id, trigger_type, trigger_source, status, variables, started_at, finished_at, created_by " +
		baseQuery + fmt.Sprintf(" ORDER BY started_at DESC NULLS LAST LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.pool.Query(context.Background(), dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var executions []*biz.WorkflowExecution
	for rows.Next() {
		e := &biz.WorkflowExecution{}
		var triggerType, status string
		if err := rows.Scan(&e.ID, &e.WorkflowID, &triggerType, &e.TriggerSource, &status,
			&e.Variables, &e.StartedAt, &e.FinishedAt, &e.CreatedBy); err != nil {
			return nil, 0, err
		}
		e.TriggerType = biz.TriggerType(triggerType)
		e.Status = biz.ExecutionStatus(status)
		executions = append(executions, e)
	}

	return executions, total, nil
}

// CreateStep 创建执行步骤记录。
func (r *ExecutionRepo) CreateStep(s *biz.ExecutionStep) error {
	sql := `INSERT INTO execution_steps (id, execution_id, step_index, step_name, step_type, status, input, output, started_at, finished_at, error_msg)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := r.pool.Exec(context.Background(), sql,
		s.ID, s.ExecutionID, s.StepIndex, s.StepName, string(s.StepType), string(s.Status),
		s.Input, s.Output, s.StartedAt, s.FinishedAt, s.ErrorMsg)
	return err
}

// UpdateStep 更新执行步骤记录的状态、输出和时间。
func (r *ExecutionRepo) UpdateStep(s *biz.ExecutionStep) error {
	sql := `UPDATE execution_steps SET status=$1, output=$2, started_at=$3, finished_at=$4, error_msg=$5 WHERE id=$6`
	_, err := r.pool.Exec(context.Background(), sql,
		string(s.Status), s.Output, s.StartedAt, s.FinishedAt, s.ErrorMsg, s.ID)
	return err
}

// GetStep 根据执行 ID 和步骤索引查询步骤记录。
func (r *ExecutionRepo) GetStep(executionID string, stepIndex int) (*biz.ExecutionStep, error) {
	s := &biz.ExecutionStep{}
	var stepType, status string
	err := r.pool.QueryRow(context.Background(),
		`SELECT id, execution_id, step_index, step_name, step_type, status, input, output, started_at, finished_at, error_msg
		FROM execution_steps WHERE execution_id=$1 AND step_index=$2`, executionID, stepIndex).Scan(
		&s.ID, &s.ExecutionID, &s.StepIndex, &s.StepName, &stepType, &status,
		&s.Input, &s.Output, &s.StartedAt, &s.FinishedAt, &s.ErrorMsg)
	if err != nil {
		return nil, err
	}
	s.StepType = biz.StepType(stepType)
	s.Status = biz.StepStatus(status)
	return s, nil
}

// ListSteps 查询指定执行 ID 的所有步骤记录，按步骤索引排序。
func (r *ExecutionRepo) ListSteps(executionID string) ([]*biz.ExecutionStep, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT id, execution_id, step_index, step_name, step_type, status, input, output, started_at, finished_at, error_msg
		FROM execution_steps WHERE execution_id=$1 ORDER BY step_index`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []*biz.ExecutionStep
	for rows.Next() {
		s := &biz.ExecutionStep{}
		var stepType, status string
		if err := rows.Scan(&s.ID, &s.ExecutionID, &s.StepIndex, &s.StepName, &stepType, &status,
			&s.Input, &s.Output, &s.StartedAt, &s.FinishedAt, &s.ErrorMsg); err != nil {
			return nil, err
		}
		s.StepType = biz.StepType(stepType)
		s.Status = biz.StepStatus(status)
		steps = append(steps, s)
	}

	return steps, nil
}

// marshalJSON 将任意对象序列化为 JSON，用于步骤的输入输出。
func marshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}
