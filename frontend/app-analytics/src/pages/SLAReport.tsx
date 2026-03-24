/**
 * SLA 报告页面（旧版） - 简化版 SLA 展示
 * 包含：概览卡片、业务明细表格、12 个月 SLA 趋势折线图、错误预算消耗进度条
 * 注：新版 SLA 大盘请使用 SLADashboard 页面
 */
import React, { useRef, useEffect } from 'react';
import * as echarts from 'echarts';
import { Card, Row, Col, Table, Select, Space, Typography, Tag, Progress } from 'antd';
import { CheckCircleOutlined, WarningOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

/* ========== 模拟数据 ========== */

/** 12 个月的月份标签（最近 12 个月） */
const MONTHS_LABELS: string[] = (() => {
  const labels: string[] = [];
  const now = new Date();
  for (let i = 11; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
    labels.push(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`);
  }
  return labels;
})();

/** 模拟 12 个月实际 SLA 达成率数据（百分比） */
const MOCK_ACTUAL_SLA = [
  99.92, 99.95, 99.88, 99.97, 99.93, 99.96,
  99.91, 99.94, 99.89, 99.98, 99.95, 99.93,
];

/** SLA 目标线数值（所有月份一致为 99.9%） */
const SLA_TARGET = 99.9;

/** 模拟错误预算消耗数据（各业务已消耗的错误预算百分比） */
const MOCK_ERROR_BUDGETS = [
  { business: '支付业务', consumed: 62, total: '43.2 分钟', used: '26.8 分钟' },
  { business: '订单业务', consumed: 35, total: '43.2 分钟', used: '15.1 分钟' },
  { business: '用户中心', consumed: 18, total: '43.2 分钟', used: '7.8 分钟' },
  { business: '商品服务', consumed: 78, total: '43.2 分钟', used: '33.7 分钟' },
  { business: '物流服务', consumed: 45, total: '43.2 分钟', used: '19.4 分钟' },
];

/**
 * SLA 报告组件
 * - 过滤栏：时间周期、业务板块、服务等级、资产分级
 * - 概览卡片行：SLA 达成率、错误预算、事件数
 * - 业务 SLA 明细表格
 * - 12 个月 SLA 趋势折线图（目标线 99.9% + 实际 SLA 线）
 * - 错误预算消耗进度条
 */
const SLAReport: React.FC = () => {
  const { t } = useTranslation('analytics');

  /** SLA 趋势图容器引用 */
  const trendChartRef = useRef<HTMLDivElement>(null);

  /** 概览卡片数据定义 */
  const overviewCards = [
    { key: 'sla', label: t('sla.overview.sla'), value: '99.93%', sub: t('sla.overview.target', { value: '99.9%' }) },
    { key: 'budget', label: t('sla.overview.errorBudget'), value: '45%', sub: t('sla.overview.healthy') },
    { key: 'incidents', label: t('sla.overview.incidents'), value: '12', sub: t('sla.overview.downtime', { time: '26.8 分钟' }) },
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

  /**
   * 渲染 12 个月 SLA 趋势折线图
   * 使用 ECharts 绘制包含目标线 (99.9%) 和实际 SLA 线的双折线图
   */
  useEffect(() => {
    if (!trendChartRef.current) return;

    /** 初始化 ECharts 实例 */
    const chart = echarts.init(trendChartRef.current);

    /** 配置 ECharts 图表选项 */
    chart.setOption({
      /** 图表提示框配置 */
      tooltip: {
        trigger: 'axis',
        formatter: (params: any) => {
          const p = Array.isArray(params) ? params : [params];
          let html = `<strong>${p[0]?.axisValue}</strong><br/>`;
          p.forEach((item: any) => {
            html += `${item.marker} ${item.seriesName}：${item.value}%<br/>`;
          });
          return html;
        },
      },
      /** 图例配置 */
      legend: {
        data: ['实际 SLA', 'SLA 目标 (99.9%)'],
        top: 4,
      },
      /** 图表网格边距配置 */
      grid: { left: 50, right: 20, top: 40, bottom: 30 },
      /** X 轴配置：12 个月份 */
      xAxis: {
        type: 'category',
        data: MONTHS_LABELS,
        axisLine: { lineStyle: { color: '#E5E6EB' } },
        axisLabel: { color: '#86909C', fontSize: 11 },
      },
      /** Y 轴配置：范围 99.5% ~ 100%，突出细微差异 */
      yAxis: {
        type: 'value',
        min: 99.5,
        max: 100,
        axisLabel: { formatter: (v: number) => `${v}%`, color: '#86909C' },
        splitLine: { lineStyle: { color: '#F2F3F5' } },
      },
      /** 系列数据配置 */
      series: [
        {
          /** 实际 SLA 折线 - 蓝色渐变区域 */
          name: '实际 SLA',
          type: 'line',
          smooth: true,
          data: MOCK_ACTUAL_SLA,
          lineStyle: { color: '#3491FA', width: 2 },
          itemStyle: { color: '#3491FA' },
          /** 面积渐变填充 */
          areaStyle: {
            color: {
              type: 'linear',
              x: 0, y: 0, x2: 0, y2: 1,
              colorStops: [
                { offset: 0, color: 'rgba(52,145,250,0.25)' },
                { offset: 1, color: 'rgba(52,145,250,0)' },
              ],
            },
          },
          /** 强调选中点的样式 */
          emphasis: { itemStyle: { borderWidth: 2, borderColor: '#fff' } },
        },
        {
          /** SLA 目标线 - 红色虚线，固定在 99.9% */
          name: 'SLA 目标 (99.9%)',
          type: 'line',
          data: MONTHS_LABELS.map(() => SLA_TARGET),
          lineStyle: { color: '#F53F3F', width: 2, type: 'dashed' },
          itemStyle: { color: '#F53F3F' },
          symbol: 'none',
        },
      ],
    });

    /** 监听窗口 resize 事件，自适应图表宽度 */
    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);

    /** 清理函数：移除事件监听并销毁图表实例 */
    return () => {
      window.removeEventListener('resize', handleResize);
      chart.dispose();
    };
  }, []);

  /**
   * 根据错误预算消耗百分比返回进度条颜色
   * - < 50%：绿色（安全）
   * - 50% ~ 80%：橙色（警告）
   * - > 80%：红色（危险）
   */
  const getBudgetColor = (consumed: number): string => {
    if (consumed < 50) return '#00B42A';
    if (consumed < 80) return '#FF7D00';
    return '#F53F3F';
  };

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

      {/* 12 个月 SLA 趋势折线图 */}
      <Card title={t('sla.trend')} style={{ borderRadius: 8, marginBottom: 16 }}>
        <div ref={trendChartRef} style={{ height: 350, width: '100%' }} />
      </Card>

      {/* 错误预算消耗进度条 */}
      <Card title="错误预算消耗" style={{ borderRadius: 8 }}>
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          各业务本月错误预算使用情况（基于 SLA 目标 99.9%，月度允许停机 43.2 分钟）
        </Text>
        {MOCK_ERROR_BUDGETS.map((item) => (
          <div key={item.business} style={{ marginBottom: 20 }}>
            {/* 业务名称和消耗详情 */}
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
              <Text strong>{item.business}</Text>
              <Text type="secondary" style={{ fontSize: 12 }}>
                已用 {item.used} / 总计 {item.total}
              </Text>
            </div>
            {/* 错误预算消耗进度条 */}
            <Progress
              percent={item.consumed}
              strokeColor={getBudgetColor(item.consumed)}
              trailColor="#F2F3F5"
              format={(percent) => `${percent}%`}
              size={['100%', 12]}
            />
          </div>
        ))}
      </Card>
    </div>
  );
};

export default SLAReport;
