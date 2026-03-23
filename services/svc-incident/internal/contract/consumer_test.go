package contract

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AlertFiredEvent is the expected CloudEvents schema for the
// opsnexus.alert.fired Kafka event consumed by svc-incident.
// svc-incident uses this event to auto-create incidents from alerts.
type AlertFiredEvent struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Source string         `json:"source"`
	Time   string         `json:"time"`
	Data   AlertFiredData `json:"data"`
}

// AlertFiredData defines the payload fields consumed by svc-incident's AlertConsumer.
// Contract fields: alert_id, severity, rule_name, fired_at, resource_id (source.asset_id).
type AlertFiredData struct {
	AlertID     string           `json:"alert_id"`
	RuleID      string           `json:"rule_id"`
	RuleName    string           `json:"rule_name"`
	Layer       int              `json:"layer"`
	Severity    string           `json:"severity"`
	Source      AlertFiredSource `json:"source"`
	Message     string           `json:"message"`
	MetricValue float64          `json:"metric_value"`
	Threshold   float64          `json:"threshold"`
	Fingerprint string           `json:"fingerprint"`
	FiredAt     string           `json:"fired_at"`
	Tags        map[string]string `json:"tags"`
}

// AlertFiredSource contains the resource info from the alert.
type AlertFiredSource struct {
	AssetID      string `json:"asset_id"`
	Hostname     string `json:"hostname"`
	IP           string `json:"ip"`
	AssetType    string `json:"asset_type"`
	BusinessUnit string `json:"business_unit"`
}

// TestAlertFiredEventSchema_RequiredFields verifies that the opsnexus.alert.fired
// payload contains all fields required by svc-incident for incident creation.
// Contract: alert_id, severity, rule_name, fired_at, resource_id (source.asset_id).
func TestAlertFiredEventSchema_RequiredFields(t *testing.T) {
	payload := `{
		"id": "evt-alert-001",
		"type": "alert.fired",
		"source": "svc-alert",
		"time": "2026-03-22T10:00:00Z",
		"data": {
			"alert_id": "alert-cpu-001",
			"rule_id": "rule-cpu-high",
			"rule_name": "CPU Usage > 95%",
			"layer": 1,
			"severity": "critical",
			"source": {
				"asset_id": "ci-web-01",
				"hostname": "web-prod-01",
				"ip": "10.0.1.10",
				"asset_type": "server",
				"business_unit": "payment"
			},
			"message": "CPU usage exceeded 95% for 5 minutes",
			"metric_value": 97.3,
			"threshold": 95.0,
			"fingerprint": "fp-cpu-web01",
			"fired_at": "2026-03-22T09:55:00Z",
			"tags": {"env": "prod", "region": "cn-east-1"}
		}
	}`

	var event AlertFiredEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err, "alert.fired event must be valid JSON")

	// alert_id: stored in incident.source_alerts
	assert.NotEmpty(t, event.Data.AlertID, "alert_id is required for incident source tracking")
	// severity: mapped to incident severity (P0/P1/P2/P3)
	assert.NotEmpty(t, event.Data.Severity, "severity is required for incident priority mapping")
	// rule_name: used in incident title generation (hostname + rule_name)
	assert.NotEmpty(t, event.Data.RuleName, "rule_name is required for incident title")
	// fired_at: maps to incident detected_at timestamp
	assert.NotEmpty(t, event.Data.FiredAt, "fired_at is required for incident detected_at")
	// resource_id (source.asset_id): stored in incident.affected_assets
	assert.NotEmpty(t, event.Data.Source.AssetID, "resource_id (source.asset_id) is required for affected_assets")
}

