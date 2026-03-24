/**
 * 集群管理页面（页面 21）- 双节点 HA 集群状态监控与主备切换
 *
 * 功能模块（严格按设计文档）：
 * - 双节点卡片（节点 IP/角色 Master|Standby/CPU/内存/磁盘使用率）
 * - 同步状态表格（组件名 PG|ES|Kafka|Redis/同步延迟/状态/上次同步时间）
 * - 手动切换按钮（主备切换确认弹窗）
 * - 集群健康状态指示器
 */
import React, { useState, useCallback } from 'react';
import {
  Card, Row, Col, Typography, Table, Tag, Button, Space, Progress, Badge,
  Modal, Alert, Statistic, Divider, message, Tooltip,
} from 'antd';
import {
  SwapOutlined, CloudServerOutlined, SyncOutlined, CheckCircleOutlined,
  WarningOutlined, CloseCircleOutlined, ExclamationCircleOutlined,
  HddOutlined, DashboardOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;

// ==================== 类型定义 ====================

/** 节点角色类型 */
type NodeRole = 'Master' | 'Standby';

/** 节点健康状态 */
type NodeStatus = 'healthy' | 'warning' | 'error';

/** 节点信息 */
interface NodeInfo {
  id: string;            // 节点唯一标识
  ip: string;            // 节点 IP 地址
  hostname: string;      // 主机名
  role: NodeRole;        // 当前角色（主节点/备用节点）
  status: NodeStatus;    // 健康状态
  cpuUsage: number;      // CPU 使用率（百分比）
  memoryUsage: number;   // 内存使用率（百分比）
  diskUsage: number;     // 磁盘使用率（百分比）
  uptime: string;        // 运行时长
  lastHeartbeat: string; // 最后心跳时间
}

/** 同步组件状态 */
interface SyncComponent {
  key: string;           // 唯一标识
  name: string;          // 组件名称
  type: string;          // 组件类型标识
  syncDelay: string;     // 同步延迟
  status: 'synced' | 'syncing' | 'lag' | 'error'; // 同步状态
  lastSyncTime: string;  // 上次同步时间
  detail: string;        // 详细信息
}

// ==================== Mock 数据 ====================

/** Mock 双节点数据 */
const mockNodes: NodeInfo[] = [
  {
    id: 'node-1',
    ip: '192.168.1.10',
    hostname: 'opsnexus-master',
    role: 'Master',
    status: 'healthy',
    cpuUsage: 42,
    memoryUsage: 68,
    diskUsage: 55,
    uptime: '45 天 12 小时',
    lastHeartbeat: '2026-03-24 10:00:01',
  },
  {
    id: 'node-2',
    ip: '192.168.1.11',
    hostname: 'opsnexus-standby',
    role: 'Standby',
    status: 'healthy',
    cpuUsage: 18,
    memoryUsage: 35,
    diskUsage: 52,
    uptime: '45 天 12 小时',
    lastHeartbeat: '2026-03-24 10:00:02',
  },
];

/** Mock 同步组件状态数据 */
const mockSyncComponents: SyncComponent[] = [
  { key: 'pg', name: 'PostgreSQL', type: 'PG', syncDelay: '0.5ms', status: 'synced', lastSyncTime: '2026-03-24 10:00:00', detail: 'WAL 流复制正常' },
  { key: 'es', name: 'Elasticsearch', type: 'ES', syncDelay: '2.1s', status: 'syncing', lastSyncTime: '2026-03-24 09:59:58', detail: '索引同步中，3个分片待复制' },
  { key: 'kafka', name: 'Kafka', type: 'Kafka', syncDelay: '0ms', status: 'synced', lastSyncTime: '2026-03-24 10:00:01', detail: '所有 Topic 分区同步完成' },
  { key: 'redis', name: 'Redis', type: 'Redis', syncDelay: '0ms', status: 'synced', lastSyncTime: '2026-03-24 10:00:00', detail: '主从复制正常，RDB 最新' },
];

// ==================== 辅助函数 ====================

/**
 * 根据使用率返回进度条颜色
 * @param usage 使用率百分比
 * @returns 颜色字符串
 */
const getUsageColor = (usage: number): string => {
  if (usage >= 90) return '#ff6b6b';   // 危险红
  if (usage >= 70) return '#ffaa33';   // 警告橙
  return '#00e5a0';                    // 成功绿
};

/**
 * 根据节点状态返回 Badge 状态
 */
const getStatusBadge = (status: NodeStatus): 'success' | 'warning' | 'error' => {
  const map: Record<NodeStatus, 'success' | 'warning' | 'error'> = {
    healthy: 'success',
    warning: 'warning',
    error: 'error',
  };
  return map[status];
};

/**
 * 根据同步状态返回标签颜色和图标
 */
const getSyncStatusConfig = (status: SyncComponent['status']) => {
  const configs = {
    synced: { color: 'success', icon: <CheckCircleOutlined />, text: '已同步' },
    syncing: { color: 'processing', icon: <SyncOutlined spin />, text: '同步中' },
    lag: { color: 'warning', icon: <WarningOutlined />, text: '延迟' },
    error: { color: 'error', icon: <CloseCircleOutlined />, text: '异常' },
  };
  return configs[status];
};

// ==================== 组件实现 ====================

/**
 * 集群管理组件
 * 展示双节点状态卡片、同步状态表格、手动切换按钮
 */
const ClusterManagement: React.FC = () => {
  const { t } = useTranslation('settings');
  const [switchModalOpen, setSwitchModalOpen] = useState(false);  // 主备切换确认弹窗
  const [switching, setSwitching] = useState(false);               // 切换中状态

  /**
   * 执行主备切换操作
   * 弹出确认弹窗后执行切换
   */
  const handleFailover = useCallback(() => {
    setSwitching(true);
    // 模拟切换过程
    setTimeout(() => {
      setSwitching(false);
      setSwitchModalOpen(false);
      message.success(t('cluster.switchSuccess'));
      // TODO: 对接主备切换 API
    }, 2000);
  }, [t]);

  /** 同步状态表格列定义 */
  const syncColumns = [
    {
      title: t('cluster.sync.column.name'),
      dataIndex: 'name',
      key: 'name',
      width: 160,
      /** 渲染组件名称及类型标签 */
      render: (name: string, record: SyncComponent) => (
        <Space>
          <Tag>{record.type}</Tag>
          <span>{name}</span>
        </Space>
      ),
    },
    {
      title: t('cluster.sync.column.delay'),
      dataIndex: 'syncDelay',
      key: 'syncDelay',
      width: 120,
      /** 渲染同步延迟，高延迟标红 */
      render: (delay: string) => {
        const numDelay = parseFloat(delay);
        const isHigh = numDelay > 1000; // 超过 1s 视为高延迟
        return <Text type={isHigh ? 'danger' : undefined}>{delay}</Text>;
      },
    },
    {
      title: t('cluster.sync.column.status'),
      dataIndex: 'status',
      key: 'status',
      width: 120,
      /** 渲染同步状态标签 */
      render: (status: SyncComponent['status']) => {
        const config = getSyncStatusConfig(status);
        return (
          <Tag icon={config.icon} color={config.color as string}>
            {t(`cluster.sync.status.${status}`)}
          </Tag>
        );
      },
    },
    {
      title: t('cluster.sync.column.lastSync'),
      dataIndex: 'lastSyncTime',
      key: 'lastSyncTime',
      width: 180,
    },
    {
      title: t('cluster.sync.column.detail'),
      dataIndex: 'detail',
      key: 'detail',
      ellipsis: true,
      /** 渲染详细信息，过长时以 Tooltip 展示 */
      render: (detail: string) => (
        <Tooltip title={detail}>
          <Text type="secondary">{detail}</Text>
        </Tooltip>
      ),
    },
  ];

  /**
   * 渲染单个节点状态卡片
   * @param node 节点信息
   */
  const renderNodeCard = (node: NodeInfo) => (
    <Col xs={24} md={12} key={node.id}>
      <Card
        style={{ borderRadius: 8 }}
        title={
          <Space>
            <CloudServerOutlined />
            <span>{node.hostname}</span>
            {/* 节点角色标签 */}
            <Tag color={node.role === 'Master' ? '#2E75B6' : '#86909C'}>
              {t(`cluster.role.${node.role.toLowerCase()}`)}
            </Tag>
            {/* 节点健康状态 */}
            <Badge status={getStatusBadge(node.status)} text={t(`cluster.status.${node.status}`)} />
          </Space>
        }
      >
        {/* 节点基本信息 */}
        <Row gutter={[16, 12]}>
          <Col span={12}>
            <Text type="secondary">{t('cluster.node.ip')}</Text>
            <div><Text strong copyable>{node.ip}</Text></div>
          </Col>
          <Col span={12}>
            <Text type="secondary">{t('cluster.node.uptime')}</Text>
            <div><Text strong>{node.uptime}</Text></div>
          </Col>
        </Row>

        <Divider style={{ margin: '12px 0' }} />

        {/* 资源使用率指标 */}
        <Row gutter={16}>
          <Col span={8}>
            <div style={{ textAlign: 'center' }}>
              <DashboardOutlined style={{ marginBottom: 4 }} />
              <div><Text type="secondary" style={{ fontSize: 12 }}>CPU</Text></div>
              <Progress
                type="circle"
                percent={node.cpuUsage}
                size={60}
                strokeColor={getUsageColor(node.cpuUsage)}
                format={(val) => `${val}%`}
              />
            </div>
          </Col>
          <Col span={8}>
            <div style={{ textAlign: 'center' }}>
              <HddOutlined style={{ marginBottom: 4 }} />
              <div><Text type="secondary" style={{ fontSize: 12 }}>{t('cluster.node.memory')}</Text></div>
              <Progress
                type="circle"
                percent={node.memoryUsage}
                size={60}
                strokeColor={getUsageColor(node.memoryUsage)}
                format={(val) => `${val}%`}
              />
            </div>
          </Col>
          <Col span={8}>
            <div style={{ textAlign: 'center' }}>
              <HddOutlined style={{ marginBottom: 4 }} />
              <div><Text type="secondary" style={{ fontSize: 12 }}>{t('cluster.node.disk')}</Text></div>
              <Progress
                type="circle"
                percent={node.diskUsage}
                size={60}
                strokeColor={getUsageColor(node.diskUsage)}
                format={(val) => `${val}%`}
              />
            </div>
          </Col>
        </Row>

        {/* 最后心跳时间 */}
        <div style={{ marginTop: 12, textAlign: 'right' }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {t('cluster.node.lastHeartbeat')}: {node.lastHeartbeat}
          </Text>
        </div>
      </Card>
    </Col>
  );

  return (
    <div>
      {/* 页面标题与手动切换按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('cluster.title')}</Text>
        <Space>
          {/* 集群健康状态指示器 */}
          <Badge status="success" text={t('cluster.healthyIndicator')} />
          {/* 主备切换按钮 */}
          <Button
            type="primary"
            danger
            icon={<SwapOutlined />}
            onClick={() => setSwitchModalOpen(true)}
          >
            {t('cluster.manualSwitch')}
          </Button>
        </Space>
      </div>

      {/* 双节点状态卡片 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {mockNodes.map(renderNodeCard)}
      </Row>

      {/* 同步状态表格 */}
      <Card title={t('cluster.sync.title')} style={{ borderRadius: 8 }}>
        <Table<SyncComponent>
          columns={syncColumns}
          dataSource={mockSyncComponents}
          rowKey="key"
          size="middle"
          pagination={false}
        />
      </Card>

      {/* 主备切换确认弹窗 */}
      <Modal
        title={
          <Space>
            <ExclamationCircleOutlined style={{ color: '#faad14' }} />
            {t('cluster.switchModal.title')}
          </Space>
        }
        open={switchModalOpen}
        onCancel={() => setSwitchModalOpen(false)}
        onOk={handleFailover}
        confirmLoading={switching}
        okText={t('cluster.switchModal.confirm')}
        cancelText={t('cluster.switchModal.cancel')}
        okButtonProps={{ danger: true }}
        width={520}
      >
        {/* 切换警告提示 */}
        <Alert
          type="warning"
          showIcon
          message={t('cluster.switchModal.warning')}
          description={t('cluster.switchModal.warningDetail')}
          style={{ marginBottom: 16 }}
        />
        {/* 切换前后对比 */}
        <div style={{ padding: '0 16px' }}>
          <Row gutter={16}>
            <Col span={11} style={{ textAlign: 'center' }}>
              <Text type="secondary">{t('cluster.switchModal.currentMaster')}</Text>
              <div><Text strong>{mockNodes[0].ip}</Text></div>
              <Tag color="#2E75B6">Master</Tag>
            </Col>
            <Col span={2} style={{ textAlign: 'center', paddingTop: 20 }}>
              <SwapOutlined style={{ fontSize: 20 }} />
            </Col>
            <Col span={11} style={{ textAlign: 'center' }}>
              <Text type="secondary">{t('cluster.switchModal.newMaster')}</Text>
              <div><Text strong>{mockNodes[1].ip}</Text></div>
              <Tag color="#86909C">Standby</Tag>
            </Col>
          </Row>
        </div>
      </Modal>
    </div>
  );
};

export default ClusterManagement;
