import { request } from './request';

/** 告警记录 */
export interface AlertRecord {
  /** 告警唯一标识 */
  id: string;
  /** 严重程度：P0-P4 */
  severity: string;
  /** 告警内容描述 */
  content: string;
  /** 告警来源（如 Prometheus、Zabbix） */
  source: string;
  /** 告警层级（0-5 层） */
  layer: number;
  /** 触发时间 */
  triggerTime: string;
  /** 持续时间（如 "5m", "2h"） */
  duration: string;
  /** 是否为铁律告警（Layer 0 不可关闭） */
  isIronRule: boolean;
  /** 当前状态：firing / acknowledged / resolved / suppressed */
  status: string;
  /** 所属业务板块 */
  business?: string;
}

/** 告警列表查询参数 */
export interface AlertListParams {
  /** 按状态过滤 */
  status?: string;
  /** 按严重程度过滤 */
  severity?: string;
  /** 按来源过滤 */
  source?: string;
  /** 按业务板块过滤 */
  business?: string;
  /** 按层级过滤 */
  layer?: number;
  /** 关键词搜索 */
  keyword?: string;
  /** 当前页码 */
  page?: number;
  /** 每页条数 */
  pageSize?: number;
}

/** 告警列表响应结果（含统计卡片数据） */
export interface AlertListResult {
  /** 告警列表 */
  list: AlertRecord[];
  /** 匹配总数 */
  total: number;
  /** 统计概览数据，用于顶部 Stat 卡片 */
  stats: {
    /** 当前触发中告警数 */
    firing: number;
    /** 今日新增告警数 */
    todayNew: number;
    /** 今日已解决告警数 */
    todayResolved: number;
    /** 被降噪抑制的告警数 */
    suppressed: number;
    /** 降噪率百分比 */
    noiseRate: string;
    /** 触发中告警较昨日变化量 */
    firingTrend: number;
    /** 触发中告警变化方向 */
    firingDirection: 'up' | 'down';
    /** 今日新增较昨日变化量 */
    todayNewTrend: number;
    /** 今日新增变化方向 */
    todayNewDirection: 'up' | 'down';
    /** 今日已解决较昨日变化量 */
    todayResolvedTrend: number;
    /** 今日已解决变化方向 */
    todayResolvedDirection: 'up' | 'down';
  };
}

/** 告警规则 */
export interface AlertRule {
  /** 规则唯一标识 */
  id: string;
  /** 规则名称 */
  name: string;
  /** 规则类型：threshold / anomaly / trend / business */
  type: string;
  /** 所属层级（0-5） */
  layer: number;
  /** 触发条件表达式 */
  condition: string;
  /** 阈值 */
  threshold: string;
  /** 触发后的告警严重程度 */
  severity: string;
  /** 是否启用 */
  enabled: boolean;
  /** 创建时间 */
  createdAt: string;
  /** 最后更新时间 */
  updatedAt: string;
}

/** 创建告警规则的请求参数 */
export interface AlertRuleCreateParams {
  /** 规则名称 */
  name: string;
  /** 规则类型 */
  type: string;
  /** 所属层级 */
  layer: number;
  /** 触发条件 */
  condition: string;
  /** 阈值 */
  threshold: string;
  /** 严重程度 */
  severity: string;
}

/**
 * 获取告警列表
 * 调用 GET /api/alerts 接口，支持状态、级别、来源等多维过滤
 * @param params - 查询参数
 * @returns 告警列表 + 统计数据
 */
export async function fetchAlerts(params: AlertListParams): Promise<AlertListResult> {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value));
    }
  });
  return request<AlertListResult>(`/alerts?${query.toString()}`);
}

/**
 * 确认（ACK）告警
 * 调用 PATCH /api/alerts/{id}/ack 接口，确认后自动创建关联事件
 * @param id - 告警 ID
 */
export async function acknowledgeAlert(id: string): Promise<void> {
  return request<void>(`/alerts/${encodeURIComponent(id)}/ack`, { method: 'PATCH' });
}

/**
 * 获取告警规则列表
 * 调用 GET /api/alert-rules 接口
 * @param params - 分页参数
 * @returns 规则列表 + 总数
 */
export async function fetchAlertRules(params?: {
  page?: number;
  pageSize?: number;
}): Promise<{ list: AlertRule[]; total: number }> {
  const query = new URLSearchParams();
  if (params) {
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined) query.set(key, String(value));
    });
  }
  return request<{ list: AlertRule[]; total: number }>(`/alert-rules?${query.toString()}`);
}

/**
 * 切换告警规则启用/禁用状态
 * 调用 PATCH /api/alert-rules/{id}/toggle 接口
 * @param id - 规则 ID
 */
export async function toggleAlertRule(id: string): Promise<void> {
  return request<void>(`/alert-rules/${encodeURIComponent(id)}/toggle`, { method: 'PATCH' });
}

/**
 * 创建新的告警规则
 * 调用 POST /api/alert-rules 接口
 * @param params - 规则配置参数
 * @returns 创建后的规则对象
 */
export async function createAlertRule(params: AlertRuleCreateParams): Promise<AlertRule> {
  return request<AlertRule>('/alert-rules', {
    method: 'POST',
    body: JSON.stringify(params),
  });
}

/** 告警详情（含 AI 分析） */
export interface AlertDetail extends AlertRecord {
  service?: string;
  rule?: string;
  assetGrade?: string;
  tags?: string[];
  incidentId?: string;
  /** AI 根因分析结果 */
  aiAnalysis?: {
    confidence: number;        // 0-100
    rootCause: string;
    category: string;
    evidence: string[];
    suggestion: string;
    estimatedRecovery: string;
    analysisDuration: string;
  };
  /** 关联日志（最近10条） */
  relatedLogs?: Array<{
    time: string;
    level: string;
    message: string;
    host: string;
  }>;
  /** 关联指标（最近变化） */
  relatedMetrics?: Array<{
    name: string;
    current: string;
    baseline: string;
    deviation: string;
  }>;
  /** 操作审计记录 */
  auditLog?: Array<{
    time: string;
    user: string;
    action: string;
    note: string;
  }>;
}

/**
 * 获取告警详情（含 AI 分析）
 * GET /api/alerts/{id}
 */
export async function fetchAlertDetail(id: string): Promise<AlertDetail> {
  return request<AlertDetail>(`/alerts/${encodeURIComponent(id)}`);
}
