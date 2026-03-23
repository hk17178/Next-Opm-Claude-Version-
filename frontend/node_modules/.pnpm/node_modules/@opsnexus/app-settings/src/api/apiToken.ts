/**
 * API Token 管理模块
 * 提供 Token 的列表查询、生成、吊销接口
 * 对应后端 svc-auth 服务，路由前缀 /v1/auth/tokens
 */
import { request } from './request';

// ==================== 类型定义 ====================

/** Token 权限范围 */
export type TokenScope =
  | 'read:alerts'
  | 'write:alerts'
  | 'read:metrics'
  | 'read:logs'
  | 'write:config'
  | 'read:config'
  | 'admin';

/** Token 状态 */
export type TokenStatus = 'active' | 'expired' | 'revoked';

/** API Token 数据结构 */
export interface APIToken {
  /** Token 唯一 ID */
  id: string;
  /** Token 名称（用户自定义标识） */
  name: string;
  /** 权限范围列表 */
  scopes: TokenScope[];
  /** 过期时间（ISO 8601 格式，null 表示永不过期） */
  expiresAt: string | null;
  /** 最后使用时间（null 表示从未使用） */
  lastUsedAt: string | null;
  /** 累计调用次数 */
  callCount: number;
  /** Token 状态 */
  status: TokenStatus;
  /** 创建时间 */
  createdAt: string;
}

/** 生成 Token 请求参数 */
export interface GenerateTokenPayload {
  /** Token 名称 */
  name: string;
  /** 权限范围列表 */
  scopes: TokenScope[];
  /** 过期天数：30/90/365/0（0 表示永不过期） */
  expireDays: number;
}

/** 生成 Token 响应（包含明文 Token，仅返回一次） */
export interface GenerateTokenResult {
  /** Token 元数据 */
  token: APIToken;
  /** 明文 Token（仅此一次可见） */
  plainToken: string;
}

// ==================== API 函数 ====================

/**
 * 获取 API Token 列表
 * @returns Token 列表数组
 */
export function listTokens(): Promise<APIToken[]> {
  return request<APIToken[]>('/v1/auth/tokens');
}

/**
 * 生成新的 API Token
 * 生成后返回明文 Token，仅此一次可见，需提示用户立即复制保存
 * @param data - Token 生成参数（名称、权限范围、过期时间）
 * @returns 包含明文 Token 的生成结果
 */
export function generateToken(data: GenerateTokenPayload): Promise<GenerateTokenResult> {
  return request<GenerateTokenResult>('/v1/auth/tokens', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * 吊销指定 API Token
 * 吊销后该 Token 立即失效，不可恢复
 * @param id - Token ID
 */
export function revokeToken(id: string): Promise<void> {
  return request<void>(`/v1/auth/tokens/${id}`, {
    method: 'DELETE',
  });
}
