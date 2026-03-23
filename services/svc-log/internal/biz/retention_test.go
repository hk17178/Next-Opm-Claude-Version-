package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Mock ES repo with index management ---

type retentionMockESRepo struct {
	indices        []IndexInfo
	deletedIndices []string
}

func (m *retentionMockESRepo) BulkIndex(ctx context.Context, entries []LogEntry) error {
	return nil
}
func (m *retentionMockESRepo) Search(ctx context.Context, req LogSearchRequest) (*LogSearchResponse, error) {
	return &LogSearchResponse{}, nil
}
func (m *retentionMockESRepo) Aggregate(ctx context.Context, req LogStatsRequest) (*LogStatsResponse, error) {
	return &LogStatsResponse{}, nil
}
func (m *retentionMockESRepo) ListIndices(ctx context.Context, pattern string) ([]IndexInfo, error) {
	return m.indices, nil
}
func (m *retentionMockESRepo) DeleteIndex(ctx context.Context, indexName string) error {
	m.deletedIndices = append(m.deletedIndices, indexName)
	return nil
}

// --- Mock log repo for retention policies ---

type retentionMockLogRepo struct {
	mockLogRepo
	policies []*RetentionPolicy
}

func (m *retentionMockLogRepo) ListRetentionPolicies(ctx context.Context) ([]*RetentionPolicy, error) {
	return m.policies, nil
}

// --- Tests ---

func TestRetentionExecutor_DeletesExpiredIndices(t *testing.T) {
	now := time.Now()
	esRepo := &retentionMockESRepo{
		indices: []IndexInfo{
			{Name: "opsnexus-log-" + now.AddDate(0, 0, -10).Format("2006.01.02"), CreatedAt: now.AddDate(0, 0, -10)},
			{Name: "opsnexus-log-" + now.AddDate(0, 0, -5).Format("2006.01.02"), CreatedAt: now.AddDate(0, 0, -5)},
			{Name: "opsnexus-log-" + now.AddDate(0, 0, -1).Format("2006.01.02"), CreatedAt: now.AddDate(0, 0, -1)},
		},
	}
	logRepo := &retentionMockLogRepo{
		policies: []*RetentionPolicy{
			{PolicyID: "p1", Name: "default", HotDays: 3, WarmDays: 3, ColdDays: 1, Enabled: true},
		},
	}

	executor := NewRetentionExecutor(logRepo, esRepo, zap.NewNop(), 1*time.Hour, "opsnexus-log")
	executor.execute()

	// Total retention = 3+3+1 = 7 days. Index at -10 days should be deleted.
	require.Len(t, esRepo.deletedIndices, 1)
	assert.Contains(t, esRepo.deletedIndices[0], now.AddDate(0, 0, -10).Format("2006.01.02"))
}

func TestRetentionExecutor_NoPolicies_NoDeletes(t *testing.T) {
	esRepo := &retentionMockESRepo{
		indices: []IndexInfo{
			{Name: "opsnexus-log-2020.01.01", CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
	}
	logRepo := &retentionMockLogRepo{policies: nil}

	executor := NewRetentionExecutor(logRepo, esRepo, zap.NewNop(), 1*time.Hour, "opsnexus-log")
	executor.execute()

	assert.Empty(t, esRepo.deletedIndices)
}

func TestRetentionExecutor_DisabledPoliciesIgnored(t *testing.T) {
	now := time.Now()
	esRepo := &retentionMockESRepo{
		indices: []IndexInfo{
			{Name: "opsnexus-log-old", CreatedAt: now.AddDate(-2, 0, 0)},
		},
	}
	logRepo := &retentionMockLogRepo{
		policies: []*RetentionPolicy{
			{PolicyID: "p1", Name: "disabled", HotDays: 1, WarmDays: 1, ColdDays: 1, Enabled: false},
		},
	}

	executor := NewRetentionExecutor(logRepo, esRepo, zap.NewNop(), 1*time.Hour, "opsnexus-log")
	executor.execute()

	// All policies disabled -> fallback to 365 days. -2 years old index should be deleted.
	require.Len(t, esRepo.deletedIndices, 1)
}

func TestRetentionExecutor_RecentIndicesPreserved(t *testing.T) {
	now := time.Now()
	esRepo := &retentionMockESRepo{
		indices: []IndexInfo{
			{Name: "opsnexus-log-today", CreatedAt: now},
			{Name: "opsnexus-log-yesterday", CreatedAt: now.AddDate(0, 0, -1)},
		},
	}
	logRepo := &retentionMockLogRepo{
		policies: []*RetentionPolicy{
			{PolicyID: "p1", Name: "standard", HotDays: 7, WarmDays: 30, ColdDays: 365, Enabled: true},
		},
	}

	executor := NewRetentionExecutor(logRepo, esRepo, zap.NewNop(), 1*time.Hour, "opsnexus-log")
	executor.execute()

	assert.Empty(t, esRepo.deletedIndices)
}

func TestRetentionExecutor_MaxRetentionDays(t *testing.T) {
	logRepo := &retentionMockLogRepo{
		policies: []*RetentionPolicy{
			{PolicyID: "p1", HotDays: 7, WarmDays: 30, ColdDays: 365, Enabled: true},
			{PolicyID: "p2", HotDays: 3, WarmDays: 7, ColdDays: 30, Enabled: true},
		},
	}

	executor := NewRetentionExecutor(logRepo, nil, zap.NewNop(), 1*time.Hour, "opsnexus-log")
	maxDays := executor.getMaxRetentionDays(logRepo.policies)

	// Max should be 7+30+365=402
	assert.Equal(t, 402, maxDays)
}

func TestRetentionExecutor_StopGracefully(t *testing.T) {
	logRepo := &retentionMockLogRepo{}
	esRepo := &retentionMockESRepo{}

	executor := NewRetentionExecutor(logRepo, esRepo, zap.NewNop(), 100*time.Millisecond, "opsnexus-log")
	executor.Start()
	time.Sleep(50 * time.Millisecond)
	executor.Stop()

	// Should not panic or hang
	executor.Stop() // idempotent
}
