// Package data 提供 svc-ai 服务的数据访问层实现，使用 PostgreSQL 持久化存储。
package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-ai/internal/biz"
)

// analysisRepo 实现 biz.AnalysisRepo 接口，使用 PostgreSQL 存储分析任务。
type analysisRepo struct {
	pool *pgxpool.Pool
}

// NewAnalysisRepo 创建分析任务仓储实例。
func NewAnalysisRepo(pool *pgxpool.Pool) biz.AnalysisRepo {
	return &analysisRepo{pool: pool}
}

// Create 将新的分析任务插入 analysis_tasks 表。
func (r *analysisRepo) Create(ctx context.Context, task *biz.AnalysisTask) error {
	alertIDs, _ := json.Marshal(task.AlertIDs)
	timeRange, _ := json.Marshal(task.TimeRange)

	_, err := r.pool.Exec(ctx,
		`INSERT INTO analysis_tasks (id, type, status, incident_id, alert_ids, time_range, context, trigger_event_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		task.ID, task.Type, task.Status, task.IncidentID, alertIDs, timeRange,
		task.Context, task.TriggerEventID, task.CreatedAt,
	)
	return err
}

// GetByID 根据 ID 查询分析任务的完整信息，包括 JSON 字段的反序列化。
func (r *analysisRepo) GetByID(ctx context.Context, id uuid.UUID) (*biz.AnalysisTask, error) {
	task := &biz.AnalysisTask{}
	var alertIDs, timeRange, result []byte

	err := r.pool.QueryRow(ctx,
		`SELECT id, type, status, incident_id, alert_ids, time_range, context, result,
		        model_version, trigger_event_id, error_message, created_at, started_at, completed_at
		 FROM analysis_tasks WHERE id = $1`, id,
	).Scan(
		&task.ID, &task.Type, &task.Status, &task.IncidentID,
		&alertIDs, &timeRange, &task.Context, &result,
		&task.ModelVersion, &task.TriggerEventID, &task.ErrorMessage,
		&task.CreatedAt, &task.StartedAt, &task.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get analysis task: %w", err)
	}

	if alertIDs != nil {
		_ = json.Unmarshal(alertIDs, &task.AlertIDs)
	}
	if timeRange != nil {
		_ = json.Unmarshal(timeRange, &task.TimeRange)
	}
	if result != nil {
		task.Result = &biz.AnalysisResult{}
		_ = json.Unmarshal(result, task.Result)
	}

	return task, nil
}

// UpdateStatus 更新分析任务的状态、结果和时间戳。
// 根据状态自动设置 started_at（运行时）和 completed_at（完成/失败时）。
func (r *analysisRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status biz.AnalysisStatus, result *biz.AnalysisResult, errMsg string) error {
	now := time.Now()
	var resultJSON []byte
	if result != nil {
		resultJSON, _ = json.Marshal(result)
	}

	var startedAt *time.Time
	if status == biz.StatusRunning {
		startedAt = &now
	}
	var completedAt *time.Time
	if status == biz.StatusSuccess || status == biz.StatusPartial || status == biz.StatusFailed {
		completedAt = &now
	}

	_, err := r.pool.Exec(ctx,
		`UPDATE analysis_tasks
		 SET status = $2, result = $3, error_message = $4, started_at = COALESCE($5, started_at), completed_at = COALESCE($6, completed_at)
		 WHERE id = $1`,
		id, status, resultJSON, errMsg, startedAt, completedAt,
	)
	return err
}

// SaveFeedback 保存用户对分析结果的反馈信息。
func (r *analysisRepo) SaveFeedback(ctx context.Context, analysisID uuid.UUID, req biz.FeedbackRequest) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE analysis_tasks
		 SET feedback_rating = $2, feedback_helpful = $3, feedback_comment = $4,
		     feedback_correct_root_cause = $5, feedback_at = NOW()
		 WHERE id = $1`,
		analysisID, req.Rating, req.Helpful, req.Comment, req.CorrectRootCause,
	)
	return err
}

// List 根据过滤条件分页查询分析任务列表，使用游标分页（pageToken 为上一页最后一条 ID）。
func (r *analysisRepo) List(ctx context.Context, filter biz.AnalysisFilter) ([]*biz.AnalysisTask, string, error) {
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	query := `SELECT id, type, status, incident_id, model_version, created_at, completed_at
	          FROM analysis_tasks WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, *filter.Type)
		argIdx++
	}
	if filter.PageToken != "" {
		query += fmt.Sprintf(" AND id < $%d", argIdx)
		args = append(args, filter.PageToken)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argIdx)
	args = append(args, pageSize+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list analysis tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*biz.AnalysisTask
	for rows.Next() {
		task := &biz.AnalysisTask{}
		err := rows.Scan(&task.ID, &task.Type, &task.Status, &task.IncidentID,
			&task.ModelVersion, &task.CreatedAt, &task.CompletedAt)
		if err != nil {
			return nil, "", err
		}
		tasks = append(tasks, task)
	}

	nextToken := ""
	if len(tasks) > pageSize {
		nextToken = tasks[pageSize-1].ID.String()
		tasks = tasks[:pageSize]
	}

	return tasks, nextToken, nil
}
