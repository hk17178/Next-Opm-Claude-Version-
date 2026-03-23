package biz

import (
	"context"
	"fmt"
	"testing"

	"go.uber.org/zap"

	"github.com/opsnexus/opsnexus/pkg/event"
)

// --- Mock Repos ---

type mockAssetRepo struct {
	assets map[string]*Asset
	seq    int
}

func newMockAssetRepo() *mockAssetRepo {
	return &mockAssetRepo{assets: make(map[string]*Asset)}
}

func (m *mockAssetRepo) Create(_ context.Context, a *Asset) error {
	m.seq++
	if a.AssetID == "" {
		a.AssetID = fmt.Sprintf("asset-%03d", m.seq)
	}
	m.assets[a.AssetID] = a
	return nil
}

func (m *mockAssetRepo) GetByID(_ context.Context, id string) (*Asset, error) {
	a, ok := m.assets[id]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (m *mockAssetRepo) Update(_ context.Context, a *Asset) error {
	m.assets[a.AssetID] = a
	return nil
}

func (m *mockAssetRepo) Delete(_ context.Context, id string) error {
	delete(m.assets, id)
	return nil
}

func (m *mockAssetRepo) List(_ context.Context, f AssetListFilter) ([]*Asset, int64, error) {
	var result []*Asset
	for _, a := range m.assets {
		if f.AssetType != nil && a.AssetType != *f.AssetType {
			continue
		}
		if f.Status != nil && a.Status != *f.Status {
			continue
		}
		if f.Grade != nil && (a.Grade == nil || *a.Grade != *f.Grade) {
			continue
		}
		result = append(result, a)
	}
	return result, int64(len(result)), nil
}

func (m *mockAssetRepo) FindByHostnameOrIP(_ context.Context, hostname, ip string) (*Asset, error) {
	for _, a := range m.assets {
		if a.Hostname != nil && *a.Hostname == hostname {
			return a, nil
		}
		if a.IP != nil && *a.IP == ip {
			return a, nil
		}
	}
	return nil, nil
}

type mockRelationRepo struct {
	relations   map[string]*AssetRelation
	topology    *TopologyGraph
	cascadeIDs  []string
}

func newMockRelationRepo() *mockRelationRepo {
	return &mockRelationRepo{relations: make(map[string]*AssetRelation)}
}

func (m *mockRelationRepo) Create(_ context.Context, rel *AssetRelation) error {
	if rel.RelationID == "" {
		rel.RelationID = fmt.Sprintf("rel-%03d", len(m.relations)+1)
	}
	m.relations[rel.RelationID] = rel
	return nil
}

func (m *mockRelationRepo) Delete(_ context.Context, id string) error {
	delete(m.relations, id)
	return nil
}

func (m *mockRelationRepo) GetByID(_ context.Context, id string) (*AssetRelation, error) {
	rel, ok := m.relations[id]
	if !ok {
		return nil, nil
	}
	return rel, nil
}

func (m *mockRelationRepo) ListByAsset(_ context.Context, assetID, direction string) ([]*AssetRelation, error) {
	var result []*AssetRelation
	for _, rel := range m.relations {
		switch direction {
		case "upstream":
			if rel.TargetAssetID == assetID {
				result = append(result, rel)
			}
		case "downstream":
			if rel.SourceAssetID == assetID {
				result = append(result, rel)
			}
		default: // both
			if rel.SourceAssetID == assetID || rel.TargetAssetID == assetID {
				result = append(result, rel)
			}
		}
	}
	return result, nil
}

func (m *mockRelationRepo) GetTopology(_ context.Context, rootID string, depth int, direction string) (*TopologyGraph, error) {
	if m.topology != nil {
		return m.topology, nil
	}
	return &TopologyGraph{Nodes: []*Asset{}, Edges: []*AssetRelation{}}, nil
}

func (m *mockRelationRepo) GetCascadeAssets(_ context.Context, assetIDs []string) ([]string, error) {
	return m.cascadeIDs, nil
}

type mockGroupRepo struct {
	groups  map[string]*AssetGroup
	members []string
}

func newMockGroupRepo() *mockGroupRepo {
	return &mockGroupRepo{groups: make(map[string]*AssetGroup)}
}

func (m *mockGroupRepo) Create(_ context.Context, g *AssetGroup) error {
	if g.GroupID == "" {
		g.GroupID = fmt.Sprintf("grp-%03d", len(m.groups)+1)
	}
	m.groups[g.GroupID] = g
	return nil
}

func (m *mockGroupRepo) GetByID(_ context.Context, id string) (*AssetGroup, error) {
	g, ok := m.groups[id]
	if !ok {
		return nil, nil
	}
	return g, nil
}

func (m *mockGroupRepo) List(_ context.Context) ([]*AssetGroup, error) {
	var result []*AssetGroup
	for _, g := range m.groups {
		result = append(result, g)
	}
	return result, nil
}

func (m *mockGroupRepo) Update(_ context.Context, g *AssetGroup) error {
	m.groups[g.GroupID] = g
	return nil
}

func (m *mockGroupRepo) Delete(_ context.Context, id string) error {
	delete(m.groups, id)
	return nil
}

func (m *mockGroupRepo) EvalDynamicMembers(_ context.Context, g *AssetGroup) ([]string, error) {
	if m.members != nil {
		return m.members, nil
	}
	return g.StaticMembers, nil
}

type mockDimensionRepo struct {
	dims map[string]*CustomDimension
}

func newMockDimensionRepo() *mockDimensionRepo {
	return &mockDimensionRepo{dims: make(map[string]*CustomDimension)}
}

func (m *mockDimensionRepo) Create(_ context.Context, d *CustomDimension) error {
	if d.DimID == "" {
		d.DimID = fmt.Sprintf("dim-%03d", len(m.dims)+1)
	}
	m.dims[d.DimID] = d
	return nil
}

func (m *mockDimensionRepo) GetByID(_ context.Context, id string) (*CustomDimension, error) {
	d, ok := m.dims[id]
	if !ok {
		return nil, nil
	}
	return d, nil
}

func (m *mockDimensionRepo) GetByName(_ context.Context, name string) (*CustomDimension, error) {
	for _, d := range m.dims {
		if d.Name == name {
			return d, nil
		}
	}
	return nil, nil
}

func (m *mockDimensionRepo) List(_ context.Context) ([]*CustomDimension, error) {
	var result []*CustomDimension
	for _, d := range m.dims {
		result = append(result, d)
	}
	return result, nil
}

func (m *mockDimensionRepo) Update(_ context.Context, d *CustomDimension) error {
	m.dims[d.DimID] = d
	return nil
}

func (m *mockDimensionRepo) Delete(_ context.Context, id string) error {
	delete(m.dims, id)
	return nil
}

type mockDiscoveryRepo struct {
	records map[string]*DiscoveryRecord
}

func newMockDiscoveryRepo() *mockDiscoveryRepo {
	return &mockDiscoveryRepo{records: make(map[string]*DiscoveryRecord)}
}

func (m *mockDiscoveryRepo) Create(_ context.Context, d *DiscoveryRecord) error {
	m.records[d.RecordID] = d
	return nil
}

func (m *mockDiscoveryRepo) GetByID(_ context.Context, id string) (*DiscoveryRecord, error) {
	d, ok := m.records[id]
	if !ok {
		return nil, nil
	}
	return d, nil
}

func (m *mockDiscoveryRepo) List(_ context.Context, status string, page, pageSize int) ([]*DiscoveryRecord, int64, error) {
	var result []*DiscoveryRecord
	for _, d := range m.records {
		if status != "" && d.Status != status {
			continue
		}
		result = append(result, d)
	}
	return result, int64(len(result)), nil
}

func (m *mockDiscoveryRepo) UpdateStatus(_ context.Context, id, status string, matchedAssetID *string) error {
	if d, ok := m.records[id]; ok {
		d.Status = status
		d.MatchedAssetID = matchedAssetID
	}
	return nil
}

func newTestCMDBUsecase() (*AssetUsecase, *mockAssetRepo, *mockRelationRepo, *mockGroupRepo, *mockDimensionRepo, *mockDiscoveryRepo) {
	assetRepo := newMockAssetRepo()
	relationRepo := newMockRelationRepo()
	groupRepo := newMockGroupRepo()
	dimRepo := newMockDimensionRepo()
	discoveryRepo := newMockDiscoveryRepo()
	log := zap.NewNop()

	uc := &AssetUsecase{
		assets:     assetRepo,
		relations:  relationRepo,
		groups:     groupRepo,
		dimensions: dimRepo,
		discovery:  discoveryRepo,
		producer:   (*event.Producer)(nil),
		logger:     log,
	}
	return uc, assetRepo, relationRepo, groupRepo, dimRepo, discoveryRepo
}

// --- CreateAsset Tests ---

func TestCreateAsset_Success(t *testing.T) {
	uc, repo, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	a := &Asset{AssetType: AssetTypeServer}
	if err := uc.CreateAsset(ctx, a); err != nil {
		t.Fatalf("CreateAsset error: %v", err)
	}

	if a.AssetID == "" {
		t.Error("asset_id should be set")
	}
	if a.Status != StatusActive {
		t.Errorf("default status = %s, want active", a.Status)
	}
	if a.Tags == nil {
		t.Error("tags should be initialized")
	}
	if a.CustomDimensions == nil {
		t.Error("custom_dimensions should be initialized")
	}
	if a.BusinessUnits == nil {
		t.Error("business_units should be initialized")
	}
	if a.DiscoveredBy == nil || *a.DiscoveredBy != DiscoveryManual {
		t.Error("discovered_by should default to manual")
	}
	if len(repo.assets) != 1 {
		t.Errorf("repo has %d assets, want 1", len(repo.assets))
	}
}

// --- GetAsset Tests ---

func TestGetAsset_NotFound(t *testing.T) {
	uc, _, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	_, err := uc.GetAsset(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestGetAsset_Found(t *testing.T) {
	uc, _, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	a := &Asset{AssetType: AssetTypeDatabase}
	uc.CreateAsset(ctx, a)

	found, err := uc.GetAsset(ctx, a.AssetID)
	if err != nil {
		t.Fatalf("GetAsset error: %v", err)
	}
	if found.AssetID != a.AssetID {
		t.Errorf("got asset %s, want %s", found.AssetID, a.AssetID)
	}
}

// --- UpdateAsset Tests ---

func TestUpdateAsset_FieldTracking(t *testing.T) {
	uc, _, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	hostname := "web-01"
	a := &Asset{AssetType: AssetTypeServer, Hostname: &hostname}
	uc.CreateAsset(ctx, a)

	updated, err := uc.UpdateAsset(ctx, a.AssetID, map[string]any{
		"hostname": "web-02",
		"grade":    GradeA,
		"status":   StatusMaintenance,
	})
	if err != nil {
		t.Fatalf("UpdateAsset error: %v", err)
	}

	if *updated.Hostname != "web-02" {
		t.Errorf("hostname = %s, want web-02", *updated.Hostname)
	}
	if *updated.Grade != GradeA {
		t.Errorf("grade = %s, want A", *updated.Grade)
	}
	if updated.Status != StatusMaintenance {
		t.Errorf("status = %s, want maintenance", updated.Status)
	}
}

func TestUpdateAsset_NotFound(t *testing.T) {
	uc, _, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	_, err := uc.UpdateAsset(ctx, "nonexistent", map[string]any{"hostname": "x"})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

// --- DeleteAsset Tests ---

func TestDeleteAsset_Success(t *testing.T) {
	uc, repo, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	a := &Asset{AssetType: AssetTypeServer}
	uc.CreateAsset(ctx, a)

	if err := uc.DeleteAsset(ctx, a.AssetID); err != nil {
		t.Fatalf("DeleteAsset error: %v", err)
	}
	if len(repo.assets) != 0 {
		t.Errorf("repo still has %d assets after delete", len(repo.assets))
	}
}

// --- Relation Tests ---

func TestCreateRelation_BothAssetsExist(t *testing.T) {
	uc, _, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	a1 := &Asset{AssetType: AssetTypeServer}
	a2 := &Asset{AssetType: AssetTypeDatabase}
	uc.CreateAsset(ctx, a1)
	uc.CreateAsset(ctx, a2)

	rel := &AssetRelation{
		SourceAssetID: a1.AssetID,
		TargetAssetID: a2.AssetID,
		RelationType:  RelationDependsOn,
	}
	if err := uc.CreateRelation(ctx, rel); err != nil {
		t.Fatalf("CreateRelation error: %v", err)
	}
}

func TestCreateRelation_SourceNotFound(t *testing.T) {
	uc, _, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	a2 := &Asset{AssetType: AssetTypeDatabase}
	uc.CreateAsset(ctx, a2)

	rel := &AssetRelation{
		SourceAssetID: "nonexistent",
		TargetAssetID: a2.AssetID,
		RelationType:  RelationDependsOn,
	}
	if err := uc.CreateRelation(ctx, rel); err == nil {
		t.Fatal("expected error: source asset not found")
	}
}

// --- Topology Tests ---

func TestGetTopology(t *testing.T) {
	uc, _, relRepo, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	a := &Asset{AssetType: AssetTypeServer}
	uc.CreateAsset(ctx, a)

	relRepo.topology = &TopologyGraph{
		Nodes: []*Asset{a},
		Edges: []*AssetRelation{},
	}

	graph, err := uc.GetTopology(ctx, a.AssetID, 3, "both")
	if err != nil {
		t.Fatalf("GetTopology error: %v", err)
	}
	if len(graph.Nodes) != 1 {
		t.Errorf("nodes = %d, want 1", len(graph.Nodes))
	}
}

// --- Group Tests ---

func TestCreateGroup_Dynamic(t *testing.T) {
	uc, _, _, groupRepo, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	groupRepo.members = []string{"asset-001", "asset-002"}

	g := &AssetGroup{
		Name:      "Prod Servers",
		GroupType: GroupTypeDynamic,
		Conditions: map[string]any{
			"environment": "production",
		},
	}
	if err := uc.CreateGroup(ctx, g); err != nil {
		t.Fatalf("CreateGroup error: %v", err)
	}
	if g.MemberCount != 2 {
		t.Errorf("member_count = %d, want 2", g.MemberCount)
	}
}

func TestGetGroupMembers(t *testing.T) {
	uc, _, _, groupRepo, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	groupRepo.members = []string{"a-1", "a-2", "a-3"}
	g := &AssetGroup{Name: "Test", GroupType: GroupTypeStatic, StaticMembers: []string{}}
	uc.CreateGroup(ctx, g)

	members, err := uc.GetGroupMembers(ctx, g.GroupID)
	if err != nil {
		t.Fatalf("GetGroupMembers error: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("members = %d, want 3", len(members))
	}
}

// --- Dimension Tests ---

func TestCreateDimension_Success(t *testing.T) {
	uc, _, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	d := &CustomDimension{
		Name:        "compliance",
		DisplayName: "Compliance Level",
		DimType:     "enum",
	}
	if err := uc.CreateDimension(ctx, d); err != nil {
		t.Fatalf("CreateDimension error: %v", err)
	}
}

func TestCreateDimension_Duplicate(t *testing.T) {
	uc, _, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	d := &CustomDimension{Name: "compliance", DisplayName: "Compliance", DimType: "enum"}
	uc.CreateDimension(ctx, d)

	d2 := &CustomDimension{Name: "compliance", DisplayName: "Compliance v2", DimType: "enum"}
	if err := uc.CreateDimension(ctx, d2); err == nil {
		t.Fatal("expected conflict error for duplicate dimension name")
	}
}

// --- Discovery Tests ---

func TestApproveDiscovery_CreatesNewAsset(t *testing.T) {
	uc, assetRepo, _, _, _, discoveryRepo := newTestCMDBUsecase()
	ctx := context.Background()

	hostname := "new-host"
	ip := "10.0.0.5"
	detectedType := "server"

	discoveryRepo.records["rec-001"] = &DiscoveryRecord{
		RecordID:        "rec-001",
		DiscoveryMethod: DiscoveryScan,
		Hostname:        &hostname,
		IP:              &ip,
		DetectedType:    &detectedType,
		Status:          DiscoveryStatusPending,
	}

	asset, err := uc.ApproveDiscovery(ctx, "rec-001")
	if err != nil {
		t.Fatalf("ApproveDiscovery error: %v", err)
	}

	if asset == nil {
		t.Fatal("expected asset to be created")
	}
	if asset.AssetType != "server" {
		t.Errorf("asset_type = %s, want server", asset.AssetType)
	}
	if len(assetRepo.assets) != 1 {
		t.Errorf("repo has %d assets, want 1", len(assetRepo.assets))
	}
	if discoveryRepo.records["rec-001"].Status != DiscoveryStatusApproved {
		t.Errorf("record status = %s, want approved", discoveryRepo.records["rec-001"].Status)
	}
}

func TestApproveDiscovery_LinksToExisting(t *testing.T) {
	uc, _, _, _, _, discoveryRepo := newTestCMDBUsecase()
	ctx := context.Background()

	hostname := "existing-host"
	ip := "10.0.0.1"

	// Create existing asset
	existing := &Asset{Hostname: &hostname, IP: &ip, AssetType: AssetTypeServer}
	uc.CreateAsset(ctx, existing)

	discoveryRepo.records["rec-002"] = &DiscoveryRecord{
		RecordID:        "rec-002",
		DiscoveryMethod: DiscoveryScan,
		Hostname:        &hostname,
		IP:              &ip,
		Status:          DiscoveryStatusPending,
	}

	asset, err := uc.ApproveDiscovery(ctx, "rec-002")
	if err != nil {
		t.Fatalf("ApproveDiscovery error: %v", err)
	}

	if asset.AssetID != existing.AssetID {
		t.Errorf("should link to existing asset %s, got %s", existing.AssetID, asset.AssetID)
	}
}

func TestApproveDiscovery_NotFound(t *testing.T) {
	uc, _, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	_, err := uc.ApproveDiscovery(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

// --- Maintenance Cascade Tests ---

func TestGetCascadeAssets(t *testing.T) {
	uc, _, relRepo, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	relRepo.cascadeIDs = []string{"child-1", "child-2", "grandchild-1"}

	cascaded, err := uc.GetCascadeAssets(ctx, []string{"parent-1"})
	if err != nil {
		t.Fatalf("GetCascadeAssets error: %v", err)
	}
	if len(cascaded) != 3 {
		t.Errorf("cascade assets = %d, want 3", len(cascaded))
	}
}

// --- ListAssets Filter Tests ---

func TestListAssets_FilterByType(t *testing.T) {
	uc, _, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	uc.CreateAsset(ctx, &Asset{AssetType: AssetTypeServer})
	uc.CreateAsset(ctx, &Asset{AssetType: AssetTypeServer})
	uc.CreateAsset(ctx, &Asset{AssetType: AssetTypeDatabase})

	serverType := AssetTypeServer
	assets, total, err := uc.ListAssets(ctx, AssetListFilter{AssetType: &serverType})
	if err != nil {
		t.Fatalf("ListAssets error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(assets) != 2 {
		t.Errorf("len(assets) = %d, want 2", len(assets))
	}
}

func TestListAssets_FilterByGrade(t *testing.T) {
	uc, _, _, _, _, _ := newTestCMDBUsecase()
	ctx := context.Background()

	gradeA := GradeA
	gradeB := GradeB
	uc.CreateAsset(ctx, &Asset{AssetType: AssetTypeServer, Grade: &gradeA})
	uc.CreateAsset(ctx, &Asset{AssetType: AssetTypeServer, Grade: &gradeB})
	uc.CreateAsset(ctx, &Asset{AssetType: AssetTypeServer, Grade: &gradeA})

	filter := GradeA
	assets, total, _ := uc.ListAssets(ctx, AssetListFilter{Grade: &filter})
	if total != 2 {
		t.Errorf("total grade-A assets = %d, want 2", total)
	}
	if len(assets) != 2 {
		t.Errorf("len(assets) = %d, want 2", len(assets))
	}
}
