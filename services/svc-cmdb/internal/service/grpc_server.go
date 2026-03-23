package service

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	cmdbpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/cmdb"
	commonpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/common"
	"github.com/opsnexus/svc-cmdb/internal/biz"
)

// GRPCServer 实现 CMDBService gRPC 接口。
// 供 svc-alert、svc-ai、svc-notify 等服务同步查询配置项信息。
type GRPCServer struct {
	cmdbpb.UnimplementedCMDBServiceServer
	uc     *biz.AssetUsecase
	logger *zap.Logger
}

// NewGRPCServer 创建一个新的 CMDB gRPC 服务实例。
func NewGRPCServer(uc *biz.AssetUsecase, logger *zap.Logger) *GRPCServer {
	return &GRPCServer{uc: uc, logger: logger}
}

// GetCI 根据配置项 ID 查询单个 CI 信息。
func (s *GRPCServer) GetCI(ctx context.Context, req *cmdbpb.GetCIRequest) (*cmdbpb.CI, error) {
	if req.CiId == "" {
		return nil, status.Error(codes.InvalidArgument, "ci_id is required")
	}

	asset, err := s.uc.GetAsset(ctx, req.CiId)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	return assetToProto(asset), nil
}

// BatchGetCIs 批量根据 ID 列表查询多个配置项，单条查询失败时跳过继续。
func (s *GRPCServer) BatchGetCIs(ctx context.Context, req *cmdbpb.BatchGetCIsRequest) (*cmdbpb.BatchGetCIsResponse, error) {
	if len(req.CiIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ci_ids is required")
	}

	resp := &cmdbpb.BatchGetCIsResponse{
		Items: make([]*cmdbpb.CI, 0, len(req.CiIds)),
	}

	for _, id := range req.CiIds {
		asset, err := s.uc.GetAsset(ctx, id)
		if err != nil {
			s.logger.Warn("batch get: skipping CI", zap.String("ci_id", id), zap.Error(err))
			continue
		}
		resp.Items = append(resp.Items, assetToProto(asset))
	}

	return resp, nil
}

// GetTopology 返回指定 CI 周围的拓扑依赖关系图。
func (s *GRPCServer) GetTopology(ctx context.Context, req *cmdbpb.GetTopologyRequest) (*cmdbpb.GetTopologyResponse, error) {
	if req.RootCiId == "" {
		return nil, status.Error(codes.InvalidArgument, "root_ci_id is required")
	}

	depth := int(req.Depth)
	if depth == 0 {
		depth = 3
	}
	direction := req.Direction
	if direction == "" {
		direction = "both"
	}

	graph, err := s.uc.GetTopology(ctx, req.RootCiId, depth, direction)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	resp := &cmdbpb.GetTopologyResponse{
		Nodes: make([]*cmdbpb.CI, 0, len(graph.Nodes)),
		Edges: make([]*cmdbpb.Relationship, 0, len(graph.Edges)),
	}

	for _, node := range graph.Nodes {
		resp.Nodes = append(resp.Nodes, assetToProto(node))
	}
	for _, edge := range graph.Edges {
		resp.Edges = append(resp.Edges, &cmdbpb.Relationship{
			Id:           edge.RelationID,
			SourceCiId:   edge.SourceAssetID,
			TargetCiId:   edge.TargetAssetID,
			Type:         edge.RelationType,
		})
	}

	return resp, nil
}

// GetCIOwner 返回配置项的负责人信息，当前从资产标签中提取。
func (s *GRPCServer) GetCIOwner(ctx context.Context, req *cmdbpb.GetCIOwnerRequest) (*cmdbpb.CIOwner, error) {
	if req.CiId == "" {
		return nil, status.Error(codes.InvalidArgument, "ci_id is required")
	}

	asset, err := s.uc.GetAsset(ctx, req.CiId)
	if err != nil {
		return nil, mapToGRPCError(err)
	}

	// 负责人信息存储在标签中，后续将通过外部目录服务丰富
	// 当前直接返回资产记录中已有的标签信息
	owner := &cmdbpb.CIOwner{}
	if v, ok := asset.Tags["owner_id"]; ok {
		owner.UserId = v
	}
	if v, ok := asset.Tags["owner_name"]; ok {
		owner.Name = v
	}
	if v, ok := asset.Tags["owner_email"]; ok {
		owner.Email = v
	}
	if v, ok := asset.Tags["owner_phone"]; ok {
		owner.Phone = v
	}
	if v, ok := asset.Tags["owner_team"]; ok {
		owner.Team = v
	}

	return owner, nil
}

// assetToProto 将内部 Asset 结构转换为 protobuf CI 消息。
func assetToProto(a *biz.Asset) *cmdbpb.CI {
	ci := &cmdbpb.CI{
		Id:          a.AssetID,
		Type:        a.AssetType,
		Status:      a.Status,
		Attributes:  make(map[string]string),
		Labels:      make([]*commonpb.KeyValue, 0),
		CreatedAt:   timestamppb.New(a.CreatedAt),
		UpdatedAt:   timestamppb.New(a.UpdatedAt),
	}

	if a.Hostname != nil {
		ci.Name = *a.Hostname
		ci.Attributes["hostname"] = *a.Hostname
	}
	if a.IP != nil {
		ci.Attributes["ip"] = *a.IP
	}
	if a.Environment != nil {
		ci.Attributes["environment"] = *a.Environment
	}
	if a.Region != nil {
		ci.Attributes["region"] = *a.Region
	}
	if a.Grade != nil {
		ci.Attributes["grade"] = *a.Grade
	}

	// 将标签映射为 proto labels
	for k, v := range a.Tags {
		ci.Labels = append(ci.Labels, &commonpb.KeyValue{Key: k, Value: v})
	}
	// 将业务单元映射为 proto labels
	for _, bu := range a.BusinessUnits {
		ci.Labels = append(ci.Labels, &commonpb.KeyValue{Key: "business_unit", Value: bu})
	}

	return ci
}

// mapToGRPCError 将领域错误转换为 gRPC status 错误码。
func mapToGRPCError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	// 根据错误消息中的关键词匹配 gRPC 状态码
	switch {
	case contains(errMsg, "not_found"), contains(errMsg, "not found"):
		return status.Error(codes.NotFound, errMsg)
	case contains(errMsg, "already_exists"), contains(errMsg, "already exists"):
		return status.Error(codes.AlreadyExists, errMsg)
	case contains(errMsg, "VALIDATION"), contains(errMsg, "invalid"):
		return status.Error(codes.InvalidArgument, errMsg)
	default:
		return status.Error(codes.Internal, errMsg)
	}
}

// contains 检查字符串 s 是否包含子串 substr。
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

// searchString 在字符串 s 中线性搜索子串 substr。
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
