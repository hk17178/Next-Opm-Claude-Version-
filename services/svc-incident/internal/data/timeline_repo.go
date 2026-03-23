package data

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-incident/internal/biz"
)

// TimelineRepo 实现 biz.TimelineRepo 接口，负责事件时间线记录的数据库持久化操作。
type TimelineRepo struct {
	db *pgxpool.Pool
}

// NewTimelineRepo 创建一个新的 TimelineRepo 实例。
func NewTimelineRepo(db *pgxpool.Pool) *TimelineRepo {
	return &TimelineRepo{db: db}
}

// Add 插入一条新的时间线记录，将 Content 字段序列化为 JSON 存储，回填 entry_id 和 created_at。
func (r *TimelineRepo) Add(ctx context.Context, entry *biz.TimelineEntry) error {
	content, err := json.Marshal(entry.Content)
	if err != nil {
		return fmt.Errorf("marshal timeline content: %w", err)
	}

	err = r.db.QueryRow(ctx,
		`INSERT INTO incident_timeline (incident_id, timestamp, entry_type, source, content)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING entry_id, created_at`,
		entry.IncidentID, entry.Timestamp, entry.EntryType, entry.Source, content,
	).Scan(&entry.EntryID, &entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert timeline entry: %w", err)
	}
	return nil
}

// ListByIncident 返回指定事件的所有时间线记录，按时间戳升序排列。
func (r *TimelineRepo) ListByIncident(ctx context.Context, incidentID string) ([]*biz.TimelineEntry, error) {
	rows, err := r.db.Query(ctx,
		`SELECT entry_id, incident_id, timestamp, entry_type, source, content, created_at
		 FROM incident_timeline
		 WHERE incident_id = $1
		 ORDER BY timestamp ASC`, incidentID)
	if err != nil {
		return nil, fmt.Errorf("list timeline: %w", err)
	}
	defer rows.Close()

	var entries []*biz.TimelineEntry
	for rows.Next() {
		e := &biz.TimelineEntry{}
		var content []byte
		if err := rows.Scan(
			&e.EntryID, &e.IncidentID, &e.Timestamp,
			&e.EntryType, &e.Source, &content, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan timeline entry: %w", err)
		}
		var parsed any
		json.Unmarshal(content, &parsed)
		e.Content = parsed
		entries = append(entries, e)
	}
	return entries, nil
}
