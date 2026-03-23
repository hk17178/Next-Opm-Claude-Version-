package biz

import (
	"testing"
	"time"
)

// --- State Machine Tests ---

func TestCanTransition_ValidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
	}{
		{"created->triaging", StatusCreated, StatusTriaging},
		{"created->analyzing", StatusCreated, StatusAnalyzing},
		{"created->assigned", StatusCreated, StatusAssigned},
		{"triaging->analyzing", StatusTriaging, StatusAnalyzing},
		{"triaging->assigned", StatusTriaging, StatusAssigned},
		{"analyzing->assigned", StatusAnalyzing, StatusAssigned},
		{"analyzing->resolving", StatusAnalyzing, StatusResolving},
		{"assigned->resolving", StatusAssigned, StatusResolving},
		{"assigned->analyzing", StatusAssigned, StatusAnalyzing},
		{"resolving->verifying", StatusResolving, StatusVerifying},
		{"resolving->resolved", StatusResolving, StatusResolved},
		{"verifying->resolved", StatusVerifying, StatusResolved},
		{"verifying->resolving", StatusVerifying, StatusResolving},
		{"resolved->postmortem", StatusResolved, StatusPostmortem},
		{"resolved->closed", StatusResolved, StatusClosed},
		{"postmortem->closed", StatusPostmortem, StatusClosed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !CanTransition(tt.from, tt.to) {
				t.Errorf("CanTransition(%s, %s) = false, want true", tt.from, tt.to)
			}
		})
	}
}

func TestCanTransition_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
	}{
		{"created->resolved", StatusCreated, StatusResolved},
		{"created->closed", StatusCreated, StatusClosed},
		{"created->postmortem", StatusCreated, StatusPostmortem},
		{"triaging->resolving", StatusTriaging, StatusResolving},
		{"triaging->closed", StatusTriaging, StatusClosed},
		{"analyzing->triaging", StatusAnalyzing, StatusTriaging},
		{"analyzing->closed", StatusAnalyzing, StatusClosed},
		{"assigned->created", StatusAssigned, StatusCreated},
		{"assigned->closed", StatusAssigned, StatusClosed},
		{"resolving->created", StatusResolving, StatusCreated},
		{"resolving->assigned", StatusResolving, StatusAssigned},
		{"verifying->assigned", StatusVerifying, StatusAssigned},
		{"verifying->closed", StatusVerifying, StatusClosed},
		{"resolved->resolving", StatusResolved, StatusResolving},
		{"resolved->created", StatusResolved, StatusCreated},
		{"postmortem->resolving", StatusPostmortem, StatusResolving},
		{"postmortem->resolved", StatusPostmortem, StatusResolved},
		{"closed->created", StatusClosed, StatusCreated},
		{"closed->resolved", StatusClosed, StatusResolved},
		{"closed->postmortem", StatusClosed, StatusPostmortem},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if CanTransition(tt.from, tt.to) {
				t.Errorf("CanTransition(%s, %s) = true, want false", tt.from, tt.to)
			}
		})
	}
}

func TestCanTransition_UnknownStatus(t *testing.T) {
	if CanTransition(Status("unknown"), StatusCreated) {
		t.Error("CanTransition(unknown, created) = true, want false")
	}
}

