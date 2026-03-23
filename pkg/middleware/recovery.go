// recovery.go 实现 panic 恢复中间件，防止单个请求的 panic 导致整个服务崩溃。

package middleware

import (
	"net/http"
	"runtime/debug"

	"go.uber.org/zap"
)

// Recovery 返回 panic 恢复中间件，捕获 panic 后记录堆栈信息并返回 500 JSON 错误响应。
func Recovery(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()
					log.Error("panic recovered",
						zap.Any("panic", rec),
						zap.String("stack", string(stack)),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
					)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"code":"internal.panic","message":"internal server error"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
