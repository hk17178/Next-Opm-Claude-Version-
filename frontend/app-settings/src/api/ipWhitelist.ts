/**
 * IP 白名单管理 API 模块
 * 提供白名单开关配置、规则增删查接口
 * 对应后端 svc-auth 服务，路由前缀 /v1/auth/ip-whitelist
 */
import { request } from './request';

// ==================== 类型定义 ====================

/** IP 白名单全局配置 */
export interface IPWhitelistConfig {
  /** 是否启用白名单（禁用时所有 IP 可访问） */
  enabled: boolean;
}

/** IP 规则类型：永久生效 / 临时生效 */
export type RuleType = 'permanent' | 'temporary';

/** IP 白名单规则数据结构 */
export interface IPRule {
  /** 规则唯一 ID */
  id: string;
  /** IP 地址或 CIDR 网段（如 192.168.1.0/24） */
  ip: string;
  /** 备注说明 */
  remark: string;
  /** 规则类型 */
  type: RuleType;
  /** 过期时间（仅临时规则有值，ISO 8601 格式） */
  expiresAt: string | null;
  /** 创建时间 */
  createdAt: string;
}

/** 添加 IP 规则请求参数 */
export interface AddRulePayload {
  /** IP 地址或 CIDR 网段 */
  ip: string;
  /** 备注说明 */
  remark: string;
  /** 规则类型 */
  type: RuleType;
  /** 过期时间（仅临时规则需要，ISO 8601 格式） */
  expiresAt?: string;
}

// ==================== API 函数 ====================

/**
 * 获取 IP 白名单全局配置
 * @returns 白名单启用状态
 */
export function getConfig(): Promise<IPWhitelistConfig> {
  return request<IPWhitelistConfig>('/v1/auth/ip-whitelist/config');
}

/**
 * 更新 IP 白名单全局配置（启用/禁用）
 * @param enabled - 是否启用白名单
 */
export function updateConfig(enabled: boolean): Promise<void> {
  return request<void>('/v1/auth/ip-whitelist/config', {
    method: 'PUT',
    body: JSON.stringify({ enabled }),
  });
}

/**
 * 获取 IP 白名单规则列表
 * @returns 规则列表数组
 */
export function listRules(): Promise<IPRule[]> {
  return request<IPRule[]>('/v1/auth/ip-whitelist/rules');
}

/**
 * 添加 IP 白名单规则
 * 支持单个 IP 和 CIDR 网段格式
 * @param data - 规则参数
 * @returns 创建成功的规则数据
 */
export function addRule(data: AddRulePayload): Promise<IPRule> {
  return request<IPRule>('/v1/auth/ip-whitelist/rules', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * 删除 IP 白名单规则
 * @param id - 规则 ID
 */
export function deleteRule(id: string): Promise<void> {
  return request<void>(`/v1/auth/ip-whitelist/rules/${id}`, {
    method: 'DELETE',
  });
}
