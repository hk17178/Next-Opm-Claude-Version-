/**
 * 事件列表页面 - 展示所有事件，支持按状态 Tab 切换、分页查询、创建新事件
 * 包含统计卡片行、过滤条件栏、事件表格（含列配置管理）、创建事件弹窗
 */
import React, { useState, useCallback, useEffect } from 'react';
import {
  Table, Card, Row, Col, Tabs, Tag, Typography, Button, Space, Modal, Form, Input, Select,
  message,
} from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { Empty as AntEmpty } from 'antd';
import { useNavigate } from 'react-router-dom';
import { useColumnConfig, type ColumnDef } from '@opsnexus/ui-kit';
import {
  fetchIncidents, createIncident,
  type Incident, type CreateIncidentParams,
} from '../api/incident';

const { Text } = Typography;
const { TextArea } = Input;

/** 严重级别对应的颜色映射 */
const SEVERITY_COLORS: Record<string, string> = {
  P0: '#F53F3F', P1: '#FF7D00', P2: '#3491FA', P3: '#86909C', P4: '#C9CDD4',
};

/** 根因分类对应的颜色映射 */
const ROOT_CAUSE_COLORS: Record<string, string> = {
  human_action: '#722ED1',    // 人为操作
  system_fault: '#F53F3F',    // 系统故障
  change_induced: '#FF7D00',  // 变更引起
  external_dependency: '#3491FA', // 外部依赖
  pending: '#86909C',         // 待分析
};

/**
 * 事件列表组件
 * - 顶部：页面标题 + 列配置按钮 + 创建事件按钮
 * - 统计卡片行：活跃事件数、今日新增、今日解决、平均 MTTR、月度 SLA
 * - 状态 Tab：active / processing / pending_review / closed / all
 * - 过滤栏：严重级别、状态、关键词搜索
 * - 事件表格：点击行跳转详情页，支持列配置管理
 * - 创建事件弹窗：标题、描述、严重级别、处理人
 */
