package biz

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestIsHTML 验证 isHTML 函数对 HTML 内容的检测能力。
// 测试用例覆盖：标准 HTML 标签、自闭合标签、纯文本、空字符串等场景。
func TestIsHTML(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"<h1>Hello</h1>", true},
		{"<p>Paragraph</p>", true},
		{"<br/>", true},
		{"plain text message", false},
		{"no tags here", false},
		{"alert > threshold", false},
		{"a < b and c > d", true}, // contains < ... >, detected as HTML
		{"", false},
	}
	for _, tt := range tests {
		got := isHTML(tt.input)
		if got != tt.want {
			t.Errorf("isHTML(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestStripHTMLTags 验证 stripHTMLTags 函数能正确移除 HTML 标签并保留文本内容。
func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<h1>Title</h1>", "Title"},
		{"<p>Hello <b>world</b></p>", "Hello world"},
		{"no tags", "no tags"},
		{"<br/>text", "text"},
		{"<a href='url'>link</a>", "link"},
	}
	for _, tt := range tests {
		got := stripHTMLTags(tt.input)
		if got != tt.want {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestSMSConfigParsing 验证 SMSConfig 的 JSON 反序列化，确保所有字段（含自定义 API 端点）正确解析。
func TestSMSConfigParsing(t *testing.T) {
	cfgJSON := `{
		"provider": "aliyun",
		"access_key": "LTAI5xxx",
		"secret": "mysecret",
		"sign_name": "OpsNexus",
		"template_code": "SMS_12345",
		"api_endpoint": "https://custom-sms.example.com/send"
	}`

	var cfg SMSConfig
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "aliyun" {
		t.Errorf("provider: want aliyun, got %s", cfg.Provider)
	}
	if cfg.APIEndpoint != "https://custom-sms.example.com/send" {
		t.Errorf("api_endpoint: want custom endpoint, got %s", cfg.APIEndpoint)
	}
	if cfg.SignName != "OpsNexus" {
		t.Errorf("sign_name: want OpsNexus, got %s", cfg.SignName)
	}
}

// TestVoiceConfigParsing 验证 VoiceConfig 的 JSON 反序列化，确保所有字段（含主叫号码和自定义端点）正确解析。
func TestVoiceConfigParsing(t *testing.T) {
	cfgJSON := `{
		"provider": "tencent",
		"access_key": "AKID123",
		"secret": "voicesecret",
		"template_code": "TTS_001",
		"caller_id": "+8610000000",
		"api_endpoint": "https://custom-voice.example.com/call"
	}`

	var cfg VoiceConfig
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "tencent" {
		t.Errorf("provider: want tencent, got %s", cfg.Provider)
	}
	if cfg.CallerID != "+8610000000" {
		t.Errorf("caller_id: want +8610000000, got %s", cfg.CallerID)
	}
	if cfg.APIEndpoint != "https://custom-voice.example.com/call" {
		t.Errorf("api_endpoint: want custom endpoint, got %s", cfg.APIEndpoint)
	}
}

// TestFeishuConfigParsing 验证 FeishuConfig 的 JSON 反序列化，确保 Webhook URL 正确解析。
func TestFeishuConfigParsing(t *testing.T) {
	cfgJSON := `{"webhook_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxx-yyy-zzz"}`

	var cfg FeishuConfig
	err := json.Unmarshal([]byte(cfgJSON), &cfg)
	require.NoError(t, err, "FeishuConfig JSON 应能正确反序列化")
	assert.Equal(t, "https://open.feishu.cn/open-apis/bot/v2/hook/xxx-yyy-zzz", cfg.WebhookURL)
}

// TestDingtalkConfigParsing 验证 DingtalkConfig 的 JSON 反序列化，确保所有字段（含可选的签名密钥和详情链接）正确解析。
func TestDingtalkConfigParsing(t *testing.T) {
	cfgJSON := `{
		"webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=abc123",
		"secret": "SEC1234567890abcdef",
		"detail_url": "https://grafana.internal/alert/456"
	}`

	var cfg DingtalkConfig
	err := json.Unmarshal([]byte(cfgJSON), &cfg)
	require.NoError(t, err, "DingtalkConfig JSON 应能正确反序列化")
	assert.Equal(t, "https://oapi.dingtalk.com/robot/send?access_token=abc123", cfg.WebhookURL)
	assert.Equal(t, "SEC1234567890abcdef", cfg.Secret)
	assert.Equal(t, "https://grafana.internal/alert/456", cfg.DetailURL)
}

// TestDingtalkConfigParsing_NoSecret 验证 DingtalkConfig 不配置签名密钥时的反序列化。
func TestDingtalkConfigParsing_NoSecret(t *testing.T) {
	cfgJSON := `{"webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=abc123"}`

	var cfg DingtalkConfig
	err := json.Unmarshal([]byte(cfgJSON), &cfg)
	require.NoError(t, err)
	assert.Empty(t, cfg.Secret, "未配置签名密钥时 Secret 应为空")
	assert.Empty(t, cfg.DetailURL, "未配置详情链接时 DetailURL 应为空")
}

// TestResolveProviderSMSEndpoint 验证短信服务商 API 端点解析，覆盖阿里云、腾讯云和未知服务商的默认端点。
func TestResolveProviderSMSEndpoint(t *testing.T) {
	tests := []struct {
		provider string
		contains string
	}{
		{"aliyun", "aliyuncs.com"},
		{"tencent", "tencentcloudapi.com"},
		{"unknown", "example.com"},
	}
	for _, tt := range tests {
		got := resolveProviderSMSEndpoint(tt.provider)
		if !containsString(got, tt.contains) {
			t.Errorf("resolveProviderSMSEndpoint(%q) = %q, want to contain %q", tt.provider, got, tt.contains)
		}
	}
}

// TestResolveProviderVoiceEndpoint 验证语音服务商 API 端点解析，覆盖阿里云、腾讯云和未知服务商的默认端点。
func TestResolveProviderVoiceEndpoint(t *testing.T) {
	tests := []struct {
		provider string
		contains string
	}{
		{"aliyun", "aliyuncs.com"},
		{"tencent", "tencentcloudapi.com"},
		{"unknown", "example.com"},
	}
	for _, tt := range tests {
		got := resolveProviderVoiceEndpoint(tt.provider)
		if !containsString(got, tt.contains) {
			t.Errorf("resolveProviderVoiceEndpoint(%q) = %q, want to contain %q", tt.provider, got, tt.contains)
		}
	}
}

// containsString 检查字符串 s 是否包含子串 substr，用于测试断言。
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ===========================================
// SMS 短信发送器 httptest 集成测试
// 验证 SMSSender 通过 httptest mock 服务商 API 的完整发送流程。
// ===========================================

// TestSMSSender_HTTPTest_Success 验证 SMS 正常发送路径：
// 模拟短信服务商 API 返回 200，验证请求体结构、HMAC 签名头和模板参数。
func TestSMSSender_HTTPTest_Success(t *testing.T) {
	// 记录 mock 服务器收到的请求数量和最后一次请求体
	requestCount := 0
	var lastPayload map[string]interface{}

	// 启动 mock 短信服务商 HTTP 端点
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// 校验 HTTP 方法和内容类型
		assert.Equal(t, http.MethodPost, r.Method, "SMS API 应使用 POST 方法")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// 校验 HMAC-SHA256 认证三元组
		assert.NotEmpty(t, r.Header.Get("X-Access-Key"), "应携带 X-Access-Key 认证头")
		assert.NotEmpty(t, r.Header.Get("X-Timestamp"), "应携带 X-Timestamp 时间戳头")
		assert.NotEmpty(t, r.Header.Get("X-Signature"), "应携带 X-Signature 签名头")

		// 解析请求体
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &lastPayload)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":"OK"}`))
	}))
	defer server.Close()

	// 构造 SMSConfig，API 端点指向 mock 服务器
	cfg := SMSConfig{
		Provider:    "aliyun",
		AccessKey:   "test-access-key",
		Secret:      "test-hmac-secret",
		SignName:    "OpsNexus",
		Template:    "SMS_99999",
		APIEndpoint: server.URL,
	}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &SMSSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{
			Body:      "磁盘使用率超过 90%",
			Variables: map[string]string{"host": "db-01", "disk": "/data"},
		},
		[]string{"13800001111", "13900002222"},
	)

	// 断言：发送成功，且每个手机号产生一次 HTTP 调用
	require.NoError(t, err, "SMS 发送应成功")
	assert.Equal(t, 2, requestCount, "应为每个手机号各发送一次请求")

	// 断言：最后一次请求的 payload 结构正确
	assert.Equal(t, "13900002222", lastPayload["phone_number"], "最后一次请求应发送到第二个手机号")
	assert.Equal(t, "OpsNexus", lastPayload["sign_name"])
	assert.Equal(t, "SMS_99999", lastPayload["template_code"])

	// 断言：模板参数包含自定义变量和自动填充的 content
	params, ok := lastPayload["template_params"].(map[string]interface{})
	require.True(t, ok, "template_params 应为 map 类型")
	assert.Equal(t, "db-01", params["host"])
	assert.Equal(t, "/data", params["disk"])
	assert.Contains(t, params, "content", "应自动填充 content 参数")
}

