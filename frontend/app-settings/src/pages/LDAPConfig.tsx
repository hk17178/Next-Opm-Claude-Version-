/**
 * LDAP 配置页面
 * 管理 LDAP/Active Directory 集成配置，实现用户目录同步
 *
 * 功能模块：
 * - LDAP 服务器连接配置表单（服务器地址 / 端口 / Base DN / 绑定用户 / 密码）
 * - 字段映射配置（用户名 / 邮箱 / 部门 / 电话 → LDAP 属性映射）
 * - 测试连接按钮
 * - 同步设置（自动同步间隔 / 全量同步按钮）
 * - 同步日志表格（时间 / 类型 / 状态 / 同步数量 / 耗时）
 */
import React, { useState, useCallback } from 'react';
import {
  Card, Form, Input, InputNumber, Select, Switch, Button, Space, Typography,
  Row, Col, Divider, Table, Tag, message, Modal, Badge, Tabs,
} from 'antd';
import {
  SaveOutlined, ApiOutlined, SyncOutlined, CloudServerOutlined,
  LinkOutlined, FieldBinaryOutlined, ClockCircleOutlined,
  CheckCircleOutlined, CloseCircleOutlined, ExclamationCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;

/* ========== 类型定义 ========== */

/** 同步日志记录数据结构 */
interface SyncLog {
  id: string;              // 日志唯一标识
  time: string;            // 同步时间
  type: 'full' | 'incremental'; // 同步类型：全量 / 增量
  status: 'success' | 'partial' | 'failed'; // 同步结果状态
  usersAdded: number;      // 新增用户数
  usersUpdated: number;    // 更新用户数
  usersDisabled: number;   // 禁用用户数
  duration: number;        // 耗时（秒）
  errorMessage?: string;   // 错误信息（失败时）
}

/* ========== 常量配置 ========== */

/** 同步状态对应的标签颜色和文字 */
const SYNC_STATUS_CONFIG: Record<string, { color: string; text: string }> = {
  success: { color: '#52C41A', text: '成功' },
  partial: { color: '#FAAD14', text: '部分成功' },
  failed: { color: '#F5222D', text: '失败' },
};

/* ========== 模拟数据 ========== */

/** 模拟同步日志 */
const mockSyncLogs: SyncLog[] = [
  {
    id: '1', time: '2026-03-24 06:00:00', type: 'incremental',
    status: 'success', usersAdded: 2, usersUpdated: 15, usersDisabled: 0,
    duration: 12,
  },
  {
    id: '2', time: '2026-03-23 06:00:00', type: 'incremental',
    status: 'success', usersAdded: 0, usersUpdated: 8, usersDisabled: 1,
    duration: 9,
  },
  {
    id: '3', time: '2026-03-22 02:00:00', type: 'full',
    status: 'success', usersAdded: 0, usersUpdated: 156, usersDisabled: 3,
    duration: 45,
  },
  {
    id: '4', time: '2026-03-21 06:00:00', type: 'incremental',
    status: 'partial', usersAdded: 1, usersUpdated: 5, usersDisabled: 0,
    duration: 18, errorMessage: '部分用户同步超时（3/8 失败）',
  },
  {
    id: '5', time: '2026-03-20 06:00:00', type: 'incremental',
    status: 'failed', usersAdded: 0, usersUpdated: 0, usersDisabled: 0,
    duration: 3, errorMessage: 'LDAP 连接超时：无法连接到 ldap.company.com:636',
  },
  {
    id: '6', time: '2026-03-19 06:00:00', type: 'incremental',
    status: 'success', usersAdded: 5, usersUpdated: 22, usersDisabled: 0,
    duration: 14,
  },
];

/** LDAP 服务器配置默认值 */
const DEFAULT_SERVER_CONFIG = {
  host: 'ldap.company.com',
  port: 636,
  useTLS: true,
  baseDN: 'dc=company,dc=com',
  bindDN: 'cn=admin,dc=company,dc=com',
  bindPassword: '',
  searchFilter: '(objectClass=inetOrgPerson)',
  connectionTimeout: 10,
};

/** 字段映射默认值 */
const DEFAULT_FIELD_MAPPING = {
  username: 'uid',
  email: 'mail',
  displayName: 'cn',
  department: 'departmentNumber',
  phone: 'telephoneNumber',
  title: 'title',
  employeeId: 'employeeNumber',
};

/** 同步设置默认值 */
const DEFAULT_SYNC_CONFIG = {
  autoSyncEnabled: true,
  syncInterval: 360,
  syncOnLogin: true,
  disableOnRemove: true,
};

/**
 * LDAP 配置组件
 * 使用 Tabs 组织三个功能区域：连接配置、字段映射、同步设置
 */
const LDAPConfig: React.FC = () => {
  const { t } = useTranslation('settings');

  /** 服务器连接配置表单 */
  const [serverForm] = Form.useForm();
  /** 字段映射配置表单 */
  const [mappingForm] = Form.useForm();
  /** 同步设置表单 */
  const [syncForm] = Form.useForm();
  /** 当前活动 Tab */
  const [activeTab, setActiveTab] = useState('connection');
  /** 保存中状态 */
  const [saving, setSaving] = useState(false);
  /** 测试连接中状态 */
  const [testingConnection, setTestingConnection] = useState(false);
  /** 全量同步中状态 */
  const [syncing, setSyncing] = useState(false);
  /** 同步日志数据 */
  const [syncLogs] = useState<SyncLog[]>(mockSyncLogs);

  /**
   * 保存服务器连接配置
   * 校验表单后模拟保存到后端
   */
  const handleSaveServer = useCallback(async () => {
    try {
      await serverForm.validateFields();
      setSaving(true);
      // TODO: 对接后端 LDAP 配置保存 API
      await new Promise((resolve) => setTimeout(resolve, 500));
      message.success('LDAP 连接配置保存成功');
    } catch {
      // 表单校验失败
    } finally {
      setSaving(false);
    }
  }, [serverForm]);

  /**
   * 保存字段映射配置
   */
  const handleSaveMapping = useCallback(async () => {
    try {
      await mappingForm.validateFields();
      setSaving(true);
      await new Promise((resolve) => setTimeout(resolve, 500));
      message.success('字段映射配置保存成功');
    } catch {
      // 表单校验失败
    } finally {
      setSaving(false);
    }
  }, [mappingForm]);

  /**
   * 保存同步设置
   */
  const handleSaveSync = useCallback(async () => {
    try {
      await syncForm.validateFields();
      setSaving(true);
      await new Promise((resolve) => setTimeout(resolve, 500));
      message.success('同步设置保存成功');
    } catch {
      // 表单校验失败
    } finally {
      setSaving(false);
    }
  }, [syncForm]);

  /**
   * 测试 LDAP 连接
   * 使用当前表单配置尝试连接 LDAP 服务器
   */
  const handleTestConnection = useCallback(async () => {
    try {
      await serverForm.validateFields();
      setTestingConnection(true);
      // TODO: 对接后端 LDAP 连接测试 API
      await new Promise((resolve) => setTimeout(resolve, 1500));
      message.success('LDAP 连接测试成功！服务器响应正常');
    } catch (err) {
      if (err instanceof Error) {
        message.error(`连接测试失败：${err.message}`);
      }
    } finally {
      setTestingConnection(false);
    }
  }, [serverForm]);

  /**
   * 执行全量同步
   * 从 LDAP 服务器拉取所有用户数据并更新本地用户表
   */
  const handleFullSync = useCallback(() => {
    Modal.confirm({
      title: '执行全量同步',
      icon: <ExclamationCircleOutlined />,
      content: '全量同步将从 LDAP 服务器重新拉取所有用户数据，可能需要较长时间。确定要继续吗？',
      okText: '开始同步',
      cancelText: '取消',
      onOk: async () => {
        setSyncing(true);
        // TODO: 对接后端全量同步 API
        await new Promise((resolve) => setTimeout(resolve, 2000));
        message.success('全量同步已完成，共同步 156 个用户');
        setSyncing(false);
      },
    });
  }, []);

  /** 同步日志表格列定义 */
  const logColumns = [
    {
      title: '同步时间',
      dataIndex: 'time',
      key: 'time',
      width: 180,
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 90,
      /** 渲染同步类型标签 */
      render: (type: string) => (
        <Tag color={type === 'full' ? 'blue' : 'cyan'}>
          {type === 'full' ? '全量' : '增量'}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      /** 渲染同步状态标签 */
      render: (status: string) => {
        const cfg = SYNC_STATUS_CONFIG[status];
        return (
          <Tag
            color={cfg?.color}
            icon={
              status === 'success' ? <CheckCircleOutlined /> :
              status === 'failed' ? <CloseCircleOutlined /> :
              <ExclamationCircleOutlined />
            }
          >
            {cfg?.text || status}
          </Tag>
        );
      },
    },
    {
      title: '新增',
      dataIndex: 'usersAdded',
      key: 'usersAdded',
      width: 70,
      /** 新增用户数 > 0 时绿色高亮 */
      render: (count: number) => (
        <Text type={count > 0 ? 'success' : 'secondary'}>{count}</Text>
      ),
    },
    {
      title: '更新',
      dataIndex: 'usersUpdated',
      key: 'usersUpdated',
      width: 70,
      render: (count: number) => (
        <Text type={count > 0 ? undefined : 'secondary'}>{count}</Text>
      ),
    },
    {
      title: '禁用',
      dataIndex: 'usersDisabled',
      key: 'usersDisabled',
      width: 70,
      /** 禁用用户数 > 0 时橙色高亮 */
      render: (count: number) => (
        <Text type={count > 0 ? 'warning' : 'secondary'}>{count}</Text>
      ),
    },
    {
      title: '耗时',
      dataIndex: 'duration',
      key: 'duration',
      width: 80,
      /** 渲染耗时，自动转换单位 */
      render: (seconds: number) => (
        <Text>{seconds < 60 ? `${seconds} 秒` : `${Math.floor(seconds / 60)} 分 ${seconds % 60} 秒`}</Text>
      ),
    },
    {
      title: '错误信息',
      dataIndex: 'errorMessage',
      key: 'errorMessage',
      ellipsis: true,
      /** 无错误信息时显示 "--" */
      render: (msg: string | undefined) =>
        msg ? <Text type="danger" style={{ fontSize: 12 }}>{msg}</Text> : <Text type="secondary">--</Text>,
    },
  ];

  /** Tab 配置项 */
  const tabItems = [
    {
      key: 'connection',
      label: (
        <Space>
          <CloudServerOutlined />
          <span>连接配置</span>
        </Space>
      ),
      children: (
        <Form
          form={serverForm}
          layout="vertical"
          style={{ maxWidth: 600 }}
          initialValues={DEFAULT_SERVER_CONFIG}
        >
          {/* 模块说明 */}
          <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            配置 LDAP/Active Directory 服务器的连接参数
          </Text>

          <Row gutter={16}>
            <Col span={16}>
              {/* LDAP 服务器地址 */}
              <Form.Item
                name="host"
                label="服务器地址"
                rules={[{ required: true, message: '请输入 LDAP 服务器地址' }]}
              >
                <Input placeholder="ldap.company.com" />
              </Form.Item>
            </Col>
            <Col span={8}>
              {/* LDAP 端口 */}
              <Form.Item
                name="port"
                label="端口"
                rules={[{ required: true, message: '请输入端口' }]}
              >
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          {/* TLS 加密开关 */}
          <Form.Item
            name="useTLS"
            label="使用 TLS/SSL 加密"
            valuePropName="checked"
            tooltip="生产环境强烈建议启用 TLS 加密连接"
          >
            <Switch checkedChildren="LDAPS" unCheckedChildren="LDAP" />
          </Form.Item>

          {/* Base DN */}
          <Form.Item
            name="baseDN"
            label="Base DN（搜索基准）"
            rules={[{ required: true, message: '请输入 Base DN' }]}
            tooltip="LDAP 搜索的起始目录节点，如 dc=company,dc=com"
          >
            <Input placeholder="dc=company,dc=com" />
          </Form.Item>

          {/* 绑定用户 DN */}
          <Form.Item
            name="bindDN"
            label="绑定用户 DN"
            rules={[{ required: true, message: '请输入绑定用户 DN' }]}
            tooltip="用于连接 LDAP 服务器的管理员账号 DN"
          >
            <Input placeholder="cn=admin,dc=company,dc=com" />
          </Form.Item>

          {/* 绑定密码 */}
          <Form.Item
            name="bindPassword"
            label="绑定密码"
            rules={[{ required: true, message: '请输入绑定密码' }]}
          >
            <Input.Password placeholder="输入绑定账号的密码" />
          </Form.Item>

          {/* 用户搜索过滤条件 */}
          <Form.Item
            name="searchFilter"
            label="用户搜索过滤条件"
            tooltip="LDAP 搜索过滤器，用于筛选需要同步的用户对象"
          >
            <Input placeholder="(objectClass=inetOrgPerson)" />
          </Form.Item>

          {/* 连接超时 */}
          <Form.Item name="connectionTimeout" label="连接超时时间">
            <InputNumber min={5} max={60} addonAfter="秒" style={{ width: 200 }} />
          </Form.Item>

          {/* 操作按钮 */}
          <Form.Item>
            <Space>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={saving}
                onClick={handleSaveServer}
              >
                保存配置
              </Button>
              <Button
                icon={<LinkOutlined />}
                loading={testingConnection}
                onClick={handleTestConnection}
              >
                测试连接
              </Button>
            </Space>
          </Form.Item>
        </Form>
      ),
    },
    {
      key: 'mapping',
      label: (
        <Space>
          <FieldBinaryOutlined />
          <span>字段映射</span>
        </Space>
      ),
      children: (
        <Form
          form={mappingForm}
          layout="vertical"
          style={{ maxWidth: 600 }}
          initialValues={DEFAULT_FIELD_MAPPING}
        >
          {/* 模块说明 */}
          <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            配置系统用户字段与 LDAP 属性的对应关系
          </Text>

          {/* 用户名映射 */}
          <Form.Item
            name="username"
            label="用户名 → LDAP 属性"
            rules={[{ required: true, message: '请输入用户名对应的 LDAP 属性' }]}
            tooltip="通常为 uid 或 sAMAccountName（Active Directory）"
          >
            <Input placeholder="uid" addonBefore="用户名" />
          </Form.Item>

          {/* 邮箱映射 */}
          <Form.Item
            name="email"
            label="邮箱 → LDAP 属性"
            rules={[{ required: true, message: '请输入邮箱对应的 LDAP 属性' }]}
          >
            <Input placeholder="mail" addonBefore="邮箱" />
          </Form.Item>

          {/* 显示名称映射 */}
          <Form.Item
            name="displayName"
            label="显示名称 → LDAP 属性"
            tooltip="通常为 cn 或 displayName"
          >
            <Input placeholder="cn" addonBefore="姓名" />
          </Form.Item>

          {/* 部门映射 */}
          <Form.Item
            name="department"
            label="部门 → LDAP 属性"
            tooltip="通常为 departmentNumber 或 department"
          >
            <Input placeholder="departmentNumber" addonBefore="部门" />
          </Form.Item>

          {/* 电话映射 */}
          <Form.Item
            name="phone"
            label="电话 → LDAP 属性"
          >
            <Input placeholder="telephoneNumber" addonBefore="电话" />
          </Form.Item>

          {/* 职位映射 */}
          <Form.Item
            name="title"
            label="职位 → LDAP 属性"
          >
            <Input placeholder="title" addonBefore="职位" />
          </Form.Item>

          {/* 工号映射 */}
          <Form.Item
            name="employeeId"
            label="工号 → LDAP 属性"
          >
            <Input placeholder="employeeNumber" addonBefore="工号" />
          </Form.Item>

          {/* 保存按钮 */}
          <Form.Item>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              loading={saving}
              onClick={handleSaveMapping}
            >
              保存映射配置
            </Button>
          </Form.Item>
        </Form>
      ),
    },
    {
      key: 'sync',
      label: (
        <Space>
          <SyncOutlined />
          <span>同步设置</span>
        </Space>
      ),
      children: (
        <div>
          {/* 同步策略配置 */}
          <Form
            form={syncForm}
            layout="vertical"
            style={{ maxWidth: 600 }}
            initialValues={DEFAULT_SYNC_CONFIG}
          >
            {/* 模块说明 */}
            <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
              配置 LDAP 用户数据的自动同步策略
            </Text>

            {/* 启用自动同步 */}
            <Form.Item
              name="autoSyncEnabled"
              label="启用自动同步"
              valuePropName="checked"
              tooltip="开启后系统将按照设定间隔自动从 LDAP 同步用户数据"
            >
              <Switch checkedChildren="开" unCheckedChildren="关" />
            </Form.Item>

            {/* 自动同步间隔 */}
            <Form.Item
              name="syncInterval"
              label="自动同步间隔"
              tooltip="两次增量同步之间的时间间隔"
            >
              <Select
                style={{ width: 200 }}
                options={[
                  { value: 60, label: '每 1 小时' },
                  { value: 180, label: '每 3 小时' },
                  { value: 360, label: '每 6 小时' },
                  { value: 720, label: '每 12 小时' },
                  { value: 1440, label: '每 24 小时' },
                ]}
              />
            </Form.Item>

            {/* 登录时同步 */}
            <Form.Item
              name="syncOnLogin"
              label="用户登录时同步"
              valuePropName="checked"
              tooltip="LDAP 用户登录时实时从 LDAP 同步最新属性"
            >
              <Switch checkedChildren="是" unCheckedChildren="否" />
            </Form.Item>

            {/* LDAP 中移除后禁用本地账号 */}
            <Form.Item
              name="disableOnRemove"
              label="LDAP 中删除时禁用本地账号"
              valuePropName="checked"
              tooltip="当用户从 LDAP 中移除后，自动禁用其在本系统中的账号"
            >
              <Switch checkedChildren="是" unCheckedChildren="否" />
            </Form.Item>

            {/* 操作按钮 */}
            <Form.Item>
              <Space>
                <Button
                  type="primary"
                  icon={<SaveOutlined />}
                  loading={saving}
                  onClick={handleSaveSync}
                >
                  保存同步设置
                </Button>
                <Button
                  icon={<SyncOutlined />}
                  loading={syncing}
                  onClick={handleFullSync}
                >
                  立即执行全量同步
                </Button>
              </Space>
            </Form.Item>
          </Form>

          <Divider />

          {/* 同步日志表格 */}
          <Title level={5}>
            <ClockCircleOutlined style={{ marginRight: 8 }} />
            同步日志
          </Title>
          <Table
            columns={logColumns}
            dataSource={syncLogs}
            rowKey="id"
            size="middle"
            locale={{ emptyText: '暂无同步日志' }}
            pagination={{
              pageSize: 10,
              showSizeChanger: true,
              showTotal: (total) => `共 ${total} 条`,
            }}
          />
        </div>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>
          <CloudServerOutlined style={{ marginRight: 8 }} />
          LDAP 配置
        </Text>
        {/* 连接状态指示 */}
        <Space>
          <Badge status="success" />
          <Text type="secondary">LDAP 服务器已连接</Text>
        </Space>
      </div>

      {/* 配置 Tabs */}
      <Card style={{ borderRadius: 8 }}>
        <Tabs items={tabItems} activeKey={activeTab} onChange={setActiveTab} />
      </Card>
    </div>
  );
};

export default LDAPConfig;
