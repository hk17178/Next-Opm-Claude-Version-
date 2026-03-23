package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/opsnexus/opsnexus/pkg/event"
)

// --- Mock Repos ---

type mockIncidentRepo struct {
	incidents map[string]*Incident
	nextIDSeq int
}

func newMockIncidentRepo() *mockIncidentRepo {
	return &mockIncidentRepo{incidents: make(map[string]*Incident)}
}

func (m *mockIncidentRepo) NextID(_ context.Context) (string, error) {
	m.nextIDSeq++
	return fmt.Sprintf("INC-TEST-%03d", m.nextIDSeq), nil
}

func (m *mockIncidentRepo) Create(_ context.Context, inc *Incident) error {
	m.incidents[inc.IncidentID] = inc
	return nil
}

func (m *mockIncidentRepo) GetByID(_ context.Context, id string) (*Incident, error) {
	inc, ok := m.incidents[id]
	if !ok {
		return nil, nil
	}
	return inc, nil
}

func (m *mockIncidentRepo) Update(_ context.Context, inc *Incident) error {
	m.incidents[inc.IncidentID] = inc
	return nil
}

func (m *mockIncidentRepo) List(_ context.Context, f ListFilter) ([]*Incident, int64, error) {
	var result []*Incident
	for _, inc := range m.incidents {
		if f.Status != nil && inc.Status != *f.Status {
			continue
		}
		if f.Severity != nil && inc.Severity != *f.Severity {
			continue
		}
		result = append(result, inc)
	}
	return result, int64(len(result)), nil
}

func (m *mockIncidentRepo) Delete(_ context.Context, id string) error {
	delete(m.incidents, id)
	return nil
}

// SetMTTR 实现 IncidentRepo 接口的 SetMTTR 方法，测试环境中将 MTTR 写入内存。
func (m *mockIncidentRepo) SetMTTR(_ context.Context, incidentID string, mttrSeconds int64) error {
	// 测试用的 mock：不做任何持久化，只验证接口调用不报错
	return nil
}

type mockTimelineRepo struct {
	entries []*TimelineEntry
}

func newMockTimelineRepo() *mockTimelineRepo {
	return &mockTimelineRepo{}
}

func (m *mockTimelineRepo) Add(_ context.Context, entry *TimelineEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockTimelineRepo) ListByIncident(_ context.Context, incidentID string) ([]*TimelineEntry, error) {
	var result []*TimelineEntry
	for _, e := range m.entries {
		if e.IncidentID == incidentID {
			result = append(result, e)
		}
	}
	return result, nil
}

type mockScheduleRepo struct{}

func (m *mockScheduleRepo) FindByScope(_ context.Context, _ string) ([]*OncallSchedule, error) {
	return nil, nil
}

func newTestUsecase() (*IncidentUsecase, *mockIncidentRepo, *mockTimelineRepo) {
	repo := newMockIncidentRepo()
	timeline := newMockTimelineRepo()
	schedule := &mockScheduleRepo{}
	log := zap.NewNop()
	// Producer is nil — we skip event publishing in tests
	uc := &IncidentUsecase{
		repo:     repo,
		timeline: timeline,
		schedule: schedule,
		producer: (*event.Producer)(nil),
		logger:   log,
	}
	return uc, repo, timeline
}

// --- CreateIncident Tests ---

func TestCreateIncident_Success(t *testing.T) {
	uc, repo, timeline := newTestUsecase()
	ctx := context.Background()

	inc, err := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title:        "Database down",
		Severity:     SeverityP0,
		BusinessUnit: "platform",
	})
	if err != nil {
		t.Fatalf("CreateIncident error: %v", err)
	}

	if inc.IncidentID == "" {
		t.Error("incident ID should not be empty")
	}
	if inc.Status != StatusCreated {
		t.Errorf("status = %s, want created", inc.Status)
	}
	if inc.Severity != SeverityP0 {
		t.Errorf("severity = %s, want P0", inc.Severity)
	}
	if len(repo.incidents) != 1 {
		t.Errorf("repo has %d incidents, want 1", len(repo.incidents))
	}
	if len(timeline.entries) == 0 {
		t.Error("timeline should have at least one entry")
	}
}

