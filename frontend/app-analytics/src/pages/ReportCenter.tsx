/**
 * 报告中心页面（页面 17）
 * 路由: /analytics/reports
 *
 * 功能模块：
 * - Tabs 切换：管理视角 / 技术视角 / 自定义报告 / 订阅管理
 * - 管理视角 Tab：4 张报告模板卡片（运营日报、周报、月度SLA、管理驾驶舱）
 * - 技术视角 Tab：4 张报告模板卡片（告警分析、事件复盘、容量规划、变更影响）
 * - 自定义报告 Tab：拖拽选择报告模块
 * - 订阅管理 Tab：订阅列表表格（报告名/频率/接收人/格式/状态）
 *
 * 数据来源：Mock 数据（后端就绪后替换）
 */
import React, { useState } from 'react';
import {
  Card, Row, Col, Tabs, Table, Tag, Button, Space, Typography, Badge,
  Switch, Select, Checkbox, Modal, Input, message, Tooltip,
} from 'antd';
import {
  EyeOutlined, DownloadOutlined, ClockCircleOutlined,
  FileTextOutlined, BarChartOutlined, DashboardOutlined,
  AlertOutlined, BugOutlined, CloudServerOutlined, SafetyOutlined,
  PlusOutlined, EditOutlined, DeleteOutlined,
  DragOutlined, SettingOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text, Title, Paragraph } = Typography;

/* ==================== 类型定义 ==================== */

/** 报告模板卡片数据 */
interface ReportTemplate {
  key: string;
  title: string;
  description: string;
  icon: React.ReactNode;
  iconColor: string;
  tags: string[];
  lastGenerated?: string;
  schedule?: string;
}

/** 订阅记录 */
interface SubscriptionRecord {
  key: string;
  reportName: string;
  frequency: string;
  recipients: string[];
  format: string;
  enabled: boolean;
  lastSent?: string;
}

/** 自定义报告模块 */
interface ReportModule {
  key: string;
  label: string;
  category: string;
  checked: boolean;
}

/* ==================== Mock 数据 ==================== */

/** 管理视角报告模板 */
const MANAGEMENT_TEMPLATES: ReportTemplate[] = [
  {
    key: 'daily-ops',
    title: '运营日报',
    description: 'CTO月度运营报告：包含 SLA 总览、MTTR 趋势、事件趋势、根因分析、AI 自动总结',
    icon: <FileTextOutlined />,
    iconColor: '#4da6ff',
    tags: ['SLA', 'MTTR', '事件'],
    lastGenerated: '2026-03-24 08:00',
    schedule: '每日 08:00',
  },
  {
    key: 'weekly-report',
    title: 'SLA 达标报告',
    description: '各业务线 SLA 达标详情、错误预算消耗、环比趋势分析、不达标根因归类',
    icon: <BarChartOutlined />,
    iconColor: '#00e5a0',
    tags: ['SLA', '错误预算', '环比'],
    lastGenerated: '2026-03-22 09:00',
    schedule: '每周一 09:00',
  },
  {
    key: 'monthly-sla',
    title: '事件趋势报告',
    description: 'P0-P4 事件趋势统计、根因变化分析、TOP5 事件回顾、改进闭环率跟踪',
    icon: <DashboardOutlined />,
    iconColor: '#ffaa33',
    tags: ['事件', '根因', '闭环率'],
    lastGenerated: '2026-03-01 10:00',
    schedule: '每月 1 日',
  },
  {
    key: 'executive-dashboard',
    title: '资源效率报告',
    description: '弹性系数分析、过载/闲置资源识别、成本优化建议、容量规划指引',
    icon: <DashboardOutlined />,
    iconColor: '#6366f1',
    tags: ['资源', '成本', '容量'],
    lastGenerated: '2026-03-15 10:00',
    schedule: '每月 15 日',
  },
];

/** 技术视角报告模板 */
const TECHNICAL_TEMPLATES: ReportTemplate[] = [
  {
    key: 'alert-analysis',
    title: '告警质量分析',
    description: '告警有效率统计、降噪效果评估、误报 TOP10 排行、告警覆盖率分析',
    icon: <AlertOutlined />,
    iconColor: '#ff6b6b',
    tags: ['告警', '降噪', '覆盖率'],
    lastGenerated: '2026-03-23 14:00',
    schedule: '每日 14:00',
  },
  {
    key: 'incident-review',
    title: '故障复盘报告',
    description: 'P0/P1 事件时间线还原、根因定位、改进项跟踪、AI 辅助准确度评估',
    icon: <BugOutlined />,
    iconColor: '#ffaa33',
    tags: ['P0', 'P1', '根因', '改进'],
    lastGenerated: '2026-03-20 16:00',
    schedule: '事件触发',
  },
  {
    key: 'capacity-plan',
    title: '操作审计报告',
    description: '高危操作 TOP N 排行、违规操作统计、人员画像分析、操作趋势变化',
    icon: <CloudServerOutlined />,
    iconColor: '#4da6ff',
    tags: ['高危', '违规', '审计'],
    lastGenerated: '2026-03-21 08:00',
    schedule: '每周五',
  },
  {
    key: 'security-compliance',
    title: '安全合规报告',
    description: '等保条款合规检查、日志留存合规性、权限变更审计、数据脱敏验证',
    icon: <SafetyOutlined />,
    iconColor: '#00e5a0',
    tags: ['等保', '合规', '安全'],
    lastGenerated: '2026-03-18 10:00',
    schedule: '每月 1 日',
  },
];

