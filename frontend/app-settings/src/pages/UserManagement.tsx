/**
 * 用户管理页面 - 管理系统用户的增删改查，支持按角色/来源过滤、关键词搜索
 * 包含：过滤条件栏、用户列表表格、创建/编辑用户弹窗
 */
import React, { useState, useCallback } from 'react';
import {
  Table, Card, Button, Space, Typography, Tag, Modal, Form, Input, Select, Switch,
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined, UserOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

/** 用户记录数据结构 */
interface UserRecord {
  key: string;              // 记录唯一标识
  username: string;         // 用户名（登录账号）
  displayName: string;      // 显示名称
  email: string;            // 邮箱地址
  role: string;             // 角色（admin/operator/developer/auditor/viewer/manager）
  department: string;       // 所属部门
  enabled: boolean;         // 是否启用
  lastLogin?: string;       // 最后登录时间（可选）
  source: 'local' | 'ldap'; // 用户来源（本地/LDAP）
}

/** 角色对应的颜色映射 */
const ROLE_COLORS: Record<string, string> = {
  admin: '#F53F3F',       // 管理员 - 红色
  operator: '#2E75B6',    // 运维人员 - 蓝色
  developer: '#00B42A',   // 开发人员 - 绿色
  auditor: '#722ED1',     // 审计员 - 紫色
  viewer: '#86909C',      // 只读用户 - 灰色
  manager: '#FF7D00',     // 管理者 - 橙色
};

/**
 * 用户管理组件
 * - 顶部：页面标题 + 添加用户按钮
 * - 过滤栏：角色过滤、用户来源过滤、关键词搜索
 * - 用户表格：用户名、显示名、邮箱、角色、部门、来源、启用状态、最后登录、操作
 * - 创建/编辑弹窗：用户名、显示名、邮箱、角色、部门
 */
const UserManagement: React.FC = () => {
  const { t } = useTranslation('settings');
  const [loading] = useState(false);                                  // 表格加载状态
  const [modalOpen, setModalOpen] = useState(false);                  // 弹窗是否打开
  const [editingUser, setEditingUser] = useState<UserRecord | null>(null); // 正在编辑的用户
  const [form] = Form.useForm();                                      // 表单实例

  /**
   * 打开新增用户弹窗
   * 重置表单和编辑状态
   */
  const handleAdd = useCallback(() => {
    setEditingUser(null);
    form.resetFields();
    setModalOpen(true);
  }, [form]);

  /**
   * 打开编辑用户弹窗
   * 将当前用户数据填充到表单
   * @param record 用户记录
   */
  const handleEdit = useCallback((record: UserRecord) => {
    setEditingUser(record);
    form.setFieldsValue(record);
    setModalOpen(true);
  }, [form]);

  /**
   * 保存用户（新增或更新）
   * 校验表单后调用 API（当前为占位实现）
   */
  const handleSave = useCallback(() => {
    form.validateFields().then(() => {
      setModalOpen(false);
      // TODO: 对接用户管理 API
    });
  }, [form]);

  /** 表格列定义 */
  const columns = [
    {
      title: t('user.column.username'),
      dataIndex: 'username',
      key: 'username',
      /** 渲染用户名，带用户图标 */
      render: (name: string) => (
        <Space>
          <UserOutlined />
          <span>{name}</span>
        </Space>
      ),
    },
    {
      title: t('user.column.displayName'),
      dataIndex: 'displayName',
      key: 'displayName',
      width: 120,
    },
    {
      title: t('user.column.email'),
      dataIndex: 'email',
      key: 'email',
      width: 200,
      ellipsis: true,
    },
    {
      title: t('user.column.role'),
      dataIndex: 'role',
      key: 'role',
      width: 120,
      /** 渲染角色标签，使用对应颜色 */
      render: (role: string) => (
        <Tag color={ROLE_COLORS[role] || '#86909C'}>{t(`user.role.${role}`)}</Tag>
      ),
    },
    {
      title: t('user.column.department'),
      dataIndex: 'department',
      key: 'department',
      width: 120,
    },
    {
      title: t('user.column.source'),
      dataIndex: 'source',
      key: 'source',
      width: 80,
      /** 渲染用户来源标签（本地/LDAP） */
      render: (source: string) => (
        <Tag>{t(`user.source.${source}`)}</Tag>
      ),
    },
    {
      title: t('user.column.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      /** 渲染启用/禁用开关 */
      render: (enabled: boolean) => <Switch checked={enabled} size="small" />,
    },
    {
      title: t('user.column.lastLogin'),
      dataIndex: 'lastLogin',
      key: 'lastLogin',
      width: 160,
      render: (val: string | undefined) => val || <Text type="secondary">--</Text>,
    },
    {
      title: t('user.column.actions'),
      key: 'actions',
      width: 120,
      /** 渲染操作按钮：编辑、删除 */
      render: (_: unknown, record: UserRecord) => (
        <Space>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            {t('user.action.edit')}
          </Button>
          <Button type="link" size="small" danger icon={<DeleteOutlined />}>
            {t('user.action.delete')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与添加用户按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('user.title')}</Text>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
          {t('user.addUser')}
        </Button>
      </div>

      {/* 过滤条件栏 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Space wrap>
          {/* 角色过滤 */}
          <Select placeholder={t('user.filter.role')} style={{ width: 140 }} allowClear
            options={['admin', 'operator', 'developer', 'auditor', 'viewer', 'manager'].map((r) => ({
              value: r,
              label: t(`user.role.${r}`),
            }))}
          />
          {/* 用户来源过滤 */}
          <Select placeholder={t('user.filter.source')} style={{ width: 120 }} allowClear
            options={['local', 'ldap'].map((s) => ({
              value: s,
              label: t(`user.source.${s}`),
            }))}
          />
          {/* 关键词搜索 */}
          <Input.Search
            placeholder={t('user.filter.search')}
            style={{ width: 200 }}
            allowClear
          />
        </Space>
      </Card>

      {/* 用户列表表格 */}
      <Table<UserRecord>
        columns={columns}
        dataSource={[]}
        loading={loading}
        locale={{ emptyText: t('user.noData') }}
        rowKey="key"
        size="middle"
        pagination={{ pageSize: 20, showTotal: (total) => t('user.total', { count: total }) }}
      />

      {/* 创建/编辑用户弹窗 */}
      <Modal
        title={editingUser ? t('user.editTitle') : t('user.addTitle')}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        okText={t('user.save')}
        cancelText={t('user.cancel')}
        width={560}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* 用户名（编辑模式下不可修改） */}
          <Form.Item name="username" label={t('user.form.username')}
            rules={[{ required: true, message: t('user.form.usernameRequired') }]}
          >
            <Input placeholder={t('user.form.usernamePlaceholder')} disabled={!!editingUser} />
          </Form.Item>
          {/* 显示名称 */}
          <Form.Item name="displayName" label={t('user.form.displayName')}
            rules={[{ required: true, message: t('user.form.displayNameRequired') }]}
          >
            <Input placeholder={t('user.form.displayNamePlaceholder')} />
          </Form.Item>
          {/* 邮箱（含邮箱格式校验） */}
          <Form.Item name="email" label={t('user.form.email')}
            rules={[{ required: true, type: 'email', message: t('user.form.emailRequired') }]}
          >
            <Input placeholder={t('user.form.emailPlaceholder')} />
          </Form.Item>
          {/* 角色选择 */}
          <Form.Item name="role" label={t('user.form.role')}
            rules={[{ required: true, message: t('user.form.roleRequired') }]}
          >
            <Select
              placeholder={t('user.form.rolePlaceholder')}
              options={['admin', 'operator', 'developer', 'auditor', 'viewer', 'manager'].map((r) => ({
                value: r,
                label: t(`user.role.${r}`),
              }))}
            />
          </Form.Item>
          {/* 所属部门 */}
          <Form.Item name="department" label={t('user.form.department')}>
            <Input placeholder={t('user.form.departmentPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default UserManagement;