// TestAlertFiredEventSchema_SeverityMapping verifies the mapping from alert
// severity to incident priority level.
// P0 -> critical, P1 -> high, P2 -> medium, P3 -> low
func TestAlertFiredEventSchema_SeverityMapping(t *testing.T) {
	severityToPriority := map[string]string{
		"critical": "P0",
		"high":     "P1",
		"medium":   "P2",
		"low":      "P3",
	}

	for severity, expectedPriority := range severityToPriority {
		t.Run(expectedPriority+"_maps_from_"+severity, func(t *testing.T) {
			payload := `{"data":{
				"alert_id":"alert-001",
				"severity":"` + severity + `",
				"rule_name":"Test Rule",
				"fired_at":"2026-03-22T10:00:00Z",
				"source":{"asset_id":"ci-01"}
			}}`

			var event AlertFiredEvent
			err := json.Unmarshal([]byte(payload), &event)
			require.NoError(t, err)

			assert.Equal(t, severity, event.Data.Severity)

			mapped := mapSeverityToPriority(event.Data.Severity)
			assert.Equal(t, expectedPriority, mapped,
				"severity %q should map to priority %q", severity, expectedPriority)
		})
	}
}

// TestAlertFiredEventSchema_MissingFields verifies that each missing
// contract field is correctly detected as empty.
func TestAlertFiredEventSchema_MissingFields(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		missing string
	}{
		{
			name:    "missing alert_id",
			payload: `{"data":{"severity":"critical","rule_name":"CPU High","fired_at":"2026-03-22T10:00:00Z","source":{"asset_id":"ci-01"}}}`,
			missing: "alert_id",
		},
		{
			name:    "missing severity",
			payload: `{"data":{"alert_id":"alert-001","rule_name":"CPU High","fired_at":"2026-03-22T10:00:00Z","source":{"asset_id":"ci-01"}}}`,
			missing: "severity",
		},
		{
			name:    "missing rule_name",
			payload: `{"data":{"alert_id":"alert-001","severity":"critical","fired_at":"2026-03-22T10:00:00Z","source":{"asset_id":"ci-01"}}}`,
			missing: "rule_name",
		},
		{
			name:    "missing fired_at",
			payload: `{"data":{"alert_id":"alert-001","severity":"critical","rule_name":"CPU High","source":{"asset_id":"ci-01"}}}`,
			missing: "fired_at",
		},
		{
			name:    "missing resource_id",
			payload: `{"data":{"alert_id":"alert-001","severity":"critical","rule_name":"CPU High","fired_at":"2026-03-22T10:00:00Z","source":{}}}`,
			missing: "resource_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event AlertFiredEvent
			err := json.Unmarshal([]byte(tt.payload), &event)
			require.NoError(t, err, "JSON should still parse even with missing field")

			switch tt.missing {
			case "alert_id":
				assert.Empty(t, event.Data.AlertID)
			case "severity":
				assert.Empty(t, event.Data.Severity)
			case "rule_name":
				assert.Empty(t, event.Data.RuleName)
			case "fired_at":
				assert.Empty(t, event.Data.FiredAt)
			case "resource_id":
				assert.Empty(t, event.Data.Source.AssetID)
			}
		})
	}
}

// TestAlertFiredEventSchema_ExtraFieldsIgnored verifies forward compatibility:
// new fields added by svc-alert should not break svc-incident deserialization.
func TestAlertFiredEventSchema_ExtraFieldsIgnored(t *testing.T) {
	payload := `{
		"id": "evt-alert-003",
		"type": "alert.fired",
		"source": "svc-alert",
		"time": "2026-03-22T15:00:00Z",
		"data": {
			"alert_id": "alert-003",
			"severity": "medium",
			"rule_name": "Disk IO High",
			"fired_at": "2026-03-22T14:55:00Z",
			"source": {"asset_id": "ci-storage-01", "hostname": "storage-01"},
			"new_v2_field": "should be ignored",
			"escalation_policy": "oncall-team-a"
		}
	}`

	var event AlertFiredEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err, "extra fields should not break deserialization")
	assert.Equal(t, "alert-003", event.Data.AlertID)
	assert.Equal(t, "medium", event.Data.Severity)
	assert.Equal(t, "ci-storage-01", event.Data.Source.AssetID)
}

// mapSeverityToPriority simulates the severity-to-incident-priority mapping
// used by svc-incident's AlertConsumer.
func mapSeverityToPriority(severity string) string {
	switch severity {
	case "critical":
		return "P0"
	case "high", "warning":
		return "P1"
	case "medium":
		return "P2"
	case "low", "info":
		return "P3"
	default:
		return "P3"
	}
}
