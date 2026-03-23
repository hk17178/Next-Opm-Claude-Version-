// Package biz 定义通知服务的核心业务模型与领域逻辑，包括通知渠道、机器人、广播规则和去重引擎。
package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NotifyUseCase 处理通过 API 直接发送的通知（区别于事件驱动的广播）。
// 包含去重检查、渠道分发和通知日志记录。
type NotifyUseCase struct {
	channelManager *ChannelManager
	dedupEngine    *DedupEngine
	logRepo        NotificationLogRepo
	botRepo        BotRepo
	logger         *zap.Logger
}

// NewNotifyUseCase 创建通知用例实例。
func NewNotifyUseCase(
	channelManager *ChannelManager,
	dedupEngine *DedupEngine,
	logRepo NotificationLogRepo,
	botRepo BotRepo,
	logger *zap.Logger,
) *NotifyUseCase {
	return &NotifyUseCase{
		channelManager: channelManager,
		dedupEngine:    dedupEngine,
		logRepo:        logRepo,
		botRepo:        botRepo,
		logger:         logger,
	}
}

// Send 通过匹配的机器人发送通知，包含去重检查和通知日志记录。
// 返回通知日志记录和可能的错误。
func (uc *NotifyUseCase) Send(ctx context.Context, req SendRequest) (*NotificationLog, error) {
	// Find an enabled bot for this channel type
	bots, err := uc.botRepo.ListEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("list bots: %w", err)
	}

	var targetBot *Bot
	for _, bot := range bots {
		if bot.ChannelType == req.Channel {
			targetBot = bot
			break
		}
	}
	if targetBot == nil {
		return nil, fmt.Errorf("no enabled bot for channel %s", req.Channel)
	}

	// Dedup check
	dedupHash := DedupKey(req.Channel, fmt.Sprintf("%v", req.Recipients), "api_send", req.Content.Body)
	isDup, _ := uc.dedupEngine.IsDuplicate(ctx, dedupHash)
	if isDup {
		log := &NotificationLog{
			ID:          uuid.New(),
			BotID:       &targetBot.ID,
			ChannelType: req.Channel,
			Recipient:   fmt.Sprintf("%v", req.Recipients),
			MessageType: "api",
			ContentHash: dedupHash,
			Status:      StatusSuppressed,
			SentAt:      time.Now(),
		}
		_ = uc.logRepo.Create(ctx, log)
		return log, nil
	}

	// Send
	err = uc.channelManager.Send(ctx, targetBot, req.Content, req.Recipients)

	status := StatusSent
	errMsg := ""
	if err != nil {
		status = StatusFailed
		errMsg = err.Error()
		uc.logger.Error("notification send failed",
			zap.String("channel", string(req.Channel)),
			zap.Error(err),
		)
	} else {
		_ = uc.dedupEngine.MarkSent(ctx, dedupHash)
	}

	log := &NotificationLog{
		ID:             uuid.New(),
		BotID:          &targetBot.ID,
		ChannelType:    req.Channel,
		Recipient:      fmt.Sprintf("%v", req.Recipients),
		MessageType:    "api",
		IncidentID:     req.IncidentID,
		AlertID:        req.AlertID,
		ContentHash:    dedupHash,
		ContentPreview: truncatePreview(req.Content.Body, 200),
		Status:         status,
		ErrorMessage:   errMsg,
		SentAt:         time.Now(),
	}
	if err := uc.logRepo.Create(ctx, log); err != nil {
		uc.logger.Warn("failed to log notification", zap.Error(err))
	}

	if status == StatusFailed {
		return log, fmt.Errorf("send failed: %s", errMsg)
	}
	return log, nil
}

// GetNotification 根据 ID 查询通知记录。
func (uc *NotifyUseCase) GetNotification(ctx context.Context, id uuid.UUID) (*NotificationLog, error) {
	return uc.logRepo.GetByID(ctx, id)
}

// ListNotifications 查询通知历史记录列表，支持渠道、状态过滤和分页。
func (uc *NotifyUseCase) ListNotifications(ctx context.Context, filter NotificationFilter) ([]*NotificationLog, string, error) {
	return uc.logRepo.List(ctx, filter)
}
