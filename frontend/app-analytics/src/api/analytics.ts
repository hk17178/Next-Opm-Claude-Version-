/**
 * 数据分析 API 模块 - 封装 SLA、指标分析、报表导出相关的所有接口调用
 * 使用统一的 request 封装，自动处理响应解析和错误码判断
 */
import { request } from './request';

/** SLA 记录 */
export interface SLARecord {
  key: string;              // 记录唯一标识
  business: string;         // 业务板块名称
  sla: string;              // 当前 SLA 达成率（如 "99.97%"）
  target: string;           // SLA 目标值（如 "99.95%"）
  status: 'met' | 'nearMiss' | 'breached'; // 达成状态（已达成/接近未达/已违约）
  errorBudget: string;      // 错误预算剩余（百分比字符串）
  downtime: string;         // 累计宕机时长
}

/** SLA 概览数据 */
export interface SLAOverview {
  overallSLA: string;       // 全局 SLA 达成率
  errorBudgetRemaining: string; // 错误预算剩余
  monthlyIncidents: number; // 本月事件数
  totalDowntime: string;    // 总宕机时间
  avgMTTR: string;          // 平均修复时间
}

/** SLA 查询参数 */
export interface SLAParams {
  period?: string;          // 时间周期（week/month/quarter/year）
  business?: string;        // 业务板块过滤
  tier?: string;            // 服务等级过滤
  grade?: string;           // 资产分级过滤
}

/** SLA 接口返回数据结构 */
export interface SLAResult {
  list: SLARecord[];        // SLA 明细列表
  overview: SLAOverview;    // SLA 概览统计
}

/** 关联分析记录 */
export interface CorrelationRecord {
  key: string;              // 记录唯一标识
  asset: string;            // 资产名称
  alertCount: number;       // 告警次数
  incidentCount: number;    // 事件次数
  avgMTTR: string;          // 平均修复时间
  riskScore: number;        // 风险评分（0-100）
}

/** 交易分析记录 */
export interface TransactionRecord {
  key: string;              // 记录唯一标识
  service: string;          // 服务名称
  qps: number;              // 每秒查询数
  p99: string;              // P99 延迟
  errorRate: string;        // 错误率
  trend: 'up' | 'down' | 'stable'; // 趋势方向
}

/** 指标查询参数 */
export interface MetricsParams {
  period?: string;          // 时间周期（24h/7d/30d/90d）
  business?: string;        // 业务板块过滤
  assetType?: string;       // 资产类型过滤
}

/** 指标概览数据 */
export interface MetricsSummary {
  totalAlerts: number;      // 告警总数
  totalIncidents: number;   // 事件总数
  topRisk: string;          // 最高风险资产
  avgErrorRate: string;     // 平均错误率
}

/** 指标分析接口返回数据结构 */
export interface MetricsResult {
  correlationList: CorrelationRecord[]; // 关联分析列表
  transactionList: TransactionRecord[]; // 交易分析列表
  summary: MetricsSummary;              // 概览统计数据
}

/**
 * 构建查询字符串
 * 遍历参数对象，将非空值拼接为 URL 查询参数
 * @param params 参数键值对
 * @returns 拼接好的查询字符串
 */
function buildQuery(params: Record<string, unknown>): string {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') query.set(k, String(v));
  });
  return query.toString();
}

/**
 * 获取 SLA 数据
 * 调用 GET /api/analytics/sla 接口
 * @param params 查询参数（周期、业务、等级、分级过滤）
 * @returns SLA 概览 + 各业务 SLA 明细列表
 */
export async function fetchSLA(params: SLAParams = {}): Promise<SLAResult> {
  const qs = buildQuery(params);
  return request<SLAResult>(`/analytics/sla?${qs}`);
}

/**
 * 获取指标分析数据
 * 调用 GET /api/analytics/metrics 接口
 * @param params 查询参数（周期、业务、资产类型过滤）
 * @returns 指标概览 + 关联分析/交易分析列表
 */
export async function fetchMetrics(params: MetricsParams = {}): Promise<MetricsResult> {
  const qs = buildQuery(params);
  return request<MetricsResult>(`/analytics/metrics?${qs}`);
}

/**
 * 导出分析报表
 * 调用 GET /api/analytics/reports/{id}/export?format=csv
 * 注：文件下载使用原始 fetch，不走通用 request 封装（返回 Blob 而非 JSON）
 * @param reportId 报表 ID（correlation/transaction）
 * @param format 导出格式（csv/xlsx/pdf）
 * @returns 文件下载 Blob
 */
export async function exportReport(reportId: string, format: string = 'csv'): Promise<Blob> {
  const BASE_URL = 'http://localhost:8000/api';
  const res = await fetch(`${BASE_URL}/analytics/reports/${reportId}/export?format=${format}`);
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  return res.blob();
}
