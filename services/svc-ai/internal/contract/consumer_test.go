package contract

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// IncidentCreatedEvent is the expected schema for the opsnexus.incident.created
// Kafka event consumed by svc-ai to trigger root cause analysis.
type IncidentCreatedEvent struct {
	ID     string              `json:"id"`
	Type   string              `json:"type"`
	Source string              `json:"source"`
	Time   string              `json:"time"`
	Data   IncidentCreatedData `json:"data"`
}

type IncidentCreatedData struct {
	IncidentID    string   `json:"incident_id"`
	Severity      string   `json:"severity"`
	DetectedAt    string   `json:"detected_at"`
	RelatedAlerts []string `json:"related_alerts"`
	CMDBAssets    []string `json:"cmdb_assets"`
	Title         string   `json:"title"`
	Status        string   `json:"status"`
}

// TestIncidentCreatedEventSchema_RequiredFields verifies the consumer contract:
// svc-ai expects opsnexus.incident.created events to contain the required fields
// needed for AI analysis context collection.
func TestIncidentCreatedEventSchema_RequiredFields(t *testing.T) {
	payload := `{
		"id": "evt-inc-001",
		"type": "incident.created",
		"source": "svc-incident",
		"time": "2026-03-22T10:00:00Z",
		"data": {
			"incident_id": "inc-a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			"severity": "P1",
			"detected_at": "2026-03-22T09:50:00Z",
			"related_alerts": ["alert-001", "alert-002", "alert-003"],
			"cmdb_assets": ["ci-web-01", "ci-db-primary"],
			"title": "Multiple services degraded in us-east-1",
			"status": "open"
		}
	}`

	var event IncidentCreatedEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err, "incident.created event must be valid JSON")

	// Contract: these 5 fields MUST be present and non-empty
	assert.NotEmpty(t, event.Data.IncidentID, "incident_id is required by consumer contract")
	assert.NotEmpty(t, event.Data.Severity, "severity is required by consumer contract")
	assert.NotEmpty(t, event.Data.DetectedAt, "detected_at is required by consumer contract")
	assert.NotEmpty(t, event.Data.RelatedAlerts, "related_alerts is required by consumer contract")
	assert.NotEmpty(t, event.Data.CMDBAssets, "cmdb_assets is required by consumer contract")
}

// TestIncidentCreatedEventSchema_MissingRequiredFields verifies detection of
// missing contract fields.
func TestIncidentCreatedEventSchema_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		missing string
	}{
		{
			name:    "missing incident_id",
			payload: `{"data":{"severity":"P1","detected_at":"2026-03-22T09:50:00Z","related_alerts":["a1"],"cmdb_assets":["ci-01"]}}`,
			missing: "incident_id",
		},
		{
			name:    "missing severity",
			payload: `{"data":{"incident_id":"inc-001","detected_at":"2026-03-22T09:50:00Z","related_alerts":["a1"],"cmdb_assets":["ci-01"]}}`,
			missing: "severity",
		},
		{
			name:    "missing detected_at",
			payload: `{"data":{"incident_id":"inc-001","severity":"P1","related_alerts":["a1"],"cmdb_assets":["ci-01"]}}`,
			missing: "detected_at",
		},
		{
			name:    "missing related_alerts",
			payload: `{"data":{"incident_id":"inc-001","severity":"P1","detected_at":"2026-03-22T09:50:00Z","cmdb_assets":["ci-01"]}}`,
			missing: "related_alerts",
		},
		{
			name:    "missing cmdb_assets",
			payload: `{"data":{"incident_id":"inc-001","severity":"P1","detected_at":"2026-03-22T09:50:00Z","related_alerts":["a1"]}}`,
			missing: "cmdb_assets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event IncidentCreatedEvent
			err := json.Unmarshal([]byte(tt.payload), &event)
			require.NoError(t, err, "JSON should still parse even with missing field")

			switch tt.missing {
			case "incident_id":
				assert.Empty(t, event.Data.IncidentID)
			case "severity":
				assert.Empty(t, event.Data.Severity)
			case "detected_at":
				assert.Empty(t, event.Data.DetectedAt)
			case "related_alerts":
				assert.Empty(t, event.Data.RelatedAlerts)
			case "cmdb_assets":
				assert.Empty(t, event.Data.CMDBAssets)
			}
		})
	}
}

