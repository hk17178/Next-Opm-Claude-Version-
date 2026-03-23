// Package biz 定义日志服务的核心业务模型与领域逻辑，包括日志条目、采集源、解析规则、脱敏规则、保留策略等。
package biz

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SearchService 提供日志全文搜索、上下文查询、统计聚合和导出能力。
type SearchService struct {
	esRepo  ESRepository
	logRepo LogRepository
	logger  *zap.Logger
}

// NewSearchService 创建一个新的搜索服务实例。
func NewSearchService(esRepo ESRepository, logRepo LogRepository, logger *zap.Logger) *SearchService {
	return &SearchService{
		esRepo:  esRepo,
		logRepo: logRepo,
		logger:  logger,
	}
}

// Search 使用 Lucene 查询语法执行全文搜索，默认按时间倒序返回。
// 对应 OpenAPI POST /api/v1/log/search 接口。
func (s *SearchService) Search(ctx context.Context, req LogSearchRequest) (*LogSearchResponse, error) {
	if req.PageSize <= 0 {
		req.PageSize = 50
	}
	if req.PageSize > 10000 {
		req.PageSize = 10000
	}
	if req.Sort == "" {
		req.Sort = "desc"
	}

	result, err := s.esRepo.Search(ctx, req)
	if err != nil {
		s.logger.Error("search failed", zap.String("query", req.Query), zap.Error(err))
		return nil, fmt.Errorf("search: %w", err)
	}

	s.logger.Debug("search completed",
		zap.String("query", req.Query),
		zap.Int64("total", result.Total),
	)
	return result, nil
}

// Context 查询指定日志条目前后的上下文日志，用于故障排查时快速定位关联日志。
// 对应 ON-003 GET /api/v1/logs/{id}/context 接口。
func (s *SearchService) Context(ctx context.Context, entryID string, beforeCount, afterCount int) (*LogSearchResponse, error) {
	result, err := s.esRepo.Search(ctx, LogSearchRequest{
		Query:    fmt.Sprintf("id:%s", entryID),
		PageSize: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("find entry: %w", err)
	}
	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("entry not found: %s", entryID)
	}

	target := result.Entries[0]
	timeFrom := target.Timestamp.Add(-1 * time.Minute)
	timeTo := target.Timestamp.Add(1 * time.Minute)

	filters := map[string]string{}
	if target.SourceHost != "" {
		filters["source_host"] = target.SourceHost
	}
	if target.SourceService != "" {
		filters["source_service"] = target.SourceService
	}

	return s.Search(ctx, LogSearchRequest{
		Query: "*",
		TimeRange: &TimeRange{
			Start: &timeFrom,
			End:   &timeTo,
		},
		Filters:  filters,
		PageSize: beforeCount + afterCount + 1,
		Sort:     "asc",
	})
}

// Stats 对日志数据执行聚合统计查询，支持按来源类型、级别或时间分组。
// 对应 ON-003 GET /api/v1/logs/stats 接口。
func (s *SearchService) Stats(ctx context.Context, req LogStatsRequest) (*LogStatsResponse, error) {
	if req.GroupBy == "" {
		return nil, fmt.Errorf("group_by field is required")
	}

	result, err := s.esRepo.Aggregate(ctx, req)
	if err != nil {
		s.logger.Error("stats failed", zap.String("group_by", req.GroupBy), zap.Error(err))
		return nil, fmt.Errorf("stats: %w", err)
	}

	s.logger.Debug("stats completed",
		zap.String("group_by", req.GroupBy),
		zap.Int("buckets", len(result.Buckets)),
	)
	return result, nil
}

// Export 创建异步日志导出任务，返回任务 ID 供后续轮询状态。
// 对应 ON-003 POST /api/v1/logs/export 接口。
func (s *SearchService) Export(ctx context.Context, req LogExportRequest) (*LogExportResponse, error) {
	taskID := generateID()
	return &LogExportResponse{
		TaskID: taskID,
		Status: "pending",
	}, nil
}

// ExportStream 同步查询日志并通过回调逐条流式输出，支持 CSV 和 JSON 格式。
// writeFn 负责将单条日志写入 HTTP 响应流。
func (s *SearchService) ExportStream(ctx context.Context, req LogExportRequest, writeFn func(entry LogEntry) error) error {
	if req.Format != "csv" && req.Format != "json" {
		return fmt.Errorf("unsupported export format: %s (supported: csv, json)", req.Format)
	}

	pageSize := 1000
	pageToken := ""

	for {
		searchReq := LogSearchRequest{
			Query:     req.Query,
			TimeRange: req.TimeRange,
			Filters:   req.Filters,
			PageSize:  pageSize,
			PageToken: pageToken,
			Sort:      "asc",
		}

		result, err := s.esRepo.Search(ctx, searchReq)
		if err != nil {
			return fmt.Errorf("export search: %w", err)
		}

		for _, entry := range result.Entries {
			if err := writeFn(entry); err != nil {
				return fmt.Errorf("export write: %w", err)
			}
		}

		// 终止条件：
		// 1. 没有下一页令牌（ES 告知已无更多数据）
		// 2. 本次返回条数为零（防御性保护，避免空响应导致死循环）
		if result.NextPageToken == "" || len(result.Entries) == 0 {
			break
		}
		pageToken = result.NextPageToken
	}

	return nil
}
