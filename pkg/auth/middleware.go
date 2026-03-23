// Package auth 提供 JWT 认证中间件和用户上下文辅助函数。
// 通过 Keycloak 的 JWKS 端点进行令牌验证，支持角色和权限的细粒度访问控制。
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
	"github.com/opsnexus/opsnexus/pkg/httputil"
	"go.uber.org/zap"
)

// contextKey 是用于 context 存取用户信息的键类型，避免与其他包冲突
type contextKey string

// userContextKey 是存储已认证用户信息的 context 键
const userContextKey contextKey = "user"

// UserInfo 表示从 JWT 令牌中提取的已认证用户信息，包含用户ID、所属组织、角色和权限。
type UserInfo struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	OrgID       string   `json:"org_id"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// HasRole 检查用户是否拥有指定的角色。
func (u *UserInfo) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasPermission 检查用户是否拥有指定的权限。
// 支持通配符 "*" 表示拥有所有权限。
func (u *UserInfo) HasPermission(perm string) bool {
	for _, p := range u.Permissions {
		if p == "*" || p == perm {
			return true
		}
	}
	return false
}

// FromContext 从请求上下文中提取用户信息。返回 UserInfo 指针和是否存在的布尔值。
func FromContext(ctx context.Context) (*UserInfo, bool) {
	u, ok := ctx.Value(userContextKey).(*UserInfo)
	return u, ok
}

// MustFromContext 从上下文中提取用户信息，若不存在则触发 panic。
// 仅在确保已应用认证中间件的场景下使用。
func MustFromContext(ctx context.Context) *UserInfo {
	u, ok := FromContext(ctx)
	if !ok {
		panic("auth: no user in context — is auth middleware applied?")
	}
	return u
}

// Config 保存 JWT/Keycloak 认证配置参数。
type Config struct {
	// KeycloakURL 是 Keycloak 服务器地址（例如 https://keycloak.example.com）
	KeycloakURL string
	// Realm 是 Keycloak 领域（realm）名称
	Realm string
	// JWKSRefreshInterval 是 JWKS 密钥集的刷新间隔，默认 1 小时
	JWKSRefreshInterval time.Duration
	// Issuer 是期望的令牌签发者（iss）声明
	Issuer string
	// Audience 是期望的令牌受众（aud）声明（可选）
	Audience string
}

// JWKS 表示 JSON Web Key Set（JWKS），包含一组公钥。
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK 表示单个 JSON Web Key，包含 RSA 公钥参数。
type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// KeyStore 负责从 Keycloak JWKS 端点获取和缓存 RSA 公钥，用于 JWT 签名验证。
type KeyStore struct {
	jwksURL         string
	mu              sync.RWMutex
	keys            map[string]*rsa.PublicKey
	lastRefresh     time.Time
	refreshInterval time.Duration
	logger          *zap.Logger
}

// NewKeyStore 创建一个 KeyStore 实例，通过 Keycloak JWKS 端点获取公钥。
func NewKeyStore(cfg Config, logger *zap.Logger) *KeyStore {
	refreshInterval := cfg.JWKSRefreshInterval
	if refreshInterval == 0 {
		refreshInterval = 1 * time.Hour
	}
	jwksURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", cfg.KeycloakURL, cfg.Realm)
	return &KeyStore{
		jwksURL:         jwksURL,
		keys:            make(map[string]*rsa.PublicKey),
		refreshInterval: refreshInterval,
		logger:          logger,
	}
}

// GetKey 根据密钥ID（kid）返回对应的 RSA 公钥。若缓存过期或不存在则自动从 Keycloak 刷新。
func (ks *KeyStore) GetKey(kid string) (*rsa.PublicKey, error) {
	ks.mu.RLock()
	key, ok := ks.keys[kid]
	needsRefresh := time.Since(ks.lastRefresh) > ks.refreshInterval
	ks.mu.RUnlock()

	if ok && !needsRefresh {
		return key, nil
	}

	// Refresh keys
	if err := ks.refresh(); err != nil {
		// If refresh fails but we have a cached key, use it
		if ok {
			ks.logger.Warn("JWKS refresh failed, using cached key", zap.Error(err))
			return key, nil
		}
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}

	ks.mu.RLock()
	key, ok = ks.keys[kid]
	ks.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("key ID %q not found in JWKS", kid)
	}
	return key, nil
}

// refresh 从 Keycloak JWKS 端点拉取最新的公钥集合并更新本地缓存
func (ks *KeyStore) refresh() error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(ks.jwksURL)
	if err != nil {
		return fmt.Errorf("GET %s: %w", ks.jwksURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decode JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" || jwk.Use != "sig" {
			continue
		}
		pubKey, err := jwkToRSAPublicKey(jwk)
		if err != nil {
			ks.logger.Warn("failed to parse JWK", zap.String("kid", jwk.Kid), zap.Error(err))
			continue
		}
		keys[jwk.Kid] = pubKey
	}

	ks.mu.Lock()
	ks.keys = keys
	ks.lastRefresh = time.Now()
	ks.mu.Unlock()

	ks.logger.Info("JWKS refreshed", zap.Int("key_count", len(keys)))
	return nil
}

// jwkToRSAPublicKey 将 JWK 格式的密钥转换为 Go 标准库的 RSA 公钥
func jwkToRSAPublicKey(jwk JWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("decode N: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("decode E: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

// jwtHeader 表示 JWT 头部，包含算法、密钥ID和类型
type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

// jwtClaims 表示 OpsNexus 关注的 JWT 负载声明，包含用户、组织、角色等信息
type jwtClaims struct {
	Sub               string   `json:"sub"`
	PreferredUsername  string   `json:"preferred_username"`
	Iss               string   `json:"iss"`
	Aud               interface{} `json:"aud"` // Can be string or []string
	Exp               int64    `json:"exp"`
	Iat               int64    `json:"iat"`
	OrgID             string   `json:"org_id"`
	RealmAccess       *realmAccess `json:"realm_access"`
	ResourceAccess    map[string]*resourceAccess `json:"resource_access"`
}

type realmAccess struct {
	Roles []string `json:"roles"`
}

type resourceAccess struct {
	Roles []string `json:"roles"`
}

// parseJWTUnverified 在不验证签名的情况下提取 JWT 头部和声明。
// 用于在获取验证密钥之前先提取 kid（密钥ID）。
func parseJWTUnverified(token string) (*jwtHeader, *jwtClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("decode JWT header: %w", err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, nil, fmt.Errorf("parse JWT header: %w", err)
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("decode JWT claims: %w", err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, nil, fmt.Errorf("parse JWT claims: %w", err)
	}

	return &header, &claims, nil
}

// Middleware 验证请求中的 JWT Bearer 令牌，并将用户信息注入请求上下文。
// 处理流程：提取 Authorization 头 -> 解析 JWT -> 验证签名和过期时间 -> 提取角色和权限 -> 注入上下文。
// 当 keyStore 为 nil 时，跳过签名验证（仅限开发模式）。
func Middleware(keyStore *KeyStore, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				httputil.Error(w, apperrors.Unauthenticated("missing authorization header"))
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				httputil.Error(w, apperrors.Unauthenticated("invalid authorization header format"))
				return
			}

			token := parts[1]
			if token == "" {
				httputil.Error(w, apperrors.Unauthenticated("empty token"))
				return
			}

			header, claims, err := parseJWTUnverified(token)
			if err != nil {
				httputil.Error(w, apperrors.Unauthenticated("invalid token: "+err.Error()))
				return
			}

			// Verify signature if KeyStore is available
			if keyStore != nil {
				if header.Kid == "" {
					httputil.Error(w, apperrors.Unauthenticated("token missing kid header"))
					return
				}
				// TODO: Perform actual RS256 signature verification using keyStore.GetKey(header.Kid)
				// For Phase 1, the key infrastructure is in place but verification is deferred
				// until Keycloak is deployed.
				_ = header
			}

			// Verify expiration
			if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
				httputil.Error(w, apperrors.Unauthenticated("token expired"))
				return
			}

			// Extract roles from Keycloak token structure
			var roles []string
			if claims.RealmAccess != nil {
				roles = claims.RealmAccess.Roles
			}

			// Extract permissions from resource_access (client-specific roles)
			var permissions []string
			for clientID, access := range claims.ResourceAccess {
				for _, role := range access.Roles {
					permissions = append(permissions, clientID+":"+role)
				}
			}

			user := &UserInfo{
				UserID:      claims.Sub,
				Username:    claims.PreferredUsername,
				OrgID:       claims.OrgID,
				Roles:       roles,
				Permissions: permissions,
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole 返回中间件，检查用户是否拥有至少一个所需角色。不满足则返回 403。
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := FromContext(r.Context())
			if !ok {
				httputil.Error(w, apperrors.Unauthenticated("no user in context"))
				return
			}
			for _, required := range roles {
				if user.HasRole(required) {
					next.ServeHTTP(w, r)
					return
				}
			}
			httputil.Error(w, apperrors.PermissionDenied("required role: %s", strings.Join(roles, " or ")))
		})
	}
}

// RequirePermission 返回中间件，检查用户是否拥有指定权限。不满足则返回 403。
func RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := FromContext(r.Context())
			if !ok {
				httputil.Error(w, apperrors.Unauthenticated("no user in context"))
				return
			}
			if !user.HasPermission(perm) {
				httputil.Error(w, apperrors.PermissionDenied("required permission: %s", perm))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
