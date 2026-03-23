package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/opsnexus/svc-cmdb/internal/biz"
)

// --- Mock Repos ---

type stubAssetRepo struct {
	assets map[string]*biz.Asset
}

func newStubAssetRepo() *stubAssetRepo {
	return &stubAssetRepo{assets: make(map[string]*biz.Asset)}
}

func (s *stubAssetRepo) Create(_ context.Context, a *biz.Asset) error {
	if a.AssetID == "" {
		a.AssetID = "ci-test-001"
	}
	s.assets[a.AssetID] = a
	return nil
}

func (s *stubAssetRepo) GetByID(_ context.Context, id string) (*biz.Asset, error) {
	a, ok := s.assets[id]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (s *stubAssetRepo) Update(_ context.Context, a *biz.Asset) error {
	s.assets[a.AssetID] = a
	return nil
}

func (s *stubAssetRepo) Delete(_ context.Context, id string) error {
	delete(s.assets, id)
	return nil
}

func (s *stubAssetRepo) List(_ context.Context, f biz.AssetListFilter) ([]*biz.Asset, int64, error) {
	var result []*biz.Asset
	for _, a := range s.assets {
		if f.AssetType != nil && a.AssetType != *f.AssetType {
			continue
		}
		result = append(result, a)
	}
	return result, int64(len(result)), nil
}

func (s *stubAssetRepo) FindByHostnameOrIP(_ context.Context, _, _ string) (*biz.Asset, error) {
	return nil, nil
}

type stubRelationRepo struct {
	relations map[string]*biz.AssetRelation
}

func newStubRelationRepo() *stubRelationRepo {
	return &stubRelationRepo{relations: make(map[string]*biz.AssetRelation)}
}

func (s *stubRelationRepo) Create(_ context.Context, rel *biz.AssetRelation) error {
	if rel.RelationID == "" {
		rel.RelationID = "rel-test-001"
	}
	s.relations[rel.RelationID] = rel
	return nil
}

func (s *stubRelationRepo) Delete(_ context.Context, id string) error {
	delete(s.relations, id)
	return nil
}

func (s *stubRelationRepo) GetByID(_ context.Context, id string) (*biz.AssetRelation, error) {
	return s.relations[id], nil
}

func (s *stubRelationRepo) ListByAsset(_ context.Context, assetID, _ string) ([]*biz.AssetRelation, error) {
	var result []*biz.AssetRelation
	for _, r := range s.relations {
		if r.SourceAssetID == assetID || r.TargetAssetID == assetID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *stubRelationRepo) GetTopology(_ context.Context, rootID string, _ int, _ string) (*biz.TopologyGraph, error) {
	return &biz.TopologyGraph{
		Nodes: []*biz.Asset{},
		Edges: []*biz.AssetRelation{},
	}, nil
}

func (s *stubRelationRepo) GetCascadeAssets(_ context.Context, _ []string) ([]string, error) {
	return nil, nil
}

type stubGroupRepo struct{}

func (s *stubGroupRepo) Create(_ context.Context, g *biz.AssetGroup) error { return nil }
func (s *stubGroupRepo) GetByID(_ context.Context, _ string) (*biz.AssetGroup, error) {
	return nil, nil
}
func (s *stubGroupRepo) List(_ context.Context) ([]*biz.AssetGroup, error) { return nil, nil }
func (s *stubGroupRepo) Update(_ context.Context, _ *biz.AssetGroup) error  { return nil }
func (s *stubGroupRepo) Delete(_ context.Context, _ string) error           { return nil }
func (s *stubGroupRepo) EvalDynamicMembers(_ context.Context, _ *biz.AssetGroup) ([]string, error) {
	return nil, nil
}

type stubDimensionRepo struct{}

func (s *stubDimensionRepo) Create(_ context.Context, _ *biz.CustomDimension) error { return nil }
func (s *stubDimensionRepo) GetByID(_ context.Context, _ string) (*biz.CustomDimension, error) {
	return nil, nil
}
func (s *stubDimensionRepo) GetByName(_ context.Context, _ string) (*biz.CustomDimension, error) {
	return nil, nil
}
func (s *stubDimensionRepo) List(_ context.Context) ([]*biz.CustomDimension, error) { return nil, nil }
func (s *stubDimensionRepo) Update(_ context.Context, _ *biz.CustomDimension) error { return nil }
func (s *stubDimensionRepo) Delete(_ context.Context, _ string) error                { return nil }

type stubDiscoveryRepo struct{}

func (s *stubDiscoveryRepo) Create(_ context.Context, _ *biz.DiscoveryRecord) error { return nil }
func (s *stubDiscoveryRepo) GetByID(_ context.Context, _ string) (*biz.DiscoveryRecord, error) {
	return nil, nil
}
func (s *stubDiscoveryRepo) List(_ context.Context, _ string, _, _ int) ([]*biz.DiscoveryRecord, int64, error) {
	return nil, 0, nil
}
func (s *stubDiscoveryRepo) UpdateStatus(_ context.Context, _, _ string, _ *string) error {
	return nil
}

// --- Test Setup ---

func newTestRouter() (chi.Router, *stubAssetRepo) {
	assetRepo := newStubAssetRepo()
	relationRepo := newStubRelationRepo()
	log := zap.NewNop()

	uc := biz.NewAssetUsecase(
		assetRepo,
		relationRepo,
		&stubGroupRepo{},
		&stubDimensionRepo{},
		&stubDiscoveryRepo{},
		nil, // producer — nil for tests
		log,
	)

	r := chi.NewRouter()
	handler := NewHandler(uc, log)
	handler.RegisterRoutes(r)
	return r, assetRepo
}

// --- POST /api/v1/cmdb/ci ---

func TestCreateAsset_Success(t *testing.T) {
	r, _ := newTestRouter()

	body := `{"asset_type":"server","hostname":"web-01"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/ci", bytes.NewBufferString(body))
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
	if data["asset_type"] != "server" {
		t.Errorf("asset_type = %v, want server", data["asset_type"])
	}
}

func TestCreateAsset_MissingType(t *testing.T) {
	r, _ := newTestRouter()

	body := `{"hostname":"web-01"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/ci", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateAsset_InvalidJSON(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/ci", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- GET /api/v1/cmdb/ci ---

func TestListAssets_Empty(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/ci", nil)
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

func TestListAssets_WithTypeFilter(t *testing.T) {
	r, assetRepo := newTestRouter()

	// Pre-populate assets
	assetRepo.assets["a-1"] = &biz.Asset{AssetID: "a-1", AssetType: "server", Status: "active"}
	assetRepo.assets["a-2"] = &biz.Asset{AssetID: "a-2", AssetType: "database", Status: "active"}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/ci?type=server", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	total := resp["total"].(float64)
	if total != 1 {
		t.Errorf("total = %v, want 1 (filtered by type=server)", total)
	}
}

// --- GET /api/v1/cmdb/ci/{ci_id} ---

func TestGetAsset_Found(t *testing.T) {
	r, assetRepo := newTestRouter()

	hostname := "web-01"
	assetRepo.assets["ci-001"] = &biz.Asset{
		AssetID:   "ci-001",
		AssetType: "server",
		Hostname:  &hostname,
		Status:    "active",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/ci/ci-001", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data["asset_id"] != "ci-001" {
		t.Errorf("asset_id = %v, want ci-001", data["asset_id"])
	}
}

func TestGetAsset_NotFound(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/ci/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// --- PUT /api/v1/cmdb/ci/{ci_id} ---

func TestUpdateAsset_Success(t *testing.T) {
	r, assetRepo := newTestRouter()

	hostname := "web-01"
	assetRepo.assets["ci-001"] = &biz.Asset{
		AssetID:          "ci-001",
		AssetType:        "server",
		Hostname:         &hostname,
		Status:           "active",
		Tags:             map[string]string{},
		BusinessUnits:    []string{},
		CustomDimensions: map[string]any{},
	}

	body := `{"hostname":"web-02","grade":"A"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cmdb/ci/ci-001", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data["hostname"] != "web-02" {
		t.Errorf("hostname = %v, want web-02", data["hostname"])
	}
}

func TestUpdateAsset_NotFound(t *testing.T) {
	r, _ := newTestRouter()

	body := `{"hostname":"web-02"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cmdb/ci/nonexistent", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// --- DELETE /api/v1/cmdb/ci/{ci_id} ---

func TestDeleteAsset_Success(t *testing.T) {
	r, assetRepo := newTestRouter()

	assetRepo.assets["ci-001"] = &biz.Asset{
		AssetID:   "ci-001",
		AssetType: "server",
		Status:    "active",
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cmdb/ci/ci-001", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestDeleteAsset_NotFound(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cmdb/ci/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// --- POST /api/v1/cmdb/topology ---

func TestQueryTopology_Success(t *testing.T) {
	r, assetRepo := newTestRouter()

	assetRepo.assets["ci-001"] = &biz.Asset{
		AssetID:   "ci-001",
		AssetType: "server",
		Status:    "active",
	}

	body := `{"root_ci_id":"ci-001","depth":2,"direction":"both"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/topology", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
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
	if _, ok := data["nodes"]; !ok {
		t.Error("response should contain nodes")
	}
	if _, ok := data["edges"]; !ok {
		t.Error("response should contain edges")
	}
}

func TestQueryTopology_MissingRootID(t *testing.T) {
	r, _ := newTestRouter()

	body := `{"depth":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/topology", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestQueryTopology_RootNotFound(t *testing.T) {
	r, _ := newTestRouter()

	body := `{"root_ci_id":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/topology", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// --- GET /api/v1/cmdb/ci-types ---

func TestListCITypes(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/ci-types", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("response data is nil")
	}
	types, ok := data["types"].([]any)
	if !ok {
		t.Fatal("types should be an array")
	}
	if len(types) != 22 {
		t.Errorf("types count = %d, want 22", len(types))
	}

	// Verify first type has required fields
	first, _ := types[0].(map[string]any)
	if first["name"] == nil || first["display_name"] == nil || first["description"] == nil {
		t.Error("each CI type should have name, display_name, and description")
	}
}

// --- POST /api/v1/cmdb/relationships ---

func TestCreateRelation_Success(t *testing.T) {
	r, assetRepo := newTestRouter()

	assetRepo.assets["a-1"] = &biz.Asset{AssetID: "a-1", AssetType: "server", Status: "active"}
	assetRepo.assets["a-2"] = &biz.Asset{AssetID: "a-2", AssetType: "database", Status: "active"}

	body := `{"source_asset_id":"a-1","target_asset_id":"a-2","relation_type":"depends_on"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/relationships", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestCreateRelation_MissingFields(t *testing.T) {
	r, _ := newTestRouter()

	body := `{"source_asset_id":"a-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/relationships", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- GET /api/v1/cmdb/ci/{ci_id}/relationships ---

func TestGetAssetRelations(t *testing.T) {
	r, assetRepo := newTestRouter()

	assetRepo.assets["a-1"] = &biz.Asset{AssetID: "a-1", AssetType: "server", Status: "active"}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/ci/a-1/relationships", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("response data is nil")
	}
	if _, ok := data["relationships"]; !ok {
		t.Error("response should contain relationships key")
	}
}

// --- DELETE /api/v1/cmdb/relationships/{relationship_id} ---

func TestDeleteRelation(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cmdb/relationships/rel-001", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

// --- Pagination params ---

func TestListAssets_PaginationParams(t *testing.T) {
	r, assetRepo := newTestRouter()

	for i := 0; i < 5; i++ {
		id := "a-" + string(rune('0'+i))
		assetRepo.assets[id] = &biz.Asset{AssetID: id, AssetType: "server", Status: "active"}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/ci?page=1&page_size=2", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["page"].(float64) != 1 {
		t.Errorf("page = %v, want 1", resp["page"])
	}
	if resp["page_size"].(float64) != 2 {
		t.Errorf("page_size = %v, want 2", resp["page_size"])
	}
}

// --- Content-Type Header ---

func TestResponseContentType(t *testing.T) {
	r, _ := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/ci-types", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}
}
