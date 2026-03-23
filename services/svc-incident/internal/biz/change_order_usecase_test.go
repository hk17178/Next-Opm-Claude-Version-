package biz

import (
	"context"
	"fmt"
	"testing"

	"go.uber.org/zap"
)

// --- Mock ChangeOrderRepo ---

type mockChangeOrderRepo struct {
	orders    map[string]*ChangeOrder
	nextIDSeq int
}

func newMockChangeOrderRepo() *mockChangeOrderRepo {
	return &mockChangeOrderRepo{orders: make(map[string]*ChangeOrder)}
}

func (m *mockChangeOrderRepo) NextID(_ context.Context) (string, error) {
	m.nextIDSeq++
	return fmt.Sprintf("CHG-TEST-%03d", m.nextIDSeq), nil
}

func (m *mockChangeOrderRepo) Create(_ context.Context, co *ChangeOrder) error {
	m.orders[co.ChangeID] = co
	return nil
}

func (m *mockChangeOrderRepo) GetByID(_ context.Context, id string) (*ChangeOrder, error) {
	co, ok := m.orders[id]
	if !ok {
		return nil, nil
	}
	return co, nil
}

func (m *mockChangeOrderRepo) Update(_ context.Context, co *ChangeOrder) error {
	m.orders[co.ChangeID] = co
	return nil
}

func (m *mockChangeOrderRepo) List(_ context.Context, status string, page, pageSize int) ([]*ChangeOrder, int64, error) {
	var result []*ChangeOrder
	for _, co := range m.orders {
		if status != "" && co.Status != status {
			continue
		}
		result = append(result, co)
	}
	return result, int64(len(result)), nil
}

func newTestChangeOrderUsecase() (*ChangeOrderUsecase, *mockChangeOrderRepo, *mockIncidentRepo) {
	coRepo := newMockChangeOrderRepo()
	incRepo := newMockIncidentRepo()
	log := zap.NewNop()
	return NewChangeOrderUsecase(coRepo, incRepo, log), coRepo, incRepo
}

// --- CreateChangeOrder Tests ---

func TestCreateChangeOrder_Success(t *testing.T) {
	uc, repo, _ := newTestChangeOrderUsecase()
	ctx := context.Background()

	co := &ChangeOrder{
		Title:      "Deploy v2.0",
		ChangeType: "standard",
		RiskLevel:  "low",
		Plan:       map[string]any{"steps": []string{"build", "test", "deploy"}},
	}
	if err := uc.CreateChangeOrder(ctx, co); err != nil {
		t.Fatalf("CreateChangeOrder error: %v", err)
	}

	if co.ChangeID == "" {
		t.Error("change_id should be set")
	}
	if co.Status != "draft" {
		t.Errorf("default status = %s, want draft", co.Status)
	}
	if co.RelatedIncidents == nil {
		t.Error("related_incidents should be initialized")
	}
	if len(repo.orders) != 1 {
		t.Errorf("repo has %d orders, want 1", len(repo.orders))
	}
}

// --- GetChangeOrder Tests ---

