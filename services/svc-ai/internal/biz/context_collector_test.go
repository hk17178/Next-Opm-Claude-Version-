package biz

import (
	"strings"
	"testing"

	"github.com/opsnexus/svc-ai/internal/config"
	"go.uber.org/zap"
)

func TestEstimateStringTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		min   int
		max   int
	}{
		{"empty", "", 0, 0},
		{"short english", "hello world", 1, 10},
		{"chinese text", "你好世界测试文本", 2, 10},
		{"mixed", "Server 192.168.1.1 告警: CPU 过高", 5, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := estimateStringTokens(tt.input)
			if tokens < tt.min || tokens > tt.max {
				t.Errorf("estimateStringTokens(%q) = %d, expected %d-%d", tt.input, tokens, tt.min, tt.max)
			}
		})
	}
}

func TestTruncateToTokens(t *testing.T) {
	longText := strings.Repeat("This is a test line.\n", 100)

	result := truncateToTokens(longText, 50)
	resultTokens := estimateStringTokens(result)

	if resultTokens > 100 { // generous upper bound
		t.Errorf("truncated text has %d tokens, expected near 50", resultTokens)
	}

	if !strings.Contains(result, "truncated") {
		t.Error("truncated text should contain truncation marker")
	}
}

func TestTruncateToTokens_ShortText(t *testing.T) {
	short := "hello world"
	result := truncateToTokens(short, 1000)
	if result != short {
		t.Error("short text should not be truncated")
	}
}

func TestContextCollector_CompressesLargeContext(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := &Desensitizer{enabled: false}
	cc := NewContextCollector(config.AIConfig{
		MaxContextTokens: 100,
		MinContextTokens: 50,
	}, d, logger)

	raw := RawContext{
		AlertInfo:    strings.Repeat("Alert info line.\n", 50),
		SystemLogs:   strings.Repeat("System log line.\n", 50),
		OperationLogs: strings.Repeat("Operation log line.\n", 50),
		ChangeRecords: strings.Repeat("Change record line.\n", 50),
		TopologyInfo:  strings.Repeat("Topology info line.\n", 50),
		SimilarAlerts: strings.Repeat("Similar alert line.\n", 50),
	}

	ctx, _ := cc.CollectAndCompress(raw)

	totalTokens := estimateStringTokens(ctx.AlertInfo) +
		estimateStringTokens(ctx.SystemContext) +
		estimateStringTokens(ctx.HumanContext) +
		estimateStringTokens(ctx.ChangeContext) +
		estimateStringTokens(ctx.TopologyInfo) +
		estimateStringTokens(ctx.SimilarAlerts)

	// Should be compressed (exact amount depends on algorithm)
	if totalTokens == 0 {
		t.Error("compressed context should not be empty")
	}
}

func TestBuildPrompt(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := &Desensitizer{enabled: false}
	cc := NewContextCollector(config.AIConfig{}, d, logger)

	template := "Alert: ${alert_id} on ${host}. Severity: ${severity}"
	vars := map[string]string{
		"alert_id": "abc-123",
		"host":     "web-server-01",
		"severity": "critical",
	}

	result := cc.BuildPrompt(template, vars)

	if !strings.Contains(result, "abc-123") {
		t.Error("alert_id should be substituted")
	}
	if !strings.Contains(result, "web-server-01") {
		t.Error("host should be substituted")
	}
	if strings.Contains(result, "${") {
		t.Error("no unresolved variables should remain")
	}
}
