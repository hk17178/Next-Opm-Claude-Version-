/**
 * 资产发现页面 (/cmdb/discovery)
 *
 * 功能模块：
 * 1. 发现任务列表（任务名 / 扫描范围 / 状态 / 上次执行 / 发现数 / 操作）
 * 2. 创建发现任务弹窗（IP 范围 / 协议 / 凭据 / 调度策略）
 * 3. 发现结果列表（IP / 主机名 / 类型 / 状态：待确认 / 已导入 / 已忽略）
 * 4. 批量导入按钮
 */
import React, { useState, useCallback } from 'react';
import {
  Typography, Card, Table, Button, Space, Modal, Form,
  Input, Select, Tag, Badge, Tabs, Tooltip, message, Row, Col,
} from 'antd';
import {
  PlusOutlined, PlayCircleOutlined, PauseCircleOutlined,
  ImportOutlined, SearchOutlined, ReloadOutlined,
  CheckCircleOutlined, CloseCircleOutlined, SyncOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

/* ============================== 类型定义 ============================== */

/** 发现任务数据结构 */
interface DiscoveryTask {
  /** 任务唯一 ID */
  id: string;
  /** 任务名称 */
  name: string;
  /** 扫描 IP 范围 */
  ipRange: string;
  /** 扫描协议 */
  protocol: string;
  /** 任务状态 */
  status: 'running' | 'completed' | 'failed' | 'scheduled' | 'paused';
  /** 上次执行时间 */
  lastRun: string;
  /** 发现资产数量 */
  discoveredCount: number;
  /** 调度策略 */
  schedule: string;
}

/** 发现结果数据结构 */
interface DiscoveryResult {
  /** 结果唯一 ID */
  id: string;
  /** IP 地址 */
  ip: string;
  /** 主机名 */
  hostname: string;
  /** 资产类型（自动识别） */
  type: string;
  /** 操作系统 */
  os: string;
  /** 结果状态 */
  status: 'pending' | 'imported' | 'ignored';
  /** 发现时间 */
  discoveredAt: string;
  /** 来源任务名 */
  taskName: string;
}

/* ============================== Mock 数据 ============================== */

/** 模拟发现任务列表 */
const MOCK_TASKS: DiscoveryTask[] = [
  {
    id: 't1', name: '生产网段扫描', ipRange: '10.0.0.0/16',
    protocol: 'SNMP + SSH', status: 'completed', lastRun: '2026-03-24 02:00:00',
    discoveredCount: 45, schedule: '每日 02:00',
  },
  {
    id: 't2', name: '办公网段扫描', ipRange: '192.168.1.0/24',
    protocol: 'ICMP + WMI', status: 'running', lastRun: '2026-03-24 10:30:00',
    discoveredCount: 12, schedule: '每周一 10:00',
  },
  {
    id: 't3', name: 'DMZ 区扫描', ipRange: '172.16.0.0/24',
    protocol: 'SNMP', status: 'scheduled', lastRun: '2026-03-20 03:00:00',
    discoveredCount: 8, schedule: '每周三 03:00',
  },
  {
    id: 't4', name: '容器网段发现', ipRange: '10.244.0.0/16',
    protocol: 'Kubernetes API', status: 'completed', lastRun: '2026-03-23 18:00:00',
    discoveredCount: 128, schedule: '每 6 小时',
  },
  {
    id: 't5', name: '存储网络扫描', ipRange: '10.100.0.0/24',
    protocol: 'SNMP v3', status: 'failed', lastRun: '2026-03-22 04:00:00',
    discoveredCount: 0, schedule: '每日 04:00',
  },
];

/** 模拟发现结果列表 */
const MOCK_RESULTS: DiscoveryResult[] = [
  { id: 'r1', ip: '10.0.5.101', hostname: 'unknown-server-01', type: 'Linux Server', os: 'CentOS 7.9', status: 'pending', discoveredAt: '2026-03-24 02:15:00', taskName: '生产网段扫描' },
  { id: 'r2', ip: '10.0.5.102', hostname: 'app-server-new', type: 'Linux Server', os: 'Ubuntu 22.04', status: 'pending', discoveredAt: '2026-03-24 02:15:00', taskName: '生产网段扫描' },
  { id: 'r3', ip: '10.0.6.50', hostname: 'switch-floor3', type: 'Network Switch', os: 'Cisco IOS 15.2', status: 'imported', discoveredAt: '2026-03-24 02:16:00', taskName: '生产网段扫描' },
  { id: 'r4', ip: '192.168.1.150', hostname: 'dev-pc-zhang', type: 'Workstation', os: 'Windows 11', status: 'ignored', discoveredAt: '2026-03-24 10:35:00', taskName: '办公网段扫描' },
  { id: 'r5', ip: '10.0.7.201', hostname: 'db-replica-03', type: 'Database', os: 'CentOS 8', status: 'pending', discoveredAt: '2026-03-24 02:18:00', taskName: '生产网段扫描' },
  { id: 'r6', ip: '10.244.1.15', hostname: 'pod-frontend-7b8d', type: 'Container', os: 'Alpine 3.18', status: 'imported', discoveredAt: '2026-03-23 18:05:00', taskName: '容器网段发现' },
  { id: 'r7', ip: '10.244.2.22', hostname: 'pod-api-9c3f', type: 'Container', os: 'Debian 12', status: 'imported', discoveredAt: '2026-03-23 18:05:00', taskName: '容器网段发现' },
  { id: 'r8', ip: '10.0.8.10', hostname: 'monitor-agent-new', type: 'Linux Server', os: 'Rocky 9', status: 'pending', discoveredAt: '2026-03-24 02:20:00', taskName: '生产网段扫描' },
];

/* ============================== 状态映射 ============================== */

/** 任务状态配置：图标 + 颜色 + 文本 key */
const TASK_STATUS_CONFIG: Record<string, { icon: React.ReactNode; color: string }> = {
  running: { icon: <SyncOutlined spin />, color: 'processing' },
  completed: { icon: <CheckCircleOutlined />, color: 'success' },
  failed: { icon: <CloseCircleOutlined />, color: 'error' },
  scheduled: { icon: <ClockCircleOutlined />, color: 'default' },
  paused: { icon: <PauseCircleOutlined />, color: 'warning' },
};

/** 发现结果状态对应的标签颜色 */
const RESULT_STATUS_COLORS: Record<string, string> = {
  pending: 'orange',
  imported: 'green',
  ignored: 'default',
};

/* ============================== 主组件 ============================== */

/**
 * 资产发现页面
 * 双 Tab 布局：发现任务列表 + 发现结果列表
 */
const AssetDiscovery: React.FC = () => {
  const { t } = useTranslation('cmdb');
  const [form] = Form.useForm();

  /** 当前 Tab 页 */
  const [activeTab, setActiveTab] = useState('tasks');
  /** 创建任务弹窗可见状态 */
  const [createModalVisible, setCreateModalVisible] = useState(false);
  /** 发现结果中选中的行 */
  const [selectedResultKeys, setSelectedResultKeys] = useState<React.Key[]>([]);

  /** 打开创建任务弹窗 */
  const handleCreateTask = useCallback(() => {
    form.resetFields();
    setCreateModalVisible(true);
  }, [form]);

  /** 提交创建任务表单 */
  const handleCreateOk = useCallback(async () => {
    try {
      await form.validateFields();
      message.success(t('discovery.createSuccess'));
      setCreateModalVisible(false);
    } catch {
      // 表单校验失败
    }
  }, [form, t]);

  /** 批量导入发现结果 */
  const handleBatchImport = useCallback(() => {
    if (selectedResultKeys.length === 0) {
      message.warning(t('discovery.selectFirst'));
      return;
    }
    Modal.confirm({
      title: t('discovery.importConfirm'),
      content: `${t('discovery.importContent', { count: selectedResultKeys.length })}`,
      okText: t('groups.confirm'),
      cancelText: t('groups.cancel'),
      onOk: () => {
        message.success(t('discovery.importSuccess', { count: selectedResultKeys.length }));
        setSelectedResultKeys([]);
      },
    });
  }, [selectedResultKeys, t]);

  /** 发现任务表格列定义 */
  const taskColumns = [
    {
      title: t('discovery.task.name'),
      dataIndex: 'name',
      key: 'name',
      width: 160,
      render: (name: string) => <Text strong>{name}</Text>,
    },
    {
      title: t('discovery.task.ipRange'),
      dataIndex: 'ipRange',
      key: 'ipRange',
      width: 160,
      /** 以代码风格展示 IP 范围 */
      render: (ip: string) => <code style={{ fontSize: 13, background: 'rgba(0,0,0,0.04)', padding: '2px 6px', borderRadius: 4 }}>{ip}</code>,
    },
    {
      title: t('discovery.task.protocol'),
      dataIndex: 'protocol',
      key: 'protocol',
      width: 140,
    },
    {
      title: t('discovery.task.status'),
      dataIndex: 'status',
      key: 'status',
      width: 120,
      /** 渲染任务状态标签 */
      render: (status: string) => {
        const config = TASK_STATUS_CONFIG[status];
        return (
          <Tag icon={config?.icon} color={config?.color}>
            {t(`discovery.taskStatus.${status}`)}
          </Tag>
        );
      },
    },
    {
      title: t('discovery.task.lastRun'),
      dataIndex: 'lastRun',
      key: 'lastRun',
      width: 170,
    },
    {
      title: t('discovery.task.discovered'),
      dataIndex: 'discoveredCount',
      key: 'discoveredCount',
      width: 100,
      /** 发现数量>0 时显示蓝色标签 */
      render: (count: number) => (
        <Tag color={count > 0 ? 'blue' : 'default'}>{count}</Tag>
      ),
    },
    {
      title: t('discovery.task.schedule'),
      dataIndex: 'schedule',
      key: 'schedule',
      width: 130,
    },
    {
      title: t('discovery.task.actions'),
      key: 'actions',
      width: 150,
      /** 操作列：立即执行 / 暂停 */
      render: (_: unknown, record: DiscoveryTask) => (
        <Space>
          <Tooltip title={t('discovery.runNow')}>
            <Button
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              disabled={record.status === 'running'}
              onClick={() => message.info(`${t('discovery.taskStarted')}: ${record.name}`)}
            />
          </Tooltip>
          <Tooltip title={t('discovery.pause')}>
            <Button
              type="link"
              size="small"
              icon={<PauseCircleOutlined />}
              disabled={record.status !== 'running'}
            />
          </Tooltip>
        </Space>
      ),
    },
  ];

  /** 发现结果表格列定义 */
  const resultColumns = [
    {
      title: t('discovery.result.ip'),
      dataIndex: 'ip',
      key: 'ip',
      width: 140,
      /** 以代码风格展示 IP */
      render: (ip: string) => <code style={{ fontSize: 13 }}>{ip}</code>,
    },
    {
      title: t('discovery.result.hostname'),
      dataIndex: 'hostname',
      key: 'hostname',
      width: 180,
    },
    {
      title: t('discovery.result.type'),
      dataIndex: 'type',
      key: 'type',
      width: 130,
    },
    {
      title: t('discovery.result.os'),
      dataIndex: 'os',
      key: 'os',
      width: 140,
    },
    {
      title: t('discovery.result.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      /** 渲染结果状态标签 */
      render: (status: string) => (
        <Tag color={RESULT_STATUS_COLORS[status]}>
          {t(`discovery.resultStatus.${status}`)}
        </Tag>
      ),
    },
    {
      title: t('discovery.result.discoveredAt'),
      dataIndex: 'discoveredAt',
      key: 'discoveredAt',
      width: 170,
    },
    {
      title: t('discovery.result.taskName'),
      dataIndex: 'taskName',
      key: 'taskName',
      width: 140,
    },
  ];

  return (
    <div>
      {/* 页面标题与操作按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('discovery.title')}</Text>
        <Space>
          <Button icon={<ReloadOutlined />}>{t('discovery.refresh')}</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreateTask}>
            {t('discovery.createTask')}
          </Button>
        </Space>
      </div>

      {/* 双 Tab 页面内容 */}
      <Card style={{ borderRadius: 12 }} styles={{ body: { padding: 0 } }}>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          tabBarStyle={{ paddingLeft: 16, marginBottom: 0 }}
          items={[
            {
              key: 'tasks',
              label: t('discovery.tab.tasks'),
              children: (
                /* 发现任务列表 */
                <Table
                  columns={taskColumns}
                  dataSource={MOCK_TASKS}
                  rowKey="id"
                  size="middle"
                  pagination={{ pageSize: 10 }}
                  locale={{ emptyText: t('discovery.noTasks') }}
                />
              ),
            },
            {
              key: 'results',
              label: t('discovery.tab.results'),
              children: (
                <div>
                  {/* 批量导入操作栏 */}
                  <div style={{ padding: '12px 16px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <Space>
                      <Input
                        placeholder={t('discovery.searchPlaceholder')}
                        prefix={<SearchOutlined />}
                        style={{ width: 240 }}
                        allowClear
                      />
                      <Select
                        placeholder={t('discovery.filterStatus')}
                        style={{ width: 140 }}
                        allowClear
                        options={[
                          { value: 'pending', label: t('discovery.resultStatus.pending') },
                          { value: 'imported', label: t('discovery.resultStatus.imported') },
                          { value: 'ignored', label: t('discovery.resultStatus.ignored') },
                        ]}
                      />
                    </Space>
                    <Button
                      type="primary"
                      icon={<ImportOutlined />}
                      onClick={handleBatchImport}
                      disabled={selectedResultKeys.length === 0}
                    >
                      {t('discovery.batchImport')} {selectedResultKeys.length > 0 ? `(${selectedResultKeys.length})` : ''}
                    </Button>
                  </div>
                  {/* 发现结果表格 */}
                  <Table
                    columns={resultColumns}
                    dataSource={MOCK_RESULTS}
                    rowKey="id"
                    size="middle"
                    rowSelection={{
                      selectedRowKeys: selectedResultKeys,
                      onChange: setSelectedResultKeys,
                      /** 只允许选中"待确认"状态的结果 */
                      getCheckboxProps: (record: DiscoveryResult) => ({
                        disabled: record.status !== 'pending',
                      }),
                    }}
                    pagination={{ pageSize: 10 }}
                    locale={{ emptyText: t('discovery.noResults') }}
                  />
                </div>
              ),
            },
          ]}
        />
      </Card>

      {/* 创建发现任务弹窗 */}
      <Modal
        title={t('discovery.createTask')}
        open={createModalVisible}
        onOk={handleCreateOk}
        onCancel={() => setCreateModalVisible(false)}
        okText={t('groups.confirm')}
        cancelText={t('groups.cancel')}
        width={600}
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="name"
            label={t('discovery.form.name')}
            rules={[{ required: true, message: t('discovery.form.nameRequired') }]}
          >
            <Input placeholder={t('discovery.form.namePlaceholder')} />
          </Form.Item>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="ipRange"
                label={t('discovery.form.ipRange')}
                rules={[{ required: true, message: t('discovery.form.ipRequired') }]}
              >
                <Input placeholder="例如: 10.0.0.0/24" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="protocol"
                label={t('discovery.form.protocol')}
                rules={[{ required: true, message: t('discovery.form.protocolRequired') }]}
              >
                <Select
                  placeholder={t('discovery.form.protocolPlaceholder')}
                  options={[
                    { value: 'snmp', label: 'SNMP' },
                    { value: 'ssh', label: 'SSH' },
                    { value: 'wmi', label: 'WMI' },
                    { value: 'icmp', label: 'ICMP' },
                    { value: 'k8s', label: 'Kubernetes API' },
                    { value: 'snmp_ssh', label: 'SNMP + SSH' },
                  ]}
                />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            name="credential"
            label={t('discovery.form.credential')}
          >
            <Select
              placeholder={t('discovery.form.credentialPlaceholder')}
              options={[
                { value: 'cred-ssh-root', label: 'SSH Root 密钥' },
                { value: 'cred-snmp-v3', label: 'SNMP v3 凭据' },
                { value: 'cred-wmi-admin', label: 'WMI Admin' },
                { value: 'cred-k8s-token', label: 'K8s ServiceAccount' },
              ]}
            />
          </Form.Item>

          <Form.Item
            name="schedule"
            label={t('discovery.form.schedule')}
          >
            <Select
              placeholder={t('discovery.form.schedulePlaceholder')}
              options={[
                { value: 'once', label: t('discovery.schedule.once') },
                { value: 'daily', label: t('discovery.schedule.daily') },
                { value: 'weekly', label: t('discovery.schedule.weekly') },
                { value: '6h', label: t('discovery.schedule.every6h') },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default AssetDiscovery;
