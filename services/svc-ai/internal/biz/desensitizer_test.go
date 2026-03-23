package biz

import (
	"regexp"
	"testing"

	"github.com/opsnexus/svc-ai/internal/config"
)

func newTestDesensitizer() *Desensitizer {
	d := &Desensitizer{
		enabled:       true,
		blockedFields: map[string]bool{"secret": true, "api_key_encrypted": true},
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
		},
	}
	return d
}

func TestDesensitizer_PasswordRedaction(t *testing.T) {
	d := newTestDesensitizer()
	result, mapping := d.Sanitize("Connection failed: password=MyP@ss123 on server")

	if result == "Connection failed: password=MyP@ss123 on server" {
		t.Error("password should be redacted")
	}
	if len(mapping) == 0 {
		t.Error("mapping should record original value")
	}
}

func TestDesensitizer_IPRedaction(t *testing.T) {
	d := newTestDesensitizer()
	result, _ := d.Sanitize("Server 192.168.1.100 is down, failover to 10.0.0.1")

	if result == "Server 192.168.1.100 is down, failover to 10.0.0.1" {
		t.Error("IP addresses should be redacted")
	}
}

func TestDesensitizer_EmailRedaction(t *testing.T) {
	d := newTestDesensitizer()
	result, _ := d.Sanitize("Contact admin@example.com for support")

	if result == "Contact admin@example.com for support" {
		t.Error("email should be redacted")
	}
}

func TestDesensitizer_DisabledPassthrough(t *testing.T) {
	d := &Desensitizer{enabled: false}
	result, mapping := d.Sanitize("password=secret123")
	if result != "password=secret123" {
		t.Errorf("expected passthrough, got %q", result)
	}
	if mapping != nil {
		t.Error("expected nil mapping when disabled")
	}
}

func TestDesensitizer_EmptyInput(t *testing.T) {
	cfg := config.DesensitizeConfig{Enabled: false}
	d, err := NewDesensitizer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, _ := d.Sanitize("")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestContentHash(t *testing.T) {
	h1 := ContentHash("test input")
	h2 := ContentHash("test input")
	h3 := ContentHash("different input")

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
	if len(h1) != 16 {
		t.Errorf("expected 16 char hex hash, got %d", len(h1))
	}
}

func TestSanitizeMap_RemovesBlockedFields(t *testing.T) {
	d := &Desensitizer{
		enabled:       true,
		blockedFields: map[string]bool{"secret": true, "credential": true},
		patterns:      nil,
		mappings:      make(map[string]string),
	}

	input := map[string]interface{}{
		"name":       "test-model",
		"secret":     "super-secret-key",
		"credential": "cred-value",
		"endpoint":   "https://api.example.com",
	}

	result, _ := d.SanitizeMap(input)

	if _, ok := result["secret"]; ok {
		t.Error("blocked field 'secret' should be removed")
	}
	if _, ok := result["credential"]; ok {
		t.Error("blocked field 'credential' should be removed")
	}
	if result["name"] != "test-model" {
		t.Error("non-blocked field 'name' should be preserved")
	}
	if result["endpoint"] != "https://api.example.com" {
		t.Error("non-blocked field 'endpoint' should be preserved")
	}
}
