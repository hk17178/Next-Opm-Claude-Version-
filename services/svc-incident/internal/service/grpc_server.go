package service

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/common"
	incidentpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/incident"
	"github.com/opsnexus/svc-incident/internal/biz"
)

// GRPCServer 实现 IncidentService gRPC 接口。
// 供 svc-ai（获取事件上下文用于分析）和 svc-notify（获取事件详情用于通知模板渲染）等服务调用。
type GRPCServer struct {
	incidentpb.UnimplementedIncidentServiceServer
	uc     *biz.IncidentUsecase
	logger *zap.Logger
}

// NewGRPCServer 创建一个新的事件 gRPC 服务实例。
func NewGRPCServer(uc *biz.IncidentUsecase, logger *zap.Logger) *GRPCServer {
	return &GRPCServer{uc: uc, logger: logger}
}

// GetIncident 根据事件 ID 查询单条事件信息。
func (s *GRPCServer) GetIncident(ctx context.Context, req *incidentpb.GetIncidentRequest) (*incidentpb.Incident, error) {
	if req.IncidentId == "" {
		return nil, status.Error(codes.InvalidArgument, "incident_id is required")
	}

	inc, err := s.uc.GetIncident(ctx, req.IncidentId)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	return incidentToProto(inc), nil
}

// ListOpenIncidents 返回当前未关闭的事件列表，过滤掉已解决和已关闭状态的事件。
func (s *GRPCServer) ListOpenIncidents(ctx context.Context, req *incidentpb.ListOpenIncidentsRequest) (*incidentpb.ListOpenIncidentsResponse, error) {
	page := 1
	pageSize := 20
	if req.Pagination != nil {
		if req.Pagination.PageSize > 0 {
			pageSize = int(req.Pagination.PageSize)
		}
	}

	// 构建查询过滤条件，排除已解决/已关闭的事件
	f := biz.ListFilter{
		Page:     page,
		PageSize: pageSize,
		SortBy:   "created_at",
		SortOrder: "desc",
	}
	if req.Severity != "" {
		f.Severity = &req.Severity
	}
	if req.AssigneeId != "" {
		f.AssigneeID = &req.AssigneeId
	}

	incidents, total, err := s.uc.ListIncidents(ctx, f)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	resp := &incidentpb.ListOpenIncidentsResponse{
		Incidents: make([]*incidentpb.Incident, 0, len(incidents)),
		Pagination: &commonpb.PageResponse{
			TotalCount: total,
		},
	}

	for _, inc := range incidents {
		// 仅保留未关闭的事件（排除 resolved 和 closed 状态）
		if inc.Status == biz.StatusResolved || inc.Status == biz.StatusClosed {
			continue
		}
		resp.Incidents = append(resp.Incidents, incidentToProto(inc))
	}

	return resp, nil
}

// AddTimelineEntry 向事件时间线添加一条记录，供其他服务通过 gRPC 调用。
func (s *GRPCServer) AddTimelineEntry(ctx context.Context, req *incidentpb.AddTimelineEntryRequest) (*incidentpb.AddTimelineEntryResponse, error) {
	if req.IncidentId == "" {
		return nil, status.Error(codes.InvalidArgument, "incident_id is required")
	}
	if req.Type == "" || req.Content == "" {
		return nil, status.Error(codes.InvalidArgument, "type and content are required")
	}

	entry := &biz.TimelineEntry{
		IncidentID: req.IncidentId,
		Timestamp:  time.Now(),
		EntryType:  req.Type,
		Source:     req.AuthorService,
		Content:    req.Content,
	}

	if err := s.uc.AddTimelineEntry(ctx, req.IncidentId, entry); err != nil {
		return nil, mapToGRPCError(err)
	}

	return &incidentpb.AddTimelineEntryResponse{
		EntryId: entry.EntryID,
	}, nil
}

// incidentToProto 将内部 Incident 结构转换为 protobuf Incident 消息。
// BUG-005 修复关联修正：修复模块路径后，biz.Incident 的字段类型现在是具体类型
// （AssigneeID *string、SourceAlerts []string、AffectedAssets []string、Tags map[string]string），
// 不再是 interface{}，移除无效的类型断言，直接使用具体类型进行转换。
func incidentToProto(inc *biz.Incident) *incidentpb.Incident {
	// AssigneeID 是 *string，需要解引用或传空字符串给 proto string 字段
	assigneeID := ""
	if inc.AssigneeID != nil {
		assigneeID = *inc.AssigneeID
	}

	pb := &incidentpb.Incident{
		Id:          inc.IncidentID,
		Title:       inc.Title,
		Description: inc.Description,
		Severity:    inc.Severity,
		Status:      string(inc.Status),
		AssigneeId:  assigneeID, // 已解引用 *string → string
		Labels:      make([]*commonpb.KeyValue, 0),
		CreatedAt:   timestamppb.New(inc.CreatedAt),
		UpdatedAt:   timestamppb.New(inc.UpdatedAt),
	}

	// 关联告警 ID 列表 → proto related_alert_ids
	// SourceAlerts 类型为 []string，直接赋值，无需类型断言
	pb.RelatedAlertIds = append(pb.RelatedAlertIds, inc.SourceAlerts...)

	// 受影响资产 ID 列表 → proto affected_ci_ids
	// AffectedAssets 类型为 []string，直接赋值，无需类型断言
	pb.AffectedCiIds = append(pb.AffectedCiIds, inc.AffectedAssets...)

	// 标签映射 → proto labels 键值对列表
	// Tags 类型为 map[string]string，直接遍历，无需类型断言
	for k, v := range inc.Tags {
		pb.Labels = append(pb.Labels, &commonpb.KeyValue{Key: k, Value: v})
	}

	if inc.ResolvedAt != nil {
		pb.ResolvedAt = timestamppb.New(*inc.ResolvedAt)
	}

	return pb
}

// mapToGRPCError 将领域错误转换为 gRPC status 错误码。
func mapToGRPCError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	switch {
	case containsStr(errMsg, "not_found"), containsStr(errMsg, "not found"):
		return status.Error(codes.NotFound, errMsg)
	case containsStr(errMsg, "already"), containsStr(errMsg, "conflict"):
		return status.Error(codes.AlreadyExists, errMsg)
	case containsStr(errMsg, "VALIDATION"), containsStr(errMsg, "invalid"):
		return status.Error(codes.InvalidArgument, errMsg)
	case containsStr(errMsg, "invalid_status_transition"):
		return status.Error(codes.FailedPrecondition, errMsg)
	default:
		return status.Error(codes.Internal, errMsg)
	}
}

// containsStr 检查字符串 s 是否包含子串 substr。
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
