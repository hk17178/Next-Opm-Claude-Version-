// Package middleware 提供 OpsNexus 所有服务共享的 HTTP 中间件。
// 中间件在路由层应用，每个请求都会经过。
// 包含：请求ID注入、链路追踪、日志记录、CORS、限流、panic 恢复等。
package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/opsnexus/opsnexus/pkg/logger"
)

const (
	// HeaderRequestID 是用于请求ID传播的 HTTP 头名称
	HeaderRequestID = "X-Request-Id"
)

// RequestID 为每个请求注入唯一的请求ID到上下文和响应头中。
// 如果请求已携带 X-Request-Id 头（来自 API 网关），则复用该值。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(HeaderRequestID)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := logger.ContextWithRequestID(r.Context(), requestID)
		w.Header().Set(HeaderRequestID, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
