// Package biz 包含变更管理领域的核心业务逻辑。
package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
)

// ChangeRepo 定义变更单持久化接口，由数据层实现。
type ChangeRepo interface {
	// NextID 生成下一个变更单 ID，格式为 CHG-YYYYMMDD-NNN。
	NextID(ctx context.Context) (string, error)
	// Create 创建一条新的变更单记录。
	Create(ctx context.Context, ticket *ChangeTicket) error
	// GetByID 根据变更单 ID 查询单条记录，不存在时返回 (nil, nil)。
	GetByID(ctx context.Context, id string) (*ChangeTicket, error)
	// Update 更新已有变更单记录。
	Update(ctx context.Context, ticket *ChangeTicket) error
	// List 根据过滤条件和分页参数查询变更单列表。
	List(ctx context.Context, f ListFilter) ([]*ChangeTicket, int64, error)
	// FindConflicts 查找与指定时间段和资产列表存在冲突的变更单。
	// 冲突条件：时间段重叠 AND 资产有交集，排除已完成/已取消/已拒绝的变更单。
	FindConflicts(ctx context.Context, start, end time.Time, assets []string, excludeID string) ([]*ChangeTicket, error)
	// ListByTimeRange 查询指定时间范围内的变更单（用于变更日历）。
	ListByTimeRange(ctx context.Context, start, end time.Time) ([]*ChangeTicket, error)
}

// ApprovalRepo 定义审批记录持久化接口，由数据层实现。
type ApprovalRepo interface {
	// Create 创建一条审批记录。
	Create(ctx context.Context, record *ApprovalRecord) error
	// ListByChange 查询指定变更单的所有审批记录。
	ListByChange(ctx context.Context, changeID string) ([]*ApprovalRecord, error)
}

// ChangeUsecase 编排变更单全生命周期操作，包括创建、提交审批、开始执行、完成和取消。
type ChangeUsecase struct {
	repo              ChangeRepo
	approval          ApprovalRepo
	logger            *zap.Logger
	auditLogger       AuditLogger       // 审计日志记录器，可为 nil（nil 时跳过审计记录）
	maintenanceClient MaintenanceClient // 维护模式客户端，可为 nil（nil 时跳过维护模式联动）
	aiAssessor        AIRiskAssessor    // AI 风险评估器，可为 nil（nil 时跳过风险评估）
}

