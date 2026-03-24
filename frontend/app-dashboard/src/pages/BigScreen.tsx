import React, { useState, useEffect, useRef } from 'react';
import { Row, Col, Card, Typography, Tag, Space } from 'antd';
import {
  AlertOutlined, ThunderboltOutlined, SafetyCertificateOutlined,
  CloudServerOutlined, FileTextOutlined, ClockCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import * as echarts from 'echarts';
import './BigScreen.css';

const { Text, Title } = Typography;

/* ========== Mock 数据 ========== */

const MOCK_METRICS = {
  activeAlerts: 23,
  p0Count: 3,
  p1Count: 8,
  activeIncidents: 5,
  processingCount: 3,
  slaRate: 99.92,
  slaTarget: '99.95%',
  assetOnline: 156,
  assetTotal: 160,
  logThroughput: '12.8K',
};

/** 事件状态分布 mock 数据 */
const INCIDENT_STATUS_DATA = [
  { name: '处理中', value: 3, color: '#FF7D00' },
  { name: '已确认', value: 2, color: '#3491FA' },
  { name: '待响应', value: 4, color: '#F53F3F' },
  { name: '已解决', value: 12, color: '#00B42A' },
];

/** TOP 告警资产 mock 数据 */
const TOP_ALERT_ASSETS = [
  { name: 'prod-db-master-01', count: 12, severity: 'P0' },
  { name: 'k8s-node-07', count: 9, severity: 'P1' },
  { name: 'cache-cluster-03', count: 7, severity: 'P1' },
  { name: 'payment-api-gateway', count: 5, severity: 'P2' },
  { name: 'cdn-edge-sz-02', count: 4, severity: 'P2' },
];

/** 事件时间线 mock 数据 */
const INCIDENT_TIMELINE = [
  { time: '09:15', title: '支付系统响应延迟', severity: 'P0', status: '处理中' },
  { time: '10:32', title: '数据库主从切换异常', severity: 'P0', status: '已确认' },
  { time: '11:48', title: 'CDN 节点故障', severity: 'P1', status: '处理中' },
  { time: '13:05', title: 'Redis 集群内存告警', severity: 'P1', status: '处理中' },
  { time: '14:22', title: 'ES 集群磁盘空间不足', severity: 'P2', status: '已解决' },
];

/** 日志量趋势 mock 数据 (24h) */
const LOG_VOLUME_DATA = Array.from({ length: 24 }, (_, i) => ({
  hour: `${String(i).padStart(2, '0')}:00`,
  volume: Math.floor(8000 + Math.random() * 12000),
}));

/** 严重级别颜色 */
const SEVERITY_COLORS: Record<string, string> = {
  P0: '#F53F3F', P1: '#FF7D00', P2: '#FADC19', P3: '#86909C',
};

/* ========== 子组件 ========== */

interface MetricCardProps {
  icon: React.ReactNode;
  label: string;
  value: string | number;
  color: string;
  sub?: string;
}

const MetricBlock: React.FC<MetricCardProps> = ({ icon, label, value, color, sub }) => (
  <Card
    style={{
      background: 'rgba(255,255,255,0.06)',
      border: '1px solid rgba(255,255,255,0.1)',
      borderRadius: 12,
      height: '100%',
    }}
    bodyStyle={{ padding: '24px 20px', textAlign: 'center' }}
  >
    <div style={{ fontSize: 28, color, marginBottom: 8 }}>{icon}</div>
    <div style={{ color: 'rgba(255,255,255,0.65)', fontSize: 14 }}>{label}</div>
    <div style={{ fontSize: 48, fontWeight: 700, color, marginTop: 8, lineHeight: 1.2 }}>{value}</div>
    {sub && <div style={{ color: 'rgba(255,255,255,0.45)', fontSize: 12, marginTop: 8 }}>{sub}</div>}
  </Card>
);

/** 事件状态分布 — 环形色块 + 数字 */
const IncidentStatusRing: React.FC = () => {
  const total = INCIDENT_STATUS_DATA.reduce((sum, d) => sum + d.value, 0);

  // 计算每个色块在圆环上的角度
  let cumAngle = 0;
  const segments = INCIDENT_STATUS_DATA.map((d) => {
    const angle = (d.value / total) * 360;
    const start = cumAngle;
    cumAngle += angle;
    return { ...d, startAngle: start, endAngle: cumAngle };
  });

  const size = 160;
  const cx = size / 2;
  const cy = size / 2;
  const outerR = 70;
  const innerR = 46;

  const polarToXY = (angle: number, r: number) => ({
    x: cx + r * Math.cos((angle - 90) * Math.PI / 180),
    y: cy + r * Math.sin((angle - 90) * Math.PI / 180),
  });

  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', height: '100%', justifyContent: 'center' }}>
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
        {segments.map((seg, i) => {
          const startOuter = polarToXY(seg.startAngle, outerR);
          const endOuter = polarToXY(seg.endAngle, outerR);
          const startInner = polarToXY(seg.endAngle, innerR);
          const endInner = polarToXY(seg.startAngle, innerR);
          const largeArc = seg.endAngle - seg.startAngle > 180 ? 1 : 0;
          const d = [
            `M ${startOuter.x} ${startOuter.y}`,
            `A ${outerR} ${outerR} 0 ${largeArc} 1 ${endOuter.x} ${endOuter.y}`,
            `L ${startInner.x} ${startInner.y}`,
            `A ${innerR} ${innerR} 0 ${largeArc} 0 ${endInner.x} ${endInner.y}`,
            'Z',
          ].join(' ');
          return <path key={i} d={d} fill={seg.color} opacity={0.85} />;
        })}
        <text x={cx} y={cy - 6} textAnchor="middle" fill="#E5E6EB" fontSize={28} fontWeight={700}>{total}</text>
        <text x={cx} y={cy + 14} textAnchor="middle" fill="rgba(255,255,255,0.5)" fontSize={11}>事件总数</text>
      </svg>
      <div style={{ display: 'flex', gap: 16, marginTop: 12, flexWrap: 'wrap', justifyContent: 'center' }}>
        {INCIDENT_STATUS_DATA.map((d) => (
          <span key={d.name} style={{ fontSize: 12, color: 'rgba(255,255,255,0.65)' }}>
            <span style={{
              display: 'inline-block', width: 10, height: 10, borderRadius: 2,
              background: d.color, marginRight: 4, verticalAlign: 'middle',
            }} />
            {d.name} <span style={{ color: d.color, fontWeight: 600 }}>{d.value}</span>
          </span>
        ))}
      </div>
    </div>
  );
};

