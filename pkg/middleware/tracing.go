// tracing.go 实现 W3C Trace Context 链路追踪中间件，
// 提取或生成 traceparent 头并注入请求上下文，用于分布式链路追踪关联。

package middleware

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/opsnexus/opsnexus/pkg/logger"
)

const (
	// HeaderTraceParent 是 W3C Trace Context 标准的 traceparent 头
	HeaderTraceParent = "traceparent"
	// HeaderTraceState 是 W3C Trace Context 标准的 tracestate 头
	HeaderTraceState = "tracestate"
)

// Tracing 提取或生成 W3C Trace Context 头并注入请求上下文，用于下游链路关联。
// 当集成 OpenTelemetry SDK 后，此中间件将被 otelhttp handler 替代。
// 当前实现为脚手架阶段提供链路上下文传播能力。
func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceParent := r.Header.Get(HeaderTraceParent)
		traceID, spanID := parseTraceParent(traceParent)

		if traceID == "" {
			// Generate new trace context
			traceID = strings.ReplaceAll(uuid.New().String(), "-", "")
			spanID = strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
		}

		ctx := logger.ContextWithTrace(r.Context(), traceID, spanID)

		// Propagate trace headers downstream
		w.Header().Set(HeaderTraceParent, formatTraceParent(traceID, spanID))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// parseTraceParent 从 W3C traceparent 头中提取 trace_id 和 span_id。
// 格式：{版本}-{trace_id}-{parent_id}-{trace_flags}
// 示例：00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
func parseTraceParent(header string) (traceID, spanID string) {
	if header == "" {
		return "", ""
	}

	parts := strings.Split(header, "-")
	if len(parts) != 4 {
		return "", ""
	}

	traceID = parts[1]
	spanID = parts[2]

	if len(traceID) != 32 || len(spanID) != 16 {
		return "", ""
	}

	return traceID, spanID
}

// formatTraceParent 生成 W3C traceparent 头的值。
func formatTraceParent(traceID, spanID string) string {
	return "00-" + traceID + "-" + spanID + "-01"
}
