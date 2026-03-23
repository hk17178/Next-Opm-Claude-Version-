// Package service 提供 svc-analytics 的 gRPC 服务层实现。
package service

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticspb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/analytics"
	commonpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/common"
	"github.com/opsnexus/svc-analytics/internal/biz"
)

// GRPCServer 实现 AnalyticsService gRPC 接口。
// 被 svc-alert、svc-incident、svc-ai 等服务通过 gRPC 同步调用，
// 提供指标查询、SLA 报告、相关性分析和知识库检索功能。
type GRPCServer struct {
	analyticspb.UnimplementedAnalyticsServiceServer
	metricsUC   *biz.MetricsUsecase
	slaUC       *biz.SLAUsecase
	knowledgeUC *biz.KnowledgeUsecase
	logger      *zap.Logger
}

// NewGRPCServer 创建 Analytics gRPC 服务实例。
func NewGRPCServer(
	metricsUC *biz.MetricsUsecase,
	slaUC *biz.SLAUsecase,
	knowledgeUC *biz.KnowledgeUsecase,
	logger *zap.Logger,
) *GRPCServer {
	return &GRPCServer{
		metricsUC:   metricsUC,
		slaUC:       slaUC,
		knowledgeUC: knowledgeUC,
		logger:      logger,
	}
}

// QueryMetrics 查询时间序列指标数据，支持多种聚合函数和分组维度。
func (s *GRPCServer) QueryMetrics(ctx context.Context, req *analyticspb.QueryMetricsRequest) (*analyticspb.QueryMetricsResponse, error) {
	if req.MetricName == "" {
		return nil, status.Error(codes.InvalidArgument, "metric_name is required")
	}
	if req.TimeRange == nil {
		return nil, status.Error(codes.InvalidArgument, "time_range is required")
	}

	q := biz.MetricQueryRequest{
		MetricName:  req.MetricName,
		Aggregation: req.Aggregation,
		Interval:    req.Interval,
		Filters:     req.Filters,
		GroupBy:     req.GroupBy,
	}
	if req.TimeRange.Start != nil {
		q.TimeRange.Start = req.TimeRange.Start.AsTime()
	}
	if req.TimeRange.End != nil {
		q.TimeRange.End = req.TimeRange.End.AsTime()
	}

	result, err := s.metricsUC.Query(ctx, q)
	if err != nil {
		return nil, mapAnalyticsGRPCError(err)
	}

	return convertMetricSeries(result), nil
}

// GetSLAReport 获取指定时间段的 SLA 合规报告。
// 通过 business_unit 维度匹配服务名称，返回可用性百分比和事件数量。
func (s *GRPCServer) GetSLAReport(ctx context.Context, req *analyticspb.GetSLAReportRequest) (*analyticspb.SLAReport, error) {
	if req.ServiceName == "" {
		return nil, status.Error(codes.InvalidArgument, "service_name is required")
	}
	if req.TimeRange == nil {
		return nil, status.Error(codes.InvalidArgument, "time_range is required")
	}

	var start, end time.Time
	if req.TimeRange.Start != nil {
		start = req.TimeRange.Start.AsTime()
	}
	if req.TimeRange.End != nil {
		end = req.TimeRange.End.AsTime()
	}

	// Find SLA configs matching this service name by looking up business_unit dimension.
	results, err := s.slaUC.CalculateByDimension(ctx, biz.SLALevelBusinessUnit, start, end)
	if err != nil {
		return nil, mapAnalyticsGRPCError(err)
	}

	// Find the result matching the requested service.
	report := &analyticspb.SLAReport{
		ServiceName: req.ServiceName,
		TimeRange: &commonpb.TimeRange{
			Start: timestamppb.New(start),
			End:   timestamppb.New(end),
		},
	}

	for _, r := range results {
		if strings.EqualFold(r.DimensionValue, req.ServiceName) {
			report.AvailabilityPercent = r.ActualPercentage
			report.TotalIncidents = int64(r.IncidentCount)
			break
		}
	}

	return report, nil
}

// GetMetrics 使用简化的 RPC 接口查询指标数据。
func (s *GRPCServer) GetMetrics(ctx context.Context, req *analyticspb.GetMetricsRequest) (*analyticspb.GetMetricsResponse, error) {
	if req.MetricName == "" {
		return nil, status.Error(codes.InvalidArgument, "metric_name is required")
	}
	if req.TimeRange == nil {
		return nil, status.Error(codes.InvalidArgument, "time_range is required")
	}

	q := biz.MetricQueryRequest{
		MetricName:  req.MetricName,
		Aggregation: req.Aggregation,
		Interval:    req.Interval,
		Filters:     req.Filters,
		GroupBy:     req.GroupBy,
	}
	if req.TimeRange.Start != nil {
		q.TimeRange.Start = req.TimeRange.Start.AsTime()
	}
	if req.TimeRange.End != nil {
		q.TimeRange.End = req.TimeRange.End.AsTime()
	}

	result, err := s.metricsUC.Query(ctx, q)
	if err != nil {
		return nil, mapAnalyticsGRPCError(err)
	}

	series := convertMetricSeries(result)
	return &analyticspb.GetMetricsResponse{
		Series: series.Series,
	}, nil
}

