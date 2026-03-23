package biz

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// DashboardRepo defines the data access interface for dashboards (PostgreSQL).
type DashboardRepo interface {
	Create(ctx context.Context, d *Dashboard) error
	Get(ctx context.Context, id string) (*Dashboard, error)
	List(ctx context.Context, filter DashboardListFilter) ([]*Dashboard, string, error)
	Update(ctx context.Context, d *Dashboard) error
	Delete(ctx context.Context, id string) error
}

// DashboardUsecase implements dashboard management.
type DashboardUsecase struct {
	repo   DashboardRepo
	logger *zap.Logger
}

// NewDashboardUsecase creates a new dashboard use case.
func NewDashboardUsecase(repo DashboardRepo, logger *zap.Logger) *DashboardUsecase {
	return &DashboardUsecase{repo: repo, logger: logger}
}

// Create stores a new dashboard.
func (uc *DashboardUsecase) Create(ctx context.Context, d *Dashboard) error {
	if d.Name == "" {
		return fmt.Errorf("dashboard name is required")
	}
	return uc.repo.Create(ctx, d)
}

// Get returns a dashboard by ID.
func (uc *DashboardUsecase) Get(ctx context.Context, id string) (*Dashboard, error) {
	return uc.repo.Get(ctx, id)
}

// List returns dashboards with cursor-based pagination.
func (uc *DashboardUsecase) List(ctx context.Context, filter DashboardListFilter) ([]*Dashboard, string, error) {
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	return uc.repo.List(ctx, filter)
}

// Update modifies an existing dashboard.
func (uc *DashboardUsecase) Update(ctx context.Context, d *Dashboard) error {
	return uc.repo.Update(ctx, d)
}

// Delete removes a dashboard.
func (uc *DashboardUsecase) Delete(ctx context.Context, id string) error {
	return uc.repo.Delete(ctx, id)
}
