package biz

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// --- Mock repositories ---

type mockRuleRepo struct {
	rules []*AlertRule
}

func (m *mockRuleRepo) Create(rule *AlertRule) error { m.rules = append(m.rules, rule); return nil }
func (m *mockRuleRepo) Update(rule *AlertRule) error { return nil }
func (m *mockRuleRepo) Delete(id string) error       { return nil }
func (m *mockRuleRepo) GetByID(id string) (*AlertRule, error) {
	for _, r := range m.rules {
		if r.RuleID == id {
			return r, nil
		}
	}
	return nil, nil
}
func (m *mockRuleRepo) List(enabled *bool, pageSize int, pageToken string) ([]*AlertRule, string, error) {
	return m.rules, "", nil
}
func (m *mockRuleRepo) ListByLayerAndType(layer int, ruleType RuleType) ([]*AlertRule, error) {
	var result []*AlertRule
	for _, r := range m.rules {
		if r.Layer == layer && r.RuleType == ruleType && r.Enabled {
			result = append(result, r)
		}
	}
	return result, nil
}
func (m *mockRuleRepo) ListEnabled() ([]*AlertRule, error) {
	var result []*AlertRule
	for _, r := range m.rules {
		if r.Enabled {
			result = append(result, r)
		}
	}
	return result, nil
}

type mockAlertRepo struct {
	alerts  []*Alert
	nextSeq int64
}

func (m *mockAlertRepo) Create(alert *Alert) error {
	m.alerts = append(m.alerts, alert)
	return nil
}
func (m *mockAlertRepo) GetByID(id string) (*Alert, error) {
	for _, a := range m.alerts {
		if a.AlertID == id {
			return a, nil
		}
	}
	return nil, nil
}
func (m *mockAlertRepo) UpdateStatus(id string, status AlertStatus, ackAt, resAt *time.Time) error {
	for _, a := range m.alerts {
		if a.AlertID == id {
			a.Status = status
			a.AcknowledgedAt = ackAt
			a.ResolvedAt = resAt
		}
	}
	return nil
}
func (m *mockAlertRepo) GetByFingerprint(fingerprint string) (*Alert, error) {
	for _, a := range m.alerts {
		if a.Fingerprint == fingerprint && a.Status == AlertStatusFiring {
			return a, nil
		}
	}
	return nil, nil
}
func (m *mockAlertRepo) GetActiveByRuleID(ruleID string) ([]*Alert, error) {
	var result []*Alert
	for _, a := range m.alerts {
		if a.RuleID == ruleID && a.Status == AlertStatusFiring {
			result = append(result, a)
		}
	}
	return result, nil
}
func (m *mockAlertRepo) List(status *AlertStatus, severity *Severity, pageSize int, pageToken string) ([]*Alert, string, error) {
	return m.alerts, "", nil
}
func (m *mockAlertRepo) IncrementSuppression(id string, suppressedBy string) error {
	for _, a := range m.alerts {
		if a.AlertID == id {
			a.Suppressed = true
			a.SuppressedBy = suppressedBy
		}
	}
	return nil
}
func (m *mockAlertRepo) NextAlertID() (string, error) {
	m.nextSeq++
	return "ALT-" + time.Now().Format("20060102") + "-" + padInt(m.nextSeq), nil
}

func padInt(n int64) string {
	s := ""
	for i := int64(100); i >= 1; i /= 10 {
		s += string(rune('0' + (n/i)%10))
	}
	return s
}

// --- Test helpers ---

func newTestEngine(rules []*AlertRule) (*AlertEngine, *mockAlertRepo) {
	ruleRepo := &mockRuleRepo{rules: rules}
	alertRepo := &mockAlertRepo{}
	baseline := NewBaselineTracker(5)
	dedup := NewDeduplicator(5 * time.Minute)
	logger := zap.NewNop().Sugar()

	engine := NewAlertEngine(ruleRepo, alertRepo, baseline, dedup, logger)
	return engine, alertRepo
}

func mustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// ===========================================
// Layer 0: Ironclad (iron-law alerts)
// ===========================================

func TestLayer0_IroncladBypassDedup(t *testing.T) {
	rules := []*AlertRule{
		{
			RuleID: "rule-ironclad-1", Name: "Host Down", Layer: 0,
			RuleType: RuleTypeThreshold, Severity: SeverityCritical,
			Enabled: true, Ironclad: true,
			Condition: mustJSON(ThresholdCondition{
				MetricName: "host_alive", Operator: "<", Threshold: 1.0,
			}),
		},
	}

	engine, alertRepo := newTestEngine(rules)

	sample := MetricSample{
		MetricName: "host_alive", Value: 0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "pay-01"},
	}

	// First fire
	r1, _ := engine.EvaluateMetric(sample)
	if !r1.Fired {
		t.Fatal("ironclad alert should fire")
	}

	// Second fire — ironclad bypasses dedup
	r2, _ := engine.EvaluateMetric(sample)
	if !r2.Fired {
		t.Error("ironclad alert should bypass dedup and fire again")
	}

	if len(alertRepo.alerts) != 2 {
		t.Errorf("expected 2 ironclad alerts, got %d", len(alertRepo.alerts))
	}
}