/** 订阅管理 Mock 数据 */
const MOCK_SUBSCRIPTIONS: SubscriptionRecord[] = [
  {
    key: '1',
    reportName: '运营日报',
    frequency: '每日',
    recipients: ['admin@ops.com', 'cto@ops.com'],
    format: 'PDF',
    enabled: true,
    lastSent: '2026-03-24 08:00',
  },
  {
    key: '2',
    reportName: 'SLA 达标报告',
    frequency: '每周',
    recipients: ['sre-team@ops.com'],
    format: 'Excel',
    enabled: true,
    lastSent: '2026-03-22 09:00',
  },
  {
    key: '3',
    reportName: '告警质量分析',
    frequency: '每日',
    recipients: ['alert-owner@ops.com'],
    format: 'PDF',
    enabled: false,
    lastSent: '2026-03-20 14:00',
  },
  {
    key: '4',
    reportName: '安全合规报告',
    frequency: '每月',
    recipients: ['security@ops.com', 'compliance@ops.com'],
    format: 'PDF',
    enabled: true,
    lastSent: '2026-03-01 10:00',
  },
  {
    key: '5',
    reportName: '故障复盘报告',
    frequency: '事件触发',
    recipients: ['sre-team@ops.com', 'dev-lead@ops.com'],
    format: 'HTML',
    enabled: true,
    lastSent: '2026-03-20 16:00',
  },
];

/** 自定义报告可选模块 */
const REPORT_MODULES: ReportModule[] = [
  { key: 'sla-overview', label: 'SLA 总览', category: '运营', checked: true },
  { key: 'mttr-trend', label: 'MTTR 趋势', category: '运营', checked: false },
  { key: 'incident-trend', label: '事件趋势', category: '运营', checked: true },
  { key: 'root-cause', label: '根因分析', category: '分析', checked: false },
  { key: 'alert-quality', label: '告警质量', category: '分析', checked: false },
  { key: 'capacity', label: '容量分析', category: '资源', checked: true },
  { key: 'cost-optimize', label: '成本优化', category: '资源', checked: false },
  { key: 'audit-summary', label: '审计摘要', category: '安全', checked: false },
  { key: 'compliance', label: '合规检查', category: '安全', checked: false },
  { key: 'ai-summary', label: 'AI 智能总结', category: '智能', checked: true },
];

/* ==================== 子组件 ==================== */

/**
 * 报告模板卡片组件
 * 展示单个报告模板的标题、描述、标签、调度信息、操作按钮
 */
const TemplateCard: React.FC<{
  template: ReportTemplate;
  onPreview: (key: string) => void;
  onDownload: (key: string) => void;
}> = ({ template, onPreview, onDownload }) => {
  const { t } = useTranslation('analytics');

  return (
    <Card
      hoverable
      style={{ borderRadius: 8, height: '100%' }}
      bodyStyle={{ padding: 20 }}
    >
      {/* 卡片头部：图标 + 标题 */}
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 12 }}>
        <div
          style={{
            width: 40,
            height: 40,
            borderRadius: 8,
            backgroundColor: `${template.iconColor}15`,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 20,
            color: template.iconColor,
            marginRight: 12,
          }}
        >
          {template.icon}
        </div>
        <div>
          <Text strong style={{ fontSize: 15 }}>{template.title}</Text>
          {template.schedule && (
            <div style={{ fontSize: 12, color: '#86909C' }}>
              <ClockCircleOutlined style={{ marginRight: 4 }} />
              {template.schedule}
            </div>
          )}
        </div>
      </div>

      {/* 描述 */}
      <Paragraph
        type="secondary"
        style={{ fontSize: 13, marginBottom: 12 }}
        ellipsis={{ rows: 2 }}
      >
        {template.description}
      </Paragraph>

      {/* 标签 */}
      <div style={{ marginBottom: 12 }}>
        {template.tags.map((tag) => (
          <Tag key={tag} style={{ marginBottom: 4 }}>{tag}</Tag>
        ))}
      </div>

      {/* 最近生成时间 */}
      {template.lastGenerated && (
        <div style={{ fontSize: 12, color: '#86909C', marginBottom: 12 }}>
          {t('reports.lastGenerated')}: {template.lastGenerated}
        </div>
      )}

      {/* 操作按钮 */}
      <Space>
        <Button
          type="primary"
          size="small"
          icon={<EyeOutlined />}
          onClick={() => onPreview(template.key)}
        >
          {t('reports.preview')}
        </Button>
        <Button
          size="small"
          icon={<DownloadOutlined />}
          onClick={() => onDownload(template.key)}
        >
          {t('reports.download')}
        </Button>
        <Tooltip title={t('reports.schedule')}>
          <Button size="small" icon={<SettingOutlined />} />
        </Tooltip>
      </Space>
    </Card>
  );
};

