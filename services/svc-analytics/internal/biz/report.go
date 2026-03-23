package biz

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ReportRepo defines the data access interface for reports (PostgreSQL).
type ReportRepo interface {
	Create(ctx context.Context, r *Report) error
	Get(ctx context.Context, id string) (*Report, error)
	List(ctx context.Context, filter ReportListFilter) ([]*Report, int, error)
	Update(ctx context.Context, r *Report) error
	Delete(ctx context.Context, id string) error
}

// ReportUsecase implements custom report management and execution.
type ReportUsecase struct {
	reportRepo  ReportRepo
	metricsRepo MetricsRepo
	logger      *zap.Logger
}

// NewReportUsecase creates a new report use case.
func NewReportUsecase(reportRepo ReportRepo, metricsRepo MetricsRepo, logger *zap.Logger) *ReportUsecase {
	return &ReportUsecase{
		reportRepo:  reportRepo,
		metricsRepo: metricsRepo,
		logger:      logger,
	}
}

// Create stores a new report definition.
func (uc *ReportUsecase) Create(ctx context.Context, r *Report) error {
	if r.Name == "" {
		return fmt.Errorf("report name is required")
	}
	if r.Schedule == "" {
		return fmt.Errorf("schedule is required")
	}
	if r.Query == "" {
		return fmt.Errorf("query is required")
	}
	if r.Format == "" {
		r.Format = ReportFormatJSON
	}
	r.Status = ReportStatusPending
	return uc.reportRepo.Create(ctx, r)
}

// Get returns a report by ID.
func (uc *ReportUsecase) Get(ctx context.Context, id string) (*Report, error) {
	return uc.reportRepo.Get(ctx, id)
}

// List returns paginated reports.
func (uc *ReportUsecase) List(ctx context.Context, filter ReportListFilter) ([]*Report, int, error) {
	return uc.reportRepo.List(ctx, filter)
}

// Delete removes a report definition.
func (uc *ReportUsecase) Delete(ctx context.Context, id string) error {
	return uc.reportRepo.Delete(ctx, id)
}

// Run triggers a report execution. Returns 202 (accepted) semantics.
func (uc *ReportUsecase) Run(ctx context.Context, reportID string) error {
	r, err := uc.reportRepo.Get(ctx, reportID)
	if err != nil {
		return fmt.Errorf("get report: %w", err)
	}

	r.Status = ReportStatusRunning
	_ = uc.reportRepo.Update(ctx, r)

	// Execute the query against ClickHouse.
	_, err = uc.metricsRepo.ExecuteQuery(ctx, r.Query, nil, 0)
	if err != nil {
		r.Status = ReportStatusFailed
		_ = uc.reportRepo.Update(ctx, r)
		return fmt.Errorf("execute report query: %w", err)
	}

	now := time.Now()
	r.Status = ReportStatusCompleted
	r.LastRunAt = &now
	_ = uc.reportRepo.Update(ctx, r)

	return nil
}
