/**
 * 用户建议管理 API 模块
 * 封装用户建议相关的所有接口调用，包括提交建议、查询列表、更新状态、统计数据
 */
import { request } from './request';

/** 建议状态枚举 */
export type SuggestionStatus = 'pending' | 'accepted' | 'rejected' | 'developing' | 'released';

/** 情感分析结果 */
export type SentimentType = 'positive' | 'neutral' | 'negative';

/** 用户建议记录 */
export interface Suggestion {
  /** 建议唯一标识 */
  id: string;
  /** 建议标题 */
  title: string;
  /** 建议描述 */
  description: string;
  /** AI 自动分类标签 */
  category: string;
  /** 情感分析结果 */
  sentiment: SentimentType;
  /** 当前状态 */
  status: SuggestionStatus;
  /** 提交人 */
  submitter: string;
  /** 提交时间 */
  createdAt: string;
  /** 管理员备注 */
  adminNote?: string;
}

/** 提交建议参数 */
export interface SubmitSuggestionParams {
  /** 建议标题 */
  title: string;
  /** 建议详细描述 */
  description: string;
}

/** 建议统计数据 */
export interface SuggestionStats {
  /** 总建议数 */
  total: number;
  /** 待评估数量 */
  pending: number;
  /** 已采纳数量 */
  accepted: number;
  /** 已拒绝数量 */
  rejected: number;
  /** 开发中数量 */
  developing: number;
  /** 已上线数量 */
  released: number;
}

/** 建议列表查询结果 */
export interface SuggestionListResult {
  /** 建议列表 */
  list: Suggestion[];
  /** 总记录数 */
  total: number;
}

/**
 * 提交用户建议
 * 调用 POST /api/suggestions 接口
 * @param data 建议提交参数（标题、描述）
 * @returns 创建后的建议记录
 */
export async function submitSuggestion(data: SubmitSuggestionParams): Promise<Suggestion> {
  return request<Suggestion>('/suggestions', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * 获取建议列表
 * 调用 GET /api/suggestions 接口
 * @param status 可选的状态过滤
 * @returns 建议列表及总数
 */
export async function listSuggestions(status?: SuggestionStatus): Promise<SuggestionListResult> {
  const query = status ? `?status=${status}` : '';
  return request<SuggestionListResult>(`/suggestions${query}`);
}

/**
 * 更新建议状态
 * 调用 PUT /api/suggestions/:id/status 接口
 * @param id 建议 ID
 * @param status 目标状态
 * @param note 管理员备注（可选）
 * @returns 更新后的建议记录
 */
export async function updateStatus(
  id: string,
  status: SuggestionStatus,
  note?: string,
): Promise<Suggestion> {
  return request<Suggestion>(`/suggestions/${id}/status`, {
    method: 'PUT',
    body: JSON.stringify({ status, note }),
  });
}

/**
 * 获取建议统计数据
 * 调用 GET /api/suggestions/stats 接口
 * @returns 各状态建议数量统计
 */
export async function getStats(): Promise<SuggestionStats> {
  return request<SuggestionStats>('/suggestions/stats');
}
