// Package biz 定义通知服务的核心业务模型与领域逻辑，包括通知渠道、机器人、广播规则和去重引擎。
package biz

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
)

// ChannelSender 是所有通知渠道实现的统一接口，包含发送和测试两个方法。
type ChannelSender interface {
	Send(ctx context.Context, bot *Bot, msg MessageContent, recipients []string) error
	Test(ctx context.Context, bot *Bot) error
}

// ChannelManager 管理所有渠道发送器，提供渠道级别的发送和测试操作。
type ChannelManager struct {
	senders map[ChannelType]ChannelSender
	botRepo BotRepo
	logger  *zap.Logger
}

// NewChannelManager 创建渠道管理器并注册所有支持的渠道发送器。
func NewChannelManager(botRepo BotRepo, logger *zap.Logger) *ChannelManager {
	cm := &ChannelManager{
		senders: make(map[ChannelType]ChannelSender),
		botRepo: botRepo,
		logger:  logger,
	}

	cm.senders[ChannelWecomWebhook] = &WecomWebhookSender{logger: logger}
	cm.senders[ChannelWecomApp] = &WecomAppSender{logger: logger}
	cm.senders[ChannelSMS] = &SMSSender{logger: logger}
	cm.senders[ChannelVoice] = &VoiceSender{logger: logger}
	cm.senders[ChannelEmail] = &EmailSender{logger: logger}
	cm.senders[ChannelWebhook] = &GenericWebhookSender{logger: logger}
	cm.senders[ChannelFeishu] = &FeishuSender{logger: logger}
	cm.senders[ChannelDingtalk] = &DingtalkSender{logger: logger}

	return cm
}

// Send 通过指定机器人的渠道发送消息。
func (cm *ChannelManager) Send(ctx context.Context, bot *Bot, msg MessageContent, recipients []string) error {
	sender, ok := cm.senders[bot.ChannelType]
	if !ok {
		return fmt.Errorf("unsupported channel type: %s", bot.ChannelType)
	}
	return sender.Send(ctx, bot, msg, recipients)
}

// Test 通过指定机器人发送测试消息，用于验证渠道连通性。
func (cm *ChannelManager) Test(ctx context.Context, bot *Bot) error {
	sender, ok := cm.senders[bot.ChannelType]
	if !ok {
		return fmt.Errorf("unsupported channel type: %s", bot.ChannelType)
	}
	return sender.Test(ctx, bot)
}

// GetSender 返回指定渠道类型的发送器。
func (cm *ChannelManager) GetSender(ct ChannelType) (ChannelSender, bool) {
	s, ok := cm.senders[ct]
	return s, ok
}

// --- 企业微信群机器人 Webhook 发送器 ---

// WecomWebhookSender 通过企业微信群机器人 Webhook 发送 Markdown 格式消息。
type WecomWebhookSender struct {
	logger *zap.Logger
}

func (s *WecomWebhookSender) Send(ctx context.Context, bot *Bot, msg MessageContent, recipients []string) error {
	var cfg WecomWebhookConfig
	if err := json.Unmarshal(bot.Config, &cfg); err != nil {
		return fmt.Errorf("parse wecom webhook config: %w", err)
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": msg.Body,
		},
	}

	return postJSON(ctx, cfg.WebhookURL, payload)
}

func (s *WecomWebhookSender) Test(ctx context.Context, bot *Bot) error {
	return s.Send(ctx, bot, MessageContent{Body: "OpsNexus test notification"}, nil)
}

// --- 企业微信应用消息发送器 ---

// WecomAppSender 通过企业微信应用消息接口发送文本消息，需先获取 access_token。
type WecomAppSender struct {
	logger *zap.Logger
}

func (s *WecomAppSender) Send(ctx context.Context, bot *Bot, msg MessageContent, recipients []string) error {
	var cfg WecomAppConfig
	if err := json.Unmarshal(bot.Config, &cfg); err != nil {
		return fmt.Errorf("parse wecom app config: %w", err)
	}

	// Step 1: Get access token
	tokenURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		cfg.CorpID, cfg.Secret)

	resp, err := http.Get(tokenURL)
	if err != nil {
		return fmt.Errorf("get wecom access token: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decode token response: %w", err)
	}
	if tokenResp.ErrCode != 0 {
		return fmt.Errorf("wecom token error: %s", tokenResp.ErrMsg)
	}

	// Step 2: Send message
	sendURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", tokenResp.AccessToken)
	payload := map[string]interface{}{
		"touser":  cfg.ToUser,
		"toparty": cfg.ToParty,
		"msgtype": "text",
		"agentid": cfg.AgentID,
		"text": map[string]string{
			"content": msg.Body,
		},
	}

	return postJSON(ctx, sendURL, payload)
}

