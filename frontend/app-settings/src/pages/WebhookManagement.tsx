/**
 * Webhook 管理页面
 * 管理系统的出站 Webhook 集成，支持订阅事件推送到外部系统
 *
 * 功能模块：
 * - Webhook 列表：展示所有已配置的 Webhook（URL / 订阅事件 / 状态 / 失败次数 / 操作）
 * - 创建/编辑 Webhook 弹窗：URL / HMAC 密钥 / 订阅事件勾选 / 测试按钮
 * - Webhook 投递日志表格：时间 / 事件类型 / HTTP 状态码 / 响应时间
 */
import React, { useState, useCallback } from 'react';
import {
  Card, Table, Button, Typography, Tag, Space, Modal, Form, Input, Switch,
  Checkbox, message, Popconfirm, Badge, Tabs, Tooltip, Row, Col,
} from 'antd';
import {
  PlusOutlined, DeleteOutlined, EditOutlined, SendOutlined,
  ApiOutlined, ExclamationCircleOutlined, ReloadOutlined,
  CheckCircleOutlined, CloseCircleOutlined, ClockCircleOutlined,
  EyeOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text, Paragraph } = Typography;

/* ========== 类型定义 ========== */

/** Webhook 状态类型 */
type WebhookStatus = 'active' | 'inactive' | 'error';

/** Webhook 记录数据结构 */
interface WebhookRecord {
  id: string;                    // Webhook 唯一标识
  url: string;                   // 目标 URL
  secret: string;                // HMAC 签名密钥
  events: string[];              // 订阅事件列表
  status: WebhookStatus;         // 当前状态
  failCount: number;             // 连续失败次数
  lastTriggered: string;         // 最后触发时间
  createdAt: string;             // 创建时间
  description: string;           // 描述说明
}

/** Webhook 投递日志数据结构 */
interface WebhookLog {
  id: string;                    // 日志唯一标识
  webhookId: string;             // 关联 Webhook ID
  time: string;                  // 投递时间
  event: string;                 // 事件类型
  statusCode: number;            // HTTP 响应状态码
  responseTime: number;          // 响应耗时（毫秒）
  success: boolean;              // 是否成功
  requestBody: string;           // 请求体摘要
  responseBody: string;          // 响应体摘要
}

/* ========== 常量配置 ========== */

/** 可订阅的事件类型列表 */
const EVENT_OPTIONS = [
  { value: 'alert.created', label: '告警创建' },
  { value: 'alert.resolved', label: '告警恢复' },
  { value: 'alert.escalated', label: '告警升级' },
  { value: 'incident.created', label: '事件创建' },
  { value: 'incident.updated', label: '事件更新' },
  { value: 'incident.resolved', label: '事件解决' },
  { value: 'change.submitted', label: '变更提交' },
  { value: 'change.approved', label: '变更审批' },
  { value: 'change.completed', label: '变更完成' },
  { value: 'deploy.started', label: '部署开始' },
  { value: 'deploy.finished', label: '部署完成' },
  { value: 'deploy.failed', label: '部署失败' },
];

/** Webhook 状态对应的 Badge 配置 */
const STATUS_CONFIG: Record<WebhookStatus, { color: string; text: string; badgeStatus: 'success' | 'default' | 'error' }> = {
  active: { color: '#52C41A', text: '正常', badgeStatus: 'success' },
  inactive: { color: '#D9D9D9', text: '已禁用', badgeStatus: 'default' },
  error: { color: '#F5222D', text: '异常', badgeStatus: 'error' },
};

/* ========== 模拟数据 ========== */