const IncidentList: React.FC = () => {
  const { t } = useTranslation('incident');
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);           // 表格加载状态
  const [data, setData] = useState<Incident[]>([]);        // 事件列表数据
  const [total, setTotal] = useState(0);                   // 数据总条数
  const [activeTab, setActiveTab] = useState('active');     // 当前选中的状态 Tab
  const [pagination, setPagination] = useState({ current: 1, pageSize: 20 }); // 分页参数
  const [createModalOpen, setCreateModalOpen] = useState(false);  // 创建弹窗是否打开
  const [createLoading, setCreateLoading] = useState(false);      // 创建提交中状态
  const [form] = Form.useForm<CreateIncidentParams>();             // 创建事件表单实例

  /** 默认列定义 */
  const defaultColumns: ColumnDef<Incident>[] = [
    {
      title: t('list.column.severity'),
      dataIndex: 'severity',
      key: 'severity',
      width: 80,
      /** 渲染严重级别标签，使用对应颜色 */
      render: (severity: unknown) => (
        <Tag color={SEVERITY_COLORS[severity as string]} style={{ borderRadius: 4, fontWeight: 600, border: 'none' }}>
          {severity as string}
        </Tag>
      ),
    },
    {
      title: t('list.column.incidentId'),
      dataIndex: 'incidentId',
      key: 'incidentId',
      width: 150,
    },
    {
      title: t('list.column.title'),
      dataIndex: 'title',
      key: 'title',
      ellipsis: true,
    },
    {
      title: t('list.column.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      /** 渲染状态文本，pending_review 使用橙色高亮 */
      render: (status: unknown) => {
        const color = (status as string) === 'pending_review' ? '#FF7D00' : '#86909C';
        return <Text style={{ color }}>{t(`list.status.${status as string}`)}</Text>;
      },
    },
    {
      title: t('list.column.handler'),
      dataIndex: 'handler',
      key: 'handler',
      width: 100,
    },
    {
      title: t('list.column.mttr'),
      dataIndex: 'mttr',
      key: 'mttr',
      width: 100,
    },
    {
      title: t('list.column.rootCause'),
      dataIndex: 'rootCause',
      key: 'rootCause',
      width: 120,
      /** 渲染根因分类标签，使用对应颜色 */
      render: (category: unknown) => (
        <Tag color={ROOT_CAUSE_COLORS[category as string] || '#86909C'}>
          {t(`rootCause.${category as string}`)}
        </Tag>
      ),
    },
  ];

  /** 使用 useColumnConfig 管理列配置（支持列显示/隐藏、排序、持久化） */
  const { columns, ColumnConfigButton } = useColumnConfig<Incident>(
    'incident-list',
    defaultColumns,
  );

  /**
   * 加载事件列表数据
   * @param tab 状态 Tab 标识（可选，默认使用当前 activeTab）
   * @param page 页码（可选，默认使用当前分页）
   * @param pageSize 每页条数（可选，默认使用当前分页）
   */
  const loadData = useCallback(async (tab?: string, page?: number, pageSize?: number) => {
    setLoading(true);
    try {
      // request<T> 已自动解包 ApiResponse.data，直接获取 IncidentListResult
      const result = await fetchIncidents({
        // 当 Tab 为 "all" 时不传 status，获取所有状态的事件
        status: (tab || activeTab) === 'all' ? undefined : (tab || activeTab),
        page: page || pagination.current,
        pageSize: pageSize || pagination.pageSize,
      });
      setData(result.list || []);
      setTotal(result.total || 0);
    } catch {
      // API 尚未就绪，保持空状态
    } finally {
      setLoading(false);
    }
  }, [activeTab, pagination]);

  /** 组件挂载及依赖变化时加载数据 */
  useEffect(() => {
    loadData();
  }, [loadData]);

  /**
   * 处理状态 Tab 切换
   * 切换 Tab 时重置分页至第 1 页并重新加载数据
   * @param key Tab 标识
   */
  const handleTabChange = (key: string) => {
    setActiveTab(key);
    setPagination((prev) => ({ ...prev, current: 1 }));
    loadData(key, 1);
  };

  /**
   * 处理创建事件提交
   * 校验表单 → 调用创建 API → 成功后关闭弹窗并刷新列表
   */
  const handleCreate = useCallback(async () => {
    try {
      const values = await form.validateFields();
      setCreateLoading(true);
      await createIncident(values);
      message.success(t('list.createSuccess'));
      setCreateModalOpen(false);
      form.resetFields();
      loadData();
    } catch {
      // 表单校验失败或 API 错误
    } finally {
      setCreateLoading(false);
    }
  }, [form, loadData, t]);

  /** 统计卡片数据定义 */
  const statCards = [
    { key: 'active', label: t('list.stat.active'), value: total },
    { key: 'todayNew', label: t('list.stat.todayNew'), value: 0 },
    { key: 'todayResolved', label: t('list.stat.todayResolved'), value: 0 },
    { key: 'avgMttr', label: t('list.stat.avgMTTR'), value: '--' },
    { key: 'monthSla', label: t('list.stat.monthSLA'), value: '--' },
  ];

  /** 状态 Tab 配置项 */
  const tabItems = [
    { key: 'active', label: t('list.tab.active') },
    { key: 'processing', label: t('list.tab.processing') },
    { key: 'pending_review', label: t('list.tab.pendingReview') },
    { key: 'closed', label: t('list.tab.closed') },
    { key: 'all', label: t('list.tab.all') },
  ];

  return (
    <div>
      {/* 页面标题与操作按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('list.title')}</Text>
        <Space>
          {/* 列配置按钮 */}
          {ColumnConfigButton}
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)}>
            {t('list.createIncident')}
          </Button>
        </Space>
      </div>

      {/* 统计卡片行 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {statCards.map((card) => (
          <Col flex={1} key={card.key}>
            <Card
              bordered
              style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
              bodyStyle={{ padding: '16px 20px', textAlign: 'center' }}
            >
              <div style={{ color: '#86909C', fontSize: 14 }}>{card.label}</div>
              <div style={{ fontSize: 24, fontWeight: 600, marginTop: 4 }}>{card.value}</div>
            </Card>
          </Col>
        ))}
      </Row>

      {/* 状态 Tab 切换栏 */}
      <Tabs items={tabItems} activeKey={activeTab} onChange={handleTabChange} />

      {/* 过滤条件栏 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Space wrap>
          {/* 严重级别下拉过滤 */}
          <Select placeholder={t('list.filter.severity')} style={{ width: 120 }} allowClear
            options={['P0', 'P1', 'P2', 'P3', 'P4'].map((s) => ({ value: s, label: s }))}
          />
          {/* 状态下拉过滤 */}
          <Select placeholder={t('list.filter.status')} style={{ width: 140 }} allowClear
            options={['processing', 'pending_review', 'closed'].map((s) => ({
              value: s, label: t(`list.status.${s}`),
            }))}
          />
          {/* 关键词搜索 */}
          <Input.Search placeholder={t('list.filter.search')} style={{ width: 200 }} allowClear />
        </Space>
      </Card>

      {/* 事件列表表格（使用 useColumnConfig 管理的列） */}
      <Table
        columns={columns}
        dataSource={data}
        loading={loading}
        locale={{
          emptyText: (
            <AntEmpty
              image={AntEmpty.PRESENTED_IMAGE_SIMPLE}
              description="暂无活跃事件"
            />
          ),
        }}
        rowKey="incidentId"
        onRow={(record) => ({
          style: { cursor: 'pointer' },
          // 点击行跳转至事件详情页
          onClick: () => navigate(`/detail/${record.id || record.incidentId}`),
        })}
        size="middle"
        pagination={{
          current: pagination.current,
          pageSize: pagination.pageSize,
          total,
          showSizeChanger: true,
          showQuickJumper: true,
          onChange: (page, pageSize) => {
            setPagination({ current: page, pageSize });
            loadData(undefined, page, pageSize);
          },
        }}
      />

      {/* 创建事件弹窗 */}
      <Modal
        title={t('list.createTitle')}
        open={createModalOpen}
        onCancel={() => { setCreateModalOpen(false); form.resetFields(); }}
        onOk={handleCreate}
        confirmLoading={createLoading}
        okText={t('list.createSubmit')}
        cancelText={t('list.createCancel')}
        width={560}
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* 事件标题 */}
          <Form.Item
            name="title"
            label={t('list.form.title')}
            rules={[{ required: true, message: t('list.form.titleRequired') }]}
          >
            <Input placeholder={t('list.form.titlePlaceholder')} />
          </Form.Item>
          {/* 事件描述 */}
          <Form.Item
            name="description"
            label={t('list.form.description')}
            rules={[{ required: true, message: t('list.form.descriptionRequired') }]}
          >
            <TextArea rows={3} placeholder={t('list.form.descriptionPlaceholder')} />
          </Form.Item>
          {/* 严重级别选择 */}
          <Form.Item
            name="severity"
            label={t('list.form.severity')}
            rules={[{ required: true, message: t('list.form.severityRequired') }]}
          >
            <Select
              placeholder={t('list.form.severityPlaceholder')}
              options={['P0', 'P1', 'P2', 'P3', 'P4'].map((s) => ({ value: s, label: s }))}
            />
          </Form.Item>
          {/* 处理人 */}
          <Form.Item
            name="handler"
            label={t('list.form.handler')}
            rules={[{ required: true, message: t('list.form.handlerRequired') }]}
          >
            <Input placeholder={t('list.form.handlerPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default IncidentList;
