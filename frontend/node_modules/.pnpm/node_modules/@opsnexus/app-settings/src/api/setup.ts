/**
 * Web 安装向导 API 模块
 * 提供数据库/中间件/LDAP 连接测试、管理员创建、系统初始化接口
 * 对应后端 svc-setup 服务，路由前缀 /v1/setup
 */
import { request } from './request';

// ==================== 类型定义 ====================

/** 数据库连接配置 */
export interface DatabaseConfig {
  /** PostgreSQL 主机地址 */
  host: string;
  /** 端口号 */
  port: number;
  /** 数据库名称 */
  database: string;
  /** 用户名 */
  username: string;
  /** 密码 */
  password: string;
}

/** 中间件连接配置 */
export interface MiddlewareConfig {
  /** Kafka Bootstrap 地址（如 localhost:9092） */
  kafkaBootstrap: string;
  /** Elasticsearch 地址（如 http://localhost:9200） */
  esAddress: string;
  /** Redis 地址（如 localhost:6379） */
  redisAddress: string;
}

/** LDAP 配置 */
export interface LDAPConfig {
  /** LDAP 服务器地址（如 ldap://ldap.example.com:389） */
  serverURL: string;
  /** Base DN（如 dc=example,dc=com） */
  baseDN: string;
  /** 绑定账号（如 cn=admin,dc=example,dc=com） */
  bindDN: string;
  /** 绑定密码 */
  bindPassword: string;
}

/** 管理员账号配置 */
export interface AdminConfig {
  /** 管理员用户名 */
  username: string;
  /** 管理员密码 */
  password: string;
  /** 管理员邮箱 */
  email: string;
}

/** 品牌配置 */
export interface BrandConfig {
  /** 系统名称 */
  systemName: string;
  /** Logo 文件（Base64 编码） */
  logoBase64?: string;
  /** Logo 文件名 */
  logoFileName?: string;
}

/** 完整的安装配置 */
export interface SetupPayload {
  /** 数据库配置 */
  database: DatabaseConfig;
  /** 中间件配置 */
  middleware: MiddlewareConfig;
  /** LDAP 配置（可选） */
  ldap?: LDAPConfig;
  /** 管理员配置 */
  admin: AdminConfig;
  /** 品牌配置 */
  brand: BrandConfig;
  /** 是否导入演示数据 */
  importDemoData: boolean;
}

/** 连接测试结果 */
export interface TestResult {
  /** 是否连接成功 */
  success: boolean;
  /** 错误信息（失败时返回） */
  error?: string;
}

/** 初始化进度 */
export interface SetupProgress {
  /** 当前步骤描述 */
  step: string;
  /** 进度百分比（0-100） */
  percent: number;
  /** 是否已完成 */
  completed: boolean;
  /** 错误信息（失败时返回） */
  error?: string;
}

// ==================== API 函数 ====================

/**
 * 测试数据库连接
 * @param config - 数据库连接参数
 * @returns 测试结果
 */
export function testDatabase(config: DatabaseConfig): Promise<TestResult> {
  return request<TestResult>('/v1/setup/test/database', {
    method: 'POST',
    body: JSON.stringify(config),
  });
}

/**
 * 测试 Kafka 连接
 * @param address - Kafka Bootstrap 地址
 * @returns 测试结果
 */
export function testKafka(address: string): Promise<TestResult> {
  return request<TestResult>('/v1/setup/test/kafka', {
    method: 'POST',
    body: JSON.stringify({ address }),
  });
}

/**
 * 测试 Elasticsearch 连接
 * @param address - ES 地址
 * @returns 测试结果
 */
export function testElasticsearch(address: string): Promise<TestResult> {
  return request<TestResult>('/v1/setup/test/elasticsearch', {
    method: 'POST',
    body: JSON.stringify({ address }),
  });
}

/**
 * 测试 Redis 连接
 * @param address - Redis 地址
 * @returns 测试结果
 */
export function testRedis(address: string): Promise<TestResult> {
  return request<TestResult>('/v1/setup/test/redis', {
    method: 'POST',
    body: JSON.stringify({ address }),
  });
}

/**
 * 测试 LDAP 连接
 * @param config - LDAP 配置
 * @returns 测试结果
 */
export function testLDAP(config: LDAPConfig): Promise<TestResult> {
  return request<TestResult>('/v1/setup/test/ldap', {
    method: 'POST',
    body: JSON.stringify(config),
  });
}

/**
 * 提交安装配置并开始初始化
 * @param data - 完整安装配置
 */
export function startSetup(data: SetupPayload): Promise<void> {
  return request<void>('/v1/setup/initialize', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

/**
 * 查询初始化进度
 * 轮询此接口获取初始化进度，直到 completed 为 true
 * @returns 当前初始化进度
 */
export function getSetupProgress(): Promise<SetupProgress> {
  return request<SetupProgress>('/v1/setup/progress');
}
