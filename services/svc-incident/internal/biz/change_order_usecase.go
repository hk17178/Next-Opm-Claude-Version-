package biz

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
)

// ChangeOrderRepo 定义变更工单持久化接口，由数据层实现。
type ChangeOrderRepo interface {
	NextID(ctx context.Context) (string, error)
	Create(ctx context.Context, co *ChangeOrder) error
	GetByID(ctx context.Context, id string) (*ChangeOrder, error)
	Update(ctx context.Context, co *ChangeOrder) error
	List(ctx context.Context, status string, page, pageSize int) ([]*ChangeOrder, int64, error)
}

// ChangeOrderUsecase 编排变更工单的生命周期操作，包括创建、状态变更和事件关联。
type ChangeOrderUsecase struct {
	repo     ChangeOrderRepo
	incRepo  IncidentRepo
	logger   *zap.Logger
}

// NewChangeOrderUsecase 创建变更工单用例实例。
func NewChangeOrderUsecase(repo ChangeOrderRepo, incRepo IncidentRepo, logger *zap.Logger) *ChangeOrderUsecase {
	return &ChangeOrderUsecase{
		repo:    repo,
		incRepo: incRepo,
		logger:  logger,
	}
}

// CreateChangeOrder 创建新的变更工单，自动生成唯一 ID。
func (uc *ChangeOrderUsecase) CreateChangeOrder(ctx context.Context, co *ChangeOrder) error {
	id, err := uc.repo.NextID(ctx)
	if err != nil {
		return apperrors.Internal("CHANGE_ORDER_ID", fmt.Sprintf("failed to generate change order ID: %v", err))
	}

	now := time.Now()
	co.ChangeID = id
	if co.Status == "" {
		co.Status = "draft"
	}
	if co.RelatedIncidents == nil {
		co.RelatedIncidents = []string{}
	}
	co.CreatedAt = now
	co.UpdatedAt = now

	if err := uc.repo.Create(ctx, co); err != nil {
		return apperrors.Internal("CHANGE_ORDER_CREATE", fmt.Sprintf("failed to create change order: %v", err))
	}

	uc.logger.Info("change order created",
		zap.String("change_id", co.ChangeID),
		zap.String("change_type", co.ChangeType),
		zap.String("risk_level", co.RiskLevel),
	)

	return nil
}

// GetChangeOrder 根据变更工单 ID 查询单条记录。
func (uc *ChangeOrderUsecase) GetChangeOrder(ctx context.Context, id string) (*ChangeOrder, error) {
	co, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperrors.Internal("CHANGE_ORDER_GET", fmt.Sprintf("failed to get change order: %v", err))
	}
	if co == nil {
		return nil, apperrors.NotFound(apperrors.ErrChangeOrderNotFound, fmt.Sprintf("change order %s not found", id))
	}
	return co, nil
}

// ListChangeOrders 返回按状态过滤和分页的变更工单列表。
func (uc *ChangeOrderUsecase) ListChangeOrders(ctx context.Context, status string, page, pageSize int) ([]*ChangeOrder, int64, error) {
	return uc.repo.List(ctx, status, page, pageSize)
}

// UpdateChangeOrder 更新变更工单的指定字段。
func (uc *ChangeOrderUsecase) UpdateChangeOrder(ctx context.Context, id string, updates map[string]any) (*ChangeOrder, error) {
	co, err := uc.GetChangeOrder(ctx, id)
	if err != nil {
		return nil, err
	}

	if v, ok := updates["status"].(string); ok {
		co.Status = v
	}
	if v, ok := updates["title"].(string); ok {
		co.Title = v
	}
	if v, ok := updates["risk_level"].(string); ok {
		co.RiskLevel = v
	}
	if v, ok := updates["result"]; ok {
		co.Result = v
	}

	if err := uc.repo.Update(ctx, co); err != nil {
		return nil, apperrors.Internal("CHANGE_ORDER_UPDATE", fmt.Sprintf("failed to update change order: %v", err))
	}

	return co, nil
}

// LinkIncidentToChangeOrder 将事件关联到变更工单，在双方都记录关联关系。
func (uc *ChangeOrderUsecase) LinkIncidentToChangeOrder(ctx context.Context, changeID, incidentID string) error {
	co, err := uc.GetChangeOrder(ctx, changeID)
	if err != nil {
		return err
	}

	// 验证事件存在
	inc, err := uc.incRepo.GetByID(ctx, incidentID)
	if err != nil {
		return apperrors.Internal("INCIDENT_GET", fmt.Sprintf("failed to get incident: %v", err))
	}
	if inc == nil {
		return apperrors.NotFound(apperrors.ErrIncidentNotFound, fmt.Sprintf("incident %s not found", incidentID))
	}

	// 避免重复关联
	for _, rid := range co.RelatedIncidents {
		if rid == incidentID {
			return nil
		}
	}

	co.RelatedIncidents = append(co.RelatedIncidents, incidentID)
	if err := uc.repo.Update(ctx, co); err != nil {
		return apperrors.Internal("CHANGE_ORDER_LINK", fmt.Sprintf("failed to link incident: %v", err))
	}

	uc.logger.Info("incident linked to change order",
		zap.String("change_id", changeID),
		zap.String("incident_id", incidentID),
	)

	return nil
}
