package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ===========================================
// Mock 实现：用于绕过外部依赖（HTTP/SMTP/Redis）进行单元测试
// ===========================================

// mockChannelSender 是 ChannelSender 接口的 mock 实现，
// 记录所有调用参数，可配置返回错误，用于验证 ChannelManager 的路由逻辑。
type mockChannelSender struct {
	sendCalled bool            // 是否调用过 Send
	testCalled bool            // 是否调用过 Test
	lastBot    *Bot            // 最近一次调用的 Bot 参数
	lastMsg    MessageContent  // 最近一次调用的消息内容
	lastRecips []string        // 最近一次调用的接收人列表
	sendErr    error           // Send 方法返回的错误（nil 表示成功）
	testErr    error           // Test 方法返回的错误（nil 表示成功）
}

// Send 记录调用参数并返回预设的错误。
func (m *mockChannelSender) Send(ctx context.Context, bot *Bot, msg MessageContent, recipients []string) error {
	m.sendCalled = true
	m.lastBot = bot
	m.lastMsg = msg
	m.lastRecips = recipients
	return m.sendErr
}

// Test 记录调用参数并返回预设的错误。
func (m *mockChannelSender) Test(ctx context.Context, bot *Bot) error {
	m.testCalled = true
	m.lastBot = bot
	return m.testErr
}

// mockBotRepo 是 BotRepo 接口的 mock 实现，用于测试通知用例中的机器人查询逻辑。
type mockBotRepo struct {
	bots          []*Bot                         // 所有机器人列表
	enabledBots   []*Bot                         // 启用的机器人列表（ListEnabled 返回）
	getByIDResult map[uuid.UUID]*Bot             // 按 ID 查询结果
	healthUpdates []healthUpdate                 // 记录的健康状态更新
	createErr     error                          // Create 返回的错误
}

// healthUpdate 记录一次健康状态更新的参数。
type healthUpdate struct {
	ID           uuid.UUID
	Status       string
	FailureCount int
}

func (m *mockBotRepo) Create(ctx context.Context, bot *Bot) error     { return m.createErr }
func (m *mockBotRepo) GetByID(ctx context.Context, id uuid.UUID) (*Bot, error) {
	if bot, ok := m.getByIDResult[id]; ok {
		return bot, nil
	}
	return nil, fmt.Errorf("bot not found: %s", id)
}
func (m *mockBotRepo) List(ctx context.Context) ([]*Bot, error)        { return m.bots, nil }
func (m *mockBotRepo) Update(ctx context.Context, bot *Bot) error      { return nil }
func (m *mockBotRepo) Delete(ctx context.Context, id uuid.UUID) error  { return nil }
func (m *mockBotRepo) ListEnabled(ctx context.Context) ([]*Bot, error) { return m.enabledBots, nil }
func (m *mockBotRepo) UpdateHealthStatus(ctx context.Context, id uuid.UUID, status string, failureCount int) error {
	m.healthUpdates = append(m.healthUpdates, healthUpdate{ID: id, Status: status, FailureCount: failureCount})
	return nil
}

// mockNotificationLogRepo 是 NotificationLogRepo 接口的 mock 实现，记录所有创建的通知日志。
type mockNotificationLogRepo struct {
	logs []*NotificationLog // 已记录的通知日志列表
}

func (m *mockNotificationLogRepo) Create(ctx context.Context, log *NotificationLog) error {
	m.logs = append(m.logs, log)
	return nil
}
func (m *mockNotificationLogRepo) GetByID(ctx context.Context, id uuid.UUID) (*NotificationLog, error) {
	for _, l := range m.logs {
		if l.ID == id {
			return l, nil
		}
	}
	return nil, fmt.Errorf("log not found")
}
func (m *mockNotificationLogRepo) List(ctx context.Context, filter NotificationFilter) ([]*NotificationLog, string, error) {
	return m.logs, "", nil
}

