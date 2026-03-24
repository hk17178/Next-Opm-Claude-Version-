/**
 * RBAC 权限管理页面 - 角色、权限、用户-角色分配的统一管理
 *
 * 功能模块：
 * - 角色管理 Tab：角色列表（角色名/描述/用户数/权限数/操作）、创建角色弹窗（名称+描述+权限树勾选）
 * - 权限管理 Tab：权限树形表格（模块/读/写/删除/管理 复选框矩阵）
 * - 用户分配 Tab：用户-角色分配表格，支持批量分配
 */
import React, { useState, useCallback } from 'react';
import {
  Card, Tabs, Table, Button, Space, Typography, Tag, Modal, Form, Input,
  Tree, Checkbox, Popconfirm, Badge, Tooltip, message,
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SafetyCertificateOutlined,
  TeamOutlined, LockOutlined, UserSwitchOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;
const { TextArea } = Input;

// ==================== 类型定义 ====================

/** 角色记录 */
interface RoleRecord {
  key: string;           // 唯一标识
  name: string;          // 角色名称
  description: string;   // 角色描述
  userCount: number;     // 关联用户数
  permissionCount: number; // 关联权限数
  builtIn: boolean;      // 是否内置角色（内置角色不可删除）
  createdAt: string;     // 创建时间
}

/** 权限矩阵行 */
interface PermissionRow {
  key: string;           // 唯一标识
  module: string;        // 功能模块名称
  moduleKey: string;     // 功能模块标识
  read: boolean;         // 读权限
  write: boolean;        // 写权限
  delete: boolean;       // 删权限
  manage: boolean;       // 管理权限
  children?: PermissionRow[]; // 子模块
}

/** 用户-角色分配记录 */
interface UserRoleRecord {
  key: string;           // 唯一标识
  username: string;      // 用户名
  displayName: string;   // 显示名称
  email: string;         // 邮箱
  roles: string[];       // 已分配角色列表
  lastLogin?: string;    // 最后登录时间
}

// ==================== Mock 数据 ====================

/** Mock 角色列表 */
const mockRoles: RoleRecord[] = [
  { key: '1', name: '超级管理员', description: '拥有系统所有权限', userCount: 2, permissionCount: 45, builtIn: true, createdAt: '2025-01-01 00:00' },
  { key: '2', name: '运维工程师', description: '日常运维操作权限', userCount: 8, permissionCount: 28, builtIn: true, createdAt: '2025-01-01 00:00' },
  { key: '3', name: '开发人员', description: '查看监控和日志权限', userCount: 15, permissionCount: 12, builtIn: false, createdAt: '2025-03-15 10:30' },
  { key: '4', name: '审计员', description: '查看审计日志和操作记录', userCount: 3, permissionCount: 8, builtIn: false, createdAt: '2025-04-20 14:00' },
  { key: '5', name: '只读用户', description: '仅查看权限', userCount: 20, permissionCount: 6, builtIn: true, createdAt: '2025-01-01 00:00' },
];

/** Mock 权限矩阵数据 */
const mockPermissions: PermissionRow[] = [
  {
    key: 'alert', module: '告警中心', moduleKey: 'alert',
    read: true, write: true, delete: true, manage: true,
    children: [
      { key: 'alert-list', module: '告警列表', moduleKey: 'alert.list', read: true, write: true, delete: false, manage: false },
      { key: 'alert-rule', module: '告警规则', moduleKey: 'alert.rule', read: true, write: true, delete: true, manage: true },
      { key: 'alert-silence', module: '告警静默', moduleKey: 'alert.silence', read: true, write: true, delete: true, manage: false },
    ],
  },
  {
    key: 'monitor', module: '监控管理', moduleKey: 'monitor',
    read: true, write: true, delete: false, manage: true,
    children: [
      { key: 'monitor-dashboard', module: '监控面板', moduleKey: 'monitor.dashboard', read: true, write: true, delete: false, manage: false },
      { key: 'monitor-metric', module: '指标查询', moduleKey: 'monitor.metric', read: true, write: false, delete: false, manage: false },
    ],
  },
  {
    key: 'cmdb', module: '资产管理', moduleKey: 'cmdb',
    read: true, write: true, delete: true, manage: true,
    children: [
      { key: 'cmdb-asset', module: '资产列表', moduleKey: 'cmdb.asset', read: true, write: true, delete: true, manage: false },
      { key: 'cmdb-topology', module: '拓扑图', moduleKey: 'cmdb.topology', read: true, write: false, delete: false, manage: false },
    ],
  },
  {
    key: 'kb', module: '知识库', moduleKey: 'kb',
    read: true, write: true, delete: true, manage: true,
  },
  {
    key: 'settings', module: '系统设置', moduleKey: 'settings',
    read: true, write: true, delete: true, manage: true,
    children: [
      { key: 'settings-user', module: '用户管理', moduleKey: 'settings.user', read: true, write: true, delete: true, manage: true },
      { key: 'settings-rbac', module: '角色权限', moduleKey: 'settings.rbac', read: true, write: true, delete: true, manage: true },
      { key: 'settings-cluster', module: '集群管理', moduleKey: 'settings.cluster', read: true, write: true, delete: false, manage: true },
    ],
  },
  {
    key: 'audit', module: '审计日志', moduleKey: 'audit',
    read: true, write: false, delete: false, manage: true,
  },
];

/** Mock 用户-角色分配列表 */
const mockUserRoles: UserRoleRecord[] = [
  { key: '1', username: 'admin', displayName: '系统管理员', email: 'admin@opsnexus.io', roles: ['超级管理员'], lastLogin: '2026-03-24 09:30' },
  { key: '2', username: 'zhangsan', displayName: '张三', email: 'zhangsan@opsnexus.io', roles: ['运维工程师'], lastLogin: '2026-03-24 08:15' },
  { key: '3', username: 'lisi', displayName: '李四', email: 'lisi@opsnexus.io', roles: ['开发人员', '只读用户'], lastLogin: '2026-03-23 17:45' },
  { key: '4', username: 'wangwu', displayName: '王五', email: 'wangwu@opsnexus.io', roles: ['审计员'], lastLogin: '2026-03-22 14:20' },
  { key: '5', username: 'zhaoliu', displayName: '赵六', email: 'zhaoliu@opsnexus.io', roles: ['运维工程师', '开发人员'], lastLogin: '2026-03-24 10:00' },
];

/** 权限树数据（用于创建/编辑角色弹窗中的权限勾选） */
const permissionTreeData = [
  {
    title: '告警中心', key: 'alert', children: [
      { title: '告警列表', key: 'alert.list' },
      { title: '告警规则', key: 'alert.rule' },
      { title: '告警静默', key: 'alert.silence' },
    ],
  },
  {
    title: '监控管理', key: 'monitor', children: [
      { title: '监控面板', key: 'monitor.dashboard' },
      { title: '指标查询', key: 'monitor.metric' },
    ],
  },
  {
    title: '资产管理', key: 'cmdb', children: [
      { title: '资产列表', key: 'cmdb.asset' },
      { title: '拓扑图', key: 'cmdb.topology' },
    ],
  },
  { title: '知识库', key: 'kb' },
  {
    title: '系统设置', key: 'settings', children: [
      { title: '用户管理', key: 'settings.user' },
      { title: '角色权限', key: 'settings.rbac' },
      { title: '集群管理', key: 'settings.cluster' },
    ],
  },
  { title: '审计日志', key: 'audit' },
];

// ==================== 组件实现 ====================

/**
 * RBAC 权限管理组件
 * 包含三个 Tab：角色管理 / 权限管理 / 用户分配
 */
const RBACManagement: React.FC = () => {
  const { t } = useTranslation('settings');
  const [activeTab, setActiveTab] = useState('roles');           // 当前活动 Tab
  const [roleModalOpen, setRoleModalOpen] = useState(false);     // 创建/编辑角色弹窗
  const [editingRole, setEditingRole] = useState<RoleRecord | null>(null); // 正在编辑的角色
  const [checkedKeys, setCheckedKeys] = useState<React.Key[]>([]); // 权限树勾选的 key
  const [roleForm] = Form.useForm();                              // 角色表单实例
  const [permissionData, setPermissionData] = useState<PermissionRow[]>(mockPermissions); // 权限矩阵数据
  const [assignModalOpen, setAssignModalOpen] = useState(false);  // 用户角色分配弹窗
  const [assigningUser, setAssigningUser] = useState<UserRoleRecord | null>(null); // 正在分配的用户
  const [assignedRoles, setAssignedRoles] = useState<string[]>([]); // 已分配的角色

  // ==================== 角色管理 ====================

  /**
   * 打开创建角色弹窗
   * 重置表单和权限树选中状态
   */
  const handleAddRole = useCallback(() => {
    setEditingRole(null);
    roleForm.resetFields();
    setCheckedKeys([]);
    setRoleModalOpen(true);
  }, [roleForm]);

  /**
   * 打开编辑角色弹窗
   * 填充角色数据到表单
   */
  const handleEditRole = useCallback((record: RoleRecord) => {
    setEditingRole(record);
    roleForm.setFieldsValue({ name: record.name, description: record.description });
    // TODO: 从 API 获取该角色的权限列表，设置 checkedKeys
    setCheckedKeys(['alert.list', 'alert.rule', 'monitor.dashboard']);
    setRoleModalOpen(true);
  }, [roleForm]);

  /**
   * 保存角色（创建或更新）
   * 校验表单后调用 API
   */
  const handleSaveRole = useCallback(() => {
    roleForm.validateFields().then((values) => {
      console.log('保存角色:', values, '权限:', checkedKeys);
      message.success(editingRole ? t('rbac.role.updateSuccess') : t('rbac.role.createSuccess'));
      setRoleModalOpen(false);
      // TODO: 对接角色管理 API
    });
  }, [roleForm, checkedKeys, editingRole, t]);

  /**
   * 删除角色（二次确认后调用 API）
   */
  const handleDeleteRole = useCallback((record: RoleRecord) => {
    console.log('删除角色:', record.key);
    message.success(t('rbac.role.deleteSuccess'));
    // TODO: 对接角色删除 API
  }, [t]);

  /** 角色列表表格列定义 */
  const roleColumns = [
    {
      title: t('rbac.role.column.name'),
      dataIndex: 'name',
      key: 'name',
      width: 160,
      /** 渲染角色名称，内置角色带标记 */
      render: (name: string, record: RoleRecord) => (
        <Space>
          <SafetyCertificateOutlined />
          <span>{name}</span>
          {record.builtIn && <Tag color="blue">{t('rbac.role.builtIn')}</Tag>}
        </Space>
      ),
    },
    {
      title: t('rbac.role.column.description'),
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: t('rbac.role.column.userCount'),
      dataIndex: 'userCount',
      key: 'userCount',
      width: 100,
      /** 渲染用户数量标签 */
      render: (count: number) => <Badge count={count} showZero color="#2E75B6" overflowCount={999} />,
    },
    {
      title: t('rbac.role.column.permissionCount'),
      dataIndex: 'permissionCount',
      key: 'permissionCount',
      width: 100,
      /** 渲染权限数量标签 */
      render: (count: number) => <Badge count={count} showZero color="#00B42A" overflowCount={999} />,
    },
    {
      title: t('rbac.role.column.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 160,
    },
    {
      title: t('rbac.role.column.actions'),
      key: 'actions',
      width: 160,
      /** 渲染操作按钮：编辑、删除（内置角色不可删除） */
      render: (_: unknown, record: RoleRecord) => (
        <Space>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEditRole(record)}>
            {t('rbac.role.edit')}
          </Button>
          {!record.builtIn && (
            <Popconfirm
              title={t('rbac.role.deleteConfirm')}
              onConfirm={() => handleDeleteRole(record)}
              okText={t('rbac.confirm')}
              cancelText={t('rbac.cancel')}
            >
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                {t('rbac.role.delete')}
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  // ==================== 权限管理 ====================

  /**
   * 切换权限矩阵中的权限勾选状态
   * @param key 行标识
   * @param field 权限字段（read/write/delete/manage）
   * @param checked 是否勾选
   */
  const handlePermissionChange = useCallback((key: string, field: 'read' | 'write' | 'delete' | 'manage', checked: boolean) => {
    /** 递归更新权限数据 */
    const updatePermissions = (data: PermissionRow[]): PermissionRow[] =>
      data.map((item) => {
        if (item.key === key) {
          return { ...item, [field]: checked };
        }
        if (item.children) {
          return { ...item, children: updatePermissions(item.children) };
        }
        return item;
      });
    setPermissionData(updatePermissions(permissionData));
  }, [permissionData]);

  /** 权限矩阵表格列定义 */
  const permissionColumns = [
    {
      title: t('rbac.permission.column.module'),
      dataIndex: 'module',
      key: 'module',
      width: 200,
    },
    {
      title: t('rbac.permission.column.read'),
      dataIndex: 'read',
      key: 'read',
      width: 100,
      align: 'center' as const,
      /** 渲染读权限复选框 */
      render: (val: boolean, record: PermissionRow) => (
        <Checkbox checked={val} onChange={(e) => handlePermissionChange(record.key, 'read', e.target.checked)} />
      ),
    },
    {
      title: t('rbac.permission.column.write'),
      dataIndex: 'write',
      key: 'write',
      width: 100,
      align: 'center' as const,
      /** 渲染写权限复选框 */
      render: (val: boolean, record: PermissionRow) => (
        <Checkbox checked={val} onChange={(e) => handlePermissionChange(record.key, 'write', e.target.checked)} />
      ),
    },
    {
      title: t('rbac.permission.column.delete'),
      dataIndex: 'delete',
      key: 'delete',
      width: 100,
      align: 'center' as const,
      /** 渲染删权限复选框 */
      render: (val: boolean, record: PermissionRow) => (
        <Checkbox checked={val} onChange={(e) => handlePermissionChange(record.key, 'delete', e.target.checked)} />
      ),
    },
    {
      title: t('rbac.permission.column.manage'),
      dataIndex: 'manage',
      key: 'manage',
      width: 100,
      align: 'center' as const,
      /** 渲染管理权限复选框 */
      render: (val: boolean, record: PermissionRow) => (
        <Checkbox checked={val} onChange={(e) => handlePermissionChange(record.key, 'manage', e.target.checked)} />
      ),
    },
  ];

  // ==================== 用户分配 ====================

  /**
   * 打开用户角色分配弹窗
   * @param record 用户记录
   */
  const handleAssignRoles = useCallback((record: UserRoleRecord) => {
    setAssigningUser(record);
    setAssignedRoles(record.roles);
    setAssignModalOpen(true);
  }, []);

  /**
   * 保存用户角色分配
   */
  const handleSaveAssignment = useCallback(() => {
    console.log('保存用户角色分配:', assigningUser?.username, assignedRoles);
    message.success(t('rbac.assign.saveSuccess'));
    setAssignModalOpen(false);
    // TODO: 对接用户-角色分配 API
  }, [assigningUser, assignedRoles, t]);

  /** 用户-角色分配表格列定义 */
  const userRoleColumns = [
    {
      title: t('rbac.assign.column.username'),
      dataIndex: 'username',
      key: 'username',
      width: 120,
    },
    {
      title: t('rbac.assign.column.displayName'),
      dataIndex: 'displayName',
      key: 'displayName',
      width: 120,
    },
    {
      title: t('rbac.assign.column.email'),
      dataIndex: 'email',
      key: 'email',
      width: 200,
      ellipsis: true,
    },
    {
      title: t('rbac.assign.column.roles'),
      dataIndex: 'roles',
      key: 'roles',
      /** 渲染角色标签列表 */
      render: (roles: string[]) => (
        <Space wrap>
          {roles.map((role) => (
            <Tag key={role} color="blue">{role}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: t('rbac.assign.column.lastLogin'),
      dataIndex: 'lastLogin',
      key: 'lastLogin',
      width: 160,
      render: (val: string | undefined) => val || <Text type="secondary">--</Text>,
    },
    {
      title: t('rbac.assign.column.actions'),
      key: 'actions',
      width: 120,
      /** 渲染分配角色按钮 */
      render: (_: unknown, record: UserRoleRecord) => (
        <Button type="link" size="small" icon={<UserSwitchOutlined />} onClick={() => handleAssignRoles(record)}>
          {t('rbac.assign.assignRole')}
        </Button>
      ),
    },
  ];

  // ==================== Tab 配置 ====================

  /** Tab 页配置项 */
  const tabItems = [
    {
      key: 'roles',
      label: (
        <span><SafetyCertificateOutlined /> {t('rbac.tab.roles')}</span>
      ),
      children: (
        <>
          {/* 角色列表表格 */}
          <Table<RoleRecord>
            columns={roleColumns}
            dataSource={mockRoles}
            rowKey="key"
            size="middle"
            pagination={{ pageSize: 10, showTotal: (total) => t('rbac.role.total', { count: total }) }}
          />
        </>
      ),
    },
    {
      key: 'permissions',
      label: (
        <span><LockOutlined /> {t('rbac.tab.permissions')}</span>
      ),
      children: (
        <>
          {/* 权限角色选择提示 */}
          <div style={{ marginBottom: 16 }}>
            <Text type="secondary">{t('rbac.permission.hint')}</Text>
          </div>
          {/* 权限树形表格 */}
          <Table<PermissionRow>
            columns={permissionColumns}
            dataSource={permissionData}
            rowKey="key"
            size="middle"
            pagination={false}
            expandable={{ defaultExpandAllRows: true }}
          />
          {/* 保存权限按钮 */}
          <div style={{ marginTop: 16, textAlign: 'right' }}>
            <Button type="primary" onClick={() => message.success(t('rbac.permission.saveSuccess'))}>
              {t('rbac.permission.save')}
            </Button>
          </div>
        </>
      ),
    },
    {
      key: 'assignment',
      label: (
        <span><TeamOutlined /> {t('rbac.tab.assignment')}</span>
      ),
      children: (
        <>
          {/* 用户-角色分配表格 */}
          <Table<UserRoleRecord>
            columns={userRoleColumns}
            dataSource={mockUserRoles}
            rowKey="key"
            size="middle"
            pagination={{ pageSize: 10, showTotal: (total) => t('rbac.assign.total', { count: total }) }}
          />
        </>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与创建角色按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('rbac.title')}</Text>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleAddRole}>
          {t('rbac.role.create')}
        </Button>
      </div>

      {/* Tab 面板 */}
      <Card style={{ borderRadius: 8 }}>
        <Tabs items={tabItems} activeKey={activeTab} onChange={setActiveTab} />
      </Card>

      {/* 创建/编辑角色弹窗 */}
      <Modal
        title={editingRole ? t('rbac.role.editTitle') : t('rbac.role.createTitle')}
        open={roleModalOpen}
        onCancel={() => setRoleModalOpen(false)}
        onOk={handleSaveRole}
        okText={t('rbac.confirm')}
        cancelText={t('rbac.cancel')}
        width={640}
      >
        <Form form={roleForm} layout="vertical" style={{ marginTop: 16 }}>
          {/* 角色名称 */}
          <Form.Item
            name="name"
            label={t('rbac.role.form.name')}
            rules={[{ required: true, message: t('rbac.role.form.nameRequired') }]}
          >
            <Input placeholder={t('rbac.role.form.namePlaceholder')} />
          </Form.Item>
          {/* 角色描述 */}
          <Form.Item name="description" label={t('rbac.role.form.description')}>
            <TextArea rows={3} placeholder={t('rbac.role.form.descriptionPlaceholder')} />
          </Form.Item>
          {/* 权限树勾选 */}
          <Form.Item label={t('rbac.role.form.permissions')}>
            <div style={{ border: '1px solid #d9d9d9', borderRadius: 6, padding: 12, maxHeight: 300, overflow: 'auto' }}>
              <Tree
                checkable
                checkedKeys={checkedKeys}
                onCheck={(keys) => setCheckedKeys(keys as React.Key[])}
                treeData={permissionTreeData}
                defaultExpandAll
              />
            </div>
          </Form.Item>
        </Form>
      </Modal>

      {/* 用户角色分配弹窗 */}
      <Modal
        title={t('rbac.assign.modalTitle', { user: assigningUser?.displayName })}
        open={assignModalOpen}
        onCancel={() => setAssignModalOpen(false)}
        onOk={handleSaveAssignment}
        okText={t('rbac.confirm')}
        cancelText={t('rbac.cancel')}
        width={480}
      >
        <div style={{ marginTop: 16 }}>
          <Text type="secondary" style={{ marginBottom: 12, display: 'block' }}>
            {t('rbac.assign.selectRoles')}
          </Text>
          {/* 角色复选框组 */}
          <Checkbox.Group
            value={assignedRoles}
            onChange={(values) => setAssignedRoles(values as string[])}
            style={{ display: 'flex', flexDirection: 'column', gap: 8 }}
          >
            {mockRoles.map((role) => (
              <Checkbox key={role.key} value={role.name}>
                <Space>
                  <span>{role.name}</span>
                  <Text type="secondary" style={{ fontSize: 12 }}>({role.description})</Text>
                </Space>
              </Checkbox>
            ))}
          </Checkbox.Group>
        </div>
      </Modal>
    </div>
  );
};

export default RBACManagement;
