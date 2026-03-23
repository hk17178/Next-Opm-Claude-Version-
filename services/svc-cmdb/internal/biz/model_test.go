package biz

import (
	"testing"
)

// --- Asset Type Classification Tests ---

func TestAssetTypes_AllDefined(t *testing.T) {
	types := []struct {
		constant string
		value    string
	}{
		{AssetTypeServer, "server"},
		{AssetTypeVirtualMachine, "virtual_machine"},
		{AssetTypeContainer, "container"},
		{AssetTypeNetworkDevice, "network_device"},
		{AssetTypeDatabase, "database"},
		{AssetTypeMiddleware, "middleware"},
		{AssetTypeApplication, "application"},
		{AssetTypeStorage, "storage"},
		{AssetTypeLoadBalancer, "loadbalancer"},
		{AssetTypeFirewall, "firewall"},
		{AssetTypeDNS, "dns"},
		{AssetTypeCDN, "cdn"},
		{AssetTypeMessageQueue, "message_queue"},
		{AssetTypeCache, "cache"},
		{AssetTypeMonitoring, "monitoring"},
		{AssetTypeCloudInstance, "cloud_instance"},
		{AssetTypeCloudService, "cloud_service"},
		{AssetTypeK8sCluster, "k8s_cluster"},
		{AssetTypeK8sPod, "k8s_pod"},
		{AssetTypeK8sService, "k8s_service"},
		{AssetTypeAPIGateway, "api_gateway"},
		{AssetTypeOther, "other"},
	}

	if len(types) != 22 {
		t.Errorf("expected 22 asset types, got %d", len(types))
	}

	for _, tt := range types {
		t.Run(tt.value, func(t *testing.T) {
			if tt.constant != tt.value {
				t.Errorf("constant = %q, want %q", tt.constant, tt.value)
			}
		})
	}
}

// --- Grade Level Tests ---

func TestGradeLevels(t *testing.T) {
	grades := []struct {
		constant string
		value    string
		desc     string
	}{
		{GradeS, "S", "Core lifeline"},
		{GradeA, "A", "Core business"},
		{GradeB, "B", "Important support"},
		{GradeC, "C", "General"},
		{GradeD, "D", "Non-critical"},
	}

	for _, g := range grades {
		t.Run(g.desc, func(t *testing.T) {
			if g.constant != g.value {
				t.Errorf("grade constant = %q, want %q", g.constant, g.value)
			}
		})
	}
}

// --- Status Constants Tests ---

func TestAssetStatuses(t *testing.T) {
	statuses := map[string]string{
		"active":      StatusActive,
		"idle":        StatusIdle,
		"maintenance": StatusMaintenance,
		"retired":     StatusRetired,
	}

	for expected, got := range statuses {
		if got != expected {
			t.Errorf("status constant = %q, want %q", got, expected)
		}
	}
}

// --- Relation Type Tests ---

func TestRelationTypes(t *testing.T) {
	relations := map[string]string{
		"depends_on":   RelationDependsOn,
		"deployed_on":  RelationDeployedOn,
		"connected_to": RelationConnectedTo,
		"child_of":     RelationChildOf,
	}

	for expected, got := range relations {
		if got != expected {
			t.Errorf("relation constant = %q, want %q", got, expected)
		}
	}
}

// --- Group Type Tests ---

func TestGroupTypes(t *testing.T) {
	if GroupTypeDynamic != "dynamic" {
		t.Errorf("GroupTypeDynamic = %q", GroupTypeDynamic)
	}
	if GroupTypeStatic != "static" {
		t.Errorf("GroupTypeStatic = %q", GroupTypeStatic)
	}
	if GroupTypeHybrid != "hybrid" {
		t.Errorf("GroupTypeHybrid = %q", GroupTypeHybrid)
	}
}

// --- Discovery Method/Status Tests ---

func TestDiscoveryMethods(t *testing.T) {
	methods := []string{DiscoveryScan, DiscoverySNMP, DiscoveryCloudAPI, DiscoveryDHCP, DiscoveryLogSource, DiscoveryManual}
	if len(methods) != 6 {
		t.Errorf("expected 6 discovery methods, got %d", len(methods))
	}
}

func TestDiscoveryStatuses(t *testing.T) {
	statuses := []string{DiscoveryStatusPending, DiscoveryStatusApproved, DiscoveryStatusIgnored, DiscoveryStatusBlacklisted}
	expected := map[string]bool{"pending": true, "approved": true, "ignored": true, "blacklisted": true}

	for _, s := range statuses {
		if !expected[s] {
			t.Errorf("unexpected discovery status: %q", s)
		}
	}
}

// --- Six-Dimension Classification Tests ---