// ===========================================
// Layer 1: Static Rules — Threshold
// ===========================================

func TestLayer1_ThresholdGT(t *testing.T) {
	rules := []*AlertRule{
		{
			RuleID: "rule-cpu-1", Name: "High CPU", Layer: 1,
			RuleType: RuleTypeThreshold, Severity: SeverityHigh,
			Enabled: true, CooldownMinutes: 5,
			Condition: mustJSON(ThresholdCondition{
				MetricName: "cpu_usage", Operator: ">", Threshold: 90.0,
			}),
		},
	}

	engine, alertRepo := newTestEngine(rules)

	// Below threshold — no fire
	r1, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "cpu_usage", Value: 80.0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "web-1"},
	})
	if r1.Fired {
		t.Error("should not fire below threshold")
	}

	// Above threshold — fire
	r2, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "cpu_usage", Value: 95.3, Timestamp: time.Now(),
		Labels: map[string]string{"host": "web-1"},
	})
	if !r2.Fired {
		t.Error("should fire above threshold")
	}
	if len(alertRepo.alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alertRepo.alerts))
	}
}

func TestLayer1_ThresholdLTE(t *testing.T) {
	rules := []*AlertRule{
		{
			RuleID: "rule-mem-1", Name: "Low Memory", Layer: 1,
			RuleType: RuleTypeThreshold, Severity: SeverityMedium,
			Enabled: true,
			Condition: mustJSON(ThresholdCondition{
				MetricName: "mem_free_mb", Operator: "<=", Threshold: 100.0,
			}),
		},
	}

	engine, _ := newTestEngine(rules)

	r1, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "mem_free_mb", Value: 100.0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "db-1"},
	})
	if !r1.Fired {
		t.Error("should fire at boundary (100 <= 100)")
	}

	r2, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "mem_free_mb", Value: 200.0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "db-2"},
	})
	if r2.Fired {
		t.Error("should not fire for 200 <= 100")
	}
}

// ===========================================
// Layer 1: Static Rules — Keyword
// ===========================================

func TestLayer1_KeywordMatch(t *testing.T) {
	rules := []*AlertRule{
		{
			RuleID: "rule-oom-1", Name: "OOM Detected", Layer: 1,
			RuleType: RuleTypeKeyword, Severity: SeverityCritical,
			Enabled: true,
			Condition: mustJSON(KeywordCondition{
				Pattern: "(?i)out of memory|oom",
			}),
		},
	}

	engine, alertRepo := newTestEngine(rules)

	r1, _ := engine.EvaluateLog(LogEvent{
		Source:    "kernel",
		Level:     "error",
		Message:   "Process killed: Out of Memory",
		Labels:    map[string]string{"host": "worker-3"},
		Timestamp: time.Now(),
	})
	if !r1.Fired {
		t.Error("keyword pattern should match OOM log")
	}
	if alertRepo.alerts[0].Severity != SeverityCritical {
		t.Errorf("expected severity=critical, got %s", alertRepo.alerts[0].Severity)
	}

	// Non-matching log
	r2, _ := engine.EvaluateLog(LogEvent{
		Source:  "app",
		Level:   "info",
		Message: "Request completed successfully",
		Labels:  map[string]string{"host": "worker-4"},
		Timestamp: time.Now(),
	})
	if r2.Fired {
		t.Error("keyword should not match normal log")
	}
}

// ===========================================
// Layer 2: Baseline anomaly detection
// ===========================================

func TestLayer2_BaselineAnomaly(t *testing.T) {
	rules := []*AlertRule{
		{
			RuleID: "rule-baseline-1", Name: "CPU Baseline", Layer: 2,
			RuleType: RuleTypeBaseline, Severity: SeverityMedium,
			Enabled: true,
			Condition: mustJSON(map[string]interface{}{
				"metric_name":   "cpu_baseline",
				"deviation_pct": 30.0,
			}),
		},
	}

	engine, _ := newTestEngine(rules)

	// Feed stable samples to build baseline (mean ~50)
	for i := 0; i < 10; i++ {
		engine.EvaluateMetric(MetricSample{
			MetricName: "cpu_baseline", Value: 50.0 + float64(i%3),
			Timestamp: time.Now(), Labels: map[string]string{"host": "app-1"},
		})
	}

	// Anomalous value: 200 is 300% deviation from mean ~50 (> 30%)
	r, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "cpu_baseline", Value: 200.0,
		Timestamp: time.Now(), Labels: map[string]string{"host": "app-2"},
	})
	if !r.Fired {
		t.Error("expected baseline anomaly for value 200 when mean ~50")
	}
}

// ===========================================
// Layer 3: Deduplication
// ===========================================

