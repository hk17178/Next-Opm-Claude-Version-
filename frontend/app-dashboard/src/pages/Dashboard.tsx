import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Row, Col, Card, Statistic, Tag, Typography, Space, Table, Spin } from 'antd';
import {
  AlertOutlined, WarningOutlined, CheckCircleOutlined,
  ClockCircleOutlined, DashboardOutlined, FileTextOutlined,
  HeartOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import * as echarts from 'echarts';
import './Dashboard.css';
import {
  fetchDashboardSummary, fetchAlertTrend7d, fetchRecentAlerts,
  type DashboardSummary, type AlertTrendPoint, type RecentAlert,
} from '../api/dashboard';
import { HealthMatrix } from '@opsnexus/ui-kit';
import type { HealthCell } from '@opsnexus/ui-kit';

const { Text } = Typography;

/** 告警严重程度颜色映射 */
const SEVERITY_COLORS: Record<string, string> = {
  P0: '#F53F3F', P1: '#FF7D00', P2: '#3491FA', P3: '#86909C',
};

/** 生成模拟仪表板汇总数据（后端 API 不可用时使用） */
function mockSummary(): DashboardSummary {
  return {
    activeAlerts: 12,
    todayEvents: 45,
    logVolume24h: 128340,
    serviceHealthRate: 97.5,
  };
}

/** 生成模拟 7 天告警趋势数据（后端 API 不可用时使用） */
function mockAlertTrend(): AlertTrendPoint[] {
  const points: AlertTrendPoint[] = [];
  const now = new Date();
  for (let i = 6; i >= 0; i--) {
    const d = new Date(now);
    d.setDate(d.getDate() - i);
    points.push({
      date: `${(d.getMonth() + 1).toString().padStart(2, '0')}-${d.getDate().toString().padStart(2, '0')}`,
      count: Math.floor(Math.random() * 30) + 5,
    });
  }
  return points;
}

/** 生成模拟最近 10 条未处理告警数据（后端 API 不可用时使用） */
function mockRecentAlerts(): RecentAlert[] {
  const severities = ['P0', 'P1', 'P2', 'P3'];
  const sources = ['Prometheus', 'Zabbix', 'CloudWatch', 'Datadog'];
  const contents = [
    'CPU usage exceeds 95% on prod-web-03',
    'Memory usage critical on db-master-01',
    'Disk I/O latency spike on storage-node-02',
    'API response time > 2s on gateway-01',
    'Connection pool exhausted on app-server-05',
    'SSL certificate expiring in 7 days',
    'Database replication lag > 30s',
    'Health check failed for service auth-svc',
    'Error rate > 5% on payment-api',
    'Pod restart count > 10 in last hour',
  ];
  return contents.map((content, i) => ({
    id: `alert-${i + 1}`,
    severity: severities[i % severities.length],
    content,
    source: sources[i % sources.length],
    triggerTime: new Date(Date.now() - i * 3600_000 * 2).toLocaleString(),
    status: i < 3 ? 'firing' : 'acknowledged',
  }));
}

/** 生成业务健康矩阵 mock 数据 — 5行(业务线)×5列(技术层级) */
function generateHealthCells(): HealthCell[] {
  const rows = ['payment', 'ecommerce', 'risk', 'logistics', 'account'];
  const cols = ['NET', 'HOST', 'APP', 'DB', 'MW'];

  const statusMap: Record<string, HealthCell['status']> = {
    'payment-NET': 'ok', 'payment-HOST': 'critical', 'payment-APP': 'degraded', 'payment-DB': 'ok', 'payment-MW': 'ok',
    'ecommerce-NET': 'ok', 'ecommerce-HOST': 'ok', 'ecommerce-APP': 'ok', 'ecommerce-DB': 'degraded', 'ecommerce-MW': 'ok',
    'risk-NET': 'ok', 'risk-HOST': 'ok', 'risk-APP': 'ok', 'risk-DB': 'ok', 'risk-MW': 'ok',
    'logistics-NET': 'degraded', 'logistics-HOST': 'ok', 'logistics-APP': 'ok', 'logistics-DB': 'ok', 'logistics-MW': 'ok',
    'account-NET': 'ok', 'account-HOST': 'ok', 'account-APP': 'critical', 'account-DB': 'ok', 'account-MW': 'degraded',
  };

  const detailsMap: Record<string, HealthCell['details']> = {
    'payment-NET': { cpu: 35, mem: 42 }, 'payment-HOST': { cpu: 96, mem: 88, disk: 72 }, 'payment-APP': { conn: 320, cpu: 78 },
    'payment-DB': { cpu: 45, disk: 55 }, 'payment-MW': { conn: 12, cpu: 30 },
    'ecommerce-NET': { cpu: 28, mem: 38 }, 'ecommerce-HOST': { cpu: 52, mem: 60 }, 'ecommerce-APP': { cpu: 48, conn: 150 },
    'ecommerce-DB': { disk: 82, cpu: 65 }, 'ecommerce-MW': { conn: 8, cpu: 25 },
    'risk-NET': { cpu: 22, mem: 30 }, 'risk-HOST': { cpu: 40, mem: 45 }, 'risk-APP': { cpu: 38, conn: 90 },
    'risk-DB': { cpu: 42, disk: 50 }, 'risk-MW': { conn: 5, cpu: 20 },
    'logistics-NET': { cpu: 75, mem: 70 }, 'logistics-HOST': { cpu: 50, mem: 55 }, 'logistics-APP': { cpu: 42, conn: 110 },
    'logistics-DB': { cpu: 38, disk: 48 }, 'logistics-MW': { conn: 6, cpu: 28 },
    'account-NET': { cpu: 30, mem: 35 }, 'account-HOST': { cpu: 55, mem: 50 }, 'account-APP': { cpu: 92, conn: 450 },
    'account-DB': { cpu: 40, disk: 52 }, 'account-MW': { conn: 15, cpu: 72 },
  };

  return rows.flatMap((row) =>
    cols.map((col) => {
      const key = `${row}-${col}`;
      return {
        id: key,
        label: `${row}/${col}`,
        status: statusMap[key] || 'ok',
        details: detailsMap[key],
      } satisfies HealthCell;
    }),
  );
}

/** 生成 7 天资源趋势 mock 数据（CPU + 内存使用率百分比） */
function generateResourceTrendData(): { dates: string[]; cpu: number[]; mem: number[] } {
  const dates: string[] = [];
  const cpu: number[] = [];
  const mem: number[] = [];
  const now = new Date();
  for (let i = 6; i >= 0; i--) {
    const d = new Date(now);
    d.setDate(d.getDate() - i);
    dates.push(`${(d.getMonth() + 1).toString().padStart(2, '0')}-${d.getDate().toString().padStart(2, '0')}`);
    // CPU 基线 55-75%，内存基线 60-80%，模拟波动
    cpu.push(Math.round(55 + Math.random() * 20 + (i === 2 ? 18 : 0)));  // 第5天有个尖峰
    mem.push(Math.round(60 + Math.random() * 20 + (i === 1 ? 12 : 0)));  // 第6天内存偏高
  }
  return { dates, cpu, mem };
}

const healthCells = generateHealthCells();
const resourceTrend = generateResourceTrendData();

/**
 * 告警趋势折线图组件（轻量 SVG 实现）
 * 展示最近 7 天的每日告警数量变化趋势
 * @param data - 每日告警数量数据点数组
 */
const AlertTrendChart: React.FC<{ data: AlertTrendPoint[] }> = ({ data }) => {
  if (data.length === 0) return null;

  // SVG 画布尺寸和内边距
  const width = 500;
  const height = 160;
  const pad = { top: 15, right: 15, bottom: 25, left: 40 };
  const chartW = width - pad.left - pad.right;
  const chartH = height - pad.top - pad.bottom;
  const maxCount = Math.max(...data.map((d) => d.count), 1);

  const points = data.map((d, i) => ({
    x: pad.left + (i / Math.max(data.length - 1, 1)) * chartW,
    y: pad.top + chartH - (d.count / maxCount) * chartH,
  }));
  const pathD = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x},${p.y}`).join(' ');
  const areaD = `${pathD} L${points[points.length - 1].x},${pad.top + chartH} L${points[0].x},${pad.top + chartH} Z`;

  return (
    <svg viewBox={`0 0 ${width} ${height}`} style={{ width: '100%', maxHeight: 180 }}>
      <defs>
        <linearGradient id="trendGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#F53F3F" stopOpacity={0.25} />
          <stop offset="100%" stopColor="#F53F3F" stopOpacity={0.02} />
        </linearGradient>
      </defs>
      {[0, 0.5, 1].map((ratio) => {
        const y = pad.top + chartH - ratio * chartH;
        return (
          <g key={ratio}>
            <line x1={pad.left} y1={y} x2={pad.left + chartW} y2={y} className="svg-grid-line" strokeDasharray="3,3" />
            <text x={pad.left - 6} y={y + 4} textAnchor="end" fontSize={10} className="svg-axis-text">
              {Math.round(maxCount * ratio)}
            </text>
          </g>
        );
      })}
      {data.map((d, i) => (
        <text
          key={i}
          x={pad.left + (i / Math.max(data.length - 1, 1)) * chartW}
          y={height - 4}
          textAnchor="middle"
          fontSize={10}
          className="svg-axis-text"
        >
          {d.date}
        </text>
      ))}
      <path d={areaD} fill="url(#trendGrad)" />
      <path d={pathD} fill="none" stroke="#F53F3F" strokeWidth={2} />
      {points.map((p, i) => (
        <circle key={i} cx={p.x} cy={p.y} r={3} fill="#F53F3F" />
      ))}
    </svg>
  );
};

/**
 * 运维仪表板主页组件
 * 功能：全局状态栏、4 个 KPI 卡片、告警趋势折线图、最新未处理告警列表
 * 暗色主题（NOC 大屏风格）
 */
const Dashboard: React.FC = () => {
  const { t } = useTranslation('dashboard');
  const [loading, setLoading] = useState(true);
  const [summary, setSummary] = useState<DashboardSummary | null>(null);        // 汇总数据
  const [trendData, setTrendData] = useState<AlertTrendPoint[]>([]);             // 告警趋势数据
  const [recentAlerts, setRecentAlerts] = useState<RecentAlert[]>([]);           // 最近未处理告警

  const trendRef = useRef<HTMLDivElement>(null);
  const resourceRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!trendRef.current) return;
    const chart = echarts.init(trendRef.current);
    const style = getComputedStyle(document.documentElement);
    const borderColor = style.getPropertyValue('--border-primary').trim() || '#E5E6EB';
    const textColor = style.getPropertyValue('--text-tertiary').trim() || '#86909C';
    const days = (() => {
      const arr: string[] = [];
      for (let i = 6; i >= 0; i--) {
        const d = new Date(); d.setDate(d.getDate() - i);
        arr.push(`${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`);
      }
      return arr;
    })();
    chart.setOption({
      grid: { left: 36, right: 12, top: 20, bottom: 28 },
      tooltip: { trigger: 'axis' },
      xAxis: { type: 'category', data: days, axisLine: { lineStyle: { color: borderColor } }, axisLabel: { color: textColor } },
      yAxis: { type: 'value', splitLine: { lineStyle: { color: borderColor } }, axisLabel: { color: textColor } },
      series: [{
        type: 'line', smooth: true,
        data: [12, 19, 8, 25, 15, 31, 18],
        lineStyle: { color: '#3491FA', width: 2 },
        itemStyle: { color: '#3491FA' },
        areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(52,145,250,0.25)' }, { offset: 1, color: 'rgba(52,145,250,0)' }] } },
      }],
    });
    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);
    return () => { window.removeEventListener('resize', handleResize); chart.dispose(); };
  }, []);

  /** 资源趋势曲线 ECharts 初始化 — CPU + 内存双线图 */
  useEffect(() => {
    if (!resourceRef.current) return;
    const chart = echarts.init(resourceRef.current);
    const style = getComputedStyle(document.documentElement);
    const borderColor = style.getPropertyValue('--border-primary').trim() || '#E5E6EB';
    const textColor = style.getPropertyValue('--text-tertiary').trim() || '#86909C';

    chart.setOption({
      grid: { left: 46, right: 16, top: 36, bottom: 28 },
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'var(--card-bg, #1D2129)',
        borderColor: 'var(--border-primary, #333)',
        textStyle: { color: 'var(--text-primary, #F2F3F5)' },
        formatter: (params: Array<{ seriesName: string; value: number; axisValue: string }>) => {
          if (!Array.isArray(params) || params.length === 0) return '';
          let tip = `<div style="font-size:12px;margin-bottom:4px">${params[0].axisValue}</div>`;
          params.forEach((p: { seriesName: string; value: number }) => {
            tip += `<div style="font-size:12px">${p.seriesName}: <b>${p.value}%</b></div>`;
          });
          return tip;
        },
      },
      legend: {
        data: ['CPU', 'Memory'],
        top: 4,
        right: 16,
        textStyle: { color: textColor, fontSize: 11 },
        icon: 'roundRect',
        itemWidth: 12,
        itemHeight: 3,
      },
      xAxis: {
        type: 'category',
        data: resourceTrend.dates,
        axisLine: { lineStyle: { color: borderColor } },
        axisLabel: { color: textColor, fontSize: 10 },
      },
      yAxis: {
        type: 'value',
        min: 0,
        max: 100,
        splitLine: { lineStyle: { color: borderColor, type: 'dashed' } },
        axisLabel: { color: textColor, fontSize: 10, formatter: '{value}%' },
      },
      series: [
        {
          name: 'CPU',
          type: 'line',
          smooth: true,
          data: resourceTrend.cpu,
          lineStyle: { color: '#3491FA', width: 2 },
          itemStyle: { color: '#3491FA' },
          areaStyle: {
            color: {
              type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
              colorStops: [
                { offset: 0, color: 'rgba(52,145,250,0.2)' },
                { offset: 1, color: 'rgba(52,145,250,0)' },
              ],
            },
          },
          symbol: 'circle',
          symbolSize: 5,
        },
        {
          name: 'Memory',
          type: 'line',
          smooth: true,
          data: resourceTrend.mem,
          lineStyle: { color: '#00B42A', width: 2 },
          itemStyle: { color: '#00B42A' },
          areaStyle: {
            color: {
              type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
              colorStops: [
                { offset: 0, color: 'rgba(0,180,42,0.2)' },
                { offset: 1, color: 'rgba(0,180,42,0)' },
              ],
            },
          },
          symbol: 'circle',
          symbolSize: 5,
        },
      ],
    });
    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);
    return () => { window.removeEventListener('resize', handleResize); chart.dispose(); };
  }, []);

  /**
   * 并行获取仪表板全部数据：汇总指标、告警趋势、最近告警
   * API 不可用时自动回退到模拟数据
   */
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [sum, trend, alerts] = await Promise.all([
        fetchDashboardSummary(),
        fetchAlertTrend7d(),
        fetchRecentAlerts(),
      ]);
      setSummary(sum);
      setTrendData(trend);
      setRecentAlerts(alerts);
    } catch {
      // 后端 API 不可用，使用模拟数据展示 UI
      setSummary(mockSummary());
      setTrendData(mockAlertTrend());
      setRecentAlerts(mockRecentAlerts());
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  /** 顶部 4 个 KPI 卡片配置：活跃告警、今日事件、日志量、服务健康率 */
  const kpiCards = [
    {
      key: 'alerts',
      label: t('kpi.activeAlerts'),
      value: summary?.activeAlerts ?? 0,
      icon: <AlertOutlined style={{ color: '#F53F3F' }} />,
      color: '#F53F3F',
    },
    {
      key: 'events',
      label: t('kpi.todayEvents'),
      value: summary?.todayEvents ?? 0,
      icon: <WarningOutlined style={{ color: '#FF7D00' }} />,
      color: '#FF7D00',
    },
    {
      key: 'logs',
      label: t('kpi.logVolume24h'),
      value: summary?.logVolume24h ?? 0,
      icon: <FileTextOutlined style={{ color: '#3491FA' }} />,
      color: '#3491FA',
    },
    {
      key: 'health',
      label: t('kpi.serviceHealth'),
      value: summary?.serviceHealthRate ?? 0,
      suffix: '%',
      icon: <HeartOutlined style={{ color: '#00B42A' }} />,
      color: '#00B42A',
    },
  ];

  /** 最近告警表格列定义 */
  const alertColumns = [
    {
      title: t('recentAlerts.severity'),
      dataIndex: 'severity',
      key: 'severity',
      width: 70,
      render: (severity: string) => (
        <Tag
          style={{
            background: SEVERITY_COLORS[severity] || '#86909C',
            color: '#fff',
            border: 'none',
            borderRadius: 4,
            fontWeight: 600,
          }}
        >
          {severity}
        </Tag>
      ),
    },
    {
      title: t('recentAlerts.content'),
      dataIndex: 'content',
      key: 'content',
      ellipsis: true,
      render: (text: string) => <span className="dashboard-cell-primary">{text}</span>,
    },
    {
      title: t('recentAlerts.source'),
      dataIndex: 'source',
      key: 'source',
      width: 110,
      render: (text: string) => <span className="dashboard-cell-secondary">{text}</span>,
    },
    {
      title: t('recentAlerts.time'),
      dataIndex: 'triggerTime',
      key: 'triggerTime',
      width: 160,
      render: (text: string) => <span className="dashboard-cell-secondary">{text}</span>,
    },
  ];

  return (
    <div className="dashboard-root">
      {/* 全局状态栏：展示活跃告警数、事件数、SLA 和整体健康状态 */}
      <Card
        bodyStyle={{
          padding: '12px 24px',
          background: 'var(--card-bg)',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          borderRadius: 8,
        }}
        bordered={false}
        style={{ marginBottom: 16 }}
      >
        <Space size={32}>
          <span>
            <DashboardOutlined style={{ marginRight: 8 }} />
            {t('status.title')}
          </span>
          <span>
            <AlertOutlined style={{ color: '#F53F3F', marginRight: 4 }} />
            {t('status.activeAlerts')}: <Text strong>{summary?.activeAlerts ?? 0}</Text>
          </span>
          <span>
            <WarningOutlined style={{ color: '#FF7D00', marginRight: 4 }} />
            {t('status.activeIncidents')}: <Text strong>{summary?.todayEvents ?? 0}</Text>
          </span>
          <span>
            <CheckCircleOutlined style={{ color: '#00B42A', marginRight: 4 }} />
            SLA: <Text strong>{summary?.serviceHealthRate ?? '--'}%</Text>
          </span>
        </Space>
        <Tag color={summary && summary.serviceHealthRate >= 95 ? '#00B42A' : '#F53F3F'}>
          {summary && summary.serviceHealthRate >= 95 ? t('status.normal') : t('status.degraded')}
        </Tag>
      </Card>

      {/* KPI 指标卡片区域 */}
      <Spin spinning={loading}>
        <Row gutter={16} style={{ marginBottom: 16 }}>
          {kpiCards.map((card) => (
            <Col span={6} key={card.key}>
              <Card
                style={{ background: 'var(--card-bg)', borderRadius: 8, border: '1px solid var(--border-primary)' }}
                bodyStyle={{ padding: 16 }}
              >
                <Statistic
                  title={<span className="dashboard-stat-label">{card.label}</span>}
                  value={card.value}
                  suffix={card.suffix}
                  prefix={card.icon}
                  valueStyle={{ color: 'var(--text-primary)' }}
                />
              </Card>
            </Col>
          ))}
        </Row>

        {/* 告警趋势折线图 + 最近未处理告警列表 */}
        <Row gutter={16}>
          <Col span={10}>
            <Card
              title={<span className="dashboard-card-title">{t('alertTrend')}</span>}
              style={{ background: 'var(--card-bg)', borderRadius: 8, border: '1px solid var(--border-primary)', minHeight: 300 }}
              bodyStyle={{ padding: 16 }}
            >
              <div ref={trendRef} className="dashboard-trend-chart" />
            </Card>
          </Col>
          <Col span={14}>
            <Card
              title={<span className="dashboard-card-title">{t('recentAlerts.title')}</span>}
              style={{ background: 'var(--card-bg)', borderRadius: 8, border: '1px solid var(--border-primary)', minHeight: 300 }}
              bodyStyle={{ padding: 0 }}
            >
              <Table
                columns={alertColumns}
                dataSource={recentAlerts}
                rowKey="id"
                size="small"
                pagination={false}
                style={{ background: 'transparent' }}
              />
            </Card>
          </Col>
        </Row>

        {/* 业务健康矩阵 + 资源趋势曲线 */}
        <Row gutter={16} style={{ marginTop: 16 }}>
          <Col span={12}>
            <Card
              title={<span className="dashboard-card-title">{t('businessHealth')}</span>}
              style={{
                background: 'var(--card-bg, #1D2129)',
                borderRadius: 8,
                border: '1px solid var(--border-primary, #333)',
                minHeight: 250,
              }}
              bodyStyle={{ padding: 0 }}
            >
              <HealthMatrix
                cells={healthCells}
                rows={5}
                cols={5}
                cellHeight={28}
                colLabels={['NET', 'HOST', 'APP', 'DB', 'MW']}
                rowLabels={['pay', 'shop', 'risk', 'logi', 'acct']}
              />
            </Card>
          </Col>
          <Col span={12}>
            <Card
              title={<span className="dashboard-card-title">{t('resourceCurves')}</span>}
              style={{
                background: 'var(--card-bg, #1D2129)',
                borderRadius: 8,
                border: '1px solid var(--border-primary, #333)',
                minHeight: 250,
              }}
              bodyStyle={{ padding: 16 }}
            >
              <div
                ref={resourceRef}
                style={{
                  height: 180,
                  width: '100%',
                }}
              />
            </Card>
          </Col>
        </Row>
      </Spin>
    </div>
  );
};

export default Dashboard;
