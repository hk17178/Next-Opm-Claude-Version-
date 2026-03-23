package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaselineTracker_RecordTimestamped(t *testing.T) {
	bt := NewBaselineTracker(1)
	now := time.Now()

	bt.RecordTimestamped("cpu", 50.0, now.Add(-1*time.Hour))
	bt.RecordTimestamped("cpu", 60.0, now.Add(-30*time.Minute))
	bt.RecordTimestamped("cpu", 70.0, now.Add(-10*time.Minute))

	// Get mean for last 2 hours
	mean, ok := bt.GetRecentMean("cpu", 2*time.Hour)
	require.True(t, ok)
	assert.InDelta(t, 60.0, mean, 0.001)
}

func TestBaselineTracker_GetHistoricMean(t *testing.T) {
	bt := NewBaselineTracker(1)
	now := time.Now()

	// Record data from "yesterday" (24h ago)
	bt.RecordTimestamped("cpu", 40.0, now.Add(-25*time.Hour))
	bt.RecordTimestamped("cpu", 60.0, now.Add(-24*time.Hour-30*time.Minute))

	// Record data from "today"
	bt.RecordTimestamped("cpu", 80.0, now.Add(-1*time.Hour))
	bt.RecordTimestamped("cpu", 90.0, now.Add(-30*time.Minute))

	// Historic mean from -26h to -23h
	historicMean, ok := bt.GetHistoricMean("cpu", now.Add(-26*time.Hour), now.Add(-23*time.Hour))
	require.True(t, ok)
	assert.InDelta(t, 50.0, historicMean, 0.001) // (40+60)/2

	// Recent mean
	recentMean, ok := bt.GetRecentMean("cpu", 2*time.Hour)
	require.True(t, ok)
	assert.InDelta(t, 85.0, recentMean, 0.001) // (80+90)/2
}

func TestBaselineTracker_GetHistoricMean_NoData(t *testing.T) {
	bt := NewBaselineTracker(1)
	now := time.Now()

	_, ok := bt.GetHistoricMean("cpu", now.Add(-2*time.Hour), now.Add(-1*time.Hour))
	assert.False(t, ok)
}

func TestBaselineTracker_GetRecentMean_NoData(t *testing.T) {
	bt := NewBaselineTracker(1)

	_, ok := bt.GetRecentMean("nonexistent", 1*time.Hour)
	assert.False(t, ok)
}

func TestBaselineTracker_HistoryPruning(t *testing.T) {
	bt := NewBaselineTracker(1)
	now := time.Now()

	// Record data older than 31 days - should be pruned
	bt.RecordTimestamped("cpu", 100.0, now.Add(-32*24*time.Hour))
	// Record recent data
	bt.RecordTimestamped("cpu", 50.0, now.Add(-1*time.Hour))

	// Old data should be pruned
	_, ok := bt.GetHistoricMean("cpu", now.Add(-33*24*time.Hour), now.Add(-31*24*time.Hour))
	assert.False(t, ok)

	// Recent data should still be there
	mean, ok := bt.GetRecentMean("cpu", 2*time.Hour)
	require.True(t, ok)
	assert.InDelta(t, 50.0, mean, 0.001)
}

