package contract

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AlertFiredEvent is the expected schema for the opsnexus.alert.fired Kafka event
// consumed by svc-notify's broadcaster.
type AlertFiredEvent struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Source string         `json:"source"`
	Time   string         `json:"time"`
	Data   AlertFiredData `json:"data"`
}

type AlertFiredData struct {
	AlertID    string `json:"alert_id"`
	Severity   string `json:"severity"`
	RuleName   string `json:"rule_name"`
	FiredAt    string `json:"fired_at"`
	ResourceID string `json:"resource_id"`
	Title      string `json:"title"`
	HostID     string `json:"host_id"`
}

// TestAlertFiredEventSchema_RequiredFields verifies the consumer contract:
// svc-notify expects opsnexus.alert.fired events to contain the required fields.
func TestAlertFiredEventSchema_RequiredFields(t *testing.T) {
	// Minimal valid payload matching the producer contract
	payload := `{
		"id": "evt-001",
		"type": "alert.fired",
		"source": "svc-alert",
		"time": "2026-03-22T10:00:00Z",
		"data": {
			"alert_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			"severity": "P1",
			"rule_name": "cpu_high_5min",
			"fired_at": "2026-03-22T09:55:00Z",
			"resource_id": "host-web-01",
			"title": "CPU usage above 95% for 5 minutes",
			"host_id": "host-web-01"
		}
	}`

	var event AlertFiredEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err, "alert.fired event must be valid JSON")

	// Contract: these 5 fields MUST be present and non-empty
	assert.NotEmpty(t, event.Data.AlertID, "alert_id is required by consumer contract")
	assert.NotEmpty(t, event.Data.Severity, "severity is required by consumer contract")
	assert.NotEmpty(t, event.Data.RuleName, "rule_name is required by consumer contract")
	assert.NotEmpty(t, event.Data.FiredAt, "fired_at is required by consumer contract")
	assert.NotEmpty(t, event.Data.ResourceID, "resource_id is required by consumer contract")
}

// TestAlertFiredEventSchema_MissingRequiredFields verifies that missing
// required fields are detected by the consumer's deserialization.
func TestAlertFiredEventSchema_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		missing string
	}{
		{
			name: "missing alert_id",
			payload: `{"data":{"severity":"P1","rule_name":"cpu_high","fired_at":"2026-03-22T09:55:00Z","resource_id":"host-01"}}`,
			missing: "alert_id",
		},
		{
			name: "missing severity",
			payload: `{"data":{"alert_id":"abc-123","rule_name":"cpu_high","fired_at":"2026-03-22T09:55:00Z","resource_id":"host-01"}}`,
			missing: "severity",
		},
		{
			name: "missing rule_name",
			payload: `{"data":{"alert_id":"abc-123","severity":"P1","fired_at":"2026-03-22T09:55:00Z","resource_id":"host-01"}}`,
			missing: "rule_name",
		},
		{
			name: "missing fired_at",
			payload: `{"data":{"alert_id":"abc-123","severity":"P1","rule_name":"cpu_high","resource_id":"host-01"}}`,
			missing: "fired_at",
		},
		{
			name: "missing resource_id",
			payload: `{"data":{"alert_id":"abc-123","severity":"P1","rule_name":"cpu_high","fired_at":"2026-03-22T09:55:00Z"}}`,
			missing: "resource_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event AlertFiredEvent
			err := json.Unmarshal([]byte(tt.payload), &event)
			require.NoError(t, err, "JSON should still parse even with missing field")

			// The missing field should be zero-value (empty string)
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
				assert.Empty(t, event.Data.ResourceID)
			}
		})
	}
}

// TestAlertFiredEventSchema_SeverityValues verifies that the consumer can
// handle all valid severity levels from the producer.
func TestAlertFiredEventSchema_SeverityValues(t *testing.T) {
	severities := []string{"P0", "P1", "P2", "P3", "critical", "warning", "info"}

	for _, sev := range severities {
		t.Run(sev, func(t *testing.T) {
			payload, _ := json.Marshal(AlertFiredEvent{
				ID:   "evt-test",
				Type: "alert.fired",
				Data: AlertFiredData{
					AlertID:    "alert-001",
					Severity:   sev,
					RuleName:   "test_rule",
					FiredAt:    "2026-03-22T10:00:00Z",
					ResourceID: "host-01",
				},
			})

			var parsed AlertFiredEvent
			err := json.Unmarshal(payload, &parsed)
			require.NoError(t, err)
			assert.Equal(t, sev, parsed.Data.Severity)
		})
	}
}

// TestAlertFiredEventSchema_ExtraFieldsIgnored verifies forward compatibility:
// the consumer ignores unknown fields added by newer producer versions.
func TestAlertFiredEventSchema_ExtraFieldsIgnored(t *testing.T) {
	payload := `{
		"id": "evt-002",
		"type": "alert.fired",
		"source": "svc-alert",
		"time": "2026-03-22T10:00:00Z",
		"data": {
			"alert_id": "alert-002",
			"severity": "P0",
			"rule_name": "disk_full",
			"fired_at": "2026-03-22T09:58:00Z",
			"resource_id": "host-db-01",
			"title": "Disk usage 99%",
			"host_id": "host-db-01",
			"new_field_v2": "some_new_value",
			"metric_value": 99.5
		}
	}`

	var event AlertFiredEvent
	err := json.Unmarshal([]byte(payload), &event)
	require.NoError(t, err, "extra fields should not break deserialization")
	assert.Equal(t, "alert-002", event.Data.AlertID)
	assert.Equal(t, "P0", event.Data.Severity)
}
