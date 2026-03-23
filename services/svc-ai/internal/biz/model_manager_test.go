package biz

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/opsnexus/svc-ai/internal/config"
	"go.uber.org/zap"
)

// newTestModelManager 创建用于测试的 ModelManager 实例，使用开发模式日志。
func newTestModelManager() *ModelManager {
	logger, _ := zap.NewDevelopment()
	return NewModelManager(nil, nil, config.AIConfig{DefaultTimeoutSeconds: 30}, logger)
}

// TestDecryptAPIKey_Plaintext_WhenNoEnvKey 验证未设置加密密钥环境变量时，API 密钥按明文原样返回。
func TestDecryptAPIKey_Plaintext_WhenNoEnvKey(t *testing.T) {
	os.Unsetenv("AI_KEY_ENCRYPTION_KEY")
	mm := newTestModelManager()

	key := mm.decryptAPIKey([]byte("sk-test-plaintext-key"))
	if key != "sk-test-plaintext-key" {
		t.Errorf("expected plaintext key, got %q", key)
	}
}

// TestDecryptAPIKey_Empty 验证空输入（nil 和空切片）时返回空字符串。
func TestDecryptAPIKey_Empty(t *testing.T) {
	mm := newTestModelManager()
	key := mm.decryptAPIKey(nil)
	if key != "" {
		t.Errorf("expected empty string for nil input, got %q", key)
	}

	key = mm.decryptAPIKey([]byte{})
	if key != "" {
		t.Errorf("expected empty string for empty input, got %q", key)
	}
}

// TestDecryptAPIKey_AES256GCM_RoundTrip 验证 AES-256-GCM 加解密的完整往返流程。
// 先加密一个测试密钥，再用 decryptAPIKey 解密，确认结果一致。
func TestDecryptAPIKey_AES256GCM_RoundTrip(t *testing.T) {
	// 生成随机 32 字节加密密钥
	rawKey := make([]byte, 32)
	if _, err := rand.Read(rawKey); err != nil {
		t.Fatal(err)
	}
	keyHex := hex.EncodeToString(rawKey)

	// 设置加密密钥环境变量
	os.Setenv("AI_KEY_ENCRYPTION_KEY", keyHex)
	defer os.Unsetenv("AI_KEY_ENCRYPTION_KEY")

	// 使用 AES-256-GCM 加密测试 API 密钥
	plaintext := "sk-ant-api03-test-key-12345"
	block, err := aes.NewCipher(rawKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	encryptedHex := hex.EncodeToString(ciphertext)

	// 使用 ModelManager 解密并验证结果
	mm := newTestModelManager()
	decrypted := mm.decryptAPIKey([]byte(encryptedHex))
	if decrypted != plaintext {
		t.Errorf("decrypted key mismatch: want %q, got %q", plaintext, decrypted)
	}
}

// TestDecryptAPIKey_InvalidKey 验证加密密钥格式非法时返回空字符串（安全降级）。
func TestDecryptAPIKey_InvalidKey(t *testing.T) {
	os.Setenv("AI_KEY_ENCRYPTION_KEY", "not-valid-hex")
	defer os.Unsetenv("AI_KEY_ENCRYPTION_KEY")

	mm := newTestModelManager()
	key := mm.decryptAPIKey([]byte("some-encrypted-data"))
	if key != "" {
		t.Errorf("expected empty string for invalid encryption key, got %q", key)
	}
}

// TestDecryptAPIKey_NonHexCiphertext_Fallback 验证密文非 hex 格式时回退到明文处理（兼容迁移场景）。
func TestDecryptAPIKey_NonHexCiphertext_Fallback(t *testing.T) {
	// 设置有效的加密密钥，但提供非 hex 格式的密文（模拟迁移兼容场景）
	rawKey := make([]byte, 32)
	if _, err := rand.Read(rawKey); err != nil {
		t.Fatal(err)
	}
	os.Setenv("AI_KEY_ENCRYPTION_KEY", hex.EncodeToString(rawKey))
	defer os.Unsetenv("AI_KEY_ENCRYPTION_KEY")

	mm := newTestModelManager()
	// 非 hex 字符串应回退为明文返回
	key := mm.decryptAPIKey([]byte("plaintext-api-key-not-hex!@#$"))
	if key != "plaintext-api-key-not-hex!@#$" {
		t.Errorf("expected plaintext fallback, got %q", key)
	}
}
