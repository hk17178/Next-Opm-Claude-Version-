package biz

import (
	"testing"
	"time"
)

// ===========================================
// Layer 1: 频率规则评估测试
// 验证频率类型规则在滑动窗口内事件计数达到阈值时正确触发告警。
// ===========================================

// TestLayer1_FrequencyTriggers 验证 Layer 1 频率规则：
// 当同一事件在窗口期内累计达到阈值（3次）时触发告警，未达阈值时不触发。
func TestLayer1_FrequencyTriggers(t *testing.T) {
	// 构造一条频率规则：login_failure 事件在 5 分钟内出现 3 次触发高级告警
	rules := []*AlertRule{
		{
			RuleID: "rule-freq-1", Name: "Login Burst", Layer: 1,
			RuleType: RuleTypeFrequency, Severity: SeverityHigh,
			Enabled: true,
			Condition: mustJSON(FrequencyCondition{
				Event:         "login_failure",
				Count:         3,
				WindowMinutes: 5,
			}),
		},
	}

	engine, alertRepo := newTestEngine(rules)

	// 构造指标样本，模拟登录失败事件
	sample := MetricSample{
		MetricName: "login_failure", Value: 1, Timestamp: time.Now(),
		Labels: map[string]string{"host": "auth-1"},
	}

	// 第 1 次事件：未达阈值，不应触发
	r1, _ := engine.EvaluateMetric(sample)
	if r1.Fired {
		t.Error("第 1 次事件后不应触发告警")
	}

	// 第 2 次事件：仍未达阈值，不应触发
	r2, _ := engine.EvaluateMetric(sample)
	if r2.Fired {
		t.Error("第 2 次事件后不应触发告警")
	}

	// 第 3 次事件：达到频率阈值，应触发告警
	r3, _ := engine.EvaluateMetric(sample)
	if !r3.Fired {
		t.Error("第 3 次事件后应触发告警（频率阈值已达到）")
	}

	// 验证告警仓储中记录了 1 条告警
	if len(alertRepo.alerts) != 1 {
		t.Errorf("期望 1 条告警，实际 %d 条", len(alertRepo.alerts))
	}
}

// TestLayer1_FrequencyLogEvent 验证频率规则对日志事件的评估：
// 通过 EvaluateLog 接口发送日志事件，达到频率阈值时触发告警。
func TestLayer1_FrequencyLogEvent(t *testing.T) {
	// 构造一条频率规则：error_burst 事件在 10 分钟内出现 2 次触发中级告警
	rules := []*AlertRule{
		{
			RuleID: "rule-freq-log", Name: "Error Burst", Layer: 1,
			RuleType: RuleTypeFrequency, Severity: SeverityMedium,
			Enabled: true,
			Condition: mustJSON(FrequencyCondition{
				Event:         "error_burst",
				Count:         2,
				WindowMinutes: 10,
			}),
		},
	}

	engine, _ := newTestEngine(rules)

	// 构造日志事件
	event := LogEvent{
		Source:    "app",
		Level:     "error",
		Message:   "connection timeout",
		Labels:    map[string]string{"host": "web-1"},
		Timestamp: time.Now(),
	}

	// 第 1 次日志事件：未达阈值，不应触发
	r1, _ := engine.EvaluateLog(event)
	if r1.Fired {
		t.Error("第 1 次日志事件后不应触发告警")
	}

	// 第 2 次日志事件：达到阈值，应触发告警
	r2, _ := engine.EvaluateLog(event)
	if !r2.Fired {
		t.Error("第 2 次日志事件后应触发告警（频率阈值已达到）")
	}
}

// ===========================================
// Layer 3: 趋势检测测试
// 验证趋势类型规则通过对比当前均值与历史窗口均值的变化幅度来判断是否触发告警。
// ===========================================

// TestLayer3_TrendUpward 验证上升趋势检测：
// 当前均值（80）相比历史均值（50）上升 60%，超过 20% 阈值，应触发告警。
func TestLayer3_TrendUpward(t *testing.T) {
	// 构造趋势规则：cpu_trend 指标与 1 周前相比上升超过 20% 触发中级告警
	rules := []*AlertRule{
		{
			RuleID: "rule-trend-1", Name: "CPU Trend Up", Layer: 3,
			RuleType: RuleTypeTrend, Severity: SeverityMedium,
			Enabled: true,
			Condition: mustJSON(TrendCondition{
				MetricName:      "cpu_trend",
				CompareWindow:   "1w",
				ChangeThreshold: 20.0,
				Direction:       "up",
			}),
		},
	}

	engine, _ := newTestEngine(rules)

	// 注入历史窗口数据：均值为 50
	for i := 0; i < 10; i++ {
		engine.baseline.RecordWindow("cpu_trend", "1w", 50.0)
	}

	// 注入当前基线数据：均值为 80
	for i := 0; i < 10; i++ {
		engine.baseline.Record("cpu_trend", 80.0)
	}

	// 评估指标：当前均值 80 vs 历史均值 50 = 60% 上升 > 20% 阈值，应触发
	r, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "cpu_trend", Value: 80.0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "app-1"},
	})

	if !r.Fired {
		t.Error("期望触发趋势告警：60% 上升超过 20% 阈值")
	}
}

