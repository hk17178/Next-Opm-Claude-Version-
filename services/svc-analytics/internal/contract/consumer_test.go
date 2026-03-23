package contract

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// IncidentCreatedEvent is the expected CloudEvents schema for the
// opsnexus.incident.created Kafka event consumed by svc-analytics.
// svc-analytics uses this event to populate SLA incident data in ClickHouse
// (sla_incidents table) for availability calculation.
type IncidentCreatedEvent struct {
	ID     string              `json:"id"`
	Type   string              `json:"type"`
	Source string              `json:"source"`
	Time   string              `json:"time"`
	Data   IncidentCreatedData `json:"data"`
}

// IncidentCreatedData defines the payload fields consumed by svc-analytics.
// Contract fields: incident_id, severity, detected_at, service_name, asset_id.
type IncidentCreatedData struct {
	IncidentID     string   `json:"incident_id"`
	Severity       string   `json:"severity"`
	DetectedAt     string   `json:"detected_at"`
	ServiceName    string   `json:"service_name"`
	AssetID        string   `json:"asset_id"`
	Title          string   `json:"title"`
	Status         string   `json:"status"`
	BusinessUnit   string   `json:"business_unit"`
	AffectedAssets []string `json:"affected_assets"`
	SourceAlerts   []string `json:"source_alerts"`
}

// TestIncidentCreatedEventSchema_RequiredFields verifies that the
// opsnexus.incident.created payload contains all fields required by
// svc-analytics for SLA incident ingestion into ClickHouse.
func TestIncidentCreatedEventSchema_RequiredFields(t *testing.T) {
	payload := `{
		"id": "evt-inc-analytics-001",
		"type": "incident.created",
		"source": "svc-incident",
		"time": "2026-03-22T10:00:00Z",
		"data": {
			"incident_id": "INC-20260322-001",
			"severity": "P1",
			"detected_at": "2026-03-22T09:50:00Z",
			"service_name": "payment-gateway",
			"asset_id": "ci-web-01",
			"title": "Payment gateway latency spike",
			"status": "created",
			"business_unit": "payment",
			"affected_assets": ["ci-web-01", "ci-db-primary"],
			"source_alerts": ["alert-001", "alert-002"]
		}
	}`

	var event IncidentCreatedEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err, "incident.created event must be valid JSON")

	// Contract: these 5 fields MUST be present and non-empty for SLA calculation.
	// incident_id: used as primary key in sla_incidents table
	assert.NotEmpty(t, event.Data.IncidentID, "incident_id is required for SLA incident tracking")
	// severity: used for P0/P1 incident counting in SLA reports
	assert.NotEmpty(t, event.Data.Severity, "severity is required for SLA severity classification")
	// detected_at: used as incident start time for downtime calculation
	assert.NotEmpty(t, event.Data.DetectedAt, "detected_at is required for downtime window start")
	// service_name: maps to business_unit dimension in SLA config
	assert.NotEmpty(t, event.Data.ServiceName, "service_name is required for SLA dimension mapping")
	// asset_id: maps to asset-level SLA tracking
	assert.NotEmpty(t, event.Data.AssetID, "asset_id is required for asset-level SLA tracking")
}

// TestIncidentCreatedEventSchema_MissingRequiredFields verifies that each
// missing contract field is correctly detected as empty after deserialization.
func TestIncidentCreatedEventSchema_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		missing string
	}{
		{
			name:    "missing incident_id",
			payload: `{"data":{"severity":"P1","detected_at":"2026-03-22T09:50:00Z","service_name":"payment","asset_id":"ci-01"}}`,
			missing: "incident_id",
		},
		{
			name:    "missing severity",
			payload: `{"data":{"incident_id":"INC-001","detected_at":"2026-03-22T09:50:00Z","service_name":"payment","asset_id":"ci-01"}}`,
			missing: "severity",
		},
		{
			name:    "missing detected_at",
			payload: `{"data":{"incident_id":"INC-001","severity":"P1","service_name":"payment","asset_id":"ci-01"}}`,
			missing: "detected_at",
		},
		{
			name:    "missing service_name",
			payload: `{"data":{"incident_id":"INC-001","severity":"P1","detected_at":"2026-03-22T09:50:00Z","asset_id":"ci-01"}}`,
			missing: "service_name",
		},
		{
			name:    "missing asset_id",
			payload: `{"data":{"incident_id":"INC-001","severity":"P1","detected_at":"2026-03-22T09:50:00Z","service_name":"payment"}}`,
			missing: "asset_id",
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
			case "service_name":
				assert.Empty(t, event.Data.ServiceName)
			case "asset_id":
				assert.Empty(t, event.Data.AssetID)
			}
		})
	}
}

