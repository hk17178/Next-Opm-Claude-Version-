package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/opsnexus/opsnexus/pkg/event"
	"github.com/opsnexus/svc-incident/internal/biz"
)

// --- Mock Repos ---

type stubIncidentRepo struct {
	incidents map[string]*biz.Incident
	nextIDSeq int
}

func newStubIncidentRepo() *stubIncidentRepo {
	return &stubIncidentRepo{incidents: make(map[string]*biz.Incident)}
}

func (s *stubIncidentRepo) NextID(_ context.Context) (string, error) {
	s.nextIDSeq++
	return fmt.Sprintf("INC-HTTP-%03d", s.nextIDSeq), nil
}

func (s *stubIncidentRepo) Create(_ context.Context, inc *biz.Incident) error {
	s.incidents[inc.IncidentID] = inc
	return nil
}

func (s *stubIncidentRepo) GetByID(_ context.Context, id string) (*biz.Incident, error) {
	inc, ok := s.incidents[id]
	if !ok {
		return nil, nil
	}
	return inc, nil
}

func (s *stubIncidentRepo) Update(_ context.Context, inc *biz.Incident) error {
	s.incidents[inc.IncidentID] = inc
	return nil
}

func (s *stubIncidentRepo) List(_ context.Context, f biz.ListFilter) ([]*biz.Incident, int64, error) {
	var result []*biz.Incident
	for _, inc := range s.incidents {
		if f.Status != nil && inc.Status != *f.Status {
			continue
		}
		result = append(result, inc)
	}
	return result, int64(len(result)), nil
}

func (s *stubIncidentRepo) Delete(_ context.Context, id string) error {
	delete(s.incidents, id)
	return nil
}

// SetMTTR 实现 IncidentRepo 接口，测试桩中忽略 MTTR 写入操作。
func (s *stubIncidentRepo) SetMTTR(_ context.Context, _ string, _ int64) error {
	return nil
}

type stubTimelineRepo struct {
	entries []*biz.TimelineEntry
}

func newStubTimelineRepo() *stubTimelineRepo {
	return &stubTimelineRepo{}
}

