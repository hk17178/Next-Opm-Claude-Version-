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

// ===========================================
// SMS 短信发送器 httptest 测试
// 使用 httptest.NewServer 模拟短信服务商的 HTTP API，
// 验证 SMSSender 的完整发送路径：配置解析 → 请求构建 → HMAC 签名 → 响应处理。
// ===========================================

// TestSMSSender_SendSuccess 验证 SMS 发送成功路径：
// 构造 httptest 服务器模拟短信服务商 API，验证请求体、认证头和响应处理。
func TestSMSSender_SendSuccess(t *testing.T) {
	// 记录收到的请求用于断言
	var receivedBodies []map[string]interface{}
	var receivedHeaders []http.Header

	// 启动 mock 短信服务商 API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法和内容类型
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// 验证 HMAC 认证头存在
		assert.NotEmpty(t, r.Header.Get("X-Access-Key"), "X-Access-Key 头应存在")
		assert.NotEmpty(t, r.Header.Get("X-Timestamp"), "X-Timestamp 头应存在")
		assert.NotEmpty(t, r.Header.Get("X-Signature"), "X-Signature 头应存在")

		// 记录请求体
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)
		receivedBodies = append(receivedBodies, payload)
		receivedHeaders = append(receivedHeaders, r.Header.Clone())

		// 返回 200 成功
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code": "OK"}`))
	}))
	defer server.Close()

	// 构造 SMS 配置，API 端点指向 mock 服务器
	cfg := SMSConfig{
		Provider:    "aliyun",
		AccessKey:   "LTAI5test",
		Secret:      "test-secret-key",
		SignName:    "OpsNexus",
		Template:    "SMS_12345",
		APIEndpoint: server.URL,
	}
	cfgJSON, _ := json.Marshal(cfg)

	bot := &Bot{Config: cfgJSON}
	msg := MessageContent{
		Body:      "服务器 CPU 使用率超过 95%",
		Variables: map[string]string{"host": "app-01", "value": "95%"},
	}
	recipients := []string{"13800138000", "13900139000"}

	sender := &SMSSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), bot, msg, recipients)

	require.NoError(t, err)

	// 验证：每个手机号应发送一次请求（共 2 次）
	assert.Len(t, receivedBodies, 2, "应为每个手机号发送一次请求")

	// 验证第一个请求的 payload 内容
	assert.Equal(t, "13800138000", receivedBodies[0]["phone_number"])
	assert.Equal(t, "OpsNexus", receivedBodies[0]["sign_name"])
	assert.Equal(t, "SMS_12345", receivedBodies[0]["template_code"])

	// 验证模板参数包含 content（自动从 msg.Body 填充）和自定义变量
	params, ok := receivedBodies[0]["template_params"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "app-01", params["host"])
	assert.Equal(t, "95%", params["value"])
	assert.Contains(t, params, "content", "应自动填充 content 参数")

	// 验证第二个请求发送到第二个手机号
	assert.Equal(t, "13900139000", receivedBodies[1]["phone_number"])
}

