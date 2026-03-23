// Package service 提供 svc-ai 的 gRPC 服务层实现，将 protobuf 请求转换为业务层调用。
package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	aipb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/ai"
	commonpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/common"
	"github.com/opsnexus/svc-ai/internal/biz"
)

// AIGRPCServer 实现 AIService gRPC 接口，对外提供分析请求、结果查询和知识库搜索功能。
type AIGRPCServer struct {
	aipb.UnimplementedAIServiceServer
	analysisBiz   *biz.AnalysisUseCase
	knowledgeRepo biz.KnowledgeRepo
	logger        *zap.Logger
}

// NewAIGRPCServer 创建 AI gRPC 服务实例。
func NewAIGRPCServer(analysisBiz *biz.AnalysisUseCase, knowledgeRepo biz.KnowledgeRepo, logger *zap.Logger) *AIGRPCServer {
	return &AIGRPCServer{
		analysisBiz:   analysisBiz,
		knowledgeRepo: knowledgeRepo,
		logger:        logger,
	}
}

// RequestAnalysis 接收分析请求，触发异步 AI 分析并返回 analysis_id。
// 调用方通过 GetAnalysisResult 轮询获取最终结果。
func (s *AIGRPCServer) RequestAnalysis(ctx context.Context, req *aipb.RequestAnalysisRequest) (*aipb.RequestAnalysisResponse, error) {
	if req.GetType() == "" {
		return nil, status.Error(codes.InvalidArgument, "type is required")
	}

	createReq := biz.AnalysisCreateRequest{
		Type: biz.AnalysisType(req.GetType()),
	}

	// Parse incident_id if provided
	if req.GetIncidentId() != "" {
		incID, err := uuid.Parse(req.GetIncidentId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid incident_id: %v", err)
		}
		createReq.IncidentID = &incID
	}

	// Parse alert_ids
	for _, aid := range req.GetAlertIds() {
		parsed, err := uuid.Parse(aid)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid alert_id %q: %v", aid, err)
		}
		createReq.AlertIDs = append(createReq.AlertIDs, parsed)
	}

	// Convert context map to JSON
	if reqCtx := req.GetContext(); len(reqCtx) > 0 {
		ctxJSON, err := json.Marshal(reqCtx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to marshal context: %v", err)
		}
		createReq.Context = ctxJSON
	}

	task, err := s.analysisBiz.CreateAnalysis(ctx, createReq)
	if err != nil {
		s.logger.Error("RequestAnalysis failed", zap.Error(err))
		return nil, mapToGRPCError(err)
	}

	return &aipb.RequestAnalysisResponse{
		AnalysisId: task.ID.String(),
		Status:     commonpb.OperationStatus_OPERATION_STATUS_PENDING,
	}, nil
}

// GetAnalysisResult 根据 analysis_id 查询分析结果，返回根因、置信度和证据链。
func (s *AIGRPCServer) GetAnalysisResult(ctx context.Context, req *aipb.GetAnalysisResultRequest) (*aipb.AnalysisResult, error) {
	if req.GetAnalysisId() == "" {
		return nil, status.Error(codes.InvalidArgument, "analysis_id is required")
	}

	id, err := uuid.Parse(req.GetAnalysisId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid analysis_id: %v", err)
	}

	task, err := s.analysisBiz.GetAnalysis(ctx, id)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	result := &aipb.AnalysisResult{
		AnalysisId:   task.ID.String(),
		Type:         string(task.Type),
		Status:       analysisStatusToProto(task.Status),
		ModelVersion: task.ModelVersion,
	}

	if task.Result != nil {
		result.Summary = task.Result.Summary
		result.Confidence = task.Result.Confidence
		result.Recommendations = task.Result.Recommendations

		for _, rc := range task.Result.RootCauses {
			pbRC := &aipb.RootCause{
				Description: rc.Description,
				Probability: rc.Probability,
			}
			pbRC.RelatedCiIds = rc.RelatedCIIDs
			result.RootCauses = append(result.RootCauses, pbRC)
		}
	}

	return result, nil
}

// SearchKnowledge 对知识库执行语义搜索，返回相似案例列表及相似度评分。
func (s *AIGRPCServer) SearchKnowledge(ctx context.Context, req *aipb.SearchKnowledgeRequest) (*aipb.SearchKnowledgeResponse, error) {
	if req.GetQuery() == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	if s.knowledgeRepo == nil {
		return nil, status.Error(codes.Unimplemented, "knowledge base not configured")
	}

	topK := int(req.GetTopK())
	if topK <= 0 {
		topK = 10
	}

	entries, scores, err := s.knowledgeRepo.Search(ctx, req.GetQuery(), topK)
	if err != nil {
		s.logger.Error("SearchKnowledge failed", zap.Error(err))
		return nil, mapToGRPCError(err)
	}

	resp := &aipb.SearchKnowledgeResponse{
		Results: make([]*aipb.KnowledgeResult, 0, len(entries)),
	}

	for i, entry := range entries {
		kr := &aipb.KnowledgeResult{
			Id:       entry.ID.String(),
			Title:    entry.Title,
			Content:  entry.Content,
			Category: entry.Category,
		}
		if i < len(scores) {
			kr.Score = scores[i]
		}
		resp.Results = append(resp.Results, kr)
	}

	return resp, nil
}

// analysisStatusToProto 将业务层的分析状态映射为 protobuf 操作状态枚举。
func analysisStatusToProto(s biz.AnalysisStatus) commonpb.OperationStatus {
	switch s {
	case biz.StatusPending:
		return commonpb.OperationStatus_OPERATION_STATUS_PENDING
	case biz.StatusRunning:
		return commonpb.OperationStatus_OPERATION_STATUS_RUNNING
	case biz.StatusSuccess:
		return commonpb.OperationStatus_OPERATION_STATUS_SUCCESS
	case biz.StatusPartial:
		return commonpb.OperationStatus_OPERATION_STATUS_PARTIAL
	case biz.StatusFailed:
		return commonpb.OperationStatus_OPERATION_STATUS_FAILED
	default:
		return commonpb.OperationStatus_OPERATION_STATUS_UNSPECIFIED
	}
}

// mapToGRPCError 将领域层错误转换为 gRPC 状态码错误。
// 根据错误消息内容自动映射为 NotFound、InvalidArgument 或 Internal。
func mapToGRPCError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"), strings.Contains(msg, "no rows"):
		return status.Error(codes.NotFound, msg)
	case strings.Contains(msg, "invalid"):
		return status.Error(codes.InvalidArgument, msg)
	default:
		return status.Error(codes.Internal, msg)
	}
}
