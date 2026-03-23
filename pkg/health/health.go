// Package health 提供 OpsNexus 服务的标准化健康检查 HTTP 端点。
// 每个服务必须暴露 /healthz（存活探针）和 /readyz（就绪探针）端点，
// 供 Kubernetes 进行存活和就绪检测。
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status 表示组件的健康状态。
type Status string

const (
	StatusUp   Status = "up"   // 组件正常运行
	StatusDown Status = "down" // 组件异常
)

// Check 是验证依赖组件是否健康的函数类型。
// 健康时返回 nil，异常时返回描述问题的 error。
type Check func(ctx context.Context) error

// ComponentHealth 表示单个依赖组件的健康状态，包含状态、消息和检查延迟。
type ComponentHealth struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Response 是健康检查的 HTTP 响应体，包含整体状态、服务信息和各组件详情。
type Response struct {
	Status     Status                     `json:"status"`
	Service    string                     `json:"service"`
	Version    string                     `json:"version"`
	Uptime     string                     `json:"uptime"`
	Components map[string]ComponentHealth `json:"components,omitempty"`
}

// Handler 管理服务的健康检查，分别维护存活检查和就绪检查列表。
type Handler struct {
	service    string
	version    string
	startTime  time.Time
	mu         sync.RWMutex
	liveness   map[string]Check
	readiness  map[string]Check
}

// New 为指定服务创建健康检查处理器。
func New(service, version string) *Handler {
	return &Handler{
		service:   service,
		version:   version,
		startTime: time.Now(),
		liveness:  make(map[string]Check),
		readiness: make(map[string]Check),
	}
}

// AddLivenessCheck 注册存活检查（进程是否存活？）。
func (h *Handler) AddLivenessCheck(name string, check Check) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.liveness[name] = check
}

// AddReadinessCheck 注册就绪检查（服务是否可以处理请求？）。
// 典型检查项：数据库连接、Kafka Broker 连通性、Redis 等。
func (h *Handler) AddReadinessCheck(name string, check Check) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.readiness[name] = check
}

// LivenessHandler 返回 /healthz 存活探针的 HTTP 处理函数。
// 所有检查通过返回 200，任一失败返回 503。
func (h *Handler) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleChecks(w, r, h.liveness)
	}
}

// ReadinessHandler 返回 /readyz 就绪探针的 HTTP 处理函数。
// 所有检查通过返回 200，任一失败返回 503。
func (h *Handler) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.handleChecks(w, r, h.readiness)
	}
}

// handleChecks 执行所有检查项并返回汇总结果，每个检查有 5 秒超时限制
func (h *Handler) handleChecks(w http.ResponseWriter, r *http.Request, checks map[string]Check) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	overall := StatusUp
	components := make(map[string]ComponentHealth)

	for name, check := range checks {
		start := time.Now()
		err := check(ctx)
		latency := time.Since(start)

		if err != nil {
			overall = StatusDown
			components[name] = ComponentHealth{
				Status:  StatusDown,
				Message: err.Error(),
				Latency: latency.String(),
			}
		} else {
			components[name] = ComponentHealth{
				Status:  StatusUp,
				Latency: latency.String(),
			}
		}
	}

	resp := Response{
		Status:     overall,
		Service:    h.service,
		Version:    h.version,
		Uptime:     time.Since(h.startTime).Truncate(time.Second).String(),
		Components: components,
	}

	status := http.StatusOK
	if overall == StatusDown {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// Register 在 HTTP 路由上注册健康检查端点。
// 注册 GET /healthz（存活探针）和 GET /readyz（就绪探针）。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.LivenessHandler())
	mux.HandleFunc("GET /readyz", h.ReadinessHandler())
}