/** TOP 告警资产横向条形图 */
const TopAlertAssets: React.FC = () => {
  const maxCount = Math.max(...TOP_ALERT_ASSETS.map((a) => a.count));

  return (
    <div style={{ padding: '8px 0' }}>
      {TOP_ALERT_ASSETS.map((asset, idx) => (
        <div key={asset.name} style={{ marginBottom: idx < TOP_ALERT_ASSETS.length - 1 ? 16 : 0 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
            <span style={{ color: 'rgba(255,255,255,0.85)', fontSize: 13 }}>
              <Tag
                style={{
                  background: SEVERITY_COLORS[asset.severity],
                  color: '#fff',
                  border: 'none',
                  fontSize: 10,
                  padding: '0 4px',
                  lineHeight: '16px',
                  marginRight: 6,
                }}
              >
                {asset.severity}
              </Tag>
              {asset.name}
            </span>
            <span style={{ color: SEVERITY_COLORS[asset.severity], fontWeight: 600, fontSize: 14 }}>
              {asset.count}
            </span>
          </div>
          <div style={{
            height: 6,
            borderRadius: 3,
            background: 'rgba(255,255,255,0.08)',
            overflow: 'hidden',
          }}>
            <div style={{
              width: `${(asset.count / maxCount) * 100}%`,
              height: '100%',
              borderRadius: 3,
              background: `linear-gradient(90deg, ${SEVERITY_COLORS[asset.severity]}80, ${SEVERITY_COLORS[asset.severity]})`,
              transition: 'width 0.6s ease',
            }} />
          </div>
        </div>
      ))}
    </div>
  );
};

/** 事件时间线 */
const IncidentTimeline: React.FC = () => (
  <div style={{ padding: '4px 0' }}>
    {INCIDENT_TIMELINE.map((item, idx) => (
      <div
        key={idx}
        style={{
          display: 'flex',
          alignItems: 'flex-start',
          marginBottom: idx < INCIDENT_TIMELINE.length - 1 ? 14 : 0,
          position: 'relative',
          paddingLeft: 20,
        }}
      >
        {/* 时间线竖线 + 圆点 */}
        <div style={{
          position: 'absolute',
          left: 4,
          top: 0,
          bottom: idx < INCIDENT_TIMELINE.length - 1 ? -14 : 0,
          width: 2,
          background: idx < INCIDENT_TIMELINE.length - 1 ? 'rgba(255,255,255,0.1)' : 'transparent',
        }} />
        <div style={{
          position: 'absolute',
          left: 0,
          top: 4,
          width: 10,
          height: 10,
          borderRadius: '50%',
          background: SEVERITY_COLORS[item.severity],
          boxShadow: `0 0 6px ${SEVERITY_COLORS[item.severity]}60`,
        }} />
        <div style={{ flex: 1, marginLeft: 8 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ color: 'rgba(255,255,255,0.85)', fontSize: 13, fontWeight: 500 }}>
              {item.title}
            </span>
            <Tag
              style={{
                background: item.status === '已解决' ? 'rgba(0,180,42,0.2)' : 'rgba(255,125,0,0.2)',
                color: item.status === '已解决' ? '#00B42A' : '#FF7D00',
                border: 'none',
                fontSize: 11,
              }}
            >
              {item.status}
            </Tag>
          </div>
          <div style={{ color: 'rgba(255,255,255,0.45)', fontSize: 11, marginTop: 2 }}>
            <Tag style={{ background: SEVERITY_COLORS[item.severity], color: '#fff', border: 'none', fontSize: 10, padding: '0 4px', lineHeight: '16px', marginRight: 6 }}>
              {item.severity}
            </Tag>
            {item.time}
          </div>
        </div>
      </div>
    ))}
  </div>
);

/** 日志量趋势迷你面积图 */
const LogVolumeChart: React.FC = () => {
  const chartRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!chartRef.current) return;
    const chart = echarts.init(chartRef.current, 'dark');
    chart.setOption({
      backgroundColor: 'transparent',
      grid: { left: 50, right: 16, top: 16, bottom: 30 },
      tooltip: {
        trigger: 'axis',
        formatter: (params: any) => {
          const p = params[0];
          return `${p.name}<br/>日志量: <b>${(p.value / 1000).toFixed(1)}K</b> 条/秒`;
        },
      },
      xAxis: {
        type: 'category',
        data: LOG_VOLUME_DATA.map((d) => d.hour),
        axisLabel: { color: 'rgba(255,255,255,0.5)', fontSize: 10, interval: 3 },
        axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
      },
      yAxis: {
        type: 'value',
        axisLabel: {
          color: 'rgba(255,255,255,0.5)',
          fontSize: 10,
          formatter: (v: number) => `${(v / 1000).toFixed(0)}K`,
        },
        splitLine: { lineStyle: { color: 'rgba(255,255,255,0.06)' } },
      },
      series: [{
        type: 'line',
        data: LOG_VOLUME_DATA.map((d) => d.volume),
        smooth: true,
        symbol: 'none',
        lineStyle: { color: '#722ED1', width: 2 },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(114,46,209,0.35)' },
            { offset: 1, color: 'rgba(114,46,209,0.02)' },
          ]),
        },
      }],
    });
    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);
    return () => { window.removeEventListener('resize', handleResize); chart.dispose(); };
  }, []);

  return <div ref={chartRef} style={{ height: 200, width: '100%' }} />;
};

