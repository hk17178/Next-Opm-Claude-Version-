/**
 * 事件管理 API 模块 - 封装事件相关的所有接口调用
 * 使用统一的 request 封装，自动处理响应解析和错误码判断
 */
import { request } from './request';

/** 事件记录 */
export interface Incident {
  id: string;              // 事件唯一标识
  incidentId: string;      // 事件编号（如 INC-20260322-001）
  title: string;           // 事件标题
  description: string;     // 事件描述
  severity: string;        // 严重级别（P0-P4）
  status: string;          // 状态（open/acknowledged/processing/resolved/pending_review/closed）
  handler: string;         // 处理人姓名
  mttr: string;            // 平均修复时间
  rootCause: string;       // 根因分类
  createdAt: string;       // 创建时间
  updatedAt: string;       // 更新时间
  business: string;        // 所属业务
  relatedAlerts: number;   // 关联告警数量
  duration: string;        // 持续时长
}

/** 事件评论 */
export interface IncidentComment {
  id: string;          // 评论 ID
  author: string;      // 评论作者
  content: string;     // 评论内容
  createdAt: string;   // 评论时间
}

/** 创建事件参数 */
export interface CreateIncidentParams {
  title: string;       // 事件标题
  description: string; // 事件描述
  severity: string;    // 优先级（P0-P4）
  handler: string;     // 分配人
}

/** 事件列表查询参数 */
export interface IncidentListParams {
  page?: number;       // 当前页码
  pageSize?: number;   // 每页条数
  status?: string;     // 状态过滤
  severity?: string;   // 级别过滤
}

/** 事件列表接口返回数据结构 */
export interface IncidentListResult {
  list: Incident[];    // 事件列表
  total: number;       // 总记录数
}

/**
 * 获取事件列表
 * 调用 GET /api/incidents 接口
 * @param params 查询参数（分页、状态、级别过滤）
 * @returns 事件列表及总数
 */
export async function fetchIncidents(params: IncidentListParams = {}): Promise<IncidentListResult> {
  const query = new URLSearchParams();
  if (params.page) query.set('page', String(params.page));
  if (params.pageSize) query.set('pageSize', String(params.pageSize));
  if (params.status) query.set('status', params.status);
  if (params.severity) query.set('severity', params.severity);
  return request<IncidentListResult>(`/incidents?${query.toString()}`);
}

/**
 * 获取事件详情
 * 调用 GET /api/incidents/:id 接口
 * @param id 事件 ID
 * @returns 事件详细信息
 */
export async function fetchIncidentDetail(id: string): Promise<Incident> {
  return request<Incident>(`/incidents/${id}`);
}

/**
 * 创建新事件
 * 调用 POST /api/incidents 接口
 * @param params 事件创建参数（标题、描述、优先级、分配人）
 * @returns 创建后的事件记录
 */
export async function createIncident(params: CreateIncidentParams): Promise<Incident> {
  return request<Incident>('/incidents', {
    method: 'POST',
    body: JSON.stringify(params),
  });
}

/**
 * 更新事件状态（状态流转）
 * 调用 PUT /api/incidents/:id/status 接口
 * @param id 事件 ID
 * @param status 目标状态（open → acknowledged → resolved → closed）
 * @returns 更新后的事件记录
 */
export async function updateIncidentStatus(id: string, status: string): Promise<Incident> {
  return request<Incident>(`/incidents/${id}/status`, {
    method: 'PUT',
    body: JSON.stringify({ status }),
  });
}

/**
 * 添加事件评论
 * 调用 POST /api/incidents/:id/comments 接口
 * @param id 事件 ID
 * @param content 评论内容
 * @returns 创建后的评论记录
 */
export async function addComment(id: string, content: string): Promise<IncidentComment> {
  return request<IncidentComment>(`/incidents/${id}/comments`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  });
}

/**
 * 获取事件评论列表
 * 调用 GET /api/incidents/:id/comments 接口
 * @param id 事件 ID
 * @returns 评论列表
 */
export async function fetchComments(id: string): Promise<IncidentComment[]> {
  return request<IncidentComment[]>(`/incidents/${id}/comments`);
}
