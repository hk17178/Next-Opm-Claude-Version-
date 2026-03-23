package biz

import (
	"math"
	"sync"
	"time"
)

// timedSample 带时间戳的指标采样值，用于 Layer 3 趋势对比的历史数据存储。
type timedSample struct {
	Value     float64
	Timestamp time.Time
}

// BaselineTracker 维护指标的运行时统计基线，使用 Welford 在线算法实现数值稳定的方差计算。
// 支持按 ON-002 FR-03-007 规范进行偏差百分比检测，用于 Layer 2 动态基线异常告警。
// 同时维护带时间戳的历史数据，供 Layer 3 趋势评估进行同比/日同比对比。
type BaselineTracker struct {
	mu         sync.RWMutex
	stats      map[string]*runningStat
	history    map[string][]timedSample
	minSamples int
}

// runningStat 存储单个指标的 Welford 在线统计量，包括计数、均值、方差辅助量(m2)及极值。
type runningStat struct {
	count int64
	mean  float64
	m2    float64 // Welford 算法中的差值平方和，用于计算方差
	min   float64
	max   float64
}

// NewBaselineTracker 创建基线跟踪器，minSamples 指定触发异常检测所需的最小采样数。
// 采样数不足时 IsAnomaly 始终返回 false，避免在学习阶段产生误报。
func NewBaselineTracker(minSamples int) *BaselineTracker {
	return &BaselineTracker{
		stats:      make(map[string]*runningStat),
		minSamples: minSamples,
	}
}

// Record 使用 Welford 算法为指定指标添加一个新数据点，在线更新均值和方差。
func (b *BaselineTracker) Record(metric string, value float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	s, ok := b.stats[metric]
	if !ok {
		s = &runningStat{min: value, max: value}
		b.stats[metric] = s
	}

	s.count++
	delta := value - s.mean
	s.mean += delta / float64(s.count)
	delta2 := value - s.mean
	s.m2 += delta * delta2

	if value < s.min {
		s.min = value
	}
	if value > s.max {
		s.max = value
	}
}

// IsAnomaly 判断给定值是否偏离基线均值超过 deviationPct 百分比（FR-03-007 默认 30%）。
// 采样数不足 minSamples 时返回 false；均值为零时只要值非零即视为异常。
func (b *BaselineTracker) IsAnomaly(metric string, value float64, deviationPct float64) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	s, ok := b.stats[metric]
	if !ok || s.count < int64(b.minSamples) {
		return false // not enough data to judge
	}

	if s.mean == 0 {
		return value != 0
	}

	deviation := math.Abs(value-s.mean) / math.Abs(s.mean) * 100.0
	return deviation > deviationPct
}

// GetBaseline 返回指定指标的当前基线模型，指标无记录时返回 nil。
func (b *BaselineTracker) GetBaseline(metric string) *BaselineModel {
	b.mu.RLock()
	defer b.mu.RUnlock()

	s, ok := b.stats[metric]
	if !ok {
		return nil
	}

	// mean/stddev/min/max 由 runningStat 内部维护，通过 GetMean/GetCount 等方法对外暴露
	_ = s
	return &BaselineModel{
		MetricName: metric,
		Status:     "active",
	}
}

// GetMean 返回指定指标的当前均值，第二个返回值表示该指标是否存在记录。
func (b *BaselineTracker) GetMean(metric string) (float64, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	s, ok := b.stats[metric]
	if !ok {
		return 0, false
	}
	return s.mean, true
}

// GetCount 返回指定指标的累计采样数量。
func (b *BaselineTracker) GetCount(metric string) int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	s, ok := b.stats[metric]
	if !ok {
		return 0
	}
	return s.count
}

// GetWindowMean 返回指定指标在历史窗口下的均值（组合键 "metric:windowKey"），
// 供 Layer 3 趋势评估获取同比/环比历史均值。
func (b *BaselineTracker) GetWindowMean(metric, windowKey string) (float64, bool) {
	compositeKey := metric + ":" + windowKey
	return b.GetMean(compositeKey)
}

// RecordWindow 将值记录到组合键 "metric:windowKey" 下，用于历史趋势对比，
// 与 Record 使用相同的 Welford 算法。
func (b *BaselineTracker) RecordWindow(metric, windowKey string, value float64) {
	compositeKey := metric + ":" + windowKey
	b.Record(compositeKey, value)
}

// RecordTimestamped 将带时间戳的值记录到历史窗口中，用于 Layer 3 趋势对比。
func (b *BaselineTracker) RecordTimestamped(metric string, value float64, ts time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.history == nil {
		b.history = make(map[string][]timedSample)
	}
	b.history[metric] = append(b.history[metric], timedSample{Value: value, Timestamp: ts})

	// 保留最多 31 天的数据，防止内存无限增长
	cutoff := time.Now().Add(-31 * 24 * time.Hour)
	samples := b.history[metric]
	kept := samples[:0]
	for _, s := range samples {
		if !s.Timestamp.Before(cutoff) {
			kept = append(kept, s)
		}
	}
	b.history[metric] = kept
}

// GetHistoricMean 计算指定指标在 [from, to) 时间范围内的均值。
// 用于 Layer 3 趋势评估的同比/日同比对比。
func (b *BaselineTracker) GetHistoricMean(metric string, from, to time.Time) (float64, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.history == nil {
		return 0, false
	}

	samples, ok := b.history[metric]
	if !ok {
		return 0, false
	}

	var sum float64
	var count int
	for _, s := range samples {
		if !s.Timestamp.Before(from) && s.Timestamp.Before(to) {
			sum += s.Value
			count++
		}
	}

	if count == 0 {
		return 0, false
	}
	return sum / float64(count), true
}

// GetRecentMean 计算指定指标最近 duration 时间内的均值。
func (b *BaselineTracker) GetRecentMean(metric string, duration time.Duration) (float64, bool) {
	now := time.Now()
	return b.GetHistoricMean(metric, now.Add(-duration), now)
}

// stddev 计算标准差，至少需要 2 个样本才能得到有意义的结果。
func (b *BaselineTracker) stddev(s *runningStat) float64 {
	if s.count < 2 {
		return 0
	}
	variance := s.m2 / float64(s.count-1)
	return math.Sqrt(variance)
}
