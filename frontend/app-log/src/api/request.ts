/** API 网关基础地址，通过 Kong 网关代理到后端服务 */
const BASE_URL = 'http://localhost:8000/api';

/** 后端统一响应结构 */
export interface ApiResponse<T = unknown> {
  /** 业务状态码，0 表示成功 */
  code: number;
  /** 提示信息 */
  message: string;
  /** 响应数据 */
  data: T;
}

/**
 * 通用请求函数
 * 封装 fetch，自动处理 JSON 解析和业务错误码判断
 * @param url - 请求路径（不含 BASE_URL 前缀）
 * @param options - 可选的 fetch 配置项
 * @returns 解析后的业务数据（ApiResponse.data）
 */
export async function request<T>(
  url: string,
  options?: RequestInit,
): Promise<T> {
  const res = await fetch(`${BASE_URL}${url}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  }
  const json: ApiResponse<T> = await res.json();
  // 业务状态码非 0 视为业务逻辑错误
  if (json.code !== 0) {
    throw new Error(json.message || 'Request failed');
  }
  return json.data;
}