func (s *stubTimelineRepo) Add(_ context.Context, entry *biz.TimelineEntry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func (s *stubTimelineRepo) ListByIncident(_ context.Context, incidentID string) ([]*biz.TimelineEntry, error) {
	var result []*biz.TimelineEntry
	for _, e := range s.entries {
		if e.IncidentID == incidentID {
			result = append(result, e)
		}
	}
	return result, nil
}

type stubScheduleRepo struct{}

func (s *stubScheduleRepo) FindByScope(_ context.Context, _ string) ([]*biz.OncallSchedule, error) {
	return nil, nil
}

// --- Test Setup ---

func newTestRouter() (chi.Router, *stubIncidentRepo) {
	repo := newStubIncidentRepo()
	timeline := newStubTimelineRepo()
	schedule := &stubScheduleRepo{}
	log := zap.NewNop()

	uc := biz.NewIncidentUsecase(
		repo,
		timeline,
		schedule,
		(*event.Producer)(nil),
		log,
	)

	r := chi.NewRouter()
	handler := NewHandler(uc, log)
	handler.RegisterRoutes(r)
	return r, repo
}

// helper to create an incident via the API and return its ID
func createTestIncident(t *testing.T, r chi.Router, severity string) string {
	t.Helper()
	body := fmt.Sprintf(`{"title":"Test Incident","severity":"%s","business_unit":"platform"}`, severity)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incident/incidents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("createTestIncident: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	return data["incident_id"].(string)
}

// --- POST /api/v1/incident/incidents ---

func TestCreateIncident_Success(t *testing.T) {
	r, _ := newTestRouter()

	body := `{"title":"Database down","severity":"P0","business_unit":"platform"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incident/incidents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("response data is nil")
	}
	if data["incident_id"] == nil || data["incident_id"] == "" {
		t.Error("incident_id should not be empty")
	}
	if data["status"] != "created" {
		t.Errorf("status = %v, want created", data["status"])
	}
	if data["severity"] != "P0" {
		t.Errorf("severity = %v, want P0", data["severity"])
	}
}

func TestCreateIncident_MissingTitle(t *testing.T) {
	r, _ := newTestRouter()

	body := `{"severity":"P0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incident/incidents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateIncident_MissingSeverity(t *testing.T) {
	r, _ := newTestRouter()

	body := `{"title":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incident/incidents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateIncident_InvalidJSON(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/incident/incidents", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- PATCH /api/v1/incident/incidents/{incident_id} (status change) ---

func TestUpdateStatus_ValidTransition(t *testing.T) {
	r, _ := newTestRouter()
	incID := createTestIncident(t, r, "P2")

	// created -> assigned
	body := `{"status":"assigned","note":"assigning"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/incident/incidents/"+incID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data["status"] != "assigned" {
		t.Errorf("status = %v, want assigned", data["status"])
	}
}

func TestUpdateStatus_InvalidTransition(t *testing.T) {
	r, _ := newTestRouter()
	incID := createTestIncident(t, r, "P2")

	// created -> resolved (invalid)
	body := `{"status":"resolved"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/incident/incidents/"+incID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (invalid transition)", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateStatus_NotFound(t *testing.T) {
	r, _ := newTestRouter()

	body := `{"status":"assigned"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/incident/incidents/INC-NONEXISTENT", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateStatus_MissingStatus(t *testing.T) {
	r, _ := newTestRouter()
	incID := createTestIncident(t, r, "P3")

	body := `{"note":"no status"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/incident/incidents/"+incID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (missing status)", w.Code, http.StatusBadRequest)
	}
}

// --- POST /api/v1/incident/incidents/{incident_id}/assign ---

func TestAssignIncident_Success(t *testing.T) {
	r, _ := newTestRouter()
	incID := createTestIncident(t, r, "P3")

	body := `{"assignee_id":"user-1","assignee_name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incident/incidents/"+incID+"/assign", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data["assignee_id"] != "user-1" {
		t.Errorf("assignee_id = %v, want user-1", data["assignee_id"])
	}
	if data["status"] != "assigned" {
		t.Errorf("status = %v, want assigned (auto-transition)", data["status"])
	}
}

func TestAssignIncident_MissingAssigneeID(t *testing.T) {
	r, _ := newTestRouter()
	incID := createTestIncident(t, r, "P3")

	body := `{"assignee_name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incident/incidents/"+incID+"/assign", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAssignIncident_ClosedIncident(t *testing.T) {
	r, repo := newTestRouter()

	now := time.Now()
	repo.incidents["INC-CLOSED"] = &biz.Incident{
		IncidentID: "INC-CLOSED",
		Title:      "Closed",
		Severity:   "P3",
		Status:     biz.StatusClosed,
		DetectedAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	body := `{"assignee_id":"user-1","assignee_name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incident/incidents/INC-CLOSED/assign", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (cannot assign closed incident)", w.Code, http.StatusBadRequest)
	}
}

func TestAssignIncident_NotFound(t *testing.T) {
	r, _ := newTestRouter()

	body := `{"assignee_id":"user-1","assignee_name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incident/incidents/INC-NONEXISTENT/assign", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// --- GET /api/v1/incident/incidents/{incident_id}/metrics ---

func TestGetMetrics_Success(t *testing.T) {
	r, repo := newTestRouter()

	detected := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	ack := detected.Add(5 * time.Minute)
	resolved := detected.Add(30 * time.Minute)

	repo.incidents["INC-M-001"] = &biz.Incident{
		IncidentID:     "INC-M-001",
		Title:          "Test",
		Severity:       "P2",
		Status:         biz.StatusResolved,
		DetectedAt:     detected,
		AcknowledgedAt: &ack,
		ResolvedAt:     &resolved,
		CreatedAt:      detected,
		UpdatedAt:      resolved,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incident/incidents/INC-M-001/metrics", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("response data is nil")
	}

	mtta := data["mtta_seconds"].(float64)
	if mtta != 300 {
		t.Errorf("MTTA = %v, want 300", mtta)
	}
	mttr := data["mttr_seconds"].(float64)
	if mttr != 1800 {
		t.Errorf("MTTR = %v, want 1800", mttr)
	}
}

func TestGetMetrics_NotFound(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incident/incidents/INC-NONEXISTENT/metrics", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// --- GET /api/v1/incident/incidents/{incident_id} ---

func TestGetIncident_Success(t *testing.T) {
	r, _ := newTestRouter()
	incID := createTestIncident(t, r, "P1")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incident/incidents/"+incID, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data["incident_id"] != incID {
		t.Errorf("incident_id = %v, want %s", data["incident_id"], incID)
	}
}

func TestGetIncident_NotFound(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incident/incidents/INC-NONEXISTENT", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// --- GET /api/v1/incident/incidents (list) ---

func TestListIncidents_Empty(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incident/incidents", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["total"].(float64) != 0 {
		t.Errorf("total = %v, want 0", resp["total"])
	}
}

// --- POST /api/v1/incident/incidents/{incident_id}/escalate ---

func TestEscalateIncident_Success(t *testing.T) {
	r, _ := newTestRouter()
	incID := createTestIncident(t, r, "P3")

	body := `{"severity":"P1","reason":"getting worse"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incident/incidents/"+incID+"/escalate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data["severity"] != "P1" {
		t.Errorf("severity = %v, want P1", data["severity"])
	}
}

// --- POST /api/v1/incident/incidents/{incident_id}/postmortem ---

func TestAddPostmortem_Success(t *testing.T) {
	r, _ := newTestRouter()
	incID := createTestIncident(t, r, "P0")

	// Walk to resolved: created -> assigned -> resolving -> resolved
	patchStatus := func(status string) {
		body := fmt.Sprintf(`{"status":"%s"}`, status)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/incident/incidents/"+incID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
	patchStatus("assigned")
	patchStatus("resolving")
	patchStatus("resolved")

	body := `{"root_cause":"DB pool exhausted","impact":"50% requests failed","author_id":"user-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/incident/incidents/"+incID+"/postmortem", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data["status"] != "postmortem" {
		t.Errorf("status = %v, want postmortem (auto-transition from resolved)", data["status"])
	}
}

// --- Response Content-Type ---

func TestResponseContentType(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incident/incidents", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}
}