func (s *WecomAppSender) Test(ctx context.Context, bot *Bot) error {
	return s.Send(ctx, bot, MessageContent{Body: "OpsNexus test notification"}, nil)
}

// --- 短信发送器 ---

// SMSSender 通过第三方短信服务商的通用 HTTP API 发送短信通知。
// 支持阿里云短信、腾讯云短信等主流服务商，也支持自定义 API 端点。
// 认证方式采用 HMAC-SHA256 签名，通过 X-Access-Key / X-Timestamp / X-Signature 头传递。
type SMSSender struct {
	logger *zap.Logger // 日志记录器
}

// Send 向指定的手机号列表发送短信通知。
//
// 参数:
//   - ctx: 请求上下文，支持超时和取消
//   - bot: 包含短信渠道配置的机器人实例
//   - msg: 消息内容，msg.Variables 中的键值对会作为模板参数传递给短信服务商
//   - recipients: 接收人手机号列表，不能为空
//
// 发送流程:
//  1. 解析 Bot.Config 中的 SMSConfig（服务商、密钥、签名、模板等）
//  2. 将 msg.Variables 合并为模板参数，若无 "content" 键则自动补充 msg.Body
//  3. 逐个手机号调用服务商 HTTP API，使用 HMAC-SHA256 签名认证
//  4. 任一手机号发送失败则立即返回错误
func (s *SMSSender) Send(ctx context.Context, bot *Bot, msg MessageContent, recipients []string) error {
	var cfg SMSConfig
	if err := json.Unmarshal(bot.Config, &cfg); err != nil {
		return fmt.Errorf("parse sms config: %w", err)
	}

	// 校验：接收人列表不能为空
	if len(recipients) == 0 {
		return fmt.Errorf("sms: no recipients specified")
	}

	s.logger.Info("sending SMS",
		zap.String("provider", cfg.Provider),
		zap.Int("recipient_count", len(recipients)),
	)

	// 从消息变量构建短信模板参数，若无 content 键则用消息正文填充
	templateParams := make(map[string]string)
	for k, v := range msg.Variables {
		templateParams[k] = v
	}
	if _, ok := templateParams["content"]; !ok {
		templateParams["content"] = msg.Body
	}

	// 逐个手机号通过服务商 HTTP API 发送短信
	for _, phone := range recipients {
		payload := map[string]interface{}{
			"phone_number":    phone,       // 接收人手机号
			"sign_name":       cfg.SignName, // 短信签名（如"OpsNexus"）
			"template_code":   cfg.Template, // 短信模板编号
			"template_params": templateParams, // 模板变量
		}

		// 优先使用自定义端点，否则根据服务商名称解析默认端点
		apiURL := cfg.APIEndpoint
		if apiURL == "" {
			apiURL = resolveProviderSMSEndpoint(cfg.Provider)
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("sms marshal payload: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("sms create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		// 使用 HMAC-SHA256 对时间戳进行签名，作为 API 认证凭据
		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		mac := hmac.New(sha256.New, []byte(cfg.Secret))
		mac.Write([]byte(ts))
		sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

		req.Header.Set("X-Access-Key", cfg.AccessKey) // 访问密钥
		req.Header.Set("X-Timestamp", ts)             // 请求时间戳（毫秒）
		req.Header.Set("X-Signature", sig)             // HMAC 签名

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("sms api call to %s failed for %s: %w", cfg.Provider, phone, err)
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// HTTP 状态码 >= 400 视为发送失败
		if resp.StatusCode >= 400 {
			return fmt.Errorf("sms api returned %d for %s: %s", resp.StatusCode, phone, string(respBody))
		}

		s.logger.Debug("SMS sent", zap.String("phone", phone), zap.String("template", cfg.Template))
	}

	return nil
}

// resolveProviderSMSEndpoint 根据服务商名称返回默认的短信 API 端点地址。
// 当前支持: aliyun（阿里云短信）、tencent（腾讯云短信），其他返回示例地址。
func resolveProviderSMSEndpoint(provider string) string {
	switch provider {
	case "aliyun":
		return "https://dysmsapi.aliyuncs.com/"
	case "tencent":
		return "https://sms.tencentcloudapi.com/"
	default:
		return "https://sms-api.example.com/send"
	}
}

// Test 验证短信渠道的连通性。
// 通过向服务商 API 端点发送轻量级 GET 请求来检测网络可达性和密钥有效性。
func (s *SMSSender) Test(ctx context.Context, bot *Bot) error {
	var cfg SMSConfig
	if err := json.Unmarshal(bot.Config, &cfg); err != nil {
		return fmt.Errorf("parse sms config: %w", err)
	}

	// 使用自定义端点或根据服务商解析默认端点
	apiURL := cfg.APIEndpoint
	if apiURL == "" {
		apiURL = resolveProviderSMSEndpoint(cfg.Provider)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("sms test create request: %w", err)
	}
	req.Header.Set("X-Access-Key", cfg.AccessKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sms test connectivity failed: %w", err)
	}
	resp.Body.Close()

	s.logger.Info("SMS test connectivity check completed",
		zap.String("provider", cfg.Provider),
		zap.Int("status", resp.StatusCode),
	)
	return nil
}

// --- 语音 TTS 发送器 ---

// VoiceSender 通过语音 TTS（Text-To-Speech）服务商 API 发起自动语音外呼通知。
// 支持阿里云语音、腾讯云语音等主流服务商，也支持自定义 API 端点。
// 认证方式与 SMSSender 相同，采用 HMAC-SHA256 签名。
type VoiceSender struct {
	logger *zap.Logger // 日志记录器
}

// Send 向指定的手机号列表发起语音 TTS 外呼通知。
//
// 参数:
//   - ctx: 请求上下文，支持超时和取消
//   - bot: 包含语音渠道配置的机器人实例
//   - msg: 消息内容，msg.Body 作为 TTS 播报文本，msg.Variables 作为模板参数
//   - recipients: 被叫手机号列表，不能为空
//
// 发送流程:
//  1. 解析 Bot.Config 中的 VoiceConfig（服务商、密钥、主叫号码、模板等）
//  2. 将 msg.Variables 合并为 TTS 模板参数
//  3. 逐个手机号调用服务商 HTTP API 发起语音呼叫
//  4. 任一号码呼叫失败则立即返回错误
func (s *VoiceSender) Send(ctx context.Context, bot *Bot, msg MessageContent, recipients []string) error {
	var cfg VoiceConfig
	if err := json.Unmarshal(bot.Config, &cfg); err != nil {
		return fmt.Errorf("parse voice config: %w", err)
	}

	// 校验：被叫号码列表不能为空
	if len(recipients) == 0 {
		return fmt.Errorf("voice: no recipients specified")
	}

	s.logger.Info("sending voice TTS",
		zap.String("provider", cfg.Provider),
		zap.Int("recipient_count", len(recipients)),
	)

	// 从消息变量构建 TTS 模板参数，若无 content 键则用消息正文填充
	ttsParams := make(map[string]string)
	for k, v := range msg.Variables {
		ttsParams[k] = v
	}
	if _, ok := ttsParams["content"]; !ok {
		ttsParams["content"] = msg.Body
	}

	// 优先使用自定义端点，否则根据服务商名称解析默认端点
	apiURL := cfg.APIEndpoint
	if apiURL == "" {
		apiURL = resolveProviderVoiceEndpoint(cfg.Provider)
	}

	// 逐个被叫号码发起语音外呼
	for _, phone := range recipients {
		payload := map[string]interface{}{
			"called_number":   phone,          // 被叫号码
			"caller_id":       cfg.CallerID,   // 主叫号码（显示号码）
			"template_code":   cfg.Template,   // 语音模板编号
			"template_params": ttsParams,      // 模板变量
			"tts_content":     msg.Body,        // TTS 播报原文
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("voice marshal payload: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("voice create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		// 使用 HMAC-SHA256 对时间戳进行签名，作为 API 认证凭据
		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		mac := hmac.New(sha256.New, []byte(cfg.Secret))
		mac.Write([]byte(ts))
		sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

		req.Header.Set("X-Access-Key", cfg.AccessKey) // 访问密钥
		req.Header.Set("X-Timestamp", ts)             // 请求时间戳（毫秒）
		req.Header.Set("X-Signature", sig)             // HMAC 签名

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("voice api call to %s failed for %s: %w", cfg.Provider, phone, err)
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// HTTP 状态码 >= 400 视为呼叫失败
		if resp.StatusCode >= 400 {
			return fmt.Errorf("voice api returned %d for %s: %s", resp.StatusCode, phone, string(respBody))
		}

		s.logger.Debug("voice call initiated", zap.String("phone", phone), zap.String("template", cfg.Template))
	}

	return nil
}

// resolveProviderVoiceEndpoint 根据服务商名称返回默认的语音 TTS API 端点地址。
// 当前支持: aliyun（阿里云语音服务）、tencent（腾讯云语音服务），其他返回示例地址。
func resolveProviderVoiceEndpoint(provider string) string {
	switch provider {
	case "aliyun":
		return "https://dyvmsapi.aliyuncs.com/"
	case "tencent":
		return "https://vms.tencentcloudapi.com/"
	default:
		return "https://voice-api.example.com/call"
	}
}

// Test 验证语音 TTS 渠道的连通性。
// 通过向服务商 API 端点发送轻量级 GET 请求来检测网络可达性。
func (s *VoiceSender) Test(ctx context.Context, bot *Bot) error {
	var cfg VoiceConfig
	if err := json.Unmarshal(bot.Config, &cfg); err != nil {
		return fmt.Errorf("parse voice config: %w", err)
	}

	// 使用自定义端点或根据服务商解析默认端点
	apiURL := cfg.APIEndpoint
	if apiURL == "" {
		apiURL = resolveProviderVoiceEndpoint(cfg.Provider)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("voice test create request: %w", err)
	}
	req.Header.Set("X-Access-Key", cfg.AccessKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("voice test connectivity failed: %w", err)
	}
	resp.Body.Close()

	s.logger.Info("Voice TTS test connectivity check completed",
		zap.String("provider", cfg.Provider),
		zap.Int("status", resp.StatusCode),
	)
	return nil
}

// --- 邮件（SMTP）发送器 ---

// EmailSender 通过 SMTP 协议发送 HTML 格式的邮件通知。
type EmailSender struct {
	logger *zap.Logger
}

func (s *EmailSender) Send(ctx context.Context, bot *Bot, msg MessageContent, recipients []string) error {
	var cfg EmailConfig
	if err := json.Unmarshal(bot.Config, &cfg); err != nil {
		return fmt.Errorf("parse email config: %w", err)
	}

	m := gomail.NewMessage()
	m.SetHeader("From", cfg.From)
	m.SetHeader("To", recipients...)
	m.SetHeader("Subject", msg.Subject)

	// 渲染正文：使用 ${key} 格式进行模板变量替换
	body := msg.Body
	for k, v := range msg.Variables {
		body = replaceAll(body, "${"+k+"}", v)
	}

	// 自动检测内容类型：
	// - 包含 HTML 标签时，发送 multipart/alternative 格式（同时包含纯文本和 HTML 版本）
	// - 不含 HTML 标签时，仅发送 text/plain 纯文本格式
	if isHTML(body) {
		m.SetBody("text/plain", stripHTMLTags(body))   // 纯文本版本（剥离 HTML 标签）
		m.AddAlternative("text/html", body)            // HTML 版本（保持原始格式）
	} else {
		m.SetBody("text/plain", body)                  // 纯文本发送
	}

	d := gomail.NewDialer(cfg.SMTPHost, cfg.SMTPPort, cfg.Username, cfg.Password)
	if cfg.UseTLS {
		d.SSL = true
	}

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	s.logger.Debug("email sent", zap.Int("recipient_count", len(recipients)))
	return nil
}

// isHTML 检测字符串是否包含 HTML 标签（即是否存在 <...> 模式）。
// 用于邮件发送时自动判断应使用 text/html 还是 text/plain 格式。
func isHTML(s string) bool {
	inTag := false
	for _, c := range s {
		if c == '<' {
			inTag = true
		} else if c == '>' && inTag {
			return true
		}
	}
	return false
}

// stripHTMLTags 从字符串中移除所有 HTML 标签，生成纯文本版本。
// 用于为 HTML 邮件自动生成 text/plain 备选内容。
func stripHTMLTags(s string) string {
	var result []byte
	inTag := false
	for i := 0; i < len(s); i++ {
		if s[i] == '<' {
			inTag = true
			continue
		}
		if s[i] == '>' && inTag {
			inTag = false
			continue
		}
		if !inTag {
			result = append(result, s[i])
		}
	}
	return string(result)
}

func (s *EmailSender) Test(ctx context.Context, bot *Bot) error {
	return s.Send(ctx, bot, MessageContent{
		Subject: "OpsNexus Test Notification",
		Body:    "<h3>OpsNexus</h3><p>This is a test notification.</p>",
	}, nil)
}

// --- 通用 Webhook 发送器 ---

// GenericWebhookSender 向自定义 URL 发送 JSON 格式的 Webhook 通知，支持 HMAC 签名验证。
type GenericWebhookSender struct {
	logger *zap.Logger
}

func (s *GenericWebhookSender) Send(ctx context.Context, bot *Bot, msg MessageContent, recipients []string) error {
	var cfg WebhookConfig
	if err := json.Unmarshal(bot.Config, &cfg); err != nil {
		return fmt.Errorf("parse webhook config: %w", err)
	}

	payload := map[string]interface{}{
		"subject":    msg.Subject,
		"body":       msg.Body,
		"variables":  msg.Variables,
		"recipients": recipients,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	method := cfg.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	// HMAC signature if secret is configured
	if cfg.Secret != "" {
		mac := hmac.New(sha256.New, []byte(cfg.Secret))
		mac.Write(body)
		sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-OpsNexus-Signature", sig)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook call: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *GenericWebhookSender) Test(ctx context.Context, bot *Bot) error {
	return s.Send(ctx, bot, MessageContent{
		Body: `{"event":"test","source":"opsnexus"}`,
	}, nil)
}

// --- 飞书群机器人 Webhook 发送器 ---

// FeishuSender 通过飞书自定义机器人 Webhook 发送交互式卡片（interactive card）消息。
// 卡片颜色根据消息内容中的严重级别关键词自动匹配：
//   - red: 包含 P0/严重/critical 关键词
//   - orange: 包含 P1/警告/warning 关键词
//   - blue: 包含 P2/提示/info 关键词
//   - grey: 其他情况（默认）
type FeishuSender struct {
	logger *zap.Logger
}

// Send 通过飞书 Webhook 发送交互式卡片消息。
//
// 参数:
//   - ctx: 请求上下文
//   - bot: 包含飞书渠道配置的机器人实例
//   - msg: 消息内容，msg.Subject 作为卡片标题，msg.Body 作为卡片正文（支持 lark_md 格式）
//   - recipients: 飞书 Webhook 不支持指定接收人，此参数忽略
func (s *FeishuSender) Send(ctx context.Context, bot *Bot, msg MessageContent, recipients []string) error {
	var cfg FeishuConfig
	if err := json.Unmarshal(bot.Config, &cfg); err != nil {
		return fmt.Errorf("parse feishu config: %w", err)
	}

	// 根据消息内容自动选择卡片头部颜色模板
	headerTemplate := feishuHeaderTemplate(msg.Subject, msg.Body)

	// 卡片标题：优先使用 Subject，否则使用默认标题
	title := msg.Subject
	if title == "" {
		title = "OpsNexus 通知"
	}

	// 构造飞书交互式卡片消息体
	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]string{
					"tag":     "plain_text",
					"content": title,
				},
				"template": headerTemplate,
			},
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"text": map[string]string{
						"tag":     "lark_md",
						"content": msg.Body,
					},
				},
			},
		},
	}

	return postJSON(ctx, cfg.WebhookURL, payload)
}