func TestAsset_SixDimensionClassification(t *testing.T) {
	hostname := "web-prod-01"
	ip := "10.0.1.5"
	env := "production"
	region := "cn-east-1"
	grade := GradeA

	a := &Asset{
		AssetID:          "asset-001",
		Hostname:         &hostname,
		IP:               &ip,
		AssetType:        AssetTypeServer,          // Dimension 1: asset_type
		BusinessUnits:    []string{"platform", "pay"}, // Dimension 2: business_units
		Environment:      &env,                     // Dimension 3: environment
		Region:           &region,                  // Dimension 4: region
		Grade:            &grade,                   // Dimension 5: grade
		CustomDimensions: map[string]any{           // Dimension 6: custom_dimensions
			"cost_center":  "CC-100",
			"compliance":   "pci-dss",
		},
		Status: StatusActive,
		Tags:   map[string]string{"role": "web"},
	}

	// Verify all six dimensions are populated
	if a.AssetType == "" {
		t.Error("dimension 1 (asset_type) missing")
	}
	if len(a.BusinessUnits) == 0 {
		t.Error("dimension 2 (business_units) missing")
	}
	if a.Environment == nil || *a.Environment == "" {
		t.Error("dimension 3 (environment) missing")
	}
	if a.Region == nil || *a.Region == "" {
		t.Error("dimension 4 (region) missing")
	}
	if a.Grade == nil || *a.Grade == "" {
		t.Error("dimension 5 (grade) missing")
	}
	if len(a.CustomDimensions) == 0 {
		t.Error("dimension 6 (custom_dimensions) missing")
	}

	// Verify specific values
	if len(a.BusinessUnits) != 2 {
		t.Errorf("business_units count = %d, want 2", len(a.BusinessUnits))
	}
	if *a.Grade != GradeA {
		t.Errorf("grade = %s, want A", *a.Grade)
	}
	if a.CustomDimensions["cost_center"] != "CC-100" {
		t.Errorf("custom_dimensions[cost_center] = %v, want CC-100", a.CustomDimensions["cost_center"])
	}
}

// --- TopologyGraph Structure Tests ---

func TestTopologyGraph_Structure(t *testing.T) {
	hostA := "host-a"
	hostB := "host-b"
	hostC := "host-c"

	graph := &TopologyGraph{
		Nodes: []*Asset{
			{AssetID: "a-1", Hostname: &hostA, AssetType: AssetTypeServer},
			{AssetID: "a-2", Hostname: &hostB, AssetType: AssetTypeDatabase},
			{AssetID: "a-3", Hostname: &hostC, AssetType: AssetTypeCache},
		},
		Edges: []*AssetRelation{
			{RelationID: "r-1", SourceAssetID: "a-1", TargetAssetID: "a-2", RelationType: RelationDependsOn},
			{RelationID: "r-2", SourceAssetID: "a-1", TargetAssetID: "a-3", RelationType: RelationDependsOn},
		},
	}

	if len(graph.Nodes) != 3 {
		t.Errorf("nodes count = %d, want 3", len(graph.Nodes))
	}
	if len(graph.Edges) != 2 {
		t.Errorf("edges count = %d, want 2", len(graph.Edges))
	}

	// Verify edges reference valid nodes
	nodeIDs := map[string]bool{}
	for _, n := range graph.Nodes {
		nodeIDs[n.AssetID] = true
	}
	for _, e := range graph.Edges {
		if !nodeIDs[e.SourceAssetID] {
			t.Errorf("edge %s references non-existent source %s", e.RelationID, e.SourceAssetID)
		}
		if !nodeIDs[e.TargetAssetID] {
			t.Errorf("edge %s references non-existent target %s", e.RelationID, e.TargetAssetID)
		}
	}
}

// --- AssetRelation Validation ---

func TestAssetRelation_Fields(t *testing.T) {
	rel := &AssetRelation{
		RelationID:    "rel-001",
		SourceAssetID: "asset-001",
		TargetAssetID: "asset-002",
		RelationType:  RelationDependsOn,
	}

	if rel.RelationID == "" {
		t.Error("relation_id should not be empty")
	}
	if rel.SourceAssetID == rel.TargetAssetID {
		t.Error("source and target should be different")
	}
}

// --- AssetGroup Tests ---

func TestAssetGroup_DynamicType(t *testing.T) {
	g := &AssetGroup{
		GroupID:   "grp-001",
		Name:      "Production Servers",
		GroupType: GroupTypeDynamic,
		Conditions: map[string]any{
			"environment": "production",
			"asset_type":  "server",
		},
		MemberCount: 0,
	}

	if g.GroupType != GroupTypeDynamic {
		t.Errorf("group_type = %s, want dynamic", g.GroupType)
	}
	if g.Conditions == nil {
		t.Error("conditions should be set for dynamic group")
	}
}

func TestAssetGroup_StaticType(t *testing.T) {
	g := &AssetGroup{
		GroupID:       "grp-002",
		Name:          "VIP Servers",
		GroupType:     GroupTypeStatic,
		StaticMembers: []string{"asset-001", "asset-002", "asset-003"},
		MemberCount:   3,
	}

	if g.GroupType != GroupTypeStatic {
		t.Errorf("group_type = %s, want static", g.GroupType)
	}
	if len(g.StaticMembers) != 3 {
		t.Errorf("static_members count = %d, want 3", len(g.StaticMembers))
	}
}

// --- CustomDimension Tests ---

func TestCustomDimension_Fields(t *testing.T) {
	d := &CustomDimension{
		DimID:       "dim-001",
		Name:        "compliance_level",
		DisplayName: "Compliance Level",
		DimType:     "enum",
		Config:      map[string]any{"values": []string{"pci-dss", "hipaa", "sox", "none"}},
		Required:    true,
		Sortable:    false,
		Filterable:  true,
	}

	if d.Name == "" {
		t.Error("name should not be empty")
	}
	if d.DimType != "enum" {
		t.Errorf("dim_type = %s, want enum", d.DimType)
	}
	if !d.Required {
		t.Error("required should be true")
	}
	if !d.Filterable {
		t.Error("filterable should be true")
	}
}

// --- MaintenanceWindow Tests ---

func TestMaintenanceWindow_CascadeFlag(t *testing.T) {
	mw := &MaintenanceWindow{
		MWID:    "mw-001",
		Name:    "DB Migration",
		Assets:  []string{"asset-db-01"},
		Cascade: true,
	}

	if !mw.Cascade {
		t.Error("cascade should be true for maintenance that affects dependent services")
	}
}
