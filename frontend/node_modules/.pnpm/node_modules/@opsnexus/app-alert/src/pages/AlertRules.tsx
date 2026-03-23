import React, { useState, useCallback, useEffect } from 'react';
import {
  Table, Button, Space, Typography, Switch, Tag, Modal, Form, Input, Select, message,
} from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  fetchAlertRules, toggleAlertRule, createAlertRule,
  type AlertRule, type AlertRuleCreateParams,
} from '../api/alert';

const { Text } = Typography;

/** 告警严重程度颜色映射 */
const SEVERITY_COLORS: Record<string, string> = {
  P0: '#F53F3F',
  P1: '#FF7D00',
  P2: '#3491FA',
  P3: '#86909C',
  P4: '#C9CDD4',
};

/** 告警层级对应的 i18n 键映射（Layer 0 铁律 ~ Layer 5 业务逻辑） */
const LAYER_LABELS: Record<number, string> = {
  0: 'rules.layer0',
  1: 'rules.layer1',
  2: 'rules.layer2',
  3: 'rules.layer3',
  4: 'rules.layer4',
  5: 'rules.layer5',
};

/**
 * 告警规则管理页面组件
 * 功能：规则列表展示、启用/禁用切换、创建新规则
 */
const AlertRules: React.FC = () => {
  const { t } = useTranslation('alert');
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<AlertRule[]>([]);             // 规则列表数据
  const [total, setTotal] = useState(0);                          // 总条数
  const [page, setPage] = useState(1);                            // 当前页码
  const [pageSize] = useState(20);                                // 每页条数
  const [createModalOpen, setCreateModalOpen] = useState(false);  // 创建弹窗可见性
  const [createLoading, setCreateLoading] = useState(false);      // 创建操作加载状态
  const [form] = Form.useForm();

  /**
   * 获取告警规则列表数据
   * @param currentPage - 要查询的页码
   */
  const fetchData = useCallback(async (currentPage = 1) => {
    setLoading(true);
    try {
      const result = await fetchAlertRules({ page: currentPage, pageSize });
      setData(result.list);
      setTotal(result.total);
      setPage(currentPage);
    } catch {
      setData([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [pageSize]);

  useEffect(() => {
    fetchData(1);
  }, [fetchData]);

  /** 切换规则的启用/禁用状态，调用 PATCH /api/alert-rules/{id}/toggle */
  const handleToggle = useCallback(async (record: AlertRule) => {
    try {
      await toggleAlertRule(record.id);
      message.success(record.enabled ? t('rules.disabled') : t('rules.enabled'));
      fetchData(page);
    } catch {
      message.error(t('rules.toggleFailed'));
    }
  }, [fetchData, page, t]);

  /** 提交创建规则表单，调用 POST /api/alert-rules */
  const handleCreate = useCallback(async () => {
    try {
      const values = await form.validateFields();
      setCreateLoading(true);
      const params: AlertRuleCreateParams = {
        name: values.name,
        type: values.type,
        layer: values.layer,
        condition: values.condition,
        threshold: values.threshold,
        severity: values.severity,
      };
      await createAlertRule(params);
      message.success(t('rules.createSuccess'));
      setCreateModalOpen(false);
      form.resetFields();
      fetchData(1);
    } catch (err) {
      // 表单校验失败时直接返回，不弹错误提示
      if (err && typeof err === 'object' && 'errorFields' in err) return;
      message.error(t('rules.createFailed'));
    } finally {
      setCreateLoading(false);
    }
  }, [form, fetchData, t]);

  /** 表格列定义 */
  const columns = [
    {
      title: t('rules.column.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
    },
    {
      title: t('rules.column.layer'),
      dataIndex: 'layer',
      key: 'layer',
      width: 200,
      render: (layer: number) => (
        <Tag>{t(LAYER_LABELS[layer] || `Layer ${layer}`)}</Tag>
      ),
    },
    {
      title: t('rules.column.type'),
      dataIndex: 'type',
      key: 'type',
      width: 120,
    },
    {
      title: t('rules.column.severity'),
      dataIndex: 'severity',
      key: 'severity',
      width: 80,
      render: (severity: string) => (
        <Tag
          style={{
            background: SEVERITY_COLORS[severity],
            color: '#fff',
            borderRadius: 4,
            border: 'none',
            fontWeight: 600,
          }}
        >
          {severity}
        </Tag>
      ),
    },
    {
      title: t('rules.column.condition'),
      dataIndex: 'condition',
      key: 'condition',
      width: 180,
      ellipsis: true,
    },
    {
      title: t('rules.column.threshold'),
      dataIndex: 'threshold',
      key: 'threshold',
      width: 100,
    },
    {
      title: t('rules.column.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled: boolean, record: AlertRule) => (
        <Switch
          checked={enabled}
          size="small"
          onChange={() => handleToggle(record)}
        />
      ),
    },
    {
      title: t('rules.column.updatedAt'),
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 160,
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('rules.title')}</Text>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)}>
          {t('rules.create')}
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={data}
        loading={loading}
        rowKey="id"
        size="middle"
        pagination={{
          current: page,
          pageSize,
          total,
          onChange: (p) => fetchData(p),
        }}
      />

      <Modal
        title={t('rules.create')}
        open={createModalOpen}
        onCancel={() => { setCreateModalOpen(false); form.resetFields(); }}
        onOk={handleCreate}
        confirmLoading={createLoading}
        width={520}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label={t('rules.form.name')}
            rules={[{ required: true, message: t('rules.form.nameRequired') }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="type"
            label={t('rules.form.type')}
            rules={[{ required: true, message: t('rules.form.typeRequired') }]}
          >
            <Select options={[
              { value: 'threshold', label: t('rules.form.typeThreshold') },
              { value: 'anomaly', label: t('rules.form.typeAnomaly') },
              { value: 'trend', label: t('rules.form.typeTrend') },
              { value: 'business', label: t('rules.form.typeBusiness') },
            ]} />
          </Form.Item>
          <Form.Item
            name="layer"
            label={t('rules.form.layer')}
            rules={[{ required: true, message: t('rules.form.layerRequired') }]}
          >
            <Select options={[0, 1, 2, 3, 4, 5].map((l) => ({
              value: l,
              label: t(LAYER_LABELS[l]),
            }))} />
          </Form.Item>
          <Form.Item
            name="condition"
            label={t('rules.form.condition')}
            rules={[{ required: true, message: t('rules.form.conditionRequired') }]}
          >
            <Input placeholder="cpu_usage > 90" />
          </Form.Item>
          <Form.Item
            name="threshold"
            label={t('rules.form.threshold')}
            rules={[{ required: true, message: t('rules.form.thresholdRequired') }]}
          >
            <Input placeholder="90" />
          </Form.Item>
          <Form.Item
            name="severity"
            label={t('rules.form.severity')}
            rules={[{ required: true, message: t('rules.form.severityRequired') }]}
          >
            <Select options={['P0', 'P1', 'P2', 'P3', 'P4'].map((s) => ({
              value: s,
              label: s,
            }))} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default AlertRules;
