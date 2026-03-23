package biz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

// TestDiscoverOllamaModels_Success 验证成功探测 Ollama 模型列表。
func TestDiscoverOllamaModels_Success(t *testing.T) {
	// 模拟 Ollama /api/tags 端点
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("期望请求路径 /api/tags，实际 %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("期望 GET 方法，实际 %s", r.Method)
		}

		resp := ollamaTagsResponse{
			Models: []ollamaModelInfo{
				{Name: "llama3:latest", Size: 4700000000},
				{Name: "qwen2:7b", Size: 3900000000},
				{Name: "codestral:latest", Size: 12000000000},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	discovery := NewModelDiscovery(logger)

	models, err := discovery.DiscoverOllamaModels(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("探测 Ollama 模型失败: %v", err)
	}

	if len(models) != 3 {
		t.Fatalf("期望 3 个模型，实际 %d 个", len(models))
	}

	expected := []string{"llama3:latest", "qwen2:7b", "codestral:latest"}
	for i, name := range expected {
		if models[i] != name {
			t.Errorf("模型[%d]: 期望 %q，实际 %q", i, name, models[i])
		}
	}
}

// TestDiscoverOllamaModels_EmptyList 验证 Ollama 无模型时返回空列表。
func TestDiscoverOllamaModels_EmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaTagsResponse{Models: []ollamaModelInfo{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	discovery := NewModelDiscovery(logger)

	models, err := discovery.DiscoverOllamaModels(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("探测失败: %v", err)
	}

	if len(models) != 0 {
		t.Errorf("期望空列表，实际 %d 个模型", len(models))
	}
}

// TestDiscoverOllamaModels_ConnectionError 验证连接失败时返回错误。
func TestDiscoverOllamaModels_ConnectionError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	discovery := NewModelDiscovery(logger)

	_, err := discovery.DiscoverOllamaModels(context.Background(), "http://localhost:99999")
	if err == nil {
		t.Fatal("期望返回连接错误")
	}
}

// TestDiscoverVLLMModels_Success 验证成功探测 vLLM 模型列表。
func TestDiscoverVLLMModels_Success(t *testing.T) {
	// 模拟 vLLM /v1/models 端点（OpenAI 兼容格式）
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("期望请求路径 /v1/models，实际 %s", r.URL.Path)
		}

		resp := vllmModelsResponse{
			Object: "list",
			Data: []vllmModelInfo{
				{ID: "Qwen/Qwen2-7B", Object: "model", OwnedBy: "vllm"},
				{ID: "meta-llama/Llama-3-8B", Object: "model", OwnedBy: "vllm"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	discovery := NewModelDiscovery(logger)

	models, err := discovery.DiscoverVLLMModels(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("探测 vLLM 模型失败: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("期望 2 个模型，实际 %d 个", len(models))
	}

	if models[0] != "Qwen/Qwen2-7B" {
		t.Errorf("模型[0]: 期望 %q，实际 %q", "Qwen/Qwen2-7B", models[0])
	}
	if models[1] != "meta-llama/Llama-3-8B" {
		t.Errorf("模型[1]: 期望 %q，实际 %q", "meta-llama/Llama-3-8B", models[1])
	}
}

// TestDiscoverVLLMModels_ServerError 验证服务端错误时返回错误。
func TestDiscoverVLLMModels_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	discovery := NewModelDiscovery(logger)

	_, err := discovery.DiscoverVLLMModels(context.Background(), server.URL)
	if err == nil {
		t.Fatal("期望返回服务端错误")
	}
}
