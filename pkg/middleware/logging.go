// logging.go 实现 HTTP 请求日志中间件，记录每个请求的方法、路径、状态码、耗时等信息。

package middleware

import (
	"net/http"
	"time"

	pkglogger "github.com/opsnexus/opsnexus/pkg/logger"
	"go.uber.org/zap"
)

// responseWriter 包装 http.ResponseWriter 以捕获响应状态码和写入字节数。
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// Logging 返回记录每个 HTTP 请求的结构化日志中间件。
// 日志级别：2xx/3xx 用 INFO，4xx 用 WARN，5xx 用 ERROR。
func Logging(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newResponseWriter(w)

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			// Enrich with trace context if present
			l := pkglogger.WithTraceContext(r.Context(), log)

			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", r.URL.RawQuery),
				zap.Int("status", rw.statusCode),
				zap.Duration("duration", duration),
				zap.Int64("response_bytes", rw.written),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
				zap.String("request_id", pkglogger.RequestIDFromContext(r.Context())),
			}

			switch {
			case rw.statusCode >= 500:
				l.Error("http request", fields...)
			case rw.statusCode >= 400:
				l.Warn("http request", fields...)
			default:
				l.Info("http request", fields...)
			}
		})
	}
}
