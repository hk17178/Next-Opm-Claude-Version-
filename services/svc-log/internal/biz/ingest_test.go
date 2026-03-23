package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Mock implementations ---

type mockLogRepo struct {
	maskingRules []*MaskingRule
}

func (m *mockLogRepo) CreateLogSource(ctx context.Context, src *LogSource) error   { return nil }
func (m *mockLogRepo) GetLogSource(ctx context.Context, id string) (*LogSource, error) {
	return nil, nil
}
func (m *mockLogRepo) ListLogSources(ctx context.Context) ([]*LogSource, error) { return nil, nil }
func (m *mockLogRepo) UpdateLogSource(ctx context.Context, src *LogSource) error { return nil }
func (m *mockLogRepo) DeleteLogSource(ctx context.Context, id string) error      { return nil }

func (m *mockLogRepo) CreateParseRule(ctx context.Context, rule *ParseRule) error   { return nil }
func (m *mockLogRepo) GetParseRule(ctx context.Context, id string) (*ParseRule, error) {
	return nil, nil
}
func (m *mockLogRepo) ListParseRules(ctx context.Context) ([]*ParseRule, error) { return nil, nil }
func (m *mockLogRepo) UpdateParseRule(ctx context.Context, rule *ParseRule) error { return nil }
func (m *mockLogRepo) DeleteParseRule(ctx context.Context, id string) error      { return nil }

func (m *mockLogRepo) CreateMaskingRule(ctx context.Context, rule *MaskingRule) error { return nil }
func (m *mockLogRepo) GetMaskingRule(ctx context.Context, id string) (*MaskingRule, error) {
	return nil, nil
}
func (m *mockLogRepo) ListMaskingRules(ctx context.Context) ([]*MaskingRule, error) {
	return m.maskingRules, nil
}
func (m *mockLogRepo) UpdateMaskingRule(ctx context.Context, rule *MaskingRule) error { return nil }
func (m *mockLogRepo) DeleteMaskingRule(ctx context.Context, id string) error        { return nil }

func (m *mockLogRepo) CreateRetentionPolicy(ctx context.Context, p *RetentionPolicy) error {
	return nil
}
func (m *mockLogRepo) GetRetentionPolicy(ctx context.Context, id string) (*RetentionPolicy, error) {
	return nil, nil
}
func (m *mockLogRepo) ListRetentionPolicies(ctx context.Context) ([]*RetentionPolicy, error) {
	return nil, nil
}
func (m *mockLogRepo) UpdateRetentionPolicy(ctx context.Context, p *RetentionPolicy) error {
	return nil
}
func (m *mockLogRepo) DeleteRetentionPolicy(ctx context.Context, id string) error { return nil }

func (m *mockLogRepo) CreateStream(ctx context.Context, s *Stream) error { return nil }
func (m *mockLogRepo) GetStream(ctx context.Context, id string) (*Stream, error) {
	return nil, nil
}
func (m *mockLogRepo) ListStreams(ctx context.Context, pageSize int, pageToken string) ([]*Stream, string, error) {
	return nil, "", nil
}
func (m *mockLogRepo) DeleteStream(ctx context.Context, id string) error { return nil }

type mockESRepo struct {
	indexed []LogEntry
}

func (m *mockESRepo) BulkIndex(ctx context.Context, entries []LogEntry) error {
	m.indexed = append(m.indexed, entries...)
	return nil
}

func (m *mockESRepo) Search(ctx context.Context, req LogSearchRequest) (*LogSearchResponse, error) {
	return &LogSearchResponse{Total: 0, Entries: nil}, nil
}

func (m *mockESRepo) Aggregate(ctx context.Context, req LogStatsRequest) (*LogStatsResponse, error) {
	return &LogStatsResponse{Total: 0}, nil
}

func (m *mockESRepo) ListIndices(ctx context.Context, pattern string) ([]IndexInfo, error) {
	return nil, nil
}

func (m *mockESRepo) DeleteIndex(ctx context.Context, indexName string) error { return nil }

