package biz

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
)

// ApprovalUsecase 编排变更审批流程，包括审批通过、拒绝和自动路由。
type ApprovalUsecase struct {
	changeRepo   ChangeRepo
	approvalRepo ApprovalRepo
	logger       *zap.Logger
	auditLogger  AuditLogger // 审计日志记录器，可为 nil（nil 时跳过审计记录）
}

// NewApprovalUsecase 创建一个新的审批用例实例。
func NewApprovalUsecase(changeRepo ChangeRepo, approvalRepo ApprovalRepo, logger *zap.Logger, opts ...ApprovalUsecaseOption) *ApprovalUsecase {
	uc := &ApprovalUsecase{
		changeRepo:   changeRepo,
		approvalRepo: approvalRepo,
		logger:       logger,
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// ApprovalUsecaseOption 审批用例可选配置函数。
type ApprovalUsecaseOption func(*ApprovalUsecase)

// WithApprovalAuditLogger 设置审批用例的审计日志记录器。
func WithApprovalAuditLogger(al AuditLogger) ApprovalUsecaseOption {
	return func(uc *ApprovalUsecase) {
		uc.auditLogger = al
	}
}

// logAudit 记录审计日志（如果 auditLogger 非 nil）。
func (u *ApprovalUsecase) logAudit(ctx context.Context, action, operator, resourceID, detail string) {
	if u.auditLogger != nil {
		if err := u.auditLogger.Log(ctx, action, operator, "change_ticket", resourceID, detail); err != nil {
			u.logger.Warn("审计日志记录失败",
				zap.String("action", action),
				zap.String("change_id", resourceID),
				zap.Error(err),
			)
		}
	}
}

// Approve 审批通过变更单，记录审批决策并将变更状态流转为 approved。
func (u *ApprovalUsecase) Approve(ctx context.Context, changeID, approverID, comment string) error {
	// 查询变更单
	ticket, err := u.changeRepo.GetByID(ctx, changeID)
	if err != nil {
		return apperrors.Internal("APPROVAL_GET", fmt.Sprintf("failed to get change ticket: %v", err))
	}
	if ticket == nil {
		return apperrors.NotFound("change.not_found", fmt.Sprintf("change ticket %s not found", changeID))
	}

	// 校验变更单状态：仅 pending_approval 状态可审批
	if ticket.Status != StatusPendingApproval {
		return apperrors.BadRequest("APPROVAL_INVALID_STATUS",
			fmt.Sprintf("change ticket %s is in %s status, cannot approve", changeID, ticket.Status))
	}

	// 记录审批决策
	record := &ApprovalRecord{
		ChangeID:   changeID,
		ApproverID: approverID,
		Decision:   "approved",
		Comment:    comment,
		DecidedAt:  time.Now(),
	}
	if err := u.approvalRepo.Create(ctx, record); err != nil {
		return apperrors.Internal("APPROVAL_CREATE", fmt.Sprintf("failed to create approval record: %v", err))
	}

	// 更新变更单状态为已批准
	ticket.Status = StatusApproved
	ticket.Approvers = append(ticket.Approvers, approverID)
	ticket.UpdatedAt = time.Now()

	if err := u.changeRepo.Update(ctx, ticket); err != nil {
		return apperrors.Internal("APPROVAL_UPDATE", fmt.Sprintf("failed to update change ticket status: %v", err))
	}

	u.logger.Info("change ticket approved",
		zap.String("change_id", changeID),
		zap.String("approver_id", approverID),
	)

	// 记录审计日志：审批通过
	u.logAudit(ctx, "审批通过", approverID, changeID,
		fmt.Sprintf("变更单 %s 审批通过，审批人=%s，备注=%s", changeID, approverID, comment))

	return nil
}

// Reject 拒绝变更单，记录拒绝原因并将变更状态流转为 rejected。
func (u *ApprovalUsecase) Reject(ctx context.Context, changeID, approverID, reason string) error {
	// 查询变更单
	ticket, err := u.changeRepo.GetByID(ctx, changeID)
	if err != nil {
		return apperrors.Internal("APPROVAL_GET", fmt.Sprintf("failed to get change ticket: %v", err))
	}
	if ticket == nil {
		return apperrors.NotFound("change.not_found", fmt.Sprintf("change ticket %s not found", changeID))
	}

	// 校验变更单状态：仅 pending_approval 状态可拒绝
	if ticket.Status != StatusPendingApproval {
		return apperrors.BadRequest("APPROVAL_INVALID_STATUS",
			fmt.Sprintf("change ticket %s is in %s status, cannot reject", changeID, ticket.Status))
	}

	// 记录拒绝决策
	record := &ApprovalRecord{
		ChangeID:   changeID,
		ApproverID: approverID,
		Decision:   "rejected",
		Comment:    reason,
		DecidedAt:  time.Now(),
	}
	if err := u.approvalRepo.Create(ctx, record); err != nil {
		return apperrors.Internal("APPROVAL_CREATE", fmt.Sprintf("failed to create approval record: %v", err))
	}

	// 更新变更单状态为已拒绝
	ticket.Status = StatusRejected
	ticket.UpdatedAt = time.Now()

	if err := u.changeRepo.Update(ctx, ticket); err != nil {
		return apperrors.Internal("APPROVAL_UPDATE", fmt.Sprintf("failed to update change ticket status: %v", err))
	}

	u.logger.Info("change ticket rejected",
		zap.String("change_id", changeID),
		zap.String("approver_id", approverID),
		zap.String("reason", reason),
	)

	// 记录审计日志：审批拒绝
	u.logAudit(ctx, "审批拒绝", approverID, changeID,
		fmt.Sprintf("变更单 %s 审批拒绝，审批人=%s，原因=%s", changeID, approverID, reason))

	return nil
}

// ListByChange 查询指定变更单的所有审批记录。
func (u *ApprovalUsecase) ListByChange(ctx context.Context, changeID string) ([]*ApprovalRecord, error) {
	records, err := u.approvalRepo.ListByChange(ctx, changeID)
	if err != nil {
		return nil, apperrors.Internal("APPROVAL_LIST", fmt.Sprintf("failed to list approval records: %v", err))
	}
	return records, nil
}

// AutoRoute 根据变更单的风险级别自动路由审批人。
// 路由规则：
//   - low（低风险）：自动通过，无需审批人
//   - medium（中风险）：路由给主管
//   - high（高风险）：路由给总监
//   - critical（极高风险）：路由给 VP
func (u *ApprovalUsecase) AutoRoute(ctx context.Context, change *ChangeTicket) ([]string, error) {
	switch change.RiskLevel {
	case RiskLow:
		// 低风险变更自动通过，返回空审批人列表
		return []string{}, nil
	case RiskMedium:
		// 中风险需主管审批
		return []string{"role:supervisor"}, nil
	case RiskHigh:
		// 高风险需总监审批
		return []string{"role:director"}, nil
	case RiskCritical:
		// 极高风险需 VP 审批
		return []string{"role:vp"}, nil
	default:
		return []string{"role:supervisor"}, nil
	}
}