func TestCreateIncident_DefaultFields(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, err := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title:    "Test",
		Severity: SeverityP3,
	})
	if err != nil {
		t.Fatalf("CreateIncident error: %v", err)
	}

	if inc.SourceAlerts == nil {
		t.Error("source_alerts should be initialized, not nil")
	}
	if inc.AffectedAssets == nil {
		t.Error("affected_assets should be initialized, not nil")
	}
	if inc.Tags == nil {
		t.Error("tags should be initialized, not nil")
	}
	if inc.ImprovementItems == nil {
		t.Error("improvement_items should be initialized, not nil")
	}
}

// --- UpdateStatus Tests ---

func TestUpdateStatus_ValidTransition(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Test", Severity: SeverityP2,
	})

	updated, err := uc.UpdateStatus(ctx, inc.IncidentID, StatusTriaging, "starting triage")
	if err != nil {
		t.Fatalf("UpdateStatus error: %v", err)
	}
	if updated.Status != StatusTriaging {
		t.Errorf("status = %s, want triaging", updated.Status)
	}
}

func TestUpdateStatus_InvalidTransition(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Test", Severity: SeverityP2,
	})

	_, err := uc.UpdateStatus(ctx, inc.IncidentID, StatusResolved, "")
	if err == nil {
		t.Fatal("expected error for invalid transition created->resolved")
	}
}

func TestUpdateStatus_SetsAcknowledgedAt(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Test", Severity: SeverityP2,
	})

	updated, _ := uc.UpdateStatus(ctx, inc.IncidentID, StatusAssigned, "")
	if updated.AcknowledgedAt == nil {
		t.Error("AcknowledgedAt should be set when transitioning to assigned")
	}
}

func TestUpdateStatus_SetsResolvedAt(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Test", Severity: SeverityP2,
	})

	// Walk to resolving then resolved
	uc.UpdateStatus(ctx, inc.IncidentID, StatusAssigned, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolving, "")
	updated, _ := uc.UpdateStatus(ctx, inc.IncidentID, StatusResolved, "fixed")

	if updated.ResolvedAt == nil {
		t.Error("ResolvedAt should be set when transitioning to resolved")
	}
}

func TestUpdateStatus_SetsClosedAt(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Test", Severity: SeverityP3, // P3 doesn't require postmortem
	})

	uc.UpdateStatus(ctx, inc.IncidentID, StatusAssigned, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolving, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolved, "")
	updated, _ := uc.UpdateStatus(ctx, inc.IncidentID, StatusClosed, "")

	if updated.ClosedAt == nil {
		t.Error("ClosedAt should be set when closing")
	}
}

// --- Postmortem Enforcement Tests ---

func TestUpdateStatus_P0RequiresPostmortem(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Critical", Severity: SeverityP0,
	})

	uc.UpdateStatus(ctx, inc.IncidentID, StatusAssigned, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolving, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolved, "")

	// Try to close without postmortem
	_, err := uc.UpdateStatus(ctx, inc.IncidentID, StatusClosed, "")
	if err == nil {
		t.Fatal("expected error: P0 incident should require postmortem before closing")
	}
}

func TestUpdateStatus_P1RequiresPostmortem(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Critical", Severity: SeverityP1,
	})

	uc.UpdateStatus(ctx, inc.IncidentID, StatusAssigned, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolving, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolved, "")

	_, err := uc.UpdateStatus(ctx, inc.IncidentID, StatusClosed, "")
	if err == nil {
		t.Fatal("expected error: P1 incident should require postmortem before closing")
	}
}

func TestUpdateStatus_P0CanCloseWithPostmortem(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Critical", Severity: SeverityP0,
	})

	uc.UpdateStatus(ctx, inc.IncidentID, StatusAssigned, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolving, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolved, "")

	// Add postmortem
	uc.AddPostmortem(ctx, inc.IncidentID, &Postmortem{
		RootCause: "DB connection pool exhausted",
		Impact:    "50% of requests failed",
		AuthorID:  "user-123",
	})

	// Now close should work (postmortem transitions to StatusPostmortem, then close)
	_, err := uc.UpdateStatus(ctx, inc.IncidentID, StatusClosed, "")
	if err != nil {
		t.Fatalf("expected no error closing P0 with postmortem: %v", err)
	}
}

func TestUpdateStatus_P2CanCloseWithoutPostmortem(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Minor", Severity: SeverityP2,
	})

	uc.UpdateStatus(ctx, inc.IncidentID, StatusAssigned, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolving, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolved, "")

	_, err := uc.UpdateStatus(ctx, inc.IncidentID, StatusClosed, "")
	if err != nil {
		t.Fatalf("P2 should close without postmortem: %v", err)
	}
}

