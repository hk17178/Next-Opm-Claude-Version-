package biz

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// RoutingStrategy 定义模型路由策略类型，决定请求在本地模型和云端模型之间的分发方式。
type RoutingStrategy string

const (
	// RoutePreferLocal 优先使用本地模型，本地调用失败时自动回退到云端模型。
	// 适用场景：希望降低成本和延迟，同时保留云端作为兜底。
	RoutePreferLocal RoutingStrategy = "local_first"

	// RoutePreferCloud 优先使用云端模型，云端不可用时回退到本地模型。
	// 适用场景：对分析质量要求高，本地模型作为高可用备份。
	RoutePreferCloud RoutingStrategy = "cloud_first"

	// RouteLocalOnly 仅使用本地模型，数据不出企业网络边界。
	// 适用场景：涉及敏感数据或合规要求禁止数据外传的场景。
	RouteLocalOnly RoutingStrategy = "local_only"

	// RouteCloudOnly 仅使用云端模型，不考虑本地部署。
	// 适用场景：未部署本地模型或需要最强模型能力的场景。
	RouteCloudOnly RoutingStrategy = "cloud_only"
)

// ModelRouter 根据路由策略对主模型和备用模型进行排序和筛选。
// 它不直接调用模型，而是决定模型的调用顺序，由 ModelManager 负责实际调用。
type ModelRouter struct {
	logger *zap.Logger
}

// NewModelRouter 创建模型路由器实例。
func NewModelRouter(logger *zap.Logger) *ModelRouter {
	return &ModelRouter{logger: logger}
}

// RouteResult 封装路由决策的结果，包含有序的候选模型列表。
type RouteResult struct {
	Candidates []*AIModel // 按优先级排序的候选模型列表，调用时从前往后尝试
	Strategy   RoutingStrategy // 使用的路由策略
}

// Route 根据路由策略和可用模型，返回按优先级排序的候选模型列表。
// primaryModel 和 fallbackModel 可以为 nil（如未配置）。
// 返回的 RouteResult.Candidates 中，索引 0 为最优先调用的模型。
func (r *ModelRouter) Route(ctx context.Context, strategy RoutingStrategy, primaryModel, fallbackModel *AIModel) (*RouteResult, error) {
	result := &RouteResult{Strategy: strategy}

	switch strategy {
	case RoutePreferLocal:
		result.Candidates = r.preferLocal(primaryModel, fallbackModel)
	case RoutePreferCloud:
		result.Candidates = r.preferCloud(primaryModel, fallbackModel)
	case RouteLocalOnly:
		result.Candidates = r.localOnly(primaryModel, fallbackModel)
	case RouteCloudOnly:
		result.Candidates = r.cloudOnly(primaryModel, fallbackModel)
	default:
		// 默认使用云端优先策略
		r.logger.Warn("未知的路由策略，回退到 cloud_first",
			zap.String("strategy", string(strategy)),
		)
		result.Candidates = r.preferCloud(primaryModel, fallbackModel)
		result.Strategy = RoutePreferCloud
	}

	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("路由策略 %q 下没有可用的模型", strategy)
	}

	r.logger.Debug("模型路由决策完成",
		zap.String("strategy", string(result.Strategy)),
		zap.Int("candidates", len(result.Candidates)),
	)

	return result, nil
}

// preferLocal 实现本地优先策略：本地模型排在前面，云端模型作为回退。
func (r *ModelRouter) preferLocal(primary, fallback *AIModel) []*AIModel {
	var locals, clouds []*AIModel

	for _, m := range r.collectEnabled(primary, fallback) {
		if m.DeploymentType == "local" {
			locals = append(locals, m)
		} else {
			clouds = append(clouds, m)
		}
	}

	// 本地模型优先，云端模型作为回退
	return append(locals, clouds...)
}

// preferCloud 实现云端优先策略：云端模型排在前面，本地模型作为回退。
func (r *ModelRouter) preferCloud(primary, fallback *AIModel) []*AIModel {
	var locals, clouds []*AIModel

	for _, m := range r.collectEnabled(primary, fallback) {
		if m.DeploymentType == "local" {
			locals = append(locals, m)
		} else {
			clouds = append(clouds, m)
		}
	}

	// 云端模型优先，本地模型作为回退
	return append(clouds, locals...)
}

// localOnly 实现仅本地策略：只返回本地部署的模型，云端模型被排除。
func (r *ModelRouter) localOnly(primary, fallback *AIModel) []*AIModel {
	var result []*AIModel

	for _, m := range r.collectEnabled(primary, fallback) {
		if m.DeploymentType == "local" {
			result = append(result, m)
		}
	}

	return result
}

// cloudOnly 实现仅云端策略：只返回云端部署的模型，本地模型被排除。
func (r *ModelRouter) cloudOnly(primary, fallback *AIModel) []*AIModel {
	var result []*AIModel

	for _, m := range r.collectEnabled(primary, fallback) {
		if m.DeploymentType != "local" {
			result = append(result, m)
		}
	}

	return result
}

// collectEnabled 收集所有已启用的模型（去重），返回去重后的模型列表。
func (r *ModelRouter) collectEnabled(primary, fallback *AIModel) []*AIModel {
	var models []*AIModel
	seen := make(map[string]bool)

	for _, m := range []*AIModel{primary, fallback} {
		if m != nil && m.Enabled && !seen[m.ID.String()] {
			seen[m.ID.String()] = true
			models = append(models, m)
		}
	}

	return models
}

// IsValidStrategy 检查给定的路由策略字符串是否为有效的策略常量。
func IsValidStrategy(s string) bool {
	switch RoutingStrategy(s) {
	case RoutePreferLocal, RoutePreferCloud, RouteLocalOnly, RouteCloudOnly:
		return true
	default:
		return false
	}
}