// ===========================================
// ChannelManager 路由分发测试
// 验证 ChannelManager 能正确根据 Bot 的渠道类型将 Send/Test 请求路由到对应的 ChannelSender。
// ===========================================

// newTestChannelManager 创建带有 mock sender 的 ChannelManager，
// 可自定义注入指定渠道的 mock sender。
func newTestChannelManager(senders map[ChannelType]*mockChannelSender) (*ChannelManager, map[ChannelType]*mockChannelSender) {
	logger := zap.NewNop()
	cm := &ChannelManager{
		senders: make(map[ChannelType]ChannelSender),
		logger:  logger,
	}
	for ct, s := range senders {
		cm.senders[ct] = s
	}
	return cm, senders
}

// TestChannelManager_SendRoutesToCorrectSender 验证 Send 方法根据 Bot.ChannelType 路由到正确的 sender。
func TestChannelManager_SendRoutesToCorrectSender(t *testing.T) {
	wecomMock := &mockChannelSender{}
	smsMock := &mockChannelSender{}
	emailMock := &mockChannelSender{}

	cm, _ := newTestChannelManager(map[ChannelType]*mockChannelSender{
		ChannelWecomWebhook: wecomMock,
		ChannelSMS:          smsMock,
		ChannelEmail:        emailMock,
	})

	// 构造企业微信 Webhook 类型的 Bot
	bot := &Bot{
		ID:          uuid.New(),
		Name:        "wecom-bot",
		ChannelType: ChannelWecomWebhook,
	}
	msg := MessageContent{Body: "test alert"}
	recipients := []string{"user1", "user2"}

	// 发送应路由到 wecomMock
	err := cm.Send(context.Background(), bot, msg, recipients)
	require.NoError(t, err)

	assert.True(t, wecomMock.sendCalled, "企业微信 sender 应被调用")
	assert.False(t, smsMock.sendCalled, "短信 sender 不应被调用")
	assert.False(t, emailMock.sendCalled, "邮件 sender 不应被调用")
	assert.Equal(t, "test alert", wecomMock.lastMsg.Body)
	assert.Equal(t, recipients, wecomMock.lastRecips)
}

