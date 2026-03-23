/**
 * 会话管理 API 模块
 * 提供活跃会话查询、单个踢出、批量踢出接口
 * 对应后端 svc-auth 服务，路由前缀 /v1/auth/sessions
 */
import { request } from './request';

// ==================== 类型定义 ====================

/** 活跃会话数据结构 */
export interface Session {
  /** 会话唯一 ID */
  id: string;
  /** 用户名 */
  username: string;
  /** 登录 IP 地址 */
  ip: string;
  /** 浏览器 User-Agent 描述（如 Chrome 120 / Windows） */
  userAgent: string;
  /** 登录时间（ISO 8601） */
  loginAt: string;
  /** 最后活跃时间（ISO 8601） */
  lastActiveAt: string;
  /** 是否为当前会话（用于高亮标记） */
  isCurrent: boolean;
}

// ==================== API 函数 ====================

/**
 * 获取所有活跃会话列表
 * 返回当前系统中所有未过期的会话，包含当前会话标记
 * @returns 会话列表数组
 */
export function listSessions(): Promise<Session[]> {
  return request<Session[]>('/v1/auth/sessions');
}

/**
 * 踢出指定会话
 * 强制注销指定会话，该会话对应的用户将被要求重新登录
 * @param id - 会话 ID
 */
export function revokeSession(id: string): Promise<void> {
  return request<void>(`/v1/auth/sessions/${id}`, {
    method: 'DELETE',
  });
}

/**
 * 踢出所有其他会话
 * 保留当前会话，强制注销其他所有活跃会话
 */
export function revokeAll(): Promise<void> {
  return request<void>('/v1/auth/sessions/revoke-all', {
    method: 'POST',
  });
}
