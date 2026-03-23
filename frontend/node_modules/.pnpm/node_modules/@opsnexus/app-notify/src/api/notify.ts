/**
 * 通知管理 API 模块 - 封装通知渠道及通知记录相关的所有接口调用
 * 使用统一的 request 封装，自动处理响应解析和错误码判断
 */
import { request } from './request';

/** 通知渠道 */
export interface Channel {
  key: string;                         // 渠道唯一标识
  name: string;                        // 渠道名称
  type: string;                        // 渠道类型（wecom_webhook/email/sms 等）
  enabled: boolean;                    // 是否启用
  health: string;                      // 健康状态（healthy/degraded/unavailable）
  lastCheckTime: string;               // 最后健康检查时间
  description?: string;                // 渠道描述（可选）
  config?: Record<string, unknown>;    // 渠道配置详情（可选）
}

/** 通知发送记录 */
export interface NotifyRecord {
  key: string;                 // 记录唯一标识
  time: string;                // 发送时间
  receiver: string;            // 接收人
  channel: string;             // 渠道名称
  channelType: string;         // 渠道类型
  contentSummary: string;      // 内容摘要
  fullContent?: string;        // 完整内容（可选）
  status: string;              // 发送状态（success/fail/pending）
  retryCount: number;          // 重试次数
  errorMessage?: string;       // 错误信息（可选，发送失败时）
  relatedAlertId?: string;     // 关联告警 ID（可选）
  relatedIncidentId?: string;  // 关联事件 ID（可选）
}

/** 通知历史查询参数 */
export interface NotifyHistoryParams {
  page?: number;        // 当前页码
  pageSize?: number;    // 每页条数
  channelType?: string; // 渠道类型过滤
  status?: string;      // 状态过滤
}

/** 通知历史接口返回数据结构 */
export interface NotifyHistoryResult {
  list: NotifyRecord[]; // 通知记录列表
  total: number;        // 总记录数
}

/**
 * 获取通知渠道列表
 * 调用 GET /api/notify/channels 接口
 * @returns 渠道列表
 */
export async function fetchChannels(): Promise<Channel[]> {
  return request<Channel[]>('/notify/channels');
}

/**
 * 创建通知渠道
 * 调用 POST /api/notify/channels 接口
 * @param data 渠道配置信息
 * @returns 创建后的渠道记录
 */
export async function createChannel(data: Partial<Channel>): Promise<Channel> {
  return request<Channel>('/notify/channels', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * 更新通知渠道
 * 调用 PUT /api/notify/channels/:key 接口
 * @param key 渠道标识
 * @param data 更新的渠道信息
 * @returns 更新后的渠道记录
 */
export async function updateChannel(key: string, data: Partial<Channel>): Promise<Channel> {
  return request<Channel>(`/notify/channels/${key}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

/**
 * 删除通知渠道
 * 调用 DELETE /api/notify/channels/:key 接口
 * @param key 渠道标识
 * @returns void（无返回数据）
 */
export async function deleteChannel(key: string): Promise<void> {
  return request<void>(`/notify/channels/${key}`, { method: 'DELETE' });
}

/**
 * 测试通知渠道（发送测试通知）
 * 调用 POST /api/notify/channels/:key/test 接口
 * @param key 渠道标识
 * @returns 测试结果（成功/失败信息）
 */
export async function testChannel(key: string): Promise<{ success: boolean; message: string }> {
  return request<{ success: boolean; message: string }>(`/notify/channels/${key}/test`, {
    method: 'POST',
  });
}

/**
 * 获取通知发送历史记录
 * 调用 GET /api/notify/history 接口
 * @param params 查询参数（分页、渠道类型、状态过滤）
 * @returns 通知记录列表及总数
 */
export async function fetchNotifyHistory(params: NotifyHistoryParams = {}): Promise<NotifyHistoryResult> {
  const query = new URLSearchParams();
  // 遍历参数，将非空值添加到查询字符串
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null) query.set(k, String(v));
  });
  return request<NotifyHistoryResult>(`/notify/history?${query.toString()}`);
}

/**
 * 重试失败的通知
 * 调用 POST /api/notify/history/:key/retry 接口
 * @param key 通知记录标识
 * @returns 重试后的通知记录
 */
export async function retryNotification(key: string): Promise<NotifyRecord> {
  return request<NotifyRecord>(`/notify/history/${key}/retry`, { method: 'POST' });
}
