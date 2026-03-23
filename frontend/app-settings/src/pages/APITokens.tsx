/**
 * API Token 管理页面
 * 展示系统中所有 API Token 的列表，支持生成新 Token、吊销已有 Token
 *
 * 核心交互逻辑：
 * - 表格展示 Token 列表（名称、权限范围、过期时间、最后使用、调用次数、状态）
 * - "生成新 Token" 按钮打开 Modal 表单，填写名称、权限范围（多选）、过期时间
 * - 生成成功后弹出一次性明文 Token 展示窗口，附警告提示用户立即复制
 * - 吊销 Token 需通过二次确认弹窗
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Table, Button, Tag, Typography, Space, Modal, Form, Input, Select, message,
  Popconfirm, Skeleton, Alert,
} from 'antd';
import {
  PlusOutlined, CopyOutlined, DeleteOutlined, ExclamationCircleOutlined,
} from '@ant-design/icons';
import {
  listTokens, generateToken, revokeToken,
} from '../api/apiToken';
import type { APIToken, TokenScope, TokenStatus } from '../api/apiToken';

const { Text } = Typography;

/** 权限范围选项列表 */
const SCOPE_OPTIONS: Array<{ value: TokenScope; label: string }> = [
  { value: 'read:alerts', label: '读取告警' },
  { value: 'write:alerts', label: '写入告警' },
  { value: 'read:metrics', label: '读取指标' },
  { value: 'read:logs', label: '读取日志' },
  { value: 'read:config', label: '读取配置' },
  { value: 'write:config', label: '写入配置' },
  { value: 'admin', label: '管理员权限' },
];

/** 过期时间选项 */
const EXPIRE_OPTIONS = [
  { value: 30, label: '30 天' },
  { value: 90, label: '90 天' },
  { value: 365, label: '1 年' },
  { value: 0, label: '永不过期' },
];

/** Token 状态颜色映射 */
const STATUS_COLOR: Record<TokenStatus, string> = {
  active: 'green',
  expired: 'orange',
  revoked: 'red',
};

/** Token 状态文字映射 */
const STATUS_TEXT: Record<TokenStatus, string> = {
  active: '有效',
  expired: '已过期',
  revoked: '已吊销',
};

/**
 * API Token 管理组件
 * - 顶部：页面标题 + 生成新 Token 按钮
 * - 主体：Token 列表表格
 * - 弹窗：生成 Token 表单、明文 Token 展示
 */
