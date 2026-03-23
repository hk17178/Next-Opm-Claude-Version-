/**
 * SLA 报告页面（旧版） - 简化版 SLA 展示，包含概览卡片、业务明细表格、趋势图占位
 * 注：新版 SLA 大盘请使用 SLADashboard 页面
 */
import React from 'react';
import { Card, Row, Col, Table, Select, Space, Typography, Tag } from 'antd';
import { CheckCircleOutlined, WarningOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

/**
 * SLA 报告组件
 * - 过滤栏：时间周期、业务板块、服务等级、资产分级
 * - 概览卡片行：SLA 达成率、错误预算、事件数
 * - 业务 SLA 明细表格
 * - 趋势图占位区域
 */
const SLAReport: React.FC = () => {
  const { t } = useTranslation('analytics');

  /** 概览卡片数据定义 */
  const overviewCards = [
    { key: 'sla', label: t('sla.overview.sla'), value: '--', sub: t('sla.overview.target', { value: '99.95%' }) },
    { key: 'budget', label: t('sla.overview.errorBudget'), value: '--', sub: t('sla.overview.healthy') },
    { key: 'incidents', label: t('sla.overview.incidents'), value: '--', sub: t('sla.overview.downtime', { time: '--' }) },
  ];

  /** SLA 明细表格列定义 */
  const slaColumns = [
    { title: t('sla.table.business'), dataIndex: 'business', key: 'business' },
    { title: 'SLA', dataIndex: 'sla', key: 'sla', width: 100 },
    { title: t('sla.table.target'), dataIndex: 'target', key: 'target', width: 100 },
    {
      title: t('sla.table.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      /** 渲染达成状态：met 显示绿色勾，其他显示橙色警告 */
      render: (status: string) =>
        status === 'met' ? (
          <Tag color="#00B42A" icon={<CheckCircleOutlined />}>{t('sla.status.met')}</Tag>
        ) : (
          <Tag color="#FF7D00" icon={<WarningOutlined />}>{t('sla.status.nearMiss')}</Tag>
        ),
    },
    { title: t('sla.table.errorBudget'), dataIndex: 'errorBudget', key: 'errorBudget', width: 120 },
    { title: t('sla.table.downtime'), dataIndex: 'downtime', key: 'downtime', width: 100 },
  ];

  return (
    <div>
      {/* 过滤条件栏 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Space>
          {/* 时间周期选择 */}
          <Select placeholder={t('sla.filter.period')} style={{ width: 140 }} defaultValue="month"
            options={[
              { value: 'week', label: t('sla.filter.week') },
              { value: 'month', label: t('sla.filter.month') },
              { value: 'quarter', label: t('sla.filter.quarter') },
              { value: 'year', label: t('sla.filter.year') },
            ]}
          />
          {/* 业务板块过滤 */}
          <Select placeholder={t('sla.filter.business')} style={{ width: 140 }} allowClear />
          {/* 服务等级过滤 */}
          <Select placeholder={t('sla.filter.tier')} style={{ width: 140 }} allowClear />
          {/* 资产分级过滤 */}
          <Select placeholder={t('sla.filter.grade')} style={{ width: 140 }} allowClear />
        </Space>
      </Card>

      {/* 概览卡片行 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {overviewCards.map((card) => (
          <Col span={8} key={card.key}>
            <Card bordered style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
              bodyStyle={{ padding: '16px 20px', textAlign: 'center' }}
            >
              <div style={{ color: '#86909C', fontSize: 14 }}>{card.label}</div>
              <div style={{ fontSize: 32, fontWeight: 600, marginTop: 4 }}>{card.value}</div>
              <div style={{ color: '#86909C', fontSize: 12, marginTop: 4 }}>{card.sub}</div>
            </Card>
          </Col>
        ))}
      </Row>

      {/* 各业务 SLA 明细表格 */}
      <Card title={t('sla.byBusiness')} style={{ borderRadius: 8, marginBottom: 16 }}>
        <Table
          columns={slaColumns}
          dataSource={[]}
          locale={{ emptyText: t('sla.noData') }}
          pagination={false}
          size="middle"
        />
      </Card>

      {/* SLA 趋势图占位区域 */}
      <Card title={t('sla.trend')} style={{ borderRadius: 8, minHeight: 300 }}>
        <div style={{ textAlign: 'center', color: '#86909C', padding: 48 }}>
          {t('sla.trendPlaceholder')}
        </div>
      </Card>
    </div>
  );
};

export default SLAReport;
