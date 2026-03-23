// totp_test.go 测试 TOTP 双因素认证的核心功能：
// - 密钥生成
// - 验证码验证
// - 恢复码生成和使用

package auth

import (
	"context"
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// TestGenerateSecret 测试 TOTP 密钥生成
func TestGenerateSecret(t *testing.T) {
	mgr := NewTOTPManager(DefaultTOTPConfig())

	setup, err := mgr.GenerateSecret("user-001", "OpsNexus")
	if err != nil {
		t.Fatalf("GenerateSecret 失败: %v", err)
	}

	// 验证返回的 Setup 信息
	if setup.Secret == "" {
		t.Error("密钥不应为空")
	}
	if setup.URL == "" {
		t.Error("URL 不应为空")
	}
	if setup.Issuer != "OpsNexus" {
		t.Errorf("Issuer 期望 OpsNexus，实际 %s", setup.Issuer)
	}
	if setup.AccountID != "user-001" {
		t.Errorf("AccountID 期望 user-001，实际 %s", setup.AccountID)
	}

	// 验证 URL 格式
	if !strings.HasPrefix(setup.URL, "otpauth://totp/") {
		t.Errorf("URL 应以 otpauth://totp/ 开头，实际 %s", setup.URL)
	}
	if !strings.Contains(setup.URL, "secret=") {
		t.Error("URL 应包含 secret 参数")
	}
	if !strings.Contains(setup.URL, "issuer=") {
		t.Error("URL 应包含 issuer 参数")
	}
}

// TestGenerateSecretDefaultIssuer 测试空 issuer 使用默认值
func TestGenerateSecretDefaultIssuer(t *testing.T) {
	mgr := NewTOTPManager(DefaultTOTPConfig())

	setup, err := mgr.GenerateSecret("user-001", "")
	if err != nil {
		t.Fatalf("GenerateSecret 失败: %v", err)
	}

	if setup.Issuer != "OpsNexus" {
		t.Errorf("空 issuer 应使用默认值 OpsNexus，实际 %s", setup.Issuer)
	}
}

// TestGenerateSecretEmptyUserID 测试空用户 ID
func TestGenerateSecretEmptyUserID(t *testing.T) {
	mgr := NewTOTPManager(DefaultTOTPConfig())

	_, err := mgr.GenerateSecret("", "OpsNexus")
	if err == nil {
		t.Error("空 userID 应返回错误")
	}
}

// TestVerify 测试 TOTP 验证码验证
func TestVerify(t *testing.T) {
	mgr := NewTOTPManager(DefaultTOTPConfig())

	// 生成密钥
	setup, err := mgr.GenerateSecret("user-001", "OpsNexus")
	if err != nil {
		t.Fatalf("GenerateSecret 失败: %v", err)
	}

	secret := setup.Secret

	// 手动生成一个当前时间窗口的验证码
	code := generateTestCode(t, secret, mgr.config)

	// 验证有效码
	if !mgr.Verify(secret, code) {
		t.Error("有效验证码验证失败")
	}

	// 验证空参数
	if mgr.Verify("", "123456") {
		t.Error("空密钥不应通过验证")
	}
	if mgr.Verify(secret, "") {
		t.Error("空验证码不应通过验证")
	}
}

// TestVerifyWithSkew 测试时间偏移窗口内的验证码验证
func TestVerifyWithSkew(t *testing.T) {
	config := DefaultTOTPConfig()
	config.Skew = 1 // 允许前后各 1 个时间窗口
	mgr := NewTOTPManager(config)

	setup, err := mgr.GenerateSecret("user-001", "OpsNexus")
	if err != nil {
		t.Fatalf("GenerateSecret 失败: %v", err)
	}

	// 当前时间窗口的验证码应有效
	code := generateTestCode(t, setup.Secret, mgr.config)
	if !mgr.Verify(setup.Secret, code) {
		t.Error("当前窗口验证码应有效")
	}
}

// TestGenerateRecoveryCodes 测试恢复码生成
func TestGenerateRecoveryCodes(t *testing.T) {
	mgr := NewTOTPManager(DefaultTOTPConfig())

	codes, err := mgr.GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes 失败: %v", err)
	}

	// 应生成 8 个恢复码
	if len(codes) != 8 {
		t.Errorf("期望 8 个恢复码，实际 %d 个", len(codes))
	}

	// 每个恢复码应有 8 个字符
	for i, code := range codes {
		if len(code) != 8 {
			t.Errorf("恢复码 %d 长度应为 8，实际 %d", i, len(code))
		}
	}

	// 恢复码应互不相同
	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("恢复码不应重复: %s", code)
		}
		seen[code] = true
	}
}

