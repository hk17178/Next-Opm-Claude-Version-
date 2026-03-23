package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Mock ES repo for search tests ---

// searchMockESRepo 是用于搜索测试的 ES 仓库 mock 实现。
// BUG-004 修复：原始实现的 BulkIndex 方法未记录调用信息（只返回 nil），
// 导致依赖 bulkIndexCalls/indexed 字段的测试断言无法正确工作。
// 修复方案：在 BulkIndex 中追加条目到 indexed 切片并递增计数器，使测试可验证写入行为。
type searchMockESRepo struct {
	lastSearchReq  LogSearchRequest
	lastStatsReq   LogStatsRequest
	searchResp     *LogSearchResponse
	statsResp      *LogStatsResponse
	searchErr      error
	statsErr       error
	indexed        []LogEntry // BUG-004 修复：记录所有被 BulkIndex 写入的日志条目
	bulkIndexCalls int        // BUG-004 修复：记录 BulkIndex 被调用的总次数
}

// BulkIndex 批量索引日志条目到 Elasticsearch（mock 实现）。
// BUG-004 修复：原始实现未记录调用信息，现在将所有写入的条目追加到 indexed 切片，
// 并递增 bulkIndexCalls 计数器，以便测试断言可以验证批量写入行为。
func (m *searchMockESRepo) BulkIndex(ctx context.Context, entries []LogEntry) error {
	// BUG-004 修复：记录被写入的条目和调用次数，支持测试断言
	m.indexed = append(m.indexed, entries...)
	m.bulkIndexCalls++
	return nil
}

func (m *searchMockESRepo) Search(ctx context.Context, req LogSearchRequest) (*LogSearchResponse, error) {
	m.lastSearchReq = req
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	if m.searchResp != nil {
		return m.searchResp, nil
	}
	return &LogSearchResponse{Total: 0, Entries: nil}, nil
}

func (m *searchMockESRepo) Aggregate(ctx context.Context, req LogStatsRequest) (*LogStatsResponse, error) {
	m.lastStatsReq = req
	if m.statsErr != nil {
		return nil, m.statsErr
	}
	if m.statsResp != nil {
		return m.statsResp, nil
	}
	return &LogStatsResponse{Total: 0}, nil
}

func (m *searchMockESRepo) ListIndices(ctx context.Context, pattern string) ([]IndexInfo, error) {
	return nil, nil
}

func (m *searchMockESRepo) DeleteIndex(ctx context.Context, indexName string) error { return nil }

func newTestSearchService(esRepo *searchMockESRepo) *SearchService {
	logRepo := &mockLogRepo{}
	logger := zap.NewNop()
	return NewSearchService(esRepo, logRepo, logger)
}

// --- Tests ---

func TestSearch_DefaultPageSize(t *testing.T) {
	es := &searchMockESRepo{}
	svc := newTestSearchService(es)

	svc.Search(context.Background(), LogSearchRequest{Query: "error"})

	if es.lastSearchReq.PageSize != 50 {
		t.Errorf("expected default page_size 50, got %d", es.lastSearchReq.PageSize)
	}
}

func TestSearch_CapsPageSize(t *testing.T) {
	es := &searchMockESRepo{}
	svc := newTestSearchService(es)

	svc.Search(context.Background(), LogSearchRequest{Query: "*", PageSize: 99999})

	if es.lastSearchReq.PageSize != 10000 {
		t.Errorf("expected capped page_size 10000, got %d", es.lastSearchReq.PageSize)
	}
}

func TestSearch_DefaultSortDesc(t *testing.T) {
	es := &searchMockESRepo{}
	svc := newTestSearchService(es)

	svc.Search(context.Background(), LogSearchRequest{Query: "*"})

	if es.lastSearchReq.Sort != "desc" {
		t.Errorf("expected default sort desc, got %s", es.lastSearchReq.Sort)
	}
}

func TestSearch_PreservesSort(t *testing.T) {
	es := &searchMockESRepo{}
	svc := newTestSearchService(es)

	svc.Search(context.Background(), LogSearchRequest{Query: "*", Sort: "asc"})

	if es.lastSearchReq.Sort != "asc" {
		t.Errorf("expected sort asc, got %s", es.lastSearchReq.Sort)
	}
}

func TestSearch_PassesThroughTimeRange(t *testing.T) {
	es := &searchMockESRepo{}
	svc := newTestSearchService(es)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	svc.Search(context.Background(), LogSearchRequest{
		Query:     "error",
		TimeRange: &TimeRange{Start: &start, End: &end},
	})

	if es.lastSearchReq.TimeRange == nil {
		t.Fatal("expected time_range to be passed through")
	}
	if !es.lastSearchReq.TimeRange.Start.Equal(start) {
		t.Errorf("expected start %v, got %v", start, es.lastSearchReq.TimeRange.Start)
	}
}

