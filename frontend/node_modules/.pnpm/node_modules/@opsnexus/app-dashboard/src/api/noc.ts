/**
 * NOC 运维大屏 API 模块
 * 封装 NOC 大屏相关的所有接口调用，包括系统状态、业务健康矩阵、资源指标
 */
import { request } from './request';

/** 全局系统状态数据 */
export interface SystemStatus {
  /** 系统整体健康状态：normal / degraded / critical */
  health: 'normal' | 'degraded' | 'critical';
  /** 当前活跃事件数量 */
  activeIncidents: number;
  /** 今日告警总数 */
  todayAlerts: number;
  /** 当前 SLA 达成率（百分比，如 99.92） */
  slaRate: number;
  /** 在线服务数量 */
  onlineServices: number;
  /** 服务总数 */
  totalServices: number;
}

/** 业务健康矩阵单元格数据 */
export interface BusinessHealthCell {
  /** 业务名称 */
  name: string;
  /** 健康度分值（0-100） */
  score: number;
  /** 健康状态：healthy / warning / degraded / critical */
  status: 'healthy' | 'warning' | 'degraded' | 'critical';
  /** 当前活跃告警数 */
  activeAlerts: number;
}

/** 资源指标数据点 */
export interface ResourceMetricPoint {
  /** 时间戳 */
  timestamp: string;
  /** CPU 使用率（百分比） */
  cpu: number;
  /** 内存使用率（百分比） */
  memory: number;
  /** 网络流量（Mbps） */
  network: number;
}

/** 实时告警条目（用于瀑布流展示） */
export interface RealtimeAlert {
  /** 告警唯一标识 */
  id: string;
  /** 严重级别：P0-P3 */
  severity: string;
  /** 告警内容 */
  content: string;
  /** 告警来源 */
  source: string;
  /** 触发时间 */
  triggerTime: string;
  /** 告警状态 */
  status: string;
}

/** 活跃事件条目（用于事件驾驶舱展示） */
export interface ActiveIncident {
  /** 事件唯一标识 */
  id: string;
  /** 事件编号 */
  incidentId: string;
  /** 事件标题 */
  title: string;
  /** 严重级别：P0/P1 */
  severity: string;
  /** 处理人 */
  handler: string;
  /** 持续时长 */
  duration: string;
  /** 当前状态 */
  status: string;
}

/**
 * 获取全局系统状态
 * 调用 GET /api/noc/system-status 接口
 * @returns 系统健康度、活跃事件数、今日告警数、SLA 达成率等
 */
export async function getSystemStatus(): Promise<SystemStatus> {
  return request<SystemStatus>('/noc/system-status');
}

/**
 * 获取业务健康矩阵数据
 * 调用 GET /api/noc/business-health 接口
 * @returns 6x4 业务健康矩阵数据数组
 */
export async function getBusinessHealthMatrix(): Promise<BusinessHealthCell[]> {
  return request<BusinessHealthCell[]>('/noc/business-health');
}

/**
 * 获取资源指标折线图数据
 * 调用 GET /api/noc/resource-metrics 接口
 * @returns CPU/内存/网络指标时间序列数据
 */
export async function getResourceMetrics(): Promise<ResourceMetricPoint[]> {
  return request<ResourceMetricPoint[]>('/noc/resource-metrics');
}

/**
 * 获取实时告警列表（用于瀑布流）
 * 调用 GET /api/noc/realtime-alerts 接口
 * @returns 最新告警列表
 */
export async function getRealtimeAlerts(): Promise<RealtimeAlert[]> {
  return request<RealtimeAlert[]>('/noc/realtime-alerts');
}

/**
 * 获取活跃 P0/P1 事件列表（用于事件驾驶舱）
 * 调用 GET /api/noc/active-incidents 接口
 * @returns 活跃高优先级事件列表
 */
export async function getActiveIncidents(): Promise<ActiveIncident[]> {
  return request<ActiveIncident[]>('/noc/active-incidents');
}
