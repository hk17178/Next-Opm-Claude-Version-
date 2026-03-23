package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-incident/internal/biz"
)

// ScheduleRepo 实现值班排班的数据库持久化操作，支持 JSONB 范围查询匹配业务单元。
type ScheduleRepo struct {
	db *pgxpool.Pool
}

// NewScheduleRepo 创建一个新的 ScheduleRepo 实例。
func NewScheduleRepo(db *pgxpool.Pool) *ScheduleRepo {
	return &ScheduleRepo{db: db}
}

// Create 插入一条新的值班排班记录，将 scope/rotation/escalation/overrides 序列化为 JSON 存储。
func (r *ScheduleRepo) Create(ctx context.Context, s *biz.OncallSchedule) error {
	scopeJSON, _ := json.Marshal(s.Scope)
	rotationJSON, _ := json.Marshal(s.Rotation)
	escalationJSON, _ := json.Marshal(s.Escalation)
	overridesJSON, _ := json.Marshal(s.Overrides)

	err := r.db.QueryRow(ctx,
		`INSERT INTO oncall_schedules (name, scope, rotation, escalation, overrides, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING schedule_id`,
		s.Name, scopeJSON, rotationJSON, escalationJSON, overridesJSON, s.Enabled, time.Now(), time.Now(),
	).Scan(&s.ScheduleID)
	if err != nil {
		return fmt.Errorf("insert schedule: %w", err)
	}
	return nil
}

// GetByID 根据排班 ID 查询单条记录，不存在时返回 (nil, nil)。
func (r *ScheduleRepo) GetByID(ctx context.Context, id string) (*biz.OncallSchedule, error) {
	s := &biz.OncallSchedule{}
	var scopeJSON, rotationJSON, escalationJSON, overridesJSON []byte

	err := r.db.QueryRow(ctx,
		`SELECT schedule_id, name, scope, rotation, escalation, overrides, enabled, created_at, updated_at
		 FROM oncall_schedules WHERE schedule_id = $1`, id).Scan(
		&s.ScheduleID, &s.Name, &scopeJSON, &rotationJSON, &escalationJSON,
		&overridesJSON, &s.Enabled, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get schedule: %w", err)
	}

	json.Unmarshal(scopeJSON, &s.Scope)
	json.Unmarshal(rotationJSON, &s.Rotation)
	json.Unmarshal(escalationJSON, &s.Escalation)
	json.Unmarshal(overridesJSON, &s.Overrides)

	return s, nil
}

// List 返回所有值班排班列表，按名称升序排列。
func (r *ScheduleRepo) List(ctx context.Context) ([]*biz.OncallSchedule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT schedule_id, name, scope, rotation, escalation, overrides, enabled, created_at, updated_at
		 FROM oncall_schedules ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()

	var schedules []*biz.OncallSchedule
	for rows.Next() {
		s := &biz.OncallSchedule{}
		var scopeJSON, rotationJSON, escalationJSON, overridesJSON []byte
		if err := rows.Scan(
			&s.ScheduleID, &s.Name, &scopeJSON, &rotationJSON, &escalationJSON,
			&overridesJSON, &s.Enabled, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan schedule: %w", err)
		}
		json.Unmarshal(scopeJSON, &s.Scope)
		json.Unmarshal(rotationJSON, &s.Rotation)
		json.Unmarshal(escalationJSON, &s.Escalation)
		json.Unmarshal(overridesJSON, &s.Overrides)
		schedules = append(schedules, s)
	}
	return schedules, nil
}

// Update 更新排班记录并刷新 updated_at 时间戳。
func (r *ScheduleRepo) Update(ctx context.Context, s *biz.OncallSchedule) error {
	scopeJSON, _ := json.Marshal(s.Scope)
	rotationJSON, _ := json.Marshal(s.Rotation)
	escalationJSON, _ := json.Marshal(s.Escalation)
	overridesJSON, _ := json.Marshal(s.Overrides)

	_, err := r.db.Exec(ctx,
		`UPDATE oncall_schedules SET
			name=$2, scope=$3, rotation=$4, escalation=$5, overrides=$6,
			enabled=$7, updated_at=$8
		 WHERE schedule_id = $1`,
		s.ScheduleID, s.Name, scopeJSON, rotationJSON, escalationJSON,
		overridesJSON, s.Enabled, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	return nil
}

// Delete 根据排班 ID 删除一条记录。
func (r *ScheduleRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM oncall_schedules WHERE schedule_id = $1", id)
	return err
}

// FindByScope 通过 JSONB 包含查询（@>）查找匹配指定业务单元的已启用排班计划。
func (r *ScheduleRepo) FindByScope(ctx context.Context, businessUnit string) ([]*biz.OncallSchedule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT schedule_id, name, scope, rotation, escalation, overrides, enabled, created_at, updated_at
		 FROM oncall_schedules
		 WHERE enabled = true AND scope @> $1::jsonb
		 ORDER BY name ASC`,
		fmt.Sprintf(`{"business_units": ["%s"]}`, businessUnit))
	if err != nil {
		return nil, fmt.Errorf("find schedules by scope: %w", err)
	}
	defer rows.Close()

	var schedules []*biz.OncallSchedule
	for rows.Next() {
		s := &biz.OncallSchedule{}
		var scopeJSON, rotationJSON, escalationJSON, overridesJSON []byte
		if err := rows.Scan(
			&s.ScheduleID, &s.Name, &scopeJSON, &rotationJSON, &escalationJSON,
			&overridesJSON, &s.Enabled, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan schedule: %w", err)
		}
		json.Unmarshal(scopeJSON, &s.Scope)
		json.Unmarshal(rotationJSON, &s.Rotation)
		json.Unmarshal(escalationJSON, &s.Escalation)
		json.Unmarshal(overridesJSON, &s.Overrides)
		schedules = append(schedules, s)
	}
	return schedules, nil
}