/* ========== 主组件 ========== */

const BigScreen: React.FC = () => {
  const { t } = useTranslation('dashboard');
  const [currentTime, setCurrentTime] = useState(new Date());

  useEffect(() => {
    const timer = setInterval(() => setCurrentTime(new Date()), 1000);
    return () => clearInterval(timer);
  }, []);

  /** 告警趋势图 */
  const alertTrendRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!alertTrendRef.current) return;
    const chart = echarts.init(alertTrendRef.current, 'dark');
    const hours = Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:00`);
    const p0Data = [0, 0, 1, 0, 0, 0, 2, 1, 3, 2, 1, 4, 2, 1, 3, 2, 4, 3, 2, 1, 2, 1, 0, 1];
    const p1Data = [1, 2, 3, 2, 1, 3, 4, 6, 8, 7, 5, 9, 7, 5, 8, 6, 10, 8, 6, 5, 4, 3, 2, 2];
    chart.setOption({
      backgroundColor: 'transparent',
      grid: { left: 40, right: 20, top: 30, bottom: 30 },
      tooltip: { trigger: 'axis' },
      legend: { data: ['P0', 'P1'], top: 4, textStyle: { color: 'rgba(255,255,255,0.7)' } },
      xAxis: { type: 'category', data: hours, axisLabel: { color: 'rgba(255,255,255,0.5)', fontSize: 10, interval: 2 } },
      yAxis: { type: 'value', axisLabel: { color: 'rgba(255,255,255,0.5)' }, splitLine: { lineStyle: { color: 'rgba(255,255,255,0.08)' } } },
      series: [
        {
          name: 'P0', type: 'bar', stack: 'total',
          data: p0Data,
          itemStyle: { color: '#F53F3F' },
          barMaxWidth: 16,
        },
        {
          name: 'P1', type: 'bar', stack: 'total',
          data: p1Data,
          itemStyle: { color: '#FF7D00' },
          barMaxWidth: 16,
        },
      ],
    });
    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);
    return () => { window.removeEventListener('resize', handleResize); chart.dispose(); };
  }, []);

  const metrics = [
    {
      key: 'alerts',
      icon: <AlertOutlined />,
      label: t('bigScreen.metric.activeAlerts'),
      value: MOCK_METRICS.activeAlerts,
      color: '#F53F3F',
      sub: t('bigScreen.metric.alertSub', { p0: MOCK_METRICS.p0Count, p1: MOCK_METRICS.p1Count }),
    },
    {
      key: 'incidents',
      icon: <ThunderboltOutlined />,
      label: t('bigScreen.metric.activeIncidents'),
      value: MOCK_METRICS.activeIncidents,
      color: '#FF7D00',
      sub: t('bigScreen.metric.incidentSub', { processing: MOCK_METRICS.processingCount }),
    },
    {
      key: 'sla',
      icon: <SafetyCertificateOutlined />,
      label: t('bigScreen.metric.slaHealth'),
      value: `${MOCK_METRICS.slaRate}%`,
      color: '#00B42A',
      sub: t('bigScreen.metric.slaSub', { target: MOCK_METRICS.slaTarget }),
    },
    {
      key: 'assets',
      icon: <CloudServerOutlined />,
      label: t('bigScreen.metric.assetOnline'),
      value: MOCK_METRICS.assetOnline,
      color: '#3491FA',
      sub: t('bigScreen.metric.assetSub', { total: MOCK_METRICS.assetTotal }),
    },
    {
      key: 'logs',
      icon: <FileTextOutlined />,
      label: t('bigScreen.metric.logThroughput'),
      value: MOCK_METRICS.logThroughput,
      color: '#722ED1',
      sub: t('bigScreen.metric.logSub'),
    },
  ];

  return (
    <div
      style={{
        background: 'linear-gradient(135deg, #0F1923 0%, #1A2332 50%, #0D1520 100%)',
        minHeight: '100vh',
        padding: '24px 32px',
        color: '#E5E6EB',
      }}
    >
      {/* Header */}
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: 32,
        padding: '0 8px',
      }}>
        <Title level={2} style={{ color: '#E5E6EB', margin: 0 }}>
          {t('bigScreen.title')}
        </Title>
        <Space size="large">
          <Tag color="#2E75B6" style={{ fontSize: 14, padding: '4px 12px' }}>
            <ClockCircleOutlined style={{ marginRight: 6 }} />
            {currentTime.toLocaleString()}
          </Tag>
        </Space>
      </div>

      {/* Metric Cards */}
      <Row gutter={[24, 24]} style={{ marginBottom: 32 }}>
        {metrics.map((m) => (
          <Col span={Math.floor(24 / metrics.length)} key={m.key}>
            <MetricBlock
              icon={m.icon}
              label={m.label}
              value={m.value}
              color={m.color}
              sub={m.sub}
            />
          </Col>
        ))}
      </Row>

      {/* Charts Area */}
      <Row gutter={16} style={{ marginTop: 24 }}>
        <Col span={16}>
          <Card
            title={<span className="bigscreen-chart-title">24小时告警趋势</span>}
            style={{ background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.08)', borderRadius: 12 }}
            headStyle={{ borderBottom: '1px solid rgba(255,255,255,0.08)' }}
          >
            <div ref={alertTrendRef} style={{ height: 220, width: '100%' }} />
          </Card>
        </Col>
        <Col span={8}>
          <Card
            title={<span className="bigscreen-chart-title">事件状态分布</span>}
            style={{ background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.08)', borderRadius: 12 }}
            headStyle={{ borderBottom: '1px solid rgba(255,255,255,0.08)' }}
            bodyStyle={{ height: 220, padding: '12px 16px' }}
          >
            <IncidentStatusRing />
          </Card>
        </Col>
      </Row>

      {/* Bottom Row: TOP 告警资产 / 事件时间线 / 日志量趋势 */}
      <Row gutter={[24, 24]} style={{ marginTop: 24 }}>
        <Col span={8}>
          <Card
            title={<Text style={{ color: '#E5E6EB' }}>{t('bigScreen.chart.topAlertAssets')}</Text>}
            style={{
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(255,255,255,0.1)',
              borderRadius: 12,
            }}
            headStyle={{ borderBottom: '1px solid rgba(255,255,255,0.1)' }}
            bodyStyle={{ minHeight: 200, padding: '16px 20px' }}
          >
            <TopAlertAssets />
          </Card>
        </Col>
        <Col span={8}>
          <Card
            title={<Text style={{ color: '#E5E6EB' }}>{t('bigScreen.chart.incidentTimeline')}</Text>}
            style={{
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(255,255,255,0.1)',
              borderRadius: 12,
            }}
            headStyle={{ borderBottom: '1px solid rgba(255,255,255,0.1)' }}
            bodyStyle={{ minHeight: 200, padding: '16px 20px' }}
          >
            <IncidentTimeline />
          </Card>
        </Col>
        <Col span={8}>
          <Card
            title={<Text style={{ color: '#E5E6EB' }}>{t('bigScreen.chart.logVolume')}</Text>}
            style={{
              background: 'rgba(255,255,255,0.04)',
              border: '1px solid rgba(255,255,255,0.1)',
              borderRadius: 12,
            }}
            headStyle={{ borderBottom: '1px solid rgba(255,255,255,0.1)' }}
            bodyStyle={{ minHeight: 200, padding: '8px 0' }}
          >
            <LogVolumeChart />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default BigScreen;
