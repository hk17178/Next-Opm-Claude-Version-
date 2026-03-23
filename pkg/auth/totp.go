// totp.go 实现基于 TOTP（基于时间的一次性密码）的双因素认证。
// 包括密钥生成、验证码校验、恢复码生成和使用。
// 恢复码使用 SHA-256 哈希存储，明文只展示一次。

package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TOTPSetup TOTP 设置信息，包含密钥和配置 URL
type TOTPSetup struct {
	Secret    string `json:"secret"`      // Base32 编码的密钥
	URL       string `json:"url"`         // otpauth:// 协议 URL，可用于生成二维码
	Issuer    string `json:"issuer"`      // 发行方名称
	AccountID string `json:"account_id"`  // 账户标识（通常为用户 ID）
}

// TOTPConfig TOTP 管理器配置
type TOTPConfig struct {
	Digits    int           // 验证码位数（默认 6）
	Period    int           // 验证码有效时间窗口（秒，默认 30）
	Skew     int           // 允许的时间偏移窗口数（默认 1，即前后各 1 个周期）
	SecretLen int           // 密钥长度（字节，默认 20）
}

// DefaultTOTPConfig 返回默认的 TOTP 配置
func DefaultTOTPConfig() TOTPConfig {
	return TOTPConfig{
		Digits:    6,
		Period:    30,
		Skew:     1,
		SecretLen: 20,
	}
}

// TOTPManager TOTP 双因素认证管理器
type TOTPManager struct {
	config        TOTPConfig
	mu            sync.RWMutex
	recoveryCodes map[string][]string // key: userID, value: SHA-256 哈希后的恢复码列表
}

// NewTOTPManager 创建 TOTP 管理器实例
func NewTOTPManager(config TOTPConfig) *TOTPManager {
	// 填充默认值
	if config.Digits <= 0 {
		config.Digits = 6
	}
	if config.Period <= 0 {
		config.Period = 30
	}
	if config.Skew < 0 {
		config.Skew = 1
	}
	if config.SecretLen <= 0 {
		config.SecretLen = 20
	}

	return &TOTPManager{
		config:        config,
		recoveryCodes: make(map[string][]string),
	}
}

// GenerateSecret 为用户生成 TOTP 密钥和配置 URL。
// 返回的 TOTPSetup 包含 Base32 编码的密钥和可用于生成二维码的 otpauth:// URL。
func (m *TOTPManager) GenerateSecret(userID, issuer string) (*TOTPSetup, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID 不能为空")
	}
	if issuer == "" {
		issuer = "OpsNexus"
	}

	// 生成随机密钥
	secret := make([]byte, m.config.SecretLen)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("生成随机密钥失败: %w", err)
	}

	// Base32 编码（去除填充字符，标准 TOTP 格式）
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)

	// 构建 otpauth:// URL
	// 格式：otpauth://totp/{issuer}:{account}?secret={secret}&issuer={issuer}&digits={digits}&period={period}
	otpauthURL := fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&digits=%d&period=%d",
		url.PathEscape(issuer),
		url.PathEscape(userID),
		encoded,
		url.QueryEscape(issuer),
		m.config.Digits,
		m.config.Period,
	)

	return &TOTPSetup{
		Secret:    encoded,
		URL:       otpauthURL,
		Issuer:    issuer,
		AccountID: userID,
	}, nil
}

// Verify 验证 TOTP 验证码。
// 在当前时间窗口及允许的偏移范围内验证 6 位数字码。
// 使用 constant-time 比较防止时序攻击。
func (m *TOTPManager) Verify(secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}

	// 解码 Base32 密钥
	secretBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return false
	}

	now := time.Now().Unix()
	period := int64(m.config.Period)

	// 在允许的时间偏移范围内逐一验证
	for i := -m.config.Skew; i <= m.config.Skew; i++ {
		counter := (now / period) + int64(i)
		expected := generateTOTPCode(secretBytes, counter, m.config.Digits)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return true
		}
	}

	return false
}

// generateTOTPCode 基于 HMAC-SHA1 生成 TOTP 验证码
// 实现 RFC 6238 (TOTP) 和 RFC 4226 (HOTP) 算法
func generateTOTPCode(secret []byte, counter int64, digits int) string {
	// 将计数器转换为 8 字节大端序
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))

	// 计算 HMAC-SHA1
	mac := hmac.New(sha256.New, secret)
	mac.Write(buf)
	hash := mac.Sum(nil)

	// 动态截取（Dynamic Truncation）
	offset := hash[len(hash)-1] & 0x0F
	truncated := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7FFFFFFF

	// 取模得到指定位数的验证码
	mod := uint32(math.Pow10(digits))
	otp := truncated % mod

	// 格式化为固定位数（前补零）
	format := fmt.Sprintf("%%0%dd", digits)
	return fmt.Sprintf(format, otp)
}

// GenerateRecoveryCodes 生成 8 个一次性恢复码。
// 返回明文恢复码（仅展示一次），同时以 SHA-256 哈希形式存储。
func (m *TOTPManager) GenerateRecoveryCodes() ([]string, error) {
	const codeCount = 8
	const codeLen = 8 // 每个恢复码 8 个字符

	codes := make([]string, codeCount)
	for i := 0; i < codeCount; i++ {
		code, err := generateRandomCode(codeLen)
		if err != nil {
			return nil, fmt.Errorf("生成恢复码失败: %w", err)
		}
		codes[i] = code
	}

	return codes, nil
}

// StoreRecoveryCodes 将恢复码的 SHA-256 哈希存储到内存中。
// 生产环境应替换为持久化存储（如 PostgreSQL）。
func (m *TOTPManager) StoreRecoveryCodes(_ context.Context, userID string, codes []string) error {
	if userID == "" {
		return fmt.Errorf("userID 不能为空")
	}

	hashes := make([]string, len(codes))
	for i, code := range codes {
		hashes[i] = hashCode(code)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.recoveryCodes[userID] = hashes
	return nil
}

// UseRecoveryCode 使用恢复码验证身份。
// 验证通过后该恢复码立即失效（一次性使用）。
func (m *TOTPManager) UseRecoveryCode(_ context.Context, userID, code string) error {
	if userID == "" {
		return fmt.Errorf("userID 不能为空")
	}
	if code == "" {
		return fmt.Errorf("恢复码不能为空")
	}

	codeHash := hashCode(code)

	m.mu.Lock()
	defer m.mu.Unlock()

	hashes, ok := m.recoveryCodes[userID]
	if !ok || len(hashes) == 0 {
		return fmt.Errorf("无可用恢复码")
	}

	// 查找匹配的恢复码（constant-time 比较）
	for i, h := range hashes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(codeHash)) == 1 {
			// 移除已使用的恢复码
			m.recoveryCodes[userID] = append(hashes[:i], hashes[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("恢复码无效")
}

// GetRemainingRecoveryCodes 获取用户剩余恢复码数量
func (m *TOTPManager) GetRemainingRecoveryCodes(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.recoveryCodes[userID])
}

// hashCode 使用 SHA-256 对恢复码进行哈希处理
func hashCode(code string) string {
	h := sha256.Sum256([]byte(code))
	return hex.EncodeToString(h[:])
}

// generateRandomCode 生成指定长度的随机字母数字恢复码
func generateRandomCode(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	randomBytes := make([]byte, length)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	for i := 0; i < length; i++ {
		result[i] = charset[int(randomBytes[i])%len(charset)]
	}

	return string(result), nil
}
