package biz

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/opsnexus/svc-ai/internal/config"
	"go.uber.org/zap"
)

// ModelProvider 定义模型提供商类型常量，用于区分不同的 API 调用方式。
type ModelProvider string

const (
	ProviderClaude  ModelProvider = "claude"   // Anthropic Claude，使用 Messages API
	ProviderOpenAI  ModelProvider = "openai"   // OpenAI 或兼容接口（通义千问、DeepSeek 等）
	ProviderOllama  ModelProvider = "ollama"   // Ollama 本地模型，使用 /api/chat 端点
	ProviderVLLM    ModelProvider = "vllm"     // vLLM 本地推理引擎，使用 OpenAI 兼容接口
	ProviderLocalAI ModelProvider = "local_ai" // LocalAI 通用本地模型框架
)

// ModelManager 负责 AI 模型选择、路由策略、故障转移和预算控制。
// 通过 SceneBinding 配置将不同业务场景映射到对应的主/备模型，
// 支持 cloud_first（云端优先）和 local_first（本地优先）等路由策略。
type ModelManager struct {
	modelRepo  ModelRepo
	budgetRepo BudgetRepo
	cfg        config.AIConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewModelManager 创建模型管理器实例，配置 HTTP 客户端超时时间。
func NewModelManager(modelRepo ModelRepo, budgetRepo BudgetRepo, cfg config.AIConfig, logger *zap.Logger) *ModelManager {
	return &ModelManager{
		modelRepo:  modelRepo,
		budgetRepo: budgetRepo,
		cfg:        cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.DefaultTimeoutSeconds) * time.Second,
		},
		logger: logger,
	}
}

// CallResult 封装模型调用结果及其元数据（模型 ID、延迟、是否使用主模型等）。
type CallResult struct {
	Response    ChatResponse
	ModelID     uuid.UUID
	ModelName   string
	LatencyMs   int
	UsedPrimary bool
}

// CallModel 为指定场景选择并调用合适的 AI 模型。
// 实现多层 fallback 逻辑：
//  1. 检查主模型预算是否耗尽（云端模型），耗尽则切换到备用模型
//  2. 尝试调用主模型，成功则记录 token 用量并返回
//  3. 主模型失败时自动降级到备用模型，并将主模型标记为 degraded
//  4. 备用模型也失败则返回错误
func (mm *ModelManager) CallModel(ctx context.Context, scene string, req ChatRequest) (*CallResult, error) {
	binding, err := mm.modelRepo.GetSceneBinding(ctx, scene)
	if err != nil {
		return nil, fmt.Errorf("no scene binding for %q: %w", scene, err)
	}

	strategy := binding.RoutingStrategy
	if strategy == "" {
		strategy = "cloud_first"
	}

	var primaryModel, fallbackModel *AIModel

	if binding.PrimaryModelID != nil {
		primaryModel, err = mm.modelRepo.GetByID(ctx, *binding.PrimaryModelID)
		if err != nil {
			mm.logger.Warn("failed to load primary model", zap.Error(err))
		}
	}
	if binding.FallbackModelID != nil {
		fallbackModel, err = mm.modelRepo.GetByID(ctx, *binding.FallbackModelID)
		if err != nil {
			mm.logger.Warn("failed to load fallback model", zap.Error(err))
		}
	}

	// Check budget for cloud models
	if primaryModel != nil && primaryModel.DeploymentType == "cloud" {
		exhausted, err := mm.checkBudget(ctx, primaryModel.ID)
		if err != nil {
			mm.logger.Warn("budget check failed", zap.Error(err))
		}
		if exhausted && fallbackModel != nil {
			mm.logger.Warn("primary model budget exhausted, switching to fallback",
				zap.String("primary", primaryModel.Name),
				zap.String("fallback", fallbackModel.Name),
			)
			primaryModel, fallbackModel = fallbackModel, nil
		}
	}

	// Try primary model
	if primaryModel != nil && primaryModel.Enabled {
		result, err := mm.callSingleModel(ctx, primaryModel, req)
		if err == nil {
			mm.recordUsage(ctx, primaryModel.ID, result.Response.InputTokens+result.Response.OutputTokens)
			result.UsedPrimary = true
			return result, nil
		}
		mm.logger.Warn("primary model call failed, trying fallback",
			zap.String("model", primaryModel.Name),
			zap.Error(err),
		)
		_ = mm.modelRepo.UpdateHealthStatus(ctx, primaryModel.ID, "degraded")
	}

	// Try fallback model
	if fallbackModel != nil && fallbackModel.Enabled {
		result, err := mm.callSingleModel(ctx, fallbackModel, req)
		if err == nil {
			mm.recordUsage(ctx, fallbackModel.ID, result.Response.InputTokens+result.Response.OutputTokens)
			result.UsedPrimary = false
			return result, nil
		}
		mm.logger.Error("fallback model also failed",
			zap.String("model", fallbackModel.Name),
			zap.Error(err),
		)
		_ = mm.modelRepo.UpdateHealthStatus(ctx, fallbackModel.ID, "degraded")
	}

	return nil, fmt.Errorf("all models failed for scene %q", scene)
}

