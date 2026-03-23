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

// NoopMaintenanceClient 空操作维护模式客户端，用于测试和未配置告警服务的场景。
type NoopMaintenanceClient struct{}

// EnableMaintenance 空操作实现，直接返回 nil。
func (n *NoopMaintenanceClient) EnableMaintenance(_ context.Context, _ []string, _ time.Duration, _ string) error {
	return nil
}

// DisableMaintenance 空操作实现，直接返回 nil。
func (n *NoopMaintenanceClient) DisableMaintenance(_ context.Context, _ []string) error {
	return nil
}

// HTTPMaintenanceClient 通过 HTTP 调用 svc-alert 维护模式接口管理资产维护状态。
// 采用异步方式发送请求，失败仅记录日志不影响主流程。
type HTTPMaintenanceClient struct {
	baseURL    string       // svc-alert 服务基地址，如 http://svc-alert:8080
	httpClient *http.Client // HTTP 客户端
	logger     *zap.Logger  // 内部日志记录器
}

// NewHTTPMaintenanceClient 创建一个新的 HTTP 维护模式客户端。
func NewHTTPMaintenanceClient(baseURL string, logger *zap.Logger) *HTTPMaintenanceClient {
	return &HTTPMaintenanceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// enableMaintenanceRequest 启用维护模式请求体。
type enableMaintenanceRequest struct {
	ResourceIDs []string `json:"resource_ids"` // 资产 ID 列表
	Duration    string   `json:"duration"`     // 维护时长（ISO 8601 duration 格式）
	Reason      string   `json:"reason"`       // 维护原因
}

// disableMaintenanceRequest 解除维护模式请求体。
type disableMaintenanceRequest struct {
	ResourceIDs []string `json:"resource_ids"` // 资产 ID 列表
}

// EnableMaintenance 异步向 svc-alert 发送启用维护模式请求。
// 失败仅记录日志，不阻塞主流程。
func (h *HTTPMaintenanceClient) EnableMaintenance(ctx context.Context, resourceIDs []string, duration time.Duration, reason string) error {
	req := &enableMaintenanceRequest{
		ResourceIDs: resourceIDs,
		Duration:    duration.String(),
		Reason:      reason,
	}

	// 异步发送，不阻塞主流程
	go func() {
		body, err := json.Marshal(req)
		if err != nil {
			h.logger.Error("序列化维护模式请求失败", zap.Error(err))
			return
		}

		url := fmt.Sprintf("%s/api/v1/maintenance/enable", h.baseURL)
		httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			h.logger.Error("创建维护模式 HTTP 请求失败", zap.Error(err))
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := h.httpClient.Do(httpReq)
		if err != nil {
			h.logger.Warn("启用维护模式请求失败",
				zap.Strings("resource_ids", resourceIDs),
				zap.Error(err),
			)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			h.logger.Warn("维护模式接口返回错误",
				zap.Int("status_code", resp.StatusCode),
			)
		}
	}()

	return nil
}

// DisableMaintenance 异步向 svc-alert 发送解除维护模式请求。
// 失败仅记录日志，不阻塞主流程。
func (h *HTTPMaintenanceClient) DisableMaintenance(ctx context.Context, resourceIDs []string) error {
	req := &disableMaintenanceRequest{
		ResourceIDs: resourceIDs,
	}

	// 异步发送，不阻塞主流程
	go func() {
		body, err := json.Marshal(req)
		if err != nil {
			h.logger.Error("序列化解除维护模式请求失败", zap.Error(err))
			return
		}

		url := fmt.Sprintf("%s/api/v1/maintenance/disable", h.baseURL)
		httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			h.logger.Error("创建解除维护模式 HTTP 请求失败", zap.Error(err))
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := h.httpClient.Do(httpReq)
		if err != nil {
			h.logger.Warn("解除维护模式请求失败",
				zap.Strings("resource_ids", resourceIDs),
				zap.Error(err),
			)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			h.logger.Warn("解除维护模式接口返回错误",
				zap.Int("status_code", resp.StatusCode),
			)
		}
	}()

	return nil
}
