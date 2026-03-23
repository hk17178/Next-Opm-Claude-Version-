/**
 * 通知渠道配置页面 - 管理所有通知渠道（企微机器人、邮件、短信、语音、Webhook 等）
 * 支持渠道的增删改查、启用/禁用切换、发送测试通知、按类型/健康状态过滤
 */
import React, { useState, useCallback, useMemo, useEffect } from 'react';
import {
  Card, Table, Button, Space, Typography, Tag, Modal, Form, Input, Select, Switch, Badge, Tooltip,
  message,
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined, ExperimentOutlined,
  CheckCircleOutlined, WarningOutlined, CloseCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { fetchChannels, createChannel, updateChannel, deleteChannel, testChannel } from '../api/notify';

const { Text, Paragraph } = Typography;

/** 渠道类型枚举 */
type ChannelType = 'wecom_webhook' | 'wecom_app' | 'sms' | 'email' | 'voice_tts' | 'webhook';

/** 渠道健康状态枚举 */
type ChannelHealth = 'healthy' | 'degraded' | 'unavailable';

/** 渠道记录数据结构 */
interface ChannelRecord {
  key: string;              // 渠道唯一标识
  name: string;             // 渠道名称
  type: ChannelType;        // 渠道类型
  enabled: boolean;         // 是否启用
  health: ChannelHealth;    // 健康状态
  lastCheckTime: string;    // 最后检查时间
  description?: string;     // 渠道描述（可选）
}

/** 渠道类型对应的颜色映射 */
const CHANNEL_TYPE_COLORS: Record<ChannelType, string> = {
  wecom_webhook: '#2E75B6',  // 企微机器人
  wecom_app: '#4C9AE6',     // 企微应用
  sms: '#00B42A',           // 短信
  email: '#FF7D00',         // 邮件
  voice_tts: '#F53F3F',     // 语音
  webhook: '#722ED1',       // 自定义 Webhook
};

/** 健康状态配置（颜色、图标、Badge 状态） */
const HEALTH_CONFIG: Record<ChannelHealth, { color: string; icon: React.ReactNode; status: 'success' | 'warning' | 'error' }> = {
  healthy: { color: '#00B42A', icon: <CheckCircleOutlined />, status: 'success' },       // 健康
  degraded: { color: '#FF7D00', icon: <WarningOutlined />, status: 'warning' },          // 降级
  unavailable: { color: '#F53F3F', icon: <CloseCircleOutlined />, status: 'error' },     // 不可用
};

/** 所有渠道类型列表（用于下拉选择） */
const ALL_CHANNEL_TYPES: ChannelType[] = [
  'wecom_webhook', 'wecom_app', 'sms', 'email', 'voice_tts', 'webhook',
];

/**
 * 通知渠道配置组件
 * - 顶部：页面标题 + 添加渠道按钮
 * - 过滤栏：渠道类型、健康状态、启用状态
 * - 渠道列表表格：名称、类型、健康状态、启用开关、最后检查时间、操作
 * - 创建/编辑弹窗：基础信息 + 根据渠道类型动态展示的配置表单
 * - 删除确认弹窗
 */
const ChannelConfig: React.FC = () => {
  const { t } = useTranslation('notify');
  const [loading, setLoading] = useState(false);              // 表格加载状态
  const [data, setData] = useState<ChannelRecord[]>([]);      // 渠道列表数据
  const [modalOpen, setModalOpen] = useState(false);          // 创建/编辑弹窗是否打开
  const [deleteModalOpen, setDeleteModalOpen] = useState(false); // 删除确认弹窗
  const [editingChannel, setEditingChannel] = useState<ChannelRecord | null>(null); // 正在编辑的渠道
  const [deletingChannel, setDeletingChannel] = useState<ChannelRecord | null>(null); // 待删除的渠道
  const [selectedType, setSelectedType] = useState<ChannelType | undefined>(undefined); // 表单中选择的渠道类型
  const [testingKey, setTestingKey] = useState<string | null>(null); // 正在测试的渠道 key
  const [form] = Form.useForm();

  // 监听表单中渠道类型字段的变化，用于动态渲染配置表单
  const formChannelType = Form.useWatch('type', form) as ChannelType | undefined;

  /**
   * 加载渠道列表数据
   */
  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      // request<T> 已自动解包，fetchChannels 直接返回 Channel[]
      const result = await fetchChannels();
      setData(result || []);
    } catch {
      // API 尚未就绪
    } finally {
      setLoading(false);
    }
  }, []);

  /** 组件挂载时加载数据 */
  useEffect(() => {
    loadData();
  }, [loadData]);

  /**
   * 打开新增渠道弹窗
   * 重置表单和编辑状态
   */
  const handleAdd = useCallback(() => {
    setEditingChannel(null);
    form.resetFields();
    setSelectedType(undefined);
    setModalOpen(true);
  }, [form]);

  /**
   * 打开编辑渠道弹窗
   * 将当前渠道数据填充到表单
   * @param record 渠道记录
   */
  const handleEdit = useCallback((record: ChannelRecord) => {
    setEditingChannel(record);
    setSelectedType(record.type);
    form.setFieldsValue(record);
    setModalOpen(true);
  }, [form]);

  /**
   * 打开删除确认弹窗
   * @param record 待删除的渠道记录
   */
  const handleDelete = useCallback((record: ChannelRecord) => {
    setDeletingChannel(record);
    setDeleteModalOpen(true);
  }, []);

  /**
   * 确认删除渠道
   * 调用删除 API → 成功后刷新列表
   */
  const handleConfirmDelete = useCallback(async () => {
    if (!deletingChannel) return;
    try {
      await deleteChannel(deletingChannel.key);
      message.success(t('channel.deleteSuccess'));
      loadData();
    } catch {
      message.error(t('channel.deleteFail'));
    }
    setDeleteModalOpen(false);
    setDeletingChannel(null);
  }, [deletingChannel, loadData, t]);

  /**
   * 保存渠道（新增或更新）
   * 校验表单 → 根据是否有 editingChannel 判断是创建还是更新 → 成功后刷新列表
   */
  const handleSave = useCallback(async () => {
    try {
      const values = await form.validateFields();
      if (editingChannel) {
        // 编辑模式：更新渠道
        await updateChannel(editingChannel.key, values);
        message.success(t('channel.updateSuccess'));
      } else {
        // 新增模式：创建渠道
        await createChannel(values);
        message.success(t('channel.createSuccess'));
      }
      setModalOpen(false);
      setEditingChannel(null);
      loadData();
    } catch {
      // 表单校验失败或 API 错误
    }
  }, [form, editingChannel, loadData, t]);

  /**
   * 测试通知渠道 - 发送测试消息
   * @param record 渠道记录
   */
  const handleTest = useCallback(async (record: ChannelRecord) => {
    setTestingKey(record.key);
    try {
      await testChannel(record.key);
      message.success(t('channel.testSuccess'));
    } catch {
      message.error(t('channel.testFail'));
    } finally {
      setTestingKey(null);
    }
  }, [t]);

  /**
   * 切换渠道启用/禁用状态
   * @param record 渠道记录
   * @param enabled 目标启用状态
   */
  const handleToggleEnabled = useCallback(async (record: ChannelRecord, enabled: boolean) => {
    try {
      await updateChannel(record.key, { ...record, enabled });
      message.success(enabled ? t('channel.enabled') : t('channel.disabled'));
      loadData();
    } catch {
      message.error(t('channel.toggleFail'));
    }
  }, [loadData, t]);

  /**
   * 处理渠道类型变更（表单内）
   * @param value 选择的渠道类型
   */
  const handleTypeChange = useCallback((value: ChannelType) => {
    setSelectedType(value);
  }, []);

  /**
   * 根据渠道类型动态生成配置表单字段
   * 不同类型的渠道需要不同的配置项：
   * - wecom_webhook: Webhook URL、@所有人开关
   * - wecom_app: 企业 ID、应用 ID、应用密钥
   * - sms: 短信服务商、AccessKey、签名
   * - email: SMTP 主机、端口、用户名、密码、TLS 开关
   * - voice_tts: 语音服务商、AccessKey、被叫号码
   * - webhook: URL、HTTP 方法、自定义请求头
   */
  const dynamicFormFields = useMemo(() => {
    const type = formChannelType || selectedType;
    if (!type) return null;

    switch (type) {
      case 'wecom_webhook':
        return (
          <>
            <Form.Item name="webhookUrl" label={t('channel.form.webhookUrl')}
              rules={[{ required: true, message: t('channel.form.webhookUrlRequired') }]}>
              <Input placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..." />
            </Form.Item>
            <Form.Item name="mentionAll" label={t('channel.form.mentionAll')} valuePropName="checked">
              <Switch />
            </Form.Item>
          </>
        );
      case 'wecom_app':
        return (
          <>
            <Form.Item name="corpId" label={t('channel.form.corpId')}
              rules={[{ required: true, message: t('channel.form.corpIdRequired') }]}>
              <Input placeholder={t('channel.form.corpIdPlaceholder')} />
            </Form.Item>
            <Form.Item name="agentId" label={t('channel.form.agentId')}
              rules={[{ required: true, message: t('channel.form.agentIdRequired') }]}>
              <Input placeholder={t('channel.form.agentIdPlaceholder')} />
            </Form.Item>
            <Form.Item name="appSecret" label={t('channel.form.appSecret')}
              rules={[{ required: true, message: t('channel.form.appSecretRequired') }]}>
              <Input.Password placeholder={t('channel.form.appSecretPlaceholder')} />
            </Form.Item>
          </>
        );
      case 'sms':
        return (
          <>
            <Form.Item name="smsProvider" label={t('channel.form.smsProvider')}
              rules={[{ required: true, message: t('channel.form.smsProviderRequired') }]}>
              <Select placeholder={t('channel.form.smsProviderPlaceholder')} options={[
                { value: 'aliyun', label: t('channel.form.smsAliyun') },
                { value: 'tencent', label: t('channel.form.smsTencent') },
              ]} />
            </Form.Item>
            <Form.Item name="smsAccessKey" label={t('channel.form.smsAccessKey')}
              rules={[{ required: true, message: t('channel.form.smsAccessKeyRequired') }]}>
              <Input placeholder={t('channel.form.smsAccessKeyPlaceholder')} />
            </Form.Item>
            <Form.Item name="smsSignName" label={t('channel.form.smsSignName')}
              rules={[{ required: true, message: t('channel.form.smsSignNameRequired') }]}>
              <Input placeholder={t('channel.form.smsSignNamePlaceholder')} />
            </Form.Item>
          </>
        );
      case 'email':
        return (
          <>
            <Form.Item name="smtpHost" label={t('channel.form.smtpHost')}
              rules={[{ required: true, message: t('channel.form.smtpHostRequired') }]}>
              <Input placeholder="smtp.example.com" />
            </Form.Item>
            <Form.Item name="smtpPort" label={t('channel.form.smtpPort')}
              rules={[{ required: true, message: t('channel.form.smtpPortRequired') }]}>
              <Input placeholder="465" />
            </Form.Item>
            <Form.Item name="smtpUser" label={t('channel.form.smtpUser')}
              rules={[{ required: true, message: t('channel.form.smtpUserRequired') }]}>
              <Input placeholder={t('channel.form.smtpUserPlaceholder')} />
            </Form.Item>
            <Form.Item name="smtpPassword" label={t('channel.form.smtpPassword')}
              rules={[{ required: true, message: t('channel.form.smtpPasswordRequired') }]}>
              <Input.Password placeholder={t('channel.form.smtpPasswordPlaceholder')} />
            </Form.Item>
            <Form.Item name="smtpTls" label={t('channel.form.smtpTls')} valuePropName="checked" initialValue>
              <Switch />
            </Form.Item>
          </>
        );
      case 'voice_tts':
        return (
          <>
            <Form.Item name="voiceProvider" label={t('channel.form.voiceProvider')}
              rules={[{ required: true, message: t('channel.form.voiceProviderRequired') }]}>
              <Select placeholder={t('channel.form.voiceProviderPlaceholder')} options={[
                { value: 'aliyun', label: t('channel.form.smsAliyun') },
                { value: 'tencent', label: t('channel.form.smsTencent') },
              ]} />
            </Form.Item>
            <Form.Item name="voiceAccessKey" label={t('channel.form.voiceAccessKey')}
              rules={[{ required: true, message: t('channel.form.voiceAccessKeyRequired') }]}>
              <Input placeholder={t('channel.form.voiceAccessKeyPlaceholder')} />
            </Form.Item>
            <Form.Item name="voiceCalledNumber" label={t('channel.form.voiceCalledNumber')}>
              <Input placeholder={t('channel.form.voiceCalledNumberPlaceholder')} />
            </Form.Item>
          </>
        );
      case 'webhook':
        return (
          <>
            <Form.Item name="webhookUrl" label={t('channel.form.webhookUrl')}
              rules={[{ required: true, message: t('channel.form.webhookUrlRequired') }]}>
              <Input placeholder="https://hooks.example.com/notify" />
            </Form.Item>
            <Form.Item name="webhookMethod" label={t('channel.form.webhookMethod')} initialValue="POST">
              <Select options={[{ value: 'POST', label: 'POST' }, { value: 'PUT', label: 'PUT' }]} />
            </Form.Item>
            <Form.Item name="webhookHeaders" label={t('channel.form.webhookHeaders')}>
              <Input.TextArea rows={3} placeholder={'{"Authorization": "Bearer xxx"}'} />
            </Form.Item>
          </>
        );
      default:
        return null;
    }
  }, [formChannelType, selectedType, t]);

  /** 表格列定义 */
  const columns = [
    {
      title: t('channel.column.name'),
      dataIndex: 'name',
      key: 'name',
      /** 渲染渠道名称及描述 */
      render: (name: string, record: ChannelRecord) => (
        <div>
          <Text strong>{name}</Text>
          {record.description && (
            <Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 12 }} ellipsis={{ rows: 1 }}>
              {record.description}
            </Paragraph>
          )}
        </div>
      ),
    },
    {
      title: t('channel.column.type'),
      dataIndex: 'type',
      key: 'type',
      width: 150,
      /** 渲染渠道类型标签 */
      render: (type: ChannelType) => (
        <Tag color={CHANNEL_TYPE_COLORS[type]}>{t(`channel.type.${type}`)}</Tag>
      ),
    },
    {
      title: t('channel.column.health'),
      dataIndex: 'health',
      key: 'health',
      width: 120,
      /** 渲染健康状态 Badge */
      render: (health: ChannelHealth) => {
        const cfg = HEALTH_CONFIG[health];
        return <Badge status={cfg.status} text={t(`channel.health.${health}`)} />;
      },
    },
    {
      title: t('channel.column.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      /** 渲染启用/禁用开关 */
      render: (enabled: boolean, record: ChannelRecord) => (
        <Switch checked={enabled} size="small" onChange={(val) => handleToggleEnabled(record, val)} />
      ),
    },
    {
      title: t('channel.column.lastCheck'),
      dataIndex: 'lastCheckTime',
      key: 'lastCheckTime',
      width: 180,
      render: (time: string) => time || <Text type="secondary">--</Text>,
    },
    {
      title: t('channel.column.actions'),
      key: 'actions',
      width: 180,
      /** 渲染操作按钮组：编辑、测试、删除 */
      render: (_: unknown, record: ChannelRecord) => (
        <Space>
          <Tooltip title={t('channel.action.edit')}>
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          </Tooltip>
          <Tooltip title={t('channel.action.test')}>
            <Button
              type="link" size="small" icon={<ExperimentOutlined />}
              loading={testingKey === record.key}
              onClick={() => handleTest(record)}
            />
          </Tooltip>
          <Tooltip title={t('channel.action.delete')}>
            <Button type="link" size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record)} />
          </Tooltip>
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与添加按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('channel.title')}</Text>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
          {t('channel.addChannel')}
        </Button>
      </div>

      {/* 过滤条件栏 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Space wrap>
          {/* 渠道类型过滤 */}
          <Select placeholder={t('channel.filter.type')} style={{ width: 160 }} allowClear
            options={ALL_CHANNEL_TYPES.map((v) => ({ value: v, label: t(`channel.type.${v}`) }))} />
          {/* 健康状态过滤 */}
          <Select placeholder={t('channel.filter.health')} style={{ width: 140 }} allowClear
            options={(['healthy', 'degraded', 'unavailable'] as ChannelHealth[]).map((v) => ({
              value: v, label: t(`channel.health.${v}`),
            }))} />
          {/* 启用状态过滤 */}
          <Select placeholder={t('channel.filter.status')} style={{ width: 120 }} allowClear
            options={[
              { value: 'enabled', label: t('channel.filter.enabled') },
              { value: 'disabled', label: t('channel.filter.disabled') },
            ]} />
        </Space>
      </Card>

      {/* 渠道列表表格 */}
      <Table<ChannelRecord>
        columns={columns}
        dataSource={data}
        loading={loading}
        locale={{ emptyText: t('channel.noData') }}
        rowKey="key"
        size="middle"
        pagination={{ pageSize: 20, showTotal: (total) => t('channel.total', { count: total }) }}
      />

      {/* 创建 / 编辑渠道弹窗 */}
      <Modal
        title={editingChannel ? t('channel.editTitle') : t('channel.addTitle')}
        open={modalOpen}
        onCancel={() => { setModalOpen(false); setEditingChannel(null); }}
        onOk={handleSave}
        okText={t('channel.save')}
        cancelText={t('channel.cancel')}
        width={640}
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* 渠道名称 */}
          <Form.Item name="name" label={t('channel.form.name')}
            rules={[{ required: true, message: t('channel.form.nameRequired') }]}>
            <Input placeholder={t('channel.form.namePlaceholder')} />
          </Form.Item>
          {/* 渠道类型选择 */}
          <Form.Item name="type" label={t('channel.form.type')}
            rules={[{ required: true, message: t('channel.form.typeRequired') }]}>
            <Select placeholder={t('channel.form.typePlaceholder')} onChange={handleTypeChange}
              options={ALL_CHANNEL_TYPES.map((v) => ({ value: v, label: t(`channel.type.${v}`) }))} />
          </Form.Item>
          {/* 渠道描述 */}
          <Form.Item name="description" label={t('channel.form.description')}>
            <Input.TextArea rows={2} placeholder={t('channel.form.descriptionPlaceholder')} />
          </Form.Item>
          {/* 根据渠道类型动态渲染的配置字段 */}
          {dynamicFormFields}
          {/* 启用开关 */}
          <Form.Item name="enabled" label={t('channel.form.enabled')} valuePropName="checked" initialValue>
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      {/* 删除确认弹窗 */}
      <Modal
        title={t('channel.deleteTitle')}
        open={deleteModalOpen}
        onCancel={() => { setDeleteModalOpen(false); setDeletingChannel(null); }}
        onOk={handleConfirmDelete}
        okText={t('channel.confirmDelete')}
        cancelText={t('channel.cancel')}
        okButtonProps={{ danger: true }}
      >
        <p>{t('channel.deleteMessage')}</p>
        {deletingChannel && (
          <div>
            <Tag color={CHANNEL_TYPE_COLORS[deletingChannel.type]}>
              {t(`channel.type.${deletingChannel.type}`)}
            </Tag>
            <Text strong>{deletingChannel.name}</Text>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default ChannelConfig;
