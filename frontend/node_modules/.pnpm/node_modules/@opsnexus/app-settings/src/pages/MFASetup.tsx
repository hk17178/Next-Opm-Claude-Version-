/**
 * MFA（多因素认证）设置页面
 * 展示当前 MFA 状态，提供启用/禁用 MFA 的完整流程
 *
 * 核心交互逻辑：
 * - 未启用时：显示"启用 MFA"按钮，点击后进入 TOTP 绑定流程
 *   1. 调用后端获取 TOTP 密钥和二维码 URL
 *   2. 展示二维码供用户扫码
 *   3. 用户输入 6 位验证码确认绑定
 *   4. 绑定成功后展示 8 个恢复码（提示用户下载保存）
 * - 已启用时：显示 MFA 信息和"禁用 MFA"按钮
 *   - 禁用需输入当前验证码确认
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Button, Typography, Space, Modal, Input, message, Skeleton, Tag, Alert, Divider,
  Steps, Result,
} from 'antd';
import {
  SafetyCertificateOutlined, LockOutlined, DownloadOutlined, CheckCircleOutlined,
} from '@ant-design/icons';
import { getMFAStatus, generateTOTPSecret, verifyAndEnableMFA, disableMFA } from '../api/mfa';
import type { MFAStatus, TOTPSecret } from '../api/mfa';

const { Text, Title, Paragraph } = Typography;

/**
 * MFA 设置组件
 * - 主卡片：展示 MFA 启用状态和操作入口
 * - 启用流程：分步引导（生成密钥 → 扫码 → 输入验证码 → 展示恢复码）
 * - 禁用流程：输入验证码确认弹窗
 */
