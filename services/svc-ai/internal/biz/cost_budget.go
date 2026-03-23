package biz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrBudgetExhausted 当月度 token 预算已 100% 耗尽时返回此错误。
// 调用方应据此停止使用该模型或切换到备用模型。
var ErrBudgetExhausted = errors.New("月度 token 预算已耗尽")

// BudgetStatus 表示预算检查的结果状态。
type BudgetStatus int

const (
	// BudgetOK 预算充足，可以正常调用模型（用量低于 80%）。
	BudgetOK BudgetStatus = iota

	// BudgetWarning80 已消耗 80% 以上的预算，发出告警但仍允许调用。
	BudgetWarning80

	// BudgetCritical100 预算已完全耗尽（100%），禁止继续调用。
	BudgetCritical100
)

// String 返回预算状态的字符串表示。
func (s BudgetStatus) String() string {
	switch s {
	case BudgetOK:
		return "ok"
	case BudgetWarning80:
		return "warning_80"
	case BudgetCritical100:
		return "critical_100"
	default:
		return "unknown"
	}
}

// BudgetManager 管理 AI 模型调用的月度 token 预算。
// 提供预算检查、用量记录和阈值告警功能：
//   - 用量达到 80% 时返回 BudgetWarning80 状态（告警）
//   - 用量达到 100% 时返回 BudgetCritical100 并拒绝后续调用
//
// 使用内存缓存减少数据库查询，通过互斥锁保证并发安全。
type BudgetManager struct {
	budgetRepo BudgetRepo
	logger     *zap.Logger

	mu    sync.RWMutex
	cache map[string]*budgetCacheEntry // 缓存键格式："2006-01:model_id"
}

// budgetCacheEntry 是预算信息的内存缓存条目。
type budgetCacheEntry struct {
	tokensUsed  int64     // 已消耗的 token 数
	budgetLimit int64     // 月度 token 预算上限
	lastUpdated time.Time // 缓存最后更新时间
}

// NewBudgetManager 创建预算管理器实例。
func NewBudgetManager(budgetRepo BudgetRepo, logger *zap.Logger) *BudgetManager {
	return &BudgetManager{
		budgetRepo: budgetRepo,
		logger:     logger,
		cache:      make(map[string]*budgetCacheEntry),
	}
}

// CheckBudget 检查指定模型在当月的预算使用情况。
//
// 参数：
//   - modelID: 要检查预算的模型 ID
//   - tokensNeeded: 本次调用预计消耗的 token 数
//
// 返回值：
//   - BudgetOK: 预算充足（加上 tokensNeeded 后仍低于 80%）
//   - BudgetWarning80: 加上 tokensNeeded 后用量将达到或超过 80%（仍允许调用）
//   - BudgetCritical100: 加上 tokensNeeded 后用量将达到或超过 100%，返回 ErrBudgetExhausted
func (bm *BudgetManager) CheckBudget(ctx context.Context, modelID uuid.UUID, tokensNeeded int) (BudgetStatus, error) {
	month := time.Now().Format("2006-01")
	cacheKey := fmt.Sprintf("%s:%s", month, modelID.String())

	// 先尝试从缓存获取预算信息
	bm.mu.RLock()
	entry, exists := bm.cache[cacheKey]
	bm.mu.RUnlock()

	// 缓存未命中或已过期（超过 60 秒），从数据库加载
	if !exists || time.Since(entry.lastUpdated) > 60*time.Second {
		budget, err := bm.budgetRepo.GetOrCreate(ctx, month, modelID, 1000000) // 默认 100 万 token
		if err != nil {
			return BudgetOK, fmt.Errorf("查询预算失败: %w", err)
		}

		entry = &budgetCacheEntry{
			tokensUsed:  budget.TokensUsed,
			budgetLimit: budget.BudgetLimit,
			lastUpdated: time.Now(),
		}

		bm.mu.Lock()
		bm.cache[cacheKey] = entry
		bm.mu.Unlock()
	}

	// 计算加上本次调用后的总用量
	projected := entry.tokensUsed + int64(tokensNeeded)

	// 预算已耗尽（达到 100%）
	if projected >= entry.budgetLimit {
		bm.logger.Warn("模型预算已耗尽",
			zap.String("model_id", modelID.String()),
			zap.Int64("used", entry.tokensUsed),
			zap.Int64("limit", entry.budgetLimit),
			zap.Int("tokens_needed", tokensNeeded),
		)
		return BudgetCritical100, ErrBudgetExhausted
	}

	// 用量达到 80%，发出告警
	warningThreshold := int64(float64(entry.budgetLimit) * 0.8)
	if projected >= warningThreshold {
		usagePercent := float64(projected) / float64(entry.budgetLimit) * 100
		bm.logger.Warn("模型预算使用率超过 80%",
			zap.String("model_id", modelID.String()),
			zap.Float64("usage_percent", usagePercent),
			zap.Int64("used", entry.tokensUsed),
			zap.Int64("limit", entry.budgetLimit),
		)
		return BudgetWarning80, nil
	}

	return BudgetOK, nil
}

