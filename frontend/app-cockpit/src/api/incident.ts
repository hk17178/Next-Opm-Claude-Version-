/**
 * 事件 API 客户端
 * 封装事件相关的接口调用，包括列表查询、详情获取、时间线获取
 */
import { request } from './request';

/** 活跃事件数据结构（与 Cockpit.tsx 中的 ActiveIncident 一致） */
export interface Incident {
  /** 事件唯一标识（UUID 格式） */
  id: string;
  /** 事件标题 */
  title: string;
  /** 事件描述 */
  description?: string;
  /** 严重级别：P0 / P1 / P2 / P3 / P4 */
  severity: string;
  /** 事件状态：open / investigating / identified / mitigated / resolved / closed */
  status: string;
  /** 指派处理人 ID */
  assignee_id?: string;
  /** 处理人姓名（前端展示用，由后端关联查询填充） */
  handler?: string;
  /** 持续时长（前端展示用，由后端计算填充） */
  duration?: string;
  /** 关联告警 ID 列表 */
  related_alert_ids?: string[];
  /** 受影响配置项 ID 列表 */
  affected_ci_ids?: string[];
  /** 自定义标签键值对 */
  labels?: Record<string, string>;
  /** 创建时间（ISO 格式） */
  created_at?: string;
  /** 更新时间（ISO 格式） */
  updated_at?: string;
  /** 解决时间（ISO 格式） */
  resolved_at?: string;
}

/** 事件列表响应结构 */
export interface IncidentListResponse {
  /** 事件列表 */
  incidents: Incident[];
  /** 下一页分页令牌，无更多数据时为空 */
  next_page_token?: string;
}

/** 事件列表查询参数 */
export interface IncidentListParams {
  /** 状态过滤 */
  status?: string;
  /** 严重级别过滤 */
  severity?: string;
  /** 指派人 ID 过滤 */
  assignee_id?: string;
  /** 分页令牌 */
  page_token?: string;
  /** 每页条数（默认 20，最大 100） */
  page_size?: number;
}

/** 时间线条目数据结构（与 Cockpit.tsx 中的 TimelineItem 一致） */
export interface TimelineEntry {
  /** 条目唯一标识 */
  id: string;
  /** 时间戳（ISO 格式） */
  time: string;
  /** 事件类型：system / human / ai / recovery / comment / status_change 等 */
  type: 'system' | 'human' | 'ai' | 'recovery' | 'comment' | 'status_change' | 'assignment' | 'escalation' | 'ai_analysis' | 'notification';
  /** 标题 */
  title: string;
  /** 描述（可选） */
  description?: string;
  /** 内容文本 */
  content?: string;
  /** 操作人 ID */
  author_id?: string;
  /** 创建时间（ISO 格式） */
  created_at?: string;
}

/** 时间线响应结构 */
export interface TimelineResponse {
  /** 时间线条目列表 */
  entries: TimelineEntry[];
}

/**
 * 获取活跃事件列表
 * 调用 GET /incidents?status=... 接口，支持状态、级别等过滤
 * @param params - 查询参数，默认过滤活跃状态事件
 * @returns 事件列表响应
 */
export async function listActiveIncidents(
  params: IncidentListParams = {},
): Promise<IncidentListResponse> {
  const query = new URLSearchParams();
  // 默认查询活跃（非 resolved/closed）事件
  const queryParams = { status: 'open', ...params };
  Object.entries(queryParams).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value));
    }
  });
  return request<IncidentListResponse>(`/incidents?${query.toString()}`);
}

/**
 * 获取事件详情
 * 调用 GET /incidents/{id} 接口
 * @param id - 事件 ID（UUID 格式）
 * @returns 事件详情
 */
export async function getIncidentDetail(id: string): Promise<Incident> {
  return request<Incident>(`/incidents/${id}`);
}

/**
 * 获取事件时间线
 * 调用 GET /incidents/{id}/timeline 接口
 * @param id - 事件 ID（UUID 格式）
 * @returns 时间线条目列表
 */
export async function getIncidentTimeline(id: string): Promise<TimelineEntry[]> {
  const res = await request<TimelineResponse>(`/incidents/${id}/timeline`);
  return res.entries;
}