// feishuHeaderTemplate 根据消息标题和正文中的严重级别关键词返回飞书卡片头部颜色。
// 返回值为飞书支持的模板颜色：red（严重）、orange（警告）、blue（提示）、grey（默认）。
func feishuHeaderTemplate(subject, body string) string {
	combined := subject + body
	// 按优先级匹配严重级别关键词
	for _, keyword := range []string{"P0", "严重", "critical", "CRITICAL"} {
		if containsStr(combined, keyword) {
			return "red"
		}
	}
	for _, keyword := range []string{"P1", "警告", "warning", "WARNING"} {
		if containsStr(combined, keyword) {
			return "orange"
		}
	}
	for _, keyword := range []string{"P2", "提示", "info", "INFO"} {
		if containsStr(combined, keyword) {
			return "blue"
		}
	}
	return "grey"
}

// containsStr 检查字符串 s 是否包含子串 substr。
func containsStr(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && findSubstr(s, substr)
}

// findSubstr 在字符串 s 中查找子串 substr，找到返回 true。
func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test 验证飞书 Webhook 渠道的连通性，发送一条测试卡片消息。
func (s *FeishuSender) Test(ctx context.Context, bot *Bot) error {
	return s.Send(ctx, bot, MessageContent{
		Subject: "OpsNexus 连通性测试",
		Body:    "这是一条来自 OpsNexus 的测试通知，用于验证飞书机器人 Webhook 连通性。",
	}, nil)
}

