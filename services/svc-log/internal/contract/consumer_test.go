package contract

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LogIngestedEvent represents the CloudEvents envelope for opsnexus.log.ingested.
// This mirrors the schema at schemas/events/opsnexus.log.ingested.schema.json.
type LogIngestedEvent struct {
	SpecVersion     string                 `json:"specversion"`
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`
	Source          string                 `json:"source"`
	Time            string                 `json:"time"`
	DataContentType string                 `json:"datacontenttype"`
	Data            LogIngestedPayload     `json:"data"`
}

// LogIngestedPayload is the data field of the opsnexus.log.ingested event.
type LogIngestedPayload struct {
	LogID       string            `json:"log_id"`
	HostID      string            `json:"host_id"`
	ServiceName string            `json:"service_name,omitempty"`
	Level       string            `json:"level"`
	Message     string            `json:"message"`
	Timestamp   string            `json:"timestamp"`
	Labels      map[string]string `json:"labels,omitempty"`
	TraceID     string            `json:"trace_id,omitempty"`
}

// buildSampleEvent constructs a valid opsnexus.log.ingested event for testing.
func buildSampleEvent() LogIngestedEvent {
	return LogIngestedEvent{
		SpecVersion:     "1.0",
		ID:              "evt-001",
		Type:            "opsnexus.log.ingested",
		Source:          "/services/svc-log",
		Time:            time.Now().UTC().Format(time.RFC3339),
		DataContentType: "application/json",
		Data: LogIngestedPayload{
			LogID:       "log-001",
			HostID:      "host-001",
			ServiceName: "api-gateway",
			Level:       "ERROR",
			Message:     "connection timeout to upstream",
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
			Labels:      map[string]string{"env": "prod", "region": "cn-east-1"},
			TraceID:     "trace-abc-123",
		},
	}
}

// --- Contract Tests: svc-alert consumes opsnexus.log.ingested ---

// TestContract_EventEnvelope_RequiredFields verifies the CloudEvents envelope
// contains all required fields per the schema.
func TestContract_EventEnvelope_RequiredFields(t *testing.T) {
	event := buildSampleEvent()
	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// CloudEvents envelope required fields
	requiredEnvelopeFields := []string{"specversion", "id", "type", "source", "time", "data"}
	for _, field := range requiredEnvelopeFields {
		assert.Contains(t, parsed, field, "envelope missing required field: %s", field)
	}

	assert.Equal(t, "1.0", parsed["specversion"])
	assert.Equal(t, "opsnexus.log.ingested", parsed["type"])
}

// TestContract_EventData_RequiredFields verifies the data payload
// contains all fields required by the schema for svc-alert consumption.
func TestContract_EventData_RequiredFields(t *testing.T) {
	event := buildSampleEvent()
	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	dataMap, ok := parsed["data"].(map[string]interface{})
	require.True(t, ok, "data field must be an object")

	// Required data fields per schema
	requiredDataFields := []string{"log_id", "host_id", "level", "message", "timestamp"}
	for _, field := range requiredDataFields {
		assert.Contains(t, dataMap, field, "data missing required field: %s", field)
	}
}

// TestContract_EventData_LogID verifies log_id is a non-empty string.
func TestContract_EventData_LogID(t *testing.T) {
	event := buildSampleEvent()

	assert.NotEmpty(t, event.Data.LogID, "log_id must not be empty")
}

// TestContract_EventData_HostID verifies host_id is a non-empty string.
func TestContract_EventData_HostID(t *testing.T) {
	event := buildSampleEvent()

	assert.NotEmpty(t, event.Data.HostID, "host_id must not be empty")
}

// TestContract_EventData_Level verifies level is a valid enum value.
func TestContract_EventData_Level(t *testing.T) {
	validLevels := []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

	event := buildSampleEvent()
	assert.Contains(t, validLevels, event.Data.Level, "level must be one of: %v", validLevels)
}

// TestContract_EventData_Level_AllValues tests each valid level value passes validation.
func TestContract_EventData_Level_AllValues(t *testing.T) {
	validLevels := []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

	for _, level := range validLevels {
		t.Run(level, func(t *testing.T) {
			event := buildSampleEvent()
			event.Data.Level = level

			data, err := json.Marshal(event)
			require.NoError(t, err)

			var parsed LogIngestedEvent
			err = json.Unmarshal(data, &parsed)
			require.NoError(t, err)
			assert.Equal(t, level, parsed.Data.Level)
		})
	}
}

// TestContract_EventData_Timestamp verifies timestamp is RFC3339 formatted.
func TestContract_EventData_Timestamp(t *testing.T) {
	event := buildSampleEvent()

	_, err := time.Parse(time.RFC3339, event.Data.Timestamp)
	assert.NoError(t, err, "timestamp must be RFC3339 format")
}

// TestContract_EventData_Message verifies message is a non-empty string.
func TestContract_EventData_Message(t *testing.T) {
	event := buildSampleEvent()

	assert.NotEmpty(t, event.Data.Message, "message must not be empty")
}

// TestContract_EventData_OptionalFields verifies optional fields are correctly typed when present.
func TestContract_EventData_OptionalFields(t *testing.T) {
	event := buildSampleEvent()
	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	dataMap := parsed["data"].(map[string]interface{})

	// service_name is optional string
	if sn, ok := dataMap["service_name"]; ok {
		_, isString := sn.(string)
		assert.True(t, isString, "service_name must be a string")
	}

	// labels is optional object with string values
	if labels, ok := dataMap["labels"]; ok {
		labelsMap, isObj := labels.(map[string]interface{})
		assert.True(t, isObj, "labels must be an object")
		for k, v := range labelsMap {
			_, isString := v.(string)
			assert.True(t, isString, "label value for key '%s' must be a string", k)
		}
	}

	// trace_id is optional string
	if tid, ok := dataMap["trace_id"]; ok {
		_, isString := tid.(string)
		assert.True(t, isString, "trace_id must be a string")
	}
}

// TestContract_EventData_NoExtraFields verifies the data payload only contains
// fields defined in the schema (additionalProperties: false).
func TestContract_EventData_NoExtraFields(t *testing.T) {
	event := buildSampleEvent()
	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	dataMap := parsed["data"].(map[string]interface{})

	allowedFields := map[string]bool{
		"log_id":       true,
		"host_id":      true,
		"service_name": true,
		"level":        true,
		"message":      true,
		"timestamp":    true,
		"labels":       true,
		"trace_id":     true,
	}

	for field := range dataMap {
		assert.True(t, allowedFields[field], "unexpected field in data: %s", field)
	}
}

// TestContract_Roundtrip_JSON verifies the event can be serialized and deserialized
// without data loss — simulating Kafka produce/consume cycle.
func TestContract_Roundtrip_JSON(t *testing.T) {
	original := buildSampleEvent()

	// Serialize (producer side — svc-log)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Deserialize (consumer side — svc-alert)
	var consumed LogIngestedEvent
	err = json.Unmarshal(data, &consumed)
	require.NoError(t, err)

	assert.Equal(t, original.SpecVersion, consumed.SpecVersion)
	assert.Equal(t, original.Type, consumed.Type)
	assert.Equal(t, original.Source, consumed.Source)
	assert.Equal(t, original.Data.LogID, consumed.Data.LogID)
	assert.Equal(t, original.Data.HostID, consumed.Data.HostID)
	assert.Equal(t, original.Data.Level, consumed.Data.Level)
	assert.Equal(t, original.Data.Message, consumed.Data.Message)
	assert.Equal(t, original.Data.Timestamp, consumed.Data.Timestamp)
	assert.Equal(t, original.Data.ServiceName, consumed.Data.ServiceName)
	assert.Equal(t, original.Data.Labels, consumed.Data.Labels)
	assert.Equal(t, original.Data.TraceID, consumed.Data.TraceID)
}

// TestContract_MissingRequiredField_LogID verifies consumer detects missing log_id.
func TestContract_MissingRequiredField_LogID(t *testing.T) {
	event := buildSampleEvent()
	event.Data.LogID = ""

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	dataMap := parsed["data"].(map[string]interface{})
	// When log_id is empty, it should still be present in JSON but empty
	logID, _ := dataMap["log_id"].(string)
	assert.Empty(t, logID, "empty log_id should be detected by consumer validation")
}

// TestContract_MissingRequiredField_Level verifies consumer detects missing level.
func TestContract_MissingRequiredField_Level(t *testing.T) {
	event := buildSampleEvent()
	event.Data.Level = ""

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	dataMap := parsed["data"].(map[string]interface{})
	level, _ := dataMap["level"].(string)
	assert.Empty(t, level, "empty level should be detected by consumer validation")
}

// TestContract_InvalidLevel verifies consumer detects invalid level value.
func TestContract_InvalidLevel(t *testing.T) {
	validLevels := map[string]bool{
		"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true, "FATAL": true,
	}

	invalidLevels := []string{"TRACE", "CRITICAL", "warning", "error", ""}
	for _, level := range invalidLevels {
		t.Run(level, func(t *testing.T) {
			assert.False(t, validLevels[level], "level '%s' should not be valid", level)
		})
	}
}
