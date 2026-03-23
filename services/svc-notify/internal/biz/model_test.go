package biz

import (
	"testing"
)

func TestChannelTypeConstants(t *testing.T) {
	types := []ChannelType{
		ChannelWecomWebhook,
		ChannelWecomApp,
		ChannelSMS,
		ChannelVoice,
		ChannelEmail,
		ChannelWebhook,
	}

	seen := make(map[ChannelType]bool)
	for _, ct := range types {
		if seen[ct] {
			t.Errorf("duplicate channel type: %s", ct)
		}
		seen[ct] = true
		if ct == "" {
			t.Error("channel type should not be empty")
		}
	}
}

func TestLifecycleNodeConstants(t *testing.T) {
	nodes := []LifecycleNode{
		NodeAlertFired,
		NodeIncidentCreated,
		NodeAIAnalysisDone,
		NodeIncidentAcknowledged,
		NodePhaseChanged,
		NodeIncidentResolved,
		NodePostmortem,
	}

	if len(nodes) != 7 {
		t.Errorf("expected 7 lifecycle nodes, got %d", len(nodes))
	}

	seen := make(map[LifecycleNode]bool)
	for _, n := range nodes {
		if seen[n] {
			t.Errorf("duplicate lifecycle node: %s", n)
		}
		seen[n] = true
	}
}

func TestNotifyStatusConstants(t *testing.T) {
	statuses := []NotifyStatus{StatusSent, StatusFailed, StatusMerged, StatusSuppressed}

	seen := make(map[NotifyStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %s", s)
		}
		seen[s] = true
	}
}