type mockPublisher struct {
	events []CloudEvent
}

func (m *mockPublisher) Publish(ctx context.Context, topic string, event CloudEvent) error {
	m.events = append(m.events, event)
	return nil
}

func newTestIngestService(maskingRules []*MaskingRule, sensitiveFields []string) (*IngestService, *mockESRepo, *mockPublisher) {
	logRepo := &mockLogRepo{maskingRules: maskingRules}
	esRepo := &mockESRepo{}
	pub := &mockPublisher{}
	svc := NewIngestService(logRepo, esRepo, pub, zap.NewNop(), IngestConfig{
		MaxBatchSize:    100,
		FlushInterval:   1 * time.Hour, // long interval so we control flushes manually
		SensitiveFields: sensitiveFields,
	})
	return svc, esRepo, pub
}

// --- Tests ---

func TestIngestHTTP_BasicAcceptance(t *testing.T) {
	svc, _, _ := newTestIngestService(nil, nil)
	ctx := context.Background()

	req := LogIngestRequest{
		Entries: []LogEntry{
			{Timestamp: time.Now(), Level: LogLevelInfo, Message: "hello world", ServiceName: "test-svc"},
			{Timestamp: time.Now(), Level: LogLevelError, Message: "something failed"},
		},
	}

	resp, err := svc.IngestHTTP(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Accepted != 2 {
		t.Errorf("expected 2 accepted, got %d", resp.Accepted)
	}
	if resp.Rejected != 0 {
		t.Errorf("expected 0 rejected, got %d", resp.Rejected)
	}
}

func TestIngestHTTP_RejectsEmptyMessage(t *testing.T) {
	svc, _, _ := newTestIngestService(nil, nil)
	ctx := context.Background()

	req := LogIngestRequest{
		Entries: []LogEntry{
			{Timestamp: time.Now(), Level: LogLevelInfo, Message: "valid"},
			{Timestamp: time.Now(), Level: LogLevelInfo, Message: ""},
		},
	}

	resp, err := svc.IngestHTTP(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Accepted != 1 {
		t.Errorf("expected 1 accepted, got %d", resp.Accepted)
	}
	if resp.Rejected != 1 {
		t.Errorf("expected 1 rejected, got %d", resp.Rejected)
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(resp.Errors))
	}
	if resp.Errors[0].Index != 1 {
		t.Errorf("expected error at index 1, got %d", resp.Errors[0].Index)
	}
}

func TestIngestHTTP_AssignsIDAndTimestamp(t *testing.T) {
	svc, esRepo, _ := newTestIngestService(nil, nil)
	ctx := context.Background()

	req := LogIngestRequest{
		Entries: []LogEntry{
			{Message: "no timestamp"},
		},
	}

	resp, err := svc.IngestHTTP(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Accepted != 1 {
		t.Fatalf("expected 1 accepted, got %d", resp.Accepted)
	}

	// Flush to get entries into ES
	svc.Flush(ctx)

	if len(esRepo.indexed) != 1 {
		t.Fatalf("expected 1 indexed entry, got %d", len(esRepo.indexed))
	}
	entry := esRepo.indexed[0]
	if entry.ID == "" {
		t.Error("expected non-empty ID")
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
}

func TestIngestHTTP_AppliesMaskingRules(t *testing.T) {
	rules := []*MaskingRule{
		{RuleID: "1", Name: "phone", Pattern: `1[3-9]\d{9}`, Replacement: "***PHONE***", Enabled: true},
		{RuleID: "2", Name: "idcard", Pattern: `\d{17}[\dXx]`, Replacement: "***IDCARD***", Enabled: true},
	}
	svc, esRepo, _ := newTestIngestService(rules, nil)
	ctx := context.Background()

	req := LogIngestRequest{
		Entries: []LogEntry{
			{Message: "user phone is 13800138000 and id is 110101199001011234"},
		},
	}

	svc.IngestHTTP(ctx, req)
	svc.Flush(ctx)

	if len(esRepo.indexed) != 1 {
		t.Fatalf("expected 1 indexed, got %d", len(esRepo.indexed))
	}
	msg := esRepo.indexed[0].Message
	if msg == "user phone is 13800138000 and id is 110101199001011234" {
		t.Error("message was not masked")
	}
	if !containsStr(msg, "***PHONE***") {
		t.Errorf("expected phone masked in message: %s", msg)
	}
	if !containsStr(msg, "***IDCARD***") {
		t.Errorf("expected idcard masked in message: %s", msg)
	}
}

func TestIngestHTTP_MasksLabels(t *testing.T) {
	rules := []*MaskingRule{
		{RuleID: "1", Name: "secret", Pattern: `secret-\w+`, Replacement: "***", Enabled: true},
	}
	svc, esRepo, _ := newTestIngestService(rules, nil)
	ctx := context.Background()

	req := LogIngestRequest{
		Entries: []LogEntry{
			{
				Message: "ok",
				Labels:  map[string]string{"token": "secret-abc123", "env": "prod"},
			},
		},
	}

	svc.IngestHTTP(ctx, req)
	svc.Flush(ctx)

	entry := esRepo.indexed[0]
	if entry.Labels["token"] != "***" {
		t.Errorf("expected label masked, got: %s", entry.Labels["token"])
	}
	if entry.Labels["env"] != "prod" {
		t.Errorf("expected env label unchanged, got: %s", entry.Labels["env"])
	}
}

func TestIngestHTTP_DisabledMaskingRuleSkipped(t *testing.T) {
	rules := []*MaskingRule{
		{RuleID: "1", Name: "disabled", Pattern: `\d+`, Replacement: "***", Enabled: false},
	}
	svc, esRepo, _ := newTestIngestService(rules, nil)
	ctx := context.Background()

	req := LogIngestRequest{
		Entries: []LogEntry{
			{Message: "error code 12345"},
		},
	}

	svc.IngestHTTP(ctx, req)
	svc.Flush(ctx)

	if esRepo.indexed[0].Message != "error code 12345" {
		t.Errorf("disabled rule should not mask: %s", esRepo.indexed[0].Message)
	}
}

func TestMaskSensitiveFields_MasksLabels(t *testing.T) {
	svc, esRepo, _ := newTestIngestService(nil, []string{"password", "token"})
	ctx := context.Background()

	req := LogIngestRequest{
		Entries: []LogEntry{
			{
				Message: "login ok",
				Labels:  map[string]string{"password": "abc123", "user_token": "xyz", "env": "prod"},
			},
		},
	}

	svc.IngestHTTP(ctx, req)
	svc.Flush(ctx)

	entry := esRepo.indexed[0]
	if entry.Labels["password"] != "***MASKED***" {
		t.Errorf("expected password masked, got: %s", entry.Labels["password"])
	}
	if entry.Labels["user_token"] != "***MASKED***" {
		t.Errorf("expected user_token masked, got: %s", entry.Labels["user_token"])
	}
	if entry.Labels["env"] != "prod" {
		t.Errorf("expected env unchanged, got: %s", entry.Labels["env"])
	}
}

func TestMaskSensitiveFields_MasksMessage(t *testing.T) {
	svc, esRepo, _ := newTestIngestService(nil, []string{"password"})
	ctx := context.Background()

	req := LogIngestRequest{
		Entries: []LogEntry{
			{Message: "login with password=supersecret123 user=admin"},
		},
	}

	svc.IngestHTTP(ctx, req)
	svc.Flush(ctx)

	msg := esRepo.indexed[0].Message
	if containsStr(msg, "supersecret123") {
		t.Errorf("password should be masked in message: %s", msg)
	}
	if !containsStr(msg, "password=***MASKED***") {
		t.Errorf("expected masked password in message: %s", msg)
	}
}

func TestFlush_PublishesCloudEvent(t *testing.T) {
	svc, _, pub := newTestIngestService(nil, nil)
	ctx := context.Background()

	req := LogIngestRequest{
		Entries: []LogEntry{
			{Message: "test1", Level: LogLevelInfo, SourceHost: "host1", SourceType: "host"},
			{Message: "test2", Level: LogLevelError, SourceHost: "host1", SourceType: "host"},
		},
	}

	svc.IngestHTTP(ctx, req)
	svc.Flush(ctx)

	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event published, got %d", len(pub.events))
	}
	event := pub.events[0]
	if event.SpecVersion != "1.0" {
		t.Errorf("expected specversion 1.0, got %s", event.SpecVersion)
	}
	if event.Type != "opsnexus.log.ingested" {
		t.Errorf("expected type opsnexus.log.ingested, got %s", event.Type)
	}
	if event.Source != "/services/svc-log" {
		t.Errorf("expected source /services/svc-log, got %s", event.Source)
	}

	data, ok := event.Data.(LogIngestedEventData)
	if !ok {
		t.Fatalf("expected LogIngestedEventData, got %T", event.Data)
	}
	if data.LogCount != 2 {
		t.Errorf("expected log_count 2, got %d", data.LogCount)
	}
	if data.LevelsSummary["INFO"] != 1 || data.LevelsSummary["ERROR"] != 1 {
		t.Errorf("unexpected levels_summary: %v", data.LevelsSummary)
	}
}

func TestFlush_EmptyBuffer_NoPublish(t *testing.T) {
	svc, _, pub := newTestIngestService(nil, nil)
	ctx := context.Background()

	svc.Flush(ctx)

	if len(pub.events) != 0 {
		t.Errorf("expected no events for empty buffer, got %d", len(pub.events))
	}
}

func TestValidateEntry(t *testing.T) {
	tests := []struct {
		name    string
		entry   LogEntry
		wantErr bool
	}{
		{"valid", LogEntry{Message: "hello"}, false},
		{"empty message", LogEntry{Message: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEntry(&tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEntry() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- Specifically requested tests ---

// TestDesensitizePhone verifies phone number masking (13xxxxxxxxx pattern).
func TestDesensitizePhone(t *testing.T) {
	rules := []*MaskingRule{
		{RuleID: "1", Name: "phone", Pattern: `1[3-9]\d{9}`, Replacement: "***PHONE***", Enabled: true},
	}
	svc, esRepo, _ := newTestIngestService(rules, nil)
	ctx := context.Background()

	req := LogIngestRequest{
		Entries: []LogEntry{
			{Message: "user called from 13800138000 and 15912345678"},
		},
	}

	svc.IngestHTTP(ctx, req)
	svc.Flush(ctx)

	if len(esRepo.indexed) != 1 {
		t.Fatalf("expected 1 indexed, got %d", len(esRepo.indexed))
	}
	msg := esRepo.indexed[0].Message
	if containsStr(msg, "13800138000") {
		t.Errorf("phone 13800138000 should be masked in message: %s", msg)
	}
	if containsStr(msg, "15912345678") {
		t.Errorf("phone 15912345678 should be masked in message: %s", msg)
	}
	if !containsStr(msg, "***PHONE***") {
		t.Errorf("expected ***PHONE*** replacement in message: %s", msg)
	}
}

// TestDesensitizeIDCard verifies ID card number masking (18-digit pattern including X suffix).
func TestDesensitizeIDCard(t *testing.T) {
	rules := []*MaskingRule{
		{RuleID: "1", Name: "idcard", Pattern: `\d{17}[\dXx]`, Replacement: "***IDCARD***", Enabled: true},
	}
	svc, esRepo, _ := newTestIngestService(rules, nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		message string
	}{
		{"numeric suffix", "citizen id is 110101199001011234"},
		{"X suffix", "id card 11010119900101123X detected"},
	}
	for _, tt := range tests {
		esRepo.indexed = nil
		req := LogIngestRequest{
			Entries: []LogEntry{
				{Message: tt.message},
			},
		}

		svc.IngestHTTP(ctx, req)
		svc.Flush(ctx)

		if len(esRepo.indexed) != 1 {
			t.Fatalf("[%s] expected 1 indexed, got %d", tt.name, len(esRepo.indexed))
		}
		msg := esRepo.indexed[0].Message
		if !containsStr(msg, "***IDCARD***") {
			t.Errorf("[%s] expected ***IDCARD*** replacement in message: %s", tt.name, msg)
		}
	}
}

// TestParseJSON verifies JSON format parsing extracts fields into a LogEntry.
func TestParseJSON(t *testing.T) {
	svc, esRepo, _ := newTestIngestService(nil, nil)
	ctx := context.Background()

	jsonLog := `{"message": "request completed", "level": "INFO", "service_name": "api-gw", "host_id": "host-01", "trace_id": "abc-123"}`
	value := []byte(jsonLog)

	err := svc.IngestKafka(ctx, nil, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc.Flush(ctx)

	if len(esRepo.indexed) != 1 {
		t.Fatalf("expected 1 indexed, got %d", len(esRepo.indexed))
	}
	entry := esRepo.indexed[0]
	if entry.Message != "request completed" {
		t.Errorf("expected message 'request completed', got '%s'", entry.Message)
	}
	if entry.Level != LogLevelInfo {
		t.Errorf("expected level INFO, got '%s'", entry.Level)
	}
	if entry.ServiceName != "api-gw" {
		t.Errorf("expected service_name 'api-gw', got '%s'", entry.ServiceName)
	}
	if entry.HostID != "host-01" {
		t.Errorf("expected host_id 'host-01', got '%s'", entry.HostID)
	}
	if entry.TraceID != "abc-123" {
		t.Errorf("expected trace_id 'abc-123', got '%s'", entry.TraceID)
	}
}

// TestIngestHTTP verifies the full normal ingest flow with mock repository.
func TestIngestHTTP(t *testing.T) {
	svc, esRepo, pub := newTestIngestService(nil, nil)
	ctx := context.Background()

	req := LogIngestRequest{
		Entries: []LogEntry{
			{Timestamp: time.Now(), Level: LogLevelInfo, Message: "hello world", ServiceName: "test-svc"},
			{Timestamp: time.Now(), Level: LogLevelError, Message: "something failed", HostID: "host-1"},
		},
	}

	resp, err := svc.IngestHTTP(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Accepted != 2 {
		t.Errorf("expected 2 accepted, got %d", resp.Accepted)
	}
	if resp.Rejected != 0 {
		t.Errorf("expected 0 rejected, got %d", resp.Rejected)
	}
	if len(resp.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(resp.Errors))
	}

	// Flush and verify entries reached ES
	svc.Flush(ctx)

	if len(esRepo.indexed) != 2 {
		t.Fatalf("expected 2 indexed entries, got %d", len(esRepo.indexed))
	}
	for i, entry := range esRepo.indexed {
		if entry.ID == "" {
			t.Errorf("entry[%d]: expected non-empty ID", i)
		}
		if entry.CreatedAt.IsZero() {
			t.Errorf("entry[%d]: expected non-zero created_at", i)
		}
	}
	// Verify CloudEvent was published
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(pub.events))
	}
	if pub.events[0].Type != "opsnexus.log.ingested" {
		t.Errorf("expected event type opsnexus.log.ingested, got %s", pub.events[0].Type)
	}
}

// --- Testify-based tests for additional coverage ---

func TestParseJSON_Testify(t *testing.T) {
	svc, esRepo, _ := newTestIngestService(nil, nil)
	ctx := context.Background()

	t.Run("json entry with labels", func(t *testing.T) {
		req := LogIngestRequest{
			Entries: []LogEntry{
				{
					Message: `{"action":"login","user":"admin"}`,
					Level:   LogLevelInfo,
					Labels:  map[string]string{"app": "auth-service"},
				},
			},
		}
		resp, err := svc.IngestHTTP(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Accepted)
	})

	t.Run("json entry with extra fields", func(t *testing.T) {
		req := LogIngestRequest{
			Entries: []LogEntry{
				{
					Message: "structured log",
					Level:   LogLevelDebug,
					Extra:   map[string]any{"request_id": "abc-123", "duration_ms": 42},
				},
			},
		}
		resp, err := svc.IngestHTTP(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Accepted)
	})

	_ = esRepo // esRepo used indirectly
}

func TestParseRegex_Testify(t *testing.T) {
	rules := []*MaskingRule{
		{RuleID: "regex-ip", Pattern: `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`, Replacement: "[IP_MASKED]", Enabled: true},
	}
	svc, esRepo, _ := newTestIngestService(rules, nil)
	ctx := context.Background()

	t.Run("regex masking applied to message", func(t *testing.T) {
		req := LogIngestRequest{
			Entries: []LogEntry{
				{Message: "connection from 192.168.1.100 established"},
			},
		}
		resp, err := svc.IngestHTTP(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Accepted)

		svc.Flush(ctx)
		require.NotEmpty(t, esRepo.indexed)
		assert.Contains(t, esRepo.indexed[len(esRepo.indexed)-1].Message, "[IP_MASKED]")
	})
}

func TestDesensitize_Testify(t *testing.T) {
	t.Run("masks phone number in labels", func(t *testing.T) {
		svc, esRepo, _ := newTestIngestService(nil, []string{"phone"})
		ctx := context.Background()
		req := LogIngestRequest{
			Entries: []LogEntry{
				{Message: "user logged in", Labels: map[string]string{"phone": "13800138000"}},
			},
		}
		resp, err := svc.IngestHTTP(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Accepted)

		svc.Flush(ctx)
		require.NotEmpty(t, esRepo.indexed)
		assert.Equal(t, "***MASKED***", esRepo.indexed[len(esRepo.indexed)-1].Labels["phone"])
	})

	t.Run("masks idcard in labels", func(t *testing.T) {
		svc, esRepo, _ := newTestIngestService(nil, []string{"idcard"})
		ctx := context.Background()
		req := LogIngestRequest{
			Entries: []LogEntry{
				{Message: "verification", Labels: map[string]string{"idcard": "110101199001011234"}},
			},
		}
		resp, err := svc.IngestHTTP(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Accepted)

		svc.Flush(ctx)
		require.NotEmpty(t, esRepo.indexed)
		assert.Equal(t, "***MASKED***", esRepo.indexed[len(esRepo.indexed)-1].Labels["idcard"])
	})

	t.Run("masks sensitive field in message", func(t *testing.T) {
		svc, esRepo, _ := newTestIngestService(nil, []string{"password"})
		ctx := context.Background()
		req := LogIngestRequest{
			Entries: []LogEntry{
				{Message: "login password=mysecretpass done"},
			},
		}
		resp, err := svc.IngestHTTP(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Accepted)

		svc.Flush(ctx)
		require.NotEmpty(t, esRepo.indexed)
		assert.Contains(t, esRepo.indexed[len(esRepo.indexed)-1].Message, "password=***MASKED***")
	})
}

func TestIngestKafka_SingleEntry(t *testing.T) {
	svc, esRepo, _ := newTestIngestService(nil, nil)
	ctx := context.Background()

	value := []byte(`{"message": "kafka log", "level": "INFO"}`)
	err := svc.IngestKafka(ctx, nil, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc.Flush(ctx)

	if len(esRepo.indexed) != 1 {
		t.Fatalf("expected 1 indexed, got %d", len(esRepo.indexed))
	}
	if esRepo.indexed[0].Message != "kafka log" {
		t.Errorf("unexpected message: %s", esRepo.indexed[0].Message)
	}
}

func TestIngestKafka_BatchEntries(t *testing.T) {
	svc, esRepo, _ := newTestIngestService(nil, nil)
	ctx := context.Background()

	value := []byte(`{"entries": [{"message": "log1"}, {"message": "log2"}]}`)
	err := svc.IngestKafka(ctx, nil, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc.Flush(ctx)

	if len(esRepo.indexed) != 2 {
		t.Fatalf("expected 2 indexed, got %d", len(esRepo.indexed))
	}
}

// Helper

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

