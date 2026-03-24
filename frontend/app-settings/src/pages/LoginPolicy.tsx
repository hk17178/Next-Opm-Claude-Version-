/**
 * 登录安全策略配置页面
 * 管理系统登录相关的安全策略，包含四个 Tab：
 * - 登录失败锁定配置（失败次数 / 锁定时长）
 * - 并发登录限制（最大设备数 / 冲突策略）
 * - 异地登录检测（开关 / 通知方式 / 白名单城市）
 * - 密码策略（最小长度 / 复杂度 / 过期天数 / 历史密码限制）
 */
import React, { useState } from 'react';
import {
  Card, Tabs, Form, InputNumber, Switch, Select, Button, Space, Typography,
  Row, Col, Divider, Tag, message,
} from 'antd';
import {
  LockOutlined, TeamOutlined, EnvironmentOutlined, KeyOutlined,
  SaveOutlined, SafetyCertificateOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;

/* ========== 模拟数据 - 当前策略默认值 ========== */

/** 登录失败锁定策略默认值 */
const DEFAULT_LOCKOUT = {
  enabled: true,              // 是否启用锁定
  maxAttempts: 5,             // 最大失败次数
  lockDuration: 30,           // 锁定时长（分钟）
  resetWindow: 60,            // 失败计数重置窗口（分钟）
  captchaAfterAttempts: 3,    // 连续失败多少次后显示验证码
};

/** 并发登录限制默认值 */
const DEFAULT_CONCURRENT = {
  enabled: true,              // 是否启用并发限制
  maxDevices: 3,              // 最大同时登录设备数
  conflictStrategy: 'kick_oldest' as string, // 冲突策略：踢掉最早的 / 拒绝新登录
};

/** 异地登录检测默认值 */
const DEFAULT_GEO = {
  enabled: false,             // 是否启用异地检测
  notifyMethod: 'wecom' as string, // 通知方式
  blockLogin: false,          // 是否直接阻断异地登录
  whitelistCities: ['北京', '上海', '深圳'], // 白名单城市列表
};

/** 密码策略默认值 */
const DEFAULT_PASSWORD = {
  minLength: 8,               // 最小密码长度
  requireUppercase: true,     // 是否要求大写字母
  requireLowercase: true,     // 是否要求小写字母
  requireNumber: true,        // 是否要求数字
  requireSpecialChar: true,   // 是否要求特殊字符
  expirationDays: 90,         // 密码过期天数（0 为不过期）
  historyCount: 5,            // 不能重复使用的历史密码数
  expirationWarningDays: 14,  // 过期前提醒天数
};

/**
 * 登录安全策略组件
 * 提供四个 Tab 分别配置登录失败锁定、并发登录、异地检测、密码策略
 */
const LoginPolicy: React.FC = () => {
  const { t } = useTranslation('settings');

  /** 登录失败锁定策略表单 */
  const [lockoutForm] = Form.useForm();
  /** 并发登录限制表单 */
  const [concurrentForm] = Form.useForm();
  /** 异地登录检测表单 */
  const [geoForm] = Form.useForm();
  /** 密码策略表单 */
  const [passwordForm] = Form.useForm();
  /** 当前活动 Tab */
  const [activeTab, setActiveTab] = useState('lockout');
  /** 保存中状态 */
  const [saving, setSaving] = useState(false);

  /**
   * 通用保存处理函数
   * 校验当前 Tab 对应的表单，模拟保存操作
   * @param form - 要提交的表单实例
   * @param tabName - Tab 名称（用于提示）
   */
  const handleSave = async (form: ReturnType<typeof Form.useForm>[0], tabName: string) => {
    try {
      await form.validateFields();
      setSaving(true);
      // TODO: 对接后端 API 保存策略配置
      await new Promise((resolve) => setTimeout(resolve, 500));
      message.success(`${tabName}保存成功`);
    } catch {
      // 表单校验失败，antd 自动处理
    } finally {
      setSaving(false);
    }
  };

  /** Tab 配置项 */
  const tabItems = [
    {
      key: 'lockout',
      label: (
        <Space>
          <LockOutlined />
          <span>登录失败锁定</span>
        </Space>
      ),
      children: (
        <Form
          form={lockoutForm}
          layout="vertical"
          style={{ maxWidth: 600 }}
          initialValues={DEFAULT_LOCKOUT}
        >
          {/* 模块说明 */}
          <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            配置连续登录失败后的账号保护策略，防止暴力破解攻击
          </Text>

          {/* 启用锁定开关 */}
          <Form.Item
            name="enabled"
            label="启用登录失败锁定"
            valuePropName="checked"
          >
            <Switch checkedChildren="开" unCheckedChildren="关" />
          </Form.Item>

          <Row gutter={16}>
            <Col span={12}>
              {/* 最大失败次数 */}
              <Form.Item
                name="maxAttempts"
                label="最大允许失败次数"
                rules={[{ required: true, message: '请输入最大失败次数' }]}
                tooltip="连续失败达到此次数后锁定账号"
              >
                <InputNumber min={3} max={20} addonAfter="次" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              {/* 锁定时长 */}
              <Form.Item
                name="lockDuration"
                label="锁定时长"
                rules={[{ required: true, message: '请输入锁定时长' }]}
                tooltip="账号被锁定后的自动解锁时间"
              >
                <InputNumber min={5} max={1440} addonAfter="分钟" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              {/* 失败计数重置窗口 */}
              <Form.Item
                name="resetWindow"
                label="失败计数重置窗口"
                tooltip="在此时间窗口内无失败尝试，失败计数将重置为 0"
              >
                <InputNumber min={10} max={1440} addonAfter="分钟" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              {/* 验证码触发次数 */}
              <Form.Item
                name="captchaAfterAttempts"
                label="验证码触发次数"
                tooltip="连续失败达到此次数后要求输入验证码"
              >
                <InputNumber min={1} max={10} addonAfter="次" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          {/* 保存按钮 */}
          <Form.Item>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              loading={saving}
              onClick={() => handleSave(lockoutForm, '登录失败锁定策略')}
            >
              保存配置
            </Button>
          </Form.Item>
        </Form>
      ),
    },
    {
      key: 'concurrent',
      label: (
        <Space>
          <TeamOutlined />
          <span>并发登录限制</span>
        </Space>
      ),
      children: (
        <Form
          form={concurrentForm}
          layout="vertical"
          style={{ maxWidth: 600 }}
          initialValues={DEFAULT_CONCURRENT}
        >
          {/* 模块说明 */}
          <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            限制同一用户在多个设备上同时登录，增强账号安全性
          </Text>

          {/* 启用并发限制开关 */}
          <Form.Item
            name="enabled"
            label="启用并发登录限制"
            valuePropName="checked"
          >
            <Switch checkedChildren="开" unCheckedChildren="关" />
          </Form.Item>

          {/* 最大同时登录设备数 */}
          <Form.Item
            name="maxDevices"
            label="最大同时登录设备数"
            rules={[{ required: true, message: '请输入最大设备数' }]}
            tooltip="同一用户允许同时在多少台设备上保持登录状态"
          >
            <InputNumber min={1} max={10} addonAfter="台" style={{ width: 200 }} />
          </Form.Item>

          {/* 冲突策略 */}
          <Form.Item
            name="conflictStrategy"
            label="超出限制时的处理策略"
            rules={[{ required: true, message: '请选择冲突策略' }]}
            tooltip="当用户在新设备登录且已达设备上限时的处理方式"
          >
            <Select
              style={{ width: 300 }}
              options={[
                { value: 'kick_oldest', label: '踢掉最早登录的设备' },
                { value: 'deny_new', label: '拒绝新设备登录' },
                { value: 'notify_and_kick', label: '通知用户并踢掉最早设备' },
              ]}
            />
          </Form.Item>

          {/* 保存按钮 */}
          <Form.Item>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              loading={saving}
              onClick={() => handleSave(concurrentForm, '并发登录限制')}
            >
              保存配置
            </Button>
          </Form.Item>
        </Form>
      ),
    },
    {
      key: 'geo',
      label: (
        <Space>
          <EnvironmentOutlined />
          <span>异地登录检测</span>
        </Space>
      ),
      children: (
        <Form
          form={geoForm}
          layout="vertical"
          style={{ maxWidth: 600 }}
          initialValues={DEFAULT_GEO}
        >
          {/* 模块说明 */}
          <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            检测用户登录地理位置异常，防止账号被异地盗用
          </Text>

          {/* 启用异地检测开关 */}
          <Form.Item
            name="enabled"
            label="启用异地登录检测"
            valuePropName="checked"
          >
            <Switch checkedChildren="开" unCheckedChildren="关" />
          </Form.Item>

          {/* 异地登录通知方式 */}
          <Form.Item
            name="notifyMethod"
            label="异地登录通知方式"
            tooltip="检测到异地登录后的通知方式"
          >
            <Select
              style={{ width: 300 }}
              options={[
                { value: 'wecom', label: '企业微信推送' },
                { value: 'email', label: '邮件通知' },
                { value: 'sms', label: '短信通知' },
                { value: 'all', label: '全部渠道通知' },
              ]}
            />
          </Form.Item>

          {/* 是否阻断异地登录 */}
          <Form.Item
            name="blockLogin"
            label="直接阻断异地登录"
            valuePropName="checked"
            tooltip="开启后异地登录将被直接拒绝，需管理员审批后放行"
          >
            <Switch checkedChildren="阻断" unCheckedChildren="仅通知" />
          </Form.Item>

          {/* 白名单城市 */}
          <Form.Item
            name="whitelistCities"
            label="白名单城市"
            tooltip="以下城市的登录不会触发异地登录告警"
          >
            <Select
              mode="tags"
              style={{ width: '100%' }}
              placeholder="输入城市名称后按回车添加"
              tokenSeparators={[',']}
            />
          </Form.Item>

          {/* 保存按钮 */}
          <Form.Item>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              loading={saving}
              onClick={() => handleSave(geoForm, '异地登录检测策略')}
            >
              保存配置
            </Button>
          </Form.Item>
        </Form>
      ),
    },
    {
      key: 'password',
      label: (
        <Space>
          <KeyOutlined />
          <span>密码策略</span>
        </Space>
      ),
      children: (
        <Form
          form={passwordForm}
          layout="vertical"
          style={{ maxWidth: 600 }}
          initialValues={DEFAULT_PASSWORD}
        >
          {/* 模块说明 */}
          <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            配置用户密码的安全要求，确保账号密码强度符合安全规范
          </Text>

          {/* 密码长度 */}
          <Title level={5}>密码长度要求</Title>
          <Form.Item
            name="minLength"
            label="最小密码长度"
            rules={[{ required: true, message: '请输入最小密码长度' }]}
          >
            <InputNumber min={6} max={32} addonAfter="位" style={{ width: 200 }} />
          </Form.Item>

          <Divider />

          {/* 密码复杂度要求 */}
          <Title level={5}>密码复杂度要求</Title>
          <Row gutter={[16, 0]}>
            <Col span={12}>
              {/* 要求包含大写字母 */}
              <Form.Item name="requireUppercase" label="要求包含大写字母" valuePropName="checked">
                <Switch checkedChildren="是" unCheckedChildren="否" />
              </Form.Item>
            </Col>
            <Col span={12}>
              {/* 要求包含小写字母 */}
              <Form.Item name="requireLowercase" label="要求包含小写字母" valuePropName="checked">
                <Switch checkedChildren="是" unCheckedChildren="否" />
              </Form.Item>
            </Col>
            <Col span={12}>
              {/* 要求包含数字 */}
              <Form.Item name="requireNumber" label="要求包含数字" valuePropName="checked">
                <Switch checkedChildren="是" unCheckedChildren="否" />
              </Form.Item>
            </Col>
            <Col span={12}>
              {/* 要求包含特殊字符 */}
              <Form.Item name="requireSpecialChar" label="要求包含特殊字符" valuePropName="checked">
                <Switch checkedChildren="是" unCheckedChildren="否" />
              </Form.Item>
            </Col>
          </Row>

          <Divider />

          {/* 密码有效期 */}
          <Title level={5}>密码有效期</Title>
          <Row gutter={16}>
            <Col span={12}>
              {/* 密码过期天数 */}
              <Form.Item
                name="expirationDays"
                label="密码过期天数"
                tooltip="设置为 0 表示密码永不过期"
              >
                <InputNumber min={0} max={365} addonAfter="天" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              {/* 过期前提醒天数 */}
              <Form.Item
                name="expirationWarningDays"
                label="过期前提醒天数"
                tooltip="密码即将过期时提前通知用户修改密码"
              >
                <InputNumber min={1} max={30} addonAfter="天" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          {/* 历史密码限制 */}
          <Form.Item
            name="historyCount"
            label="历史密码不可重复次数"
            tooltip="新密码不能与最近 N 次使用过的密码相同"
          >
            <InputNumber min={0} max={24} addonAfter="次" style={{ width: 200 }} />
          </Form.Item>

          {/* 密码强度说明 */}
          <div style={{ padding: 12, background: '#F7F8FA', borderRadius: 6, marginBottom: 16 }}>
            <Text type="secondary">当前密码策略要求示例：</Text>
            <div style={{ marginTop: 8 }}>
              <Tag color="blue">至少 8 位</Tag>
              <Tag color="green">大写字母</Tag>
              <Tag color="green">小写字母</Tag>
              <Tag color="green">数字</Tag>
              <Tag color="green">特殊字符</Tag>
              <Tag color="orange">90 天过期</Tag>
            </div>
          </div>

          {/* 保存按钮 */}
          <Form.Item>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              loading={saving}
              onClick={() => handleSave(passwordForm, '密码策略')}
            >
              保存配置
            </Button>
          </Form.Item>
        </Form>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>
          <SafetyCertificateOutlined style={{ marginRight: 8 }} />
          登录安全策略
        </Text>
      </div>

      {/* 策略配置 Tabs */}
      <Card style={{ borderRadius: 8 }}>
        <Tabs items={tabItems} activeKey={activeTab} onChange={setActiveTab} />
      </Card>
    </div>
  );
};

export default LoginPolicy;
