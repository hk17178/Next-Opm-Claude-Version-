package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAPITokenRepo 是 APITokenRepo 的内存实现，用于单元测试。
type mockAPITokenRepo struct {
	tokens map[string]*APIToken // 以 ID 为键存储 Token
}

func newMockAPITokenRepo() *mockAPITokenRepo {
	return &mockAPITokenRepo{
		tokens: make(map[string]*APIToken),
	}
}

func (m *mockAPITokenRepo) Create(_ context.Context, token *APIToken) error {
	if token.ID == "" {
		token.ID = "test-uuid-" + token.TokenPrefix
	}
	m.tokens[token.ID] = token
	return nil
}

func (m *mockAPITokenRepo) GetByHash(_ context.Context, hash string) (*APIToken, error) {
	for _, t := range m.tokens {
		if t.TokenHash == hash {
			return t, nil
		}
	}
	return nil, assert.AnError
}

func (m *mockAPITokenRepo) List(_ context.Context, createdBy string) ([]*APIToken, error) {
	var result []*APIToken
	for _, t := range m.tokens {
		if t.CreatedBy == createdBy {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockAPITokenRepo) Revoke(_ context.Context, id string) error {
	if t, ok := m.tokens[id]; ok {
		t.Revoked = true
		return nil
	}
	return assert.AnError
}

func (m *mockAPITokenRepo) UpdateUsage(_ context.Context, id string, ip string) error {
	if t, ok := m.tokens[id]; ok {
		now := time.Now()
		t.LastUsedAt = &now
		t.LastUsedIP = ip
		t.CallCount++
		return nil
	}
	return assert.AnError
}

// TestGenerateTokenFormat 验证生成的 Token 格式正确：以 "opn_" 开头，总长度为 68 字符。
func TestGenerateTokenFormat(t *testing.T) {
	repo := newMockAPITokenRepo()
	uc := NewAPITokenUsecase(repo)

	result, err := uc.Generate(context.Background(), "test-token", []string{"read"}, nil, "user1")
	require.NoError(t, err)
	require.NotNil(t, result)

	// 验证明文令牌格式
	assert.True(t, strings.HasPrefix(result.RawToken, "opn_"), "令牌应以 opn_ 前缀开头")
	// opn_ (4 字符) + 64 个十六进制字符 = 68
	assert.Len(t, result.RawToken, 68, "令牌总长度应为 68 字符")

	// 验证 Token 元数据
	assert.Equal(t, "test-token", result.Token.Name)
	assert.Equal(t, []string{"read"}, result.Token.Permissions)
	assert.Equal(t, "user1", result.Token.CreatedBy)
	assert.False(t, result.Token.Revoked)
	assert.Nil(t, result.Token.ExpiresAt)

	// 验证前缀用于展示
	assert.True(t, strings.HasSuffix(result.Token.TokenPrefix, "..."), "展示前缀应以 ... 结尾")
}

// TestGenerateTokenWithExpiration 验证带有效期的 Token 生成正确。
func TestGenerateTokenWithExpiration(t *testing.T) {
	repo := newMockAPITokenRepo()
	uc := NewAPITokenUsecase(repo)

	duration := 24 * time.Hour
	result, err := uc.Generate(context.Background(), "expiring-token", []string{"read"}, &duration, "user1")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotNil(t, result.Token.ExpiresAt, "应设置过期时间")
	assert.True(t, result.Token.ExpiresAt.After(time.Now()), "过期时间应在未来")
	assert.True(t, result.Token.ExpiresAt.Before(time.Now().Add(25*time.Hour)), "过期时间应在 25 小时内")
}

// TestHashTokenConsistency 验证相同明文令牌始终生成相同的哈希值。
func TestHashTokenConsistency(t *testing.T) {
	token := "opn_abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	hash1 := HashToken(token)
	hash2 := HashToken(token)

	assert.Equal(t, hash1, hash2, "相同令牌的哈希值应一致")
	assert.Len(t, hash1, 64, "SHA-256 哈希值应为 64 个十六进制字符")
}

// TestHashTokenUniqueness 验证不同明文令牌生成不同的哈希值。
func TestHashTokenUniqueness(t *testing.T) {
	hash1 := HashToken("opn_token_a")
	hash2 := HashToken("opn_token_b")

	assert.NotEqual(t, hash1, hash2, "不同令牌的哈希值应不同")
}

// TestAuthenticateSuccess 验证正确的令牌可以通过认证。
func TestAuthenticateSuccess(t *testing.T) {
	repo := newMockAPITokenRepo()
	uc := NewAPITokenUsecase(repo)

	// 先生成一个 Token
	result, err := uc.Generate(context.Background(), "auth-test", []string{"read", "write"}, nil, "user1")
	require.NoError(t, err)

	// 使用明文令牌进行认证
	token, err := uc.Authenticate(context.Background(), result.RawToken, "192.168.1.1")
	require.NoError(t, err)
	require.NotNil(t, token)

	assert.Equal(t, "auth-test", token.Name)
	assert.Equal(t, []string{"read", "write"}, token.Permissions)
}

// TestAuthenticateInvalidFormat 验证格式错误的令牌被拒绝。
func TestAuthenticateInvalidFormat(t *testing.T) {
	repo := newMockAPITokenRepo()
	uc := NewAPITokenUsecase(repo)

	// 不以 opn_ 开头的令牌应被拒绝
	_, err := uc.Authenticate(context.Background(), "invalid_token_format", "192.168.1.1")
	assert.Error(t, err, "非 opn_ 前缀的令牌应认证失败")
}

// TestAuthenticateRevokedToken 验证已吊销的令牌无法通过认证。
func TestAuthenticateRevokedToken(t *testing.T) {
	repo := newMockAPITokenRepo()
	uc := NewAPITokenUsecase(repo)

	// 生成并吊销 Token
	result, err := uc.Generate(context.Background(), "revoke-test", []string{"read"}, nil, "user1")
	require.NoError(t, err)

	err = uc.Revoke(context.Background(), result.Token.ID, "user1")
	require.NoError(t, err)

	// 尝试使用已吊销的令牌认证
	_, err = uc.Authenticate(context.Background(), result.RawToken, "192.168.1.1")
	assert.Error(t, err, "已吊销的令牌应认证失败")
}

// TestAuthenticateExpiredToken 验证过期令牌无法通过认证。
func TestAuthenticateExpiredToken(t *testing.T) {
	repo := newMockAPITokenRepo()
	uc := NewAPITokenUsecase(repo)

	// 生成一个已过期的 Token（有效期为 -1 小时，即立即过期）
	duration := -1 * time.Hour
	result, err := uc.Generate(context.Background(), "expired-test", []string{"read"}, &duration, "user1")
	require.NoError(t, err)

	// 尝试使用已过期的令牌认证
	_, err = uc.Authenticate(context.Background(), result.RawToken, "192.168.1.1")
	assert.Error(t, err, "已过期的令牌应认证失败")
}

// TestHasPermission 验证权限检查逻辑。
func TestHasPermission(t *testing.T) {
	token := &APIToken{
		Permissions: []string{"read", "write", "alert:read"},
	}

	// 精确匹配
	assert.True(t, token.HasPermission("read"), "应匹配 read 权限")
	assert.True(t, token.HasPermission("write"), "应匹配 write 权限")
	assert.True(t, token.HasPermission("alert:read"), "应匹配 alert:read 权限")

	// 通配符匹配：拥有 "read" 权限时应匹配 "incident:read"
	assert.True(t, token.HasPermission("incident:read"), "read 权限应通配匹配 incident:read")

	// 不应匹配的权限
	assert.False(t, token.HasPermission("admin"), "不应匹配 admin 权限")
	assert.False(t, token.HasPermission("delete"), "不应匹配 delete 权限")
}

// TestGenerateValidationErrors 验证参数校验错误。
func TestGenerateValidationErrors(t *testing.T) {
	repo := newMockAPITokenRepo()
	uc := NewAPITokenUsecase(repo)

	// 空名称
	_, err := uc.Generate(context.Background(), "", []string{"read"}, nil, "user1")
	assert.Error(t, err, "空名称应返回错误")

	// 空权限
	_, err = uc.Generate(context.Background(), "test", []string{}, nil, "user1")
	assert.Error(t, err, "空权限列表应返回错误")

	// nil 权限
	_, err = uc.Generate(context.Background(), "test", nil, nil, "user1")
	assert.Error(t, err, "nil 权限列表应返回错误")
}

// TestListTokens 验证按创建者列出 Token。
func TestListTokens(t *testing.T) {
	repo := newMockAPITokenRepo()
	uc := NewAPITokenUsecase(repo)

	// 为不同用户创建 Token
	_, err := uc.Generate(context.Background(), "user1-token1", []string{"read"}, nil, "user1")
	require.NoError(t, err)
	_, err = uc.Generate(context.Background(), "user1-token2", []string{"write"}, nil, "user1")
	require.NoError(t, err)
	_, err = uc.Generate(context.Background(), "user2-token1", []string{"read"}, nil, "user2")
	require.NoError(t, err)

	// 列出 user1 的 Token
	tokens, err := uc.List(context.Background(), "user1")
	require.NoError(t, err)
	assert.Len(t, tokens, 2, "user1 应有 2 个 Token")

	// 列出 user2 的 Token
	tokens, err = uc.List(context.Background(), "user2")
	require.NoError(t, err)
	assert.Len(t, tokens, 1, "user2 应有 1 个 Token")
}

// TestRevokeToken 验证 Token 吊销流程。
func TestRevokeToken(t *testing.T) {
	repo := newMockAPITokenRepo()
	uc := NewAPITokenUsecase(repo)

	result, err := uc.Generate(context.Background(), "revoke-test", []string{"read"}, nil, "user1")
	require.NoError(t, err)

	// 吊销 Token
	err = uc.Revoke(context.Background(), result.Token.ID, "user1")
	require.NoError(t, err)

	// 验证 Token 已被标记为吊销
	assert.True(t, repo.tokens[result.Token.ID].Revoked, "Token 应被标记为已吊销")
}

// TestRevokeEmptyID 验证空 ID 吊销返回错误。
func TestRevokeEmptyID(t *testing.T) {
	repo := newMockAPITokenRepo()
	uc := NewAPITokenUsecase(repo)

	err := uc.Revoke(context.Background(), "", "user1")
	assert.Error(t, err, "空 ID 应返回错误")
}

// TestTokenUniqueGeneration 验证每次生成的 Token 都是唯一的。
func TestTokenUniqueGeneration(t *testing.T) {
	repo := newMockAPITokenRepo()
	uc := NewAPITokenUsecase(repo)

	tokens := make(map[string]bool)
	for i := 0; i < 10; i++ {
		result, err := uc.Generate(context.Background(), "unique-test", []string{"read"}, nil, "user1")
		require.NoError(t, err)
		assert.False(t, tokens[result.RawToken], "生成的 Token 应唯一")
		tokens[result.RawToken] = true
	}
}