func TestTrendEvaluation_DayOverDay(t *testing.T) {
	rules := []*AlertRule{
		{
			RuleID: "rule-trend-1d", Name: "CPU Day Trend", Layer: 3,
			RuleType: RuleTypeTrend, Severity: SeverityMedium,
			Enabled: true,
			Condition: mustJSON(TrendCondition{
				MetricName:      "cpu_trend",
				CompareWindow:   "1d",
				ChangeThreshold: 20.0,
				Direction:       "up",
			}),
		},
	}

	engine, _ := newTestEngine(rules)
	now := time.Now()

	// Feed yesterday's data directly into baseline tracker (mean ~50)
	for i := 0; i < 20; i++ {
		engine.baseline.RecordTimestamped("cpu_trend",
			50.0+float64(i%5),
			now.Add(-25*time.Hour+time.Duration(i)*time.Minute))
	}
	// Also populate the window key for fallback
	for i := 0; i < 20; i++ {
		engine.baseline.RecordWindow("cpu_trend", "1d", 50.0+float64(i%5))
	}

	// Feed today's data (mean ~80, which is ~60% increase from 50)
	for i := 0; i < 10; i++ {
		engine.baseline.RecordTimestamped("cpu_trend",
			80.0+float64(i%3),
			now.Add(-time.Duration(i)*time.Minute))
	}

	// Now evaluate with a value that continues the trend
	r, err := engine.EvaluateMetric(MetricSample{
		MetricName: "cpu_trend", Value: 82.0, Timestamp: now,
		Labels: map[string]string{"host": "web-1"},
	})
	require.NoError(t, err)
	// The trend should fire because ~60% increase exceeds 20% threshold
	assert.True(t, r.Fired, "day-over-day trend should fire for 60%% increase exceeding 20%% threshold")
}

func TestTrendEvaluation_WeekOverWeek(t *testing.T) {
	rules := []*AlertRule{
		{
			RuleID: "rule-trend-1w", Name: "Latency Week Trend", Layer: 3,
			RuleType: RuleTypeTrend, Severity: SeverityHigh,
			Enabled: true,
			Condition: mustJSON(TrendCondition{
				MetricName:      "latency_trend",
				CompareWindow:   "1w",
				ChangeThreshold: 30.0,
				Direction:       "either",
			}),
		},
	}

	engine, _ := newTestEngine(rules)

	// Feed stable data for the past week (mean ~100)
	for i := 0; i < 20; i++ {
		engine.baseline.RecordWindow("latency_trend", "1w", 100.0)
	}
	// Update the main baseline to reflect current elevated values
	for i := 0; i < 10; i++ {
		engine.baseline.Record("latency_trend", 150.0)
	}

	r, err := engine.EvaluateMetric(MetricSample{
		MetricName: "latency_trend", Value: 150.0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "api-1"},
	})
	require.NoError(t, err)
	// 50% increase from 100 to 150 exceeds 30% threshold
	assert.True(t, r.Fired, "week-over-week trend should fire for 50%% increase exceeding 30%% threshold")
}

func TestTrendEvaluation_DownDirection(t *testing.T) {
	rules := []*AlertRule{
		{
			RuleID: "rule-trend-down", Name: "Traffic Drop", Layer: 3,
			RuleType: RuleTypeTrend, Severity: SeverityLow,
			Enabled: true,
			Condition: mustJSON(TrendCondition{
				MetricName:      "traffic",
				CompareWindow:   "1w",
				ChangeThreshold: 25.0,
				Direction:       "down",
			}),
		},
	}

	engine, _ := newTestEngine(rules)

	// Historic mean ~200
	for i := 0; i < 20; i++ {
		engine.baseline.RecordWindow("traffic", "1w", 200.0)
	}
	// Current mean ~100 (50% drop)
	for i := 0; i < 10; i++ {
		engine.baseline.Record("traffic", 100.0)
	}

	r, err := engine.EvaluateMetric(MetricSample{
		MetricName: "traffic", Value: 100.0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "lb-1"},
	})
	require.NoError(t, err)
	assert.True(t, r.Fired, "downward trend should fire for 50%% decrease exceeding 25%% threshold")
}

func TestParseWindowDuration(t *testing.T) {
	assert.Equal(t, 24*time.Hour, parseWindowDuration("1d"))
	assert.Equal(t, 7*24*time.Hour, parseWindowDuration("1w"))
	assert.Equal(t, 30*24*time.Hour, parseWindowDuration("1m"))
	assert.Equal(t, 7*24*time.Hour, parseWindowDuration("unknown"))
}
