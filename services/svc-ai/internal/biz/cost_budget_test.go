package biz

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// mockBudgetRepo 是 BudgetRepo 的内存模拟实现，用于测试。
type mockBudgetRepo struct {
	mu      sync.Mutex
	budgets map[string]*AIBudget // 键格式："month:model_id"
}

func newMockBudgetRepo() *mockBudgetRepo {
	return &mockBudgetRepo{
		budgets: make(map[string]*AIBudget),
	}
}

func (r *mockBudgetRepo) GetOrCreate(ctx context.Context, month string, modelID uuid.UUID, defaultLimit int64) (*AIBudget, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := month + ":" + modelID.String()
	if b, ok := r.budgets[key]; ok {
		return b, nil
	}

	b := &AIBudget{
		Month:       month,
		ModelID:     modelID,
		TokensUsed:  0,
		BudgetLimit: defaultLimit,
		AlertSent:   false,
		Exhausted:   false,
	}
	r.budgets[key] = b
	return b, nil
}

func (r *mockBudgetRepo) IncrementUsage(ctx context.Context, month string, modelID uuid.UUID, tokens int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := month + ":" + modelID.String()
	b, ok := r.budgets[key]
	if !ok {
		return nil
	}

	b.TokensUsed += tokens
	if float64(b.TokensUsed) >= float64(b.BudgetLimit)*0.8 {
		b.AlertSent = true
	}
	if b.TokensUsed >= b.BudgetLimit {
		b.Exhausted = true
	}
	return nil
}

// TestBudgetManager_CheckBudget_OK 验证预算充足时返回 BudgetOK。
func TestBudgetManager_CheckBudget_OK(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	repo := newMockBudgetRepo()
	bm := NewBudgetManager(repo, logger)

	modelID := uuid.New()
	status, err := bm.CheckBudget(context.Background(), modelID, 100)
	if err != nil {
		t.Fatalf("预算检查失败: %v", err)
	}
	if status != BudgetOK {
		t.Errorf("期望 BudgetOK，实际 %s", status)
	}
}

// TestBudgetManager_CheckBudget_Warning80 验证用量达到 80% 时返回告警状态。
func TestBudgetManager_CheckBudget_Warning80(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	repo := newMockBudgetRepo()
	bm := NewBudgetManager(repo, logger)

	modelID := uuid.New()

	// 先通过 CheckBudget 触发 GetOrCreate 创建预算记录
	_, _ = bm.CheckBudget(context.Background(), modelID, 0)

	// 记录 750000 token 的用量（75%）
	bm.RecordUsage(context.Background(), modelID, 750000)

	// 清除缓存以强制从数据库加载最新数据
	bm.mu.Lock()
	bm.cache = make(map[string]*budgetCacheEntry)
	bm.mu.Unlock()

	// 检查需要 60000 token（总计 81%），应触发 80% 告警
	status, err := bm.CheckBudget(context.Background(), modelID, 60000)
	if err != nil {
		t.Fatalf("预算检查失败: %v", err)
	}
	if status != BudgetWarning80 {
		t.Errorf("期望 BudgetWarning80，实际 %s", status)
	}
}

// TestBudgetManager_CheckBudget_Critical100 验证用量达到 100% 时返回耗尽错误。
func TestBudgetManager_CheckBudget_Critical100(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	repo := newMockBudgetRepo()
	bm := NewBudgetManager(repo, logger)

	modelID := uuid.New()

	// 先通过 CheckBudget 触发 GetOrCreate 创建预算记录
	_, _ = bm.CheckBudget(context.Background(), modelID, 0)

	// 记录 950000 token 的用量（95%）
	bm.RecordUsage(context.Background(), modelID, 950000)

	// 清除缓存
	bm.mu.Lock()
	bm.cache = make(map[string]*budgetCacheEntry)
	bm.mu.Unlock()

	// 检查需要 60000 token（总计 101%），应返回耗尽错误
	status, err := bm.CheckBudget(context.Background(), modelID, 60000)
	if !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("期望 ErrBudgetExhausted 错误，实际 %v", err)
	}
	if status != BudgetCritical100 {
		t.Errorf("期望 BudgetCritical100，实际 %s", status)
	}
}

// TestBudgetManager_RecordUsage 验证 token 用量被正确记录。
func TestBudgetManager_RecordUsage(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	repo := newMockBudgetRepo()
	bm := NewBudgetManager(repo, logger)

	modelID := uuid.New()

	// 先触发 GetOrCreate 创建预算记录
	_, _ = bm.CheckBudget(context.Background(), modelID, 0)

	// 记录用量
	bm.RecordUsage(context.Background(), modelID, 500)
	bm.RecordUsage(context.Background(), modelID, 300)

	// 从 repo 验证累积用量
	summary, err := bm.GetUsageSummary(context.Background(), modelID)
	if err != nil {
		t.Fatalf("获取用量摘要失败: %v", err)
	}

	if summary.TokensUsed != 800 {
		t.Errorf("期望已用 800 token，实际 %d", summary.TokensUsed)
	}
}

// TestBudgetStatus_String 验证预算状态的字符串表示。
func TestBudgetStatus_String(t *testing.T) {
	tests := []struct {
		status BudgetStatus
		want   string
	}{
		{BudgetOK, "ok"},
		{BudgetWarning80, "warning_80"},
		{BudgetCritical100, "critical_100"},
		{BudgetStatus(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("BudgetStatus(%d).String() = %q, 期望 %q", tt.status, got, tt.want)
		}
	}
}
