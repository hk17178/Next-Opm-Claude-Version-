package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery returns middleware that recovers from panics and logs the stack trace.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()
					logger.ErrorContext(r.Context(), "panic recovered",
						slog.Any("panic", rec),
						slog.String("stack", string(stack)),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
					)
					http.Error(w, `{"code":"INTERNAL_ERROR","message":"internal server error"}`, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
