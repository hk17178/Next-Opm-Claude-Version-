/**
 * 自动化编排引擎 API 模块
 * 封装工作流管理和执行管理的所有接口调用
 */
import { request } from './request';

// ======================== 类型定义 ========================

/** 步骤类型枚举 */
export type StepType = 'script' | 'approval' | 'condition' | 'wait' | 'notify';

/** 触发类型枚举 */
export type TriggerType = 'manual' | 'alert' | 'cron';

/** 执行状态枚举 */
export type ExecutionStatus = 'pending' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled';

/** 步骤执行状态枚举 */
export type StepStatus = 'pending' | 'running' | 'success' | 'failed' | 'skipped' | 'waiting_approval';

/** 审批逾期动作 */
export type ApprovalTimeoutAction = 'auto_approve' | 'auto_reject';

/** 通知渠道 */
export type NotifyChannel = 'email' | 'webhook' | 'dingtalk' | 'wechat' | 'sms';

/** 工作流步骤定义 */
export interface WorkflowStep {
  /** 步骤名称 */
  name: string;
  /** 步骤类型 */
  type: StepType;
  /** 脚本内容（type=script 时使用） */
  script?: string;
  /** 超时时间（秒） */
  timeout?: number;
  /** 审批人列表（type=approval 时使用） */
  approvers?: string[];
  /** 审批逾期动作（type=approval 时使用） */
  approvalTimeoutAction?: ApprovalTimeoutAction;
  /** 条件表达式（type=condition 时使用） */
  condition?: string;
  /** 条件为真时跳转的步骤序号（type=condition 时使用） */
  trueBranch?: number;
  /** 条件为假时跳转的步骤序号（type=condition 时使用） */
  falseBranch?: number;
  /** 等待时间（分钟，type=wait 时使用） */
  waitMinutes?: number;
  /** 通知标题（type=notify 时使用） */
  notifyTitle?: string;
  /** 通知内容（type=notify 时使用） */
  notifyContent?: string;
  /** 通知渠道列表（type=notify 时使用） */
  notifyChannels?: NotifyChannel[];
}

/** 工作流变量定义 */
export interface WorkflowVariable {
  /** 变量名 */
  name: string;
  /** 变量类型 */
  type: 'string' | 'number' | 'boolean';
  /** 默认值 */
  defaultValue?: string;
  /** 变量说明 */
  description?: string;
}

/** 工作流定义 */
export interface Workflow {
  /** 工作流唯一标识 */
  id: string;
  /** 工作流名称 */
  name: string;
  /** 工作流描述 */
  description: string;
  /** 触发类型 */
  triggerType: TriggerType;
  /** Cron 表达式（triggerType=cron 时使用） */
  cronExpr?: string;
  /** 步骤列表 */
  steps: WorkflowStep[];
  /** 变量定义列表 */
  variables: WorkflowVariable[];
  /** 是否启用 */
  enabled: boolean;
  /** 最近一次执行时间 */
  lastExecutedAt?: string;
  /** 最近一次执行状态 */
  lastExecutionStatus?: ExecutionStatus;
  /** 创建时间 */
  createdAt: string;
  /** 更新时间 */
  updatedAt: string;
  /** 创建人 */
  createdBy: string;
}

/** 创建工作流请求参数 */
export interface CreateWorkflowPayload {
  name: string;
  description: string;
  triggerType: TriggerType;
  cronExpr?: string;
  steps: WorkflowStep[];
  variables?: WorkflowVariable[];
}

/** 更新工作流请求参数 */
export interface UpdateWorkflowPayload extends Partial<CreateWorkflowPayload> {
  enabled?: boolean;
}

/** 工作流模板 */
export interface WorkflowTemplate {
  /** 模板 ID */
  id: string;
  /** 模板名称 */
  name: string;
  /** 模板描述 */
  description: string;
  /** 模板图标 */
  icon: string;
  /** 预定义步骤 */
  steps: WorkflowStep[];
  /** 预定义变量 */
  variables: WorkflowVariable[];
}

/** 执行步骤记录 */
export interface ExecutionStepRecord {
  /** 步骤序号 */
  index: number;
  /** 步骤名称 */
  name: string;
  /** 步骤类型 */
  type: StepType;
  /** 步骤执行状态 */
  status: StepStatus;
  /** 开始时间 */
  startedAt?: string;
  /** 结束时间 */
  finishedAt?: string;
  /** 耗时（秒） */
  duration?: number;
  /** 步骤输入参数 */
  input?: Record<string, unknown>;
  /** 步骤输出结果 */
  output?: Record<string, unknown>;
  /** 错误信息（status=failed 时） */
  error?: string;
  /** 审批人（type=approval 时） */
  approver?: string;
  /** 审批意见（type=approval 时） */
  approvalComment?: string;
}

/** 工作流执行记录 */
export interface WorkflowExecution {
  /** 执行记录唯一标识 */
  id: string;
  /** 关联工作流 ID */
  workflowId: string;
  /** 工作流名称 */
  workflowName: string;
  /** 执行状态 */
  status: ExecutionStatus;
  /** 触发来源描述 */
  triggerSource: string;
  /** 执行变量 */
  variables: Record<string, string>;
  /** 步骤执行记录列表 */
  steps: ExecutionStepRecord[];
  /** 开始时间 */
  startedAt: string;
  /** 结束时间 */
  finishedAt?: string;
  /** 总耗时（秒） */
  duration?: number;
  /** 错误信息 */
  error?: string;
}

/** 执行日志 */
export interface ExecutionLog {
  /** 执行 ID */
  executionId: string;
  /** 日志行列表 */
  lines: Array<{
    /** 时间戳 */
    timestamp: string;
    /** 日志级别 */
    level: 'info' | 'warn' | 'error';
    /** 日志内容 */
    message: string;
    /** 关联步骤序号 */
    stepIndex?: number;
  }>;
}

