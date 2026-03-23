/**
 * 资产管理 API 模块 - 封装 CMDB 资产相关的所有接口调用
 * 使用统一的 request 封装，自动处理响应解析和错误码判断
 */
import { request } from './request';

/** 资产记录 */
export interface Asset {
  id: string;                          // 资产唯一标识
  hostname: string;                    // 主机名
  ip: string;                         // IP 地址
  type: string;                       // 资产类型（ECS/RDS/SLB 等）
  business: string;                   // 所属业务板块
  grade: string;                      // 资产分级（S/A/B/C/D）
  env: string;                        // 环境（prod/staging/dev）
  status: string;                     // 状态（online/offline/maintenance）
  os?: string;                        // 操作系统（可选）
  region?: string;                    // 地域（可选）
  tags?: Record<string, string>;      // 标签键值对（可选）
  group?: string;                     // 所属资产组（可选）
  createdAt?: string;                 // 创建时间（可选）
  updatedAt?: string;                 // 更新时间（可选）
}

/** 资产列表查询参数 */
export interface AssetListParams {
  page?: number;       // 当前页码
  pageSize?: number;   // 每页条数
  type?: string;       // 资产类型过滤
  business?: string;   // 业务板块过滤
  env?: string;        // 环境过滤
  grade?: string;      // 资产分级过滤
  status?: string;     // 状态过滤
}

/** 资产列表接口返回数据结构 */
export interface AssetListResult {
  list: Asset[];       // 资产列表
  total: number;       // 总记录数
}

/**
 * 获取资产列表
 * 调用 GET /api/assets 接口
 * @param params 查询参数（分页及各类过滤条件）
 * @returns 资产列表及总数
 */
export async function fetchAssets(params: AssetListParams = {}): Promise<AssetListResult> {
  const query = new URLSearchParams();
  // 遍历参数，将非空值添加到查询字符串
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null) query.set(k, String(v));
  });
  return request<AssetListResult>(`/assets?${query.toString()}`);
}

/**
 * 获取资产详情
 * 调用 GET /api/assets/:id 接口
 * @param id 资产 ID
 * @returns 资产详细信息（含标签、所属组等）
 */
export async function fetchAssetDetail(id: string): Promise<Asset> {
  return request<Asset>(`/assets/${id}`);
}
