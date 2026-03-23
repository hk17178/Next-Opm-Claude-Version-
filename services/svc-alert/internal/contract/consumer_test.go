package contract_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers — mirror the event envelope shapes from the JSON schemas
// ---------------------------------------------------------------------------

// AlertFiredEvent represents the CloudEvents envelope for opsnexus.alert.fired.
type AlertFiredEvent struct {
	SpecVersion     string              `json:"specversion"`
	ID              string              `json:"id"`
	Type            string              `json:"type"`
	Source          string              `json:"source"`
	Time            string              `json:"time"`
	DataContentType string              `json:"datacontenttype,omitempty"`
	Data            AlertFiredEventData `json:"data"`
}

type AlertFiredEventData struct {
	AlertID        string            `json:"alert_id"`
	RuleID         string            `json:"rule_id"`
	Severity       string            `json:"severity"`
	Title          string            `json:"title"`
	Description    string            `json:"description,omitempty"`
	FiredAt        string            `json:"fired_at"`
	HostID         string            `json:"host_id,omitempty"`
	ServiceName    string            `json:"service_name,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	TriggerValues  map[string]any    `json:"trigger_values,omitempty"`
	SourceEventIDs []string          `json:"source_event_ids,omitempty"`
	// Extension: ironclad flag for Layer-0 alerts
	IsIronclad *bool `json:"is_ironclad,omitempty"`
}

// LogIngestedEvent represents the CloudEvents envelope for opsnexus.log.ingested.
type LogIngestedEvent struct {
	SpecVersion     string               `json:"specversion"`
	ID              string               `json:"id"`
	Type            string               `json:"type"`
	Source          string               `json:"source"`
	Time            string               `json:"time"`
	DataContentType string               `json:"datacontenttype,omitempty"`
	Data            LogIngestedEventData  `json:"data"`
}

type LogIngestedEventData struct {
	LogID       string            `json:"log_id"`
	HostID      string            `json:"host_id"`
	ServiceName string            `json:"service_name,omitempty"`
	Level       string            `json:"level"`
	Message     string            `json:"message"`
	Timestamp   string            `json:"timestamp"`
	Labels      map[string]string `json:"labels,omitempty"`
	TraceID     string            `json:"trace_id,omitempty"`
}

// fingerprint mirrors biz.Fingerprint for contract-level verification.
func fingerprint(ruleID string, labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(ruleID)
	b.WriteByte('|')
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
		b.WriteByte(',')
	}

	hash := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("sha256:%x", hash[:16])
}

// validSeverities lists the severity values accepted by the schema.
var validSeverities = map[string]bool{
	"critical": true, // P0
	"high":     true, // P1
	"medium":   true, // P2
	"low":      true, // P3
	"info":     true, // P4
}

// ---------------------------------------------------------------------------
// Test 1: Producer schema — required fields & value constraints
// ---------------------------------------------------------------------------

func TestAlertFiredEvent_ProducerSchema(t *testing.T) {
	now := time.Now().UTC()
	event := AlertFiredEvent{
		SpecVersion:     "1.0",
		ID:              "evt-0001",
		Type:            "opsnexus.alert.fired",
		Source:          "/services/svc-alert",
		Time:            now.Format(time.RFC3339),
		DataContentType: "application/json",
		Data: AlertFiredEventData{
			AlertID:     "ALT-20260322-000001",
			RuleID:      "rule-cpu-high",
			Severity:    "critical",
			Title:       "CPU > 90%",
			Description: "Host web-01 CPU usage exceeded threshold",
			FiredAt:     now.Format(time.RFC3339),
			HostID:      "host-web-01",
			ServiceName: "api-gateway",
			Labels:      map[string]string{"env": "prod", "host": "web-01"},
		},
	}

	// Marshal → unmarshal round-trip to simulate wire format
	raw, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded AlertFiredEvent
	require.NoError(t, json.Unmarshal(raw, &decoded))

	// Required envelope fields
	assert.Equal(t, "1.0", decoded.SpecVersion)
	assert.NotEmpty(t, decoded.ID)
	assert.Equal(t, "opsnexus.alert.fired", decoded.Type)
	assert.NotEmpty(t, decoded.Source)
	assert.NotEmpty(t, decoded.Time)

	// Required data fields
	assert.NotEmpty(t, decoded.Data.AlertID, "alert_id is required")
	assert.NotEmpty(t, decoded.Data.RuleID, "rule_id is required")
	assert.NotEmpty(t, decoded.Data.Severity, "severity is required")
	assert.NotEmpty(t, decoded.Data.Title, "title is required")
	assert.NotEmpty(t, decoded.Data.FiredAt, "fired_at is required")

	// Severity must be a valid enum value
	assert.True(t, validSeverities[decoded.Data.Severity],
		"severity %q must be one of critical/high/medium/low/info", decoded.Data.Severity)

	// fired_at must be RFC3339
	_, parseErr := time.Parse(time.RFC3339, decoded.Data.FiredAt)
	assert.NoError(t, parseErr, "fired_at must be RFC3339 format")
}

// ---------------------------------------------------------------------------
// Test 2: Layer-0 ironclad flag
// ---------------------------------------------------------------------------

func TestAlertFiredEvent_Layer0_IroncladFlag(t *testing.T) {
	t.Run("ironclad alert has is_ironclad=true", func(t *testing.T) {
		ironcladTrue := true
		event := AlertFiredEvent{
			SpecVersion: "1.0",
			ID:          "evt-ironclad-001",
			Type:        "opsnexus.alert.fired",
			Source:      "/services/svc-alert",
			Time:        time.Now().UTC().Format(time.RFC3339),
			Data: AlertFiredEventData{
				AlertID:  "ALT-20260322-000010",
				RuleID:   "rule-disk-full",
				Severity: "critical",
				Title:    "Disk full (ironclad)",
				FiredAt:  time.Now().UTC().Format(time.RFC3339),
				IsIronclad: &ironcladTrue,
			},
		}

		raw, err := json.Marshal(event)
		require.NoError(t, err)

		var decoded AlertFiredEvent
		require.NoError(t, json.Unmarshal(raw, &decoded))

		require.NotNil(t, decoded.Data.IsIronclad, "Layer-0 alert must carry is_ironclad field")
		assert.True(t, *decoded.Data.IsIronclad)
	})

	t.Run("normal alert has is_ironclad=false or absent", func(t *testing.T) {
		// Case A: explicitly false
		ironcladFalse := false
		event := AlertFiredEvent{
			SpecVersion: "1.0",
			ID:          "evt-normal-001",
			Type:        "opsnexus.alert.fired",
			Source:      "/services/svc-alert",
			Time:        time.Now().UTC().Format(time.RFC3339),
			Data: AlertFiredEventData{
				AlertID:    "ALT-20260322-000020",
				RuleID:     "rule-latency",
				Severity:   "medium",
				Title:      "High latency",
				FiredAt:    time.Now().UTC().Format(time.RFC3339),
				IsIronclad: &ironcladFalse,
			},
		}

		raw, err := json.Marshal(event)
		require.NoError(t, err)

		var decoded AlertFiredEvent
		require.NoError(t, json.Unmarshal(raw, &decoded))

		if decoded.Data.IsIronclad != nil {
			assert.False(t, *decoded.Data.IsIronclad)
		}

		// Case B: field absent (omitempty)
		eventNoField := AlertFiredEvent{
			SpecVersion: "1.0",
			ID:          "evt-normal-002",
			Type:        "opsnexus.alert.fired",
			Source:      "/services/svc-alert",
			Time:        time.Now().UTC().Format(time.RFC3339),
			Data: AlertFiredEventData{
				AlertID:  "ALT-20260322-000021",
				RuleID:   "rule-latency",
				Severity: "low",
				Title:    "Slow query",
				FiredAt:  time.Now().UTC().Format(time.RFC3339),
			},
		}

		raw2, err := json.Marshal(eventNoField)
		require.NoError(t, err)

		// Verify the key is absent from JSON
		var rawMap map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw2, &rawMap))
		var dataMap map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(rawMap["data"], &dataMap))
		_, hasIronclad := dataMap["is_ironclad"]
		assert.False(t, hasIronclad, "is_ironclad should be absent for normal alerts when nil")
	})
}

// ---------------------------------------------------------------------------
// Test 3: Consumer schema — svc-alert consumes opsnexus.log.ingested
// ---------------------------------------------------------------------------

func TestLogIngestedEvent_ConsumerSchema(t *testing.T) {
	event := LogIngestedEvent{
		SpecVersion:     "1.0",
		ID:              "evt-log-001",
		Type:            "opsnexus.log.ingested",
		Source:          "/services/svc-log",
		Time:            time.Now().UTC().Format(time.RFC3339),
		DataContentType: "application/json",
		Data: LogIngestedEventData{
			LogID:       "log-abc-123",
			HostID:      "host-web-01",
			ServiceName: "api-gateway",
			Level:       "ERROR",
			Message:     "connection refused to database primary",
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
			Labels:      map[string]string{"env": "prod"},
			TraceID:     "trace-xyz-789",
		},
	}

	raw, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded LogIngestedEvent
	require.NoError(t, json.Unmarshal(raw, &decoded))

	// Required fields that svc-alert depends on
	assert.NotEmpty(t, decoded.Data.LogID, "log_id is required")
	assert.NotEmpty(t, decoded.Data.HostID, "host_id is required")
	assert.NotEmpty(t, decoded.Data.Level, "level is required")
	assert.NotEmpty(t, decoded.Data.Message, "message is required")
	assert.NotEmpty(t, decoded.Data.Timestamp, "timestamp is required")

	// Level must be a valid enum
	validLevels := map[string]bool{"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true, "FATAL": true}
	assert.True(t, validLevels[decoded.Data.Level],
		"level %q must be one of DEBUG/INFO/WARN/ERROR/FATAL", decoded.Data.Level)

	// Timestamp must be RFC3339
	_, parseErr := time.Parse(time.RFC3339, decoded.Data.Timestamp)
	assert.NoError(t, parseErr, "timestamp must be RFC3339 format")
}

// ---------------------------------------------------------------------------
// Test 4: Forward compatibility — extra fields ignored
// ---------------------------------------------------------------------------

func TestAlertFiredEvent_ExtraFieldsIgnored(t *testing.T) {
	// Simulate a producer that adds new fields the consumer doesn't know about
	rawJSON := `{
		"specversion": "1.0",
		"id": "evt-compat-001",
		"type": "opsnexus.alert.fired",
		"source": "/services/svc-alert",
		"time": "2026-03-22T10:00:00Z",
		"data": {
			"alert_id": "ALT-20260322-000100",
			"rule_id": "rule-mem-high",
			"severity": "high",
			"title": "Memory pressure",
			"fired_at": "2026-03-22T10:00:00Z",
			"new_field_v2": "some-future-value",
			"another_future_field": 42
		}
	}`

	var decoded AlertFiredEvent
	err := json.Unmarshal([]byte(rawJSON), &decoded)
	require.NoError(t, err, "unknown fields must not break deserialization")

	// Core fields still intact
	assert.Equal(t, "ALT-20260322-000100", decoded.Data.AlertID)
	assert.Equal(t, "rule-mem-high", decoded.Data.RuleID)
	assert.Equal(t, "high", decoded.Data.Severity)
	assert.Equal(t, "Memory pressure", decoded.Data.Title)
	assert.Equal(t, "2026-03-22T10:00:00Z", decoded.Data.FiredAt)
}

// ---------------------------------------------------------------------------
// Test 5: Fingerprint uniqueness and stability
// ---------------------------------------------------------------------------

func TestAlertFiredEvent_FingerprintUniqueness(t *testing.T) {
	t.Run("same rule_id + labels produce identical fingerprint", func(t *testing.T) {
		labels := map[string]string{"host": "web-01", "env": "prod"}
		fp1 := fingerprint("rule-cpu-high", labels)
		fp2 := fingerprint("rule-cpu-high", labels)
		assert.Equal(t, fp1, fp2, "fingerprint must be deterministic")
		assert.True(t, strings.HasPrefix(fp1, "sha256:"), "fingerprint must start with sha256:")
	})

	t.Run("different labels produce different fingerprint", func(t *testing.T) {
		labelsA := map[string]string{"host": "web-01", "env": "prod"}
		labelsB := map[string]string{"host": "web-02", "env": "prod"}
		fpA := fingerprint("rule-cpu-high", labelsA)
		fpB := fingerprint("rule-cpu-high", labelsB)
		assert.NotEqual(t, fpA, fpB, "different labels must yield different fingerprints")
	})

	t.Run("different rule_id produce different fingerprint", func(t *testing.T) {
		labels := map[string]string{"host": "web-01"}
		fpA := fingerprint("rule-cpu-high", labels)
		fpB := fingerprint("rule-mem-high", labels)
		assert.NotEqual(t, fpA, fpB, "different rule_id must yield different fingerprints")
	})
}
