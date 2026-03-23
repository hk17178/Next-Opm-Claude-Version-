package biz

import (
	"regexp"
	"testing"
	"time"

	"github.com/opsnexus/svc-ai/internal/config"
	"go.uber.org/zap"
)

func TestDesensitizationComplexity(t *testing.T) {
	d := &Desensitizer{
		enabled:       true,
		blockedFields: map[string]bool{"api_key": true, "secret": true},
		mappings:      make(map[string]string),
		patterns: []sensitivePattern{
			{
				Name:        "password_field",
				Replacement: "$1=***REDACTED***",
				compiled:    regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*\S+`),
			},
			{
				Name:        "ip_address",
				Replacement: "***IP***",
				compiled:    regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
			},
			{
				Name:        "email_address",
				Replacement: "***EMAIL***",
				compiled:    regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
			},
			{
				Name:        "bearer_token",
				Replacement: "Bearer ***TOKEN***",
				compiled:    regexp.MustCompile(`Bearer\s+[A-Za-z0-9\-._~+/]+=*`),
			},
		},
	}

	// Complex text containing multiple types of sensitive information
	input := `Alert on host 192.168.1.100: service crashed.
Connection string: password=SuperS3cret! host=10.0.0.55 port=5432
Contact: ops-team@example.com or admin@internal.corp for escalation.
api_key=sk-live-abc123 was exposed in logs.
Auth header: Bearer eyJhbGciOiJIUzI1NiJ9.payload.signature
Secondary host 172.16.0.42 also affected.`

	result, mapping := d.Sanitize(input)

	// Verify IPs are redacted
	if regexp.MustCompile(`\b192\.168\.1\.100\b`).MatchString(result) {
		t.Error("IP 192.168.1.100 should be redacted")
	}
	if regexp.MustCompile(`\b10\.0\.0\.55\b`).MatchString(result) {
		t.Error("IP 10.0.0.55 should be redacted")
	}
	if regexp.MustCompile(`\b172\.16\.0\.42\b`).MatchString(result) {
		t.Error("IP 172.16.0.42 should be redacted")
	}

	// Verify password is redacted
	if regexp.MustCompile(`SuperS3cret`).MatchString(result) {
		t.Error("password value should be redacted")
	}

	// Verify emails are redacted
	if regexp.MustCompile(`ops-team@example\.com`).MatchString(result) {
		t.Error("email ops-team@example.com should be redacted")
	}
	if regexp.MustCompile(`admin@internal\.corp`).MatchString(result) {
		t.Error("email admin@internal.corp should be redacted")
	}

	// Verify bearer token is redacted
	if regexp.MustCompile(`eyJhbGciOiJIUzI1NiJ9`).MatchString(result) {
		t.Error("bearer token should be redacted")
	}

	// Verify mapping records originals for traceability
	if len(mapping) == 0 {
		t.Error("mapping should contain original values for reverse lookup")
	}

	// Verify non-sensitive content is preserved
	if !regexp.MustCompile(`Alert on host`).MatchString(result) {
		t.Error("non-sensitive prefix text should be preserved")
	}
	if !regexp.MustCompile(`service crashed`).MatchString(result) {
		t.Error("non-sensitive context should be preserved")
	}

	// SanitizeMap: verify blocked fields are removed entirely
	mapInput := map[string]interface{}{
		"host":    "db-primary 10.0.0.55",
		"api_key": "sk-live-abc123",
		"secret":  "top-secret-value",
		"message": "User admin@example.com reported issue on 192.168.1.1",
	}

	mapResult, mapMapping := d.SanitizeMap(mapInput)

	if _, ok := mapResult["api_key"]; ok {
		t.Error("blocked field 'api_key' should be removed from map")
	}
	if _, ok := mapResult["secret"]; ok {
		t.Error("blocked field 'secret' should be removed from map")
	}
	if _, ok := mapResult["host"]; !ok {
		t.Error("non-blocked field 'host' should be preserved")
	}

	// Verify string values in map are also desensitized
	if msg, ok := mapResult["message"].(string); ok {
		if regexp.MustCompile(`admin@example\.com`).MatchString(msg) {
			t.Error("email in map value should be redacted")
		}
		if regexp.MustCompile(`192\.168\.1\.1\b`).MatchString(msg) {
			t.Error("IP in map value should be redacted")
		}
	}

	if len(mapMapping) == 0 {
		t.Error("map sanitization should produce mappings")
	}
}

func TestCircuitBreakerStateTransition(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cb := NewCircuitBreaker(config.CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		TimeoutSeconds:   1,
	}, logger)

	// Phase 1: Closed state (initial)
	if cb.State() != StateClosed {
		t.Fatalf("expected initial state closed, got %s", cb.State())
	}
	if !cb.Allow() {
		t.Fatal("closed breaker should allow requests")
	}

	// Phase 2: Closed -> Open (after failure threshold)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Fatalf("should still be closed after 2 failures (threshold=3), got %s", cb.State())
	}

	cb.RecordFailure() // 3rd failure hits threshold
	if cb.State() != StateOpen {
		t.Fatalf("expected open after 3 failures, got %s", cb.State())
	}

	// Phase 3: Open state blocks requests
	if cb.Allow() {
		t.Fatal("open breaker should block requests within timeout")
	}

	// Phase 4: Open -> HalfOpen (after timeout expires)
	// Wait for timeout to expire
	time.Sleep(1100 * time.Millisecond)

	if !cb.Allow() {
		t.Fatal("breaker should allow probe request after timeout (transitioning to half-open)")
	}
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected half-open after timeout, got %s", cb.State())
	}

	// Phase 5: HalfOpen -> Open (on failure during half-open)
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("expected re-open after failure in half-open, got %s", cb.State())
	}

	// Phase 6: Open -> HalfOpen -> Closed (on sufficient successes)
	time.Sleep(1100 * time.Millisecond)

	if !cb.Allow() {
		t.Fatal("breaker should allow probe after second timeout")
	}
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected half-open, got %s", cb.State())
	}

	cb.RecordSuccess()
	if cb.State() != StateHalfOpen {
		t.Fatalf("should still be half-open after 1 success (threshold=2), got %s", cb.State())
	}

	cb.RecordSuccess() // 2nd success hits threshold
	if cb.State() != StateClosed {
		t.Fatalf("expected closed after 2 successes in half-open, got %s", cb.State())
	}

	// Phase 7: Verify fully recovered - counters reset
	if !cb.Allow() {
		t.Fatal("recovered breaker should allow requests")
	}

	// Need full 3 failures again to re-open
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Fatalf("should still be closed after 2 failures post-recovery, got %s", cb.State())
	}
}
