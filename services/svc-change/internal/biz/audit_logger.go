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

// NoopAuditLogger 空操作审计日志记录器，用于测试和未配置审计服务的场景。
type NoopAuditLogger struct{}

// Log 空操作实现，直接返回 nil。
func (n *NoopAuditLogger) Log(_ context.Context, _, _, _, _, _ string) error {
	return nil
}

// HTTPAuditLogger 通过 HTTP POST 调用 svc-log 审计接口记录日志。
// 采用异步方式发送请求，不阻塞主流程。
type HTTPAuditLogger struct {
	baseURL    string       // svc-log 服务基地址，如 http://svc-log:8080
	httpClient *http.Client // HTTP 客户端
	logger     *zap.Logger  // 内部日志记录器
}

// NewHTTPAuditLogger 创建一个新的 HTTP 审计日志记录器。
func NewHTTPAuditLogger(baseURL string, logger *zap.Logger) *HTTPAuditLogger {
	return &HTTPAuditLogger{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// auditLogRequest 审计日志请求体。
type auditLogRequest struct {
	Action     string `json:"action"`      // 操作动作
	Operator   string `json:"operator"`    // 操作人
	Resource   string `json:"resource"`    // 资源类型
	ResourceID string `json:"resource_id"` // 资源 ID
	Detail     string `json:"detail"`      // 操作详情
	Timestamp  string `json:"timestamp"`   // 操作时间
}

// Log 异步发送审计日志到 svc-log 服务。
// 使用 goroutine 异步执行 HTTP 请求，不阻塞调用方。
func (h *HTTPAuditLogger) Log(ctx context.Context, action, operator, resource, resourceID, detail string) error {
	req := &auditLogRequest{
		Action:     action,
		Operator:   operator,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     detail,
		Timestamp:  time.Now().Format(time.RFC3339),
	}

	// 异步发送，不阻塞主流程
	go func() {
		body, err := json.Marshal(req)
		if err != nil {
			h.logger.Error("序列化审计日志请求失败", zap.Error(err))
			return
		}

		url := fmt.Sprintf("%s/api/v1/audit-logs", h.baseURL)
		httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			h.logger.Error("创建审计日志 HTTP 请求失败", zap.Error(err))
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := h.httpClient.Do(httpReq)
		if err != nil {
			h.logger.Warn("发送审计日志失败",
				zap.String("action", action),
				zap.String("resource_id", resourceID),
				zap.Error(err),
			)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			h.logger.Warn("审计日志接口返回错误",
				zap.String("action", action),
				zap.Int("status_code", resp.StatusCode),
			)
		}
	}()

	return nil
}