// TestChannelManager_SendUnsupportedChannel 验证使用未注册的渠道类型时返回错误。
func TestChannelManager_SendUnsupportedChannel(t *testing.T) {
	cm, _ := newTestChannelManager(map[ChannelType]*mockChannelSender{})

	bot := &Bot{
		ID:          uuid.New(),
		ChannelType: ChannelType("dingtalk"), // 未注册的渠道类型
	}

	err := cm.Send(context.Background(), bot, MessageContent{Body: "hello"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported channel type")
}

// TestChannelManager_SendPropagatesSenderError 验证 sender 返回的错误能正确传播到调用方。
func TestChannelManager_SendPropagatesSenderError(t *testing.T) {
	smsMock := &mockChannelSender{sendErr: fmt.Errorf("sms api timeout")}

	cm, _ := newTestChannelManager(map[ChannelType]*mockChannelSender{
		ChannelSMS: smsMock,
	})

	bot := &Bot{ID: uuid.New(), ChannelType: ChannelSMS}
	err := cm.Send(context.Background(), bot, MessageContent{Body: "alert"}, []string{"13800138000"})

	require.Error(t, err)
	assert.Equal(t, "sms api timeout", err.Error())
	assert.True(t, smsMock.sendCalled)
}

// TestChannelManager_TestRoutesToCorrectSender 验证 Test 方法根据 Bot.ChannelType 路由到正确的 sender。
func TestChannelManager_TestRoutesToCorrectSender(t *testing.T) {
	webhookMock := &mockChannelSender{}
	voiceMock := &mockChannelSender{}

	cm, _ := newTestChannelManager(map[ChannelType]*mockChannelSender{
		ChannelWebhook: webhookMock,
		ChannelVoice:   voiceMock,
	})

	bot := &Bot{ID: uuid.New(), ChannelType: ChannelVoice}
	err := cm.Test(context.Background(), bot)

	require.NoError(t, err)
	assert.True(t, voiceMock.testCalled, "语音 sender 的 Test 应被调用")
	assert.False(t, webhookMock.testCalled, "Webhook sender 的 Test 不应被调用")
}

// TestChannelManager_TestUnsupportedChannel 验证对未注册渠道类型调用 Test 时返回错误。
func TestChannelManager_TestUnsupportedChannel(t *testing.T) {
	cm, _ := newTestChannelManager(map[ChannelType]*mockChannelSender{})

	bot := &Bot{ID: uuid.New(), ChannelType: ChannelType("slack")}
	err := cm.Test(context.Background(), bot)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported channel type")
}

// TestChannelManager_GetSender 验证 GetSender 按渠道类型正确返回已注册的 sender。
func TestChannelManager_GetSender(t *testing.T) {
	emailMock := &mockChannelSender{}
	cm, _ := newTestChannelManager(map[ChannelType]*mockChannelSender{
		ChannelEmail: emailMock,
	})

	// 已注册的渠道类型应返回 sender 和 true
	sender, ok := cm.GetSender(ChannelEmail)
	assert.True(t, ok)
	assert.NotNil(t, sender)

	// 未注册的渠道类型应返回 nil 和 false
	sender, ok = cm.GetSender(ChannelSMS)
	assert.False(t, ok)
	assert.Nil(t, sender)
}

// ===========================================
// 全渠道类型 Send 路由测试
// 遍历所有 6 种渠道类型，验证 ChannelManager 对每种类型都能正确路由。
// ===========================================

// TestChannelManager_AllChannelTypes 验证所有 8 种渠道类型均能正确路由 Send 调用。
func TestChannelManager_AllChannelTypes(t *testing.T) {
	channelTypes := []ChannelType{
		ChannelWecomWebhook,
		ChannelWecomApp,
		ChannelSMS,
		ChannelVoice,
		ChannelEmail,
		ChannelWebhook,
		ChannelFeishu,
		ChannelDingtalk,
	}

	for _, ct := range channelTypes {
		t.Run(string(ct), func(t *testing.T) {
			mock := &mockChannelSender{}
			cm, _ := newTestChannelManager(map[ChannelType]*mockChannelSender{ct: mock})

			bot := &Bot{ID: uuid.New(), ChannelType: ct}
			err := cm.Send(context.Background(), bot, MessageContent{Body: "test"}, nil)

			require.NoError(t, err)
			assert.True(t, mock.sendCalled, "渠道 %s 的 sender 应被调用", ct)
		})
	}
}

// ===========================================
// NotifyUseCase Bot 匹配测试
// 验证 NotifyUseCase 在查找匹配渠道的 Bot 时的行为。
// 注意：由于 DedupEngine 依赖 Redis（具体类型非接口），
// 完整的 Send 流程需要在集成测试中验证。此处仅测试 Bot 查找逻辑。
// ===========================================

// TestNotifyUseCase_NoBotForChannel 验证无匹配渠道的 Bot 时返回错误。
func TestNotifyUseCase_NoBotForChannel(t *testing.T) {
	logger := zap.NewNop()
	cm := &ChannelManager{
		senders: map[ChannelType]ChannelSender{ChannelWecomWebhook: &mockChannelSender{}},
		logger:  logger,
	}

	// 机器人仓储中只有 wecom_webhook 渠道的 Bot
	botRepo := &mockBotRepo{
		enabledBots: []*Bot{
			{
				ID:           uuid.New(),
				Name:         "wecom-bot",
				ChannelType:  ChannelWecomWebhook,
				Enabled:      true,
				HealthStatus: "healthy",
			},
		},
	}

	uc := &NotifyUseCase{
		channelManager: cm,
		botRepo:        botRepo,
		logRepo:        &mockNotificationLogRepo{},
		logger:         logger,
	}

	req := SendRequest{
		Channel:    ChannelEmail, // 请求邮件渠道，但没有对应的 Bot
		Recipients: []string{"admin@example.com"},
		Content:    MessageContent{Body: "test"},
	}

	log, err := uc.Send(context.Background(), req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no enabled bot")
	assert.Nil(t, log)
}

// ===========================================
// ChannelManager 与 MessageContent 交互测试
// 验证消息内容（Subject/Body/Variables）正确传递到 sender。
// ===========================================

// TestChannelManager_MessageContentPassThrough 验证完整的 MessageContent 被正确传递到 sender。
func TestChannelManager_MessageContentPassThrough(t *testing.T) {
	mock := &mockChannelSender{}
	cm, _ := newTestChannelManager(map[ChannelType]*mockChannelSender{
		ChannelEmail: mock,
	})

	bot := &Bot{ID: uuid.New(), ChannelType: ChannelEmail}
	msg := MessageContent{
		Subject:   "[P0] 数据库连接池耗尽",
		Body:      "<h3>告警详情</h3><p>连接池使用率: 100%</p>",
		Variables: map[string]string{"host": "db-master-01", "pool_size": "100"},
	}
	recipients := []string{"dba@example.com", "oncall@example.com"}

	err := cm.Send(context.Background(), bot, msg, recipients)

	require.NoError(t, err)
	assert.Equal(t, "[P0] 数据库连接池耗尽", mock.lastMsg.Subject)
	assert.Equal(t, "<h3>告警详情</h3><p>连接池使用率: 100%</p>", mock.lastMsg.Body)
	assert.Equal(t, "db-master-01", mock.lastMsg.Variables["host"])
	assert.Equal(t, recipients, mock.lastRecips)
}

// ===========================================
// ChannelHealthProbe 健康探测测试
// 验证健康探测器对 Bot 渠道的探测结果能正确更新健康状态。
// ===========================================

// TestChannelManager_TestSuccessAndFailure 验证 Test 成功和失败场景。
func TestChannelManager_TestSuccessAndFailure(t *testing.T) {
	t.Run("test success", func(t *testing.T) {
		mock := &mockChannelSender{testErr: nil}
		cm, _ := newTestChannelManager(map[ChannelType]*mockChannelSender{
			ChannelWebhook: mock,
		})

		bot := &Bot{ID: uuid.New(), ChannelType: ChannelWebhook}
		err := cm.Test(context.Background(), bot)

		require.NoError(t, err)
		assert.True(t, mock.testCalled)
	})

	t.Run("test failure", func(t *testing.T) {
		mock := &mockChannelSender{testErr: fmt.Errorf("connection refused")}
		cm, _ := newTestChannelManager(map[ChannelType]*mockChannelSender{
			ChannelWebhook: mock,
		})

		bot := &Bot{ID: uuid.New(), ChannelType: ChannelWebhook}
		err := cm.Test(context.Background(), bot)

		require.Error(t, err)
		assert.Equal(t, "connection refused", err.Error())
	})
}

// ===========================================
// NewChannelManager 工厂方法测试
// 验证通过标准构造函数创建的 ChannelManager 已注册所有 6 种渠道发送器。
// ===========================================

// TestNewChannelManager_RegistersAllSenders 验证 NewChannelManager 注册了全部 8 种渠道发送器。
func TestNewChannelManager_RegistersAllSenders(t *testing.T) {
	logger := zap.NewNop()
	botRepo := &mockBotRepo{}

	cm := NewChannelManager(botRepo, logger)

	expectedChannels := []ChannelType{
		ChannelWecomWebhook,
		ChannelWecomApp,
		ChannelSMS,
		ChannelVoice,
		ChannelEmail,
		ChannelWebhook,
		ChannelFeishu,
		ChannelDingtalk,
	}

	for _, ct := range expectedChannels {
		sender, ok := cm.GetSender(ct)
		assert.True(t, ok, "渠道 %s 应已注册", ct)
		assert.NotNil(t, sender, "渠道 %s 的 sender 不应为 nil", ct)
	}
}

// ===========================================
// Bot 配置解析测试
// 验证各渠道类型的 JSON 配置能正确反序列化。
// ===========================================

// TestWecomWebhookConfigParsing 验证企业微信 Webhook 配置解析。
func TestWecomWebhookConfigParsing(t *testing.T) {
	cfgJSON := `{"webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"}`

	var cfg WecomWebhookConfig
	err := json.Unmarshal([]byte(cfgJSON), &cfg)

	require.NoError(t, err)
	assert.Equal(t, "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx", cfg.WebhookURL)
}

// TestWecomAppConfigParsing 验证企业微信应用消息配置解析。
func TestWecomAppConfigParsing(t *testing.T) {
	cfgJSON := `{
		"corp_id": "ww12345",
		"agent_id": 1000001,
		"secret": "appsecret",
		"to_user": "@all",
		"to_party": "1|2"
	}`

	var cfg WecomAppConfig
	err := json.Unmarshal([]byte(cfgJSON), &cfg)

	require.NoError(t, err)
	assert.Equal(t, "ww12345", cfg.CorpID)
	assert.Equal(t, 1000001, cfg.AgentID)
	assert.Equal(t, "appsecret", cfg.Secret)
	assert.Equal(t, "@all", cfg.ToUser)
	assert.Equal(t, "1|2", cfg.ToParty)
}

// TestEmailConfigParsing 验证邮件 SMTP 配置解析。
func TestEmailConfigParsing(t *testing.T) {
	cfgJSON := `{
		"smtp_host": "smtp.example.com",
		"smtp_port": 465,
		"username": "alert@example.com",
		"password": "smtp-pass",
		"from": "OpsNexus <alert@example.com>",
		"use_tls": true
	}`

	var cfg EmailConfig
	err := json.Unmarshal([]byte(cfgJSON), &cfg)

	require.NoError(t, err)
	assert.Equal(t, "smtp.example.com", cfg.SMTPHost)
	assert.Equal(t, 465, cfg.SMTPPort)
	assert.True(t, cfg.UseTLS)
	assert.Equal(t, "OpsNexus <alert@example.com>", cfg.From)
}

// TestWebhookConfigParsing 验证通用 Webhook 配置解析（含自定义 headers 和 HMAC secret）。
func TestWebhookConfigParsing(t *testing.T) {
	cfgJSON := `{
		"url": "https://hooks.example.com/notify",
		"method": "POST",
		"headers": {"Authorization": "Bearer token123", "X-Custom": "value"},
		"secret": "hmac-secret"
	}`

	var cfg WebhookConfig
	err := json.Unmarshal([]byte(cfgJSON), &cfg)

	require.NoError(t, err)
	assert.Equal(t, "https://hooks.example.com/notify", cfg.URL)
	assert.Equal(t, "POST", cfg.Method)
	assert.Equal(t, "Bearer token123", cfg.Headers["Authorization"])
	assert.Equal(t, "hmac-secret", cfg.Secret)
}

// ===========================================
// Bot 结构体基本测试
// ===========================================

// TestBot_TimestampsSet 验证 Bot 的时间戳字段能正确赋值。
func TestBot_TimestampsSet(t *testing.T) {
	now := time.Now()
	bot := &Bot{
		ID:           uuid.New(),
		Name:         "test-bot",
		ChannelType:  ChannelSMS,
		Enabled:      true,
		HealthStatus: "healthy",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	assert.False(t, bot.CreatedAt.IsZero())
	assert.False(t, bot.UpdatedAt.IsZero())
	assert.True(t, bot.Enabled)
	assert.Equal(t, "healthy", bot.HealthStatus)
}
