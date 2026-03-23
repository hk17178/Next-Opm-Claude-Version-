// cors.go 实现跨域资源共享（CORS）中间件，处理预检请求和响应头设置。

package middleware

import (
	"net/http"
	"strings"
)

// CORSConfig 配置 CORS 跨域行为。
type CORSConfig struct {
	AllowedOrigins []string // 允许的来源域名列表
	AllowedMethods []string // 允许的 HTTP 方法列表
	AllowedHeaders []string // 允许的请求头列表
	MaxAge         int      // 预检请求缓存时长（秒）
}

// DefaultCORSConfig 返回适用于开发环境的宽松 CORS 配置（允许所有来源）。
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-Request-Id", "X-Org-Id"},
		MaxAge:         86400,
	}
}

// CORS 返回处理跨域资源共享的中间件。
// 对 OPTIONS 预检请求直接返回 204，其他请求设置 CORS 响应头后继续处理。
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	origins := strings.Join(cfg.AllowedOrigins, ", ")
	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origins)
			w.Header().Set("Access-Control-Allow-Methods", methods)
			w.Header().Set("Access-Control-Allow-Headers", headers)
			w.Header().Set("Access-Control-Max-Age", itoa(cfg.MaxAge))

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// itoa 将整数转换为字符串，避免引入 strconv 包
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
