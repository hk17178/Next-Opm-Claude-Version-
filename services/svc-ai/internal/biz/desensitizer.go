package biz

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/opsnexus/svc-ai/internal/config"
	"gopkg.in/yaml.v3"
)

// Desensitizer 在数据发送给 AI 模型之前执行多层脱敏处理。
//
// 脱敏规则包含两层：
//   - 第一层：阻止字段（blockedFields）—— 完全移除指定字段的值（如 password、api_key、secret 等），
//     这些字段包含高敏感信息（密码、密钥），不应以任何形式出现在 AI 输入中
//   - 第二层：正则模式（patterns）—— 通过可配置的正则表达式匹配并替换 PII 信息，
//     如 IP 地址、邮箱、手机号、身份证号等，替换为占位符（如 [IP:***]）
//
// 所有脱敏操作都会记录 SHA-256 哈希到原始值的映射，支持分析完成后的反向追溯。
// 脱敏规则从外部 YAML 文件加载，支持运行时更新而无需重启服务。
type Desensitizer struct {
	patterns      []sensitivePattern
	blockedFields map[string]bool
	enabled       bool
	mu            sync.RWMutex
	mappings      map[string]string // hash -> original (per-request, not persisted)
}

// sensitivePattern 定义一条正则脱敏规则，从 YAML 配置文件加载。
type sensitivePattern struct {
	Name        string `yaml:"name"`
	Regex       string `yaml:"regex"`
	Replacement string `yaml:"replacement"`
	compiled    *regexp.Regexp
}

// patternsFile 是脱敏规则 YAML 文件的结构映射。
type patternsFile struct {
	Patterns      []sensitivePattern `yaml:"patterns"`
	BlockedFields []string           `yaml:"blocked_fields"`
}

// NewDesensitizer 根据配置创建脱敏器实例。
// 如果 cfg.Enabled 为 false，返回的脱敏器将直接透传所有输入不做处理。
func NewDesensitizer(cfg config.DesensitizeConfig) (*Desensitizer, error) {
	d := &Desensitizer{
		enabled:       cfg.Enabled,
		blockedFields: make(map[string]bool),
		mappings:      make(map[string]string),
	}

	if !cfg.Enabled {
		return d, nil
	}

	data, err := os.ReadFile(cfg.PatternsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read patterns file: %w", err)
	}

	var pf patternsFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("failed to parse patterns file: %w", err)
	}

	for i := range pf.Patterns {
		compiled, err := regexp.Compile(pf.Patterns[i].Regex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern %q: %w", pf.Patterns[i].Name, err)
		}
		pf.Patterns[i].compiled = compiled
	}
	d.patterns = pf.Patterns

	for _, f := range pf.BlockedFields {
		d.blockedFields[strings.ToLower(f)] = true
	}

	return d, nil
}

// Sanitize 对输入文本应用所有脱敏规则。
// 先执行阻止字段移除（第一层），再执行正则模式替换（第二层）。
// 返回脱敏后的文本和哈希到原始值的映射表。
func (d *Desensitizer) Sanitize(input string) (string, map[string]string) {
	if !d.enabled || input == "" {
		return input, nil
	}

	mapping := make(map[string]string)
	result := input

	// Layer 1: Remove blocked field values (key-value pairs where key is blocked)
	for field := range d.blockedFields {
		pattern := regexp.MustCompile(fmt.Sprintf(`(?i)(%s)\s*[:=]\s*\S+`, regexp.QuoteMeta(field)))
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			hash := hashValue(match)
			mapping[hash] = match
			return fmt.Sprintf("[BLOCKED:%s]", field)
		})
	}

	// Layer 2: Apply regex patterns in order
	for _, p := range d.patterns {
		result = p.compiled.ReplaceAllStringFunc(result, func(match string) string {
			hash := hashValue(match)
			mapping[hash] = match
			return p.compiled.ReplaceAllString(match, p.Replacement)
		})
	}

	return result, mapping
}

// SanitizeMap 对 map 中所有字符串值执行脱敏，阻止字段的键值对会被整体移除。
// 递归处理嵌套的 map 结构。
func (d *Desensitizer) SanitizeMap(input map[string]interface{}) (map[string]interface{}, map[string]string) {
	if !d.enabled {
		return input, nil
	}

	allMappings := make(map[string]string)
	result := make(map[string]interface{}, len(input))

	for k, v := range input {
		if d.blockedFields[strings.ToLower(k)] {
			continue
		}

		switch val := v.(type) {
		case string:
			sanitized, m := d.Sanitize(val)
			result[k] = sanitized
			for mk, mv := range m {
				allMappings[mk] = mv
			}
		case map[string]interface{}:
			sanitized, m := d.SanitizeMap(val)
			result[k] = sanitized
			for mk, mv := range m {
				allMappings[mk] = mv
			}
		default:
			result[k] = v
		}
	}

	return result, allMappings
}

// ContentHash 计算输入的 SHA-256 哈希值，用于 ai_call_logs 表的 input_hash 字段去重。
func ContentHash(input string) string {
	return hashValue(input)
}

// hashValue 计算 SHA-256 哈希并返回前 8 字节的十六进制编码。
func hashValue(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:8])
}
