/**
 * 变更管理 API 模块
 * 封装变更单全生命周期的接口调用：创建、审批、执行、取消、日历排期、冲突检测
 */
import { request } from './request';

/** 变更类型枚举 */
export type ChangeType = 'standard' | 'normal' | 'emergency';

/** 变更风险等级 */
export type ChangeRisk = 'low' | 'medium' | 'high' | 'critical';

/** 变更状态枚举 */
export type ChangeStatus =
  | 'draft'        // 草稿
  | 'submitted'    // 已提交（待审批）
  | 'approved'     // 已审批
  | 'rejected'     // 已拒绝
  | 'executing'    // 执行中
  | 'completed'    // 已完成
  | 'cancelled';   // 已取消

/** 变更单记录 */
export interface Change {
  /** 变更单唯一标识 */
  id: string;
  /** 变更编号（如 CHG-20260323-001） */
  changeId: string;
  /** 变更标题 */
  title: string;
  /** 变更描述 */
  description: string;
  /** 变更类型 */
  type: ChangeType;
  /** 风险等级 */
  risk: ChangeRisk;
  /** 当前状态 */
  status: ChangeStatus;
  /** 申请人 */
  applicant: string;
  /** 计划开始时间 */
  plannedStart: string;
  /** 计划结束时间 */
  plannedEnd: string;
  /** 实际开始时间 */
  actualStart?: string;
  /** 实际结束时间 */
  actualEnd?: string;
  /** 创建时间 */
  createdAt: string;
  /** 更新时间 */
  updatedAt: string;
  /** 受影响资产列表 */
  affectedAssets: string[];
  /** 回滚方案 */
  rollbackPlan: string;
}

/** 审批记录 */
export interface ApprovalRecord {
  /** 记录 ID */
  id: string;
  /** 审批人 */
  approver: string;
  /** 审批动作：approved / rejected */
  action: 'approved' | 'rejected';
  /** 审批意见 */
  comment: string;
  /** 审批时间 */
  createdAt: string;
}

/** 变更列表查询参数 */
export interface ChangeListParams {
  /** 当前页码 */
  page?: number;
  /** 每页条数 */
  pageSize?: number;
  /** 状态过滤 */
  status?: ChangeStatus;
  /** 变更类型过滤 */
  type?: ChangeType;
  /** 开始时间 */
  startDate?: string;
  /** 结束时间 */
  endDate?: string;
}

/** 变更列表返回结果 */
export interface ChangeListResult {
  /** 变更单列表 */
  list: Change[];
  /** 总记录数 */
  total: number;
}

/** 变更详情（含审批记录） */
export interface ChangeDetail extends Change {
  /** 审批记录列表 */
  approvalRecords: ApprovalRecord[];
}

/** 日历数据条目 */
export interface CalendarEntry {
  /** 日期（YYYY-MM-DD） */
  date: string;
  /** 当天变更数量 */
  count: number;
  /** 当天变更列表 */
  changes: Change[];
  /** 是否存在冲突 */
  hasConflict: boolean;
}

/** 冲突检测参数 */
export interface ConflictCheckParams {
  /** 计划开始时间 */
  plannedStart: string;
  /** 计划结束时间 */
  plannedEnd: string;
  /** 受影响资产（用于检测同资产时间重叠） */
  affectedAssets?: string[];
}

/** 冲突检测结果 */
export interface ConflictResult {
  /** 是否存在冲突 */
  hasConflict: boolean;
  /** 冲突的变更单列表 */
  conflicts: Change[];
}

/** 创建变更单参数 */
export interface CreateChangeParams {
  /** 变更标题 */
  title: string;
  /** 变更描述 */
  description: string;
  /** 变更类型 */
  type: ChangeType;
  /** 风险等级 */
  risk: ChangeRisk;
  /** 计划开始时间 */
  plannedStart: string;
  /** 计划结束时间 */
  plannedEnd: string;
  /** 受影响资产 */
  affectedAssets: string[];
  /** 回滚方案 */
  rollbackPlan: string;
}

/**
 * 获取变更单列表
 * 调用 GET /api/changes 接口
 * @param params 查询参数（分页、状态、类型、时间范围过滤）
 * @returns 变更单列表及总数
 */
