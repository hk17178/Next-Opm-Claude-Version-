import React, { useState, useCallback } from 'react';
import {
  Card, Row, Col, Table, Button, Space, Typography, Tag, Tabs, Switch,
  Modal, Form, Input, Select, InputNumber, TimePicker, message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  PlusOutlined, ReloadOutlined, ArrowDownOutlined,
  FilterOutlined, ThunderboltOutlined, ClockCircleOutlined, RobotOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { NoiseFunnel } from '@opsnexus/ui-kit';

const { Text } = Typography;

/* ================================================================== */
/*  类型定义                                                           */
/* ================================================================== */

/** 降噪规则类型 */
type NoiseRuleType = 'maintenance' | 'convergence' | 'baseline_filter' | 'ai_suppression';

/** 降噪规则状态 */
type NoiseRuleStatus = 'enabled' | 'disabled';

/** 降噪规则记录 */
interface NoiseRuleRecord {
  key: string;
  /** 规则名称 */
  ruleName: string;
  /** 规则类型 */
  ruleType: NoiseRuleType;
  /** 匹配数量 */
  matchCount: number;
  /** 抑制数量 */
  suppressCount: number;
  /** 规则状态 */
  status: NoiseRuleStatus;
  /** 创建时间 */
  createdAt: string;
  /** 更新时间 */
  updatedAt: string;
  /** 描述 */
  description: string;
}

/* ================================================================== */
/*  Mock 数据                                                          */
/* ================================================================== */

/** 降噪效果概览统计 */
const NOISE_OVERVIEW = {
  /** 原始告警数 */
  rawCount: 1247,
  /** 过滤后数量 */
  filteredCount: 342,
  /** 抑制率 */
  suppressionRate: '72.6%',
  /** 误报率 */
  falsePositiveRate: '3.2%',
};

/** 降噪漏斗层级数据 */
const NOISE_FUNNEL_LAYERS = [
  { label: '原始告警', value: 1247 },
  { label: '维护窗口', value: 980 },
  { label: '收敛合并', value: 620 },
  { label: '基线过滤', value: 420 },
  { label: 'AI 抑制', value: 342 },
];

/** 降噪规则列表 Mock 数据 */
const NOISE_RULES: NoiseRuleRecord[] = [
  // 维护窗口类型规则
  { key: '1', ruleName: '每周二凌晨发布窗口', ruleType: 'maintenance', matchCount: 85, suppressCount: 78, status: 'enabled', createdAt: '2026-03-01', updatedAt: '2026-03-24', description: '每周二 02:00-04:00 发布维护窗口' },
  { key: '2', ruleName: '月度巡检窗口', ruleType: 'maintenance', matchCount: 42, suppressCount: 40, status: 'enabled', createdAt: '2026-02-15', updatedAt: '2026-03-20', description: '每月第一个周六 00:00-06:00' },
  // 收敛合并类型规则
  { key: '3', ruleName: '同源告警收敛（5min窗口）', ruleType: 'convergence', matchCount: 320, suppressCount: 245, status: 'enabled', createdAt: '2026-01-10', updatedAt: '2026-03-24', description: '同一来源 5 分钟内相同告警合并' },
  { key: '4', ruleName: '关联告警合并', ruleType: 'convergence', matchCount: 180, suppressCount: 115, status: 'enabled', createdAt: '2026-02-01', updatedAt: '2026-03-22', description: '上下游服务关联告警自动合并' },
  // 基线过滤类型规则
  { key: '5', ruleName: 'CPU 基线动态阈值', ruleType: 'baseline_filter', matchCount: 150, suppressCount: 95, status: 'enabled', createdAt: '2026-01-20', updatedAt: '2026-03-24', description: '基于 14 天基线动态调整阈值' },
  { key: '6', ruleName: '内存使用率基线', ruleType: 'baseline_filter', matchCount: 88, suppressCount: 52, status: 'enabled', createdAt: '2026-02-05', updatedAt: '2026-03-23', description: '内存使用率智能基线过滤' },
  { key: '7', ruleName: '磁盘 IO 基线', ruleType: 'baseline_filter', matchCount: 65, suppressCount: 38, status: 'disabled', createdAt: '2026-03-01', updatedAt: '2026-03-18', description: '磁盘 IO 延迟动态基线（训练中）' },
  // AI 抑制类型规则
  { key: '8', ruleName: 'AI 噪声识别 v2', ruleType: 'ai_suppression', matchCount: 200, suppressCount: 165, status: 'enabled', createdAt: '2026-02-10', updatedAt: '2026-03-24', description: '基于 ML 模型自动识别噪声告警' },
  { key: '9', ruleName: 'AI 关联性分析', ruleType: 'ai_suppression', matchCount: 120, suppressCount: 78, status: 'enabled', createdAt: '2026-03-05', updatedAt: '2026-03-24', description: 'AI 分析告警关联性，抑制冗余告警' },
  { key: '10', ruleName: 'AI 误报检测', ruleType: 'ai_suppression', matchCount: 95, suppressCount: 62, status: 'disabled', createdAt: '2026-03-15', updatedAt: '2026-03-20', description: '基于历史数据训练的误报检测模型' },
];

/* ================================================================== */
/*  常量定义                                                           */
/* ================================================================== */

/** 规则类型对应的图标和颜色 */
const RULE_TYPE_CONFIG: Record<NoiseRuleType, { icon: React.ReactNode; color: string; label: string }> = {
  maintenance: { icon: <ClockCircleOutlined />, color: 'orange', label: 'noiseReduction.ruleType.maintenance' },
  convergence: { icon: <FilterOutlined />, color: 'blue', label: 'noiseReduction.ruleType.convergence' },
  baseline_filter: { icon: <ThunderboltOutlined />, color: 'purple', label: 'noiseReduction.ruleType.baselineFilter' },
  ai_suppression: { icon: <RobotOutlined />, color: 'green', label: 'noiseReduction.ruleType.aiSuppression' },
};

/* ================================================================== */
/*  组件                                                               */
/* ================================================================== */

/**
 * 告警降噪配置页面
 *
 * 功能模块：
 * 1. 降噪效果概览（4 张卡片：原始数/过滤数/抑制率/误报率）
 * 2. 降噪规则列表（规则名/类型/匹配数/抑制数/状态/操作）
 * 3. 规则类型 Tab（维护窗口/收敛合并/基线过滤/AI 抑制）
 * 4. 创建规则弹窗
 * 5. 降噪漏斗可视化（NoiseFunnel 组件）
 */
const AlertNoiseReduction: React.FC = () => {
  const { t } = useTranslation('alert');

  // ---- 状态管理 ----
  /** 当前选中的规则类型 Tab */
  const [activeTab, setActiveTab] = useState<string>('all');
  /** 降噪规则列表数据 */
  const [rules, setRules] = useState<NoiseRuleRecord[]>(NOISE_RULES);
  /** 创建规则弹窗可见性 */
  const [createModalOpen, setCreateModalOpen] = useState(false);
  /** 创建表单实例 */
  const [form] = Form.useForm();

  /** 根据 Tab 筛选规则列表数据 */
  const filteredRules = activeTab === 'all'
    ? rules
    : rules.filter((rule) => rule.ruleType === activeTab);

  /** 处理规则启用/禁用切换 */
  const handleToggleRule = useCallback((key: string, checked: boolean) => {
    setRules((prev) =>
      prev.map((rule) =>
        rule.key === key ? { ...rule, status: checked ? 'enabled' : 'disabled' } : rule
      )
    );
    message.success(checked ? t('noiseReduction.ruleEnabled') : t('noiseReduction.ruleDisabled'));
  }, [t]);

  /** 处理删除规则 */
  const handleDeleteRule = useCallback((key: string) => {
    setRules((prev) => prev.filter((rule) => rule.key !== key));
    message.success(t('noiseReduction.deleteSuccess'));
  }, [t]);

  /** 处理创建规则提交 */
  const handleCreateRule = useCallback(async () => {
    try {
      const values = await form.validateFields();
      const newRule: NoiseRuleRecord = {
        key: String(Date.now()),
        ruleName: values.ruleName,
        ruleType: values.ruleType,
        matchCount: 0,
        suppressCount: 0,
        status: 'enabled',
        createdAt: new Date().toLocaleDateString('zh-CN'),
        updatedAt: new Date().toLocaleDateString('zh-CN'),
        description: values.description || '',
      };
      setRules((prev) => [newRule, ...prev]);
      message.success(t('noiseReduction.createSuccess'));
      setCreateModalOpen(false);
      form.resetFields();
    } catch {
      // 表单校验失败，antd 自动显示错误提示
    }
  }, [form, t]);

  /* ---------- 概览统计卡片配置 ---------- */
  const overviewCards = [
    {
      key: 'rawCount',
      label: t('noiseReduction.overview.rawCount'),
      value: NOISE_OVERVIEW.rawCount,
      color: '#4da6ff',
      icon: '📊',
    },
    {
      key: 'filteredCount',
      label: t('noiseReduction.overview.filteredCount'),
      value: NOISE_OVERVIEW.filteredCount,
      color: '#00e5a0',
      icon: '✅',
    },
    {
      key: 'suppressionRate',
      label: t('noiseReduction.overview.suppressionRate'),
      value: NOISE_OVERVIEW.suppressionRate,
      color: '#ffaa33',
      icon: '🔽',
    },
    {
      key: 'falsePositiveRate',
      label: t('noiseReduction.overview.falsePositiveRate'),
      value: NOISE_OVERVIEW.falsePositiveRate,
      color: '#F53F3F',
      icon: '⚠️',
    },
  ];

  /* ---------- 规则类型 Tab 配置 ---------- */
  const tabItems = [
    { key: 'all', label: t('noiseReduction.tab.all') },
    {
      key: 'maintenance',
      label: (
        <Space size={4}>
          <ClockCircleOutlined />
          <span>{t('noiseReduction.ruleType.maintenance')}</span>
        </Space>
      ),
    },
    {
      key: 'convergence',
      label: (
        <Space size={4}>
          <FilterOutlined />
          <span>{t('noiseReduction.ruleType.convergence')}</span>
        </Space>
      ),
    },
    {
      key: 'baseline_filter',
      label: (
        <Space size={4}>
          <ThunderboltOutlined />
          <span>{t('noiseReduction.ruleType.baselineFilter')}</span>
        </Space>
      ),
    },
    {
      key: 'ai_suppression',
      label: (
        <Space size={4}>
          <RobotOutlined />
          <span>{t('noiseReduction.ruleType.aiSuppression')}</span>
        </Space>
      ),
    },
  ];

  /* ---------- 规则列表表格列定义 ---------- */
  const columns: ColumnsType<NoiseRuleRecord> = [
    {
      title: t('noiseReduction.column.ruleName'),
      dataIndex: 'ruleName',
      key: 'ruleName',
      ellipsis: true,
      render: (name: string, record: NoiseRuleRecord) => (
        <div>
          <div style={{ fontWeight: 500 }}>{name}</div>
          <div style={{ fontSize: 12, color: '#86909C', marginTop: 2 }}>{record.description}</div>
        </div>
      ),
    },
    {
      title: t('noiseReduction.column.ruleType'),
      dataIndex: 'ruleType',
      key: 'ruleType',
      width: 130,
      render: (type: NoiseRuleType) => {
        const config = RULE_TYPE_CONFIG[type];
        return (
          <Tag icon={config.icon} color={config.color}>
            {t(config.label)}
          </Tag>
        );
      },
    },
    {
      title: t('noiseReduction.column.matchCount'),
      dataIndex: 'matchCount',
      key: 'matchCount',
      width: 90,
      sorter: (a, b) => a.matchCount - b.matchCount,
      render: (val: number) => <Text strong>{val}</Text>,
    },
    {
      title: t('noiseReduction.column.suppressCount'),
      dataIndex: 'suppressCount',
      key: 'suppressCount',
      width: 90,
      sorter: (a, b) => a.suppressCount - b.suppressCount,
      render: (val: number) => (
        <Text strong style={{ color: '#00e5a0' }}>
          <ArrowDownOutlined style={{ fontSize: 11, marginRight: 2 }} />
          {val}
        </Text>
      ),
    },
    {
      title: t('noiseReduction.column.status'),
      dataIndex: 'status',
      key: 'status',
      width: 80,
      render: (status: NoiseRuleStatus, record: NoiseRuleRecord) => (
        <Switch
          checked={status === 'enabled'}
          size="small"
          onChange={(checked) => handleToggleRule(record.key, checked)}
        />
      ),
    },
    {
      title: t('noiseReduction.column.updatedAt'),
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 110,
    },
    {
      title: t('noiseReduction.column.actions'),
      key: 'actions',
      width: 100,
      render: (_: unknown, record: NoiseRuleRecord) => (
        <Space size={0}>
          <Button type="link" size="small">{t('noiseReduction.action.edit')}</Button>
          <Button
            type="link"
            size="small"
            danger
            onClick={() => handleDeleteRule(record.key)}
          >
            {t('noiseReduction.action.delete')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* ---- 页面标题和操作栏 ---- */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('noiseReduction.title')}</Text>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => setRules([...NOISE_RULES])} />
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateModalOpen(true)}
          >
            {t('noiseReduction.createRule')}
          </Button>
        </Space>
      </div>

      {/* ---- 降噪效果概览（4 张统计卡片） ---- */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {overviewCards.map((card) => (
          <Col span={6} key={card.key}>
            <Card
              bordered
              style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
              bodyStyle={{ padding: '16px 20px' }}
            >
              <div style={{ color: '#86909C', fontSize: 14 }}>{card.label}</div>
              <div style={{ fontSize: 28, fontWeight: 600, marginTop: 4, color: card.color }}>
                {card.value}
              </div>
            </Card>
          </Col>
        ))}
      </Row>

      {/* ---- 降噪漏斗可视化 ---- */}
      <Card
        title={t('noiseReduction.funnelTitle')}
        bordered
        style={{ borderRadius: 8, marginBottom: 16 }}
        bodyStyle={{ padding: '16px 20px' }}
      >
        <NoiseFunnel layers={NOISE_FUNNEL_LAYERS} height={8} />
        {/* 漏斗阶段说明 */}
        <div style={{ display: 'flex', justifyContent: 'space-around', marginTop: 16 }}>
          {NOISE_FUNNEL_LAYERS.map((layer, index) => {
            /** 计算当前阶段相对上一阶段的过滤比例 */
            const prevValue = index > 0 ? NOISE_FUNNEL_LAYERS[index - 1].value : layer.value;
            const filterRate = index > 0
              ? `${((1 - layer.value / prevValue) * 100).toFixed(1)}%`
              : '-';
            return (
              <div key={index} style={{ textAlign: 'center' }}>
                <div style={{ fontSize: 16, fontWeight: 600, color: '#4da6ff' }}>{layer.value}</div>
                <div style={{ fontSize: 11, color: '#86909C' }}>{layer.label}</div>
                {index > 0 && (
                  <div style={{ fontSize: 10, color: '#00e5a0', marginTop: 2 }}>
                    <ArrowDownOutlined /> {filterRate}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </Card>

      {/* ---- 规则类型 Tab + 规则列表 ---- */}
      <Card
        bordered
        style={{ borderRadius: 8 }}
        bodyStyle={{ padding: '8px 16px' }}
      >
        <Tabs
          items={tabItems}
          activeKey={activeTab}
          onChange={setActiveTab}
        />
        <Table<NoiseRuleRecord>
          columns={columns}
          dataSource={filteredRules}
          pagination={{ pageSize: 10, size: 'small' }}
          size="small"
          rowKey="key"
        />
      </Card>

      {/* ---- 创建规则弹窗 ---- */}
      <Modal
        title={t('noiseReduction.createRule')}
        open={createModalOpen}
        onCancel={() => {
          setCreateModalOpen(false);
          form.resetFields();
        }}
        onOk={handleCreateRule}
        okText={t('noiseReduction.form.submit')}
        cancelText={t('noiseReduction.form.cancel')}
        width={560}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* 规则名称 */}
          <Form.Item
            name="ruleName"
            label={t('noiseReduction.form.ruleName')}
            rules={[{ required: true, message: t('noiseReduction.form.ruleNameRequired') }]}
          >
            <Input placeholder={t('noiseReduction.form.ruleNamePlaceholder')} />
          </Form.Item>

          {/* 规则类型 */}
          <Form.Item
            name="ruleType"
            label={t('noiseReduction.form.ruleType')}
            rules={[{ required: true, message: t('noiseReduction.form.ruleTypeRequired') }]}
          >
            <Select
              placeholder={t('noiseReduction.form.ruleTypePlaceholder')}
              options={[
                { value: 'maintenance', label: t('noiseReduction.ruleType.maintenance') },
                { value: 'convergence', label: t('noiseReduction.ruleType.convergence') },
                { value: 'baseline_filter', label: t('noiseReduction.ruleType.baselineFilter') },
                { value: 'ai_suppression', label: t('noiseReduction.ruleType.aiSuppression') },
              ]}
            />
          </Form.Item>

          {/* 描述 */}
          <Form.Item
            name="description"
            label={t('noiseReduction.form.description')}
          >
            <Input.TextArea
              rows={3}
              placeholder={t('noiseReduction.form.descriptionPlaceholder')}
            />
          </Form.Item>

          {/* 维护窗口时间设置（条件显示） */}
          <Form.Item
            noStyle
            shouldUpdate={(prevValues, currentValues) => prevValues.ruleType !== currentValues.ruleType}
          >
            {({ getFieldValue }) =>
              getFieldValue('ruleType') === 'maintenance' ? (
                <Form.Item
                  name="timeRange"
                  label={t('noiseReduction.form.timeRange')}
                >
                  <TimePicker.RangePicker format="HH:mm" style={{ width: '100%' }} />
                </Form.Item>
              ) : null
            }
          </Form.Item>

          {/* 收敛窗口大小（条件显示） */}
          <Form.Item
            noStyle
            shouldUpdate={(prevValues, currentValues) => prevValues.ruleType !== currentValues.ruleType}
          >
            {({ getFieldValue }) =>
              getFieldValue('ruleType') === 'convergence' ? (
                <Form.Item
                  name="windowSize"
                  label={t('noiseReduction.form.windowSize')}
                  initialValue={5}
                >
                  <InputNumber
                    min={1}
                    max={60}
                    addonAfter={t('noiseReduction.form.minutes')}
                    style={{ width: '100%' }}
                  />
                </Form.Item>
              ) : null
            }
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default AlertNoiseReduction;
