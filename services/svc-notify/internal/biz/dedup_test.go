package biz

import (
	"testing"
)

func TestDedupKey_Deterministic(t *testing.T) {
	k1 := DedupKey(ChannelWecomWebhook, "bot-1", "alert_fired", "CPU高")
	k2 := DedupKey(ChannelWecomWebhook, "bot-1", "alert_fired", "CPU高")

	if k1 != k2 {
		t.Error("same inputs should produce same dedup key")
	}
}

func TestDedupKey_DifferentInputs(t *testing.T) {
	k1 := DedupKey(ChannelWecomWebhook, "bot-1", "alert_fired", "CPU高")
	k2 := DedupKey(ChannelWecomWebhook, "bot-1", "alert_fired", "内存高")
	k3 := DedupKey(ChannelEmail, "bot-1", "alert_fired", "CPU高")

	if k1 == k2 {
		t.Error("different content should produce different keys")
	}
	if k1 == k3 {
		t.Error("different channel should produce different keys")
	}
}

func TestDedupKey_Length(t *testing.T) {
	key := DedupKey(ChannelSMS, "recipient", "node", "content")
	if len(key) != 32 {
		t.Errorf("expected 32 char hex key, got %d", len(key))
	}
}