// NewChangeUsecase 创建一个新的变更单用例实例，注入仓储依赖。
func NewChangeUsecase(repo ChangeRepo, approval ApprovalRepo, logger *zap.Logger, opts ...ChangeUsecaseOption) *ChangeUsecase {
	uc := &ChangeUsecase{
		repo:     repo,
		approval: approval,
		logger:   logger,
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// ChangeUsecaseOption 变更用例可选配置函数。
type ChangeUsecaseOption func(*ChangeUsecase)

// WithAuditLogger 设置审计日志记录器。
func WithAuditLogger(al AuditLogger) ChangeUsecaseOption {
	return func(uc *ChangeUsecase) {
		uc.auditLogger = al
	}
}

// WithMaintenanceClient 设置维护模式客户端。
func WithMaintenanceClient(mc MaintenanceClient) ChangeUsecaseOption {
	return func(uc *ChangeUsecase) {
		uc.maintenanceClient = mc
	}
}

// WithAIRiskAssessor 设置 AI 风险评估器。
func WithAIRiskAssessor(ar AIRiskAssessor) ChangeUsecaseOption {
	return func(uc *ChangeUsecase) {
		uc.aiAssessor = ar
	}
}

// logAudit 记录审计日志（如果 auditLogger 非 nil）。
func (u *ChangeUsecase) logAudit(ctx context.Context, action, operator, resourceID, detail string) {
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

// Create 创建新的变更单，自动生成唯一 ID，初始状态为 draft。
func (u *ChangeUsecase) Create(ctx context.Context, req *CreateChangeReq) (*ChangeTicket, error) {
	// 校验必填字段
	if req.Title == "" {
		return nil, apperrors.BadRequest("VALIDATION", "title is required")
	}
	if req.Type == "" {
		return nil, apperrors.BadRequest("VALIDATION", "type is required")
	}
	if req.RiskLevel == "" {
		return nil, apperrors.BadRequest("VALIDATION", "risk_level is required")
	}

	// 生成唯一变更单 ID
	id, err := u.repo.NextID(ctx)
	if err != nil {
		return nil, apperrors.Internal("CHANGE_ID_GENERATE", fmt.Sprintf("failed to generate change ID: %v", err))
	}

	now := time.Now()
	ticket := &ChangeTicket{
		ID:             id,
		Title:          req.Title,
		Type:           req.Type,
		RiskLevel:      req.RiskLevel,
		Status:         StatusDraft,
		Requester:      req.Requester,
		ExecutorID:     req.ExecutorID,
		AffectedAssets: req.AffectedAssets,
		RollbackPlan:   req.RollbackPlan,
		ScheduledStart: req.ScheduledStart,
		ScheduledEnd:   req.ScheduledEnd,
		Description:    req.Description,
		MaintenanceID:  req.MaintenanceID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// 初始化切片字段，避免 JSON 序列化时出现 null
	if ticket.AffectedAssets == nil {
		ticket.AffectedAssets = []string{}
	}
	if ticket.Approvers == nil {
		ticket.Approvers = []string{}
	}
	if ticket.RelatedChangeIDs == nil {
		ticket.RelatedChangeIDs = []string{}
	}

	if err := u.repo.Create(ctx, ticket); err != nil {
		return nil, apperrors.Internal("CHANGE_CREATE", fmt.Sprintf("failed to create change ticket: %v", err))
	}

	u.logger.Info("change ticket created",
		zap.String("change_id", ticket.ID),
		zap.String("type", string(ticket.Type)),
		zap.String("risk_level", string(ticket.RiskLevel)),
	)

	// 记录审计日志：创建变更单
	u.logAudit(ctx, "创建变更单", req.Requester, ticket.ID,
		fmt.Sprintf("创建变更单 %s，类型=%s，风险=%s", ticket.ID, ticket.Type, ticket.RiskLevel))

	return ticket, nil
}

// GetByID 根据变更单 ID 查询单条记录，不存在时返回 NotFound 错误。
func (u *ChangeUsecase) GetByID(ctx context.Context, id string) (*ChangeTicket, error) {
	ticket, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperrors.Internal("CHANGE_GET", fmt.Sprintf("failed to get change ticket: %v", err))
	}
	if ticket == nil {
		return nil, apperrors.NotFound("change.not_found", fmt.Sprintf("change ticket %s not found", id))
	}
	return ticket, nil
}

// List 返回按条件过滤和分页的变更单列表。
func (u *ChangeUsecase) List(ctx context.Context, filter ListFilter) ([]*ChangeTicket, int64, error) {
	tickets, total, err := u.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, apperrors.Internal("CHANGE_LIST", fmt.Sprintf("failed to list change tickets: %v", err))
	}
	return tickets, total, nil
}

// Update 更新变更单信息，仅允许在 draft 状态下修改。
func (u *ChangeUsecase) Update(ctx context.Context, id string, req *UpdateChangeReq) (*ChangeTicket, error) {
	ticket, err := u.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 仅草稿状态允许编辑
	if ticket.Status != StatusDraft {
		return nil, apperrors.BadRequest("CHANGE_NOT_EDITABLE",
			fmt.Sprintf("change ticket %s is in %s status, only draft status can be edited", id, ticket.Status))
	}

	// 应用更新字段
	if req.Title != nil {
		ticket.Title = *req.Title
	}
	if req.Type != nil {
		ticket.Type = *req.Type
	}
	if req.RiskLevel != nil {
		ticket.RiskLevel = *req.RiskLevel
	}
	if req.ExecutorID != nil {
		ticket.ExecutorID = *req.ExecutorID
	}
	if req.AffectedAssets != nil {
		ticket.AffectedAssets = req.AffectedAssets
	}
	if req.RollbackPlan != nil {
		ticket.RollbackPlan = *req.RollbackPlan
	}
	if req.ScheduledStart != nil {
		ticket.ScheduledStart = *req.ScheduledStart
	}
	if req.ScheduledEnd != nil {
		ticket.ScheduledEnd = *req.ScheduledEnd
	}
	if req.Description != nil {
		ticket.Description = *req.Description
	}
	if req.MaintenanceID != nil {
		ticket.MaintenanceID = *req.MaintenanceID
	}

	ticket.UpdatedAt = time.Now()

	if err := u.repo.Update(ctx, ticket); err != nil {
		return nil, apperrors.Internal("CHANGE_UPDATE", fmt.Sprintf("failed to update change ticket: %v", err))
	}

	u.logger.Info("change ticket updated", zap.String("change_id", id))

	return ticket, nil
}

// Submit 提交变更单进入审批流程，状态从 draft 流转到 pending_approval。
// 紧急变更（emergency）将自动通过审批。
func (u *ChangeUsecase) Submit(ctx context.Context, id string) error {
	ticket, err := u.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !CanTransition(ticket.Status, StatusPendingApproval) {
		return apperrors.BadRequest("CHANGE_INVALID_TRANSITION",
			fmt.Sprintf("cannot submit change ticket in %s status", ticket.Status))
	}

	// AI 风险评估：若 aiAssessor 非 nil，调用 AssessRisk（带 5 秒超时）
	if u.aiAssessor != nil {
		assessCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		assessment, assessErr := u.aiAssessor.AssessRisk(assessCtx, ticket)
		if assessErr != nil {
			u.logger.Warn("AI 风险评估失败，使用空结果",
				zap.String("change_id", id),
				zap.Error(assessErr),
			)
		} else if assessment != nil {
			// 将评估结果序列化存入 AIRiskSummary 字段
			if data, err := json.Marshal(assessment); err == nil {
				ticket.AIRiskSummary = string(data)
			}
		}
	}

	ticket.Status = StatusPendingApproval
	ticket.UpdatedAt = time.Now()

	if err := u.repo.Update(ctx, ticket); err != nil {
		return apperrors.Internal("CHANGE_SUBMIT", fmt.Sprintf("failed to submit change ticket: %v", err))
	}

	u.logger.Info("change ticket submitted for approval",
		zap.String("change_id", id),
		zap.String("risk_level", string(ticket.RiskLevel)),
	)

	// 记录审计日志：提交审批
	u.logAudit(ctx, "提交审批", ticket.Requester, id,
		fmt.Sprintf("变更单 %s 提交审批，风险级别=%s", id, ticket.RiskLevel))

	return nil
}

// StartExecution 开始执行变更，状态从 approved 流转到 in_progress，记录实际开始时间。
func (u *ChangeUsecase) StartExecution(ctx context.Context, id string) error {
	ticket, err := u.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !CanTransition(ticket.Status, StatusInProgress) {
		return apperrors.BadRequest("CHANGE_INVALID_TRANSITION",
			fmt.Sprintf("cannot start execution in %s status, must be approved first", ticket.Status))
	}

	now := time.Now()
	ticket.Status = StatusInProgress
	ticket.ActualStart = &now
	ticket.UpdatedAt = now

	if err := u.repo.Update(ctx, ticket); err != nil {
		return apperrors.Internal("CHANGE_START", fmt.Sprintf("failed to start change execution: %v", err))
	}

	// 维护模式联动：若 maintenanceClient 非 nil 且变更单有影响资产，启用维护模式
	if u.maintenanceClient != nil && len(ticket.AffectedAssets) > 0 {
		// 维护时长 = 计划结束 - 计划开始 + 30 分钟缓冲
		duration := ticket.ScheduledEnd.Sub(ticket.ScheduledStart) + 30*time.Minute
		reason := fmt.Sprintf("变更单 %s 执行中", id)
		if err := u.maintenanceClient.EnableMaintenance(ctx, ticket.AffectedAssets, duration, reason); err != nil {
			// 维护模式启用失败不影响主流程，仅记录日志
			u.logger.Warn("启用维护模式失败",
				zap.String("change_id", id),
				zap.Error(err),
			)
		}
	}

	u.logger.Info("change execution started", zap.String("change_id", id))

	// 记录审计日志：开始执行变更
	u.logAudit(ctx, "开始执行变更", ticket.ExecutorID, id,
		fmt.Sprintf("变更单 %s 开始执行", id))

	return nil
}

// Complete 完成变更，状态从 in_progress 流转到 completed，记录实际结束时间。
func (u *ChangeUsecase) Complete(ctx context.Context, id string) error {
	ticket, err := u.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !CanTransition(ticket.Status, StatusCompleted) {
		return apperrors.BadRequest("CHANGE_INVALID_TRANSITION",
			fmt.Sprintf("cannot complete change ticket in %s status", ticket.Status))
	}

	now := time.Now()
	ticket.Status = StatusCompleted
	ticket.ActualEnd = &now
	ticket.UpdatedAt = now

	if err := u.repo.Update(ctx, ticket); err != nil {
		return apperrors.Internal("CHANGE_COMPLETE", fmt.Sprintf("failed to complete change ticket: %v", err))
	}

	// 维护模式联动：变更完成后解除维护模式
	if u.maintenanceClient != nil && len(ticket.AffectedAssets) > 0 {
		if err := u.maintenanceClient.DisableMaintenance(ctx, ticket.AffectedAssets); err != nil {
			u.logger.Warn("解除维护模式失败",
				zap.String("change_id", id),
				zap.Error(err),
			)
		}
	}

	u.logger.Info("change ticket completed", zap.String("change_id", id))

	// 记录审计日志：完成变更
	u.logAudit(ctx, "完成变更", ticket.ExecutorID, id,
		fmt.Sprintf("变更单 %s 已完成", id))

	return nil
}

// Cancel 取消变更，记录取消原因。已完成的变更不可取消。
func (u *ChangeUsecase) Cancel(ctx context.Context, id, reason string) error {
	ticket, err := u.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !CanTransition(ticket.Status, StatusCancelled) {
		return apperrors.BadRequest("CHANGE_INVALID_TRANSITION",
			fmt.Sprintf("cannot cancel change ticket in %s status", ticket.Status))
	}

	ticket.Status = StatusCancelled
	ticket.CancelReason = reason
	ticket.UpdatedAt = time.Now()

	if err := u.repo.Update(ctx, ticket); err != nil {
		return apperrors.Internal("CHANGE_CANCEL", fmt.Sprintf("failed to cancel change ticket: %v", err))
	}

	// 维护模式联动：变更取消后解除维护模式
	if u.maintenanceClient != nil && len(ticket.AffectedAssets) > 0 {
		if err := u.maintenanceClient.DisableMaintenance(ctx, ticket.AffectedAssets); err != nil {
			u.logger.Warn("解除维护模式失败",
				zap.String("change_id", id),
				zap.Error(err),
			)
		}
	}

	u.logger.Info("change ticket cancelled",
		zap.String("change_id", id),
		zap.String("reason", reason),
	)

	// 记录审计日志：取消变更
	u.logAudit(ctx, "取消变更", ticket.Requester, id,
		fmt.Sprintf("变更单 %s 已取消，原因=%s", id, reason))

	return nil
}

// CheckConflicts 检测与已有变更的时间和资产冲突。
// 冲突条件：计划时间段有重叠 AND 影响的资产有交集。
func (u *ChangeUsecase) CheckConflicts(ctx context.Context, start, end time.Time, assets []string, excludeID string) ([]*ChangeTicket, error) {
	conflicts, err := u.repo.FindConflicts(ctx, start, end, assets, excludeID)
	if err != nil {
		return nil, apperrors.Internal("CHANGE_CONFLICT_CHECK", fmt.Sprintf("failed to check conflicts: %v", err))
	}
	return conflicts, nil
}

// GetCalendar 获取未来指定天数内的变更排期日历。
func (u *ChangeUsecase) GetCalendar(ctx context.Context, days int) ([]*ChangeTicket, error) {
	now := time.Now()
	end := now.AddDate(0, 0, days)
	tickets, err := u.repo.ListByTimeRange(ctx, now, end)
	if err != nil {
		return nil, apperrors.Internal("CHANGE_CALENDAR", fmt.Sprintf("failed to get change calendar: %v", err))
	}
	return tickets, nil
}
