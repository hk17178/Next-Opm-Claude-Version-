// Package service 实现告警服务的对外接口层，包括 HTTP REST API、gRPC 服务和 Kafka 消费者。
package service

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	alertpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/alert"
	commonpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/common"
	"github.com/opsnexus/svc-alert/internal/biz"
)

// GRPCServer 实现 AlertService gRPC 接口。
// 被 svc-incident（告警关联）和 svc-notify（告警通知模板渲染）调用。
type GRPCServer struct {
	alertpb.UnimplementedAlertServiceServer
	ruleUC    *biz.RuleUseCase
	alertRepo biz.AlertRepository
	log       *zap.SugaredLogger
}

// NewGRPCServer 创建告警 gRPC 服务器实例。
func NewGRPCServer(ruleUC *biz.RuleUseCase, alertRepo biz.AlertRepository, log *zap.SugaredLogger) *GRPCServer {
	return &GRPCServer{
		ruleUC:    ruleUC,
		alertRepo: alertRepo,
		log:       log,
	}
}

// GetAlert 按 ID 查询单条告警，返回 protobuf 格式的告警信息。
func (s *GRPCServer) GetAlert(ctx context.Context, req *alertpb.GetAlertRequest) (*alertpb.Alert, error) {
	if req.GetAlertId() == "" {
		return nil, status.Error(codes.InvalidArgument, "alert_id is required")
	}

	alert, err := s.alertRepo.GetByID(req.GetAlertId())
	if err != nil {
		return nil, mapAlertGRPCError(err)
	}
	if alert == nil {
		return nil, status.Errorf(codes.NotFound, "alert not found: %s", req.GetAlertId())
	}

	return bizAlertToProto(alert), nil
}

// ListActiveAlerts 返回当前触发中的告警列表，支持按严重等级过滤和分页。
func (s *GRPCServer) ListActiveAlerts(ctx context.Context, req *alertpb.ListActiveAlertsRequest) (*alertpb.ListActiveAlertsResponse, error) {
	pageSize := 20
	pageToken := ""
	if p := req.GetPagination(); p != nil {
		if p.GetPageSize() > 0 {
			pageSize = int(p.GetPageSize())
		}
		pageToken = p.GetPageToken()
	}

	firingStatus := biz.AlertStatusFiring
	var severity *biz.Severity
	if req.GetSeverity() != "" {
		s := biz.Severity(req.GetSeverity())
		severity = &s
	}

	alerts, nextToken, err := s.alertRepo.List(&firingStatus, severity, pageSize, pageToken)
	if err != nil {
		return nil, mapAlertGRPCError(err)
	}

	resp := &alertpb.ListActiveAlertsResponse{
		Alerts: make([]*alertpb.Alert, 0, len(alerts)),
		Pagination: &commonpb.PageResponse{
			NextPageToken: nextToken,
			TotalCount:    int64(len(alerts)),
		},
	}

	for _, a := range alerts {
		resp.Alerts = append(resp.Alerts, bizAlertToProto(a))
	}

	return resp, nil
}

// GetAlertRule 按 ID 查询单条告警规则。
func (s *GRPCServer) GetAlertRule(ctx context.Context, req *alertpb.GetAlertRuleRequest) (*alertpb.AlertRule, error) {
	if req.GetRuleId() == "" {
		return nil, status.Error(codes.InvalidArgument, "rule_id is required")
	}

	rule, err := s.ruleUC.GetRule(req.GetRuleId())
	if err != nil {
		return nil, mapAlertGRPCError(err)
	}
	if rule == nil {
		return nil, status.Errorf(codes.NotFound, "rule not found: %s", req.GetRuleId())
	}

	return bizRuleToProto(rule), nil
}

// bizAlertToProto 将业务层 biz.Alert 转换为 protobuf Alert 消息。
func bizAlertToProto(a *biz.Alert) *alertpb.Alert {
	pb := &alertpb.Alert{
		Id:          a.AlertID,
		RuleId:      a.RuleID,
		Severity:    string(a.Severity),
		Status:      string(a.Status),
		Title:       a.Title,
		Description: a.Description,
		FiredAt:     timestamppb.New(a.TriggeredAt),
		HostId:      a.SourceHost,
		ServiceName: a.SourceService,
	}

	if a.ResolvedAt != nil {
		pb.ResolvedAt = timestamppb.New(*a.ResolvedAt)
	}

	for k, v := range a.Tags {
		pb.Labels = append(pb.Labels, &commonpb.KeyValue{Key: k, Value: v})
	}

	return pb
}

// bizRuleToProto 将业务层 biz.AlertRule 转换为 protobuf AlertRule 消息。
func bizRuleToProto(r *biz.AlertRule) *alertpb.AlertRule {
	pb := &alertpb.AlertRule{
		Id:          r.RuleID,
		Name:        r.Name,
		Description: r.Description,
		Severity:    string(r.Severity),
		Condition:   string(r.Condition),
		Enabled:     r.Enabled,
	}

	for k, v := range r.Labels {
		pb.Labels = append(pb.Labels, &commonpb.KeyValue{Key: k, Value: v})
	}

	return pb
}

// mapAlertGRPCError 将业务层错误映射为 gRPC 状态码（NotFound/AlreadyExists/InvalidArgument/Internal）。
func mapAlertGRPCError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "not found"):
		return status.Error(codes.NotFound, errMsg)
	case strings.Contains(errMsg, "already exists"):
		return status.Error(codes.AlreadyExists, errMsg)
	case strings.Contains(errMsg, "is required"), strings.Contains(errMsg, "invalid"):
		return status.Error(codes.InvalidArgument, errMsg)
	default:
		return status.Error(codes.Internal, errMsg)
	}
}

// Suppress unused import warnings.
var _ = time.Now