// TestIncidentCreatedEventSchema_ContextCollectorParsing verifies that the
// context_collector can correctly parse the incident.created event fields into
// the format needed for AI analysis context building.
func TestIncidentCreatedEventSchema_ContextCollectorParsing(t *testing.T) {
	payload := `{
		"id": "evt-inc-002",
		"type": "incident.created",
		"source": "svc-incident",
		"time": "2026-03-22T11:00:00Z",
		"data": {
			"incident_id": "inc-deadbeef-1234-5678-abcd-ef0123456789",
			"severity": "P0",
			"detected_at": "2026-03-22T10:55:00Z",
			"related_alerts": ["alert-aaa", "alert-bbb", "alert-ccc"],
			"cmdb_assets": ["ci-web-cluster", "ci-redis-primary", "ci-lb-01"],
			"title": "Complete service outage in production",
			"status": "open"
		}
	}`

	var event IncidentCreatedEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err)

	// Simulate the context_collector's field extraction:
	// The collector reads incident_id, severity, related_alerts, cmdb_assets
	// to build alert_info and topology_info context strings.

	// incident_id must be a valid UUID-like string for biz.AnalysisCreateRequest.IncidentID
	assert.Equal(t, "inc-deadbeef-1234-5678-abcd-ef0123456789", event.Data.IncidentID)

	// severity feeds into scene selection and prompt variables
	assert.Contains(t, []string{"P0", "P1", "P2", "P3", "critical", "warning", "info"}, event.Data.Severity)

	// related_alerts are parsed into biz.AnalysisCreateRequest.AlertIDs
	assert.Len(t, event.Data.RelatedAlerts, 3)
	for _, alertID := range event.Data.RelatedAlerts {
		assert.NotEmpty(t, alertID, "each related_alert must be a non-empty string")
	}

	// cmdb_assets are used to build topology context
	assert.Len(t, event.Data.CMDBAssets, 3)
	for _, assetID := range event.Data.CMDBAssets {
		assert.NotEmpty(t, assetID, "each cmdb_asset must be a non-empty string")
	}

	// detected_at is used for time range in context collection
	assert.Equal(t, "2026-03-22T10:55:00Z", event.Data.DetectedAt)

	// Build raw context map as context_collector would
	contextMap := map[string]interface{}{
		"alert_info":    buildAlertInfoString(event),
		"topology_info": buildTopologyInfoString(event),
	}

	alertInfo, ok := contextMap["alert_info"].(string)
	require.True(t, ok)
	assert.Contains(t, alertInfo, event.Data.IncidentID)
	assert.Contains(t, alertInfo, event.Data.Severity)

	topoInfo, ok := contextMap["topology_info"].(string)
	require.True(t, ok)
	for _, asset := range event.Data.CMDBAssets {
		assert.Contains(t, topoInfo, asset)
	}
}

// TestIncidentCreatedEventSchema_ExtraFieldsIgnored verifies forward compatibility.
func TestIncidentCreatedEventSchema_ExtraFieldsIgnored(t *testing.T) {
	payload := `{
		"id": "evt-inc-003",
		"type": "incident.created",
		"source": "svc-incident",
		"time": "2026-03-22T12:00:00Z",
		"data": {
			"incident_id": "inc-003",
			"severity": "P2",
			"detected_at": "2026-03-22T11:55:00Z",
			"related_alerts": ["alert-x"],
			"cmdb_assets": ["ci-y"],
			"title": "Test incident",
			"status": "open",
			"new_v2_field": "should be ignored",
			"responder_ids": ["user-1", "user-2"]
		}
	}`

	var event IncidentCreatedEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err, "extra fields should not break deserialization")
	assert.Equal(t, "inc-003", event.Data.IncidentID)
	assert.Equal(t, "P2", event.Data.Severity)
}

// TestIncidentCreatedEventSchema_EmptyArrays verifies behavior when
// related_alerts and cmdb_assets are present but empty.
func TestIncidentCreatedEventSchema_EmptyArrays(t *testing.T) {
	payload := `{
		"data": {
			"incident_id": "inc-004",
			"severity": "P3",
			"detected_at": "2026-03-22T12:00:00Z",
			"related_alerts": [],
			"cmdb_assets": []
		}
	}`

	var event IncidentCreatedEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err)

	assert.NotNil(t, event.Data.RelatedAlerts, "empty array should deserialize as empty slice, not nil")
	assert.NotNil(t, event.Data.CMDBAssets, "empty array should deserialize as empty slice, not nil")
	assert.Len(t, event.Data.RelatedAlerts, 0)
	assert.Len(t, event.Data.CMDBAssets, 0)
}

// Helper: simulates how context_collector builds alert_info from incident event
func buildAlertInfoString(event IncidentCreatedEvent) string {
	return "IncidentID: " + event.Data.IncidentID +
		"\nSeverity: " + event.Data.Severity +
		"\nDetectedAt: " + event.Data.DetectedAt +
		"\nTitle: " + event.Data.Title +
		"\nRelatedAlerts: " + joinStrings(event.Data.RelatedAlerts)
}

// Helper: simulates how context_collector builds topology context
func buildTopologyInfoString(event IncidentCreatedEvent) string {
	return "AffectedAssets: " + joinStrings(event.Data.CMDBAssets)
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
