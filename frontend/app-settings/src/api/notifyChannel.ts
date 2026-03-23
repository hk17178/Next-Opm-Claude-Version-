/**
 * 通知通道管理 API 模块
 * 提供通知通道的增删改查和测试接口
 * 支持企微、飞书、钉钉、SMS、邮件、Webhook、Slack 等通道类型
 * 对应后端 svc-notify 服务，路由前缀 /v1/notify
 */
import { request } from './request';

// ==================== 类型定义 ====================

/** 通知通道类型 */
export type ChannelType = 'email' | 'sms' | 'webhook' | 'dingtalk' | 'wecom' | 'feishu' | 'slack';

/** 通知通道数据结构 */
export interface NotifyChannel {
  /** 通道唯一 ID */
  id: string;
  /** 通道名称 */
  name: string;
  /** 通道类型 */
  type: ChannelType;
  /** 是否启用 */
  enabled: boolean;
  /** 通道配置（不同类型配置项不同） */
  config: Record<string, unknown>;
  /** 创建时间 */
  createdAt: string;
}

/** 创建通道请求参数 */
export interface ChannelCreatePayload {
  /** 通道名称 */
  name: string;
  /** 通道类型 */
  type: ChannelType;
  /** 通道配置 */
  config: Record<string, unknown>;
}

/** 更新通道请求参数 */
export interface ChannelUpdatePayload {
  /** 通道名称 */
  name?: string;
  /** 是否启用 */
  enabled?: boolean;
  /** 通道配置 */
  config?: Record<string, unknown>;
}

// ==================== API 函数 ====================

/**
 * 获取所有通知通道列表
 * @returns 通道列表数组
 */
export function listChannels(): Promise<NotifyChannel[]> {
  return request<NotifyChannel[]>('/v1/notify/channels');
}

/**
 * 创建新通知通道
 * 支持企微/飞书/钉钉/SMS/邮件/Webhook/Slack
 * @param data - 通道创建参数
 * @returns 创建成功的通道数据
 */
export function createChannel(data: ChannelCreatePayload): Promise<NotifyChannel> {
  return request<NotifyChannel>('/v1/notify/channels', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * 更新通知通道配置
 * @param id - 通道 ID
 * @param data - 待更新的通道配置
 * @returns 更新后的通道数据
 */
export function updateChannel(id: string, data: ChannelUpdatePayload): Promise<NotifyChannel> {
  return request<NotifyChannel>(`/v1/notify/channels/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

/**
 * 删除通知通道
 * @param id - 通道 ID
 */
export function deleteChannel(id: string): Promise<void> {
  return request<void>(`/v1/notify/channels/${id}`, {
    method: 'DELETE',
  });
}

/**
 * 测试通知通道
 * 通过该通道发送一条测试消息，验证通道配置是否正确
 * @param id - 通道 ID
 * @returns 测试结果
 */
export function testChannel(id: string): Promise<{ success: boolean; error?: string }> {
  return request(`/v1/notify/channels/${id}/test`, {
    method: 'POST',
  });
}