/** 分页结果通用结构 */
export interface PageResult<T> {
  /** 数据列表 */
  list: T[];
  /** 总记录数 */
  total: number;
}

/** 工作流列表查询参数 */
export interface WorkflowListParams {
  page?: number;
  pageSize?: number;
  keyword?: string;
  enabled?: boolean;
}

/** 执行记录列表查询参数 */
export interface ExecutionListParams {
  page?: number;
  pageSize?: number;
  workflowId?: string;
  status?: ExecutionStatus;
}

// ======================== 工作流管理 API ========================

/**
 * 获取工作流列表
 * @param params 查询参数（分页、关键词、启用状态过滤）
 * @returns 工作流分页列表
 */
export async function listWorkflows(params: WorkflowListParams = {}): Promise<PageResult<Workflow>> {
  const query = new URLSearchParams();
  if (params.page) query.set('page', String(params.page));
  if (params.pageSize) query.set('pageSize', String(params.pageSize));
  if (params.keyword) query.set('keyword', params.keyword);
  if (params.enabled !== undefined) query.set('enabled', String(params.enabled));
  return request<PageResult<Workflow>>(`/orchestration/workflows?${query.toString()}`);
}

/**
 * 获取工作流详情
 * @param id 工作流 ID
 * @returns 工作流详细信息
 */
export async function getWorkflow(id: string): Promise<Workflow> {
  return request<Workflow>(`/orchestration/workflows/${id}`);
}

/**
 * 创建工作流
 * @param data 工作流创建参数
 * @returns 创建后的工作流
 */
export async function createWorkflow(data: CreateWorkflowPayload): Promise<Workflow> {
  return request<Workflow>('/orchestration/workflows', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * 更新工作流
 * @param id 工作流 ID
 * @param data 工作流更新参数
 * @returns 更新后的工作流
 */
export async function updateWorkflow(id: string, data: UpdateWorkflowPayload): Promise<Workflow> {
  return request<Workflow>(`/orchestration/workflows/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

/**
 * 删除工作流
 * @param id 工作流 ID
 */
export async function deleteWorkflow(id: string): Promise<void> {
  return request<void>(`/orchestration/workflows/${id}`, {
    method: 'DELETE',
  });
}

/**
 * 获取工作流模板列表
 * @returns 预置模板列表
 */
export async function listTemplates(): Promise<WorkflowTemplate[]> {
  return request<WorkflowTemplate[]>('/orchestration/templates');
}

/**
 * 从模板创建工作流
 * @param templateId 模板 ID
 * @param name 工作流名称
 * @returns 创建后的工作流
 */
export async function createFromTemplate(templateId: string, name: string): Promise<Workflow> {
  return request<Workflow>('/orchestration/workflows/from-template', {
    method: 'POST',
    body: JSON.stringify({ templateId, name }),
  });
}

// ======================== 执行管理 API ========================

/**
 * 获取执行记录列表
 * @param params 查询参数（分页、工作流过滤、状态过滤）
 * @returns 执行记录分页列表
 */
export async function listExecutions(params: ExecutionListParams = {}): Promise<PageResult<WorkflowExecution>> {
  const query = new URLSearchParams();
  if (params.page) query.set('page', String(params.page));
  if (params.pageSize) query.set('pageSize', String(params.pageSize));
  if (params.workflowId) query.set('workflowId', params.workflowId);
  if (params.status) query.set('status', params.status);
  return request<PageResult<WorkflowExecution>>(`/orchestration/executions?${query.toString()}`);
}

/**
 * 获取执行记录详情
 * @param id 执行记录 ID
 * @returns 执行记录详细信息
 */
export async function getExecution(id: string): Promise<WorkflowExecution> {
  return request<WorkflowExecution>(`/orchestration/executions/${id}`);
}

/**
 * 触发工作流执行
 * @param workflowId 工作流 ID
 * @param variables 执行变量（可选）
 * @returns 创建的执行记录
 */
export async function triggerExecution(
  workflowId: string,
  variables?: Record<string, string>,
): Promise<WorkflowExecution> {
  return request<WorkflowExecution>('/orchestration/executions', {
    method: 'POST',
    body: JSON.stringify({ workflowId, variables }),
  });
}

/**
 * 取消执行
 * @param id 执行记录 ID
 */
export async function cancelExecution(id: string): Promise<void> {
  return request<void>(`/orchestration/executions/${id}/cancel`, {
    method: 'POST',
  });
}

/**
 * 审批通过步骤
 * @param executionId 执行记录 ID
 * @param stepIndex 步骤序号
 * @param comment 审批意见（可选）
 */
export async function approveStep(
  executionId: string,
  stepIndex: number,
  comment?: string,
): Promise<void> {
  return request<void>(`/orchestration/executions/${executionId}/steps/${stepIndex}/approve`, {
    method: 'POST',
    body: JSON.stringify({ comment }),
  });
}

/**
 * 审批拒绝步骤
 * @param executionId 执行记录 ID
 * @param stepIndex 步骤序号
 * @param reason 拒绝原因
 */
export async function rejectStep(
  executionId: string,
  stepIndex: number,
  reason: string,
): Promise<void> {
  return request<void>(`/orchestration/executions/${executionId}/steps/${stepIndex}/reject`, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  });
}

/**
 * 获取执行日志
 * @param id 执行记录 ID
 * @returns 执行日志
 */
export async function getExecutionLogs(id: string): Promise<ExecutionLog> {
  return request<ExecutionLog>(`/orchestration/executions/${id}/logs`);
}