// TestLayer3_TrendDownward 验证下降趋势检测：
// 当前均值（60）相比历史均值（100）下降 40%，超过 15% 阈值，应触发告警。
func TestLayer3_TrendDownward(t *testing.T) {
	// 构造趋势规则：revenue 指标与 1 月前相比下降超过 15% 触发高级告警
	rules := []*AlertRule{
		{
			RuleID: "rule-trend-down", Name: "Revenue Drop", Layer: 3,
			RuleType: RuleTypeTrend, Severity: SeverityHigh,
			Enabled: true,
			Condition: mustJSON(TrendCondition{
				MetricName:      "revenue",
				CompareWindow:   "1m",
				ChangeThreshold: 15.0,
				Direction:       "down",
			}),
		},
	}

	engine, _ := newTestEngine(rules)

	// 注入历史窗口数据：均值为 100
	for i := 0; i < 10; i++ {
		engine.baseline.RecordWindow("revenue", "1m", 100.0)
	}

	// 注入当前基线数据：均值为 60（下降 40%）
	for i := 0; i < 10; i++ {
		engine.baseline.Record("revenue", 60.0)
	}

	// 评估指标：40% 下降 > 15% 阈值，应触发
	r, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "revenue", Value: 60.0, Timestamp: time.Now(),
		Labels: map[string]string{"unit": "payment"},
	})

	if !r.Fired {
		t.Error("期望触发下降趋势告警：40% 下降超过 15% 阈值")
	}
}

// TestLayer3_TrendEitherDirection 验证双向趋势检测（direction="either"）：
// 无论上升还是下降，只要变化幅度超过阈值即触发。
// 本例：当前均值（70）相比历史均值（100）下降 30%，超过 25% 阈值。
func TestLayer3_TrendEitherDirection(t *testing.T) {
	// 构造趋势规则：traffic 指标与 1 周前相比任意方向变化超过 25% 触发中级告警
	rules := []*AlertRule{
		{
			RuleID: "rule-trend-either", Name: "Traffic Anomaly", Layer: 3,
			RuleType: RuleTypeTrend, Severity: SeverityMedium,
			Enabled: true,
			Condition: mustJSON(TrendCondition{
				MetricName:      "traffic",
				CompareWindow:   "1w",
				ChangeThreshold: 25.0,
				Direction:       "either",
			}),
		},
	}

	engine, _ := newTestEngine(rules)

	// 注入历史窗口数据：均值为 100
	for i := 0; i < 10; i++ {
		engine.baseline.RecordWindow("traffic", "1w", 100.0)
	}
	// 注入当前基线数据：均值为 70（下降 30%）
	for i := 0; i < 10; i++ {
		engine.baseline.Record("traffic", 70.0)
	}

	// 评估指标：30% 下降 > 25% 阈值，双向模式下应触发
	r, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "traffic", Value: 70.0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "lb-1"},
	})

	if !r.Fired {
		t.Error("期望触发双向趋势告警：30% 下降超过 25% 阈值")
	}
}

// TestLayer3_TrendNoHistoryNoFire 验证无历史数据时趋势规则不触发：
// 没有历史窗口数据作为对比基准时，即使当前值很高也不应产生告警。
func TestLayer3_TrendNoHistoryNoFire(t *testing.T) {
	// 构造趋势规则：new_metric 指标与 1 周前相比上升超过 10%
	rules := []*AlertRule{
		{
			RuleID: "rule-trend-nodata", Name: "No Data", Layer: 3,
			RuleType: RuleTypeTrend, Severity: SeverityLow,
			Enabled: true,
			Condition: mustJSON(TrendCondition{
				MetricName:      "new_metric",
				CompareWindow:   "1w",
				ChangeThreshold: 10.0,
				Direction:       "up",
			}),
		},
	}

	engine, _ := newTestEngine(rules)

	// 不注入任何历史数据，直接评估
	r, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "new_metric", Value: 100.0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "test"},
	})

	if r.Fired {
		t.Error("无历史数据时不应触发趋势告警")
	}
}

// TestLayer3_TrendWithinThreshold 验证变化幅度在阈值范围内时不触发：
// 当前均值（110）相比历史均值（100）仅上升 10%，未超过 30% 阈值，不应告警。
func TestLayer3_TrendWithinThreshold(t *testing.T) {
	// 构造趋势规则：stable_metric 指标与 1 周前相比上升超过 30%
	rules := []*AlertRule{
		{
			RuleID: "rule-trend-within", Name: "Stable", Layer: 3,
			RuleType: RuleTypeTrend, Severity: SeverityInfo,
			Enabled: true,
			Condition: mustJSON(TrendCondition{
				MetricName:      "stable_metric",
				CompareWindow:   "1w",
				ChangeThreshold: 30.0,
				Direction:       "up",
			}),
		},
	}

	engine, _ := newTestEngine(rules)

	// 注入历史窗口数据：均值为 100
	for i := 0; i < 10; i++ {
		engine.baseline.RecordWindow("stable_metric", "1w", 100.0)
	}
	// 注入当前基线数据：均值为 110（仅上升 10%）
	for i := 0; i < 10; i++ {
		engine.baseline.Record("stable_metric", 110.0)
	}

	// 评估指标：10% 上升 < 30% 阈值，不应触发
	r, _ := engine.EvaluateMetric(MetricSample{
		MetricName: "stable_metric", Value: 110.0, Timestamp: time.Now(),
		Labels: map[string]string{"host": "stable"},
	})

	if r.Fired {
		t.Error("变化幅度（10%）在阈值（30%）范围内，不应触发告警")
	}
}
