import { request } from './request';

/** 日志搜索请求参数 */
export interface LogSearchParams {
  /** 搜索关键词（支持 Lucene 语法） */
  keyword?: string;
  /** 日志级别过滤：ERROR / WARN / INFO / DEBUG */
  level?: string;
  /** 主机名过滤 */
  host?: string;
  /** 服务名过滤 */
  service?: string;
  /** 来源类型过滤 */
  sourceType?: string;
  /** 时间快捷选项：15min / 1h / 6h / 24h / 7d / 30d */
  timePreset?: string;
  /** 自定义起始时间（ISO 格式） */
  startTime?: string;
  /** 自定义结束时间（ISO 格式） */
  endTime?: string;
  /** 当前页码 */
  page?: number;
  /** 每页条数 */
  pageSize?: number;
}

/** 单条日志记录 */
export interface LogEntry {
  /** 日志唯一标识 */
  id: string;
  /** 时间戳 */
  timestamp: string;
  /** 日志级别 */
  level: string;
  /** 主机名 */
  host: string;
  /** 服务名 */
  service: string;
  /** 日志摘要消息 */
  message: string;
  /** 链路追踪 ID */
  traceId?: string;
  /** 来源类型 */
  sourceType?: string;
  /** 自定义标签键值对 */
  tags?: Record<string, string>;
  /** 完整原始日志消息 */
  fullMessage?: string;
}

/** 日志搜索结果 */
export interface LogSearchResult {
  /** 日志列表 */
  list: LogEntry[];
  /** 匹配总数 */
  total: number;
  /** 查询耗时（秒） */
  took: number;
}

/** 日志量趋势数据点 */
export interface LogVolumePoint {
  /** 时间标签（如 "14:00"） */
  time: string;
  /** 该时段日志数量 */
  count: number;
}

/** 日志级别分布数据 */
export interface LogLevelDistribution {
  /** 日志级别 */
  level: string;
  /** 该级别日志数量 */
  count: number;
}

/**
 * 搜索日志
 * 调用 GET /api/logs/search 接口，支持关键词、时间范围、级别等多维过滤
 * @param params - 搜索参数
 * @returns 搜索结果（列表 + 总数 + 耗时）
 */
export async function searchLogs(params: LogSearchParams): Promise<LogSearchResult> {
  // 将参数对象转为 URL 查询字符串，跳过空值
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value));
    }
  });
  return request<LogSearchResult>(`/logs/search?${query.toString()}`);
}

/**
 * 获取日志量趋势数据
 * 调用 GET /api/logs/volume 接口，用于折线图展示
 * @param params - 时间范围参数
 * @returns 时间序列数据点数组
 */
export async function getLogVolumeTrend(params: {
  /** 时间快捷选项 */
  timePreset?: string;
  /** 自定义起始时间 */
  startTime?: string;
  /** 自定义结束时间 */
  endTime?: string;
}): Promise<LogVolumePoint[]> {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value));
    }
  });
  return request<LogVolumePoint[]>(`/logs/volume?${query.toString()}`);
}

/**
 * 获取日志级别分布数据
 * 调用 GET /api/logs/level-distribution 接口，用于饼图展示
 * @param params - 时间范围参数
 * @returns 各级别日志数量数组
 */
export async function getLogLevelDistribution(params: {
  /** 时间快捷选项 */
  timePreset?: string;
  /** 自定义起始时间 */
  startTime?: string;
  /** 自定义结束时间 */
  endTime?: string;
}): Promise<LogLevelDistribution[]> {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value));
    }
  });
  return request<LogLevelDistribution[]>(`/logs/level-distribution?${query.toString()}`);
}

/**
 * 导出日志文件
 * 调用 GET /api/logs/export 接口，触发浏览器下载 CSV 或 JSON 文件
 * @param params - 导出参数（格式、搜索条件、最大行数等）
 */
export async function exportLogs(params: {
  /** 导出格式：csv / json */
  format: string;
  /** 搜索关键词 */
  keyword?: string;
  /** 日志级别过滤 */
  level?: string;
  /** 主机名过滤 */
  host?: string;
  /** 服务名过滤 */
  service?: string;
  /** 来源类型过滤 */
  sourceType?: string;
  /** 时间快捷选项 */
  timePreset?: string;
  /** 自定义起始时间 */
  startTime?: string;
  /** 自定义结束时间 */
  endTime?: string;
  /** 最大导出行数 */
  maxRows?: number;
}): Promise<void> {
  const BASE_URL = 'http://localhost:8000/api';
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value));
    }
  });

  // 直接通过 fetch 下载文件流，不走通用 request 封装
  const res = await fetch(`${BASE_URL}/logs/export?${query.toString()}`);
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  }

  // 从响应头获取文件名，若无则用默认名
  const disposition = res.headers.get('Content-Disposition');
  const filenameMatch = disposition?.match(/filename="?(.+?)"?$/);
  const filename = filenameMatch?.[1] ?? `logs-export.${params.format}`;

  // 将响应转为 Blob 并触发浏览器下载
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
