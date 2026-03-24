/**
 * 资产分组管理页面 (/cmdb/groups)
 *
 * 功能模块：
 * 1. 左侧分组树形列表（Tree 组件，支持展开/折叠）
 * 2. 右侧分组详情面板（组名 / 描述 / 成员数 / 创建时间）
 * 3. 分组成员列表表格（主机名 / IP / 类型 / 分级 / 状态）
 * 4. 创建 / 编辑分组弹窗（Modal + Form）
 * 5. 右键菜单操作（编辑 / 删除）
 */
import React, { useState, useCallback } from 'react';
import {
  Typography, Card, Tree, Table, Button, Space, Modal, Form,
  Input, Tag, Badge, Descriptions, Empty, Dropdown, message,
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined,
  FolderOutlined, FolderOpenOutlined, DesktopOutlined,
  DatabaseOutlined, CloudServerOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { DataNode } from 'antd/es/tree';

const { Text } = Typography;
const { TextArea } = Input;

/* ============================== 类型定义 ============================== */

/** 分组数据结构 */
interface AssetGroup {
  /** 分组唯一 ID */
  id: string;
  /** 分组名称 */
  name: string;
  /** 分组描述 */
  description: string;
  /** 成员数量 */
  memberCount: number;
  /** 创建时间 */
  createdAt: string;
  /** 父分组 ID（顶层为 null） */
  parentId: string | null;
}

/** 分组成员数据结构 */
interface GroupMember {
  /** 资产唯一 ID */
  id: string;
  /** 主机名 */
  hostname: string;
  /** IP 地址 */
  ip: string;
  /** 资产类型 */
  type: string;
  /** 资产分级 */
  grade: string;
  /** 状态 */
  status: string;
}

/* ============================== Mock 数据 ============================== */

/** 模拟分组列表 */
const MOCK_GROUPS: Record<string, AssetGroup> = {
  'grp-prod': {
    id: 'grp-prod',
    name: '生产环境',
    description: '所有生产环境资产，包括应用服务器、数据库、中间件等核心设备',
    memberCount: 156,
    createdAt: '2025-06-15 10:00:00',
    parentId: null,
  },
  'grp-prod-payment': {
    id: 'grp-prod-payment',
    name: '支付业务组',
    description: '支付核心链路相关服务器及数据库',
    memberCount: 24,
    createdAt: '2025-07-01 14:30:00',
    parentId: 'grp-prod',
  },
  'grp-prod-order': {
    id: 'grp-prod-order',
    name: '订单业务组',
    description: '订单处理系统相关服务集群',
    memberCount: 18,
    createdAt: '2025-07-05 09:00:00',
    parentId: 'grp-prod',
  },
  'grp-prod-user': {
    id: 'grp-prod-user',
    name: '用户中心组',
    description: '用户注册、登录、权限管理相关服务',
    memberCount: 12,
    createdAt: '2025-07-10 11:20:00',
    parentId: 'grp-prod',
  },
  'grp-staging': {
    id: 'grp-staging',
    name: '预发环境',
    description: '预发布验证环境，与生产保持一致的配置',
    memberCount: 42,
    createdAt: '2025-06-20 15:00:00',
    parentId: null,
  },
  'grp-staging-all': {
    id: 'grp-staging-all',
    name: '全业务验证组',
    description: '预发环境全业务线集成测试资产',
    memberCount: 42,
    createdAt: '2025-06-22 10:00:00',
    parentId: 'grp-staging',
  },
  'grp-dev': {
    id: 'grp-dev',
    name: '开发环境',
    description: '开发与联调环境资产',
    memberCount: 68,
    createdAt: '2025-06-25 09:00:00',
    parentId: null,
  },
};

/** 模拟分组树结构 */
const MOCK_TREE: DataNode[] = [
  {
    title: '生产环境',
    key: 'grp-prod',
    icon: <FolderOpenOutlined />,
    children: [
      { title: '支付业务组', key: 'grp-prod-payment', icon: <FolderOutlined />, children: [] },
      { title: '订单业务组', key: 'grp-prod-order', icon: <FolderOutlined />, children: [] },
      { title: '用户中心组', key: 'grp-prod-user', icon: <FolderOutlined />, children: [] },
    ],
  },
  {
    title: '预发环境',
    key: 'grp-staging',
    icon: <FolderOpenOutlined />,
    children: [
      { title: '全业务验证组', key: 'grp-staging-all', icon: <FolderOutlined />, children: [] },
    ],
  },
  {
    title: '开发环境',
    key: 'grp-dev',
    icon: <FolderOutlined />,
    children: [],
  },
];

/** 模拟分组成员数据 —— 根据分组 ID 返回成员列表 */
const MOCK_MEMBERS: Record<string, GroupMember[]> = {
  'grp-prod-payment': [
    { id: '1', hostname: 'pay-gateway-01', ip: '10.0.1.10', type: 'ECS', grade: 'S', status: 'online' },
    { id: '2', hostname: 'pay-gateway-02', ip: '10.0.1.11', type: 'ECS', grade: 'S', status: 'online' },
    { id: '3', hostname: 'pay-db-master', ip: '10.0.1.20', type: 'RDS', grade: 'S', status: 'online' },
    { id: '4', hostname: 'pay-db-slave', ip: '10.0.1.21', type: 'RDS', grade: 'A', status: 'online' },
    { id: '5', hostname: 'pay-cache-01', ip: '10.0.1.30', type: 'Redis', grade: 'A', status: 'online' },
    { id: '6', hostname: 'pay-mq-01', ip: '10.0.1.40', type: 'MQ', grade: 'B', status: 'online' },
  ],
  'grp-prod-order': [
    { id: '7', hostname: 'order-api-01', ip: '10.0.2.10', type: 'ECS', grade: 'A', status: 'online' },
    { id: '8', hostname: 'order-api-02', ip: '10.0.2.11', type: 'ECS', grade: 'A', status: 'maintenance' },
    { id: '9', hostname: 'order-mysql', ip: '10.0.2.20', type: 'RDS', grade: 'A', status: 'online' },
    { id: '10', hostname: 'order-redis', ip: '10.0.2.30', type: 'Redis', grade: 'B', status: 'online' },
  ],
  'grp-prod-user': [
    { id: '11', hostname: 'user-api-01', ip: '10.0.3.10', type: 'ECS', grade: 'B', status: 'online' },
    { id: '12', hostname: 'user-db-01', ip: '10.0.3.20', type: 'RDS', grade: 'B', status: 'online' },
  ],
};

/** 资产分级对应的颜色映射 */
const GRADE_COLORS: Record<string, string> = {
  S: '#F53F3F', A: '#FF7D00', B: '#3491FA', C: '#86909C', D: '#C9CDD4',
};

/** 资产状态对应的 Badge 状态映射 */
const STATUS_MAP: Record<string, 'success' | 'warning' | 'error' | 'default'> = {
  online: 'success',
  offline: 'error',
  maintenance: 'warning',
};

/** 资产类型对应的图标映射 */
const TYPE_ICONS: Record<string, React.ReactNode> = {
  ECS: <CloudServerOutlined />,
  RDS: <DatabaseOutlined />,
  Redis: <DatabaseOutlined />,
  MQ: <CloudServerOutlined />,
};

/* ============================== 主组件 ============================== */

/**
 * 资产分组管理页面
 * 左右双栏布局：左侧树 + 右侧详情/成员表格
 */
const AssetGroups: React.FC = () => {
  const { t } = useTranslation('cmdb');
  const [form] = Form.useForm();

  /** 当前选中的分组 ID */
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null);
  /** 创建/编辑弹窗可见状态 */
  const [modalVisible, setModalVisible] = useState(false);
  /** 弹窗模式：create（创建）或 edit（编辑） */
  const [modalMode, setModalMode] = useState<'create' | 'edit'>('create');

  /** 获取当前选中分组信息 */
  const selectedGroup = selectedGroupId ? MOCK_GROUPS[selectedGroupId] : null;
  /** 获取当前选中分组的成员列表 */
  const members = selectedGroupId ? (MOCK_MEMBERS[selectedGroupId] || []) : [];

  /**
   * 处理树节点选中事件
   * @param selectedKeys 被选中的节点 key 数组
   */
  const handleSelect = useCallback((selectedKeys: React.Key[]) => {
    const key = selectedKeys[0] as string;
    setSelectedGroupId(key || null);
  }, []);

  /** 打开创建分组弹窗 */
  const handleCreate = useCallback(() => {
    setModalMode('create');
    form.resetFields();
    setModalVisible(true);
  }, [form]);

  /** 打开编辑分组弹窗 */
  const handleEdit = useCallback(() => {
    if (!selectedGroup) return;
    setModalMode('edit');
    form.setFieldsValue({
      name: selectedGroup.name,
      description: selectedGroup.description,
    });
    setModalVisible(true);
  }, [selectedGroup, form]);

  /** 提交创建/编辑表单 */
  const handleModalOk = useCallback(async () => {
    try {
      await form.validateFields();
      message.success(
        modalMode === 'create'
          ? t('groups.createSuccess')
          : t('groups.editSuccess')
      );
      setModalVisible(false);
    } catch {
      // 表单校验失败，不执行操作
    }
  }, [form, modalMode, t]);

  /** 处理删除分组 */
  const handleDelete = useCallback(() => {
    if (!selectedGroup) return;
    Modal.confirm({
      title: t('groups.deleteConfirm'),
      content: `${t('groups.deleteContent')} "${selectedGroup.name}"?`,
      okText: t('groups.confirm'),
      cancelText: t('groups.cancel'),
      okButtonProps: { danger: true },
      onOk: () => {
        message.success(t('groups.deleteSuccess'));
        setSelectedGroupId(null);
      },
    });
  }, [selectedGroup, t]);

  /** 树节点右键菜单项 */
  const contextMenuItems = [
    { key: 'edit', label: t('groups.edit'), icon: <EditOutlined /> },
    { key: 'delete', label: t('groups.delete'), icon: <DeleteOutlined />, danger: true },
  ];

  /** 分组成员表格列定义 */
  const memberColumns = [
    {
      title: t('assets.column.hostname'),
      dataIndex: 'hostname',
      key: 'hostname',
      width: 180,
      /** 主机名前添加类型图标 */
      render: (hostname: string, record: GroupMember) => (
        <Space>
          {TYPE_ICONS[record.type] || <DesktopOutlined />}
          <span>{hostname}</span>
        </Space>
      ),
    },
    {
      title: t('assets.column.ip'),
      dataIndex: 'ip',
      key: 'ip',
      width: 130,
    },
    {
      title: t('assets.column.type'),
      dataIndex: 'type',
      key: 'type',
      width: 100,
    },
    {
      title: t('assets.column.grade'),
      dataIndex: 'grade',
      key: 'grade',
      width: 80,
      /** 渲染分级标签 */
      render: (grade: string) => (
        <Tag color={GRADE_COLORS[grade]} style={{ borderRadius: 4, fontWeight: 600, border: 'none' }}>
          {grade}
        </Tag>
      ),
    },
    {
      title: t('assets.column.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      /** 渲染状态 Badge */
      render: (status: string) => (
        <Badge status={STATUS_MAP[status] || 'default'} text={t(`assets.status.${status}`)} />
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与操作按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('groups.title')}</Text>
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            {t('groups.create')}
          </Button>
        </Space>
      </div>

      {/* 左右双栏布局 */}
      <div style={{ display: 'flex', gap: 16 }}>
        {/* 左侧：分组树 */}
        <Card style={{ borderRadius: 12, width: 280, minHeight: 600, flexShrink: 0 }}>
          <Tree
            showIcon
            defaultExpandAll
            treeData={MOCK_TREE}
            onSelect={handleSelect}
            selectedKeys={selectedGroupId ? [selectedGroupId] : []}
            titleRender={(nodeData) => (
              <Dropdown
                menu={{
                  items: contextMenuItems,
                  onClick: ({ key }) => {
                    setSelectedGroupId(nodeData.key as string);
                    if (key === 'edit') {
                      setTimeout(() => handleEdit(), 0);
                    } else if (key === 'delete') {
                      setTimeout(() => handleDelete(), 0);
                    }
                  },
                }}
                trigger={['contextMenu']}
              >
                <span>{nodeData.title as React.ReactNode}</span>
              </Dropdown>
            )}
            style={{ fontSize: 14 }}
          />
        </Card>

        {/* 右侧：分组详情 + 成员列表 */}
        <div style={{ flex: 1 }}>
          {selectedGroup ? (
            <>
              {/* 分组详情卡片 */}
              <Card
                title={t('groups.detail')}
                style={{ borderRadius: 12, marginBottom: 16 }}
                extra={
                  <Space>
                    <Button icon={<EditOutlined />} onClick={handleEdit}>{t('groups.edit')}</Button>
                    <Button danger icon={<DeleteOutlined />} onClick={handleDelete}>{t('groups.delete')}</Button>
                  </Space>
                }
              >
                <Descriptions column={2} size="small">
                  <Descriptions.Item label={t('groups.name')}>{selectedGroup.name}</Descriptions.Item>
                  <Descriptions.Item label={t('groups.memberCount')}>
                    <Tag color="blue">{selectedGroup.memberCount}</Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label={t('groups.description')} span={2}>
                    {selectedGroup.description}
                  </Descriptions.Item>
                  <Descriptions.Item label={t('groups.createdAt')}>
                    {selectedGroup.createdAt}
                  </Descriptions.Item>
                </Descriptions>
              </Card>

              {/* 分组成员列表 */}
              <Card
                title={t('groups.members')}
                style={{ borderRadius: 12 }}
                styles={{ body: { padding: 0 } }}
              >
                <Table
                  columns={memberColumns}
                  dataSource={members}
                  rowKey="id"
                  size="middle"
                  pagination={false}
                  locale={{ emptyText: t('groups.noMembers') }}
                />
              </Card>
            </>
          ) : (
            /* 未选中分组时的空状态提示 */
            <Card style={{ borderRadius: 12, minHeight: 400, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Empty description={t('groups.selectGroup')} />
            </Card>
          )}
        </div>
      </div>

      {/* 创建/编辑分组弹窗 */}
      <Modal
        title={modalMode === 'create' ? t('groups.createTitle') : t('groups.editTitle')}
        open={modalVisible}
        onOk={handleModalOk}
        onCancel={() => setModalVisible(false)}
        okText={t('groups.confirm')}
        cancelText={t('groups.cancel')}
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="name"
            label={t('groups.name')}
            rules={[{ required: true, message: t('groups.nameRequired') }]}
          >
            <Input placeholder={t('groups.namePlaceholder')} />
          </Form.Item>
          <Form.Item
            name="description"
            label={t('groups.description')}
          >
            <TextArea rows={3} placeholder={t('groups.descPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default AssetGroups;