// --- 钉钉群机器人 Webhook 发送器 ---

// DingtalkSender 通过钉钉自定义机器人 Webhook 发送 ActionCard 格式消息。
// 支持可选的 HMAC-SHA256 签名验证（在钉钉机器人安全设置中配置"加签"时启用）。
type DingtalkSender struct {
	logger *zap.Logger
}

// Send 通过钉钉 Webhook 发送 ActionCard 格式消息。
//
// 参数:
//   - ctx: 请求上下文
//   - bot: 包含钉钉渠道配置的机器人实例
//   - msg: 消息内容，msg.Subject 作为 ActionCard 标题，msg.Body 作为 Markdown 正文
//   - recipients: 钉钉 Webhook 不支持指定接收人，此参数忽略
//
// 签名机制:
//   - 若 DingtalkConfig.Secret 非空，则在 Webhook URL 中追加 timestamp 和 sign 参数
//   - 签名算法：HMAC-SHA256(timestamp + "\n" + secret)，结果 Base64 + URL 编码
func (s *DingtalkSender) Send(ctx context.Context, bot *Bot, msg MessageContent, recipients []string) error {
	var cfg DingtalkConfig
	if err := json.Unmarshal(bot.Config, &cfg); err != nil {
		return fmt.Errorf("parse dingtalk config: %w", err)
	}

	// ActionCard 标题：优先使用 Subject，否则使用默认标题
	title := msg.Subject
	if title == "" {
		title = "OpsNexus 通知"
	}

	// 构造 ActionCard 消息体
	actionCard := map[string]interface{}{
		"title":          title,
		"text":           msg.Body,
		"btnOrientation": "0",
	}

	// 如果配置了详情链接，添加"查看详情"按钮
	detailURL := cfg.DetailURL
	if detailURL == "" {
		detailURL = "https://opsnexus.example.com"
	}
	actionCard["btns"] = []map[string]string{
		{"title": "查看详情", "actionURL": detailURL},
	}

	payload := map[string]interface{}{
		"msgtype":    "actionCard",
		"actionCard": actionCard,
	}

	// 计算请求目标 URL（可能需要追加签名参数）
	webhookURL := cfg.WebhookURL
	if cfg.Secret != "" {
		webhookURL = dingtalkSignURL(webhookURL, cfg.Secret)
	}

	return postJSON(ctx, webhookURL, payload)
}

