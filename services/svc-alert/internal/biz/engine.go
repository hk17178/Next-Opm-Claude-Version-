package biz

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"time"

	"go.uber.org/zap"
)

// AlertEngine 实现 6 层告警评估流水线（Layer 0-5），是整个告警系统的核心组件。
//
// 流水线层级说明：
//
//	Layer 0: Ironclad 铁律告警 — 最高优先级，bypass 所有去重和抑制机制，确保关键告警永不丢失
//	Layer 1: 静态规则 — 阈值(threshold)、关键字(keyword)、频率(frequency) 三类传统规则
//	Layer 2: 动态基线 — 基于 Welford 算法学习的指标基线，检测偏离正常范围的异常
//	Layer 3: 趋势检测 — 同比(周/月)对比，发现指标的渐进式恶化趋势
//	Layer 4: AI 异常检测 — 多维异常检测与日志模式突变（Phase 2 实现）
//	Layer 5: 业务逻辑告警 — 交易成功率、支付延迟等业务指标（Phase 2 实现）
//
// 每个传入的指标/日志事件会依次经过所有启用的规则评估，触发的告警经过去重后持久化。
type AlertEngine struct {
	ruleRepo    RuleRepository
	alertRepo   AlertRepository
	baseline    *BaselineTracker
	dedup       *Deduplicator
	freqCounter *FrequencyCounter
	log         *zap.SugaredLogger
}

// NewAlertEngine 创建告警引擎实例，注入规则仓储、告警仓储、基线跟踪器和去重器等依赖。
// 内部会自动创建 FrequencyCounter 用于 Layer 1 频率规则的事件计数。
func NewAlertEngine(
	ruleRepo RuleRepository,
	alertRepo AlertRepository,
	baseline *BaselineTracker,
	dedup *Deduplicator,
	log *zap.SugaredLogger,
) *AlertEngine {
	return &AlertEngine{
		ruleRepo:    ruleRepo,
		alertRepo:   alertRepo,
		baseline:    baseline,
		dedup:       dedup,
		freqCounter: NewFrequencyCounter(),
		log:         log,
	}
}

// EvalResult 封装单次事件评估的结果，包含是否触发告警及触发的告警列表。
type EvalResult struct {
	Fired  bool     `json:"fired"`
	Alerts []*Alert `json:"alerts,omitempty"`
}

// EvaluateMetric 对一个指标采样点运行完整的 6 层评估流水线。
// 流程：先将数据喂入基线跟踪器用于持续学习，然后遍历所有启用的规则逐一评估。
// 关键字规则(keyword)仅适用于日志，在指标评估中会被跳过。
func (e *AlertEngine) EvaluateMetric(sample MetricSample) (*EvalResult, error) {
	result := &EvalResult{}

	// 将当前值喂入基线跟踪器，持续更新统计模型（即使未触发告警也需要学习）
	e.baseline.Record(sample.MetricName, sample.Value)

	// 加载所有启用的规则
	rules, err := e.ruleRepo.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("list enabled rules: %w", err)
	}

	for _, rule := range rules {
		// 关键字规则仅用于日志匹配，指标评估时跳过
		if rule.RuleType == RuleTypeKeyword {
			continue
		}

		triggered := e.evaluateCondition(rule, &sample, nil)
		if !triggered {
			continue
		}

		alert, skip := e.processTriggered(rule, sample.Labels, sample.HostID, sample.Service, &sample.Value, nil)
		if skip {
			continue
		}

		result.Alerts = append(result.Alerts, alert)
		result.Fired = true
	}

	return result, nil
}

// EvaluateLog 对一条日志事件运行完整的 6 层评估流水线。
// 与 EvaluateMetric 类似，但不过滤规则类型——所有规则类型都可能匹配日志事件。
func (e *AlertEngine) EvaluateLog(event LogEvent) (*EvalResult, error) {
	result := &EvalResult{}

	rules, err := e.ruleRepo.ListEnabled()
	if err != nil {
		return nil, fmt.Errorf("list enabled rules: %w", err)
	}

	for _, rule := range rules {
		triggered := e.evaluateCondition(rule, nil, &event)
		if !triggered {
			continue
		}

		alert, skip := e.processTriggered(rule, event.Labels, event.HostID, event.Service, nil, &event)
		if skip {
			continue
		}

		result.Alerts = append(result.Alerts, alert)
		result.Fired = true
	}

	return result, nil
}

