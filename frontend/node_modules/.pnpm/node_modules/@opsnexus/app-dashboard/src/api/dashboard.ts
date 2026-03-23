import { request } from './request';

/** 仪表板概览汇总数据 */
export interface DashboardSummary {
  /** 当前活跃告警数量 */
  activeAlerts: number;
  /** 今日事件数量 */
  todayEvents: number;
  /** 最近 24 小时日志总量 */
  logVolume24h: number;
  /** 服务健康率（百分比，如 97.5） */
  serviceHealthRate: number;
}

/** 告警趋势数据点（用于 7 天趋势折线图） */
export interface AlertTrendPoint {
  /** 日期标签（如 "03-22"） */
  date: string;
  /** 当日告警数量 */
  count: number;
}

/** 最近未处理告警记录 */
export interface RecentAlert {
  /** 告警唯一标识 */
  id: string;
  /** 严重程度：P0-P3 */
  severity: string;
  /** 告警内容描述 */
  content: string;
  /** 告警来源 */
  source: string;
  /** 触发时间 */
  triggerTime: string;
  /** 当前状态 */
  status: string;
}

/**
 * 获取仪表板概览汇总数据
 * 调用 GET /api/dashboard/summary 接口
 * @returns 4 个 KPI 指标：活跃告警、今日事件、日志量、服务健康率
 */
export async function fetchDashboardSummary(): Promise<DashboardSummary> {
  return request<DashboardSummary>('/dashboard/summary');
}

/**
 * 获取最近 7 天告警趋势数据
 * 调用 GET /api/dashboard/alert-trend 接口，用于趋势折线图
 * @returns 每日告警数量数组
 */
export async function fetchAlertTrend7d(): Promise<AlertTrendPoint[]> {
  return request<AlertTrendPoint[]>('/dashboard/alert-trend');
}

/**
 * 获取最近 10 条未处理告警
 * 调用 GET /api/dashboard/recent-alerts 接口
 * @returns 最新告警列表
 */
export async function fetchRecentAlerts(): Promise<RecentAlert[]> {
  return request<RecentAlert[]>('/dashboard/recent-alerts');
}