// callSingleModel 调用单个 AI 模型，根据 Provider 类型自动选择正确的 API 格式。
// - Claude 提供商：使用 Anthropic Messages API（POST /v1/messages），system 为顶级参数
// - Ollama 提供商：使用 Ollama 原生 Chat API（POST /api/chat）
// - vLLM / LocalAI 提供商：使用 OpenAI Chat Completions 兼容格式（POST /v1/chat/completions）
// - 其他提供商（通义千问/DeepSeek 等）：使用 OpenAI Chat Completions 兼容格式
func (mm *ModelManager) callSingleModel(ctx context.Context, model *AIModel, req ChatRequest) (*CallResult, error) {
	switch ModelProvider(model.Provider) {
	case ProviderClaude:
		return mm.callClaude(ctx, model, req)
	case ProviderOllama:
		return mm.callLocalModel(ctx, model, req)
	case ProviderVLLM, ProviderLocalAI:
		return mm.callOpenAICompatible(ctx, model, req)
	default:
		return mm.callOpenAICompatible(ctx, model, req)
	}
}

// callClaude 调用 Anthropic Claude Messages API（POST /v1/messages）。
//
// Claude Messages API 与 OpenAI 格式的主要区别：
//  1. system prompt 是顶级参数，不在 messages 数组中
//  2. 响应中 content 是数组，每个元素有 type 字段（text/image 等）
//  3. 认证使用 x-api-key 头而非 Authorization: Bearer
//  4. 需要设置 anthropic-version 头（当前使用 2023-06-01）
//  5. max_tokens 是必填参数
func (mm *ModelManager) callClaude(ctx context.Context, model *AIModel, req ChatRequest) (*CallResult, error) {
	start := time.Now()

	// 设置默认参数
	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.3 // 默认温度 0.3，适合运维分析场景
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2000 // 默认最大输出 2000 tokens
	}

	// 构建 Claude Messages API 请求体
	// 注意：system 是顶级参数，messages 中只包含 user 消息
	claudeReq := claudeMessagesRequest{
		Model:       mm.resolveModelName(model),
		MaxTokens:   maxTokens,
		System:      req.SystemPrompt,                                     // system prompt 作为顶级参数
		Messages:    []claudeMessage{{Role: "user", Content: req.UserPrompt}}, // 用户消息
		Temperature: temperature,
	}

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("marshal claude request: %w", err)
	}

	// 使用配置的端点或默认 Anthropic API 地址
	endpoint := model.APIEndpoint
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1/messages"
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create claude request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01") // API 版本号

	// Claude 使用 x-api-key 头进行认证
	apiKey := mm.decryptAPIKey(model.APIKeyEncrypted)
	if apiKey != "" {
		httpReq.Header.Set("x-api-key", apiKey)
	}

	resp, err := mm.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("claude http call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read claude response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var claudeResp claudeMessagesResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return nil, fmt.Errorf("unmarshal claude response: %w", err)
	}

	// 从 content 数组中提取所有 text 类型的内容块
	content := ""
	for _, block := range claudeResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	latency := int(time.Since(start).Milliseconds())

	return &CallResult{
		Response: ChatResponse{
			Content:      content,
			InputTokens:  claudeResp.Usage.InputTokens,
			OutputTokens: claudeResp.Usage.OutputTokens,
			ModelName:    model.Name,
		},
		ModelID:   model.ID,
		ModelName: model.Name,
		LatencyMs: latency,
	}, nil
}

