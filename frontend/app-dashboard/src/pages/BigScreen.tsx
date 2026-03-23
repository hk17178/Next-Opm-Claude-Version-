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
      value: '--',
      color: '#F53F3F',
      sub: t('bigScreen.metric.alertSub', { p0: 0, p1: 0 }),
    },
    {
      key: 'incidents',
      icon: <ThunderboltOutlined />,
      label: t('bigScreen.metric.activeIncidents'),
      value: '--',
      color: '#FF7D00',
      sub: t('bigScreen.metric.incidentSub', { processing: 0 }),
    },
    {
      key: 'sla',
      icon: <SafetyCertificateOutlined />,
      label: t('bigScreen.metric.slaHealth'),
      value: '--',
      color: '#00B42A',
      sub: t('bigScreen.metric.slaSub', { target: '99.95%' }),
    },
    {
      key: 'assets',
      icon: <CloudServerOutlined />,
      label: t('bigScreen.metric.assetOnline'),
      value: '--',
      color: '#3491FA',
      sub: t('bigScreen.metric.assetSub', { total: 0 }),
    },
    {
      key: 'logs',
      icon: <FileTextOutlined />,
      label: t('bigScreen.metric.logThroughput'),
      value: '--',
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
          >
            <div id="bigscreen-incident-pie" className="bigscreen-incident-pie-container">
              {/* 饼图占位，可后续用 ECharts 渲染 */}
              <div className="bigscreen-incident-pie-placeholder">
                <div className="bigscreen-incident-pie-count">5</div>
                <div className="bigscreen-incident-pie-label">活跃事件</div>
              </div>
            </div>
          </Card>
        </Col>
      </Row>

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
            bodyStyle={{ minHeight: 200 }}
          >
            <div style={{ textAlign: 'center', color: 'rgba(255,255,255,0.35)', padding: 60 }}>
              {t('bigScreen.chart.placeholder')}
            </div>
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
            bodyStyle={{ minHeight: 200 }}
          >
            <div style={{ textAlign: 'center', color: 'rgba(255,255,255,0.35)', padding: 60 }}>
              {t('bigScreen.chart.placeholder')}
            </div>
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
            bodyStyle={{ minHeight: 200 }}
          >
            <div style={{ textAlign: 'center', color: 'rgba(255,255,255,0.35)', padding: 60 }}>
              {t('bigScreen.chart.placeholder')}
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default BigScreen;
