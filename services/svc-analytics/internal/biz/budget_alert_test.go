package biz

import (
	"testing"
	"time"
)

// TestCheckBudgetAlert_NilWhenHealthy verifies no alert fires when budget is healthy (> 20%).
func TestCheckBudgetAlert_NilWhenHealthy(t *testing.T) {
	tests := []struct {
		name   string
		remain float64
	}{
		{"50% remaining", 50.0},
		{"exactly 20% remaining", 20.0},
		{"100% remaining", 100.0},
		{"barely above warning", 20.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &SLAResult{
				ConfigID:          "cfg-healthy",
				Dimension:         SLALevelBusinessUnit,
				DimensionValue:    "payment",
				TargetPercentage:  99.95,
				ActualPercentage:  99.96,
				ErrorBudgetTotal:  1296.0,
				ErrorBudgetUsed:   648.0,
				ErrorBudgetRemain: tt.remain,
				PeriodStart:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
				PeriodEnd:         time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
			}

			evt := CheckBudgetAlert(result)
			if evt != nil {
				t.Errorf("expected nil alert for remain=%.2f%%, got severity=%s", tt.remain, evt.Severity)
			}
		})
	}
}

// TestCheckBudgetAlert_WarningBand verifies warning fires when 5% <= budget < 20%.
func TestCheckBudgetAlert_WarningBand(t *testing.T) {
	tests := []struct {
		name   string
		remain float64
	}{
		{"10% remaining", 10.0},
		{"19.99% remaining", 19.99},
		{"exactly 5% remaining", 5.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &SLAResult{
				ConfigID:          "cfg-warn",
				Dimension:         SLALevelInfraLayer,
				DimensionValue:    "network",
				TargetPercentage:  99.9,
				ActualPercentage:  99.85,
				ErrorBudgetTotal:  2592.0,
				ErrorBudgetUsed:   2332.8,
				ErrorBudgetRemain: tt.remain,
				PeriodStart:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
				PeriodEnd:         time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
			}

			evt := CheckBudgetAlert(result)
			if evt == nil {
				t.Fatalf("expected warning alert for remain=%.2f%%, got nil", tt.remain)
			}
			if evt.Severity != "warning" {
				t.Errorf("expected severity=warning, got %s", evt.Severity)
			}
			if evt.ConfigID != result.ConfigID {
				t.Errorf("expected config_id=%s, got %s", result.ConfigID, evt.ConfigID)
			}
			if evt.Dimension != result.Dimension {
				t.Errorf("expected dimension=%s, got %s", result.Dimension, evt.Dimension)
			}
			if evt.ErrorBudgetRemain != tt.remain {
				t.Errorf("expected remain=%.2f, got %.2f", tt.remain, evt.ErrorBudgetRemain)
			}
		})
	}
}

// TestCheckBudgetAlert_CriticalBand verifies critical fires when budget < 5%.
func TestCheckBudgetAlert_CriticalBand(t *testing.T) {
	tests := []struct {
		name   string
		remain float64
	}{
		{"3% remaining", 3.0},
		{"0% remaining (exhausted)", 0.0},
		{"4.99% remaining", 4.99},
		{"1% remaining", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &SLAResult{
				ConfigID:          "cfg-crit",
				Dimension:         SLALevelAssetGrade,
				DimensionValue:    "S",
				TargetPercentage:  99.99,
				ActualPercentage:  99.80,
				ErrorBudgetTotal:  259.2,
				ErrorBudgetUsed:   246.24,
				ErrorBudgetRemain: tt.remain,
				PeriodStart:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
				PeriodEnd:         time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
			}

			evt := CheckBudgetAlert(result)
			if evt == nil {
				t.Fatalf("expected critical alert for remain=%.2f%%, got nil", tt.remain)
			}
			if evt.Severity != "critical" {
				t.Errorf("expected severity=critical, got %s", evt.Severity)
			}
			if evt.ConfigID != result.ConfigID {
				t.Errorf("expected config_id=%s, got %s", result.ConfigID, evt.ConfigID)
			}
			if evt.TargetPercentage != result.TargetPercentage {
				t.Errorf("expected target_pct=%.2f, got %.2f", result.TargetPercentage, evt.TargetPercentage)
			}
			if evt.FiredAt.IsZero() {
				t.Error("expected non-zero fired_at")
			}
		})
	}
}

// TestCheckBudgetAlert_EventFieldsPopulated verifies all event fields are correctly populated.
func TestCheckBudgetAlert_EventFieldsPopulated(t *testing.T) {
	result := &SLAResult{
		ConfigID:          "cfg-full",
		Dimension:         SLALevelRegion,
		DimensionValue:    "cn-east",
		TargetPercentage:  99.95,
		ActualPercentage:  99.80,
		ErrorBudgetTotal:  1296.0,
		ErrorBudgetUsed:   1230.0,
		ErrorBudgetRemain: 5.09, // warning band
		PeriodStart:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:         time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
	}

	evt := CheckBudgetAlert(result)
	if evt == nil {
		t.Fatal("expected alert event, got nil")
	}

	if evt.ConfigID != "cfg-full" {
		t.Errorf("config_id: want cfg-full, got %s", evt.ConfigID)
	}
	if evt.Dimension != SLALevelRegion {
		t.Errorf("dimension: want %s, got %s", SLALevelRegion, evt.Dimension)
	}
	if evt.DimensionValue != "cn-east" {
		t.Errorf("dimension_value: want cn-east, got %s", evt.DimensionValue)
	}
	if evt.TargetPercentage != 99.95 {
		t.Errorf("target_pct: want 99.95, got %f", evt.TargetPercentage)
	}
	if evt.ActualPercentage != 99.80 {
		t.Errorf("actual_pct: want 99.80, got %f", evt.ActualPercentage)
	}
	if evt.ErrorBudgetTotal != 1296.0 {
		t.Errorf("error_budget_total: want 1296.0, got %f", evt.ErrorBudgetTotal)
	}
	if evt.ErrorBudgetUsed != 1230.0 {
		t.Errorf("error_budget_used: want 1230.0, got %f", evt.ErrorBudgetUsed)
	}
	if evt.ErrorBudgetRemain != 5.09 {
		t.Errorf("error_budget_remain: want 5.09, got %f", evt.ErrorBudgetRemain)
	}
	if evt.Severity != "warning" {
		t.Errorf("severity: want warning, got %s", evt.Severity)
	}
}
