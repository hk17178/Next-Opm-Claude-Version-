package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Mock ES repo for export tests ---

type exportMockESRepo struct {
	searchCalls int
	pages       []*LogSearchResponse
}

func (m *exportMockESRepo) BulkIndex(ctx context.Context, entries []LogEntry) error { return nil }
func (m *exportMockESRepo) Search(ctx context.Context, req LogSearchRequest) (*LogSearchResponse, error) {
	if m.searchCalls < len(m.pages) {
		resp := m.pages[m.searchCalls]
		m.searchCalls++
		return resp, nil
	}
	return &LogSearchResponse{}, nil
}
func (m *exportMockESRepo) Aggregate(ctx context.Context, req LogStatsRequest) (*LogStatsResponse, error) {
	return &LogStatsResponse{}, nil
}
func (m *exportMockESRepo) ListIndices(ctx context.Context, pattern string) ([]IndexInfo, error) {
	return nil, nil
}
func (m *exportMockESRepo) DeleteIndex(ctx context.Context, indexName string) error { return nil }

func newTestSearchServiceWithESRepo(esRepo ESRepository) *SearchService {
	return NewSearchService(esRepo, &mockLogRepo{}, zap.NewNop())
}

func TestExportStream_JSON(t *testing.T) {
	now := time.Now()
	esRepo := &exportMockESRepo{
		pages: []*LogSearchResponse{
			{
				Total: 2,
				Entries: []LogEntry{
					{ID: "1", Message: "log1", Timestamp: now},
					{ID: "2", Message: "log2", Timestamp: now},
				},
				NextPageToken: "",
			},
		},
	}
	svc := newTestSearchServiceWithESRepo(esRepo)

	var exported []LogEntry
	err := svc.ExportStream(context.Background(), LogExportRequest{
		Query:  "*",
		Format: "json",
	}, func(entry LogEntry) error {
		exported = append(exported, entry)
		return nil
	})

	require.NoError(t, err)
	assert.Len(t, exported, 2)
	assert.Equal(t, "log1", exported[0].Message)
	assert.Equal(t, "log2", exported[1].Message)
}

func TestExportStream_CSV(t *testing.T) {
	esRepo := &exportMockESRepo{
		pages: []*LogSearchResponse{
			{
				Total: 1,
				Entries: []LogEntry{
					{ID: "1", Message: "csv test", Level: LogLevelInfo},
				},
			},
		},
	}
	svc := newTestSearchServiceWithESRepo(esRepo)

	var exported []LogEntry
	err := svc.ExportStream(context.Background(), LogExportRequest{
		Query:  "*",
		Format: "csv",
	}, func(entry LogEntry) error {
		exported = append(exported, entry)
		return nil
	})

	require.NoError(t, err)
	assert.Len(t, exported, 1)
}

func TestExportStream_Pagination(t *testing.T) {
	esRepo := &exportMockESRepo{
		pages: []*LogSearchResponse{
			{
				Total: 3,
				Entries: []LogEntry{
					{ID: "1", Message: "page1-1"},
					{ID: "2", Message: "page1-2"},
				},
				NextPageToken: "token1",
			},
			{
				Total: 3,
				Entries: []LogEntry{
					{ID: "3", Message: "page2-1"},
				},
				NextPageToken: "",
			},
		},
	}
	svc := newTestSearchServiceWithESRepo(esRepo)

	var exported []LogEntry
	err := svc.ExportStream(context.Background(), LogExportRequest{
		Query:  "*",
		Format: "json",
	}, func(entry LogEntry) error {
		exported = append(exported, entry)
		return nil
	})

	require.NoError(t, err)
	assert.Len(t, exported, 3)
	assert.Equal(t, 2, esRepo.searchCalls)
}

func TestExportStream_UnsupportedFormat(t *testing.T) {
	esRepo := &exportMockESRepo{}
	svc := newTestSearchServiceWithESRepo(esRepo)

	err := svc.ExportStream(context.Background(), LogExportRequest{
		Query:  "*",
		Format: "xml",
	}, func(entry LogEntry) error {
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported export format")
}

func TestExportStream_EmptyResult(t *testing.T) {
	esRepo := &exportMockESRepo{
		pages: []*LogSearchResponse{
			{Total: 0, Entries: nil},
		},
	}
	svc := newTestSearchServiceWithESRepo(esRepo)

	var exported []LogEntry
	err := svc.ExportStream(context.Background(), LogExportRequest{
		Query:  "nonexistent",
		Format: "json",
	}, func(entry LogEntry) error {
		exported = append(exported, entry)
		return nil
	})

	require.NoError(t, err)
	assert.Empty(t, exported)
}

func TestExportStream_WithFilters(t *testing.T) {
	esRepo := &exportMockESRepo{
		pages: []*LogSearchResponse{
			{
				Total:   1,
				Entries: []LogEntry{{ID: "1", Message: "filtered", Level: LogLevelError}},
			},
		},
	}
	svc := newTestSearchServiceWithESRepo(esRepo)

	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	var exported []LogEntry
	err := svc.ExportStream(context.Background(), LogExportRequest{
		Query:     "error",
		Format:    "json",
		TimeRange: &TimeRange{Start: &start, End: &end},
		Filters:   map[string]string{"level": "ERROR"},
	}, func(entry LogEntry) error {
		exported = append(exported, entry)
		return nil
	})

	require.NoError(t, err)
	assert.Len(t, exported, 1)
}
