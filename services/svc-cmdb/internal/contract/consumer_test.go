package contract

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CIResponse represents the CMDB gRPC CI response schema consumed by
// svc-incident (for affected asset enrichment) and svc-ai (for topology context).
// This mirrors the proto CI message in pkg/proto/gen/go/cmdb/cmdb_service.pb.go.
type CIResponse struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Attributes  map[string]string `json:"attributes"`
	Labels      []KeyValue        `json:"labels"`
	OwnerID     string            `json:"owner_id"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// KeyValue mirrors commonpb.KeyValue.
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// GetCIRequest represents the CMDB gRPC GetCI request schema.
type GetCIRequest struct {
	AssetID string `json:"asset_id"`
}

// ListAssetsRequest represents the HTTP list assets query parameters.
type ListAssetsRequest struct {
	AssetType string `json:"asset_type"`
	Grade     string `json:"grade"`
	Status    string `json:"status"`
	PageSize  int    `json:"page_size"`
	PageToken string `json:"page_token"`
}

// ListAssetsResponse represents the HTTP list assets response schema.
type ListAssetsResponse struct {
	Assets    []CIResponse `json:"assets"`
	Total     int64        `json:"total"`
	PageSize  int          `json:"page_size"`
	PageToken string       `json:"page_token"`
}

// TestGetAssetRequest_RequiredFields verifies that the GetCI request must
// contain a non-empty asset_id.
func TestGetAssetRequest_RequiredFields(t *testing.T) {
	payload := `{"asset_id": "ci-web-01"}`

	var req GetCIRequest
	err := json.Unmarshal([]byte(payload), &req)
	require.NoError(t, err, "GetCI request must be valid JSON")

	assert.NotEmpty(t, req.AssetID, "asset_id is required for GetCI request")

	// Missing asset_id should be detectable
	emptyPayload := `{}`
	var emptyReq GetCIRequest
	err = json.Unmarshal([]byte(emptyPayload), &emptyReq)
	require.NoError(t, err)
	assert.Empty(t, emptyReq.AssetID, "missing asset_id should deserialize as empty string")
}

// TestGetAssetResponse_RequiredFields verifies that the CI response contains
// the fields required by downstream consumers (svc-incident, svc-ai, svc-analytics).
// Contract fields: name, asset_type, asset_grade, business_unit.
func TestGetAssetResponse_RequiredFields(t *testing.T) {
	payload := `{
		"id": "ci-web-01",
		"type": "server",
		"name": "web-prod-01",
		"description": "Production web server",
		"status": "active",
		"attributes": {
			"hostname": "web-prod-01.dc1.example.com",
			"ip": "10.0.1.10",
			"environment": "prod",
			"region": "cn-east-1",
			"grade": "S"
		},
		"labels": [
			{"key": "business_unit", "value": "payment"},
			{"key": "business_unit", "value": "orders"}
		],
		"owner_id": "user-001",
		"created_at": "2026-01-15T08:00:00Z",
		"updated_at": "2026-03-20T12:00:00Z"
	}`

	var ci CIResponse
	err := json.Unmarshal([]byte(payload), &ci)
	require.NoError(t, err, "CI response must be valid JSON")

	// name: used by svc-incident for incident title generation
	assert.NotEmpty(t, ci.Name, "name is required by consumer contract")

	// asset_type (type field): used by svc-ai for topology context classification
	assert.NotEmpty(t, ci.Type, "asset_type is required by consumer contract")

	// asset_grade (attributes.grade): used by svc-analytics for SLA grade-level calculation
	grade, hasGrade := ci.Attributes["grade"]
	assert.True(t, hasGrade, "attributes must include asset_grade (grade)")
	assert.Contains(t, []string{"S", "A", "B", "C", "D"}, grade, "asset_grade must be S/A/B/C/D")

	// business_unit: used by svc-analytics for SLA dimension mapping
	var businessUnits []string
	for _, kv := range ci.Labels {
		if kv.Key == "business_unit" {
			businessUnits = append(businessUnits, kv.Value)
		}
	}
	assert.NotEmpty(t, businessUnits, "labels must include at least one business_unit")
}

// TestListAssets_PaginationFields verifies that list assets request supports
// page_size and page_token pagination parameters.
// Empty page_token string means first page.
func TestListAssets_PaginationFields(t *testing.T) {
	t.Run("first page with empty page_token", func(t *testing.T) {
		payload := `{
			"asset_type": "server",
			"grade": "S",
			"status": "active",
			"page_size": 20,
			"page_token": ""
		}`

		var req ListAssetsRequest
		err := json.Unmarshal([]byte(payload), &req)
		require.NoError(t, err)

		assert.Equal(t, 20, req.PageSize, "page_size must be provided")
		assert.Empty(t, req.PageToken, "empty page_token indicates first page")
		assert.Equal(t, "server", req.AssetType, "asset_type filter must be parsed")
	})

	t.Run("subsequent page with page_token", func(t *testing.T) {
		payload := `{
			"asset_type": "server",
			"page_size": 20,
			"page_token": "eyJwYWdlIjoyfQ=="
		}`

		var req ListAssetsRequest
		err := json.Unmarshal([]byte(payload), &req)
		require.NoError(t, err)

		assert.Equal(t, 20, req.PageSize, "page_size must be provided")
		assert.NotEmpty(t, req.PageToken, "page_token must be non-empty for subsequent pages")
	})

	t.Run("response includes page_token for next page", func(t *testing.T) {
		payload := `{
			"assets": [
				{"id": "ci-001", "type": "server", "name": "web-01", "status": "active", "attributes": {"grade": "S"}},
				{"id": "ci-002", "type": "server", "name": "web-02", "status": "active", "attributes": {"grade": "A"}}
			],
			"total": 150,
			"page_size": 20,
			"page_token": "eyJwYWdlIjoyfQ=="
		}`

		var resp ListAssetsResponse
		err := json.Unmarshal([]byte(payload), &resp)
		require.NoError(t, err)

		assert.Len(t, resp.Assets, 2)
		assert.Equal(t, int64(150), resp.Total, "total count must be present for pagination")
		assert.Equal(t, 20, resp.PageSize, "page_size must be echoed in response")
		assert.NotEmpty(t, resp.PageToken, "page_token must be present when more pages exist")
	})
}