// callOpenAICompatible 调用 OpenAI Chat Completions 兼容的 API。
// 适用于通义千问（DashScope）、DeepSeek、本地 Ollama 等实现了 OpenAI 兼容接口的模型。
// 认证方式使用标准的 Authorization: Bearer <token> 头。
func (mm *ModelManager) callOpenAICompatible(ctx context.Context, model *AIModel, req ChatRequest) (*CallResult, error) {
	start := time.Now()

	apiReq := openAIChatRequest{
		Model: mm.resolveModelName(model),
		Messages: []openAIMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserPrompt},
		},
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	if apiReq.Temperature == 0 {
		apiReq.Temperature = 0.3
	}
	if apiReq.MaxTokens == 0 {
		apiReq.MaxTokens = 2000
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := mm.resolveEndpoint(model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	apiKey := mm.decryptAPIKey(model.APIKeyEncrypted)
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := mm.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp openAIChatResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	content := ""
	if len(apiResp.Choices) > 0 {
		content = apiResp.Choices[0].Message.Content
	}

	latency := int(time.Since(start).Milliseconds())

	return &CallResult{
		Response: ChatResponse{
			Content:      content,
			InputTokens:  apiResp.Usage.PromptTokens,
			OutputTokens: apiResp.Usage.CompletionTokens,
			ModelName:    model.Name,
		},
		ModelID:   model.ID,
		ModelName: model.Name,
		LatencyMs: latency,
	}, nil
}

// callLocalModel 调用 Ollama 原生 Chat API（POST /api/chat）。
//
// Ollama API 特点：
//  1. 端点为 /api/chat，不同于 OpenAI 兼容格式
//  2. 不需要 API Key 认证（本地部署）
//  3. 响应中 message.content 直接包含文本内容
//  4. token 统计在 prompt_eval_count 和 eval_count 字段中
//  5. 需要设置 stream: false 来获取完整响应而非流式输出
func (mm *ModelManager) callLocalModel(ctx context.Context, model *AIModel, req ChatRequest) (*CallResult, error) {
	start := time.Now()

	// 构建 Ollama Chat API 请求体
	ollamaReq := ollamaChatRequest{
		Model: mm.resolveModelName(model),
		Messages: []openAIMessage{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: req.UserPrompt},
		},
		Stream: false, // 使用非流式模式，等待完整响应
	}

	// 设置可选参数
	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.3
	}
	ollamaReq.Options.Temperature = temperature
	if req.MaxTokens > 0 {
		ollamaReq.Options.NumPredict = req.MaxTokens
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal ollama request: %w", err)
	}

	// 构建 Ollama API 端点（默认 http://localhost:11434/api/chat）
	endpoint := model.LocalEndpoint
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	endpoint = endpoint + "/api/chat"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := mm.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama http call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ollama response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaChatResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("unmarshal ollama response: %w", err)
	}

	latency := int(time.Since(start).Milliseconds())

	return &CallResult{
		Response: ChatResponse{
			Content:      ollamaResp.Message.Content,
			InputTokens:  ollamaResp.PromptEvalCount,
			OutputTokens: ollamaResp.EvalCount,
			ModelName:    model.Name,
		},
		ModelID:   model.ID,
		ModelName: model.Name,
		LatencyMs: latency,
	}, nil
}

