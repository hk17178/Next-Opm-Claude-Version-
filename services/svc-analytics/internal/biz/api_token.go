// api_token.go 实现 API Token 管理的数据模型、仓储接口和业务逻辑用例。
// API Token 用于第三方系统集成，支持长期访问令牌的生成、验证、吊销和用量统计。
package biz

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
)

// Token 前缀，用于标识 OpsNexus 平台颁发的 API Token
const tokenPrefix = "opn_"

// Token 随机部分的字节长度（生成 64 个十六进制字符）
const tokenRandomBytes = 32

// APIToken 代表一个用于第三方集成的长期访问令牌。
// 明文令牌仅在创建时返回一次，后续只存储 SHA-256 哈希值用于验证。
type APIToken struct {
	ID          string     `json:"id" db:"id"`                       // 令牌唯一 ID（UUID）
	Name        string     `json:"name" db:"name"`                   // 令牌用途备注
	TokenHash   string     `json:"-" db:"token_hash"`                // SHA-256 哈希值（明文只展示一次）
	TokenPrefix string     `json:"token_prefix" db:"token_prefix"`   // 令牌前缀（用于展示，如 "opn_xxxx..."）
	Permissions []string   `json:"permissions" db:"permissions"`     // 权限范围：["read", "write", "alert:read", "incident:write"]
	ExpiresAt   *time.Time `json:"expires_at,omitempty" db:"expires_at"`   // 过期时间（nil 表示永不过期）
	LastUsedAt  *time.Time `json:"last_used_at,omitempty" db:"last_used_at"` // 最后使用时间
	LastUsedIP  string     `json:"last_used_ip,omitempty" db:"last_used_ip"` // 最后使用来源 IP
	CallCount   int64      `json:"call_count" db:"call_count"`       // 累计调用次数
	CreatedBy   string     `json:"created_by" db:"created_by"`       // 创建人
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`       // 创建时间
	Revoked     bool       `json:"revoked" db:"revoked"`             // 是否已吊销
}

// APITokenResult 是创建 Token 后的返回结果，包含明文令牌（仅此一次可见）。
type APITokenResult struct {
	Token    *APIToken `json:"token"`      // Token 元数据（不含明文）
	RawToken string    `json:"raw_token"`  // 明文令牌，仅在创建时返回一次
}

// APITokenRepo 定义 API Token 数据访问接口。
type APITokenRepo interface {
	// Create 将新 Token 记录持久化到数据库
	Create(ctx context.Context, token *APIToken) error
	// GetByHash 根据令牌哈希值查询 Token（用于认证验证）
	GetByHash(ctx context.Context, hash string) (*APIToken, error)
	// List 列出指定用户创建的所有 Token
	List(ctx context.Context, createdBy string) ([]*APIToken, error)
	// Revoke 吊销指定 ID 的 Token
	Revoke(ctx context.Context, id string) error
	// UpdateUsage 更新 Token 的最后使用时间、来源 IP 和累计调用次数
	UpdateUsage(ctx context.Context, id string, ip string) error
}

// APITokenUsecase 封装 API Token 管理的业务逻辑。
type APITokenUsecase struct {
	repo APITokenRepo // 数据访问层
}

// NewAPITokenUsecase 创建 API Token 业务用例实例。
func NewAPITokenUsecase(repo APITokenRepo) *APITokenUsecase {
	return &APITokenUsecase{repo: repo}
}

// Generate 生成新的 API Token，返回包含明文令牌的结果（明文仅此一次可见）。
// 令牌格式：opn_<64个随机十六进制字符>
// 参数：
//   - name: 令牌用途描述
//   - permissions: 权限列表
//   - expiresIn: 有效期时长，nil 表示永不过期
//   - createdBy: 创建者标识
func (u *APITokenUsecase) Generate(ctx context.Context, name string, permissions []string, expiresIn *time.Duration, createdBy string) (*APITokenResult, error) {
	// 参数校验
	if name == "" {
		return nil, apperrors.BadRequest("VALIDATION", "token name is required")
	}
	if len(permissions) == 0 {
		return nil, apperrors.BadRequest("VALIDATION", "at least one permission is required")
	}

	// 生成随机令牌
	randomBytes := make([]byte, tokenRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, apperrors.Internal("TOKEN_GENERATE", "failed to generate random token")
	}
	rawToken := tokenPrefix + hex.EncodeToString(randomBytes)

	// 计算 SHA-256 哈希值用于存储
	hash := HashToken(rawToken)

	// 计算前缀用于展示（如 "opn_abcd..."）
	displayPrefix := rawToken[:12] + "..."

	// 计算过期时间
	var expiresAt *time.Time
	if expiresIn != nil {
		t := time.Now().Add(*expiresIn)
		expiresAt = &t
	}

	token := &APIToken{
		Name:        name,
		TokenHash:   hash,
		TokenPrefix: displayPrefix,
		Permissions: permissions,
		ExpiresAt:   expiresAt,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
	}

	// 持久化到数据库
	if err := u.repo.Create(ctx, token); err != nil {
		return nil, apperrors.Internal("TOKEN_CREATE", fmt.Sprintf("failed to create token: %v", err))
	}

	return &APITokenResult{
		Token:    token,
		RawToken: rawToken,
	}, nil
}

// List 列出指定用户创建的所有 Token（不返回明文令牌和哈希值）。
func (u *APITokenUsecase) List(ctx context.Context, createdBy string) ([]*APIToken, error) {
	tokens, err := u.repo.List(ctx, createdBy)
	if err != nil {
		return nil, apperrors.Internal("TOKEN_LIST", fmt.Sprintf("failed to list tokens: %v", err))
	}
	return tokens, nil
}

// Revoke 吊销指定 ID 的 Token。
// operatorID 用于权限校验（当前简化实现不做校验，可后续扩展）。
func (u *APITokenUsecase) Revoke(ctx context.Context, id string, operatorID string) error {
	if id == "" {
		return apperrors.BadRequest("VALIDATION", "token id is required")
	}
	if err := u.repo.Revoke(ctx, id); err != nil {
		return apperrors.Internal("TOKEN_REVOKE", fmt.Sprintf("failed to revoke token: %v", err))
	}
	return nil
}

// Authenticate 验证明文令牌的有效性，通过后更新调用统计信息。
// 验证流程：
//  1. 检查令牌前缀格式
//  2. 计算哈希值并查询数据库
//  3. 检查是否已吊销
//  4. 检查是否已过期
//  5. 更新使用统计（最后使用时间、IP、调用次数）
func (u *APITokenUsecase) Authenticate(ctx context.Context, plainToken string, ip string) (*APIToken, error) {
	// 检查令牌前缀格式
	if !strings.HasPrefix(plainToken, tokenPrefix) {
		return nil, apperrors.New(apperrors.ErrAuthTokenInvalid, "invalid token format", 401)
	}

	// 计算哈希值
	hash := HashToken(plainToken)

	// 根据哈希值查询 Token
	token, err := u.repo.GetByHash(ctx, hash)
	if err != nil {
		return nil, apperrors.New(apperrors.ErrAuthTokenInvalid, "invalid token", 401)
	}

	// 检查是否已吊销
	if token.Revoked {
		return nil, apperrors.New(apperrors.ErrAuthTokenInvalid, "token has been revoked", 401)
	}

	// 检查是否已过期
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return nil, apperrors.New(apperrors.ErrAuthTokenExpired, "token has expired", 401)
	}

	// 异步更新使用统计（不阻塞认证流程）
	_ = u.repo.UpdateUsage(ctx, token.ID, ip)

	return token, nil
}

// HashToken 计算明文令牌的 SHA-256 哈希值，返回十六进制编码字符串。
func HashToken(plainToken string) string {
	h := sha256.Sum256([]byte(plainToken))
	return hex.EncodeToString(h[:])
}

// HasPermission 检查 Token 是否具有指定权限。
// 支持通配符匹配：如 Token 拥有 "read" 权限，则匹配 "alert:read" 等子权限。
func (t *APIToken) HasPermission(required string) bool {
	for _, p := range t.Permissions {
		if p == required {
			return true
		}
		// 通配符匹配：例如 "read" 匹配 "alert:read"
		if strings.HasSuffix(required, ":"+p) {
			return true
		}
	}
	return false
}
