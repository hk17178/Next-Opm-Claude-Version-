/**
 * IP 白名单管理页面
 * 管理系统的 IP 访问白名单，支持启用/禁用开关、添加/删除 IP 规则
 *
 * 核心交互逻辑：
 * - 顶部开关控制白名单启用/禁用（禁用时所有 IP 可访问，启用时仅白名单内可访问）
 * - IP 规则列表：展示 IP/CIDR、备注、类型（永久/临时）、过期时间
 * - 添加规则 Modal：输入 IP 或 CIDR 网段，可选备注和过期时间
 * - 删除规则需二次确认弹窗
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Table, Button, Typography, Tag, Space, Modal, Form, Input, Select, Switch,
  DatePicker, message, Popconfirm, Skeleton, Alert,
} from 'antd';
import {
  PlusOutlined, DeleteOutlined, ExclamationCircleOutlined, SafetyOutlined,
} from '@ant-design/icons';
import {
  getConfig, updateConfig, listRules, addRule, deleteRule,
} from '../api/ipWhitelist';
import type { IPRule, RuleType } from '../api/ipWhitelist';

const { Text } = Typography;

/**
 * IP 白名单管理组件
 * - 顶部：页面标题 + 添加规则按钮
 * - 白名单开关卡片
 * - 规则列表表格
 * - 添加规则弹窗
 */
