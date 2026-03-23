/**
 * AI 模型管理页面 - 展示已注册的 AI 模型列表、场景绑定关系、用量监控
 * 包含：模型列表表格、场景绑定表格、用量图表占位
 * 已接入真实 API，支持加载状态、空状态引导、模型注册弹窗
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Table, Button, Tag, Badge, Typography, Space, Skeleton, Modal, Form,
  Input, Select, InputNumber, message, Empty, Popconfirm,
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined, ApiOutlined, SearchOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  listModels, createModel, updateModel, deleteModel, testModel,
  listScenarioBindings, getUsageStats,
  discoverLocalModels,
} from '../api/aiModel';
import type {
  AIModel, AIModelPayload, ScenarioBinding, UsageStats, DiscoveredModel,
  ModelDeployType, ModelProvider,
} from '../api/aiModel';

const { Text } = Typography;

/**
 * AI 模型管理组件
 * - 模型列表：展示已注册模型的名称、供应商、状态、今日调用次数、Token 用量
 * - 场景绑定：展示各 AI 场景（根因分析、告警降噪等）与模型的绑定关系
 * - 用量监控：图表展示区（占位）
 */
const Settings: React.FC = () => {
  const { t } = useTranslation('settings');

  // ==================== 状态管理 ====================
  /** 模型列表数据 */
  const [models, setModels] = useState<AIModel[]>([]);
  /** 场景绑定数据 */
  const [scenarios, setScenarios] = useState<ScenarioBinding[]>([]);
  /** 用量统计数据 */
  const [usageStats, setUsageStats] = useState<UsageStats | null>(null);
  /** 页面加载状态 */
  const [loading, setLoading] = useState(true);
  /** 注册/编辑模型弹窗是否打开 */
  const [modalOpen, setModalOpen] = useState(false);
  /** 当前正在编辑的模型（null 表示新增） */
  const [editingModel, setEditingModel] = useState<AIModel | null>(null);
  /** 模型表单实例 */
  const [form] = Form.useForm();
  /** 弹窗提交中状态 */
  const [submitting, setSubmitting] = useState(false);
  /** 当前选择的部署类型，用于控制表单字段显示 */
  const [deployType, setDeployType] = useState<ModelDeployType>('cloud');
  /** 本地模型探测结果 */
  const [discoveredModels, setDiscoveredModels] = useState<DiscoveredModel[]>([]);
  /** 本地模型探测中状态 */
  const [discovering, setDiscovering] = useState(false);

  /**
   * 加载页面数据：模型列表、场景绑定、用量统计
   * 页面初始化和数据变更后调用
   */
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [modelList, scenarioList, stats] = await Promise.all([
        listModels(),
        listScenarioBindings(),
        getUsageStats(),
      ]);
      setModels(modelList);
      setScenarios(scenarioList);
      setUsageStats(stats);
    } catch (err) {
      message.error(t('ai.loadError'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  /** 页面首次加载时获取数据 */
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  /**
   * 打开注册模型弹窗
   * 重置表单和编辑状态
   */
  const handleAdd = useCallback(() => {
    setEditingModel(null);
    setDeployType('cloud');
    setDiscoveredModels([]);
    form.resetFields();
    form.setFieldsValue({ deployType: 'cloud', temperature: 0.7, maxTokens: 4096, timeout: 30 });
    setModalOpen(true);
  }, [form]);

  /**
   * 打开编辑模型弹窗
   * 将当前模型数据填充到表单
   */
  const handleEdit = useCallback((record: AIModel) => {
    setEditingModel(record);
    setDeployType(record.deployType);
    setDiscoveredModels([]);
    form.setFieldsValue(record);
    setModalOpen(true);
  }, [form]);

  /**
   * 提交注册/更新模型表单
   * 校验表单后调用 API 保存
   */
  const handleSubmit = useCallback(async () => {
    try {
      const values = await form.validateFields();
      setSubmitting(true);
      const payload: AIModelPayload = {
        name: values.name,
        provider: values.provider,
        deployType: values.deployType,
        modelId: values.modelId,
        baseURL: values.baseURL,
        apiKey: values.apiKey,
        maxTokens: values.maxTokens,
        temperature: values.temperature,
        timeout: values.timeout,
      };
      if (editingModel) {
        await updateModel(editingModel.id, payload);
        message.success(t('ai.updateSuccess'));
      } else {
        await createModel(payload);
        message.success(t('ai.createSuccess'));
      }
      setModalOpen(false);
      fetchData();
    } catch (err) {
      // 表单校验失败时不提示（antd 自动处理）
      if (err instanceof Error) {
        message.error(err.message);
      }
    } finally {
      setSubmitting(false);
    }
  }, [form, editingModel, fetchData, t]);

  /**
   * 删除模型
   * @param id - 待删除的模型 ID
   */
  const handleDelete = useCallback(async (id: string) => {
    try {
      await deleteModel(id);
      message.success(t('ai.deleteSuccess'));
      fetchData();
    } catch (err) {
      message.error(t('ai.deleteError'));
    }
  }, [fetchData, t]);

  /**
   * 测试模型连通性
   * @param id - 待测试的模型 ID
   */
  const handleTest = useCallback(async (id: string) => {
    try {
      const result = await testModel(id);
      if (result.success) {
        message.success(t('ai.testSuccess', { latency: result.latencyMs }));
      } else {
        message.error(t('ai.testFailed', { error: result.error }));
      }
    } catch (err) {
      message.error(t('ai.testError'));
    }
  }, [t]);

  /**
   * 探测本地模型
   * 根据表单中的 baseURL 和 provider 自动发现可用模型
   */
  const handleDiscover = useCallback(async () => {
    const baseURL = form.getFieldValue('baseURL');
    const provider = form.getFieldValue('provider');
    if (!baseURL || !provider) {
      message.warning(t('ai.discoverHint'));
      return;
    }
    setDiscovering(true);
    try {
      const result = await discoverLocalModels(baseURL, provider as 'ollama' | 'vllm');
      setDiscoveredModels(result);
      if (result.length === 0) {
        message.info(t('ai.noLocalModels'));
      }
    } catch (err) {
      message.error(t('ai.discoverError'));
    } finally {
      setDiscovering(false);
    }
  }, [form, t]);

  /** 模型列表表格列定义 */
  const modelColumns = [
    { title: t('ai.column.name'), dataIndex: 'name', key: 'name' },
    { title: t('ai.column.provider'), dataIndex: 'provider', key: 'provider' },
    {
      title: t('ai.column.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      /** 渲染模型状态 Badge：active 绿色、standby 黄色、其他红色 */
      render: (status: string) => (
        <Badge
          status={status === 'active' ? 'success' : status === 'standby' ? 'warning' : 'error'}
          text={t(`ai.status.${status}`)}
        />
      ),
    },
    { title: t('ai.column.todayCalls'), dataIndex: 'todayCalls', key: 'todayCalls', width: 100 },
    { title: t('ai.column.todayTokens'), dataIndex: 'todayTokens', key: 'todayTokens', width: 120 },
    {
      title: t('ai.column.actions'),
      key: 'actions',
      width: 200,
      /** 渲染操作按钮：编辑、测试、删除 */
      render: (_: unknown, record: AIModel) => (
        <Space>
          <Button type="link" icon={<EditOutlined />} size="small" onClick={() => handleEdit(record)}>
            {t('ai.edit')}
          </Button>
          <Button type="link" icon={<ApiOutlined />} size="small" onClick={() => handleTest(record.id)}>
            {t('ai.test')}
          </Button>
          <Popconfirm title={t('ai.deleteConfirm')} onConfirm={() => handleDelete(record.id)}>
            <Button type="link" danger icon={<DeleteOutlined />} size="small">
              {t('ai.delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  /** 场景绑定表格列定义 */
  const scenarioColumns = [
    { title: t('ai.scenario.name'), dataIndex: 'scenarioName', key: 'scenarioName' },
    { title: t('ai.scenario.primaryModel'), dataIndex: 'primaryModel', key: 'primaryModel' },
    { title: t('ai.scenario.backupModel'), dataIndex: 'backupModel', key: 'backupModel' },
    { title: t('ai.scenario.promptVersion'), dataIndex: 'promptVersion', key: 'promptVersion', width: 120 },
    {
      title: t('ai.scenario.approvalRate'),
      dataIndex: 'approvalRate',
      key: 'approvalRate',
      width: 100,
      /** 渲染采纳率，>=80% 绿色，<80% 橙色 */
      render: (rate: number) => <span style={{ color: rate >= 80 ? '#00B42A' : '#FF7D00' }}>{rate}%</span>,
    },
  ];

  /** 供应商选项，按部署类型过滤 */
  const providerOptions: Record<ModelDeployType, Array<{ value: ModelProvider; label: string }>> = {
    cloud: [
      { value: 'openai', label: 'OpenAI' },
      { value: 'anthropic', label: 'Anthropic' },
      { value: 'qwen', label: '通义千问' },
      { value: 'deepseek', label: 'DeepSeek' },
      { value: 'azure_openai', label: 'Azure OpenAI' },
      { value: 'custom', label: '自定义' },
    ],
    local: [
      { value: 'ollama', label: 'Ollama' },
      { value: 'vllm', label: 'vLLM' },
    ],
  };

  return (
    <div>
      {/* 页面标题与注册模型按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('ai.title')}</Text>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>{t('ai.registerModel')}</Button>
      </div>

      {/* 模型列表 */}
      <Card title={t('ai.modelList')} style={{ borderRadius: 8, marginBottom: 16 }}>
        {loading ? (
          /* 加载中显示骨架屏 */
          <Skeleton active paragraph={{ rows: 4 }} />
        ) : models.length === 0 ? (
          /* 空状态引导提示 */
          <Empty description={t('ai.emptyGuide')}>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
              {t('ai.addFirstModel')}
            </Button>
          </Empty>
        ) : (
          <Table
            columns={modelColumns}
            dataSource={models}
            rowKey="id"
            pagination={false}
            size="middle"
          />
        )}
      </Card>

      {/* 场景绑定 */}
      <Card title={t('ai.scenarioBinding')} style={{ borderRadius: 8, marginBottom: 16 }}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 3 }} />
        ) : (
          <Table
            columns={scenarioColumns}
            dataSource={scenarios}
            rowKey="scenario"
            locale={{ emptyText: t('ai.noScenarios') }}
            pagination={false}
            size="middle"
          />
        )}
      </Card>

      {/* 用量监控图表占位 */}
      <Card title={t('ai.usageMonitor')} style={{ borderRadius: 8, minHeight: 200 }}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 3 }} />
        ) : usageStats ? (
          /* 用量概览：今日/本月统计 */
          <div style={{ display: 'flex', gap: 48, padding: '16px 0' }}>
            <div>
              <Text type="secondary">{t('ai.usage.todayCalls')}</Text>
              <div style={{ fontSize: 24, fontWeight: 600 }}>{usageStats.todayCalls}</div>
            </div>
            <div>
              <Text type="secondary">{t('ai.usage.todayTokens')}</Text>
              <div style={{ fontSize: 24, fontWeight: 600 }}>{usageStats.todayTokens.toLocaleString()}</div>
            </div>
            <div>
              <Text type="secondary">{t('ai.usage.monthCalls')}</Text>
              <div style={{ fontSize: 24, fontWeight: 600 }}>{usageStats.monthCalls}</div>
            </div>
            <div>
              <Text type="secondary">{t('ai.usage.monthTokens')}</Text>
              <div style={{ fontSize: 24, fontWeight: 600 }}>{usageStats.monthTokens.toLocaleString()}</div>
            </div>
          </div>
        ) : (
          <div style={{ textAlign: 'center', color: '#86909C', padding: 48 }}>
            {t('ai.usagePlaceholder')}
          </div>
        )}
      </Card>

      {/* 注册/编辑模型弹窗 */}
      <Modal
        title={editingModel ? t('ai.editModelTitle') : t('ai.registerModelTitle')}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSubmit}
        confirmLoading={submitting}
        okText={editingModel ? t('ai.save') : t('ai.register')}
        cancelText={t('ai.cancel')}
        width={600}
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* 部署类型选择：云端/本地 */}
          <Form.Item name="deployType" label={t('ai.form.deployType')} rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'cloud', label: t('ai.form.cloud') },
                { value: 'local', label: t('ai.form.local') },
              ]}
              onChange={(val: ModelDeployType) => {
                setDeployType(val);
                setDiscoveredModels([]);
                form.setFieldsValue({ provider: undefined, modelId: undefined });
              }}
            />
          </Form.Item>

          {/* 模型名称 */}
          <Form.Item name="name" label={t('ai.form.name')} rules={[{ required: true, message: t('ai.form.nameRequired') }]}>
            <Input placeholder={t('ai.form.namePlaceholder')} />
          </Form.Item>

          {/* 供应商选择 */}
          <Form.Item name="provider" label={t('ai.form.provider')} rules={[{ required: true, message: t('ai.form.providerRequired') }]}>
            <Select options={providerOptions[deployType]} placeholder={t('ai.form.providerPlaceholder')} />
          </Form.Item>

          {/* API 端点地址 */}
          <Form.Item name="baseURL" label={t('ai.form.baseURL')} rules={[{ required: true, message: t('ai.form.baseURLRequired') }]}>
            <Input placeholder={deployType === 'local' ? 'http://localhost:11434' : 'https://api.openai.com/v1'} />
          </Form.Item>

          {/* 本地模型探测按钮 */}
          {deployType === 'local' && (
            <Form.Item>
              <Button icon={<SearchOutlined />} loading={discovering} onClick={handleDiscover}>
                {t('ai.form.discoverModels')}
              </Button>
            </Form.Item>
          )}

          {/* 模型标识符 */}
          <Form.Item name="modelId" label={t('ai.form.modelId')} rules={[{ required: true, message: t('ai.form.modelIdRequired') }]}>
            {discoveredModels.length > 0 ? (
              /* 如果探测到本地模型，使用下拉选择 */
              <Select
                placeholder={t('ai.form.selectDiscovered')}
                options={discoveredModels.map((m) => ({
                  value: m.modelId,
                  label: `${m.name}${m.size ? ` (${m.size})` : ''}`,
                }))}
              />
            ) : (
              <Input placeholder={t('ai.form.modelIdPlaceholder')} />
            )}
          </Form.Item>

          {/* API 密钥（仅云端模型显示） */}
          {deployType === 'cloud' && (
            <Form.Item name="apiKey" label={t('ai.form.apiKey')}>
              <Input.Password placeholder={t('ai.form.apiKeyPlaceholder')} />
            </Form.Item>
          )}

          {/* 推理参数 */}
          <Form.Item name="temperature" label={t('ai.form.temperature')}>
            <InputNumber min={0} max={2} step={0.1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="maxTokens" label={t('ai.form.maxTokens')}>
            <InputNumber min={100} max={100000} step={100} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="timeout" label={t('ai.form.timeout')}>
            <InputNumber min={5} max={300} addonAfter="s" style={{ width: 200 }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Settings;