// TestSMSSender_SendAPIError 验证 SMS 服务商返回 HTTP 错误时 sender 正确返回错误。
func TestSMSSender_SendAPIError(t *testing.T) {
	// mock 服务器返回 400 错误
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"code": "InvalidParameter", "message": "invalid phone number"}`))
	}))
	defer server.Close()

	cfg := SMSConfig{
		Provider:    "aliyun",
		AccessKey:   "test",
		Secret:      "secret",
		SignName:    "Test",
		Template:    "SMS_001",
		APIEndpoint: server.URL,
	}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &SMSSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "test"}, []string{"invalid-phone"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

// TestSMSSender_NoRecipients 验证接收人列表为空时返回错误。
func TestSMSSender_NoRecipients(t *testing.T) {
	cfg := SMSConfig{Provider: "aliyun", AccessKey: "k", Secret: "s", SignName: "T", Template: "T1"}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &SMSSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "test"}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no recipients")
}

// TestSMSSender_InvalidConfig 验证配置 JSON 解析失败时返回错误。
func TestSMSSender_InvalidConfig(t *testing.T) {
	sender := &SMSSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: []byte("invalid json")},
		MessageContent{Body: "test"}, []string{"13800138000"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse sms config")
}

// TestSMSSender_TestConnectivity 验证 SMS Test 方法向 API 端点发送 GET 请求检测连通性。
func TestSMSSender_TestConnectivity(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := SMSConfig{Provider: "aliyun", AccessKey: "test-key", Secret: "s", APIEndpoint: server.URL}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &SMSSender{logger: zap.NewNop()}
	err := sender.Test(context.Background(), &Bot{Config: cfgJSON})

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, receivedMethod, "Test 应发送 GET 请求")
}

// ===========================================
// Voice TTS 语音发送器 httptest 测试
// 使用 httptest.NewServer 模拟语音服务商 API，
// 验证 VoiceSender 的完整发送路径。
// ===========================================

// TestVoiceSender_SendSuccess 验证语音外呼发送成功路径。
func TestVoiceSender_SendSuccess(t *testing.T) {
	var receivedBodies []map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.NotEmpty(t, r.Header.Get("X-Signature"), "HMAC 签名头应存在")

		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)
		receivedBodies = append(receivedBodies, payload)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := VoiceConfig{
		Provider:    "tencent",
		AccessKey:   "AKID-test",
		Secret:      "voice-secret",
		Template:    "TTS_001",
		CallerID:    "+8610000000",
		APIEndpoint: server.URL,
	}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &VoiceSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "紧急告警：数据库主从切换", Variables: map[string]string{"db": "mysql-master"}},
		[]string{"13800138000"})

	require.NoError(t, err)
	assert.Len(t, receivedBodies, 1)

	// 验证 payload 字段
	assert.Equal(t, "13800138000", receivedBodies[0]["called_number"])
	assert.Equal(t, "+8610000000", receivedBodies[0]["caller_id"])
	assert.Equal(t, "TTS_001", receivedBodies[0]["template_code"])
	assert.Equal(t, "紧急告警：数据库主从切换", receivedBodies[0]["tts_content"])
}

// TestVoiceSender_SendAPIError 验证语音服务商返回错误时 sender 正确传播错误。
func TestVoiceSender_SendAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`service unavailable`))
	}))
	defer server.Close()

	cfg := VoiceConfig{Provider: "aliyun", AccessKey: "k", Secret: "s", Template: "T", CallerID: "C", APIEndpoint: server.URL}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &VoiceSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "test"}, []string{"13800138000"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

// TestVoiceSender_NoRecipients 验证被叫号码列表为空时返回错误。
func TestVoiceSender_NoRecipients(t *testing.T) {
	cfg := VoiceConfig{Provider: "aliyun", AccessKey: "k", Secret: "s"}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &VoiceSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "test"}, []string{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no recipients")
}

// TestVoiceSender_MultipleRecipients 验证多个被叫号码逐一发起呼叫。
func TestVoiceSender_MultipleRecipients(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := VoiceConfig{Provider: "aliyun", AccessKey: "k", Secret: "s", Template: "T", CallerID: "C", APIEndpoint: server.URL}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &VoiceSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "alert"}, []string{"13800138000", "13900139000", "15000150000"})

	require.NoError(t, err)
	assert.Equal(t, 3, callCount, "应为 3 个被叫号码分别发起呼叫")
}

// ===========================================
// GenericWebhookSender 通用 Webhook 发送器 httptest 测试
// 验证自定义 URL 的 JSON POST 请求、自定义 headers 和 HMAC 签名。
// ===========================================

// TestWebhookSender_SendSuccess 验证 Webhook 发送成功路径。
func TestWebhookSender_SendSuccess(t *testing.T) {
	var receivedBody map[string]interface{}
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := WebhookConfig{
		URL:     server.URL,
		Method:  "POST",
		Headers: map[string]string{"X-Custom": "custom-value"},
		Secret:  "webhook-hmac-secret",
	}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &GenericWebhookSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{
			Subject:   "[P0] 数据库告警",
			Body:      "MySQL 主库连接数超限",
			Variables: map[string]string{"db": "mysql-master", "connections": "500"},
		},
		[]string{"oncall@example.com"})

	require.NoError(t, err)

	// 验证 payload 字段
	assert.Equal(t, "[P0] 数据库告警", receivedBody["subject"])
	assert.Equal(t, "MySQL 主库连接数超限", receivedBody["body"])
	assert.NotEmpty(t, receivedBody["timestamp"], "timestamp 字段应存在")

	// 验证自定义 headers
	assert.Equal(t, "custom-value", receivedHeaders.Get("X-Custom"))
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))

	// 验证 HMAC 签名头（因为配置了 Secret）
	assert.NotEmpty(t, receivedHeaders.Get("X-OpsNexus-Signature"), "配置 Secret 后应有 HMAC 签名头")
}

// TestWebhookSender_NoSignatureWithoutSecret 验证未配置 Secret 时不附加 HMAC 签名头。
func TestWebhookSender_NoSignatureWithoutSecret(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := WebhookConfig{URL: server.URL, Method: "POST"}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &GenericWebhookSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "test"}, nil)

	require.NoError(t, err)
	assert.Empty(t, receivedHeaders.Get("X-OpsNexus-Signature"), "未配置 Secret 时不应有签名头")
}

// TestWebhookSender_DefaultMethod 验证未指定 Method 时默认使用 POST。
func TestWebhookSender_DefaultMethod(t *testing.T) {
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := WebhookConfig{URL: server.URL} // Method 为空
	cfgJSON, _ := json.Marshal(cfg)

	sender := &GenericWebhookSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "test"}, nil)

	require.NoError(t, err)
	assert.Equal(t, "POST", receivedMethod, "默认 HTTP 方法应为 POST")
}

// TestWebhookSender_ServerError 验证 Webhook 目标返回 5xx 时 sender 返回错误。
func TestWebhookSender_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := WebhookConfig{URL: server.URL, Method: "POST"}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &GenericWebhookSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "test"}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// ===========================================
// WecomWebhookSender 企业微信 Webhook 发送器 httptest 测试
// ===========================================

// TestWecomWebhookSender_SendSuccess 验证企业微信 Webhook 发送 Markdown 格式消息。
func TestWecomWebhookSender_SendSuccess(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := WecomWebhookConfig{WebhookURL: server.URL}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &WecomWebhookSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "## 告警通知\n- CPU: 95%\n- Host: app-01"}, nil)

	require.NoError(t, err)

	// 验证 Markdown 消息格式
	assert.Equal(t, "markdown", receivedBody["msgtype"])
	markdown, ok := receivedBody["markdown"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, markdown["content"], "告警通知")
}

// TestWecomWebhookSender_ServerError 验证企业微信 API 返回错误时传播错误。
func TestWecomWebhookSender_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	cfg := WecomWebhookConfig{WebhookURL: server.URL}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &WecomWebhookSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "test"}, nil)

	require.Error(t, err)
}

// ===========================================
// FeishuSender 飞书群机器人 Webhook httptest 测试
// 验证飞书交互式卡片（interactive card）消息格式和颜色模板选择。
// ===========================================

// TestFeishuSender_SendSuccess 验证飞书卡片消息格式正确：
// msg_type 为 interactive，card 包含 header 和 elements，header 中有 title 和 template。
func TestFeishuSender_SendSuccess(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "飞书 Webhook 应使用 POST 方法")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer server.Close()

	cfg := FeishuConfig{WebhookURL: server.URL}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &FeishuSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{
			Subject: "[P0] MySQL 主库宕机",
			Body:    "**主机**: db-master-01\n**状态**: 连接超时\n**持续时间**: 5 分钟",
		}, nil)

	require.NoError(t, err, "飞书消息发送应成功")

	// 验证消息类型为 interactive
	assert.Equal(t, "interactive", receivedBody["msg_type"], "msg_type 应为 interactive")

	// 验证卡片结构
	card, ok := receivedBody["card"].(map[string]interface{})
	require.True(t, ok, "card 字段应存在且为 map 类型")

	// 验证卡片头部
	header, ok := card["header"].(map[string]interface{})
	require.True(t, ok, "header 字段应存在")

	// 验证标题
	title, ok := header["title"].(map[string]interface{})
	require.True(t, ok, "title 字段应存在")
	assert.Equal(t, "plain_text", title["tag"], "标题 tag 应为 plain_text")
	assert.Equal(t, "[P0] MySQL 主库宕机", title["content"], "标题内容应正确")

	// 验证颜色模板：包含 P0 关键词应为 red
	assert.Equal(t, "red", header["template"], "P0 告警卡片颜色应为 red")

	// 验证卡片内容元素
	elements, ok := card["elements"].([]interface{})
	require.True(t, ok, "elements 字段应存在且为数组")
	require.Len(t, elements, 1, "应有 1 个内容元素")

	elem, ok := elements[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "div", elem["tag"], "元素 tag 应为 div")

	text, ok := elem["text"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "lark_md", text["tag"], "文本 tag 应为 lark_md")
	assert.Contains(t, text["content"], "db-master-01", "文本内容应包含主机名")
}

// TestFeishuSender_HeaderTemplate 验证飞书卡片根据不同严重级别选择不同颜色模板。
func TestFeishuSender_HeaderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		subject  string
		body     string
		wantTmpl string
	}{
		{"P0 严重告警", "[P0] 数据库宕机", "紧急", "red"},
		{"critical 关键词", "critical alert", "server down", "red"},
		{"CRITICAL 关键词", "CRITICAL Alert", "server down", "red"},
		{"严重关键词", "告警", "严重故障", "red"},
		{"P1 警告", "[P1] CPU 高负载", "注意", "orange"},
		{"warning 关键词", "warning alert", "disk usage high", "orange"},
		{"WARNING 关键词", "WARNING", "disk usage high", "orange"},
		{"P2 提示", "[P2] 日志清理", "定期任务", "blue"},
		{"info 关键词", "info notice", "scheduled maintenance", "blue"},
		{"INFO 关键词", "INFO", "scheduled maintenance", "blue"},
		{"默认灰色", "通知", "系统正常运行", "grey"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := feishuHeaderTemplate(tt.subject, tt.body)
			assert.Equal(t, tt.wantTmpl, got, "颜色模板应匹配严重级别")
		})
	}
}

// TestFeishuSender_DefaultTitle 验证未设置 Subject 时使用默认标题。
func TestFeishuSender_DefaultTitle(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := FeishuConfig{WebhookURL: server.URL}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &FeishuSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "普通通知消息"}, nil)

	require.NoError(t, err)

	card := receivedBody["card"].(map[string]interface{})
	header := card["header"].(map[string]interface{})
	title := header["title"].(map[string]interface{})
	assert.Equal(t, "OpsNexus 通知", title["content"], "无 Subject 时应使用默认标题")
}

// TestFeishuSender_ServerError 验证飞书 API 返回错误时传播错误。
func TestFeishuSender_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	cfg := FeishuConfig{WebhookURL: server.URL}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &FeishuSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "test"}, nil)

	require.Error(t, err, "飞书 API 返回 403 时应报错")
}

// TestFeishuSender_InvalidConfig 验证飞书配置 JSON 无效时返回解析错误。
func TestFeishuSender_InvalidConfig(t *testing.T) {
	sender := &FeishuSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: []byte("invalid")},
		MessageContent{Body: "test"}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse feishu config")
}

// ===========================================
// DingtalkSender 钉钉群机器人 Webhook httptest 测试
// 验证钉钉 ActionCard 消息格式、HMAC-SHA256 签名计算和 URL 编码。
// ===========================================

// TestDingtalkSender_SendSuccess 验证钉钉 ActionCard 消息格式正确：
// msgtype 为 actionCard，包含 title、text、btnOrientation 和 btns 字段。
func TestDingtalkSender_SendSuccess(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "钉钉 Webhook 应使用 POST 方法")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	cfg := DingtalkConfig{
		WebhookURL: server.URL,
		DetailURL:  "https://grafana.internal/alert/123",
	}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &DingtalkSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{
			Subject: "[P1] Redis 内存告警",
			Body:    "### Redis 内存告警\n- **实例**: redis-cluster-01\n- **内存使用率**: 92%\n- **建议**: 扩容或清理过期 key",
		}, nil)

	require.NoError(t, err, "钉钉消息发送应成功")

	// 验证消息类型为 actionCard
	assert.Equal(t, "actionCard", receivedBody["msgtype"], "msgtype 应为 actionCard")

	// 验证 ActionCard 结构
	ac, ok := receivedBody["actionCard"].(map[string]interface{})
	require.True(t, ok, "actionCard 字段应存在")

	assert.Equal(t, "[P1] Redis 内存告警", ac["title"], "标题应正确")
	assert.Contains(t, ac["text"].(string), "redis-cluster-01", "正文应包含实例名")
	assert.Equal(t, "0", ac["btnOrientation"], "按钮排列方向应为 0（竖向）")

	// 验证按钮
	btns, ok := ac["btns"].([]interface{})
	require.True(t, ok, "btns 字段应存在且为数组")
	require.Len(t, btns, 1, "应有 1 个按钮")

	btn, ok := btns[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "查看详情", btn["title"], "按钮标题应为'查看详情'")
	assert.Equal(t, "https://grafana.internal/alert/123", btn["actionURL"], "按钮链接应为配置的 detail_url")
}

// TestDingtalkSender_WithSignature 验证启用签名后 Webhook URL 包含 timestamp 和 sign 参数。
func TestDingtalkSender_WithSignature(t *testing.T) {
	var receivedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DingtalkConfig{
		WebhookURL: server.URL + "?access_token=test123",
		Secret:     "SEC1234567890abcdef",
	}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &DingtalkSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Subject: "测试", Body: "签名测试"}, nil)

	require.NoError(t, err, "带签名的钉钉消息发送应成功")

	// 验证 URL 中包含 timestamp 和 sign 参数
	assert.Contains(t, receivedURL, "timestamp=", "签名 URL 应包含 timestamp 参数")
	assert.Contains(t, receivedURL, "sign=", "签名 URL 应包含 sign 参数")
}

// TestDingtalkSender_WithoutSignature 验证未配置签名密钥时 URL 不包含签名参数。
func TestDingtalkSender_WithoutSignature(t *testing.T) {
	var receivedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := DingtalkConfig{WebhookURL: server.URL}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &DingtalkSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "无签名测试"}, nil)

	require.NoError(t, err)
	assert.NotContains(t, receivedURL, "timestamp=", "未配置签名时 URL 不应包含 timestamp")
	assert.NotContains(t, receivedURL, "sign=", "未配置签名时 URL 不应包含 sign")
}

// TestDingtalkSender_SignatureComputation 验证钉钉 HMAC-SHA256 签名计算正确性。
// 独立测试 dingtalkSignURL 函数，确认签名格式和 URL 编码。
func TestDingtalkSender_SignatureComputation(t *testing.T) {
	secret := "SEC1234567890"
	baseURL := "https://oapi.dingtalk.com/robot/send?access_token=abc123"

	signedURL := dingtalkSignURL(baseURL, secret)

	// 验证签名 URL 格式
	assert.Contains(t, signedURL, baseURL, "签名 URL 应保留原始 URL")
	assert.Contains(t, signedURL, "&timestamp=", "应追加 timestamp 参数")
	assert.Contains(t, signedURL, "&sign=", "应追加 sign 参数")

	// 验证签名不包含未编码的特殊字符（+ 应被编码为 %2B）
	// 提取 sign 参数值
	signIdx := len(baseURL)
	signPart := signedURL[signIdx:]
	// sign 参数中不应有裸露的 + 号（Base64 中的 + 应被 URL 编码）
	for i := 0; i < len(signPart); i++ {
		if signPart[i] == '+' {
			t.Errorf("签名 URL 中不应包含未编码的 + 号: %s", signPart)
			break
		}
	}
}

// TestDingtalkSender_URLEncode 验证 URL 编码函数对 Base64 特殊字符的处理。
func TestDingtalkSender_URLEncode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc+def/ghi=", "abc%2Bdef%2Fghi%3D"},
		{"noSpecialChars", "noSpecialChars"},
		{"+/=", "%2B%2F%3D"},
		{"", ""},
	}

	for _, tt := range tests {
		got := urlEncode(tt.input)
		assert.Equal(t, tt.want, got, "urlEncode(%q) 应返回 %q", tt.input, tt.want)
	}
}

// TestDingtalkSender_ServerError 验证钉钉 API 返回错误时传播错误。
func TestDingtalkSender_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := DingtalkConfig{WebhookURL: server.URL}
	cfgJSON, _ := json.Marshal(cfg)

	sender := &DingtalkSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: cfgJSON},
		MessageContent{Body: "test"}, nil)

	require.Error(t, err, "钉钉 API 返回 500 时应报错")
}

// TestDingtalkSender_InvalidConfig 验证钉钉配置 JSON 无效时返回解析错误。
func TestDingtalkSender_InvalidConfig(t *testing.T) {
	sender := &DingtalkSender{logger: zap.NewNop()}
	err := sender.Send(context.Background(), &Bot{Config: []byte("invalid")},
		MessageContent{Body: "test"}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse dingtalk config")
}