func TestLayer3_Deduplication(t *testing.T) {
	rules := []*AlertRule{
		{
			RuleID: "rule-disk-1", Name: "Disk Full", Layer: 1,
			RuleType: RuleTypeThreshold, Severity: SeverityCritical,
			Enabled: true,
			Condition: mustJSON(ThresholdCondition{
				MetricName: "disk_pct", Operator: ">", Threshold: 95.0,
			}),
		},
	}

	engine, alertRepo := newTestEngine(rules)

	sample := MetricSample{
		MetricName: "disk_pct", Value: 99.0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "storage-1", "mount": "/data"},
	}

	// First fire
	r1, _ := engine.EvaluateMetric(sample)
	if !r1.Fired {
		t.Fatal("first alert should fire")
	}

	// Second evaluation — deduplicated
	r2, _ := engine.EvaluateMetric(sample)
	if r2.Fired {
		t.Error("second alert should be deduplicated")
	}

	if len(alertRepo.alerts) != 1 {
		t.Errorf("expected 1 persisted alert, got %d", len(alertRepo.alerts))
	}
}

// ===========================================
// Layer 4: Full pipeline — alert fields
// ===========================================

func TestLayer4_AlertFields(t *testing.T) {
	rules := []*AlertRule{
		{
			RuleID: "rule-lat-1", Name: "High Latency", Layer: 1,
			RuleType: RuleTypeThreshold, Severity: SeverityCritical,
			Enabled: true,
			Condition: mustJSON(ThresholdCondition{
				MetricName: "http_latency_ms", Operator: ">", Threshold: 500.0,
			}),
		},
	}

	engine, alertRepo := newTestEngine(rules)

	r, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "http_latency_ms", Value: 750.0, Timestamp: time.Now(),
		Labels: map[string]string{"endpoint": "/api/v1/users"},
		HostID: "web-01", Service: "api-gateway",
	})

	if !r.Fired || len(r.Alerts) != 1 {
		t.Fatal("expected 1 alert to fire")
	}

	alert := r.Alerts[0]
	if alert.RuleID != "rule-lat-1" {
		t.Errorf("expected rule_id=rule-lat-1, got %s", alert.RuleID)
	}
	if alert.Severity != SeverityCritical {
		t.Errorf("expected severity=critical, got %s", alert.Severity)
	}
	if alert.Status != AlertStatusFiring {
		t.Errorf("expected status=firing, got %s", alert.Status)
	}
	if alert.Title != "High Latency" {
		t.Errorf("expected title='High Latency', got %s", alert.Title)
	}
	if alert.SourceHost != "web-01" {
		t.Errorf("expected source_host=web-01, got %s", alert.SourceHost)
	}

	// Alert ID format: ALT-YYYYMMDD-NNN
	if !strings.HasPrefix(alert.AlertID, "ALT-") {
		t.Errorf("alert_id should start with ALT-, got %s", alert.AlertID)
	}

	// Fingerprint format: sha256:...
	if !strings.HasPrefix(alert.Fingerprint, "sha256:") {
		t.Errorf("fingerprint should start with sha256:, got %s", alert.Fingerprint)
	}

	if len(alertRepo.alerts) != 1 {
		t.Errorf("expected 1 persisted alert, got %d", len(alertRepo.alerts))
	}
}

// ===========================================
// Disabled rule
// ===========================================

func TestDisabledRule(t *testing.T) {
	rules := []*AlertRule{
		{
			RuleID: "rule-disabled", Name: "Disabled", Layer: 1,
			RuleType: RuleTypeThreshold, Severity: SeverityCritical,
			Enabled: false,
			Condition: mustJSON(ThresholdCondition{
				MetricName: "test_metric", Operator: ">", Threshold: 10.0,
			}),
		},
	}

	engine, _ := newTestEngine(rules)

	r, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "test_metric", Value: 100.0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "test"},
	})
	if r.Fired {
		t.Error("disabled rule should not fire")
	}
}

// ===========================================
// Fingerprint stability & format
// ===========================================

func TestFingerprintFormat(t *testing.T) {
	labels := map[string]string{"host": "web-1", "env": "prod", "region": "cn-east-1"}

	fp1 := Fingerprint("rule-1", labels)
	fp2 := Fingerprint("rule-1", labels)

	if fp1 != fp2 {
		t.Errorf("fingerprints should be stable: %s != %s", fp1, fp2)
	}
	if !strings.HasPrefix(fp1, "sha256:") {
		t.Errorf("fingerprint should have sha256: prefix, got %s", fp1)
	}

	fp3 := Fingerprint("rule-2", labels)
	if fp1 == fp3 {
		t.Error("different rule IDs should produce different fingerprints")
	}
}

// ===========================================
// Severity values match spec
// ===========================================

func TestSeverityValues(t *testing.T) {
	expected := []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo}
	values := []string{"critical", "high", "medium", "low", "info"}

	for i, sev := range expected {
		if string(sev) != values[i] {
			t.Errorf("severity %d: expected %s, got %s", i, values[i], sev)
		}
	}
}
