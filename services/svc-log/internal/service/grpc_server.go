// Package service 提供日志服务的传输层实现，包括 gRPC 服务端、HTTP 路由处理和 Kafka 消费者。
package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/opsnexus/svc-log/internal/biz"
	commonpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/common"
	logpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/log"
)

// LogGRPCServer 实现 logpb.LogServiceServer 接口，提供日志服务的 gRPC 端点。
type LogGRPCServer struct {
	logpb.UnimplementedLogServiceServer
	ingestSvc *biz.IngestService
	searchSvc *biz.SearchService
	logger    *zap.Logger
}

// NewLogGRPCServer 创建日志服务的 gRPC 服务端实例。
func NewLogGRPCServer(ingestSvc *biz.IngestService, searchSvc *biz.SearchService, logger *zap.Logger) *LogGRPCServer {
	return &LogGRPCServer{
		ingestSvc: ingestSvc,
		searchSvc: searchSvc,
		logger:    logger,
	}
}

// RegisterLogService 将日志 gRPC 服务注册到 gRPC Server 实例上。
func RegisterLogService(srv *grpc.Server, logSrv *LogGRPCServer) {
	logpb.RegisterLogServiceServer(srv, logSrv)
}

// IngestLog 通过 gRPC 接收一批日志条目并送入摄入管道。
func (s *LogGRPCServer) IngestLog(ctx context.Context, req *logpb.IngestLogRequest) (*logpb.IngestLogResponse, error) {
	bizReq := biz.LogIngestRequest{}
	for _, pe := range req.GetEntries() {
		entry := protoEntryToBiz(pe)
		bizReq.Entries = append(bizReq.Entries, entry)
	}

	result, err := s.ingestSvc.IngestHTTP(ctx, bizReq)
	if err != nil {
		s.logger.Error("gRPC IngestLog failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "ingest failed: %v", err)
	}

	resp := &logpb.IngestLogResponse{
		Accepted: int32(result.Accepted),
		Rejected: int32(result.Rejected),
	}
	for _, e := range result.Errors {
		resp.Errors = append(resp.Errors, &logpb.IngestError{
			Index:  int32(e.Index),
			Reason: e.Reason,
		})
	}

	return resp, nil
}

// SearchLogs 通过 gRPC 执行同步日志搜索。
func (s *LogGRPCServer) SearchLogs(ctx context.Context, req *logpb.SearchLogsRequest) (*logpb.SearchLogsResponse, error) {
	searchReq := biz.LogSearchRequest{
		Query: req.GetQuery(),
		Sort:  req.GetSortOrder(),
	}

	// Map pagination
	if p := req.GetPagination(); p != nil {
		searchReq.PageSize = int(p.GetPageSize())
		searchReq.PageToken = p.GetPageToken()
	}

	// Map time range
	if tr := req.GetTimeRange(); tr != nil {
		searchReq.TimeRange = &biz.TimeRange{}
		if tr.Start != nil {
			t := tr.Start.AsTime()
			searchReq.TimeRange.Start = &t
		}
		if tr.End != nil {
			t := tr.End.AsTime()
			searchReq.TimeRange.End = &t
		}
	}

	// Map filters (repeated KeyValue -> map[string]string)
	if len(req.GetFilters()) > 0 {
		searchReq.Filters = make(map[string]string, len(req.GetFilters()))
		for _, kv := range req.GetFilters() {
			searchReq.Filters[kv.GetKey()] = kv.GetValue()
		}
	}

	result, err := s.searchSvc.Search(ctx, searchReq)
	if err != nil {
		s.logger.Error("gRPC SearchLogs failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	resp := &logpb.SearchLogsResponse{
		Pagination: &commonpb.PageResponse{
			TotalCount:    result.Total,
			NextPageToken: result.NextPageToken,
		},
	}
	for _, e := range result.Entries {
		resp.Entries = append(resp.Entries, bizEntryToProto(&e))
	}

	return resp, nil
}

// GetLogEntry 通过 gRPC 根据 ID 获取单条日志条目。
func (s *LogGRPCServer) GetLogEntry(ctx context.Context, req *logpb.GetLogEntryRequest) (*logpb.LogEntry, error) {
	logID := req.GetLogId()
	if logID == "" {
		return nil, status.Error(codes.InvalidArgument, "log_id is required")
	}

	result, err := s.searchSvc.Search(ctx, biz.LogSearchRequest{
		Query:    fmt.Sprintf("id:%s", logID),
		PageSize: 1,
	})
	if err != nil {
		s.logger.Error("gRPC GetLogEntry failed", zap.String("log_id", logID), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}
	if len(result.Entries) == 0 {
		return nil, status.Errorf(codes.NotFound, "log entry not found: %s", logID)
	}

	return bizEntryToProto(&result.Entries[0]), nil
}

// GetLogContext 通过 gRPC 查询指定日志条目的上下文日志。
func (s *LogGRPCServer) GetLogContext(ctx context.Context, req *logpb.GetLogContextRequest) (*logpb.GetLogContextResponse, error) {
	logID := req.GetLogId()
	if logID == "" {
		return nil, status.Error(codes.InvalidArgument, "log_id is required")
	}

	linesBefore := int(req.GetLinesBefore())
	linesAfter := int(req.GetLinesAfter())
	if linesBefore <= 0 {
		linesBefore = 20
	}
	if linesAfter <= 0 {
		linesAfter = 20
	}

	// First, get the target entry
	targetResult, err := s.searchSvc.Search(ctx, biz.LogSearchRequest{
		Query:    fmt.Sprintf("id:%s", logID),
		PageSize: 1,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}
	if len(targetResult.Entries) == 0 {
		return nil, status.Errorf(codes.NotFound, "log entry not found: %s", logID)
	}

	target := targetResult.Entries[0]

	// Get context entries around the target
	contextResult, err := s.searchSvc.Context(ctx, logID, linesBefore, linesAfter)
	if err != nil {
		s.logger.Error("gRPC GetLogContext failed", zap.String("log_id", logID), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "context query failed: %v", err)
	}

	resp := &logpb.GetLogContextResponse{
		Target: bizEntryToProto(&target),
	}

	// Split context entries into before and after based on target timestamp
	for _, e := range contextResult.Entries {
		if e.ID == target.ID {
			continue
		}
		pe := bizEntryToProto(&e)
		if e.Timestamp.Before(target.Timestamp) {
			resp.Before = append(resp.Before, pe)
		} else {
			resp.After = append(resp.After, pe)
		}
	}

	return resp, nil
}

// GetLogStats 通过 gRPC 执行日志数据的聚合统计查询。
func (s *LogGRPCServer) GetLogStats(ctx context.Context, req *logpb.GetLogStatsRequest) (*logpb.GetLogStatsResponse, error) {
	groupBy := req.GetGroupBy()
	if groupBy == "" {
		return nil, status.Error(codes.InvalidArgument, "group_by is required")
	}

	statsReq := biz.LogStatsRequest{
		GroupBy:  groupBy,
		Interval: req.GetInterval(),
	}

	if tr := req.GetTimeRange(); tr != nil {
		statsReq.TimeRange = &biz.TimeRange{}
		if tr.Start != nil {
			t := tr.Start.AsTime()
			statsReq.TimeRange.Start = &t
		}
		if tr.End != nil {
			t := tr.End.AsTime()
			statsReq.TimeRange.End = &t
		}
	}

	result, err := s.searchSvc.Stats(ctx, statsReq)
	if err != nil {
		s.logger.Error("gRPC GetLogStats failed", zap.String("group_by", groupBy), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "stats failed: %v", err)
	}

	resp := &logpb.GetLogStatsResponse{
		Total: result.Total,
	}
	for _, b := range result.Buckets {
		resp.Buckets = append(resp.Buckets, &logpb.StatsBucket{
			Key:      b.Key,
			DocCount: b.DocCount,
			Value:    b.Value,
		})
	}

	return resp, nil
}

// ExportLogs 通过 gRPC 创建异步日志导出任务。
func (s *LogGRPCServer) ExportLogs(ctx context.Context, req *logpb.ExportLogsRequest) (*logpb.ExportLogsResponse, error) {
	exportReq := biz.LogExportRequest{
		Query:  req.GetQuery(),
		Format: req.GetFormat(),
	}

	if len(req.GetFilters()) > 0 {
		exportReq.Filters = make(map[string]string, len(req.GetFilters()))
		for _, kv := range req.GetFilters() {
			exportReq.Filters[kv.GetKey()] = kv.GetValue()
		}
	}

	if tr := req.GetTimeRange(); tr != nil {
		exportReq.TimeRange = &biz.TimeRange{}
		if tr.Start != nil {
			t := tr.Start.AsTime()
			exportReq.TimeRange.Start = &t
		}
		if tr.End != nil {
			t := tr.End.AsTime()
			exportReq.TimeRange.End = &t
		}
	}

	result, err := s.searchSvc.Export(ctx, exportReq)
	if err != nil {
		s.logger.Error("gRPC ExportLogs failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "export failed: %v", err)
	}

	return &logpb.ExportLogsResponse{
		TaskId: result.TaskID,
		Status: result.Status,
	}, nil
}

// bizEntryToProto 将业务层 biz.LogEntry 转换为 protobuf LogEntry。
func bizEntryToProto(e *biz.LogEntry) *logpb.LogEntry {
	pe := &logpb.LogEntry{
		Id:          e.ID,
		Timestamp:   timestamppb.New(e.Timestamp),
		Level:       string(e.Level),
		Message:     e.Message,
		ServiceName: e.ServiceName,
		HostId:      e.HostID,
		TraceId:     e.TraceID,
		SpanId:      e.SpanID,
	}

	for k, v := range e.Labels {
		pe.Labels = append(pe.Labels, &commonpb.KeyValue{
			Key:   k,
			Value: v,
		})
	}

	return pe
}

// protoEntryToBiz 将 protobuf LogEntry 转换为业务层 biz.LogEntry。
func protoEntryToBiz(pe *logpb.LogEntry) biz.LogEntry {
	entry := biz.LogEntry{
		ID:          pe.Id,
		Level:       biz.LogLevel(pe.Level),
		Message:     pe.Message,
		ServiceName: pe.ServiceName,
		HostID:      pe.HostId,
		TraceID:     pe.TraceId,
		SpanID:      pe.SpanId,
	}

	if pe.Timestamp != nil {
		entry.Timestamp = pe.Timestamp.AsTime()
	} else {
		entry.Timestamp = time.Now().UTC()
	}

	if len(pe.Labels) > 0 {
		entry.Labels = make(map[string]string, len(pe.Labels))
		for _, kv := range pe.Labels {
			entry.Labels[kv.GetKey()] = kv.GetValue()
		}
	}

	return entry
}