// TestSMSSender_HTTPTest_Failure 验证 SMS 服务商返回非 200 状态码时 sender 正确传播错误。
func TestSMSSender_HTTPTest_Failure(t *testing.T) {
	// mock 服务器返回 429 限流错误
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"code":"Throttling","message":"rate limit exceeded"}`))
	}))
	defer server.Close()

	cfg := SMSConfig{
		Provider:    "aliyun",
		AccessKey:   "key",
		Secret:      "secret",
		SignName:    "Test",
		Template:    "SMS_001",
		APIEndpoint: server.URL,
	}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &SMSSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "test alert"}, []string{"13800138000"})

	// 断言：应返回包含 HTTP 状态码的错误
	require.Error(t, err, "服务商返回 429 时应报错")
	assert.Contains(t, err.Error(), "429", "错误信息应包含 HTTP 状态码")
}

// ===========================================
// Voice TTS 语音发送器 httptest 集成测试
// 验证 VoiceSender 通过 httptest mock 语音服务商 API 的完整发送流程。
// ===========================================

// TestVoiceSender_HTTPTest_Success 验证语音 TTS 正常外呼路径：
// 模拟语音服务商 API 返回 200，验证 payload 中的被叫号码、主叫号码和 TTS 文本。
func TestVoiceSender_HTTPTest_Success(t *testing.T) {
	var receivedPayloads []map[string]interface{}

	// 启动 mock 语音服务商 HTTP 端点
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 校验 HMAC 签名头存在（语音 API 与 SMS 使用相同认证机制）
		assert.NotEmpty(t, r.Header.Get("X-Signature"), "应携带 HMAC 签名头")

		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)
		receivedPayloads = append(receivedPayloads, payload)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"call_id":"call-001"}`))
	}))
	defer server.Close()

	cfg := VoiceConfig{
		Provider:    "tencent",
		AccessKey:   "AKID-voice-test",
		Secret:      "voice-hmac-secret",
		Template:    "TTS_ALERT_001",
		CallerID:    "+861000000000",
		APIEndpoint: server.URL,
	}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &VoiceSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{
			Body:      "紧急告警：数据库主从延迟超过 10 秒",
			Variables: map[string]string{"db": "mysql-primary", "delay": "12s"},
		},
		[]string{"13800001111"},
	)

	require.NoError(t, err, "语音外呼应成功")
	require.Len(t, receivedPayloads, 1, "应发送 1 次呼叫请求")

	// 验证 payload 关键字段
	p := receivedPayloads[0]
	assert.Equal(t, "13800001111", p["called_number"], "被叫号码应正确")
	assert.Equal(t, "+861000000000", p["caller_id"], "主叫号码应正确")
	assert.Equal(t, "TTS_ALERT_001", p["template_code"], "模板编号应正确")
	assert.Equal(t, "紧急告警：数据库主从延迟超过 10 秒", p["tts_content"], "TTS 播报文本应为消息正文")
}

