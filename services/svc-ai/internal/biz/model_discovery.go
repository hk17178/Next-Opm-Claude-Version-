package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// ModelDiscovery 负责自动探测本地部署的 AI 模型服务，
// 支持 Ollama 和 vLLM 两种常见的本地推理引擎。
type ModelDiscovery struct {
	httpClient *http.Client
	logger     *zap.Logger
}

// NewModelDiscovery 创建模型探测器实例，设置 HTTP 客户端超时时间为 10 秒。
func NewModelDiscovery(logger *zap.Logger) *ModelDiscovery {
	return &ModelDiscovery{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// ollamaTagsResponse 是 Ollama GET /api/tags 接口的响应结构。
type ollamaTagsResponse struct {
	Models []ollamaModelInfo `json:"models"` // 可用模型列表
}

// ollamaModelInfo 是 Ollama 模型信息条目。
type ollamaModelInfo struct {
	Name       string `json:"name"`        // 模型名称（如 llama3:latest）
	Model      string `json:"model"`       // 模型标识
	ModifiedAt string `json:"modified_at"` // 最后修改时间
	Size       int64  `json:"size"`        // 模型文件大小（字节）
}

// vllmModelsResponse 是 vLLM GET /v1/models 接口的响应结构（OpenAI 兼容格式）。
type vllmModelsResponse struct {
	Object string          `json:"object"` // 固定为 "list"
	Data   []vllmModelInfo `json:"data"`   // 模型信息数组
}

// vllmModelInfo 是 vLLM 模型信息条目（OpenAI Model 对象格式）。
type vllmModelInfo struct {
	ID      string `json:"id"`       // 模型 ID（如 Qwen/Qwen2-7B）
	Object  string `json:"object"`   // 固定为 "model"
	OwnedBy string `json:"owned_by"` // 所有者标识
}

// DiscoverOllamaModels 通过调用 Ollama 的 GET /api/tags 接口，自动探测可用的本地模型列表。
//
// 参数：
//   - baseURL: Ollama 服务的基础地址（如 http://localhost:11434）
//
// 返回：
//   - 可用模型名称列表（如 ["llama3:latest", "qwen2:7b"]）
//   - 错误信息（连接失败、解析失败等）
func (d *ModelDiscovery) DiscoverOllamaModels(ctx context.Context, baseURL string) ([]string, error) {
	endpoint := baseURL + "/api/tags"

	d.logger.Info("正在探测 Ollama 模型", zap.String("endpoint", endpoint))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Ollama 探测请求失败: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 Ollama 服务失败 (%s): %w", baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Ollama 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama 返回非 200 状态码 %d: %s", resp.StatusCode, string(body))
	}

	var tagsResp ollamaTagsResponse
	if err := json.Unmarshal(body, &tagsResp); err != nil {
		return nil, fmt.Errorf("解析 Ollama 模型列表失败: %w", err)
	}

	models := make([]string, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		models = append(models, m.Name)
	}

	d.logger.Info("Ollama 模型探测完成",
		zap.String("base_url", baseURL),
		zap.Int("model_count", len(models)),
		zap.Strings("models", models),
	)

	return models, nil
}

// DiscoverVLLMModels 通过调用 vLLM 的 GET /v1/models 接口，自动探测可用的本地模型列表。
// vLLM 使用 OpenAI 兼容的 API 格式。
//
// 参数：
//   - baseURL: vLLM 服务的基础地址（如 http://localhost:8000）
//
// 返回：
//   - 可用模型 ID 列表（如 ["Qwen/Qwen2-7B", "meta-llama/Llama-3-8B"]）
//   - 错误信息（连接失败、解析失败等）
func (d *ModelDiscovery) DiscoverVLLMModels(ctx context.Context, baseURL string) ([]string, error) {
	endpoint := baseURL + "/v1/models"

	d.logger.Info("正在探测 vLLM 模型", zap.String("endpoint", endpoint))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 vLLM 探测请求失败: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 vLLM 服务失败 (%s): %w", baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 vLLM 响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vLLM 返回非 200 状态码 %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp vllmModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("解析 vLLM 模型列表失败: %w", err)
	}

	models := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, m.ID)
	}

	d.logger.Info("vLLM 模型探测完成",
		zap.String("base_url", baseURL),
		zap.Int("model_count", len(models)),
		zap.Strings("models", models),
	)

	return models, nil
}