const IPWhitelist: React.FC = () => {
  /** IP 规则列表 */
  const [rules, setRules] = useState<IPRule[]>([]);
  /** 白名单是否启用 */
  const [enabled, setEnabled] = useState(false);
  /** 页面加载状态 */
  const [loading, setLoading] = useState(true);
  /** 添加规则弹窗是否打开 */
  const [modalOpen, setModalOpen] = useState(false);
  /** 表单提交中状态 */
  const [submitting, setSubmitting] = useState(false);
  /** 当前选择的规则类型（控制过期时间字段显示） */
  const [ruleType, setRuleType] = useState<RuleType>('permanent');
  /** 开关切换中状态 */
  const [toggling, setToggling] = useState(false);
  /** 表单实例 */
  const [form] = Form.useForm();

  /**
   * 加载白名单配置和规则列表
   */
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [config, ruleList] = await Promise.all([getConfig(), listRules()]);
      setEnabled(config.enabled);
      setRules(ruleList);
    } catch {
      message.error('加载 IP 白名单数据失败');
    } finally {
      setLoading(false);
    }
  }, []);

  /** 页面首次加载 */
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  /**
   * 切换白名单启用/禁用
   * @param checked - 开关状态
   */
  const handleToggle = useCallback(async (checked: boolean) => {
    setToggling(true);
    try {
      await updateConfig(checked);
      setEnabled(checked);
      message.success(checked ? 'IP 白名单已启用' : 'IP 白名单已禁用');
    } catch {
      message.error('切换白名单状态失败');
    } finally {
      setToggling(false);
    }
  }, []);

  /**
   * 打开添加规则弹窗
   */
  const handleOpenModal = useCallback(() => {
    form.resetFields();
    form.setFieldsValue({ type: 'permanent' });
    setRuleType('permanent');
    setModalOpen(true);
  }, [form]);

  /**
   * 提交添加规则表单
   */
  const handleAddRule = useCallback(async () => {
    try {
      const values = await form.validateFields();
      setSubmitting(true);
      await addRule({
        ip: values.ip,
        remark: values.remark || '',
        type: values.type,
        expiresAt: values.type === 'temporary' && values.expiresAt
          ? values.expiresAt.toISOString()
          : undefined,
      });
      message.success('规则添加成功');
      setModalOpen(false);
      fetchData();
    } catch (err) {
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setSubmitting(false);
    }
  }, [form, fetchData]);

  /**
   * 删除 IP 规则
   * @param id - 规则 ID
   */
  const handleDelete = useCallback(async (id: string) => {
    try {
      await deleteRule(id);
      message.success('规则已删除');
      fetchData();
    } catch {
      message.error('删除规则失败');
    }
  }, [fetchData]);

  /** 规则类型标签颜色映射 */
  const TYPE_CONFIG: Record<RuleType, { color: string; text: string }> = {
    permanent: { color: 'blue', text: '永久' },
    temporary: { color: 'orange', text: '临时' },
  };

  /** 规则列表表格列定义 */
  const columns = [
    {
      title: 'IP / CIDR',
      dataIndex: 'ip',
      key: 'ip',
      width: 200,
      /** 渲染 IP 地址，使用等宽字体 */
      render: (ip: string) => (
        <Text code>{ip}</Text>
      ),
    },
    {
      title: '备注',
      dataIndex: 'remark',
      key: 'remark',
      /** 无备注时显示占位符 */
      render: (remark: string) => remark || <Text type="secondary">--</Text>,
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 80,
      /** 渲染规则类型标签 */
      render: (type: RuleType) => (
        <Tag color={TYPE_CONFIG[type].color}>{TYPE_CONFIG[type].text}</Tag>
      ),
    },
    {
      title: '过期时间',
      dataIndex: 'expiresAt',
      key: 'expiresAt',
      width: 170,
      /** 渲染过期时间（永久规则显示为 "--"） */
      render: (val: string | null) =>
        val ? new Date(val).toLocaleString('zh-CN') : <Text type="secondary">--</Text>,
    },
    {
      title: '添加时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 170,
      render: (val: string) => new Date(val).toLocaleString('zh-CN'),
    },
    {
      title: '操作',
      key: 'actions',
      width: 80,
      /** 渲染删除按钮（带二次确认） */
      render: (_: unknown, record: IPRule) => (
        <Popconfirm
          title="确定要删除此规则吗？"
          description="删除后该 IP 将无法访问系统（白名单启用时）。"
          icon={<ExclamationCircleOutlined style={{ color: '#F53F3F' }} />}
          onConfirm={() => handleDelete(record.id)}
          okText="确定删除"
          cancelText="取消"
          okButtonProps={{ danger: true }}
        >
          <Button type="link" danger icon={<DeleteOutlined />} size="small">
            删除
          </Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与添加按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>IP 白名单管理</Text>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenModal}>
          添加规则
        </Button>
      </div>

      {/* 白名单开关卡片 */}
      <Card style={{ borderRadius: 8, marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <SafetyOutlined style={{ fontSize: 32, color: enabled ? '#00B42A' : '#86909C' }} />
          <div style={{ flex: 1 }}>
            <div style={{ marginBottom: 4 }}>
              <Text strong>IP 白名单</Text>
            </div>
            <Text type="secondary">
              {enabled
                ? '白名单已启用，仅允许白名单内的 IP 地址访问系统'
                : '白名单已禁用，所有 IP 地址均可访问系统'}
            </Text>
          </div>
          <Switch
            checked={enabled}
            loading={toggling}
            onChange={handleToggle}
          />
        </div>
      </Card>

      {/* 规则列表 */}
      <Card style={{ borderRadius: 8 }}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 4 }} />
        ) : (
          <Table
            columns={columns}
            dataSource={rules}
            rowKey="id"
            pagination={{ pageSize: 20 }}
            size="middle"
            locale={{ emptyText: '暂无 IP 白名单规则' }}
          />
        )}
      </Card>

      {/* 添加规则弹窗 */}
      <Modal
        title="添加 IP 白名单规则"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleAddRule}
        confirmLoading={submitting}
        okText="添加"
        cancelText="取消"
        width={480}
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* IP 地址或 CIDR */}
          <Form.Item
            name="ip"
            label="IP 地址或 CIDR 网段"
            rules={[
              { required: true, message: '请输入 IP 地址或 CIDR 网段' },
              {
                pattern: /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/,
                message: '请输入有效的 IPv4 地址或 CIDR 格式（如 192.168.1.0/24）',
              },
            ]}
          >
            <Input placeholder="如 192.168.1.100 或 10.0.0.0/8" />
          </Form.Item>

          {/* 备注 */}
          <Form.Item name="remark" label="备注说明">
            <Input placeholder="如：办公网络、VPN 出口" />
          </Form.Item>

          {/* 规则类型 */}
          <Form.Item
            name="type"
            label="规则类型"
            rules={[{ required: true }]}
          >
            <Select
              options={[
                { value: 'permanent', label: '永久生效' },
                { value: 'temporary', label: '临时生效' },
              ]}
              onChange={(val: RuleType) => setRuleType(val)}
            />
          </Form.Item>

          {/* 过期时间（仅临时规则显示） */}
          {ruleType === 'temporary' && (
            <Form.Item
              name="expiresAt"
              label="过期时间"
              rules={[{ required: true, message: '请选择过期时间' }]}
            >
              <DatePicker
                showTime
                style={{ width: '100%' }}
                placeholder="选择过期时间"
              />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </div>
  );
};

export default IPWhitelist;