// processTriggered 处理已触发规则的去重、抑制和告警生成。
// 返回生成的告警和是否应跳过（因去重被抑制）。
// 核心流程：指纹计算 -> 去重检查 -> 生成告警ID -> 构建告警消息 -> 持久化 -> 记录指纹。
func (e *AlertEngine) processTriggered(
	rule *AlertRule,
	labels map[string]string,
	hostID, service string,
	metricValue *float64,
	logEvent *LogEvent,
) (*Alert, bool) {
	fp := Fingerprint(rule.RuleID, labels)

	// Ironclad（铁律）告警 bypass 去重冷却期，确保始终触发——
	// 因为铁律告警代表不可忽视的严重事件（如安全入侵、数据丢失），宁可重复也不能遗漏
	if !rule.Ironclad {
		if e.dedup.IsDuplicate(fp) {
			e.log.Debugw("alert deduplicated", "rule", rule.Name, "fingerprint", fp)
			existing, _ := e.alertRepo.GetByFingerprint(fp)
			if existing != nil {
				_ = e.alertRepo.IncrementSuppression(existing.AlertID, "convergence")
			}
			return nil, true
		}
	}

	// 从数据库序列生成告警 ID（格式：ALT-YYYYMMDD-NNN），失败时降级为基于时间戳的 ID
	alertID, err := e.alertRepo.NextAlertID()
	if err != nil {
		alertID = fmt.Sprintf("ALT-%s-%06d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000)
	}

	// 构建告警消息，根据规则类型生成不同格式的描述信息
	message := fmt.Sprintf("[%s] Rule triggered: %s", rule.Name, rule.Description)
	title := rule.Name

	var thresholdVal *float64
	if rule.RuleType == RuleTypeThreshold {
		var cond ThresholdCondition
		if err := json.Unmarshal(rule.Condition, &cond); err == nil {
			thresholdVal = &cond.Threshold
			if metricValue != nil {
				message = fmt.Sprintf("%s %s %s %.2f (current: %.2f)",
					rule.Name, cond.MetricName, cond.Operator, cond.Threshold, *metricValue)
			}
		}
	}

	if logEvent != nil && metricValue == nil {
		message = fmt.Sprintf("[%s] pattern matched: %s", rule.Name, logEvent.Message)
	}

	alert := &Alert{
		AlertID:        alertID,
		RuleID:         rule.RuleID,
		Severity:       rule.Severity,
		Status:         AlertStatusFiring,
		Title:          title,
		Description:    rule.Description,
		SourceHost:     hostID,
		SourceService:  service,
		Message:        message,
		MetricValue:    metricValue,
		ThresholdValue: thresholdVal,
		Fingerprint:    fp,
		Layer:          rule.Layer,
		Ironclad:       rule.Ironclad,
		TriggeredAt:    time.Now(),
		Tags:           labels,
	}

	if err := e.alertRepo.Create(alert); err != nil {
		e.log.Errorw("failed to persist alert", "error", err, "rule", rule.Name)
		return nil, true
	}

	e.dedup.Record(fp)

	e.log.Infow("alert fired",
		"alert_id", alertID,
		"rule", rule.Name,
		"layer", rule.Layer,
		"severity", rule.Severity,
		"ironclad", rule.Ironclad,
		"fingerprint", fp,
	)

	return alert, false
}

// evaluateCondition 根据规则类型分发到对应层级的评估函数。
// 这是流水线的核心路由逻辑，将规则类型映射到 Layer 0-5 的具体评估实现。
func (e *AlertEngine) evaluateCondition(rule *AlertRule, sample *MetricSample, logEvent *LogEvent) bool {
	switch rule.RuleType {
	case RuleTypeThreshold:
		if sample == nil {
			return false
		}
		return e.evalThreshold(rule, sample)
	case RuleTypeKeyword:
		if logEvent == nil {
			return false
		}
		return e.evalKeyword(rule, logEvent)
	case RuleTypeFrequency:
		return e.evalFrequency(rule, sample, logEvent)
	case RuleTypeBaseline:
		if sample == nil {
			return false
		}
		return e.evalBaseline(rule, sample)
	case RuleTypeTrend:
		return e.evalTrend(rule, sample)
	case RuleTypeAI:
		// Layer 4: AI 多维异常检测与日志模式突变 — Phase 2 实现
		return false
	case RuleTypeBusiness:
		// Layer 5: 业务逻辑告警（交易成功率、支付延迟等）— Phase 2 实现
		return false
	default:
		return false
	}
}

// evalThreshold 执行 Layer 1 静态阈值检查。
// 从规则条件中解析阈值和操作符，将当前指标值与阈值进行比较。
// 支持 >、>=、<、<=、=、!= 六种操作符。
func (e *AlertEngine) evalThreshold(rule *AlertRule, sample *MetricSample) bool {
	var cond ThresholdCondition
	if err := json.Unmarshal(rule.Condition, &cond); err != nil {
		e.log.Warnw("invalid threshold condition", "rule_id", rule.RuleID, "error", err)
		return false
	}

	if cond.MetricName != "" && cond.MetricName != sample.MetricName {
		return false
	}

	switch cond.Operator {
	case ">":
		return sample.Value > cond.Threshold
	case ">=":
		return sample.Value >= cond.Threshold
	case "<":
		return sample.Value < cond.Threshold
	case "<=":
		return sample.Value <= cond.Threshold
	case "=", "==":
		return sample.Value == cond.Threshold
	case "!=":
		return sample.Value != cond.Threshold
	default:
		return false
	}
}

// evalKeyword 执行 Layer 1 关键字/正则匹配检查。
// 从规则条件中解析正则模式，对日志消息进行模式匹配。
func (e *AlertEngine) evalKeyword(rule *AlertRule, event *LogEvent) bool {
	var cond KeywordCondition
	if err := json.Unmarshal(rule.Condition, &cond); err != nil {
		e.log.Warnw("invalid keyword condition", "rule_id", rule.RuleID, "error", err)
		return false
	}

	matched, err := regexp.MatchString(cond.Pattern, event.Message)
	if err != nil {
		e.log.Warnw("regex error", "rule_id", rule.RuleID, "pattern", cond.Pattern, "error", err)
		return false
	}
	return matched
}

// evalFrequency 执行 Layer 1 频率规则检查。
// 使用内存中的 FrequencyCounter 按规则 ID 统计事件次数，
// 当滑动时间窗口内的事件数量达到阈值时触发告警。
func (e *AlertEngine) evalFrequency(rule *AlertRule, sample *MetricSample, logEvent *LogEvent) bool {
	var cond FrequencyCondition
	if err := json.Unmarshal(rule.Condition, &cond); err != nil {
		e.log.Warnw("invalid frequency condition", "rule_id", rule.RuleID, "error", err)
		return false
	}

	window := time.Duration(cond.WindowMinutes) * time.Minute

	// Record this event occurrence
	e.freqCounter.Record(rule.RuleID)

	// Check if the count within the window meets the threshold
	count := e.freqCounter.Count(rule.RuleID, window)
	return count >= cond.Count
}

// evalTrend 执行 Layer 3 同比趋势检测，支持三种对比窗口：
//   - "1d"：日同比 —— 当前值 vs 24 小时前同时段均值
//   - "1w"：周同比 —— 当前值 vs 7 天前同时段均值（委托给 evalWeekOverWeek 专项实现）
//   - "1m"：月同比 —— 当前值 vs 30 天前同时段均值
//
// 偏差方向支持：
//   - "up"：仅上升超过阈值时触发（如流量激增、错误率上升）
//   - "down"：仅下降超过阈值时触发（如吞吐量下降、成功率降低）
//   - "either"：双向超阈值均触发
//
// 数据来源优先级：带时间戳的精确历史数据 > 旧式滑动窗口均值（向后兼容降级）。
func (e *AlertEngine) evalTrend(rule *AlertRule, sample *MetricSample) bool {
	if sample == nil {
		return false
	}

	var cond TrendCondition
	if err := json.Unmarshal(rule.Condition, &cond); err != nil {
		e.log.Warnw("invalid trend condition", "rule_id", rule.RuleID, "error", err)
		return false
	}

	// 若规则指定了目标指标名，则仅匹配该指标，其余指标直接跳过
	if cond.MetricName != "" && cond.MetricName != sample.MetricName {
		return false
	}

	// 周同比（"1w"）委托给专项实现，以获得更精确的"7 天前同时段"语义
	if cond.CompareWindow == "1w" {
		return e.evalWeekOverWeek(cond, sample)
	}

	// 日同比 / 月同比：通用趋势评估逻辑
	return e.evalTrendGeneric(cond, sample)
}

// evalWeekOverWeek 执行 Layer 3 周同比（Week-over-Week）告警评估。
//
// 周同比业务语义：
//   - 当前时段：最近 1 小时内的采样均值，代表"本周此刻"的指标水平。
//   - 历史同期：7 天前同一时段（±1 小时窗口）的采样均值，代表"上周此刻"的基准。
//   - 偏差公式：changePct = (currentMean - historicMean) / |historicMean| × 100%
//   - 触发条件：偏差百分比按 direction（up/down/either）与 change_threshold 比较。
//
// 数据降级策略：
//  1. 优先使用 BaselineTracker 中带时间戳的精确历史记录（RecordTimestamped 写入）。
//  2. 若精确历史数据不足（服务首次启动或窗口内无数据），降级为旧式窗口均值
//     （RecordWindow "1w" 写入），保证向后兼容。
//
// 防护机制：
//   - 历史均值为 0 时跳过（避免除零错误，也避免误报）。
//   - 当前均值获取失败时跳过（数据不足，拒绝误报）。
//
// 参数：
//   - cond：趋势规则条件（含 MetricName、ChangeThreshold、Direction）。
//   - sample：当前指标采样点（Value + Timestamp）。
//
// 返回：true 表示周同比偏差超过阈值，应触发告警。
func (e *AlertEngine) evalWeekOverWeek(cond TrendCondition, sample *MetricSample) bool {
	// 记录本次采样到带时间戳历史数据中，供后续周同比使用
	ts := sample.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	e.baseline.RecordTimestamped(sample.MetricName, sample.Value, ts)

	// 同时写入旧式窗口记录，保持向后兼容性
	e.baseline.RecordWindow(sample.MetricName, "1w", sample.Value)

	// --- 获取当前时段均值（最近 1 小时，代表"本周此刻"）---
	recentDuration := 1 * time.Hour
	currentMean, currentOK := e.baseline.GetRecentMean(sample.MetricName, recentDuration)
	if !currentOK {
		// 带时间戳数据不足，降级为全量基线均值
		mean, ok := e.baseline.GetMean(sample.MetricName)
		if !ok {
			// 完全没有历史数据，无法判断趋势，跳过本次评估
			return false
		}
		currentMean = mean
	}

	// --- 获取历史同期均值（7 天前同时段，代表"上周此刻"）---
	// 历史窗口：[now-7d-1h, now-7d)，覆盖"上周"同一小时的数据
	weekAgo := 7 * 24 * time.Hour
	now := time.Now()
	historicFrom := now.Add(-weekAgo - recentDuration) // 上周同时段开始：8天+1小时前
	historicTo := now.Add(-weekAgo)                    // 上周同时段结束：7天前
	historicMean, historicOK := e.baseline.GetHistoricMean(sample.MetricName, historicFrom, historicTo)

	if !historicOK {
		// 精确历史数据不足，降级为旧式窗口均值（"metric:1w" 复合键）
		mean, ok := e.baseline.GetWindowMean(sample.MetricName, "1w")
		if !ok {
			// 没有任何历史基准数据，无法计算同比，跳过
			return false
		}
		historicMean = mean
	}

	// 历史均值为 0 时无法计算百分比变化，跳过以避免误报
	if historicMean == 0 {
		return false
	}

	// 计算周同比变化百分比
	// 正值表示本周高于上周（上升），负值表示低于上周（下降）
	changePct := (currentMean - historicMean) / math.Abs(historicMean) * 100.0

	e.log.Debugw("week-over-week evaluation",
		"metric", sample.MetricName,
		"current_mean", currentMean,
		"historic_mean", historicMean,
		"change_pct", changePct,
		"threshold", cond.ChangeThreshold,
		"direction", cond.Direction,
	)

	// 根据配置的方向判断是否超过阈值
	switch cond.Direction {
	case "up":
		// 仅检测上升趋势：本周比上周同期高出超过阈值百分比
		return changePct >= cond.ChangeThreshold
	case "down":
		// 仅检测下降趋势：本周比上周同期低出超过阈值百分比
		return changePct <= -cond.ChangeThreshold
	case "either":
		// 双向检测：无论上升还是下降，只要幅度超过阈值即告警
		return math.Abs(changePct) >= cond.ChangeThreshold
	default:
		e.log.Warnw("unknown trend direction", "direction", cond.Direction)
		return false
	}
}

// evalTrendGeneric 执行通用的同比趋势评估，供日同比("1d")和月同比("1m")使用。
// 周同比由专项的 evalWeekOverWeek 处理，此函数不会被 "1w" 窗口调用。
//
// 评估步骤：
//  1. 将当前采样记录到带时间戳历史数据和旧式窗口数据中。
//  2. 获取当前时段均值（最近 1 小时）。
//  3. 获取历史同期均值（距今 windowDuration 前的相同时段）。
//  4. 计算变化百分比并与阈值比较。
//
// 参数：
//   - cond：趋势规则条件。
//   - sample：当前指标采样点。
//
// 返回：true 表示偏差超过阈值，应触发告警。
func (e *AlertEngine) evalTrendGeneric(cond TrendCondition, sample *MetricSample) bool {
	// 记录带时间戳的历史数据，用于精确的同期均值计算
	ts := sample.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	e.baseline.RecordTimestamped(sample.MetricName, sample.Value, ts)

	// 同时写入旧式窗口记录，保持向后兼容
	e.baseline.RecordWindow(sample.MetricName, cond.CompareWindow, sample.Value)

	// 解析对比窗口时长（"1d" = 24h，"1m" = 720h）
	windowDuration := parseWindowDuration(cond.CompareWindow)

	// 获取当前时段均值（最近 1 小时）
	recentDuration := 1 * time.Hour
	currentMean, currentOK := e.baseline.GetRecentMean(sample.MetricName, recentDuration)
	if !currentOK {
		// 降级：使用全量基线均值
		mean, ok := e.baseline.GetMean(sample.MetricName)
		if !ok {
			return false
		}
		currentMean = mean
	}

	// 获取历史同期均值（距今 windowDuration 前的同时段）
	now := time.Now()
	historicFrom := now.Add(-windowDuration - recentDuration)
	historicTo := now.Add(-windowDuration)
	historicMean, historicOK := e.baseline.GetHistoricMean(sample.MetricName, historicFrom, historicTo)
	if !historicOK {
		// 降级：使用旧式窗口均值
		mean, ok := e.baseline.GetWindowMean(sample.MetricName, cond.CompareWindow)
		if !ok {
			return false
		}
		historicMean = mean
	}

	// 避免除零错误
	if historicMean == 0 {
		return false
	}

	changePct := (currentMean - historicMean) / math.Abs(historicMean) * 100.0

	switch cond.Direction {
	case "up":
		return changePct >= cond.ChangeThreshold
	case "down":
		return changePct <= -cond.ChangeThreshold
	case "either":
		return math.Abs(changePct) >= cond.ChangeThreshold
	default:
		return false
	}
}

// parseWindowDuration 将对比窗口字符串解析为 time.Duration。
//
// 支持的窗口标识：
//   - "1d"：日同比，24 小时
//   - "1w"：周同比，7 天（168 小时）
//   - "1m"：月同比，30 天（720 小时，近似值）
//   - 其他：默认回退到周同比（7 天），并在调用方日志中体现
func parseWindowDuration(window string) time.Duration {
	switch window {
	case "1d":
		return 24 * time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	case "1m":
		return 30 * 24 * time.Hour
	default:
		return 7 * 24 * time.Hour // 未知窗口默认使用周同比
	}
}

// evalBaseline 执行 Layer 2 动态基线异常检测。
// 将当前指标值与基线跟踪器中学习到的均值进行比较，
// 偏差百分比超过阈值（默认 30%，对应 FR-03-007）时判定为异常。
func (e *AlertEngine) evalBaseline(rule *AlertRule, sample *MetricSample) bool {
	// 从规则条件解析偏差百分比，缺省使用 30%（FR-03-007 默认值）
	deviationPct := 30.0
	type baselineCond struct {
		MetricName   string  `json:"metric_name"`
		DeviationPct float64 `json:"deviation_pct"`
	}
	var cond baselineCond
	if err := json.Unmarshal(rule.Condition, &cond); err == nil {
		if cond.DeviationPct > 0 {
			deviationPct = cond.DeviationPct
		}
		if cond.MetricName != "" && cond.MetricName != sample.MetricName {
			return false
		}
	}

	return e.baseline.IsAnomaly(sample.MetricName, sample.Value, deviationPct)
}

func boolPtr(b bool) *bool { return &b }
