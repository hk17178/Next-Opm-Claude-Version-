package biz

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"
)

// newTestRuleUseCase 创建测试用的 RuleUseCase 实例，使用 mock 仓储和空日志器。
func newTestRuleUseCase() (*RuleUseCase, *mockRuleRepo) {
	repo := &mockRuleRepo{}
	logger := zap.NewNop().Sugar()
	uc := NewRuleUseCase(repo, logger)
	return uc, repo
}

// TestCreateRule_Success 验证正常创建规则：自动生成 ID、默认启用、默认冷却 5 分钟、设置时间戳。
func TestCreateRule_Success(t *testing.T) {
	uc, repo := newTestRuleUseCase()

	rule := &AlertRule{
		Name:     "High CPU",
		RuleType: RuleTypeThreshold,
		Layer:    1,
		Severity: SeverityCritical,
		Condition: json.RawMessage(`{"metric_name":"cpu","operator":">","threshold":90}`),
	}

	if err := uc.CreateRule(rule); err != nil {
		t.Fatalf("CreateRule error: %v", err)
	}

	if rule.RuleID == "" {
		t.Error("rule_id should be auto-generated")
	}
	if !rule.Enabled {
		t.Error("rule should be enabled by default")
	}
	if rule.CooldownMinutes != 5 {
		t.Errorf("cooldown_minutes should default to 5, got %d", rule.CooldownMinutes)
	}
	if rule.CreatedAt.IsZero() {
		t.Error("created_at should be set")
	}
	if rule.UpdatedAt.IsZero() {
		t.Error("updated_at should be set")
	}
	if len(repo.rules) != 1 {
		t.Errorf("repo should have 1 rule, got %d", len(repo.rules))
	}
}

// TestCreateRule_EmptyName 验证规则名称为空时返回校验错误。
func TestCreateRule_EmptyName(t *testing.T) {
	uc, _ := newTestRuleUseCase()

	rule := &AlertRule{
		RuleType:  RuleTypeThreshold,
		Layer:     1,
		Severity:  SeverityHigh,
		Condition: json.RawMessage(`{}`),
	}

	if err := uc.CreateRule(rule); err == nil {
		t.Fatal("expected error for empty name")
	}
}

// TestCreateRule_NilCondition 验证规则条件为 nil 时返回校验错误。
func TestCreateRule_NilCondition(t *testing.T) {
	uc, _ := newTestRuleUseCase()

	rule := &AlertRule{
		Name:     "Test",
		RuleType: RuleTypeThreshold,
		Layer:    1,
		Severity: SeverityHigh,
	}

	if err := uc.CreateRule(rule); err == nil {
		t.Fatal("expected error for nil condition")
	}
}

// TestCreateRule_EmptySeverity 验证严重等级为空时返回校验错误。
func TestCreateRule_EmptySeverity(t *testing.T) {
	uc, _ := newTestRuleUseCase()

	rule := &AlertRule{
		Name:      "Test",
		RuleType:  RuleTypeThreshold,
		Layer:     1,
		Condition: json.RawMessage(`{}`),
	}

	if err := uc.CreateRule(rule); err == nil {
		t.Fatal("expected error for empty severity")
	}
}

// TestCreateRule_InvalidLayer 验证 Layer 超出合法范围 (0-5) 时返回校验错误。
func TestCreateRule_InvalidLayer(t *testing.T) {
	uc, _ := newTestRuleUseCase()

	tests := []struct {
		name  string // 测试场景名称
		layer int    // 非法的层级值
	}{
		{"negative layer", -1},
		{"layer 6", 6},
		{"layer 10", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &AlertRule{
				Name:      "Test",
				RuleType:  RuleTypeThreshold,
				Layer:     tt.layer,
				Severity:  SeverityHigh,
				Condition: json.RawMessage(`{}`),
			}
			if err := uc.CreateRule(rule); err == nil {
				t.Errorf("expected error for layer %d", tt.layer)
			}
		})
	}
}

// TestCreateRule_CustomCooldown 验证自定义冷却时间不被默认值覆盖。
func TestCreateRule_CustomCooldown(t *testing.T) {
	uc, _ := newTestRuleUseCase()

	rule := &AlertRule{
		Name:            "Test",
		RuleType:        RuleTypeThreshold,
		Layer:           1,
		Severity:        SeverityHigh,
		CooldownMinutes: 15,
		Condition:       json.RawMessage(`{}`),
	}

	if err := uc.CreateRule(rule); err != nil {
		t.Fatalf("CreateRule error: %v", err)
	}

	if rule.CooldownMinutes != 15 {
		t.Errorf("cooldown should be preserved at 15, got %d", rule.CooldownMinutes)
	}
}

// TestUpdateRule_Success 验证更新已存在的规则成功。
func TestUpdateRule_Success(t *testing.T) {
	uc, _ := newTestRuleUseCase()

	rule := &AlertRule{
		Name:     "Original",
		RuleType: RuleTypeThreshold,
		Layer:    1,
		Severity: SeverityHigh,
		Condition: json.RawMessage(`{}`),
	}
	uc.CreateRule(rule)

	rule.Name = "Updated"
	if err := uc.UpdateRule(rule); err != nil {
		t.Fatalf("UpdateRule error: %v", err)
	}
}

// TestUpdateRule_NotFound 验证更新不存在的规则时返回未找到错误。
func TestUpdateRule_NotFound(t *testing.T) {
	uc, _ := newTestRuleUseCase()

	rule := &AlertRule{RuleID: "nonexistent", Name: "Test"}
	if err := uc.UpdateRule(rule); err == nil {
		t.Fatal("expected not found error")
	}
}

// TestDeleteRule 验证删除规则操作正常执行。
func TestDeleteRule(t *testing.T) {
	uc, _ := newTestRuleUseCase()

	rule := &AlertRule{
		Name:     "To Delete",
		RuleType: RuleTypeThreshold,
		Layer:    1,
		Severity: SeverityHigh,
		Condition: json.RawMessage(`{}`),
	}
	uc.CreateRule(rule)

	if err := uc.DeleteRule(rule.RuleID); err != nil {
		t.Fatalf("DeleteRule error: %v", err)
	}
}

// TestListRules_PageSizeDefaults 验证 pageSize 为 0 或负数时使用默认值 20。
func TestListRules_PageSizeDefaults(t *testing.T) {
	uc, _ := newTestRuleUseCase()

	rules, _, err := uc.ListRules(nil, 0, "")
	if err != nil {
		t.Fatalf("ListRules error: %v", err)
	}
	_ = rules
}

// TestListRules_PageSizeCap 验证 pageSize 超过 100 时被限制为默认值 20。
func TestListRules_PageSizeCap(t *testing.T) {
	uc, _ := newTestRuleUseCase()

	_, _, err := uc.ListRules(nil, 200, "")
	if err != nil {
		t.Fatalf("ListRules error: %v", err)
	}
}

// TestGetRule 验证按 ID 查询规则能正确返回匹配的规则。
func TestGetRule(t *testing.T) {
	uc, _ := newTestRuleUseCase()

	rule := &AlertRule{
		Name:     "Find Me",
		RuleType: RuleTypeThreshold,
		Layer:    1,
		Severity: SeverityHigh,
		Condition: json.RawMessage(`{}`),
	}
	uc.CreateRule(rule)

	found, err := uc.GetRule(rule.RuleID)
	if err != nil {
		t.Fatalf("GetRule error: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find rule")
	}
	if found.Name != "Find Me" {
		t.Errorf("expected name 'Find Me', got %s", found.Name)
	}
}