// RecordUsage 记录一次模型调用的 token 消耗量。
// 同时更新数据库中的用量记录和内存缓存。
func (bm *BudgetManager) RecordUsage(ctx context.Context, modelID uuid.UUID, tokens int) {
	month := time.Now().Format("2006-01")
	cacheKey := fmt.Sprintf("%s:%s", month, modelID.String())

	// 更新数据库中的用量
	if err := bm.budgetRepo.IncrementUsage(ctx, month, modelID, int64(tokens)); err != nil {
		bm.logger.Warn("记录 token 用量失败",
			zap.String("model_id", modelID.String()),
			zap.Int("tokens", tokens),
			zap.Error(err),
		)
		return
	}

	// 更新内存缓存
	bm.mu.Lock()
	if entry, exists := bm.cache[cacheKey]; exists {
		entry.tokensUsed += int64(tokens)
		entry.lastUpdated = time.Now()
	}
	bm.mu.Unlock()

	bm.logger.Debug("token 用量已记录",
		zap.String("model_id", modelID.String()),
		zap.Int("tokens", tokens),
	)
}

// GetUsageSummary 获取指定模型当月的预算使用摘要。
func (bm *BudgetManager) GetUsageSummary(ctx context.Context, modelID uuid.UUID) (*BudgetUsageSummary, error) {
	month := time.Now().Format("2006-01")

	budget, err := bm.budgetRepo.GetOrCreate(ctx, month, modelID, 1000000)
	if err != nil {
		return nil, fmt.Errorf("查询预算摘要失败: %w", err)
	}

	usagePercent := float64(0)
	if budget.BudgetLimit > 0 {
		usagePercent = float64(budget.TokensUsed) / float64(budget.BudgetLimit) * 100
	}

	return &BudgetUsageSummary{
		Month:        budget.Month,
		ModelID:      budget.ModelID,
		TokensUsed:   budget.TokensUsed,
		BudgetLimit:  budget.BudgetLimit,
		UsagePercent: usagePercent,
		AlertSent:    budget.AlertSent,
		Exhausted:    budget.Exhausted,
	}, nil
}

// BudgetUsageSummary 是预算使用情况的摘要信息，用于管理界面展示。
type BudgetUsageSummary struct {
	Month        string    `json:"month"`         // 预算月份（格式 2006-01）
	ModelID      uuid.UUID `json:"model_id"`      // 模型 ID
	TokensUsed   int64     `json:"tokens_used"`   // 已消耗 token 数
	BudgetLimit  int64     `json:"budget_limit"`  // 月度预算上限
	UsagePercent float64   `json:"usage_percent"` // 使用率百分比
	AlertSent    bool      `json:"alert_sent"`    // 是否已发送 80% 告警
	Exhausted    bool      `json:"exhausted"`     // 是否已耗尽
}