// resolveEndpoint 根据部署类型返回模型 API 端点。
// 本地模型使用 LocalEndpoint + /v1/chat/completions，云端模型直接使用 APIEndpoint。
func (mm *ModelManager) resolveEndpoint(model *AIModel) string {
	if model.DeploymentType == "local" && model.LocalEndpoint != "" {
		return model.LocalEndpoint + "/v1/chat/completions"
	}
	return model.APIEndpoint
}

// resolveModelName 返回请求体中使用的模型名称。
// 本地模型使用 LocalModelName（如 llama3），云端模型使用 Name。
func (mm *ModelManager) resolveModelName(model *AIModel) string {
	if model.DeploymentType == "local" && model.LocalModelName != "" {
		return model.LocalModelName
	}
	return model.Name
}

// decryptAPIKey 使用 AES-256-GCM 解密存储的 API 密钥。
// 加密密钥从环境变量 AI_KEY_ENCRYPTION_KEY 读取（32 字节 hex 编码）。
// 密文格式：nonce (12 bytes) || ciphertext+tag，整体 hex 编码存储在数据库中。
// 若环境变量未设置，则作为明文处理（向后兼容开发环境）。
func (mm *ModelManager) decryptAPIKey(encrypted []byte) string {
	// 空密钥直接返回
	if len(encrypted) == 0 {
		return ""
	}

	// 从环境变量读取 AES-256 加密密钥（64 位 hex 字符 = 32 字节）
	keyHex := os.Getenv("AI_KEY_ENCRYPTION_KEY")
	if keyHex == "" {
		// 未配置加密密钥 — 按明文处理（适用于开发环境）
		mm.logger.Debug("AI_KEY_ENCRYPTION_KEY not set, treating API key as plaintext")
		return string(encrypted)
	}

	// 解码 hex 格式的加密密钥，校验长度必须为 32 字节
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != 32 {
		mm.logger.Error("AI_KEY_ENCRYPTION_KEY must be 64 hex chars (32 bytes)", zap.Error(err))
		return ""
	}

	// 尝试将密文从 hex 解码，若非 hex 格式则视为明文（兼容迁移场景）
	ciphertext, err := hex.DecodeString(string(encrypted))
	if err != nil {
		mm.logger.Debug("API key is not hex-encoded, treating as plaintext")
		return string(encrypted)
	}

	// 创建 AES 分组密码
	block, err := aes.NewCipher(key)
	if err != nil {
		mm.logger.Error("failed to create AES cipher", zap.Error(err))
		return ""
	}

	// 创建 GCM（Galois/Counter Mode）认证加密实例
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		mm.logger.Error("failed to create GCM", zap.Error(err))
		return ""
	}

	// 从密文中分离 nonce（前 12 字节）和加密数据
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		mm.logger.Error("ciphertext too short for GCM nonce")
		return ""
	}

	nonce, cipherData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	// 执行 AES-256-GCM 解密并验证完整性（认证标签）
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		mm.logger.Error("AES-256-GCM decryption failed", zap.Error(err))
		return ""
	}

	return string(plaintext)
}

// checkBudget 检查指定模型当月的 token 预算是否已耗尽。
func (mm *ModelManager) checkBudget(ctx context.Context, modelID uuid.UUID) (bool, error) {
	month := time.Now().Format("2006-01")
	budget, err := mm.budgetRepo.GetOrCreate(ctx, month, modelID, 1000000) // default 1M tokens
	if err != nil {
		return false, err
	}
	return budget.Exhausted, nil
}

// recordUsage 记录模型调用的 token 用量，用于预算追踪。
func (mm *ModelManager) recordUsage(ctx context.Context, modelID uuid.UUID, tokens int) {
	month := time.Now().Format("2006-01")
	if err := mm.budgetRepo.IncrementUsage(ctx, month, modelID, int64(tokens)); err != nil {
		mm.logger.Warn("failed to record token usage", zap.Error(err))
	}
}