func TestSearch_PassesThroughFilters(t *testing.T) {
	es := &searchMockESRepo{}
	svc := newTestSearchService(es)

	svc.Search(context.Background(), LogSearchRequest{
		Query:   "*",
		Filters: map[string]string{"source_host": "server-01", "level": "ERROR"},
	})

	if len(es.lastSearchReq.Filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(es.lastSearchReq.Filters))
	}
	if es.lastSearchReq.Filters["source_host"] != "server-01" {
		t.Errorf("expected source_host filter, got %v", es.lastSearchReq.Filters)
	}
}

func TestSearch_ReturnsError(t *testing.T) {
	es := &searchMockESRepo{
		searchErr: fmt.Errorf("es unavailable"),
	}
	svc := newTestSearchService(es)

	_, err := svc.Search(context.Background(), LogSearchRequest{Query: "*"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestSearch_ReturnsResults(t *testing.T) {
	es := &searchMockESRepo{
		searchResp: &LogSearchResponse{
			Total: 2,
			Entries: []LogEntry{
				{ID: "1", Message: "log1"},
				{ID: "2", Message: "log2"},
			},
			NextPageToken: "token123",
		},
	}
	svc := newTestSearchService(es)

	resp, err := svc.Search(context.Background(), LogSearchRequest{Query: "*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Total)
	}
	if len(resp.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(resp.Entries))
	}
	if resp.NextPageToken != "token123" {
		t.Errorf("expected next_page_token, got %s", resp.NextPageToken)
	}
}

func TestContext_EntryNotFound(t *testing.T) {
	es := &searchMockESRepo{
		searchResp: &LogSearchResponse{Total: 0, Entries: nil},
	}
	svc := newTestSearchService(es)

	_, err := svc.Context(context.Background(), "nonexistent", 10, 10)
	if err == nil {
		t.Error("expected error for non-existent entry")
	}
}

func TestContext_BuildsContextQuery(t *testing.T) {
	now := time.Now().UTC()
	es := &searchMockESRepo{}
	svc := newTestSearchService(es)

	// First call returns the target entry, second call returns context
	es.searchResp = nil

	// Override to handle two calls
	targetEntry := LogEntry{
		ID:            "target-1",
		Timestamp:     now,
		SourceHost:    "host-1",
		SourceService: "svc-1",
		Message:       "target log",
	}

	// For Context, the first Search finds the target, second gets surrounding.
	// We simulate by setting searchResp for first call.
	es.searchResp = &LogSearchResponse{
		Total:   1,
		Entries: []LogEntry{targetEntry},
	}

	_, err := svc.Context(context.Background(), "target-1", 10, 10)
	// The second internal search will use the default (empty) response from mock.
	// We only verify it didn't error fatally.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the second search had the right filters
	if es.lastSearchReq.Sort != "asc" {
		t.Errorf("context query should sort asc, got %s", es.lastSearchReq.Sort)
	}
	if es.lastSearchReq.PageSize != 21 { // 10+10+1
		t.Errorf("expected page_size 21, got %d", es.lastSearchReq.PageSize)
	}
}

func TestStats_RequiresGroupBy(t *testing.T) {
	es := &searchMockESRepo{}
	svc := newTestSearchService(es)

	_, err := svc.Stats(context.Background(), LogStatsRequest{})
	if err == nil {
		t.Error("expected error for empty group_by")
	}
}

func TestStats_PassesRequest(t *testing.T) {
	es := &searchMockESRepo{
		statsResp: &LogStatsResponse{
			Total: 100,
			Buckets: []StatsBucket{
				{Key: "ERROR", DocCount: 50},
				{Key: "INFO", DocCount: 50},
			},
		},
	}
	svc := newTestSearchService(es)

	resp, err := svc.Stats(context.Background(), LogStatsRequest{
		GroupBy:  "level",
		Interval: "1h",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 100 {
		t.Errorf("expected total 100, got %d", resp.Total)
	}
	if len(resp.Buckets) != 2 {
		t.Errorf("expected 2 buckets, got %d", len(resp.Buckets))
	}
	if es.lastStatsReq.GroupBy != "level" {
		t.Errorf("expected group_by level, got %s", es.lastStatsReq.GroupBy)
	}
}

func TestExport_ReturnsPending(t *testing.T) {
	es := &searchMockESRepo{}
	svc := newTestSearchService(es)

	resp, err := svc.Export(context.Background(), LogExportRequest{
		Query:  "error",
		Format: "csv",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "pending" {
		t.Errorf("expected status pending, got %s", resp.Status)
	}
	if resp.TaskID == "" {
		t.Error("expected non-empty task_id")
	}
}

// --- Specifically requested tests ---

// TestBuildTimeRangeFilter verifies time range filter construction for search queries.
func TestBuildTimeRangeFilter(t *testing.T) {
	es := &searchMockESRepo{}
	svc := newTestSearchService(es)

	t.Run("both start and end set", func(t *testing.T) {
		start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

		svc.Search(context.Background(), LogSearchRequest{
			Query:     "error",
			TimeRange: &TimeRange{Start: &start, End: &end},
		})

		if es.lastSearchReq.TimeRange == nil {
			t.Fatal("expected time_range to be set")
		}
		if !es.lastSearchReq.TimeRange.Start.Equal(start) {
			t.Errorf("expected start %v, got %v", start, *es.lastSearchReq.TimeRange.Start)
		}
		if !es.lastSearchReq.TimeRange.End.Equal(end) {
			t.Errorf("expected end %v, got %v", end, *es.lastSearchReq.TimeRange.End)
		}
	})

	t.Run("only start set", func(t *testing.T) {
		start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

		svc.Search(context.Background(), LogSearchRequest{
			Query:     "*",
			TimeRange: &TimeRange{Start: &start},
		})

		if es.lastSearchReq.TimeRange == nil {
			t.Fatal("expected time_range to be set")
		}
		if !es.lastSearchReq.TimeRange.Start.Equal(start) {
			t.Errorf("expected start %v, got %v", start, *es.lastSearchReq.TimeRange.Start)
		}
		if es.lastSearchReq.TimeRange.End != nil {
			t.Error("expected end to be nil")
		}
	})

	t.Run("only end set", func(t *testing.T) {
		end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

		svc.Search(context.Background(), LogSearchRequest{
			Query:     "*",
			TimeRange: &TimeRange{End: &end},
		})

		if es.lastSearchReq.TimeRange == nil {
			t.Fatal("expected time_range to be set")
		}
		if es.lastSearchReq.TimeRange.Start != nil {
			t.Error("expected start to be nil")
		}
		if !es.lastSearchReq.TimeRange.End.Equal(end) {
			t.Errorf("expected end %v, got %v", end, *es.lastSearchReq.TimeRange.End)
		}
	})

	t.Run("no time range", func(t *testing.T) {
		svc.Search(context.Background(), LogSearchRequest{Query: "*"})

		if es.lastSearchReq.TimeRange != nil {
			t.Error("expected nil time_range when not set")
		}
	})

	t.Run("time range combined with filters", func(t *testing.T) {
		start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)

		svc.Search(context.Background(), LogSearchRequest{
			Query:     "error",
			TimeRange: &TimeRange{Start: &start, End: &end},
			Filters:   map[string]string{"level": "ERROR", "source_host": "server-01"},
		})

		if es.lastSearchReq.TimeRange == nil {
			t.Fatal("expected time_range to be set")
		}
		if len(es.lastSearchReq.Filters) != 2 {
			t.Errorf("expected 2 filters, got %d", len(es.lastSearchReq.Filters))
		}
	})
}

// TestSearchPagination verifies next_page_token pagination logic.
func TestSearchPagination(t *testing.T) {
	t.Run("first page returns next_page_token", func(t *testing.T) {
		es := &searchMockESRepo{
			searchResp: &LogSearchResponse{
				Total: 100,
				Entries: []LogEntry{
					{ID: "1", Message: "log1"},
					{ID: "2", Message: "log2"},
				},
				NextPageToken: "eyJzb3J0IjpbMTcwOTI4MDAwMDAwMCwiMiJdfQ==",
			},
		}
		svc := newTestSearchService(es)

		resp, err := svc.Search(context.Background(), LogSearchRequest{
			Query:    "*",
			PageSize: 2,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Total != 100 {
			t.Errorf("expected total 100, got %d", resp.Total)
		}
		if len(resp.Entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(resp.Entries))
		}
		if resp.NextPageToken == "" {
			t.Error("expected non-empty next_page_token for first page")
		}
	})

	t.Run("second page uses page_token", func(t *testing.T) {
		es := &searchMockESRepo{
			searchResp: &LogSearchResponse{
				Total: 100,
				Entries: []LogEntry{
					{ID: "3", Message: "log3"},
					{ID: "4", Message: "log4"},
				},
				NextPageToken: "eyJzb3J0IjpbMTcwOTI4MDAwMDAwMCwiNCJdfQ==",
			},
		}
		svc := newTestSearchService(es)

		pageToken := "eyJzb3J0IjpbMTcwOTI4MDAwMDAwMCwiMiJdfQ=="
		resp, err := svc.Search(context.Background(), LogSearchRequest{
			Query:     "*",
			PageSize:  2,
			PageToken: pageToken,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Verify page_token was passed through to ES
		if es.lastSearchReq.PageToken != pageToken {
			t.Errorf("expected page_token %s, got %s", pageToken, es.lastSearchReq.PageToken)
		}
		if len(resp.Entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(resp.Entries))
		}
		if resp.Entries[0].ID != "3" {
			t.Errorf("expected first entry ID '3', got '%s'", resp.Entries[0].ID)
		}
	})

	t.Run("last page returns empty next_page_token", func(t *testing.T) {
		es := &searchMockESRepo{
			searchResp: &LogSearchResponse{
				Total: 3,
				Entries: []LogEntry{
					{ID: "3", Message: "log3"},
				},
				NextPageToken: "", // last page
			},
		}
		svc := newTestSearchService(es)

		resp, err := svc.Search(context.Background(), LogSearchRequest{
			Query:     "*",
			PageSize:  2,
			PageToken: "some-token",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.NextPageToken != "" {
			t.Errorf("expected empty next_page_token for last page, got %s", resp.NextPageToken)
		}
	})

	t.Run("default page size applied", func(t *testing.T) {
		es := &searchMockESRepo{}
		svc := newTestSearchService(es)

		svc.Search(context.Background(), LogSearchRequest{Query: "*"})

		if es.lastSearchReq.PageSize != 50 {
			t.Errorf("expected default page_size 50, got %d", es.lastSearchReq.PageSize)
		}
	})

	t.Run("page size capped at 10000", func(t *testing.T) {
		es := &searchMockESRepo{}
		svc := newTestSearchService(es)

		svc.Search(context.Background(), LogSearchRequest{Query: "*", PageSize: 50000})

		if es.lastSearchReq.PageSize != 10000 {
			t.Errorf("expected capped page_size 10000, got %d", es.lastSearchReq.PageSize)
		}
	})
}

// --- Testify-based tests ---

func TestSearch_Testify(t *testing.T) {
	t.Run("lucene query with results", func(t *testing.T) {
		es := &searchMockESRepo{
			searchResp: &LogSearchResponse{
				Total: 3,
				Entries: []LogEntry{
					{ID: "1", Message: "auth error 1", Level: LogLevelError},
					{ID: "2", Message: "auth error 2", Level: LogLevelError},
					{ID: "3", Message: "auth error 3", Level: LogLevelError},
				},
			},
		}
		svc := newTestSearchService(es)
		resp, err := svc.Search(context.Background(), LogSearchRequest{
			Query: "level:ERROR AND service:auth",
		})
		require.NoError(t, err)
		assert.Equal(t, int64(3), resp.Total)
		assert.Len(t, resp.Entries, 3)
	})
}

func TestSearchWithTimeRange_Testify(t *testing.T) {
	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now

	t.Run("passes time range to ES", func(t *testing.T) {
		es := &searchMockESRepo{
			searchResp: &LogSearchResponse{Total: 1, Entries: []LogEntry{{ID: "t1"}}},
		}
		svc := newTestSearchService(es)
		resp, err := svc.Search(context.Background(), LogSearchRequest{
			Query: "*",
			TimeRange: &TimeRange{
				Start: &start,
				End:   &end,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), resp.Total)
		require.NotNil(t, es.lastSearchReq.TimeRange)
	})
}

func TestSearchAggregation_Testify(t *testing.T) {
	t.Run("aggregation by level", func(t *testing.T) {
		es := &searchMockESRepo{
			statsResp: &LogStatsResponse{
				Total: 100,
				Buckets: []StatsBucket{
					{Key: "ERROR", DocCount: 30},
					{Key: "WARN", DocCount: 25},
					{Key: "INFO", DocCount: 45},
				},
			},
		}
		svc := newTestSearchService(es)
		resp, err := svc.Stats(context.Background(), LogStatsRequest{GroupBy: "level"})
		require.NoError(t, err)
		assert.Equal(t, int64(100), resp.Total)
		assert.Len(t, resp.Buckets, 3)
	})

	t.Run("aggregation requires group_by", func(t *testing.T) {
		es := &searchMockESRepo{}
		svc := newTestSearchService(es)
		_, err := svc.Stats(context.Background(), LogStatsRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "group_by")
	})
}