/* ==================== 主组件 ==================== */

/**
 * 报告中心页面组件
 * 提供多视角报告模板管理、自定义报告构建、订阅管理等功能
 */
const ReportCenter: React.FC = () => {
  const { t } = useTranslation('analytics');

  /* ---------- 状态管理 ---------- */
  /** 当前活动 Tab */
  const [activeTab, setActiveTab] = useState('management');
  /** 订阅数据 */
  const [subscriptions, setSubscriptions] = useState<SubscriptionRecord[]>(MOCK_SUBSCRIPTIONS);
  /** 自定义报告模块勾选状态 */
  const [modules, setModules] = useState<ReportModule[]>(REPORT_MODULES);
  /** 预览弹窗可见性 */
  const [previewVisible, setPreviewVisible] = useState(false);
  /** 当前预览的报告 key */
  const [previewKey, setPreviewKey] = useState('');

  /* ---------- 事件处理 ---------- */

  /** 处理预览报告 */
  const handlePreview = (key: string) => {
    setPreviewKey(key);
    setPreviewVisible(true);
  };

  /** 处理下载报告 */
  const handleDownload = (key: string) => {
    message.success(t('reports.downloadStarted'));
  };

  /** 切换订阅启用状态 */
  const handleToggleSubscription = (key: string, enabled: boolean) => {
    setSubscriptions((prev) =>
      prev.map((s) => (s.key === key ? { ...s, enabled } : s))
    );
    message.success(enabled ? t('reports.subscriptionEnabled') : t('reports.subscriptionDisabled'));
  };

  /** 切换自定义报告模块选中状态 */
  const handleModuleToggle = (moduleKey: string) => {
    setModules((prev) =>
      prev.map((m) => (m.key === moduleKey ? { ...m, checked: !m.checked } : m))
    );
  };

  /* ---------- 渲染模板卡片网格 ---------- */

  /** 渲染 2x2 报告模板卡片网格 */
  const renderTemplateGrid = (templates: ReportTemplate[]) => (
    <Row gutter={[16, 16]}>
      {templates.map((tpl) => (
        <Col span={12} key={tpl.key}>
          <TemplateCard
            template={tpl}
            onPreview={handlePreview}
            onDownload={handleDownload}
          />
        </Col>
      ))}
    </Row>
  );

  /* ---------- 订阅管理表格列定义 ---------- */

  const subscriptionColumns = [
    {
      title: t('reports.subscription.reportName'),
      dataIndex: 'reportName',
      key: 'reportName',
      render: (name: string) => <Text strong>{name}</Text>,
    },
    {
      title: t('reports.subscription.frequency'),
      dataIndex: 'frequency',
      key: 'frequency',
      width: 100,
      render: (freq: string) => <Tag>{freq}</Tag>,
    },
    {
      title: t('reports.subscription.recipients'),
      dataIndex: 'recipients',
      key: 'recipients',
      width: 240,
      /** 渲染收件人列表，超过 2 个显示 +N */
      render: (recipients: string[]) => (
        <Space size={4} wrap>
          {recipients.slice(0, 2).map((r) => (
            <Tag key={r} color="blue">{r}</Tag>
          ))}
          {recipients.length > 2 && (
            <Tag>+{recipients.length - 2}</Tag>
          )}
        </Space>
      ),
    },
    {
      title: t('reports.subscription.format'),
      dataIndex: 'format',
      key: 'format',
      width: 80,
      render: (format: string) => <Tag color="green">{format}</Tag>,
    },
    {
      title: t('reports.subscription.status'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      /** 渲染启用/禁用开关 */
      render: (enabled: boolean, record: SubscriptionRecord) => (
        <Switch
          checked={enabled}
          size="small"
          onChange={(checked) => handleToggleSubscription(record.key, checked)}
        />
      ),
    },
    {
      title: t('reports.subscription.lastSent'),
      dataIndex: 'lastSent',
      key: 'lastSent',
      width: 160,
      render: (time: string) => <Text type="secondary">{time || '--'}</Text>,
    },
    {
      title: t('reports.subscription.actions'),
      key: 'actions',
      width: 100,
      /** 渲染编辑/删除操作按钮 */
      render: (_: unknown, record: SubscriptionRecord) => (
        <Space>
          <Tooltip title={t('reports.subscription.edit')}>
            <Button type="link" size="small" icon={<EditOutlined />} />
          </Tooltip>
          <Tooltip title={t('reports.subscription.delete')}>
            <Button type="link" size="small" danger icon={<DeleteOutlined />} />
          </Tooltip>
        </Space>
      ),
    },
  ];

  /* ---------- Tab 配置 ---------- */

  /** 自定义报告模块分类展示 */
  const moduleCategories = ['运营', '分析', '资源', '安全', '智能'];

  const tabItems = [
    {
      key: 'management',
      label: t('reports.tab.management'),
      children: renderTemplateGrid(MANAGEMENT_TEMPLATES),
    },
    {
      key: 'technical',
      label: t('reports.tab.technical'),
      children: renderTemplateGrid(TECHNICAL_TEMPLATES),
    },
    {
      key: 'custom',
      label: t('reports.tab.custom'),
      children: (
        <div>
          {/* 自定义报告说明 */}
          <Card style={{ borderRadius: 8, marginBottom: 16 }}>
            <Title level={5}>{t('reports.custom.title')}</Title>
            <Paragraph type="secondary">
              {t('reports.custom.description')}
            </Paragraph>
          </Card>

          {/* 可选模块列表（按分类分组） */}
          <Row gutter={[16, 16]}>
            {moduleCategories.map((category) => (
              <Col span={8} key={category}>
                <Card
                  title={
                    <Space>
                      <DragOutlined style={{ color: '#86909C' }} />
                      <span>{category}</span>
                    </Space>
                  }
                  size="small"
                  style={{ borderRadius: 8 }}
                >
                  {modules
                    .filter((m) => m.category === category)
                    .map((mod) => (
                      <div key={mod.key} style={{ padding: '6px 0' }}>
                        <Checkbox
                          checked={mod.checked}
                          onChange={() => handleModuleToggle(mod.key)}
                        >
                          {mod.label}
                        </Checkbox>
                      </div>
                    ))}
                </Card>
              </Col>
            ))}
          </Row>

          {/* 已选模块预览和生成按钮 */}
          <Card style={{ borderRadius: 8, marginTop: 16 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div>
                <Text strong>{t('reports.custom.selected')}: </Text>
                {modules
                  .filter((m) => m.checked)
                  .map((m) => (
                    <Tag key={m.key} color="blue" style={{ marginBottom: 4 }}>
                      {m.label}
                    </Tag>
                  ))}
              </div>
              <Button type="primary" icon={<FileTextOutlined />}>
                {t('reports.custom.generate')}
              </Button>
            </div>
          </Card>
        </div>
      ),
    },
    {
      key: 'subscriptions',
      label: (
        <Badge count={subscriptions.filter((s) => s.enabled).length} size="small" offset={[8, 0]}>
          {t('reports.tab.subscriptions')}
        </Badge>
      ),
      children: (
        <div>
          {/* 操作栏 */}
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
            <Text type="secondary">
              {t('reports.subscription.total', { count: subscriptions.length })}
            </Text>
            <Button type="primary" icon={<PlusOutlined />}>
              {t('reports.subscription.add')}
            </Button>
          </div>

          {/* 订阅列表表格 */}
          <Table<SubscriptionRecord>
            columns={subscriptionColumns}
            dataSource={subscriptions}
            pagination={false}
            size="middle"
          />
        </div>
      ),
    },
  ];

  /* ---------- 渲染 ---------- */

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('reports.title')}</Text>
      </div>

      {/* Tab 面板 */}
      <Card style={{ borderRadius: 8 }}>
        <Tabs items={tabItems} activeKey={activeTab} onChange={setActiveTab} />
      </Card>

      {/* 预览弹窗 */}
      <Modal
        title={t('reports.previewModal.title')}
        open={previewVisible}
        onCancel={() => setPreviewVisible(false)}
        width={800}
        footer={[
          <Button key="close" onClick={() => setPreviewVisible(false)}>
            {t('reports.previewModal.close')}
          </Button>,
          <Button key="download" type="primary" icon={<DownloadOutlined />} onClick={() => handleDownload(previewKey)}>
            {t('reports.previewModal.download')}
          </Button>,
        ]}
      >
        {/* 报告预览内容占位 */}
        <div style={{ minHeight: 400, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <div style={{ textAlign: 'center' }}>
            <FileTextOutlined style={{ fontSize: 48, color: '#86909C', marginBottom: 16 }} />
            <div>
              <Text type="secondary">{t('reports.previewModal.placeholder')}</Text>
            </div>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default ReportCenter;