// TestVoiceSender_HTTPTest_Failure 验证语音服务商返回错误时 sender 正确传播错误。
func TestVoiceSender_HTTPTest_Failure(t *testing.T) {
	// mock 服务器返回 502 网关错误
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"upstream timeout"}`))
	}))
	defer server.Close()

	cfg := VoiceConfig{
		Provider:    "aliyun",
		AccessKey:   "key",
		Secret:      "secret",
		Template:    "TTS_001",
		CallerID:    "+8610000",
		APIEndpoint: server.URL,
	}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &VoiceSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "测试告警"}, []string{"13900001111"})

	// 断言：应返回包含 HTTP 状态码的错误
	require.Error(t, err, "服务商返回 502 时应报错")
	assert.Contains(t, err.Error(), "502", "错误信息应包含 HTTP 状态码")
}

// ===========================================
// Email 邮件发送器测试
// 注意：EmailSender 使用 SMTP（gomail），无法用 httptest 模拟。
// 以下测试覆盖邮件相关的配置解析和内容处理逻辑。
// ===========================================

// TestEmailConfig_Parsing 验证 EmailConfig 的 JSON 反序列化，确保 SMTP 相关字段正确解析。
func TestEmailConfig_Parsing(t *testing.T) {
	cfgJSON := `{
		"smtp_host": "smtp.example.com",
		"smtp_port": 465,
		"username": "alerts@example.com",
		"password": "smtp-password",
		"from": "OpsNexus <alerts@example.com>",
		"use_tls": true
	}`

	var cfg EmailConfig
	err := json.Unmarshal([]byte(cfgJSON), &cfg)
	require.NoError(t, err, "EmailConfig JSON 应能正确反序列化")

	assert.Equal(t, "smtp.example.com", cfg.SMTPHost, "SMTP 主机应正确")
	assert.Equal(t, 465, cfg.SMTPPort, "SMTP 端口应正确")
	assert.Equal(t, "alerts@example.com", cfg.Username, "SMTP 用户名应正确")
	assert.Equal(t, "OpsNexus <alerts@example.com>", cfg.From, "发件人应正确")
	assert.True(t, cfg.UseTLS, "应启用 TLS")
}

// TestEmailSender_InvalidConfig 验证 EmailSender 在配置 JSON 无效时返回解析错误。
func TestEmailSender_InvalidConfig(t *testing.T) {
	sender := &EmailSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(),
		&Bot{Config: []byte("{invalid json}")},
		MessageContent{Subject: "Test", Body: "test"},
		[]string{"user@example.com"},
	)

	require.Error(t, err, "无效 JSON 配置应返回错误")
	assert.Contains(t, err.Error(), "parse email config", "错误应包含配置解析提示")
}

// TestEmailSender_HTMLBodyDetection 验证邮件发送器的 HTML 内容检测和纯文本转换。
// isHTML 和 stripHTMLTags 是 EmailSender 内部使用的辅助函数，
// 此测试通过更多场景验证其在邮件上下文中的正确性。
func TestEmailSender_HTMLBodyDetection(t *testing.T) {
	tests := []struct {
		name     string   // 测试场景名称
		body     string   // 输入的邮件正文
		wantHTML bool     // 是否应被检测为 HTML
		wantText string   // 剥离标签后的预期纯文本
	}{
		{
			name:     "标准告警 HTML 模板",
			body:     "<h2>P0 告警</h2><p>MySQL 主库 CPU 100%</p>",
			wantHTML: true,
			wantText: "P0 告警MySQL 主库 CPU 100%",
		},
		{
			name:     "纯文本告警内容",
			body:     "告警：服务器 app-01 磁盘使用率 95%",
			wantHTML: false,
			wantText: "告警：服务器 app-01 磁盘使用率 95%",
		},
		{
			name:     "包含链接的 HTML 内容",
			body:     `<a href="https://grafana.internal">查看面板</a>`,
			wantHTML: true,
			wantText: "查看面板",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHTML := isHTML(tt.body)
			assert.Equal(t, tt.wantHTML, gotHTML, "HTML 检测结果应正确")

			gotText := stripHTMLTags(tt.body)
			assert.Equal(t, tt.wantText, gotText, "标签剥离结果应正确")
		})
	}
}