const APITokens: React.FC = () => {
  /** Token 列表数据 */
  const [tokens, setTokens] = useState<APIToken[]>([]);
  /** 页面加载状态 */
  const [loading, setLoading] = useState(true);
  /** 生成 Token 弹窗是否打开 */
  const [modalOpen, setModalOpen] = useState(false);
  /** 表单提交中状态 */
  const [submitting, setSubmitting] = useState(false);
  /** 生成的明文 Token（一次性展示） */
  const [plainToken, setPlainToken] = useState<string | null>(null);
  /** 明文 Token 展示弹窗是否打开 */
  const [tokenModalOpen, setTokenModalOpen] = useState(false);
  /** 表单实例 */
  const [form] = Form.useForm();

  /**
   * 加载 Token 列表
   * 页面初始化和操作后刷新调用
   */
  const fetchTokens = useCallback(async () => {
    setLoading(true);
    try {
      const list = await listTokens();
      setTokens(list);
    } catch {
      message.error('加载 Token 列表失败');
    } finally {
      setLoading(false);
    }
  }, []);

  /** 页面首次加载获取数据 */
  useEffect(() => {
    fetchTokens();
  }, [fetchTokens]);

  /**
   * 打开生成 Token 弹窗
   * 重置表单为默认值
   */
  const handleOpenModal = useCallback(() => {
    form.resetFields();
    form.setFieldsValue({ expireDays: 90 });
    setModalOpen(true);
  }, [form]);

  /**
   * 提交生成 Token 表单
   * 校验表单后调用 API，成功后展示明文 Token
   */
  const handleGenerate = useCallback(async () => {
    try {
      const values = await form.validateFields();
      setSubmitting(true);
      const result = await generateToken({
        name: values.name,
        scopes: values.scopes,
        expireDays: values.expireDays,
      });
      setModalOpen(false);
      // 展示一次性明文 Token
      setPlainToken(result.plainToken);
      setTokenModalOpen(true);
      fetchTokens();
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setSubmitting(false);
    }
  }, [form, fetchTokens]);

  /**
   * 吊销 Token
   * 调用 API 吊销后刷新列表
   * @param id - 待吊销的 Token ID
   */
  const handleRevoke = useCallback(async (id: string) => {
    try {
      await revokeToken(id);
      message.success('Token 已吊销');
      fetchTokens();
    } catch {
      message.error('吊销 Token 失败');
    }
  }, [fetchTokens]);

  /**
   * 复制明文 Token 到剪贴板
   */
  const handleCopyToken = useCallback(() => {
    if (plainToken) {
      navigator.clipboard.writeText(plainToken).then(() => {
        message.success('已复制到剪贴板');
      });
    }
  }, [plainToken]);

  /** Token 列表表格列定义 */
  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 160,
    },
    {
      title: '权限范围',
      dataIndex: 'scopes',
      key: 'scopes',
      /** 渲染权限范围标签列表 */
      render: (scopes: TokenScope[]) => (
        <Space wrap size={[4, 4]}>
          {scopes.map((scope) => (
            <Tag key={scope} color="blue">{scope}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: '过期时间',
      dataIndex: 'expiresAt',
      key: 'expiresAt',
      width: 160,
      /** 渲染过期时间，null 显示为"永不过期" */
      render: (val: string | null) =>
        val ? new Date(val).toLocaleString('zh-CN') : <Tag>永不过期</Tag>,
    },
    {
      title: '最后使用',
      dataIndex: 'lastUsedAt',
      key: 'lastUsedAt',
      width: 160,
      /** 渲染最后使用时间，null 显示为"从未使用" */
      render: (val: string | null) =>
        val ? new Date(val).toLocaleString('zh-CN') : <Text type="secondary">从未使用</Text>,
    },
    {
      title: '调用次数',
      dataIndex: 'callCount',
      key: 'callCount',
      width: 100,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      /** 渲染 Token 状态标签 */
      render: (status: TokenStatus) => (
        <Tag color={STATUS_COLOR[status]}>{STATUS_TEXT[status]}</Tag>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 100,
      /** 渲染吊销按钮（仅活跃 Token 可操作） */
      render: (_: unknown, record: APIToken) => (
        record.status === 'active' ? (
          <Popconfirm
            title="确定要吊销此 Token 吗？"
            description="吊销后该 Token 将立即失效，不可恢复。"
            icon={<ExclamationCircleOutlined style={{ color: '#F53F3F' }} />}
            onConfirm={() => handleRevoke(record.id)}
            okText="确定吊销"
            cancelText="取消"
            okButtonProps={{ danger: true }}
          >
            <Button type="link" danger icon={<DeleteOutlined />} size="small">
              吊销
            </Button>
          </Popconfirm>
        ) : (
          <Text type="secondary">--</Text>
        )
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与生成按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>API Token 管理</Text>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenModal}>
          生成新 Token
        </Button>
      </div>

      {/* Token 列表 */}
      <Card style={{ borderRadius: 8 }}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 4 }} />
        ) : (
          <Table
            columns={columns}
            dataSource={tokens}
            rowKey="id"
            pagination={{ pageSize: 20 }}
            size="middle"
            locale={{ emptyText: '暂无 API Token，点击上方按钮生成' }}
          />
        )}
      </Card>

      {/* 生成 Token 表单弹窗 */}
      <Modal
        title="生成新 API Token"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleGenerate}
        confirmLoading={submitting}
        okText="生成"
        cancelText="取消"
        width={520}
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* Token 名称 */}
          <Form.Item
            name="name"
            label="Token 名称"
            rules={[{ required: true, message: '请输入 Token 名称' }]}
          >
            <Input placeholder="如：CI/CD Pipeline、监控采集器" />
          </Form.Item>

          {/* 权限范围多选 */}
          <Form.Item
            name="scopes"
            label="权限范围"
            rules={[{ required: true, message: '请选择至少一个权限范围' }]}
          >
            <Select
              mode="multiple"
              placeholder="选择 Token 可访问的权限范围"
              options={SCOPE_OPTIONS}
            />
          </Form.Item>

          {/* 过期时间选择 */}
          <Form.Item
            name="expireDays"
            label="过期时间"
            rules={[{ required: true, message: '请选择过期时间' }]}
          >
            <Select options={EXPIRE_OPTIONS} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 明文 Token 一次性展示弹窗 */}
      <Modal
        title="Token 已生成"
        open={tokenModalOpen}
        onCancel={() => { setTokenModalOpen(false); setPlainToken(null); }}
        footer={
          <Button type="primary" onClick={() => { setTokenModalOpen(false); setPlainToken(null); }}>
            我已复制，关闭
          </Button>
        }
        closable={false}
        maskClosable={false}
        width={560}
      >
        {/* 安全警告 */}
        <Alert
          type="warning"
          showIcon
          icon={<ExclamationCircleOutlined />}
          message="此 Token 只显示一次，请立即复制并妥善保存！"
          description="关闭此窗口后将无法再次查看完整 Token。如果丢失，需要重新生成。"
          style={{ marginBottom: 16 }}
        />
        {/* 明文 Token 展示区域 */}
        <div style={{
          background: '#F7F8FA',
          borderRadius: 6,
          padding: '12px 16px',
          fontFamily: 'monospace',
          fontSize: 13,
          wordBreak: 'break-all',
          position: 'relative',
        }}>
          {plainToken}
          <Button
            type="primary"
            icon={<CopyOutlined />}
            size="small"
            style={{ position: 'absolute', top: 8, right: 8 }}
            onClick={handleCopyToken}
          >
            复制
          </Button>
        </div>
      </Modal>
    </div>
  );
};

export default APITokens;