/** 生成模拟 Webhook 列表 */
const mockWebhooks: WebhookRecord[] = [
  {
    id: '1',
    url: 'https://hooks.slack.com/services/T00000000/B00000000/XXXX',
    secret: 'whsec_abc123def456',
    events: ['alert.created', 'alert.resolved', 'incident.created'],
    status: 'active',
    failCount: 0,
    lastTriggered: '2026-03-24 10:30:00',
    createdAt: '2026-01-15 09:00:00',
    description: 'Slack 告警通知通道',
  },
  {
    id: '2',
    url: 'https://api.pagerduty.com/webhooks/v3',
    secret: 'whsec_xyz789ghi012',
    events: ['incident.created', 'incident.updated', 'incident.resolved'],
    status: 'active',
    failCount: 2,
    lastTriggered: '2026-03-24 09:15:00',
    createdAt: '2026-02-01 14:00:00',
    description: 'PagerDuty 事件同步',
  },
  {
    id: '3',
    url: 'https://internal.company.com/ci/webhook',
    secret: 'whsec_jkl345mno678',
    events: ['deploy.started', 'deploy.finished', 'deploy.failed'],
    status: 'error',
    failCount: 15,
    lastTriggered: '2026-03-23 22:00:00',
    createdAt: '2026-03-01 10:00:00',
    description: 'CI/CD 部署通知',
  },
  {
    id: '4',
    url: 'https://audit.company.com/events',
    secret: 'whsec_pqr901stu234',
    events: ['change.submitted', 'change.approved', 'change.completed'],
    status: 'inactive',
    failCount: 0,
    lastTriggered: '2026-03-20 16:00:00',
    createdAt: '2026-03-10 08:00:00',
    description: '审计系统变更同步（已暂停）',
  },
];

/** 生成模拟 Webhook 投递日志 */
const mockLogs: WebhookLog[] = [
  {
    id: 'log-1', webhookId: '1', time: '2026-03-24 10:30:00',
    event: 'alert.created', statusCode: 200, responseTime: 156,
    success: true, requestBody: '{"type":"alert.created","alert_id":"ALT-001"}',
    responseBody: '{"ok": true}',
  },
  {
    id: 'log-2', webhookId: '1', time: '2026-03-24 10:15:00',
    event: 'alert.resolved', statusCode: 200, responseTime: 203,
    success: true, requestBody: '{"type":"alert.resolved","alert_id":"ALT-002"}',
    responseBody: '{"ok": true}',
  },
  {
    id: 'log-3', webhookId: '2', time: '2026-03-24 09:15:00',
    event: 'incident.created', statusCode: 504, responseTime: 30000,
    success: false, requestBody: '{"type":"incident.created","incident_id":"INC-010"}',
    responseBody: 'Gateway Timeout',
  },
  {
    id: 'log-4', webhookId: '3', time: '2026-03-23 22:00:00',
    event: 'deploy.finished', statusCode: 502, responseTime: 5000,
    success: false, requestBody: '{"type":"deploy.finished","deploy_id":"DEP-045"}',
    responseBody: 'Bad Gateway',
  },
  {
    id: 'log-5', webhookId: '1', time: '2026-03-24 08:00:00',
    event: 'incident.created', statusCode: 200, responseTime: 178,
    success: true, requestBody: '{"type":"incident.created","incident_id":"INC-009"}',
    responseBody: '{"ok": true}',
  },
  {
    id: 'log-6', webhookId: '2', time: '2026-03-24 07:30:00',
    event: 'incident.updated', statusCode: 200, responseTime: 245,
    success: true, requestBody: '{"type":"incident.updated","incident_id":"INC-008"}',
    responseBody: '{"ok": true}',
  },
];

/**
 * Webhook 管理组件
 * - 顶部：页面标题 + 创建 Webhook 按钮
 * - Tabs：Webhook 列表 + 投递日志
 * - 创建/编辑 Webhook 弹窗
 * - 日志详情弹窗
 */