func TestGetChangeOrder_NotFound(t *testing.T) {
	uc, _, _ := newTestChangeOrderUsecase()
	ctx := context.Background()

	_, err := uc.GetChangeOrder(ctx, "CHG-NONEXISTENT")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestGetChangeOrder_Found(t *testing.T) {
	uc, _, _ := newTestChangeOrderUsecase()
	ctx := context.Background()

	co := &ChangeOrder{
		Title:      "Test",
		ChangeType: "normal",
		RiskLevel:  "medium",
		Plan:       map[string]any{},
	}
	uc.CreateChangeOrder(ctx, co)

	found, err := uc.GetChangeOrder(ctx, co.ChangeID)
	if err != nil {
		t.Fatalf("GetChangeOrder error: %v", err)
	}
	if found.ChangeID != co.ChangeID {
		t.Errorf("got %s, want %s", found.ChangeID, co.ChangeID)
	}
}

// --- UpdateChangeOrder Tests ---

func TestUpdateChangeOrder_Status(t *testing.T) {
	uc, _, _ := newTestChangeOrderUsecase()
	ctx := context.Background()

	co := &ChangeOrder{
		Title:      "Deploy",
		ChangeType: "standard",
		RiskLevel:  "low",
		Plan:       map[string]any{},
	}
	uc.CreateChangeOrder(ctx, co)

	updated, err := uc.UpdateChangeOrder(ctx, co.ChangeID, map[string]any{
		"status": "approved",
	})
	if err != nil {
		t.Fatalf("UpdateChangeOrder error: %v", err)
	}
	if updated.Status != "approved" {
		t.Errorf("status = %s, want approved", updated.Status)
	}
}

// --- LinkIncidentToChangeOrder Tests ---

func TestLinkIncidentToChangeOrder_Success(t *testing.T) {
	uc, _, incRepo := newTestChangeOrderUsecase()
	ctx := context.Background()

	co := &ChangeOrder{
		Title:      "Deploy",
		ChangeType: "standard",
		RiskLevel:  "low",
		Plan:       map[string]any{},
	}
	uc.CreateChangeOrder(ctx, co)

	// Create a test incident
	incRepo.incidents["INC-TEST-001"] = &Incident{
		IncidentID: "INC-TEST-001",
		Title:      "Related incident",
		Severity:   SeverityP2,
		Status:     StatusCreated,
	}

	err := uc.LinkIncidentToChangeOrder(ctx, co.ChangeID, "INC-TEST-001")
	if err != nil {
		t.Fatalf("LinkIncidentToChangeOrder error: %v", err)
	}

	// Verify the link
	linked, _ := uc.GetChangeOrder(ctx, co.ChangeID)
	if len(linked.RelatedIncidents) != 1 {
		t.Errorf("related_incidents count = %d, want 1", len(linked.RelatedIncidents))
	}
	if linked.RelatedIncidents[0] != "INC-TEST-001" {
		t.Errorf("related_incidents[0] = %s, want INC-TEST-001", linked.RelatedIncidents[0])
	}
}

func TestLinkIncidentToChangeOrder_DuplicateIgnored(t *testing.T) {
	uc, _, incRepo := newTestChangeOrderUsecase()
	ctx := context.Background()

	co := &ChangeOrder{
		Title:      "Deploy",
		ChangeType: "standard",
		RiskLevel:  "low",
		Plan:       map[string]any{},
	}
	uc.CreateChangeOrder(ctx, co)

	incRepo.incidents["INC-TEST-001"] = &Incident{
		IncidentID: "INC-TEST-001",
		Title:      "Related",
		Severity:   SeverityP2,
		Status:     StatusCreated,
	}

	// Link twice
	uc.LinkIncidentToChangeOrder(ctx, co.ChangeID, "INC-TEST-001")
	uc.LinkIncidentToChangeOrder(ctx, co.ChangeID, "INC-TEST-001")

	linked, _ := uc.GetChangeOrder(ctx, co.ChangeID)
	if len(linked.RelatedIncidents) != 1 {
		t.Errorf("related_incidents count = %d, want 1 (duplicate should be ignored)", len(linked.RelatedIncidents))
	}
}

func TestLinkIncidentToChangeOrder_ChangeNotFound(t *testing.T) {
	uc, _, _ := newTestChangeOrderUsecase()
	ctx := context.Background()

	err := uc.LinkIncidentToChangeOrder(ctx, "CHG-NONEXISTENT", "INC-001")
	if err == nil {
		t.Fatal("expected error for non-existent change order")
	}
}

func TestLinkIncidentToChangeOrder_IncidentNotFound(t *testing.T) {
	uc, _, _ := newTestChangeOrderUsecase()
	ctx := context.Background()

	co := &ChangeOrder{
		Title:      "Deploy",
		ChangeType: "standard",
		RiskLevel:  "low",
		Plan:       map[string]any{},
	}
	uc.CreateChangeOrder(ctx, co)

	err := uc.LinkIncidentToChangeOrder(ctx, co.ChangeID, "INC-NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for non-existent incident")
	}
}

// --- ListChangeOrders Tests ---

func TestListChangeOrders_FilterByStatus(t *testing.T) {
	uc, _, _ := newTestChangeOrderUsecase()
	ctx := context.Background()

	uc.CreateChangeOrder(ctx, &ChangeOrder{Title: "A", ChangeType: "standard", RiskLevel: "low", Plan: map[string]any{}})
	uc.CreateChangeOrder(ctx, &ChangeOrder{Title: "B", ChangeType: "normal", RiskLevel: "medium", Plan: map[string]any{}})

	// Both should be in "draft" status
	orders, total, err := uc.ListChangeOrders(ctx, "draft", 1, 10)
	if err != nil {
		t.Fatalf("ListChangeOrders error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(orders) != 2 {
		t.Errorf("len(orders) = %d, want 2", len(orders))
	}

	// Filter by non-existent status
	orders, total, _ = uc.ListChangeOrders(ctx, "approved", 1, 10)
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}