// dingtalkSignURL 为钉钉 Webhook URL 追加 HMAC-SHA256 签名参数。
// 签名算法：sign = Base64(HmacSHA256(timestamp + "\n" + secret, secret))
// 签名后的 URL 格式：{webhook_url}&timestamp={ts}&sign={sign}
func dingtalkSignURL(webhookURL, secret string) string {
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	stringToSign := ts + "\n" + secret

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// URL 编码签名值（Base64 中的 + 和 / 需要编码）
	encodedSign := urlEncode(sign)

	return webhookURL + "&timestamp=" + ts + "&sign=" + encodedSign
}

// urlEncode 对字符串进行 URL 编码，处理 Base64 签名中的特殊字符（+、/、=）。
func urlEncode(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '+':
			result = append(result, '%', '2', 'B')
		case '/':
			result = append(result, '%', '2', 'F')
		case '=':
			result = append(result, '%', '3', 'D')
		default:
			result = append(result, s[i])
		}
	}
	return string(result)
}

// Test 验证钉钉 Webhook 渠道的连通性，发送一条测试 ActionCard 消息。
func (s *DingtalkSender) Test(ctx context.Context, bot *Bot) error {
	return s.Send(ctx, bot, MessageContent{
		Subject: "OpsNexus 连通性测试",
		Body:    "这是一条来自 OpsNexus 的测试通知，用于验证钉钉机器人 Webhook 连通性。",
	}, nil)
}

// postJSON 是向指定 URL 发送 JSON POST 请求的辅助函数。
func postJSON(ctx context.Context, url string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
