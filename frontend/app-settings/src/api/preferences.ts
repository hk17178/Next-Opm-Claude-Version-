/**
 * 个人通知偏好 API 模块
 *
 * 对接后端 /api/v1/notifications/preferences 接口
 * 管理用户的通知级别、静默时段、通知渠道、报告推送频率等偏好设置
 */
import { request } from './request';

/**
 * 通知偏好数据结构
 */
export interface NotificationPreferences {
  /** 各告警级别的接收开关 */
  levelSettings: {
    /** P0 级别（最高严重级别）是否接收通知 */
    P0: boolean;
    /** P1 级别是否接收通知 */
    P1: boolean;
    /** P2 级别是否接收通知 */
    P2: boolean;
    /** P3 级别是否接收通知 */
    P3: boolean;
    /** P4 级别（最低严重级别）是否接收通知 */
    P4: boolean;
  };
  /** 静默时段设置 */
  silentPeriod: {
    /** 是否启用静默时段 */
    enabled: boolean;
    /** 静默开始时间（格式：HH:mm） */
    startTime: string;
    /** 静默结束时间（格式：HH:mm） */
    endTime: string;
  };
  /** 偏好通知渠道（多选） */
  channels: ('wecom' | 'email' | 'sms')[];
  /** 报告推送频率 */
  reportFrequency: 'realtime' | 'daily' | 'weekly';
}

/**
 * 获取当前用户的通知偏好设置
 * @returns 通知偏好数据
 */
export async function getPreferences(): Promise<NotificationPreferences> {
  return request<NotificationPreferences>('/v1/notifications/preferences');
}

/**
 * 更新当前用户的通知偏好设置
 * @param data - 更新的偏好数据
 * @returns 更新后的偏好数据
 */
export async function updatePreferences(
  data: NotificationPreferences,
): Promise<NotificationPreferences> {
  return request<NotificationPreferences>('/v1/notifications/preferences', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}
