// Package biz 包含变更管理领域的核心业务逻辑。
package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// NoopAIRiskAssessor 空操作 AI 风险评估器，用于测试和未配置 AI 服务的场景。
type NoopAIRiskAssessor struct{}

// AssessRisk 空操作实现，返回 nil 评估结果。
func (n *NoopAIRiskAssessor) AssessRisk(_ context.Context, _ *ChangeTicket) (*RiskAssessment, error) {
	return nil, nil
}

// HTTPAIRiskAssessor 通过 HTTP 调用 svc-ai 的分析接口进行风险评估。
// 超时 5 秒，失败时返回低风险默认值，不影响主流程。
type HTTPAIRiskAssessor struct {
	baseURL    string       // svc-ai 服务基地址，如 http://svc-ai:8080
	httpClient *http.Client // HTTP 客户端
	logger     *zap.Logger  // 内部日志记录器
}

// NewHTTPAIRiskAssessor 创建一个新的 HTTP AI 风险评估器。
func NewHTTPAIRiskAssessor(baseURL string, logger *zap.Logger) *HTTPAIRiskAssessor {
	return &HTTPAIRiskAssessor{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// aiRiskRequest AI 风险评估请求体。
type aiRiskRequest struct {
	ChangeID       string   `json:"change_id"`       // 变更单 ID
	Title          string   `json:"title"`           // 变更标题
	Type           string   `json:"type"`            // 变更类型
	RiskLevel      string   `json:"risk_level"`      // 风险级别
	AffectedAssets []string `json:"affected_assets"` // 影响资产
	Description    string   `json:"description"`     // 变更描述
	RollbackPlan   string   `json:"rollback_plan"`   // 回滚方案
}

// AssessRisk 调用 svc-ai 的 /api/v1/analyze 接口进行风险评估。
// 超时 5 秒，失败时返回低风险默认值。
func (h *HTTPAIRiskAssessor) AssessRisk(ctx context.Context, ticket *ChangeTicket) (*RiskAssessment, error) {
	reqBody := &aiRiskRequest{
		ChangeID:       ticket.ID,
		Title:          ticket.Title,
		Type:           string(ticket.Type),
		RiskLevel:      string(ticket.RiskLevel),
		AffectedAssets: ticket.AffectedAssets,
		Description:    ticket.Description,
		RollbackPlan:   ticket.RollbackPlan,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		h.logger.Warn("序列化 AI 风险评估请求失败", zap.Error(err))
		return defaultLowRiskAssessment(), nil
	}

	url := fmt.Sprintf("%s/api/v1/analyze", h.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		h.logger.Warn("创建 AI 风险评估 HTTP 请求失败", zap.Error(err))
		return defaultLowRiskAssessment(), nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		h.logger.Warn("AI 风险评估请求失败",
			zap.String("change_id", ticket.ID),
			zap.Error(err),
		)
		return defaultLowRiskAssessment(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		h.logger.Warn("AI 风险评估接口返回错误",
			zap.String("change_id", ticket.ID),
			zap.Int("status_code", resp.StatusCode),
		)
		return defaultLowRiskAssessment(), nil
	}

	var assessment RiskAssessment
	if err := json.NewDecoder(resp.Body).Decode(&assessment); err != nil {
		h.logger.Warn("解析 AI 风险评估响应失败",
			zap.String("change_id", ticket.ID),
			zap.Error(err),
		)
		return defaultLowRiskAssessment(), nil
	}

	return &assessment, nil
}

// defaultLowRiskAssessment 返回低风险默认评估结果，用于 AI 服务不可用时的兜底。
func defaultLowRiskAssessment() *RiskAssessment {
	return &RiskAssessment{
		RiskScore:       10.0,
		RiskLevel:       "low",
		ImpactSummary:   "AI 风险评估服务不可用，使用默认低风险评估",
		HistoricalFails: 0,
		Suggestions:     []string{"建议人工复核风险评估结果"},
	}
}
