package data

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opsnexus/svc-alert/internal/biz"
)

// SilenceRepo 实现 biz.SilenceRepository 接口，将静默规则持久化到 PostgreSQL silences 表。
type SilenceRepo struct {
	pool *pgxpool.Pool
}

// NewSilenceRepo 创建静默规则仓储实例。
func NewSilenceRepo(pool *pgxpool.Pool) *SilenceRepo {
	return &SilenceRepo{pool: pool}
}

// Create 将静默规则持久化到 silences 表。
func (r *SilenceRepo) Create(s *biz.Silence) error {
	matchersJSON, err := json.Marshal(s.Matchers)
	if err != nil {
		return fmt.Errorf("marshal matchers: %w", err)
	}

	_, err = r.pool.Exec(context.Background(),
		`INSERT INTO silences (silence_id, matchers, starts_at, ends_at, comment, created_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		s.ID, matchersJSON, s.StartsAt, s.EndsAt, s.Comment, s.CreatedBy, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("insert silence: %w", err)
	}
	return nil
}

// List 查询所有静默规则。
func (r *SilenceRepo) List() ([]*biz.Silence, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT silence_id, matchers, starts_at, ends_at, comment, created_by
		 FROM silences ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list silences: %w", err)
	}
	defer rows.Close()

	var silences []*biz.Silence
	for rows.Next() {
		var s biz.Silence
		var matchersJSON []byte
		if err := rows.Scan(&s.ID, &matchersJSON, &s.StartsAt, &s.EndsAt, &s.Comment, &s.CreatedBy); err != nil {
			return nil, fmt.Errorf("scan silence: %w", err)
		}
		_ = json.Unmarshal(matchersJSON, &s.Matchers)
		silences = append(silences, &s)
	}
	return silences, rows.Err()
}

// GetActive 查询当前时间内有效且匹配给定标签集的静默规则。
func (r *SilenceRepo) GetActive(labels map[string]string) ([]*biz.Silence, error) {
	now := time.Now()
	rows, err := r.pool.Query(context.Background(),
		`SELECT silence_id, matchers, starts_at, ends_at, comment, created_by
		 FROM silences WHERE starts_at <= $1 AND ends_at >= $1
		 ORDER BY created_at DESC`, now)
	if err != nil {
		return nil, fmt.Errorf("get active silences: %w", err)
	}
	defer rows.Close()

	var active []*biz.Silence
	for rows.Next() {
		var s biz.Silence
		var matchersJSON []byte
		if err := rows.Scan(&s.ID, &matchersJSON, &s.StartsAt, &s.EndsAt, &s.Comment, &s.CreatedBy); err != nil {
			return nil, fmt.Errorf("scan silence: %w", err)
		}
		_ = json.Unmarshal(matchersJSON, &s.Matchers)

		if matchesLabels(s.Matchers, labels) {
			active = append(active, &s)
		}
	}
	return active, rows.Err()
}

// Delete 按 silence_id 从数据库中删除指定的沉默规则（提前解除静默）。
// 实现了 biz.SilenceUseCase.DeleteSilence 通过类型断言检测的可选接口 SilenceDeletable。
//
// 参数：
//   - id：要删除的沉默规则 UUID（对应 silence_id 列）。
//
// 返回：删除成功返回 nil，数据库错误时返回包装后的错误。
func (r *SilenceRepo) Delete(id string) error {
	_, err := r.pool.Exec(context.Background(),
		`DELETE FROM silences WHERE silence_id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete silence %s: %w", id, err)
	}
	return nil
}

// matchesLabels 检查给定标签集是否匹配所有 matcher 条件。
func matchesLabels(matchers []biz.SilenceMatcher, labels map[string]string) bool {
	for _, m := range matchers {
		labelVal, exists := labels[m.Label]
		if !exists {
			return false
		}
		if m.IsRegex {
			matched, err := regexp.MatchString(m.Value, labelVal)
			if err != nil || !matched {
				return false
			}
		} else {
			if labelVal != m.Value {
				return false
			}
		}
	}
	return true
}
