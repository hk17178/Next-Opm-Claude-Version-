package biz

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// --- Mock 建议仓储 ---

// mockSuggestionRepo 是 SuggestionRepo 的内存模拟实现，用于单元测试。
type mockSuggestionRepo struct {
	suggestions map[string]*Suggestion // 内存中的建议存储
	nextID      int                    // 自增 ID 计数器
}

// newMockSuggestionRepo 创建空的模拟建议仓储实例。
func newMockSuggestionRepo() *mockSuggestionRepo {
	return &mockSuggestionRepo{
		suggestions: make(map[string]*Suggestion),
	}
}

func (m *mockSuggestionRepo) Create(_ context.Context, s *Suggestion) error {
	m.nextID++
	s.ID = fmt.Sprintf("sug-%d", m.nextID)
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	m.suggestions[s.ID] = s
	return nil
}

func (m *mockSuggestionRepo) Get(_ context.Context, id string) (*Suggestion, error) {
	s, ok := m.suggestions[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return s, nil
}

func (m *mockSuggestionRepo) List(_ context.Context, status string, page, pageSize int) ([]*Suggestion, int, error) {
	var result []*Suggestion
	for _, s := range m.suggestions {
		if status != "" && string(s.Status) != status {
			continue
		}
		result = append(result, s)
	}
	total := len(result)
	// 简单分页
	start := (page - 1) * pageSize
	if start >= total {
		return nil, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return result[start:end], total, nil
}

func (m *mockSuggestionRepo) UpdateStatus(_ context.Context, id string, status SuggestionStatus, adminNote string) error {
	s, ok := m.suggestions[id]
	if !ok {
		return fmt.Errorf("not found: %s", id)
	}
	s.Status = status
	s.AdminNote = adminNote
	s.UpdatedAt = time.Now()
	return nil
}

func (m *mockSuggestionRepo) GetStats(_ context.Context) (*SuggestionStats, error) {
	stats := &SuggestionStats{}
	for _, s := range m.suggestions {
		stats.Total++
		switch s.Status {
		case SuggestionStatusPending:
			stats.Pending++
		case SuggestionStatusAccepted:
			stats.Accepted++
		case SuggestionStatusRejected:
			stats.Rejected++
		case SuggestionStatusInProgress:
			stats.InProgress++
		case SuggestionStatusLaunched:
			stats.Launched++
		}
	}
	if stats.Total > 0 {
		stats.AcceptanceRate = float64(stats.Accepted+stats.InProgress+stats.Launched) / float64(stats.Total) * 100
	}
	return stats, nil
}

// --- Mock AI 服务 ---

// mockSuggestionAI 是 SuggestionAIService 的模拟实现。
type mockSuggestionAI struct{}

func (m *mockSuggestionAI) Classify(_ context.Context, _, _ string) (string, error) {
	return "功能", nil
}

func (m *mockSuggestionAI) ExtractKeywords(_ context.Context, _, _ string) ([]string, error) {
	return []string{"性能", "优化"}, nil
}

func (m *mockSuggestionAI) AnalyzeSentiment(_ context.Context, _, _ string) (string, error) {
	return "positive", nil
}

func (m *mockSuggestionAI) FindSimilar(_ context.Context, _, _ string) ([]string, error) {
	return []string{"sug-old-1"}, nil
}

// --- 测试用例 ---

// TestSuggestionSubmit_Success 验证正常提交建议的完整流程。
func TestSuggestionSubmit_Success(t *testing.T) {
	repo := newMockSuggestionRepo()
	ai := &mockSuggestionAI{}
	uc := NewSuggestionUsecase(repo, ai, nil)

	s, err := uc.Submit(context.Background(), "优化查询性能", "建议增加缓存层减少数据库查询", "user-001")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.ID == "" {
		t.Error("expected non-empty ID")
	}
	if s.Status != SuggestionStatusPending {
		t.Errorf("expected status=pending, got %s", s.Status)
	}
	if s.Category != "功能" {
		t.Errorf("expected category=功能, got %s", s.Category)
	}
	if len(s.Keywords) != 2 {
		t.Errorf("expected 2 keywords, got %d", len(s.Keywords))
	}
	if s.Sentiment != "positive" {
		t.Errorf("expected sentiment=positive, got %s", s.Sentiment)
	}
	if len(s.SimilarIDs) != 1 {
		t.Errorf("expected 1 similar ID, got %d", len(s.SimilarIDs))
	}
}

// TestSuggestionSubmit_ValidationErrors 验证提交建议时的参数校验。
func TestSuggestionSubmit_ValidationErrors(t *testing.T) {
	repo := newMockSuggestionRepo()
	uc := NewSuggestionUsecase(repo, nil, nil)

	tests := []struct {
		name        string
		title       string
		description string
		userID      string
		wantErr     string
	}{
		{"空标题", "", "描述", "user-1", "title is required"},
		{"空描述", "标题", "", "user-1", "description is required"},
		{"空用户", "标题", "描述", "", "user_id is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Submit(context.Background(), tt.title, tt.description, tt.userID)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error=%q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

// TestSuggestionSubmit_WithoutAI 验证无 AI 服务时建议仍可正常提交。
func TestSuggestionSubmit_WithoutAI(t *testing.T) {
	repo := newMockSuggestionRepo()
	uc := NewSuggestionUsecase(repo, nil, nil) // 不注入 AI 服务

	s, err := uc.Submit(context.Background(), "简单建议", "没有 AI 分析的建议", "user-002")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s.Category != "" {
		t.Errorf("expected empty category without AI, got %s", s.Category)
	}
	if s.Sentiment != "" {
		t.Errorf("expected empty sentiment without AI, got %s", s.Sentiment)
	}
}

// TestSuggestionList_StatusFilter 验证按状态筛选建议列表。
func TestSuggestionList_StatusFilter(t *testing.T) {
	repo := newMockSuggestionRepo()
	uc := NewSuggestionUsecase(repo, nil, nil)

	// 创建多条不同状态的建议
	ctx := context.Background()
	uc.Submit(ctx, "建议1", "描述1", "user-1")
	uc.Submit(ctx, "建议2", "描述2", "user-1")
	// 将第一条改为 accepted
	repo.UpdateStatus(ctx, "sug-1", SuggestionStatusAccepted, "已采纳")

	// 查询 pending 状态
	list, total, err := uc.List(ctx, "pending", 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1 for pending, got %d", total)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 pending suggestion, got %d", len(list))
	}

	// 查询无效状态
	_, _, err = uc.List(ctx, "invalid_status", 1, 10)
	if err == nil {
		t.Error("expected error for invalid status, got nil")
	}
}

// TestSuggestionUpdateStatus 验证状态更新逻辑。
func TestSuggestionUpdateStatus(t *testing.T) {
	repo := newMockSuggestionRepo()
	uc := NewSuggestionUsecase(repo, nil, nil)

	ctx := context.Background()
	uc.Submit(ctx, "待审核建议", "描述", "user-1")

	// 正常更新
	err := uc.UpdateStatus(ctx, "sug-1", "accepted", "已审核通过")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	s, _ := repo.Get(ctx, "sug-1")
	if s.Status != SuggestionStatusAccepted {
		t.Errorf("expected status=accepted, got %s", s.Status)
	}
	if s.AdminNote != "已审核通过" {
		t.Errorf("expected admin_note=已审核通过, got %s", s.AdminNote)
	}

	// 无效状态
	err = uc.UpdateStatus(ctx, "sug-1", "invalid", "")
	if err == nil {
		t.Error("expected error for invalid status, got nil")
	}
}

// TestSuggestionGetStats 验证统计数据计算。
func TestSuggestionGetStats(t *testing.T) {
	repo := newMockSuggestionRepo()
	uc := NewSuggestionUsecase(repo, nil, nil)

	ctx := context.Background()
	uc.Submit(ctx, "建议1", "描述1", "user-1")
	uc.Submit(ctx, "建议2", "描述2", "user-1")
	uc.Submit(ctx, "建议3", "描述3", "user-1")
	repo.UpdateStatus(ctx, "sug-1", SuggestionStatusAccepted, "")
	repo.UpdateStatus(ctx, "sug-2", SuggestionStatusRejected, "")

	stats, err := uc.GetStats(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("expected total=3, got %d", stats.Total)
	}
	if stats.Accepted != 1 {
		t.Errorf("expected accepted=1, got %d", stats.Accepted)
	}
	if stats.Rejected != 1 {
		t.Errorf("expected rejected=1, got %d", stats.Rejected)
	}
	if stats.Pending != 1 {
		t.Errorf("expected pending=1, got %d", stats.Pending)
	}
	// 采纳率 = (1+0+0)/3 * 100 = 33.33%
	expectedRate := float64(1) / float64(3) * 100
	if stats.AcceptanceRate != expectedRate {
		t.Errorf("expected acceptance_rate=%.2f, got %.2f", expectedRate, stats.AcceptanceRate)
	}
}