// --- AssignIncident Tests ---

func TestAssignIncident_AutoTransitions(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Test", Severity: SeverityP3,
	})

	updated, err := uc.AssignIncident(ctx, inc.IncidentID, "user-1", "Alice")
	if err != nil {
		t.Fatalf("AssignIncident error: %v", err)
	}

	if updated.Status != StatusAssigned {
		t.Errorf("status = %s, want assigned (should auto-transition from created)", updated.Status)
	}
	if *updated.AssigneeID != "user-1" {
		t.Errorf("assignee_id = %s, want user-1", *updated.AssigneeID)
	}
	if updated.AcknowledgedAt == nil {
		t.Error("AcknowledgedAt should be set on assignment")
	}
}

func TestAssignIncident_CannotAssignClosed(t *testing.T) {
	uc, repo, _ := newTestUsecase()
	ctx := context.Background()

	// Create a closed incident directly
	inc := &Incident{
		IncidentID: "INC-CLOSED-001",
		Title:      "Closed",
		Severity:   SeverityP3,
		Status:     StatusClosed,
		DetectedAt: time.Now(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	repo.incidents[inc.IncidentID] = inc

	_, err := uc.AssignIncident(ctx, inc.IncidentID, "user-1", "Alice")
	if err == nil {
		t.Fatal("expected error: cannot assign a closed incident")
	}
}

// --- EscalateIncident Tests ---

func TestEscalateIncident(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Minor issue", Severity: SeverityP3,
	})

	updated, err := uc.EscalateIncident(ctx, inc.IncidentID, SeverityP1, "getting worse")
	if err != nil {
		t.Fatalf("EscalateIncident error: %v", err)
	}
	if updated.Severity != SeverityP1 {
		t.Errorf("severity = %s, want P1", updated.Severity)
	}
}

// --- AddPostmortem Tests ---

func TestAddPostmortem_TransitionsToPostmortemStatus(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	inc, _ := uc.CreateIncident(ctx, CreateIncidentRequest{
		Title: "Test", Severity: SeverityP0,
	})

	uc.UpdateStatus(ctx, inc.IncidentID, StatusAssigned, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolving, "")
	uc.UpdateStatus(ctx, inc.IncidentID, StatusResolved, "")

	updated, err := uc.AddPostmortem(ctx, inc.IncidentID, &Postmortem{
		RootCause: "Memory leak",
		AuthorID:  "user-1",
	})
	if err != nil {
		t.Fatalf("AddPostmortem error: %v", err)
	}
	if updated.Status != StatusPostmortem {
		t.Errorf("status = %s, want postmortem (should auto-transition from resolved)", updated.Status)
	}
	if updated.Postmortem == nil {
		t.Error("postmortem should be set")
	}
}

// --- GetMetrics Tests ---

func TestGetMetrics(t *testing.T) {
	uc, repo, _ := newTestUsecase()
	ctx := context.Background()

	detected := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	ack := detected.Add(2 * time.Minute)
	resolved := detected.Add(20 * time.Minute)

	inc := &Incident{
		IncidentID:     "INC-METRICS-001",
		Title:          "Test",
		Severity:       SeverityP2,
		Status:         StatusResolved,
		DetectedAt:     detected,
		AcknowledgedAt: &ack,
		ResolvedAt:     &resolved,
		CreatedAt:      detected,
		UpdatedAt:      resolved,
	}
	repo.incidents[inc.IncidentID] = inc

	m, err := uc.GetMetrics(ctx, inc.IncidentID)
	if err != nil {
		t.Fatalf("GetMetrics error: %v", err)
	}
	if m.MTTASeconds == nil || *m.MTTASeconds != 120 {
		t.Errorf("MTTA = %v, want 120", m.MTTASeconds)
	}
	if m.MTTRSeconds == nil || *m.MTTRSeconds != 1200 {
		t.Errorf("MTTR = %v, want 1200", m.MTTRSeconds)
	}
}

// --- GetIncident Not Found ---

func TestGetIncident_NotFound(t *testing.T) {
	uc, _, _ := newTestUsecase()
	ctx := context.Background()

	_, err := uc.GetIncident(ctx, "INC-NONEXISTENT")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

// --- Description ---

func TestDescription_field(t *testing.T) {
	inc := &Incident{
		IncidentID: "INC-001",
		Title:      "Test",
	}
	// Description should be zero value by default
	if inc.Description != "" {
		t.Errorf("description = %q, want empty", inc.Description)
	}
}
