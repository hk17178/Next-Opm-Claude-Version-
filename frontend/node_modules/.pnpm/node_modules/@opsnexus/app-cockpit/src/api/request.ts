/**
 * 统一 HTTP 请求封装
 * 提供认证 token 注入、错误处理、统一响应格式解析
 * 通过 Kong 网关代理到后端各微服务
 */

/** API 网关基础地址，通过 Kong 网关代理到后端服务 */
const BASE_URL = 'http://localhost:8000/api/v1/incident';

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
 * 从 localStorage 获取认证 token
 * @returns JWT token 字符串，未登录时返回 null
 */
function getAuthToken(): string | null {
  return localStorage.getItem('auth_token');
}

/**
 * 通用请求函数
 * 封装 fetch，自动注入认证 token、处理 JSON 解析和业务错误码判断
 * @param url - 请求路径（不含 BASE_URL 前缀）
 * @param options - 可选的 fetch 配置项
 * @returns 解析后的业务数据（ApiResponse.data）
 */
export async function request<T>(
  url: string,
  options?: RequestInit,
): Promise<T> {
  const token = getAuthToken();

  // 构建请求头，包含 JSON 内容类型和认证信息
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options?.headers as Record<string, string>),
  };

  // 若存在认证 token，自动注入 Authorization 头
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${BASE_URL}${url}`, {
    ...options,
    headers,
  });

  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  }

  const json: ApiResponse<T> = await res.json();

  // 业务状态码非 0 视为业务逻辑错误
  if (json.code !== 0) {
    throw new Error(json.message || '请求失败');
  }

  return json.data;
}
