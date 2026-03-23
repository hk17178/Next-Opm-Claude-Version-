package biz

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// newTestRouter 创建用于测试的 ModelRouter 实例。
func newTestRouter() *ModelRouter {
	logger, _ := zap.NewDevelopment()
	return NewModelRouter(logger)
}

// newTestModel 创建用于测试的 AIModel 实例。
func newTestModel(name, deploymentType string, enabled bool) *AIModel {
	return &AIModel{
		ID:             uuid.New(),
		Name:           name,
		DeploymentType: deploymentType,
		Enabled:        enabled,
	}
}

// TestRoutePreferLocal_LocalFirst 验证 local_first 策略下本地模型排在云端前面。
func TestRoutePreferLocal_LocalFirst(t *testing.T) {
	router := newTestRouter()
	localModel := newTestModel("llama3", "local", true)
	cloudModel := newTestModel("claude-sonnet", "cloud", true)

	result, err := router.Route(context.Background(), RoutePreferLocal, cloudModel, localModel)
	if err != nil {
		t.Fatalf("路由失败: %v", err)
	}

	if len(result.Candidates) != 2 {
		t.Fatalf("期望 2 个候选模型，实际 %d 个", len(result.Candidates))
	}

	// 本地模型应排在第一位
	if result.Candidates[0].DeploymentType != "local" {
		t.Errorf("期望第一个候选模型为本地模型，实际为 %s", result.Candidates[0].DeploymentType)
	}
	if result.Candidates[1].DeploymentType != "cloud" {
		t.Errorf("期望第二个候选模型为云端模型，实际为 %s", result.Candidates[1].DeploymentType)
	}
}

// TestRoutePreferCloud_CloudFirst 验证 cloud_first 策略下云端模型排在本地前面。
func TestRoutePreferCloud_CloudFirst(t *testing.T) {
	router := newTestRouter()
	localModel := newTestModel("llama3", "local", true)
	cloudModel := newTestModel("claude-sonnet", "cloud", true)

	result, err := router.Route(context.Background(), RoutePreferCloud, localModel, cloudModel)
	if err != nil {
		t.Fatalf("路由失败: %v", err)
	}

	if len(result.Candidates) != 2 {
		t.Fatalf("期望 2 个候选模型，实际 %d 个", len(result.Candidates))
	}

	// 云端模型应排在第一位
	if result.Candidates[0].DeploymentType != "cloud" {
		t.Errorf("期望第一个候选模型为云端模型，实际为 %s", result.Candidates[0].DeploymentType)
	}
}

// TestRouteLocalOnly_ExcludesCloud 验证 local_only 策略排除云端模型。
func TestRouteLocalOnly_ExcludesCloud(t *testing.T) {
	router := newTestRouter()
	localModel := newTestModel("llama3", "local", true)
	cloudModel := newTestModel("claude-sonnet", "cloud", true)

	result, err := router.Route(context.Background(), RouteLocalOnly, localModel, cloudModel)
	if err != nil {
		t.Fatalf("路由失败: %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("期望 1 个候选模型，实际 %d 个", len(result.Candidates))
	}

	if result.Candidates[0].DeploymentType != "local" {
		t.Errorf("local_only 策略应只包含本地模型")
	}
}

// TestRouteCloudOnly_ExcludesLocal 验证 cloud_only 策略排除本地模型。
func TestRouteCloudOnly_ExcludesLocal(t *testing.T) {
	router := newTestRouter()
	localModel := newTestModel("llama3", "local", true)
	cloudModel := newTestModel("claude-sonnet", "cloud", true)

	result, err := router.Route(context.Background(), RouteCloudOnly, localModel, cloudModel)
	if err != nil {
		t.Fatalf("路由失败: %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("期望 1 个候选模型，实际 %d 个", len(result.Candidates))
	}

	if result.Candidates[0].DeploymentType != "cloud" {
		t.Errorf("cloud_only 策略应只包含云端模型")
	}
}

// TestRouteLocalOnly_NoCandidates_ReturnsError 验证 local_only 策略下无本地模型时返回错误。
func TestRouteLocalOnly_NoCandidates_ReturnsError(t *testing.T) {
	router := newTestRouter()
	cloudModel := newTestModel("claude-sonnet", "cloud", true)

	_, err := router.Route(context.Background(), RouteLocalOnly, cloudModel, nil)
	if err == nil {
		t.Fatal("期望返回错误，因为 local_only 策略下没有本地模型")
	}
}

// TestRouteDisabledModels_Excluded 验证已禁用的模型不会出现在候选列表中。
func TestRouteDisabledModels_Excluded(t *testing.T) {
	router := newTestRouter()
	disabledModel := newTestModel("llama3", "local", false) // 已禁用
	cloudModel := newTestModel("claude-sonnet", "cloud", true)

	result, err := router.Route(context.Background(), RoutePreferLocal, disabledModel, cloudModel)
	if err != nil {
		t.Fatalf("路由失败: %v", err)
	}

	// 禁用的模型不应出现在候选列表中
	for _, c := range result.Candidates {
		if c.Name == "llama3" {
			t.Error("已禁用的模型不应出现在候选列表中")
		}
	}
}

// TestIsValidStrategy 验证策略名称有效性检查。
func TestIsValidStrategy(t *testing.T) {
	tests := []struct {
		strategy string
		valid    bool
	}{
		{"local_first", true},
		{"cloud_first", true},
		{"local_only", true},
		{"cloud_only", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := IsValidStrategy(tt.strategy); got != tt.valid {
			t.Errorf("IsValidStrategy(%q) = %v, 期望 %v", tt.strategy, got, tt.valid)
		}
	}
}