// TestIncidentCreatedEventSchema_SLAIngestionMapping verifies that the event
// fields can be correctly mapped to the sla_incidents ClickHouse table columns
// used by svc-analytics for SLA calculation.
func TestIncidentCreatedEventSchema_SLAIngestionMapping(t *testing.T) {
	payload := `{
		"id": "evt-inc-analytics-002",
		"type": "incident.created",
		"source": "svc-incident",
		"time": "2026-03-22T14:00:00Z",
		"data": {
			"incident_id": "INC-20260322-007",
			"severity": "P0",
			"detected_at": "2026-03-22T13:55:00Z",
			"service_name": "order-processing",
			"asset_id": "ci-app-cluster-01",
			"title": "Complete order processing failure",
			"status": "created",
			"business_unit": "orders",
			"affected_assets": ["ci-app-cluster-01", "ci-redis-01", "ci-db-orders"],
			"source_alerts": ["alert-latency-spike", "alert-error-rate"]
		}
	}`

	var event IncidentCreatedEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err)

	// Verify incident_id format (INC-YYYYMMDD-NNN from svc-incident)
	assert.Equal(t, "INC-20260322-007", event.Data.IncidentID)

	// Verify severity is a valid P-level for SLA severity classification
	assert.Contains(t, []string{"P0", "P1", "P2", "P3", "P4"}, event.Data.Severity,
		"severity must be a valid P-level")

	// Verify detected_at is RFC3339 parseable (used for downtime window)
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`, event.Data.DetectedAt,
		"detected_at must be RFC3339 format")

	// Verify service_name maps to SLA business_unit dimension
	assert.Equal(t, "order-processing", event.Data.ServiceName)

	// Verify asset_id maps to asset-level SLA dimension
	assert.Equal(t, "ci-app-cluster-01", event.Data.AssetID)

	// Verify business_unit is present for dimension routing
	assert.Equal(t, "orders", event.Data.BusinessUnit)

	// Verify affected_assets for multi-asset SLA impact
	assert.Len(t, event.Data.AffectedAssets, 3)
}

// TestIncidentCreatedEventSchema_SeverityValues verifies all valid severity
// levels can be deserialized, as svc-analytics uses severity for P0/P1 counting.
func TestIncidentCreatedEventSchema_SeverityValues(t *testing.T) {
	severities := []string{"P0", "P1", "P2", "P3", "P4"}
	for _, sev := range severities {
		t.Run("severity_"+sev, func(t *testing.T) {
			payload := `{"data":{"incident_id":"INC-001","severity":"` + sev + `","detected_at":"2026-03-22T10:00:00Z","service_name":"web","asset_id":"ci-01"}}`
			var event IncidentCreatedEvent
			err := json.Unmarshal([]byte(payload), &event)
			require.NoError(t, err)
			assert.Equal(t, sev, event.Data.Severity)
		})
	}
}

// TestIncidentCreatedEventSchema_ExtraFieldsIgnored verifies forward compatibility:
// new fields added by svc-incident should not break svc-analytics deserialization.
func TestIncidentCreatedEventSchema_ExtraFieldsIgnored(t *testing.T) {
	payload := `{
		"id": "evt-inc-analytics-003",
		"type": "incident.created",
		"source": "svc-incident",
		"time": "2026-03-22T15:00:00Z",
		"data": {
			"incident_id": "INC-003",
			"severity": "P2",
			"detected_at": "2026-03-22T14:55:00Z",
			"service_name": "search-api",
			"asset_id": "ci-search-01",
			"title": "Search degradation",
			"status": "created",
			"new_v2_field": "should be ignored",
			"responder_ids": ["user-1", "user-2"],
			"custom_tags": {"env": "prod"}
		}
	}`

	var event IncidentCreatedEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err, "extra fields should not break deserialization")
	assert.Equal(t, "INC-003", event.Data.IncidentID)
	assert.Equal(t, "P2", event.Data.Severity)
	assert.Equal(t, "search-api", event.Data.ServiceName)
	assert.Equal(t, "ci-search-01", event.Data.AssetID)
}

// TestIncidentCreatedEventSchema_CloudEventEnvelope verifies the CloudEvents
// envelope fields that svc-analytics uses for event routing and deduplication.
func TestIncidentCreatedEventSchema_CloudEventEnvelope(t *testing.T) {
	payload := `{
		"id": "evt-inc-analytics-004",
		"type": "incident.created",
		"source": "svc-incident",
		"time": "2026-03-22T16:00:00Z",
		"data": {
			"incident_id": "INC-004",
			"severity": "P1",
			"detected_at": "2026-03-22T15:55:00Z",
			"service_name": "auth",
			"asset_id": "ci-auth-01"
		}
	}`

	var event IncidentCreatedEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err)

	// Envelope fields used for dedup and routing
	assert.NotEmpty(t, event.ID, "event id is required for deduplication")
	assert.Equal(t, "incident.created", event.Type, "event type must be incident.created")
	assert.Equal(t, "svc-incident", event.Source, "event source must be svc-incident")
	assert.NotEmpty(t, event.Time, "event time is required for ordering")
}
