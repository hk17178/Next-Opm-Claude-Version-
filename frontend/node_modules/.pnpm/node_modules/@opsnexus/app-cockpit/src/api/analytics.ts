/**
 * 分析指标 API 客户端
 * 封装驾驶舱面板所需的 SLA、MTTR 等运维指标接口
 */
import { request } from './request';

/** SLA 摘要数据结构 */
export interface SLASummary {
  /** 当前 SLA 达成率（百分比，如 99.92） */
  rate: number;
  /** 统计周期（如 "monthly" / "weekly"） */
  period?: string;
  /** 是否达标 */
  met: boolean;
}

/** 驾驶舱仪表盘指标数据结构 */
export interface DashboardMetrics {
  /** 平均修复时间（分钟） */
  mttr: number;
  /** 今日已解决事件数 */
  today_resolved: number;
  /** 今日告警总数 */
  today_alerts: number;
  /** 已抑制告警数 */
  suppressed: number;
}

/**
 * 获取 SLA 达成率摘要
 * 调用 GET /analytics/sla 接口
 * @returns SLA 摘要数据
 */
export async function getSLASummary(): Promise<SLASummary> {
  return request<SLASummary>('/analytics/sla');
}

/**
 * 获取驾驶舱仪表盘指标
 * 调用 GET /analytics/dashboard 接口
 * 返回 MTTR、今日解决数、今日告警数、已抑制数等关键指标
 * @returns 仪表盘指标数据
 */
export async function getDashboardMetrics(): Promise<DashboardMetrics> {
  return request<DashboardMetrics>('/analytics/dashboard');
}
