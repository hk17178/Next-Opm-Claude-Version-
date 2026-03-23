/**
 * SLA 大盘页面 - 展示全局 SLA 达成状况、各业务 SLA 明细、趋势图
 * 支持按时间周期、业务板块、服务等级、资产分级过滤
 * 数据来源：GET /api/analytics/sla
 */
import React, { useState, useEffect, useCallback, useRef } from 'react';
import * as echarts from 'echarts';
import {
  Card, Row, Col, Table, Select, Space, Typography, Tag, Progress,
} from 'antd';
import {
  CheckCircleOutlined, WarningOutlined, ExclamationCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { fetchSLA, type SLARecord, type SLAOverview, type SLAResult } from '../api/analytics';

const { Text } = Typography;

/** SLA 达成状态对应的颜色和图标配置 */
const STATUS_CONFIG: Record<string, { color: string; icon: React.ReactNode }> = {
  met: { color: '#00B42A', icon: <CheckCircleOutlined /> },          // 已达成 - 绿色
  nearMiss: { color: '#FF7D00', icon: <WarningOutlined /> },         // 接近未达 - 橙色
  breached: { color: '#F53F3F', icon: <ExclamationCircleOutlined /> }, // 已违约 - 红色
};

/**
 * SLA 大盘组件
 * - 顶部过滤栏：时间周期、业务板块、服务等级、资产分级
 * - 健康指标卡片行：全局 SLA、错误预算、月度事件、平均 MTTR
 * - 各业务 SLA 明细表格：业务名、SLA 值、目标、状态、错误预算进度条、宕机时长
 * - 趋势图占位区域
 */
const SLADashboard: React.FC = () => {
  const { t } = useTranslation('analytics');
  const [loading, setLoading] = useState(false);                    // 表格加载状态
  const [slaData, setSlaData] = useState<SLARecord[]>([]);          // SLA 明细列表
  const [overview, setOverview] = useState<SLAOverview | null>(null); // SLA 概览数据
  const [period, setPeriod] = useState('month');                    // 当前选中的时间周期

  /**
   * 加载 SLA 数据
   * 调用 GET /api/analytics/sla 获取概览和明细
   * @param currentPeriod 时间周期参数
   */
  const loadData = useCallback(async (currentPeriod?: string) => {
    setLoading(true);
    try {
      // request<T> 已自动解包 ApiResponse.data，直接获取 SLAResult
      const result = await fetchSLA({ period: currentPeriod || period });
      setSlaData(result.list || []);
      setOverview(result.overview || null);
    } catch {
      // API 尚未就绪，保持空状态
    } finally {
      setLoading(false);
    }
  }, [period]);

  /** 组件挂载及依赖变化时加载数据 */
  useEffect(() => {
    loadData();
  }, [loadData]);

  /**
   * 处理时间周期切换
   * @param value 周期值（week/month/quarter/year）
   */
  const handlePeriodChange = (value: string) => {
    setPeriod(value);
    loadData(value);
  };

  /** SLA 趋势图容器 ref */
  const trendChartRef = useRef<HTMLDivElement>(null);

  /** 7 天 SLA 趋势演示数据（后端就绪后替换为 API 数据） */
  const trendDates = (() => {
    const dates: string[] = [];
    for (let i = 6; i >= 0; i--) {
      const d = new Date();
      d.setDate(d.getDate() - i);
      dates.push(`${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`);
    }
    return dates;
  })();

  /** 渲染 SLA 趋势图 */
  useEffect(() => {
    if (!trendChartRef.current) return;
    const chart = echarts.init(trendChartRef.current);
    chart.setOption({
      grid: { left: 40, right: 20, top: 30, bottom: 30 },
      tooltip: { trigger: 'axis', formatter: (params: any) => {
        const p = Array.isArray(params) ? params : [params];
        return p.map((item: any) => `${item.seriesName}：${item.value}%`).join('<br/>');
      }},
      legend: { data: ['支付业务', '订单业务', '用户中心'], top: 4 },
      xAxis: { type: 'category', data: trendDates, axisLine: { lineStyle: { color: '#E5E6EB' } } },
      yAxis: {
        type: 'value', min: 99, max: 100,
        axisLabel: { formatter: (v: number) => `${v}%` },
        splitLine: { lineStyle: { color: '#F2F3F5' } },
      },
      series: [
        {
          name: '支付业务', type: 'line', smooth: true,
          data: [99.97, 99.95, 99.98, 99.96, 99.99, 99.97, 99.95],
          lineStyle: { color: '#3491FA', width: 2 },
          itemStyle: { color: '#3491FA' },
          areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(52,145,250,0.2)' }, { offset: 1, color: 'rgba(52,145,250,0)' }] } },
        },
        {
          name: '订单业务', type: 'line', smooth: true,
          data: [99.91, 99.88, 99.93, 99.87, 99.95, 99.92, 99.90],
          lineStyle: { color: '#00B42A', width: 2 },
          itemStyle: { color: '#00B42A' },
          areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(0,180,42,0.15)' }, { offset: 1, color: 'rgba(0,180,42,0)' }] } },
        },
        {
          name: '用户中心', type: 'line', smooth: true,
          data: [99.99, 100.00, 99.99, 99.98, 100.00, 99.99, 99.98],
          lineStyle: { color: '#6366F1', width: 2 },
          itemStyle: { color: '#6366F1' },
        },
      ],
    });
    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);
    return () => {
      window.removeEventListener('resize', handleResize);
      chart.dispose();
    };
  }, []);

  /** 健康指标卡片数据，优先使用 API 返回值，未就绪时显示 '--' */
  const healthCards = [
    {
      key: 'sla',
      label: t('slaDashboard.health.overallSLA'),
      value: overview?.overallSLA || '--',
      sub: t('slaDashboard.health.target', { value: '99.95%' }),
      color: '#2E75B6',
    },
    {
      key: 'budget',
      label: t('slaDashboard.health.errorBudget'),
      value: overview?.errorBudgetRemaining || '--',
      sub: t('slaDashboard.health.healthy'),
      color: '#00B42A',
    },
    {
      key: 'incidents',
      label: t('slaDashboard.health.monthlyIncidents'),
      value: overview?.monthlyIncidents ?? '--',
      sub: t('slaDashboard.health.downtime', { time: overview?.totalDowntime || '--' }),
      color: '#FF7D00',
    },
    {
      key: 'mttr',
      label: t('slaDashboard.health.avgMTTR'),
      value: overview?.avgMTTR || '--',
      sub: t('slaDashboard.health.mttrTrend'),
      color: '#3491FA',
    },
  ];

  /** SLA 明细表格列定义 */
  const slaColumns = [
    { title: t('slaDashboard.table.business'), dataIndex: 'business', key: 'business' },
    {
      title: 'SLA',
      dataIndex: 'sla',
      key: 'sla',
      width: 100,
      /** 加粗显示 SLA 达成率 */
      render: (val: string) => <Text strong>{val}</Text>,
    },
    { title: t('slaDashboard.table.target'), dataIndex: 'target', key: 'target', width: 100 },
    {
      title: t('slaDashboard.table.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      /** 渲染达成状态标签，不同状态使用不同颜色和图标 */
      render: (status: string) => {
        const cfg = STATUS_CONFIG[status] || STATUS_CONFIG.met;
        return <Tag color={cfg.color} icon={cfg.icon}>{t(`slaDashboard.status.${status}`)}</Tag>;
      },
    },
    {
      title: t('slaDashboard.table.errorBudget'),
      dataIndex: 'errorBudget',
      key: 'errorBudget',
      width: 150,
      /** 渲染错误预算进度条，根据剩余百分比动态变色 */
      render: (val: string) => {
        const num = parseFloat(val) || 0;
        return (
          <Progress
            percent={num}
            size="small"
            strokeColor={num > 50 ? '#00B42A' : num > 20 ? '#FF7D00' : '#F53F3F'}
            format={() => val}
          />
        );
      },
    },
    { title: t('slaDashboard.table.downtime'), dataIndex: 'downtime', key: 'downtime', width: 100 },
  ];

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('slaDashboard.title')}</Text>
      </div>

      {/* 过滤条件栏 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Space>
          {/* 时间周期选择 */}
          <Select placeholder={t('slaDashboard.filter.period')} style={{ width: 140 }} value={period}
            onChange={handlePeriodChange}
            options={[
              { value: 'week', label: t('slaDashboard.filter.week') },
              { value: 'month', label: t('slaDashboard.filter.month') },
              { value: 'quarter', label: t('slaDashboard.filter.quarter') },
              { value: 'year', label: t('slaDashboard.filter.year') },
            ]}
          />
          {/* 业务板块过滤 */}
          <Select placeholder={t('slaDashboard.filter.business')} style={{ width: 140 }} allowClear />
          {/* 服务等级过滤 */}
          <Select placeholder={t('slaDashboard.filter.tier')} style={{ width: 140 }} allowClear />
          {/* 资产分级过滤 */}
          <Select placeholder={t('slaDashboard.filter.grade')} style={{ width: 140 }} allowClear />
        </Space>
      </Card>

      {/* 健康指标卡片行 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {healthCards.map((card) => (
          <Col span={6} key={card.key}>
            <Card bordered style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
              bodyStyle={{ padding: '16px 20px', textAlign: 'center' }}
            >
              <div style={{ color: '#86909C', fontSize: 14 }}>{card.label}</div>
              <div style={{ fontSize: 32, fontWeight: 600, marginTop: 4, color: card.color }}>{card.value}</div>
              <div style={{ color: '#86909C', fontSize: 12, marginTop: 4 }}>{card.sub}</div>
            </Card>
          </Col>
        ))}
      </Row>

      {/* 各业务 SLA 明细表格 */}
      <Card title={t('slaDashboard.byBusiness')} style={{ borderRadius: 8, marginBottom: 16 }}>
        <Table<SLARecord>
          columns={slaColumns}
          dataSource={slaData}
          loading={loading}
          locale={{ emptyText: t('slaDashboard.noData') }}
          pagination={false}
          size="middle"
        />
      </Card>

      {/* SLA 趋势图 */}
      <Card title={t('slaDashboard.trend')} style={{ borderRadius: 8 }}>
        <div ref={trendChartRef} style={{ height: 300, width: '100%' }} />
      </Card>
    </div>
  );
};

export default SLADashboard;
