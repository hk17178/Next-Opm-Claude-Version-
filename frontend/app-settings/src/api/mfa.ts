/**
 * MFA（多因素认证）管理 API 模块
 * 提供 TOTP 绑定、验证、禁用、恢复码等接口
 * 对应后端 svc-auth 服务，路由前缀 /v1/auth/mfa
 */
import { request } from './request';

// ==================== 类型定义 ====================

/** MFA 状态数据结构 */
export interface MFAStatus {
  /** 是否已启用 MFA */
  enabled: boolean;
  /** MFA 类型（当前仅支持 TOTP） */
  type: 'totp' | null;
  /** 绑定时间（未绑定时为 null） */
  boundAt: string | null;
}

/** TOTP 密钥生成结果 */
export interface TOTPSecret {
  /** TOTP 密钥（Base32 编码） */
  secret: string;
  /** 二维码图片 Data URL（base64 PNG） */
  qrCodeDataURL: string;
  /** otpauth:// 协议 URI，可手动输入到认证器 */
  otpauthURL: string;
}

/** MFA 恢复码列表 */
export interface RecoveryCodes {
  /** 恢复码数组（8 个一次性恢复码） */
  codes: string[];
}

// ==================== API 函数 ====================

/**
 * 获取当前用户的 MFA 状态
 * @returns MFA 启用状态和绑定信息
 */
export function getMFAStatus(): Promise<MFAStatus> {
  return request<MFAStatus>('/v1/auth/mfa/status');
}

/**
 * 生成 TOTP 密钥和二维码
 * 用户扫描二维码后需调用 verifyAndEnableMFA 完成绑定
 * @returns TOTP 密钥和二维码 Data URL
 */
export function generateTOTPSecret(): Promise<TOTPSecret> {
  return request<TOTPSecret>('/v1/auth/mfa/totp/generate', {
    method: 'POST',
  });
}

/**
 * 验证 TOTP 验证码并启用 MFA
 * 用户扫描二维码后输入 6 位验证码，验证成功即完成绑定
 * @param code - 6 位 TOTP 验证码
 * @returns 绑定成功后返回恢复码
 */
export function verifyAndEnableMFA(code: string): Promise<RecoveryCodes> {
  return request<RecoveryCodes>('/v1/auth/mfa/totp/verify', {
    method: 'POST',
    body: JSON.stringify({ code }),
  });
}

/**
 * 禁用 MFA
 * 需输入当前 TOTP 验证码确认身份后方可禁用
 * @param code - 6 位 TOTP 验证码
 */
export function disableMFA(code: string): Promise<void> {
  return request<void>('/v1/auth/mfa/disable', {
    method: 'POST',
    body: JSON.stringify({ code }),
  });
}

/**
 * 获取恢复码列表
 * 仅在 MFA 启用状态下可用，返回当前有效的恢复码
 * @returns 恢复码数组
 */
export function getRecoveryCodes(): Promise<RecoveryCodes> {
  return request<RecoveryCodes>('/v1/auth/mfa/recovery-codes');
}
