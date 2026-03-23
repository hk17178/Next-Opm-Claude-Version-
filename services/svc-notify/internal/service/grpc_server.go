// Package service 提供通知服务的传输层实现，包括 gRPC 服务端。
package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonpb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/common"
	notifypb "github.com/opsnexus/opsnexus/pkg/proto/gen/go/notify"
	"github.com/opsnexus/svc-notify/internal/biz"
)

// NotifyGRPCServer 实现 NotifyService gRPC 接口，提供通知发送和状态查询端点。
type NotifyGRPCServer struct {
	notifypb.UnimplementedNotifyServiceServer
	notifyBiz *biz.NotifyUseCase
	logger    *zap.Logger
}

// NewNotifyGRPCServer 创建通知服务的 gRPC 服务端实例。
func NewNotifyGRPCServer(notifyBiz *biz.NotifyUseCase, logger *zap.Logger) *NotifyGRPCServer {
	return &NotifyGRPCServer{
		notifyBiz: notifyBiz,
		logger:    logger,
	}
}

// SendNotification 接收通知发送请求，校验参数后调用 notifyBiz.Send 发送通知，返回 notification_id。
func (s *NotifyGRPCServer) SendNotification(ctx context.Context, req *notifypb.SendNotificationRequest) (*notifypb.SendNotificationResponse, error) {
	if req.GetChannel() == "" {
		return nil, status.Error(codes.InvalidArgument, "channel is required")
	}
	if req.GetBody() == "" {
		return nil, status.Error(codes.InvalidArgument, "body is required")
	}
	if len(req.GetRecipients()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "recipients is required")
	}

	sendReq := biz.SendRequest{
		Channel:    biz.ChannelType(req.GetChannel()),
		Recipients: req.GetRecipients(),
		TemplateID: req.GetTemplateId(),
		Priority:   req.GetPriority(),
		Content: biz.MessageContent{
			Subject:   req.GetSubject(),
			Body:      req.GetBody(),
			Variables: req.GetVariables(),
		},
	}

	// Parse incident_id if provided
	if req.GetIncidentId() != "" {
		incID, err := uuid.Parse(req.GetIncidentId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid incident_id: %v", err)
		}
		sendReq.IncidentID = &incID
	}

	// Parse alert_id if provided
	if req.GetAlertId() != "" {
		alertID, err := uuid.Parse(req.GetAlertId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid alert_id: %v", err)
		}
		sendReq.AlertID = &alertID
	}

	log, err := s.notifyBiz.Send(ctx, sendReq)
	if err != nil {
		s.logger.Error("SendNotification failed", zap.Error(err))
		return &notifypb.SendNotificationResponse{
			NotificationId: log.ID.String(),
			Status:         commonpb.OperationStatus_OPERATION_STATUS_FAILED,
		}, nil
	}

	return &notifypb.SendNotificationResponse{
		NotificationId: log.ID.String(),
		Status:         commonpb.OperationStatus_OPERATION_STATUS_SUCCESS,
	}, nil
}

// GetNotificationStatus 根据 notification_id 查询通知投递状态。
func (s *NotifyGRPCServer) GetNotificationStatus(ctx context.Context, req *notifypb.GetNotificationStatusRequest) (*notifypb.NotificationStatus, error) {
	if req.GetNotificationId() == "" {
		return nil, status.Error(codes.InvalidArgument, "notification_id is required")
	}

	id, err := uuid.Parse(req.GetNotificationId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid notification_id: %v", err)
	}

	log, err := s.notifyBiz.GetNotification(ctx, id)
	if err != nil {
		return nil, mapNotifyToGRPCError(err)
	}

	return &notifypb.NotificationStatus{
		NotificationId: log.ID.String(),
		Channel:        string(log.ChannelType),
		Status:         string(log.Status),
		RetryCount:     int32(log.RetryCount),
		ErrorMessage:   log.ErrorMessage,
	}, nil
}

// mapNotifyToGRPCError 将领域层错误转换为 gRPC 状态错误码。
func mapNotifyToGRPCError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"), strings.Contains(msg, "no rows"):
		return status.Error(codes.NotFound, msg)
	case strings.Contains(msg, "invalid"):
		return status.Error(codes.InvalidArgument, msg)
	case strings.Contains(msg, "no enabled bot"):
		return status.Error(codes.FailedPrecondition, msg)
	default:
		return status.Error(codes.Internal, msg)
	}
}
