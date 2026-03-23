// token_auth_middleware.go 实现 API Token 认证中间件。
// 支持与 JWT 认证并存：先检查 Bearer token 格式，
// 以 "opn_" 前缀的令牌走 API Token 认证路径，其余放行给后续中间件处理。
package service

import (
	"context"
	"net/http"
	"strings"

	"go.uber.org/zap"

	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
	"github.com/opsnexus/opsnexus/pkg/httputil"
	"github.com/opsnexus/svc-analytics/internal/biz"
)

// apiTokenContextKey 是 API Token 在 context 中的存储键类型（避免键冲突）
type apiTokenContextKey struct{}

// TokenAuthMiddleware 创建 API Token 认证中间件。
// 解析请求头 Authorization: Bearer opn_xxx，验证通过后将 APIToken 信息注入 context。
// 支持与 JWT 认证并存：
//   - 若 Bearer token 以 "opn_" 开头，走 API Token 认证
//   - 若无 Authorization 头或非 "opn_" 前缀，直接放行给后续中间件（如 JWT 认证）
func TokenAuthMiddleware(usecase *biz.APITokenUsecase, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 提取 Authorization 请求头
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// 无认证头，放行给后续中间件处理
				next.ServeHTTP(w, r)
				return
			}

			// 解析 Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				// 非 Bearer 格式，放行
				next.ServeHTTP(w, r)
				return
			}

			token := parts[1]

			// 检查是否为 API Token 格式（opn_ 前缀）
			if !strings.HasPrefix(token, "opn_") {
				// 非 API Token（可能是 JWT），放行给后续中间件
				next.ServeHTTP(w, r)
				return
			}

			// 获取客户端 IP 地址
			clientIP := extractClientIP(r)

			// 验证 API Token
			apiToken, err := usecase.Authenticate(r.Context(), token, clientIP)
			if err != nil {
				logger.Warn("API Token 认证失败",
					zap.String("token_prefix", token[:min(12, len(token))]),
					zap.String("client_ip", clientIP),
					zap.Error(err),
				)
				httputil.Error(w, apperrors.Unauthenticated("invalid or expired API token"))
				return
			}

			// 将认证通过的 Token 信息注入 context
			ctx := context.WithValue(r.Context(), apiTokenContextKey{}, apiToken)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// APITokenFromContext 从 context 中提取 API Token 信息。
// 若 context 中不存在 API Token，返回 nil。
func APITokenFromContext(ctx context.Context) *biz.APIToken {
	token, _ := ctx.Value(apiTokenContextKey{}).(*biz.APIToken)
	return token
}

// extractClientIP 从请求中提取客户端 IP 地址。
// 优先从 X-Forwarded-For 和 X-Real-IP 头获取（适配反向代理场景），
// 回退到 RemoteAddr。
func extractClientIP(r *http.Request) string {
	// 优先检查 X-Forwarded-For（取第一个 IP）
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	// 其次检查 X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// 回退到 RemoteAddr（去掉端口号）
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
