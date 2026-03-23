package biz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWelfordUpdate(t *testing.T) {
	t.Run("single value mean equals value", func(t *testing.T) {
		bt := NewBaselineTracker(1)
		bt.Record("cpu", 50.0)
		mean, ok := bt.GetMean("cpu")
		require.True(t, ok)
		assert.Equal(t, 50.0, mean)
	})

	t.Run("two values mean is average", func(t *testing.T) {
		bt := NewBaselineTracker(1)
		bt.Record("cpu", 40.0)
		bt.Record("cpu", 60.0)
		mean, ok := bt.GetMean("cpu")
		require.True(t, ok)
		assert.InDelta(t, 50.0, mean, 0.001)
	})

	t.Run("multiple values mean converges", func(t *testing.T) {
		bt := NewBaselineTracker(1)
		values := []float64{10, 20, 30, 40, 50}
		for _, v := range values {
			bt.Record("mem", v)
		}
		mean, ok := bt.GetMean("mem")
		require.True(t, ok)
		assert.InDelta(t, 30.0, mean, 0.001)
	})

	t.Run("count tracks number of samples", func(t *testing.T) {
		bt := NewBaselineTracker(1)
		bt.Record("disk", 10.0)
		bt.Record("disk", 20.0)
		bt.Record("disk", 30.0)
		assert.Equal(t, int64(3), bt.GetCount("disk"))
	})

	t.Run("separate metrics are independent", func(t *testing.T) {
		bt := NewBaselineTracker(1)
		bt.Record("cpu", 80.0)
		bt.Record("mem", 40.0)

		cpuMean, _ := bt.GetMean("cpu")
		memMean, _ := bt.GetMean("mem")
		assert.InDelta(t, 80.0, cpuMean, 0.001)
		assert.InDelta(t, 40.0, memMean, 0.001)
	})

	t.Run("unknown metric returns false", func(t *testing.T) {
		bt := NewBaselineTracker(1)
		_, ok := bt.GetMean("nonexistent")
		assert.False(t, ok)
		assert.Equal(t, int64(0), bt.GetCount("nonexistent"))
	})
}

func TestBaselineThreshold(t *testing.T) {
	t.Run("not enough samples returns false", func(t *testing.T) {
		bt := NewBaselineTracker(10) // require 10 samples minimum
		bt.Record("cpu", 50.0)
		bt.Record("cpu", 50.0)
		// Only 2 samples, need 10
		assert.False(t, bt.IsAnomaly("cpu", 200.0, 30.0))
	})

	t.Run("value within threshold is not anomaly", func(t *testing.T) {
		bt := NewBaselineTracker(3)
		for i := 0; i < 5; i++ {
			bt.Record("cpu", 100.0)
		}
		// 10% deviation from mean=100 -> value=110 is within 30% threshold
		assert.False(t, bt.IsAnomaly("cpu", 110.0, 30.0))
	})

	t.Run("value exceeding threshold is anomaly", func(t *testing.T) {
		bt := NewBaselineTracker(3)
		for i := 0; i < 5; i++ {
			bt.Record("cpu", 100.0)
		}
		// 50% deviation from mean=100 -> value=150 exceeds 30% threshold
		assert.True(t, bt.IsAnomaly("cpu", 150.0, 30.0))
	})

	t.Run("negative deviation detected", func(t *testing.T) {
		bt := NewBaselineTracker(3)
		for i := 0; i < 5; i++ {
			bt.Record("cpu", 100.0)
		}
		// 60% below mean -> anomaly
		assert.True(t, bt.IsAnomaly("cpu", 40.0, 30.0))
	})

	t.Run("zero mean with nonzero value is anomaly", func(t *testing.T) {
		bt := NewBaselineTracker(2)
		bt.Record("zero_metric", 0.0)
		bt.Record("zero_metric", 0.0)
		bt.Record("zero_metric", 0.0)
		assert.True(t, bt.IsAnomaly("zero_metric", 5.0, 30.0))
	})

	t.Run("zero mean with zero value is not anomaly", func(t *testing.T) {
		bt := NewBaselineTracker(2)
		bt.Record("zero_metric", 0.0)
		bt.Record("zero_metric", 0.0)
		bt.Record("zero_metric", 0.0)
		assert.False(t, bt.IsAnomaly("zero_metric", 0.0, 30.0))
	})

	t.Run("unknown metric is not anomaly", func(t *testing.T) {
		bt := NewBaselineTracker(1)
		assert.False(t, bt.IsAnomaly("unknown", 100.0, 30.0))
	})
}

func TestPeakExemption(t *testing.T) {
	// Peak exemption is handled by the BaselineTracker's minSamples threshold
	// and the alert engine's condition evaluation. Here we test the boundary
	// behavior that supports peak exemption logic.

	t.Run("high deviation threshold allows peaks", func(t *testing.T) {
		bt := NewBaselineTracker(3)
		// Normal baseline around 100
		for i := 0; i < 10; i++ {
			bt.Record("traffic", 100.0)
		}
		// During peak, traffic doubles. With 100% deviation threshold, this is allowed.
		assert.False(t, bt.IsAnomaly("traffic", 200.0, 100.0))
	})

	t.Run("strict threshold catches peaks", func(t *testing.T) {
		bt := NewBaselineTracker(3)
		for i := 0; i < 10; i++ {
			bt.Record("traffic", 100.0)
		}
		// With strict 10% threshold, 200 is an anomaly
		assert.True(t, bt.IsAnomaly("traffic", 200.0, 10.0))
	})

	t.Run("gradual increase adjusts baseline", func(t *testing.T) {
		bt := NewBaselineTracker(3)
		// Simulate gradual increase where baseline adapts
		for i := 0; i < 20; i++ {
			bt.Record("traffic", float64(100+i*5))
		}
		// Mean should be around 147.5; a value of 195 is ~32% deviation
		mean, _ := bt.GetMean("traffic")
		assert.InDelta(t, 147.5, mean, 1.0)
		// Not anomaly with 50% threshold
		assert.False(t, bt.IsAnomaly("traffic", 195.0, 50.0))
	})
}