const WebhookManagement: React.FC = () => {
  const { t } = useTranslation('settings');

  /** Webhook 列表数据 */
  const [webhooks, setWebhooks] = useState<WebhookRecord[]>(mockWebhooks);
  /** 投递日志数据 */
  const [logs] = useState<WebhookLog[]>(mockLogs);
  /** 创建/编辑弹窗是否打开 */
  const [modalOpen, setModalOpen] = useState(false);
  /** 当前编辑的 Webhook（null 表示创建模式） */
  const [editingWebhook, setEditingWebhook] = useState<WebhookRecord | null>(null);
  /** 表单实例 */
  const [form] = Form.useForm();
  /** 提交中状态 */
  const [submitting, setSubmitting] = useState(false);
  /** 测试中状态 */
  const [testing, setTesting] = useState(false);
  /** 日志详情弹窗 */
  const [logDetailOpen, setLogDetailOpen] = useState(false);
  /** 当前查看的日志记录 */
  const [selectedLog, setSelectedLog] = useState<WebhookLog | null>(null);
  /** 当前活动 Tab */
  const [activeTab, setActiveTab] = useState('list');

  /**
   * 打开创建 Webhook 弹窗
   * 重置表单到初始状态
   */
  const handleCreate = useCallback(() => {
    setEditingWebhook(null);
    form.resetFields();
    form.setFieldsValue({ events: [], status: true });
    setModalOpen(true);
  }, [form]);

  /**
   * 打开编辑 Webhook 弹窗
   * 将已有数据填充到表单
   * @param record - 要编辑的 Webhook 记录
   */
  const handleEdit = useCallback((record: WebhookRecord) => {
    setEditingWebhook(record);
    form.setFieldsValue({
      url: record.url,
      secret: record.secret,
      description: record.description,
      events: record.events,
      status: record.status === 'active',
    });
    setModalOpen(true);
  }, [form]);

  /**
   * 删除 Webhook
   * @param id - 要删除的 Webhook ID
   */
  const handleDelete = useCallback((id: string) => {
    setWebhooks((prev) => prev.filter((w) => w.id !== id));
    message.success('Webhook 已删除');
  }, []);

  /**
   * 提交创建/编辑表单
   * 校验表单后模拟保存到后端
   */
  const handleSubmit = useCallback(async () => {
    try {
      const values = await form.validateFields();
      setSubmitting(true);
      // TODO: 对接后端 Webhook 管理 API
      await new Promise((resolve) => setTimeout(resolve, 500));

      if (editingWebhook) {
        /** 编辑模式：更新列表中对应记录 */
        setWebhooks((prev) =>
          prev.map((w) =>
            w.id === editingWebhook.id
              ? {
                  ...w,
                  url: values.url,
                  secret: values.secret,
                  description: values.description || '',
                  events: values.events,
                  status: values.status ? 'active' : 'inactive',
                }
              : w,
          ),
        );
        message.success('Webhook 更新成功');
      } else {
        /** 创建模式：在列表头部插入新记录 */
        const newWebhook: WebhookRecord = {
          id: String(Date.now()),
          url: values.url,
          secret: values.secret,
          description: values.description || '',
          events: values.events,
          status: values.status ? 'active' : 'inactive',
          failCount: 0,
          lastTriggered: '--',
          createdAt: new Date().toLocaleString('zh-CN'),
        };
        setWebhooks((prev) => [newWebhook, ...prev]);
        message.success('Webhook 创建成功');
      }
      setModalOpen(false);
    } catch {
      // 表单校验失败，antd 自动处理
    } finally {
      setSubmitting(false);
    }
  }, [form, editingWebhook]);

  /**
   * 测试 Webhook 连通性
   * 向目标 URL 发送测试事件，验证可达性
   */
  const handleTest = useCallback(async () => {
    const url = form.getFieldValue('url');
    if (!url) {
      message.warning('请先输入 Webhook URL');
      return;
    }
    setTesting(true);
    // TODO: 对接后端 Webhook 测试 API
    await new Promise((resolve) => setTimeout(resolve, 1000));
    message.success('测试事件已发送，请检查目标系统是否收到');
    setTesting(false);
  }, [form]);

  /**
   * 查看日志详情
   * @param record - 日志记录
   */
  const handleViewLog = useCallback((record: WebhookLog) => {
    setSelectedLog(record);
    setLogDetailOpen(true);
  }, []);

  /** Webhook 列表表格列定义 */
  const webhookColumns = [
    {
      title: 'Webhook URL',
      dataIndex: 'url',
      key: 'url',
      ellipsis: true,
      /** 渲染 URL，使用等宽字体并限制宽度 */
      render: (url: string, record: WebhookRecord) => (
        <div>
          <Text code style={{ fontSize: 12 }}>{url}</Text>
          {record.description && (
            <div style={{ marginTop: 2 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>{record.description}</Text>
            </div>
          )}
        </div>
      ),
    },
    {
      title: '订阅事件',
      dataIndex: 'events',
      key: 'events',
      width: 260,
      /** 渲染订阅事件标签列表 */
      render: (events: string[]) => (
        <Space wrap size={[4, 4]}>
          {events.slice(0, 3).map((event) => (
            <Tag key={event} color="blue" style={{ fontSize: 11 }}>
              {EVENT_OPTIONS.find((o) => o.value === event)?.label || event}
            </Tag>
          ))}
          {events.length > 3 && (
            <Tooltip title={events.slice(3).map((e) => EVENT_OPTIONS.find((o) => o.value === e)?.label || e).join('、')}>
              <Tag color="blue" style={{ fontSize: 11 }}>+{events.length - 3}</Tag>
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 90,
      /** 渲染 Webhook 状态 Badge */
      render: (status: WebhookStatus) => {
        const cfg = STATUS_CONFIG[status];
        return <Badge status={cfg.badgeStatus} text={cfg.text} />;
      },
    },
    {
      title: '失败次数',
      dataIndex: 'failCount',
      key: 'failCount',
      width: 90,
      /** 失败次数 > 0 时使用红色高亮 */
      render: (count: number) => (
        <Text type={count > 0 ? 'danger' : 'secondary'} strong={count > 0}>
          {count}
        </Text>
      ),
    },
    {
      title: '最后触发',
      dataIndex: 'lastTriggered',
      key: 'lastTriggered',
      width: 170,
    },
    {
      title: '操作',
      key: 'actions',
      width: 140,
      /** 渲染操作按钮：编辑、删除 */
      render: (_: unknown, record: WebhookRecord) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除此 Webhook 吗？"
            description="删除后将停止向该 URL 推送事件通知。"
            icon={<ExclamationCircleOutlined style={{ color: '#F53F3F' }} />}
            onConfirm={() => handleDelete(record.id)}
            okText="确定删除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
          >
            <Button type="link" danger size="small" icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  /** 投递日志表格列定义 */
  const logColumns = [
    {
      title: '时间',
      dataIndex: 'time',
      key: 'time',
      width: 180,
    },
    {
      title: '事件类型',
      dataIndex: 'event',
      key: 'event',
      width: 140,
      /** 渲染事件类型标签 */
      render: (event: string) => (
        <Tag color="blue">
          {EVENT_OPTIONS.find((o) => o.value === event)?.label || event}
        </Tag>
      ),
    },
    {
      title: 'HTTP 状态码',
      dataIndex: 'statusCode',
      key: 'statusCode',
      width: 120,
      /** 根据状态码着色：2xx 绿色，4xx 橙色，5xx 红色 */
      render: (code: number) => {
        let color = '#52C41A';
        if (code >= 400 && code < 500) color = '#FAAD14';
        if (code >= 500) color = '#F5222D';
        return <Tag color={color}>{code}</Tag>;
      },
    },
    {
      title: '响应时间',
      dataIndex: 'responseTime',
      key: 'responseTime',
      width: 120,
      /** 渲染响应时间，超过 5 秒标红 */
      render: (ms: number) => (
        <Text type={ms > 5000 ? 'danger' : undefined}>{ms} ms</Text>
      ),
    },
    {
      title: '结果',
      dataIndex: 'success',
      key: 'success',
      width: 80,
      /** 渲染成功/失败图标 */
      render: (success: boolean) =>
        success ? (
          <CheckCircleOutlined style={{ color: '#52C41A', fontSize: 16 }} />
        ) : (
          <CloseCircleOutlined style={{ color: '#F5222D', fontSize: 16 }} />
        ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 80,
      /** 查看日志详情按钮 */
      render: (_: unknown, record: WebhookLog) => (
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => handleViewLog(record)}>
          详情
        </Button>
      ),
    },
  ];

  /** Tab 配置项 */
  const tabItems = [
    {
      key: 'list',
      label: (
        <Space>
          <ApiOutlined />
          <span>Webhook 列表</span>
        </Space>
      ),
      children: (
        <Table
          columns={webhookColumns}
          dataSource={webhooks}
          rowKey="id"
          size="middle"
          locale={{ emptyText: '暂无 Webhook 配置' }}
          pagination={{ pageSize: 10 }}
        />
      ),
    },
    {
      key: 'logs',
      label: (
        <Space>
          <ClockCircleOutlined />
          <span>投递日志</span>
        </Space>
      ),
      children: (
        <Table
          columns={logColumns}
          dataSource={logs}
          rowKey="id"
          size="middle"
          locale={{ emptyText: '暂无投递日志' }}
          pagination={{
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条`,
          }}
        />
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与创建按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>
          <ApiOutlined style={{ marginRight: 8 }} />
          Webhook 管理
        </Text>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          创建 Webhook
        </Button>
      </div>

      {/* Webhook 列表与日志 Tabs */}
      <Card style={{ borderRadius: 8 }}>
        <Tabs items={tabItems} activeKey={activeTab} onChange={setActiveTab} />
      </Card>

      {/* 创建/编辑 Webhook 弹窗 */}
      <Modal
        title={editingWebhook ? '编辑 Webhook' : '创建 Webhook'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSubmit}
        confirmLoading={submitting}
        okText={editingWebhook ? '保存' : '创建'}
        cancelText="取消"
        width={640}
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* Webhook 目标 URL */}
          <Form.Item
            name="url"
            label="Webhook URL"
            rules={[
              { required: true, message: '请输入 Webhook URL' },
              { type: 'url', message: '请输入有效的 URL 地址' },
            ]}
          >
            <Input placeholder="https://example.com/webhook" />
          </Form.Item>

          {/* 描述说明 */}
          <Form.Item name="description" label="描述说明">
            <Input placeholder="如：Slack 告警通知、PagerDuty 事件同步" />
          </Form.Item>

          {/* HMAC 签名密钥 */}
          <Form.Item
            name="secret"
            label="HMAC 签名密钥"
            rules={[{ required: true, message: '请输入签名密钥' }]}
            tooltip="用于对 Webhook 请求体进行 HMAC-SHA256 签名，接收方可用此密钥验证请求合法性"
          >
            <Input.Password placeholder="whsec_xxxxxxxxxxxxxxxx" />
          </Form.Item>

          {/* 订阅事件勾选 */}
          <Form.Item
            name="events"
            label="订阅事件"
            rules={[{ required: true, message: '请至少选择一个订阅事件' }]}
          >
            <Checkbox.Group style={{ width: '100%' }}>
              <Row gutter={[0, 8]}>
                {EVENT_OPTIONS.map((opt) => (
                  <Col span={8} key={opt.value}>
                    <Checkbox value={opt.value}>{opt.label}</Checkbox>
                  </Col>
                ))}
              </Row>
            </Checkbox.Group>
          </Form.Item>

          {/* 启用状态开关 */}
          <Form.Item name="status" label="启用状态" valuePropName="checked">
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>

          {/* 测试按钮 */}
          <Form.Item>
            <Button
              icon={<SendOutlined />}
              loading={testing}
              onClick={handleTest}
            >
              发送测试事件
            </Button>
            <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>
              向目标 URL 发送一条测试事件以验证连通性
            </Text>
          </Form.Item>
        </Form>
      </Modal>

      {/* 日志详情弹窗 */}
      <Modal
        title="Webhook 投递详情"
        open={logDetailOpen}
        onCancel={() => { setLogDetailOpen(false); setSelectedLog(null); }}
        footer={null}
        width={600}
      >
        {selectedLog && (
          <div>
            {/* 投递时间 */}
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary">投递时间：</Text>
              <Text>{selectedLog.time}</Text>
            </div>
            {/* 事件类型 */}
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary">事件类型：</Text>
              <Tag color="blue">
                {EVENT_OPTIONS.find((o) => o.value === selectedLog.event)?.label || selectedLog.event}
              </Tag>
            </div>
            {/* HTTP 状态码和结果 */}
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary">HTTP 状态码：</Text>
              <Tag color={selectedLog.success ? '#52C41A' : '#F5222D'}>
                {selectedLog.statusCode}
              </Tag>
              <Text type="secondary" style={{ marginLeft: 16 }}>响应时间：</Text>
              <Text>{selectedLog.responseTime} ms</Text>
            </div>
            {/* 请求体 */}
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary">请求体：</Text>
              <Paragraph
                code
                style={{
                  marginTop: 4,
                  padding: 12,
                  background: '#F7F8FA',
                  borderRadius: 6,
                  fontSize: 12,
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {selectedLog.requestBody}
              </Paragraph>
            </div>
            {/* 响应体 */}
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary">响应体：</Text>
              <Paragraph
                code
                style={{
                  marginTop: 4,
                  padding: 12,
                  background: selectedLog.success ? '#F6FFED' : '#FFF2F0',
                  borderRadius: 6,
                  fontSize: 12,
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {selectedLog.responseBody}
              </Paragraph>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default WebhookManagement;