const MFASetup: React.FC = () => {
  /** MFA 状态数据 */
  const [mfaStatus, setMfaStatus] = useState<MFAStatus | null>(null);
  /** 页面加载状态 */
  const [loading, setLoading] = useState(true);
  /** 启用 MFA 流程弹窗是否打开 */
  const [enableModalOpen, setEnableModalOpen] = useState(false);
  /** 禁用 MFA 确认弹窗是否打开 */
  const [disableModalOpen, setDisableModalOpen] = useState(false);
  /** 当前启用流程步骤（0: 生成二维码, 1: 验证码确认, 2: 展示恢复码） */
  const [currentStep, setCurrentStep] = useState(0);
  /** TOTP 密钥和二维码数据 */
  const [totpSecret, setTotpSecret] = useState<TOTPSecret | null>(null);
  /** 用户输入的验证码 */
  const [verifyCode, setVerifyCode] = useState('');
  /** 恢复码列表 */
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  /** 操作中状态 */
  const [submitting, setSubmitting] = useState(false);
  /** 禁用确认的验证码输入 */
  const [disableCode, setDisableCode] = useState('');

  /**
   * 加载 MFA 状态
   */
  const fetchStatus = useCallback(async () => {
    setLoading(true);
    try {
      const status = await getMFAStatus();
      setMfaStatus(status);
    } catch {
      message.error('获取 MFA 状态失败');
    } finally {
      setLoading(false);
    }
  }, []);

  /** 页面首次加载 */
  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  /**
   * 开始启用 MFA 流程
   * 调用后端生成 TOTP 密钥和二维码，然后打开步骤弹窗
   */
  const handleStartEnable = useCallback(async () => {
    setSubmitting(true);
    try {
      const secret = await generateTOTPSecret();
      setTotpSecret(secret);
      setCurrentStep(0);
      setVerifyCode('');
      setRecoveryCodes([]);
      setEnableModalOpen(true);
    } catch {
      message.error('生成 TOTP 密钥失败');
    } finally {
      setSubmitting(false);
    }
  }, []);

  /**
   * 提交验证码完成 MFA 绑定
   * 验证成功后进入恢复码展示步骤
   */
  const handleVerify = useCallback(async () => {
    if (verifyCode.length !== 6) {
      message.warning('请输入 6 位验证码');
      return;
    }
    setSubmitting(true);
    try {
      const result = await verifyAndEnableMFA(verifyCode);
      setRecoveryCodes(result.codes);
      setCurrentStep(2);
      message.success('MFA 绑定成功');
    } catch {
      message.error('验证码错误，请重试');
    } finally {
      setSubmitting(false);
    }
  }, [verifyCode]);

  /**
   * 关闭启用弹窗并刷新状态
   */
  const handleCloseEnableModal = useCallback(() => {
    setEnableModalOpen(false);
    setTotpSecret(null);
    setVerifyCode('');
    setRecoveryCodes([]);
    fetchStatus();
  }, [fetchStatus]);

  /**
   * 禁用 MFA
   * 需输入当前 TOTP 验证码确认身份
   */
  const handleDisable = useCallback(async () => {
    if (disableCode.length !== 6) {
      message.warning('请输入 6 位验证码');
      return;
    }
    setSubmitting(true);
    try {
      await disableMFA(disableCode);
      message.success('MFA 已禁用');
      setDisableModalOpen(false);
      setDisableCode('');
      fetchStatus();
    } catch {
      message.error('验证码错误，禁用失败');
    } finally {
      setSubmitting(false);
    }
  }, [disableCode, fetchStatus]);

  /**
   * 下载恢复码为文本文件
   * 生成包含所有恢复码的 .txt 文件供用户保存
   */
  const handleDownloadCodes = useCallback(() => {
    const content = [
      'OpsNexus MFA 恢复码',
      '====================',
      '请妥善保存以下恢复码，每个恢复码只能使用一次。',
      '',
      ...recoveryCodes.map((code, i) => `${i + 1}. ${code}`),
      '',
      `生成时间：${new Date().toLocaleString('zh-CN')}`,
    ].join('\n');

    const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'opsnexus-recovery-codes.txt';
    a.click();
    URL.revokeObjectURL(url);
  }, [recoveryCodes]);

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>多因素认证 (MFA)</Text>
      </div>

      <Card style={{ borderRadius: 8 }}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 3 }} />
        ) : mfaStatus ? (
          <div>
            {/* MFA 状态展示 */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 24 }}>
              <SafetyCertificateOutlined style={{ fontSize: 48, color: mfaStatus.enabled ? '#00B42A' : '#86909C' }} />
              <div>
                <div style={{ marginBottom: 4 }}>
                  <Text strong style={{ fontSize: 16, marginRight: 8 }}>MFA 状态</Text>
                  <Tag color={mfaStatus.enabled ? 'green' : 'default'}>
                    {mfaStatus.enabled ? '已启用' : '未启用'}
                  </Tag>
                </div>
                {mfaStatus.enabled && mfaStatus.boundAt && (
                  <Text type="secondary">
                    绑定时间：{new Date(mfaStatus.boundAt).toLocaleString('zh-CN')}
                  </Text>
                )}
                {!mfaStatus.enabled && (
                  <Text type="secondary">
                    启用 MFA 可为您的账号添加额外的安全保护层
                  </Text>
                )}
              </div>
            </div>

            {/* 操作按钮 */}
            {mfaStatus.enabled ? (
              <Button
                danger
                icon={<LockOutlined />}
                onClick={() => { setDisableCode(''); setDisableModalOpen(true); }}
              >
                禁用 MFA
              </Button>
            ) : (
              <Button
                type="primary"
                icon={<SafetyCertificateOutlined />}
                loading={submitting}
                onClick={handleStartEnable}
              >
                启用 MFA
              </Button>
            )}
          </div>
        ) : null}
      </Card>

      {/* 启用 MFA 流程弹窗 */}
      <Modal
        title="启用多因素认证"
        open={enableModalOpen}
        onCancel={currentStep === 2 ? handleCloseEnableModal : () => setEnableModalOpen(false)}
        footer={null}
        width={560}
        closable={currentStep === 2}
        maskClosable={false}
      >
        {/* 步骤指示器 */}
        <Steps
          current={currentStep}
          size="small"
          style={{ marginBottom: 24 }}
          items={[
            { title: '扫描二维码' },
            { title: '输入验证码' },
            { title: '保存恢复码' },
          ]}
        />

        {/* 步骤 1：展示二维码 */}
        {currentStep === 0 && totpSecret && (
          <div>
            <Paragraph>
              请使用 Google Authenticator、Microsoft Authenticator 或其他 TOTP 认证器扫描以下二维码：
            </Paragraph>
            {/* 二维码图片 */}
            <div style={{ textAlign: 'center', margin: '24px 0' }}>
              <img
                src={totpSecret.qrCodeDataURL}
                alt="TOTP QR Code"
                style={{ width: 200, height: 200, border: '1px solid #E5E6EB', borderRadius: 8, padding: 8 }}
              />
            </div>
            {/* 手动输入密钥 */}
            <Paragraph type="secondary" style={{ textAlign: 'center' }}>
              无法扫码？手动输入密钥：
            </Paragraph>
            <div style={{
              background: '#F7F8FA',
              borderRadius: 6,
              padding: '8px 12px',
              fontFamily: 'monospace',
              fontSize: 14,
              textAlign: 'center',
              wordBreak: 'break-all',
              marginBottom: 16,
            }}>
              {totpSecret.secret}
            </div>
            <div style={{ textAlign: 'right' }}>
              <Button type="primary" onClick={() => setCurrentStep(1)}>
                下一步
              </Button>
            </div>
          </div>
        )}

        {/* 步骤 2：输入验证码确认 */}
        {currentStep === 1 && (
          <div>
            <Paragraph>
              请输入认证器中显示的 6 位验证码以完成绑定：
            </Paragraph>
            <div style={{ textAlign: 'center', margin: '24px 0' }}>
              <Input
                value={verifyCode}
                onChange={(e) => setVerifyCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                placeholder="000000"
                maxLength={6}
                style={{ width: 200, fontSize: 24, textAlign: 'center', letterSpacing: 8 }}
              />
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between' }}>
              <Button onClick={() => setCurrentStep(0)}>上一步</Button>
              <Button type="primary" loading={submitting} onClick={handleVerify}>
                验证并绑定
              </Button>
            </div>
          </div>
        )}

        {/* 步骤 3：展示恢复码 */}
        {currentStep === 2 && (
          <div>
            <Result
              status="success"
              title="MFA 绑定成功"
              subTitle="请妥善保存以下恢复码，它们可在您无法使用认证器时帮助您恢复账号访问。"
              style={{ paddingBottom: 0 }}
            />
            <Alert
              type="warning"
              showIcon
              message="每个恢复码只能使用一次，请下载保存或抄写到安全的地方。"
              style={{ marginBottom: 16 }}
            />
            {/* 恢复码列表 */}
            <div style={{
              background: '#F7F8FA',
              borderRadius: 6,
              padding: 16,
              fontFamily: 'monospace',
              fontSize: 14,
              display: 'grid',
              gridTemplateColumns: '1fr 1fr',
              gap: '8px 24px',
              marginBottom: 16,
            }}>
              {recoveryCodes.map((code, index) => (
                <div key={code}>
                  <Text type="secondary">{index + 1}.</Text> {code}
                </div>
              ))}
            </div>
            <Space>
              <Button icon={<DownloadOutlined />} onClick={handleDownloadCodes}>
                下载恢复码
              </Button>
              <Button type="primary" onClick={handleCloseEnableModal}>
                完成
              </Button>
            </Space>
          </div>
        )}
      </Modal>

      {/* 禁用 MFA 确认弹窗 */}
      <Modal
        title="禁用多因素认证"
        open={disableModalOpen}
        onCancel={() => setDisableModalOpen(false)}
        onOk={handleDisable}
        confirmLoading={submitting}
        okText="确定禁用"
        cancelText="取消"
        okButtonProps={{ danger: true }}
      >
        <Alert
          type="warning"
          showIcon
          message="禁用 MFA 将降低账号安全性"
          description="禁用后，登录时将不再要求输入验证码。如需重新启用，需要重新绑定认证器。"
          style={{ marginBottom: 16 }}
        />
        <Paragraph>请输入当前认证器中的 6 位验证码以确认身份：</Paragraph>
        <Input
          value={disableCode}
          onChange={(e) => setDisableCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
          placeholder="000000"
          maxLength={6}
          style={{ width: 200, fontSize: 18, textAlign: 'center', letterSpacing: 6 }}
        />
      </Modal>
    </div>
  );
};

export default MFASetup;
