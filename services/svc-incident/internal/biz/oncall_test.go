package biz

import (
	"testing"
	"time"
)

func TestGetCurrentOnCallPerson_DailyRotation(t *testing.T) {
	schedule := &OncallSchedule{
		ScheduleID: "sched-001",
		Enabled:    true,
		Rotation: map[string]any{
			"type":       "daily",
			"start_date": "2026-03-01",
			"members": []map[string]any{
				{"user_id": "user-1", "name": "Alice"},
				{"user_id": "user-2", "name": "Bob"},
				{"user_id": "user-3", "name": "Charlie"},
			},
		},
	}

	// Day 0 (March 1) -> Alice (index 0)
	at := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	member := GetCurrentOnCallPerson(schedule, at)
	if member == nil {
		t.Fatal("expected non-nil member")
	}
	if member.UserID != "user-1" {
		t.Errorf("day 0: got %s, want user-1", member.UserID)
	}

	// Day 1 (March 2) -> Bob (index 1)
	at = time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	member = GetCurrentOnCallPerson(schedule, at)
	if member.UserID != "user-2" {
		t.Errorf("day 1: got %s, want user-2", member.UserID)
	}

	// Day 2 (March 3) -> Charlie (index 2)
	at = time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)
	member = GetCurrentOnCallPerson(schedule, at)
	if member.UserID != "user-3" {
		t.Errorf("day 2: got %s, want user-3", member.UserID)
	}

	// Day 3 (March 4) -> wraps back to Alice (index 0)
	at = time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	member = GetCurrentOnCallPerson(schedule, at)
	if member.UserID != "user-1" {
		t.Errorf("day 3: got %s, want user-1 (wrap)", member.UserID)
	}
}

func TestGetCurrentOnCallPerson_WeeklyRotation(t *testing.T) {
	schedule := &OncallSchedule{
		ScheduleID: "sched-002",
		Enabled:    true,
		Rotation: map[string]any{
			"type":       "weekly",
			"start_date": "2026-03-01",
			"members": []map[string]any{
				{"user_id": "user-1", "name": "Alice"},
				{"user_id": "user-2", "name": "Bob"},
			},
		},
	}

	// Week 0 -> Alice
	at := time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)
	member := GetCurrentOnCallPerson(schedule, at)
	if member == nil || member.UserID != "user-1" {
		t.Errorf("week 0: got %v, want user-1", member)
	}

	// Week 1 -> Bob
	at = time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
	member = GetCurrentOnCallPerson(schedule, at)
	if member == nil || member.UserID != "user-2" {
		t.Errorf("week 1: got %v, want user-2", member)
	}

	// Week 2 -> wraps to Alice
	at = time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	member = GetCurrentOnCallPerson(schedule, at)
	if member == nil || member.UserID != "user-1" {
		t.Errorf("week 2: got %v, want user-1 (wrap)", member)
	}
}

func TestGetCurrentOnCallPerson_CustomRotation(t *testing.T) {
	schedule := &OncallSchedule{
		ScheduleID: "sched-003",
		Enabled:    true,
		Rotation: map[string]any{
			"type":       "custom",
			"start_date": "2026-03-01",
			"shift_days": float64(3), // JSON numbers decode as float64
			"members": []map[string]any{
				{"user_id": "user-1", "name": "Alice"},
				{"user_id": "user-2", "name": "Bob"},
			},
		},
	}

	// Day 0-2 (shift 0) -> Alice
	at := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	member := GetCurrentOnCallPerson(schedule, at)
	if member == nil || member.UserID != "user-1" {
		t.Errorf("shift 0: got %v, want user-1", member)
	}

	// Day 3-5 (shift 1) -> Bob
	at = time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	member = GetCurrentOnCallPerson(schedule, at)
	if member == nil || member.UserID != "user-2" {
		t.Errorf("shift 1: got %v, want user-2", member)
	}

	// Day 6-8 (shift 2) -> wraps to Alice
	at = time.Date(2026, 3, 7, 10, 0, 0, 0, time.UTC)
	member = GetCurrentOnCallPerson(schedule, at)
	if member == nil || member.UserID != "user-1" {
		t.Errorf("shift 2: got %v, want user-1 (wrap)", member)
	}
}

func TestGetCurrentOnCallPerson_OverrideTakesPrecedence(t *testing.T) {
	schedule := &OncallSchedule{
		ScheduleID: "sched-004",
		Enabled:    true,
		Rotation: map[string]any{
			"type":       "daily",
			"start_date": "2026-03-01",
			"members": []map[string]any{
				{"user_id": "user-1", "name": "Alice"},
				{"user_id": "user-2", "name": "Bob"},
			},
		},
		Overrides: []map[string]any{
			{
				"user_id":    "user-3",
				"name":       "Charlie (substitute)",
				"start_date": "2026-03-01",
				"end_date":   "2026-03-02",
			},
		},
	}

	// Day 0 would normally be Alice, but Charlie has an override
	at := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	member := GetCurrentOnCallPerson(schedule, at)
	if member == nil || member.UserID != "user-3" {
		t.Errorf("override day: got %v, want user-3 (override)", member)
	}

	// Day 3 is outside the override, normal rotation applies
	at = time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)
	member = GetCurrentOnCallPerson(schedule, at)
	if member == nil || member.UserID != "user-1" {
		t.Errorf("non-override day: got %v, want user-1 (normal rotation)", member)
	}
}

func TestGetCurrentOnCallPerson_EmptyMembers(t *testing.T) {
	schedule := &OncallSchedule{
		ScheduleID: "sched-005",
		Enabled:    true,
		Rotation: map[string]any{
			"type":       "daily",
			"start_date": "2026-03-01",
			"members":    []map[string]any{},
		},
	}

	at := time.Now()
	member := GetCurrentOnCallPerson(schedule, at)
	if member != nil {
		t.Errorf("expected nil for empty members, got %v", member)
	}
}

func TestGetCurrentOnCallPerson_BeforeStartDate(t *testing.T) {
	schedule := &OncallSchedule{
		ScheduleID: "sched-006",
		Enabled:    true,
		Rotation: map[string]any{
			"type":       "daily",
			"start_date": "2026-04-01",
			"members": []map[string]any{
				{"user_id": "user-1", "name": "Alice"},
			},
		},
	}

	// Before start date -> defaults to first member
	at := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	member := GetCurrentOnCallPerson(schedule, at)
	if member == nil || member.UserID != "user-1" {
		t.Errorf("before start: got %v, want user-1 (default)", member)
	}
}

func TestGetCurrentOnCallPerson_NilRotation(t *testing.T) {
	schedule := &OncallSchedule{
		ScheduleID: "sched-007",
		Enabled:    true,
		Rotation:   nil,
	}

	member := GetCurrentOnCallPerson(schedule, time.Now())
	if member != nil {
		t.Errorf("expected nil for nil rotation, got %v", member)
	}
}
