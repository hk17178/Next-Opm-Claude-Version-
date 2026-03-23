/**
 * AI 模型管理 API 模块
 * 提供模型注册、更新、删除、测试连通性、场景绑定、用量统计、本地模型探测等接口
 * 对应后端 svc-ai 服务，路由前缀 /v1/ai
 */
import { request } from './request';

// ==================== 类型定义 ====================

/** AI 模型供应商类型 */
export type ModelProvider =
  | 'openai'
  | 'anthropic'
  | 'qwen'
  | 'deepseek'
  | 'ollama'
  | 'vllm'
  | 'azure_openai'
  | 'custom';

/** 模型部署类型：云端 API 或本地部署 */
export type ModelDeployType = 'cloud' | 'local';

/** 模型运行状态 */
export type ModelStatus = 'active' | 'standby' | 'error';

/** AI 模型数据结构 */
export interface AIModel {
  /** 模型唯一 ID */
  id: string;
  /** 模型显示名称 */
  name: string;
  /** 供应商标识 */
  provider: ModelProvider;
  /** 部署类型 */
  deployType: ModelDeployType;
  /** 模型标识符（如 gpt-4o、qwen-plus） */
  modelId: string;
  /** API 端点地址 */
  baseURL: string;
  /** API 密钥（仅云端模型需要，返回时脱敏） */
  apiKey?: string;
  /** 模型运行状态 */
  status: ModelStatus;
  /** 今日调用次数 */
  todayCalls: number;
  /** 今日 Token 用量 */
  todayTokens: number;
  /** 最大 Token 上限 */
  maxTokens: number;
  /** Temperature 参数 */
  temperature: number;
  /** 请求超时时间（秒） */
  timeout: number;
  /** 创建时间 */
  createdAt: string;
  /** 更新时间 */
  updatedAt: string;
}

/** 创建/更新模型请求参数 */
export interface AIModelPayload {
  /** 模型显示名称 */
  name: string;
  /** 供应商标识 */
  provider: ModelProvider;
  /** 部署类型 */
  deployType: ModelDeployType;
  /** 模型标识符 */
  modelId: string;
  /** API 端点地址 */
  baseURL: string;
  /** API 密钥（仅云端模型需要） */
  apiKey?: string;
  /** 最大 Token 上限 */
  maxTokens?: number;
  /** Temperature 参数 */
  temperature?: number;
  /** 请求超时时间（秒） */
  timeout?: number;
}

/** AI 使用场景枚举 */
export type AIScenario =
  | 'root_cause'
  | 'anomaly_detection'
  | 'prediction'
  | 'correlation'
  | 'log_summary'
  | 'alert_noise_reduction';

/** 场景绑定关系 */
export interface ScenarioBinding {
  /** 场景标识 */
  scenario: AIScenario;
  /** 场景显示名称 */
  scenarioName: string;
  /** 主模型名称 */
  primaryModel: string;
  /** 主模型 ID */
  primaryModelId: string;
  /** 备选模型名称 */
  backupModel: string;
  /** 备选模型 ID */
  backupModelId: string;
  /** 当前 Prompt 版本号 */
  promptVersion: string;
  /** AI 建议采纳率（百分比） */
  approvalRate: number;
}

/** 更新场景绑定请求参数 */
export interface ScenarioBindingPayload {
  /** 主模型 ID */
  primaryModelId: string;
  /** 备选模型 ID */
  backupModelId?: string;
}

/** Token 用量统计数据 */
export interface UsageStats {
  /** 今日总 Token 用量 */
  todayTokens: number;
  /** 今日总调用次数 */
  todayCalls: number;
  /** 本月总 Token 用量 */
  monthTokens: number;
  /** 本月总调用次数 */
  monthCalls: number;
  /** 按模型分组的用量明细 */
  byModel: Array<{
    /** 模型 ID */
    modelId: string;
    /** 模型名称 */
    modelName: string;
    /** Token 用量 */
    tokens: number;
    /** 调用次数 */
    calls: number;
  }>;
}

/** 本地探测到的模型信息 */
export interface DiscoveredModel {
  /** 模型标识符 */
  modelId: string;
  /** 模型显示名称 */
  name: string;
  /** 模型大小（如 7B、13B） */
  size?: string;
}

// ==================== API 函数 ====================

/**
 * 获取已注册 AI 模型列表
 * @returns 模型列表数组
 */
export function listModels(): Promise<AIModel[]> {
  return request<AIModel[]>('/v1/ai/models');
}

/**
 * 注册新 AI 模型
 * 支持云端模型（OpenAI/Anthropic/Qwen 等）和本地模型（Ollama/vLLM）
 * @param data - 模型注册参数
 * @returns 创建成功的模型数据
 */
export function createModel(data: AIModelPayload): Promise<AIModel> {
  return request<AIModel>('/v1/ai/models', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * 更新已注册模型的配置
 * @param id - 模型 ID
 * @param data - 待更新的模型配置
 * @returns 更新后的模型数据
 */
export function updateModel(id: string, data: Partial<AIModelPayload>): Promise<AIModel> {
  return request<AIModel>(`/v1/ai/models/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

/**
 * 删除已注册模型
 * @param id - 模型 ID
 */
export function deleteModel(id: string): Promise<void> {
  return request<void>(`/v1/ai/models/${id}`, {
    method: 'DELETE',
  });
}

/**
 * 测试模型连通性
 * 向目标模型发送测试请求，验证 API 密钥和端点是否可用
 * @param id - 模型 ID
 * @returns 测试结果，包含延迟和状态信息
 */
export function testModel(id: string): Promise<{ success: boolean; latencyMs: number; error?: string }> {
  return request('/v1/ai/models/' + id + '/test', {
    method: 'POST',
  });
}

/**
 * 获取场景绑定关系列表
 * @returns 场景绑定数组
 */
export function listScenarioBindings(): Promise<ScenarioBinding[]> {
  return request<ScenarioBinding[]>('/v1/ai/scenarios');
}

/**
 * 更新指定场景的模型绑定
 * @param scenario - 场景标识
 * @param data - 绑定参数（主模型/备选模型 ID）
 */
export function updateScenarioBinding(scenario: AIScenario, data: ScenarioBindingPayload): Promise<void> {
  return request<void>(`/v1/ai/scenarios/${scenario}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

/**
 * 获取 Token 用量统计
 * @returns 用量统计数据
 */
export function getUsageStats(): Promise<UsageStats> {
  return request<UsageStats>('/v1/ai/usage/stats');
}

/**
 * 自动探测本地部署的模型（Ollama / vLLM）
 * 对应功能需求 FR-14-014：本地模型自动发现
 * @param baseURL - 本地模型服务地址（如 http://localhost:11434）
 * @param provider - 本地模型供应商（ollama 或 vllm）
 * @returns 探测到的模型列表
 */
export function discoverLocalModels(
  baseURL: string,
  provider: 'ollama' | 'vllm',
): Promise<DiscoveredModel[]> {
  return request<DiscoveredModel[]>('/v1/ai/models/discover', {
    method: 'POST',
    body: JSON.stringify({ baseURL, provider }),
  });
}