export async function listChanges(params: ChangeListParams = {}): Promise<ChangeListResult> {
  const query = new URLSearchParams();
  if (params.page) query.set('page', String(params.page));
  if (params.pageSize) query.set('pageSize', String(params.pageSize));
  if (params.status) query.set('status', params.status);
  if (params.type) query.set('type', params.type);
  if (params.startDate) query.set('startDate', params.startDate);
  if (params.endDate) query.set('endDate', params.endDate);
  return request<ChangeListResult>(`/changes?${query.toString()}`);
}

/**
 * 获取变更单详情
 * 调用 GET /api/changes/:id 接口
 * @param id 变更单 ID
 * @returns 变更单详情（含审批记录）
 */
export async function getChange(id: string): Promise<ChangeDetail> {
  return request<ChangeDetail>(`/changes/${id}`);
}

/**
 * 创建变更单
 * 调用 POST /api/changes 接口
 * @param data 变更单创建参数
 * @returns 创建后的变更单记录
 */
export async function createChange(data: CreateChangeParams): Promise<Change> {
  return request<Change>('/changes', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * 提交变更单（从草稿状态提交审批）
 * 调用 PUT /api/changes/:id/submit 接口
 * @param id 变更单 ID
 * @returns 更新后的变更单记录
 */
export async function submitChange(id: string): Promise<Change> {
  return request<Change>(`/changes/${id}/submit`, { method: 'PUT' });
}

/**
 * 审批通过变更单
 * 调用 PUT /api/changes/:id/approve 接口
 * @param id 变更单 ID
 * @param comment 审批意见
 * @returns 更新后的变更单记录
 */
export async function approveChange(id: string, comment: string): Promise<Change> {
  return request<Change>(`/changes/${id}/approve`, {
    method: 'PUT',
    body: JSON.stringify({ comment }),
  });
}

/**
 * 拒绝变更单
 * 调用 PUT /api/changes/:id/reject 接口
 * @param id 变更单 ID
 * @param reason 拒绝原因
 * @returns 更新后的变更单记录
 */
export async function rejectChange(id: string, reason: string): Promise<Change> {
  return request<Change>(`/changes/${id}/reject`, {
    method: 'PUT',
    body: JSON.stringify({ reason }),
  });
}

/**
 * 开始执行变更
 * 调用 PUT /api/changes/:id/start 接口
 * @param id 变更单 ID
 * @returns 更新后的变更单记录
 */
export async function startChange(id: string): Promise<Change> {
  return request<Change>(`/changes/${id}/start`, { method: 'PUT' });
}

/**
 * 完成变更执行
 * 调用 PUT /api/changes/:id/complete 接口
 * @param id 变更单 ID
 * @returns 更新后的变更单记录
 */
export async function completeChange(id: string): Promise<Change> {
  return request<Change>(`/changes/${id}/complete`, { method: 'PUT' });
}

/**
 * 取消变更单
 * 调用 PUT /api/changes/:id/cancel 接口
 * @param id 变更单 ID
 * @param reason 取消原因
 * @returns 更新后的变更单记录
 */
export async function cancelChange(id: string, reason: string): Promise<Change> {
  return request<Change>(`/changes/${id}/cancel`, {
    method: 'PUT',
    body: JSON.stringify({ reason }),
  });
}

/**
 * 获取变更日历数据
 * 调用 GET /api/changes/calendar 接口
 * @param startDate 起始日期（YYYY-MM-DD）
 * @param endDate 结束日期（YYYY-MM-DD）
 * @returns 日历条目数组
 */
export async function getCalendar(startDate: string, endDate: string): Promise<CalendarEntry[]> {
  return request<CalendarEntry[]>(`/changes/calendar?startDate=${startDate}&endDate=${endDate}`);
}

/**
 * 检测变更时间冲突
 * 调用 POST /api/changes/check-conflicts 接口
 * @param params 冲突检测参数（时间范围、受影响资产）
 * @returns 冲突检测结果
 */
export async function checkConflicts(params: ConflictCheckParams): Promise<ConflictResult> {
  return request<ConflictResult>('/changes/check-conflicts', {
    method: 'POST',
    body: JSON.stringify(params),
  });
}