// TestUseRecoveryCode 测试恢复码使用
func TestUseRecoveryCode(t *testing.T) {
	mgr := NewTOTPManager(DefaultTOTPConfig())
	ctx := context.Background()

	codes, err := mgr.GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes 失败: %v", err)
	}

	userID := "user-001"

	// 存储恢复码
	if err := mgr.StoreRecoveryCodes(ctx, userID, codes); err != nil {
		t.Fatalf("StoreRecoveryCodes 失败: %v", err)
	}

	// 使用第一个恢复码
	if err := mgr.UseRecoveryCode(ctx, userID, codes[0]); err != nil {
		t.Fatalf("UseRecoveryCode 失败: %v", err)
	}

	// 同一个恢复码不能重复使用
	if err := mgr.UseRecoveryCode(ctx, userID, codes[0]); err == nil {
		t.Error("已使用的恢复码不应再次有效")
	}

	// 剩余恢复码应减少 1 个
	remaining := mgr.GetRemainingRecoveryCodes(userID)
	if remaining != 7 {
		t.Errorf("期望剩余 7 个恢复码，实际 %d 个", remaining)
	}

	// 其他恢复码应仍然有效
	if err := mgr.UseRecoveryCode(ctx, userID, codes[1]); err != nil {
		t.Fatalf("第二个恢复码应有效: %v", err)
	}

	remaining = mgr.GetRemainingRecoveryCodes(userID)
	if remaining != 6 {
		t.Errorf("期望剩余 6 个恢复码，实际 %d 个", remaining)
	}
}

// TestUseRecoveryCodeInvalid 测试无效恢复码
func TestUseRecoveryCodeInvalid(t *testing.T) {
	mgr := NewTOTPManager(DefaultTOTPConfig())
	ctx := context.Background()

	userID := "user-001"

	codes, _ := mgr.GenerateRecoveryCodes()
	mgr.StoreRecoveryCodes(ctx, userID, codes)

	// 使用无效恢复码
	if err := mgr.UseRecoveryCode(ctx, userID, "INVALID1"); err == nil {
		t.Error("无效恢复码应返回错误")
	}
}

// TestUseRecoveryCodeEmptyParams 测试空参数
func TestUseRecoveryCodeEmptyParams(t *testing.T) {
	mgr := NewTOTPManager(DefaultTOTPConfig())
	ctx := context.Background()

	if err := mgr.UseRecoveryCode(ctx, "", "CODE1234"); err == nil {
		t.Error("空 userID 应返回错误")
	}

	if err := mgr.UseRecoveryCode(ctx, "user-001", ""); err == nil {
		t.Error("空恢复码应返回错误")
	}
}

// TestUseRecoveryCodeNoCodesStored 测试未存储恢复码时使用
func TestUseRecoveryCodeNoCodesStored(t *testing.T) {
	mgr := NewTOTPManager(DefaultTOTPConfig())
	ctx := context.Background()

	if err := mgr.UseRecoveryCode(ctx, "no-such-user", "CODE1234"); err == nil {
		t.Error("未存储恢复码的用户应返回错误")
	}
}

// TestDefaultTOTPConfig 测试默认 TOTP 配置
func TestDefaultTOTPConfig(t *testing.T) {
	cfg := DefaultTOTPConfig()

	if cfg.Digits != 6 {
		t.Errorf("默认位数应为 6，实际 %d", cfg.Digits)
	}
	if cfg.Period != 30 {
		t.Errorf("默认周期应为 30 秒，实际 %d", cfg.Period)
	}
	if cfg.Skew != 1 {
		t.Errorf("默认偏移应为 1，实际 %d", cfg.Skew)
	}
	if cfg.SecretLen != 20 {
		t.Errorf("默认密钥长度应为 20，实际 %d", cfg.SecretLen)
	}
}

// TestHashCode 测试恢复码哈希的一致性
func TestHashCode(t *testing.T) {
	code := "TESTCODE"
	hash1 := hashCode(code)
	hash2 := hashCode(code)

	if hash1 != hash2 {
		t.Error("相同输入的哈希值应一致")
	}

	hash3 := hashCode("DIFFERENT")
	if hash1 == hash3 {
		t.Error("不同输入的哈希值应不同")
	}

	// 哈希长度应为 64（SHA-256 十六进制编码）
	if len(hash1) != 64 {
		t.Errorf("SHA-256 哈希长度应为 64，实际 %d", len(hash1))
	}
}

// TestGenerateRandomCode 测试随机码生成
func TestGenerateRandomCode(t *testing.T) {
	code, err := generateRandomCode(8)
	if err != nil {
		t.Fatalf("generateRandomCode 失败: %v", err)
	}

	if len(code) != 8 {
		t.Errorf("期望长度 8，实际 %d", len(code))
	}

	// 验证字符集（大写字母和数字）
	for _, c := range code {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			t.Errorf("恢复码包含非法字符: %c", c)
		}
	}
}

// generateTestCode 生成当前时间窗口的有效 TOTP 验证码（仅用于测试）
func generateTestCode(t *testing.T, secret string, config TOTPConfig) string {
	t.Helper()

	secretBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		t.Fatalf("解码密钥失败: %v", err)
	}

	counter := time.Now().Unix() / int64(config.Period)
	return generateTOTPCode(secretBytes, counter, config.Digits)
}
