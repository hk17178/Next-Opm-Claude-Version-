/**
 * 资产拓扑页面 - 以树形结构展示资产的层级关系（环境 → 业务组 → 资产节点）
 * 点击叶子节点可查看资产详细信息
 */
import React, { useState } from 'react';
import { Card, Typography, Tree, Tag, Space, Descriptions, Badge, Empty } from 'antd';
import {
  ClusterOutlined, DesktopOutlined, CloudServerOutlined, DatabaseOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { DataNode } from 'antd/es/tree';

const { Text } = Typography;

/** 资产分级对应的颜色映射 */
const GRADE_COLORS: Record<string, string> = {
  S: '#F53F3F', A: '#FF7D00', B: '#3491FA', C: '#86909C', D: '#C9CDD4',
};

/** 资产节点数据结构 */
interface AssetNode {
  hostname: string;  // 主机名
  ip: string;        // IP 地址
  type: string;      // 资产类型（ECS/RDS 等）
  grade: string;     // 资产分级（S/A/B/C/D）
  status: string;    // 状态（online/offline/maintenance）
}

/**
 * 模拟拓扑树数据
 * 层级结构：环境（生产/预发）→ 业务组 → 资产节点
 * 使用不同图标区分层级：CloudServer（环境）、Cluster（业务组）、Desktop/Database（资产）
 */
const MOCK_TREE: DataNode[] = [
  {
    title: '生产环境',
    key: 'prod',
    icon: <CloudServerOutlined />,
    children: [
      {
        title: '支付业务组',
        key: 'prod-payment',
        icon: <ClusterOutlined />,
        children: [
          { title: 'pay-gateway-01 (10.0.1.10)', key: 'pay-gw-01', icon: <DesktopOutlined />, isLeaf: true },
          { title: 'pay-gateway-02 (10.0.1.11)', key: 'pay-gw-02', icon: <DesktopOutlined />, isLeaf: true },
          { title: 'pay-db-master (10.0.1.20)', key: 'pay-db-m', icon: <DatabaseOutlined />, isLeaf: true },
          { title: 'pay-db-slave (10.0.1.21)', key: 'pay-db-s', icon: <DatabaseOutlined />, isLeaf: true },
        ],
      },
      {
        title: '订单业务组',
        key: 'prod-order',
        icon: <ClusterOutlined />,
        children: [
          { title: 'order-api-01 (10.0.2.10)', key: 'order-api-01', icon: <DesktopOutlined />, isLeaf: true },
          { title: 'order-api-02 (10.0.2.11)', key: 'order-api-02', icon: <DesktopOutlined />, isLeaf: true },
          { title: 'order-mysql (10.0.2.20)', key: 'order-db', icon: <DatabaseOutlined />, isLeaf: true },
        ],
      },
    ],
  },
  {
    title: '预发环境',
    key: 'staging',
    icon: <CloudServerOutlined />,
    children: [
      {
        title: '全业务组',
        key: 'staging-all',
        icon: <ClusterOutlined />,
        children: [
          { title: 'staging-app-01 (10.1.0.10)', key: 'stg-app-01', icon: <DesktopOutlined />, isLeaf: true },
          { title: 'staging-db-01 (10.1.0.20)', key: 'stg-db-01', icon: <DatabaseOutlined />, isLeaf: true },
        ],
      },
    ],
  },
];

/** 模拟资产详情数据，以树节点 key 为索引 */
const MOCK_ASSET_INFO: Record<string, AssetNode> = {
  'pay-gw-01': { hostname: 'pay-gateway-01', ip: '10.0.1.10', type: 'ECS', grade: 'S', status: 'online' },
  'pay-gw-02': { hostname: 'pay-gateway-02', ip: '10.0.1.11', type: 'ECS', grade: 'S', status: 'online' },
  'pay-db-m': { hostname: 'pay-db-master', ip: '10.0.1.20', type: 'RDS', grade: 'S', status: 'online' },
  'pay-db-s': { hostname: 'pay-db-slave', ip: '10.0.1.21', type: 'RDS', grade: 'A', status: 'online' },
  'order-api-01': { hostname: 'order-api-01', ip: '10.0.2.10', type: 'ECS', grade: 'A', status: 'online' },
  'order-api-02': { hostname: 'order-api-02', ip: '10.0.2.11', type: 'ECS', grade: 'A', status: 'maintenance' },
  'order-db': { hostname: 'order-mysql', ip: '10.0.2.20', type: 'RDS', grade: 'A', status: 'online' },
  'stg-app-01': { hostname: 'staging-app-01', ip: '10.1.0.10', type: 'ECS', grade: 'C', status: 'online' },
  'stg-db-01': { hostname: 'staging-db-01', ip: '10.1.0.20', type: 'RDS', grade: 'C', status: 'online' },
};

/** 资产状态对应的 Badge 状态映射 */
const STATUS_MAP: Record<string, 'success' | 'warning' | 'error' | 'default'> = {
  online: 'success',       // 在线
  offline: 'error',        // 离线
  maintenance: 'warning',  // 维护中
};

/**
 * 拓扑页面组件
 * 左右双栏布局：
 * - 左侧：Tree 组件展示资产层级，默认展开所有节点
 * - 右侧：选中叶子节点时展示资产详细信息（Descriptions）
 */
const Topology: React.FC = () => {
  const { t } = useTranslation('cmdb');
  const [selectedNode, setSelectedNode] = useState<AssetNode | null>(null); // 当前选中的资产节点

  /**
   * 处理树节点选中事件
   * 查找选中 key 对应的资产信息并展示
   * @param selectedKeys 选中的节点 key 数组
   */
  const handleSelect = (selectedKeys: React.Key[]) => {
    const key = selectedKeys[0] as string;
    setSelectedNode(MOCK_ASSET_INFO[key] || null);
  };

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('topology.title')}</Text>
      </div>
      <div style={{ display: 'flex', gap: 16 }}>
        {/* 左侧：拓扑树 */}
        <Card style={{ borderRadius: 8, flex: 1, minHeight: 500 }}>
          <Tree
            showIcon
            defaultExpandAll
            treeData={MOCK_TREE}
            onSelect={handleSelect}
            style={{ fontSize: 14 }}
          />
        </Card>

        {/* 右侧：资产详情面板 */}
        <Card
          title={t('topology.assetDetail')}
          style={{ borderRadius: 8, width: 380 }}
        >
          {selectedNode ? (
            // 显示选中资产的详细信息
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label={t('assets.column.hostname')}>{selectedNode.hostname}</Descriptions.Item>
              <Descriptions.Item label={t('assets.column.ip')}>{selectedNode.ip}</Descriptions.Item>
              <Descriptions.Item label={t('assets.column.type')}>{selectedNode.type}</Descriptions.Item>
              <Descriptions.Item label={t('assets.column.grade')}>
                <Tag color={GRADE_COLORS[selectedNode.grade]}>{selectedNode.grade}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label={t('assets.column.status')}>
                <Badge status={STATUS_MAP[selectedNode.status] || 'default'} text={selectedNode.status} />
              </Descriptions.Item>
            </Descriptions>
          ) : (
            // 未选中时显示提示
            <Empty description={t('topology.selectNode')} />
          )}
        </Card>
      </div>
    </div>
  );
};

export default Topology;
