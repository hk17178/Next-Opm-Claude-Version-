// Package data 实现通知服务的数据访问层，提供 PostgreSQL 的具体存储操作。
package data

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-notify/internal/biz"
)

// notificationLogRepo 使用 PostgreSQL 实现 biz.NotificationLogRepo 接口，负责通知日志的存储和查询。
type notificationLogRepo struct {
	pool *pgxpool.Pool
}

// NewNotificationLogRepo 创建一个新的通知日志仓储实例。
func NewNotificationLogRepo(pool *pgxpool.Pool) biz.NotificationLogRepo {
	return &notificationLogRepo{pool: pool}
}

func (r *notificationLogRepo) Create(ctx context.Context, log *biz.NotificationLog) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notification_logs (log_id, bot_id, channel_type, recipient, message_type,
		 event_node, incident_id, alert_id, content_hash, content_preview, status, error_message, retry_count, sent_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		log.ID, log.BotID, log.ChannelType, log.Recipient, log.MessageType,
		log.EventNode, log.IncidentID, log.AlertID, log.ContentHash,
		log.ContentPreview, log.Status, log.ErrorMessage, log.RetryCount, log.SentAt,
	)
	return err
}

func (r *notificationLogRepo) GetByID(ctx context.Context, id uuid.UUID) (*biz.NotificationLog, error) {
	log := &biz.NotificationLog{}
	err := r.pool.QueryRow(ctx,
		`SELECT log_id, bot_id, channel_type, recipient, message_type,
		        event_node, incident_id, alert_id, content_hash, content_preview,
		        status, error_message, retry_count, sent_at
		 FROM notification_logs WHERE log_id = $1`, id,
	).Scan(
		&log.ID, &log.BotID, &log.ChannelType, &log.Recipient, &log.MessageType,
		&log.EventNode, &log.IncidentID, &log.AlertID, &log.ContentHash,
		&log.ContentPreview, &log.Status, &log.ErrorMessage, &log.RetryCount, &log.SentAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get notification log: %w", err)
	}
	return log, nil
}

func (r *notificationLogRepo) List(ctx context.Context, filter biz.NotificationFilter) ([]*biz.NotificationLog, string, error) {
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	query := `SELECT log_id, bot_id, channel_type, recipient, message_type,
	          event_node, incident_id, alert_id, status, error_message, retry_count, sent_at
	          FROM notification_logs WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.Channel != nil {
		query += fmt.Sprintf(" AND channel_type = $%d", argIdx)
		args = append(args, *filter.Channel)
		argIdx++
	}
	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.PageToken != "" {
		query += fmt.Sprintf(" AND log_id < $%d", argIdx)
		args = append(args, filter.PageToken)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY sent_at DESC LIMIT $%d", argIdx)
	args = append(args, pageSize+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var logs []*biz.NotificationLog
	for rows.Next() {
		log := &biz.NotificationLog{}
		err := rows.Scan(
			&log.ID, &log.BotID, &log.ChannelType, &log.Recipient, &log.MessageType,
			&log.EventNode, &log.IncidentID, &log.AlertID, &log.Status,
			&log.ErrorMessage, &log.RetryCount, &log.SentAt,
		)
		if err != nil {
			return nil, "", err
		}
		logs = append(logs, log)
	}

	nextToken := ""
	if len(logs) > pageSize {
		nextToken = logs[pageSize-1].ID.String()
		logs = logs[:pageSize]
	}

	return logs, nextToken, nil
}