// GetCorrelation 计算指定资产上两个资源指标之间的 Pearson 相关系数。
// FR-12-008：为 AI 自动洞察提供相关性分析能力。
func (s *GRPCServer) GetCorrelation(ctx context.Context, req *analyticspb.GetCorrelationRequest) (*analyticspb.GetCorrelationResponse, error) {
	if req.AssetId == "" {
		return nil, status.Error(codes.InvalidArgument, "asset_id is required")
	}
	if req.MetricA == "" || req.MetricB == "" {
		return nil, status.Error(codes.InvalidArgument, "metric_a and metric_b are required")
	}
	if req.TimeRange == nil {
		return nil, status.Error(codes.InvalidArgument, "time_range is required")
	}

	var start, end time.Time
	if req.TimeRange.Start != nil {
		start = req.TimeRange.Start.AsTime()
	}
	if req.TimeRange.End != nil {
		end = req.TimeRange.End.AsTime()
	}

	corr, err := s.metricsUC.Correlate(ctx, req.AssetId, req.MetricA, req.MetricB, start, end)
	if err != nil {
		return nil, mapAnalyticsGRPCError(err)
	}

	return &analyticspb.GetCorrelationResponse{
		MetricA:                corr.MetricA,
		MetricB:                corr.MetricB,
		AssetId:                corr.AssetID,
		CorrelationCoefficient: corr.CorrelationCoef,
		SampleCount:            int64(corr.SampleCount),
	}, nil
}

// GetKnowledgeArticle 根据 ID 获取知识库文章。
// FR-17-004：供 svc-ai 获取相关故障案例，辅助 AI 分析。
func (s *GRPCServer) GetKnowledgeArticle(ctx context.Context, req *analyticspb.GetKnowledgeArticleRequest) (*analyticspb.KnowledgeArticleResponse, error) {
	if req.ArticleId == "" {
		return nil, status.Error(codes.InvalidArgument, "article_id is required")
	}

	article, err := s.knowledgeUC.Get(ctx, req.ArticleId)
	if err != nil {
		return nil, mapAnalyticsGRPCError(err)
	}

	resp := &analyticspb.KnowledgeArticleResponse{
		ArticleId:    article.ArticleID,
		Type:         article.Type,
		Title:        article.Title,
		Content:      article.Content,
		Tags:         article.Tags,
		QualityScore: article.QualityScore,
		Status:       article.Status,
		CreatedAt:    timestamppb.New(article.CreatedAt),
		UpdatedAt:    timestamppb.New(article.UpdatedAt),
	}
	if article.RelatedIncident != nil {
		resp.RelatedIncident = *article.RelatedIncident
	}

	return resp, nil
}

// convertMetricSeries 将业务层 MetricQueryResponse 转换为 protobuf QueryMetricsResponse。
func convertMetricSeries(result *biz.MetricQueryResponse) *analyticspb.QueryMetricsResponse {
	resp := &analyticspb.QueryMetricsResponse{
		Series: make([]*analyticspb.MetricSeries, 0, len(result.Series)),
	}
	for _, series := range result.Series {
		pbSeries := &analyticspb.MetricSeries{
			Labels:     series.Labels,
			DataPoints: make([]*analyticspb.DataPoint, 0, len(series.DataPoints)),
		}
		for _, dp := range series.DataPoints {
			pbSeries.DataPoints = append(pbSeries.DataPoints, &analyticspb.DataPoint{
				Timestamp: timestamppb.New(dp.Timestamp),
				Value:     dp.Value,
			})
		}
		resp.Series = append(resp.Series, pbSeries)
	}
	return resp
}

// mapAnalyticsGRPCError 将领域层错误映射为 gRPC 状态码。
func mapAnalyticsGRPCError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "not found"):
		return status.Error(codes.NotFound, errMsg)
	case strings.Contains(errMsg, "is required"), strings.Contains(errMsg, "invalid"):
		return status.Error(codes.InvalidArgument, errMsg)
	default:
		return status.Error(codes.Internal, errMsg)
	}
}
