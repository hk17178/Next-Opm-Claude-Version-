package biz

import (
	"encoding/json"
	"testing"
)

func TestMatchesSeverity_NilFilter(t *testing.T) {
	b := &Broadcaster{}
	rule := &BroadcastRule{SeverityFilter: nil}
	if !b.matchesSeverity(rule, "P0") {
		t.Error("nil filter should match all severities")
	}
}

func TestMatchesSeverity_EmptyFilter(t *testing.T) {
	b := &Broadcaster{}
	rule := &BroadcastRule{SeverityFilter: json.RawMessage(`[]`)}
	if !b.matchesSeverity(rule, "P0") {
		t.Error("empty filter should match all severities")
	}
}

func TestMatchesSeverity_MatchingFilter(t *testing.T) {
	b := &Broadcaster{}
	rule := &BroadcastRule{SeverityFilter: json.RawMessage(`["P0","P1"]`)}

	if !b.matchesSeverity(rule, "P0") {
		t.Error("P0 should match [P0,P1]")
	}
	if !b.matchesSeverity(rule, "P1") {
		t.Error("P1 should match [P0,P1]")
	}
	if b.matchesSeverity(rule, "P2") {
		t.Error("P2 should not match [P0,P1]")
	}
}

func TestMapAlertSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"critical", "P0"},
		{"high", "P1"},
		{"medium", "P2"},
		{"low", "P3"},
		{"info", "P3"},
		{"unknown", "P3"},
	}

	for _, tt := range tests {
		result := mapAlertSeverity(tt.input)
		if result != tt.expected {
			t.Errorf("mapAlertSeverity(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestTruncatePreview(t *testing.T) {
	short := "short text"
	if truncatePreview(short, 200) != short {
		t.Error("short text should not be truncated")
	}

	long := "This is a very long text that exceeds the limit"
	result := truncatePreview(long, 20)
	if len(result) > 24 { // 20 + "..."
		t.Errorf("truncated text too long: %d", len(result))
	}
}

func TestBuildMessage_DefaultBody(t *testing.T) {
	b := &Broadcaster{}
	event := BroadcastEvent{
		Node:     NodeAlertFired,
		Title:    "CPU High",
		Severity: "P0",
	}
	rule := &BroadcastRule{}

	msg := b.buildMessage(event, rule)

	if msg.Subject == "" {
		t.Error("subject should not be empty")
	}
	if msg.Body == "" {
		t.Error("body should not be empty")
	}
}

func TestBuildMessage_WithTemplate(t *testing.T) {
	b := &Broadcaster{}
	event := BroadcastEvent{
		Node:     NodeAlertFired,
		Title:    "CPU High",
		Severity: "P0",
		ExtraVars: map[string]string{
			"host": "web-01",
		},
	}
	rule := &BroadcastRule{
		Template: "Alert: ${title} on ${host} (${severity})",
	}

	msg := b.buildMessage(event, rule)

	if msg.Body != "Alert: CPU High on web-01 (P0)" {
		t.Errorf("unexpected body: %q", msg.Body)
	}
}
