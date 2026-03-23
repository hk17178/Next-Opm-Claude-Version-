package biz

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/opsnexus/svc-ai/internal/config"
	"go.uber.org/zap"
)

// ContextCollector 从多个数据源收集分析上下文，执行脱敏处理，
// 并按优先级压缩以适应 token 预算（2000-5000）。
// 确保送入 AI 模型的数据既安全（无敏感信息）又精简（不超 token 限制）。
type ContextCollector struct {
	cfg          config.AIConfig
	desensitizer *Desensitizer
	logger       *zap.Logger
}

// NewContextCollector 创建上下文收集器实例。
func NewContextCollector(cfg config.AIConfig, desensitizer *Desensitizer, logger *zap.Logger) *ContextCollector {
	return &ContextCollector{
		cfg:          cfg,
		desensitizer: desensitizer,
		logger:       logger,
	}
}

// AnalysisContext 保存经过脱敏和压缩后的分析上下文，直接用于 prompt 模板渲染。
type AnalysisContext struct {
	AlertInfo      string `json:"alert_info"`
	SystemContext  string `json:"system_context"`
	HumanContext   string `json:"human_context"`
	ChangeContext  string `json:"change_context"`
	TopologyInfo   string `json:"topology_info"`
	SimilarAlerts  string `json:"similar_alerts,omitempty"`
}

// CollectAndCompress 从原始数据收集上下文，执行脱敏处理，并压缩至配置的 token 范围内。
// 返回处理后的上下文和脱敏映射表（用于结果中的反向追溯）。
func (cc *ContextCollector) CollectAndCompress(raw RawContext) (*AnalysisContext, map[string]string) {
	// Step 1: Desensitize all raw context
	alertInfo, m1 := cc.desensitizer.Sanitize(raw.AlertInfo)
	systemCtx, m2 := cc.desensitizer.Sanitize(raw.SystemLogs)
	humanCtx, m3 := cc.desensitizer.Sanitize(raw.OperationLogs)
	changeCtx, m4 := cc.desensitizer.Sanitize(raw.ChangeRecords)
	topoCtx, m5 := cc.desensitizer.Sanitize(raw.TopologyInfo)
	similarCtx, m6 := cc.desensitizer.Sanitize(raw.SimilarAlerts)

	allMappings := mergeMappings(m1, m2, m3, m4, m5, m6)

	ctx := &AnalysisContext{
		AlertInfo:     alertInfo,
		SystemContext: systemCtx,
		HumanContext:  humanCtx,
		ChangeContext: changeCtx,
		TopologyInfo:  topoCtx,
		SimilarAlerts: similarCtx,
	}

	// Step 2: Compress to fit token budget
	cc.compress(ctx)

	return ctx, allMappings
}

// RawContext 是从各数据源获取的未处理原始上下文，尚未脱敏和压缩。
type RawContext struct {
	AlertInfo    string
	SystemLogs   string
	OperationLogs string
	ChangeRecords string
	TopologyInfo  string
	SimilarAlerts string
}

// compress 将上下文压缩至 maxContextTokens 以内。
// 采用基于优先级的截断策略：告警信息(25%) > 人工上下文(20%) > 变更记录(15%) > 系统日志(20%) > 拓扑(10%) > 相似告警(10%)。
// 优先级越高的信息分配越多的 token 预算，确保关键信息不被截断。
func (cc *ContextCollector) compress(ctx *AnalysisContext) {
	maxTokens := cc.cfg.MaxContextTokens
	if maxTokens == 0 {
		maxTokens = 5000
	}

	total := cc.estimateTokens(ctx)
	if total <= maxTokens {
		return
	}

	// Priority-based budget allocation (approximate)
	sections := []struct {
		name     string
		content  *string
		priority int // lower = higher priority (preserved first)
		maxPct   float64
	}{
		{"alert_info", &ctx.AlertInfo, 1, 0.25},
		{"human_context", &ctx.HumanContext, 2, 0.20},
		{"change_context", &ctx.ChangeContext, 3, 0.15},
		{"system_context", &ctx.SystemContext, 4, 0.20},
		{"topology_info", &ctx.TopologyInfo, 5, 0.10},
		{"similar_alerts", &ctx.SimilarAlerts, 6, 0.10},
	}

	for _, sec := range sections {
		budget := int(float64(maxTokens) * sec.maxPct)
		tokens := estimateStringTokens(*sec.content)
		if tokens > budget {
			*sec.content = truncateToTokens(*sec.content, budget)
			cc.logger.Debug("context section truncated",
				zap.String("section", sec.name),
				zap.Int("original_tokens", tokens),
				zap.Int("budget", budget),
			)
		}
	}
}

// estimateTokens 估算整个分析上下文的总 token 数。
func (cc *ContextCollector) estimateTokens(ctx *AnalysisContext) int {
	total := 0
	total += estimateStringTokens(ctx.AlertInfo)
	total += estimateStringTokens(ctx.SystemContext)
	total += estimateStringTokens(ctx.HumanContext)
	total += estimateStringTokens(ctx.ChangeContext)
	total += estimateStringTokens(ctx.TopologyInfo)
	total += estimateStringTokens(ctx.SimilarAlerts)
	return total
}

// estimateStringTokens 粗略估算字符串的 token 数。
// 英文约 4 字符/token，中日韩字符约 2 字符/token。
func estimateStringTokens(s string) int {
	if s == "" {
		return 0
	}
	cjkCount := 0
	asciiCount := 0
	for _, r := range s {
		if r > 0x2E80 {
			cjkCount++
		} else {
			asciiCount++
		}
	}
	return (asciiCount / 4) + (cjkCount / 2) + 1
}

// truncateToTokens 将文本截断至约指定的 token 数，保持行完整性并添加截断标记。
func truncateToTokens(s string, maxTokens int) string {
	if estimateStringTokens(s) <= maxTokens {
		return s
	}

	// Rough char budget
	charBudget := maxTokens * 3 // conservative estimate
	if charBudget >= utf8.RuneCountInString(s) {
		return s
	}

	lines := strings.Split(s, "\n")
	var result strings.Builder
	count := 0

	for _, line := range lines {
		lineLen := utf8.RuneCountInString(line) + 1
		if count+lineLen > charBudget {
			break
		}
		result.WriteString(line)
		result.WriteString("\n")
		count += lineLen
	}

	result.WriteString(fmt.Sprintf("\n... [truncated, %d tokens exceeded budget]", estimateStringTokens(s)))
	return result.String()
}

// BuildPrompt 用分析上下文变量渲染 prompt 模板，替换 ${key} 占位符。
func (cc *ContextCollector) BuildPrompt(template string, vars map[string]string) string {
	result := template
	for k, v := range vars {
		result = strings.ReplaceAll(result, "${"+k+"}", v)
	}
	return result
}

// mergeMappings 合并多个脱敏映射表为一个。
func mergeMappings(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