func TestCanTransition_SameStatus(t *testing.T) {
	statuses := []Status{
		StatusCreated, StatusTriaging, StatusAnalyzing, StatusAssigned,
		StatusResolving, StatusVerifying, StatusResolved, StatusPostmortem, StatusClosed,
	}
	for _, s := range statuses {
		t.Run(string(s)+"->"+string(s), func(t *testing.T) {
			if CanTransition(s, s) {
				t.Errorf("CanTransition(%s, %s) = true, want false (self-transition)", s, s)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	if !IsTerminal(StatusClosed) {
		t.Error("IsTerminal(closed) = false, want true")
	}

	nonTerminal := []Status{
		StatusCreated, StatusTriaging, StatusAnalyzing, StatusAssigned,
		StatusResolving, StatusVerifying, StatusResolved, StatusPostmortem,
	}
	for _, s := range nonTerminal {
		if IsTerminal(s) {
			t.Errorf("IsTerminal(%s) = true, want false", s)
		}
	}
}

// --- Postmortem Requirement Tests ---

func TestRequiresPostmortem(t *testing.T) {
	tests := []struct {
		severity string
		expected bool
	}{
		{SeverityP0, true},
		{SeverityP1, true},
		{SeverityP2, false},
		{SeverityP3, false},
		{SeverityP4, false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			if got := RequiresPostmortem(tt.severity); got != tt.expected {
				t.Errorf("RequiresPostmortem(%q) = %v, want %v", tt.severity, got, tt.expected)
			}
		})
	}
}

// --- Metrics Calculation Tests ---

func TestCalculateMetrics_FullLifecycle(t *testing.T) {
	detected := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	acknowledged := detected.Add(5 * time.Minute)
	resolved := detected.Add(30 * time.Minute)

	inc := &Incident{
		DetectedAt:     detected,
		AcknowledgedAt: &acknowledged,
		ResolvedAt:     &resolved,
		Status:         StatusResolved,
	}

	m := inc.CalculateMetrics()

	// MTTA: 5 minutes = 300 seconds
	if m.MTTASeconds == nil || *m.MTTASeconds != 300 {
		t.Errorf("MTTA = %v, want 300", m.MTTASeconds)
	}

	// MTTI: same as MTTA when ack is the identification point = 300 seconds
	if m.MTTISeconds == nil || *m.MTTISeconds != 300 {
		t.Errorf("MTTI = %v, want 300", m.MTTISeconds)
	}

	// MTTR: 30 minutes = 1800 seconds
	if m.MTTRSeconds == nil || *m.MTTRSeconds != 1800 {
		t.Errorf("MTTR = %v, want 1800", m.MTTRSeconds)
	}
}

func TestCalculateMetrics_NotAcknowledged(t *testing.T) {
	detected := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	inc := &Incident{
		DetectedAt: detected,
		Status:     StatusCreated,
	}

	m := inc.CalculateMetrics()

	if m.MTTASeconds != nil {
		t.Errorf("MTTA = %v, want nil (not acknowledged)", *m.MTTASeconds)
	}
	if m.MTTISeconds != nil {
		t.Errorf("MTTI = %v, want nil (not identified)", *m.MTTISeconds)
	}
	if m.MTTRSeconds != nil {
		t.Errorf("MTTR = %v, want nil (not resolved)", *m.MTTRSeconds)
	}
}

func TestCalculateMetrics_AcknowledgedNotResolved(t *testing.T) {
	detected := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	acknowledged := detected.Add(3 * time.Minute)
	inc := &Incident{
		DetectedAt:     detected,
		AcknowledgedAt: &acknowledged,
		Status:         StatusAssigned,
	}

	m := inc.CalculateMetrics()

	if m.MTTASeconds == nil || *m.MTTASeconds != 180 {
		t.Errorf("MTTA = %v, want 180", m.MTTASeconds)
	}
	if m.MTTISeconds == nil || *m.MTTISeconds != 180 {
		t.Errorf("MTTI = %v, want 180 (assigned status triggers MTTI)", m.MTTISeconds)
	}
	if m.MTTRSeconds != nil {
		t.Errorf("MTTR = %v, want nil (not resolved)", *m.MTTRSeconds)
	}
}

func TestCalculateMetrics_ZeroDuration(t *testing.T) {
	detected := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	inc := &Incident{
		DetectedAt:     detected,
		AcknowledgedAt: &detected, // instant acknowledgement
		ResolvedAt:     &detected, // instant resolution
		Status:         StatusResolved,
	}

	m := inc.CalculateMetrics()

	if m.MTTASeconds == nil || *m.MTTASeconds != 0 {
		t.Errorf("MTTA = %v, want 0", m.MTTASeconds)
	}
	if m.MTTRSeconds == nil || *m.MTTRSeconds != 0 {
		t.Errorf("MTTR = %v, want 0", m.MTTRSeconds)
	}
}

// --- Full Lifecycle Path Tests ---

func TestStateMachine_HappyPath(t *testing.T) {
	// Verify the happy path: created -> triaging -> analyzing -> assigned -> resolving -> resolved -> closed
	path := []Status{
		StatusCreated, StatusTriaging, StatusAnalyzing, StatusAssigned,
		StatusResolving, StatusResolved, StatusClosed,
	}

	for i := 0; i < len(path)-1; i++ {
		if !CanTransition(path[i], path[i+1]) {
			t.Errorf("Happy path broken at %s -> %s", path[i], path[i+1])
		}
	}
}

func TestStateMachine_PostmortemPath(t *testing.T) {
	// P0/P1 path: ... -> resolved -> postmortem -> closed
	if !CanTransition(StatusResolved, StatusPostmortem) {
		t.Error("resolved -> postmortem should be valid")
	}
	if !CanTransition(StatusPostmortem, StatusClosed) {
		t.Error("postmortem -> closed should be valid")
	}
}

func TestStateMachine_VerificationLoop(t *testing.T) {
	// Verification can bounce back: resolving -> verifying -> resolving
	if !CanTransition(StatusResolving, StatusVerifying) {
		t.Error("resolving -> verifying should be valid")
	}
	if !CanTransition(StatusVerifying, StatusResolving) {
		t.Error("verifying -> resolving should be valid (re-open)")
	}
}

func TestStateMachine_ClosedIsTerminal(t *testing.T) {
	allStatuses := []Status{
		StatusCreated, StatusTriaging, StatusAnalyzing, StatusAssigned,
		StatusResolving, StatusVerifying, StatusResolved, StatusPostmortem, StatusClosed,
	}
	for _, target := range allStatuses {
		if CanTransition(StatusClosed, target) {
			t.Errorf("closed -> %s should not be valid (closed is terminal)", target)
		}
	}
}

// --- Severity Constants ---

func TestSeverityConstants(t *testing.T) {
	if SeverityP0 != "P0" {
		t.Errorf("SeverityP0 = %q, want P0", SeverityP0)
	}
	if SeverityP1 != "P1" {
		t.Errorf("SeverityP1 = %q, want P1", SeverityP1)
	}
	if SeverityP2 != "P2" {
		t.Errorf("SeverityP2 = %q, want P2", SeverityP2)
	}
	if SeverityP3 != "P3" {
		t.Errorf("SeverityP3 = %q, want P3", SeverityP3)
	}
	if SeverityP4 != "P4" {
		t.Errorf("SeverityP4 = %q, want P4", SeverityP4)
	}
}