// ListModels 返回所有已配置的 AI 模型列表。
func (mm *ModelManager) ListModels(ctx context.Context) ([]*AIModel, error) {
	return mm.modelRepo.List(ctx)
}

// openAIChatRequest 是 OpenAI 兼容的聊天请求结构，也适用于本地 Ollama 等兼容 API。
type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

// openAIMessage 表示聊天消息中的单条消息。
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIChatResponse 是 OpenAI 兼容的聊天响应结构。
type openAIChatResponse struct {
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

// openAIChoice 表示模型输出的一个候选结果。
type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

// openAIUsage 记录 token 消耗统计。
type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// --- Claude Messages API 结构 ---

// claudeMessagesRequest 是 Anthropic Claude Messages API 的请求结构。
type claudeMessagesRequest struct {
	Model       string           `json:"model"`          // 模型标识（如 claude-sonnet-4-20250514）
	MaxTokens   int              `json:"max_tokens"`     // 最大输出 token 数
	System      string           `json:"system,omitempty"` // 系统提示词（Claude 独有的顶层参数，非 messages 数组内）
	Messages    []claudeMessage  `json:"messages"`       // 对话消息列表
	Temperature float64          `json:"temperature,omitempty"` // 采样温度，控制输出随机性
}

// claudeMessage 表示 Claude Messages API 中的单条消息。
type claudeMessage struct {
	Role    string `json:"role"`    // 消息角色：user 或 assistant
	Content string `json:"content"` // 消息文本内容
}

// claudeMessagesResponse 是 Claude Messages API 的响应结构。
type claudeMessagesResponse struct {
	ID      string               `json:"id"`      // 响应唯一标识
	Type    string               `json:"type"`    // 响应类型（通常为 "message"）
	Role    string               `json:"role"`    // 响应角色（通常为 "assistant"）
	Content []claudeContentBlock `json:"content"` // 内容块数组，Claude 返回多个内容块
	Model   string               `json:"model"`   // 实际使用的模型名称
	Usage   claudeUsage          `json:"usage"`   // token 消耗统计
}

// claudeContentBlock 表示 Claude 响应中的一个内容块。
type claudeContentBlock struct {
	Type string `json:"type"`           // 内容类型（"text" 为文本内容）
	Text string `json:"text,omitempty"` // 文本内容（仅 type="text" 时有值）
}

// claudeUsage 记录 Claude API 的 token 消耗。
type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`  // 输入 token 数（prompt 消耗）
	OutputTokens int `json:"output_tokens"` // 输出 token 数（completion 消耗）
}

// --- Ollama Chat API 结构 ---

// ollamaChatRequest 是 Ollama 原生 Chat API 的请求结构（POST /api/chat）。
type ollamaChatRequest struct {
	Model    string          `json:"model"`             // 模型名称（如 llama3、qwen2 等）
	Messages []openAIMessage `json:"messages"`          // 对话消息列表（复用 OpenAI 消息格式）
	Stream   bool            `json:"stream"`            // 是否流式输出，设为 false 获取完整响应
	Options  ollamaOptions   `json:"options,omitempty"` // 模型推理参数
}

// ollamaOptions 包含 Ollama 模型推理的可选参数。
type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"` // 采样温度，控制输出随机性
	NumPredict  int     `json:"num_predict,omitempty"` // 最大生成 token 数（等同于 max_tokens）
}

// ollamaChatResponse 是 Ollama Chat API 的响应结构。
type ollamaChatResponse struct {
	Model           string       `json:"model"`             // 使用的模型名称
	Message         openAIMessage `json:"message"`           // 模型生成的消息
	Done            bool         `json:"done"`              // 是否已完成生成
	PromptEvalCount int          `json:"prompt_eval_count"` // 输入 token 数
	EvalCount       int          `json:"eval_count"`        // 输出 token 数
}
