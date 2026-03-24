/**
 * NOC 运维大屏页面 - 专为 NOC（网络运营中心）设计的全屏监控大屏
 *
 * 布局结构：
 * ┌─────────────── 全局状态条（实时刷新）────────────────┐
 * │  系统健康: ● 正常  |  活跃事件: 3  |  今日告警: 47  |  SLA: 99.92%  │
 * ├──────────────────────────────────────────────────────────────┤
 * │ 告警瀑布流（左25%）  │  业务健康矩阵（中40%）  │  资源曲线（右35%） │
 * ├──────────────────────────────────────────────────────────────┤
 * │            事件驾驶舱精简版（全宽）                             │
 * └──────────────────────────────────────────────────────────────┘
 */
import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Tag, Typography, Table, Tooltip } from 'antd';
import {
  AlertOutlined, ThunderboltOutlined, SafetyCertificateOutlined,
  ClockCircleOutlined, CheckCircleOutlined, WarningOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type {
  SystemStatus, BusinessHealthCell, ResourceMetricPoint,
  RealtimeAlert, ActiveIncident,
} from '../api/noc';
import {
  getSystemStatus, getBusinessHealthMatrix, getResourceMetrics,
  getRealtimeAlerts, getActiveIncidents,
} from '../api/noc';
import './NOCScreen.css';

const { Text } = Typography;

/* ========== 颜色常量定义 ========== */

/** 健康状态 → 色块颜色映射（状态指示色，保持固定不随主题变化） */
const HEALTH_COLORS: Record<string, string> = {
  healthy: '#238636',
  warning: '#D29922',
  degraded: '#DB6D28',
  critical: '#F85149',
};

/** 严重级别颜色映射 */
const SEVERITY_COLORS: Record<string, string> = {
  P0: '#F85149', P1: '#DB6D28', P2: '#D29922', P3: '#8B949E',
};

/** 事件状态颜色映射 */
const STATUS_COLORS: Record<string, string> = {
  open: '#F85149',
  acknowledged: '#D29922',
  processing: '#DB6D28',
  resolved: '#238636',
};

/* ========== 模拟数据生成（后端 API 不可用时使用） ========== */

function mockSystemStatus(): SystemStatus {
  return { health: 'normal', activeIncidents: 3, todayAlerts: 47, slaRate: 99.92, onlineServices: 156, totalServices: 160 };
}

function mockBusinessHealth(): BusinessHealthCell[] {
  const businesses = [
    '支付网关', '用户中心', '订单系统', '库存服务', '物流追踪', '消息推送',
    '风控引擎', '数据分析', '搜索服务', '推荐引擎', '内容管理', '客服系统',
    'API 网关', '认证授权', '文件存储', '缓存集群', '日志平台', '监控告警',
    '配置中心', '注册发现', '链路追踪', '负载均衡', 'CDN 加速', '数据库集群',
  ];
  return businesses.map((name, i) => ({
    name,
    score: Math.floor(Math.random() * 40) + 60,
    status: i < 16 ? 'healthy' : i < 20 ? 'warning' : i < 23 ? 'degraded' : 'critical',
    activeAlerts: i < 16 ? 0 : i < 20 ? 1 : i < 23 ? 2 : 3,
  }));
}

function mockResourceMetrics(): ResourceMetricPoint[] {
  const points: ResourceMetricPoint[] = [];
  const now = Date.now();
  for (let i = 29; i >= 0; i--) {
    const ts = new Date(now - i * 60_000);
    points.push({
      timestamp: `${ts.getHours().toString().padStart(2, '0')}:${ts.getMinutes().toString().padStart(2, '0')}`,
      cpu: 40 + Math.random() * 35,
      memory: 55 + Math.random() * 25,
      network: 100 + Math.random() * 200,
    });
  }
  return points;
}

function mockRealtimeAlerts(): RealtimeAlert[] {
  const alerts = [
    { severity: 'P0', content: 'prod-db-master-01 主从复制延迟 > 60s', source: 'MySQL Monitor' },
    { severity: 'P1', content: 'payment-api 错误率突增至 3.2%', source: 'Prometheus' },
    { severity: 'P2', content: 'cache-cluster-03 内存使用率 > 85%', source: 'Redis Exporter' },
    { severity: 'P1', content: 'k8s-node-07 CPU 使用率持续 > 90%', source: 'Node Exporter' },
    { severity: 'P3', content: 'SSL 证书将在 14 天后过期', source: 'Cert Manager' },
    { severity: 'P2', content: 'order-service Pod 重启次数 > 5', source: 'Kubernetes' },
    { severity: 'P0', content: 'CDN 回源带宽异常增长 200%', source: 'CloudWatch' },
    { severity: 'P1', content: 'ElasticSearch 集群磁盘使用率 > 80%', source: 'ES Exporter' },
  ];
  return alerts.map((a, i) => ({
    ...a,
    id: `alert-noc-${i}`,
    triggerTime: new Date(Date.now() - i * 120_000).toLocaleTimeString(),
    status: i < 2 ? 'firing' : 'acknowledged',
  }));
}

function mockActiveIncidents(): ActiveIncident[] {
  return [
    { id: '1', incidentId: 'INC-20260323-001', title: '支付系统响应延迟', severity: 'P0', handler: '张工', duration: '45min', status: 'processing' },
    { id: '2', incidentId: 'INC-20260323-002', title: '数据库主从切换异常', severity: 'P0', handler: '李工', duration: '32min', status: 'acknowledged' },
    { id: '3', incidentId: 'INC-20260323-003', title: 'CDN 节点故障导致部分区域访问慢', severity: 'P1', handler: '王工', duration: '1h 15min', status: 'processing' },
  ];
}

/* ========== 自定义 Hook：模拟 WebSocket 实时推送 ========== */

function useRealtimeAlerts(intervalMs = 5000): RealtimeAlert[] {
  const [alerts, setAlerts] = useState<RealtimeAlert[]>([]);

  useEffect(() => {
    const loadAlerts = async () => {
      try {
        const data = await getRealtimeAlerts();
        setAlerts(data);
      } catch {
        setAlerts(mockRealtimeAlerts());
      }
    };
    loadAlerts();
    const timer = setInterval(loadAlerts, intervalMs);
    return () => clearInterval(timer);
  }, [intervalMs]);

  return alerts;
}

/* ========== 子组件 ========== */

/** 告警瀑布流组件 */
const AlertWaterfall: React.FC<{ alerts: RealtimeAlert[] }> = ({ alerts }) => {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [alerts]);

  return (
    <div ref={containerRef} className="noc-waterfall">
      {alerts.map((alert, index) => (
        <div
          key={alert.id}
          className="noc-alert-item"
          style={{
            borderLeft: `3px solid ${SEVERITY_COLORS[alert.severity] || '#8B949E'}`,
            animationDelay: `${index * 0.05}s`,
          }}
        >
          <div className="noc-alert-row">
            <Tag
              style={{
                background: SEVERITY_COLORS[alert.severity],
                color: '#fff',
                border: 'none',
                fontSize: '0.7vw',
                padding: '0 6px',
                lineHeight: '18px',
              }}
            >
              {alert.severity}
            </Tag>
            <Text className="noc-alert-time">{alert.triggerTime}</Text>
          </div>
          <div className="noc-alert-content">{alert.content}</div>
          <div className="noc-alert-source">{alert.source}</div>
        </div>
      ))}
    </div>
  );
};

/** 业务健康矩阵组件 */
const BusinessHealthMatrix: React.FC<{ data: BusinessHealthCell[] }> = ({ data }) => (
  <div className="noc-health-grid">
    {data.slice(0, 24).map((cell) => (
      <Tooltip
        key={cell.name}
        title={
          <div>
            <div className="noc-tooltip-title">{cell.name}</div>
            <div>健康度: {cell.score}%</div>
            <div>活跃告警: {cell.activeAlerts}</div>
          </div>
        }
      >
        <div
          className="noc-health-cell"
          style={{ background: HEALTH_COLORS[cell.status] || '#238636' }}
          onMouseEnter={(e) => {
            (e.currentTarget as HTMLDivElement).style.transform = 'scale(1.05)';
            (e.currentTarget as HTMLDivElement).style.boxShadow = `0 0 12px ${HEALTH_COLORS[cell.status]}80`;
          }}
          onMouseLeave={(e) => {
            (e.currentTarget as HTMLDivElement).style.transform = 'scale(1)';
            (e.currentTarget as HTMLDivElement).style.boxShadow = 'none';
          }}
        >
          <span className="noc-health-cell-label">{cell.name}</span>
        </div>
      </Tooltip>
    ))}
  </div>
);

/** 资源曲线 SVG 折线图组件 */
const ResourceChart: React.FC<{ data: ResourceMetricPoint[] }> = ({ data }) => {
  if (data.length === 0) return null;

  const width = 600;
  const height = 280;
  const pad = { top: 20, right: 20, bottom: 30, left: 45 };
  const chartW = width - pad.left - pad.right;
  const chartH = height - pad.top - pad.bottom;

  const mapPoints = (values: number[], maxVal: number) =>
    values.map((v, i) => ({
      x: pad.left + (i / Math.max(data.length - 1, 1)) * chartW,
      y: pad.top + chartH - (v / maxVal) * chartH,
    }));

  const toPathD = (pts: Array<{ x: number; y: number }>) =>
    pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ');

  const cpuPoints = mapPoints(data.map((d) => d.cpu), 100);
  const memPoints = mapPoints(data.map((d) => d.memory), 100);
  const netMax = Math.max(...data.map((d) => d.network), 1);
  const netPoints = mapPoints(data.map((d) => d.network), netMax);

  const lines = [
    { label: 'CPU',  color: '#F85149', path: toPathD(cpuPoints) },
    { label: '内存', color: '#58A6FF', path: toPathD(memPoints) },
    { label: '网络', color: '#3FB950', path: toPathD(netPoints) },
  ];

  const xLabelIndices = data.map((_, i) => i).filter((i) => i % 5 === 0 || i === data.length - 1);

  return (
    <div className="noc-resource-chart-wrap">
      <div className="noc-chart-legend">
        {lines.map((l) => (
          <span key={l.label} className="noc-chart-legend-item" style={{ color: l.color }}>
            <span className="noc-chart-swatch" style={{ background: l.color }} />
            {l.label}
          </span>
        ))}
      </div>
      <svg viewBox={`0 0 ${width} ${height}`} className="noc-resource-svg">
        {[0, 25, 50, 75, 100].map((val) => {
          const y = pad.top + chartH - (val / 100) * chartH;
          return (
            <g key={val}>
              <line x1={pad.left} y1={y} x2={pad.left + chartW} y2={y} className="noc-svg-grid-line" strokeDasharray="3,3" />
              <text x={pad.left - 8} y={y + 4} textAnchor="end" fontSize={10} className="noc-svg-axis-text">{val}%</text>
            </g>
          );
        })}
        {xLabelIndices.map((idx) => (
          <text
            key={idx}
            x={pad.left + (idx / Math.max(data.length - 1, 1)) * chartW}
            y={height - 6}
            textAnchor="middle"
            fontSize={9}
            className="noc-svg-axis-text"
          >
            {data[idx].timestamp}
          </text>
        ))}
        {lines.map((l) => (
          <path key={l.label} d={l.path} fill="none" stroke={l.color} strokeWidth={2} strokeLinejoin="round" />
        ))}
      </svg>
    </div>
  );
};

/* ========== 滚动字幕组件 ========== */

const marqueeAlerts = [
  { text: '[P0] server-pay-01 不可达 - payment 集群', color: '#ff6b6b' },
  { text: '[P1] CPU 95.3% 超阈值 - db-master-02', color: '#ffaa33' },
  { text: '[P0] CDN 回源带宽异常增长 200% - CloudWatch', color: '#ff6b6b' },
  { text: '[P1] ElasticSearch 集群磁盘使用率 > 80%', color: '#ffaa33' },
  { text: '[P0] prod-db-master-01 主从复制延迟 > 60s', color: '#ff6b6b' },
  { text: '[P1] payment-api 错误率突增至 3.2%', color: '#ffaa33' },
  { text: '[P1] k8s-node-07 CPU 使用率持续 > 90%', color: '#ffaa33' },
  { text: '[P0] 支付网关 P99 响应时间 > 5s', color: '#ff6b6b' },
];

const MARQUEE_KEYFRAMES_ID = 'noc-marquee-keyframes';

const MarqueeBar: React.FC = () => {
  const containerRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // 注入 keyframes 到 document.head（仅一次）
    if (!document.getElementById(MARQUEE_KEYFRAMES_ID)) {
      const style = document.createElement('style');
      style.id = MARQUEE_KEYFRAMES_ID;
      style.textContent = `
        @keyframes nocMarqueeScroll {
          0% { transform: translateX(0); }
          100% { transform: translateX(-50%); }
        }
      `;
      document.head.appendChild(style);
    }
    return () => {
      const el = document.getElementById(MARQUEE_KEYFRAMES_ID);
      if (el) el.remove();
    };
  }, []);

  const marqueeContent = marqueeAlerts.map((a) => a.text).join('');
  // 根据内容长度动态计算动画时长，保持匀速
  const duration = Math.max(20, marqueeContent.length * 0.4);

  return (
    <div ref={containerRef} className="noc-marquee-bar">
      <div
        ref={contentRef}
        className="noc-marquee-track"
        style={{ animation: `nocMarqueeScroll ${duration}s linear infinite` }}
      >
        {/* 渲染两份内容实现无缝滚动 */}
        {[0, 1].map((round) => (
          <div key={round} className="noc-marquee-group">
            {marqueeAlerts.map((alert, idx) => (
              <span
                key={`${round}-${idx}`}
                className="noc-marquee-item"
                style={{ color: alert.color, textShadow: `0 0 6px ${alert.color}40` }}
              >
                {alert.text}
              </span>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
};

/* ========== 主组件 ========== */

const NOCScreen: React.FC = () => {
  const { t } = useTranslation('dashboard');
  const [currentTime, setCurrentTime] = useState(new Date());
  const [systemStatus, setSystemStatus] = useState<SystemStatus | null>(null);
  const [healthMatrix, setHealthMatrix] = useState<BusinessHealthCell[]>([]);
  const [resourceData, setResourceData] = useState<ResourceMetricPoint[]>([]);
  const [activeIncidents, setActiveIncidents] = useState<ActiveIncident[]>([]);

  const realtimeAlerts = useRealtimeAlerts(5000);

  useEffect(() => {
    const timer = setInterval(() => setCurrentTime(new Date()), 1000);
    return () => clearInterval(timer);
  }, []);

  const loadData = useCallback(async () => {
    try {
      const [status, health, metrics, incidents] = await Promise.all([
        getSystemStatus(), getBusinessHealthMatrix(), getResourceMetrics(), getActiveIncidents(),
      ]);
      setSystemStatus(status);
      setHealthMatrix(health);
      setResourceData(metrics);
      setActiveIncidents(incidents);
    } catch {
      setSystemStatus(mockSystemStatus());
      setHealthMatrix(mockBusinessHealth());
      setResourceData(mockResourceMetrics());
      setActiveIncidents(mockActiveIncidents());
    }
  }, []);

  useEffect(() => {
    loadData();
    const timer = setInterval(loadData, 30_000);
    return () => clearInterval(timer);
  }, [loadData]);

  const healthIcon = systemStatus?.health === 'normal'
    ? <CheckCircleOutlined style={{ color: '#3FB950' }} />
    : systemStatus?.health === 'degraded'
      ? <WarningOutlined style={{ color: '#D29922' }} />
      : <ExclamationCircleOutlined style={{ color: '#F85149' }} />;

  const healthLabel = systemStatus?.health === 'normal' ? '正常'
    : systemStatus?.health === 'degraded' ? '降级' : '严重';
  const healthColor = systemStatus?.health === 'normal' ? '#3FB950' : '#F85149';

  const incidentColumns = [
    {
      title: '级别',
      dataIndex: 'severity',
      key: 'severity',
      width: 70,
      render: (severity: string) => (
        <Tag style={{ background: SEVERITY_COLORS[severity], color: '#fff', border: 'none', fontWeight: 600 }}>
          {severity}
        </Tag>
      ),
    },
    {
      title: '事件编号',
      dataIndex: 'incidentId',
      key: 'incidentId',
      width: 180,
      render: (text: string) => <span className="noc-cell-link">{text}</span>,
    },
    {
      title: '事件标题',
      dataIndex: 'title',
      key: 'title',
      ellipsis: true,
      render: (text: string) => <span className="noc-cell-primary">{text}</span>,
    },
    {
      title: '处理人',
      dataIndex: 'handler',
      key: 'handler',
      width: 100,
      render: (text: string) => <span className="noc-cell-primary">{text}</span>,
    },
    {
      title: '持续时长',
      dataIndex: 'duration',
      key: 'duration',
      width: 120,
      render: (text: string) => <span className="noc-cell-duration">{text}</span>,
    },
    {
      title: '当前状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => (
        <Tag style={{ background: STATUS_COLORS[status] || '#8B949E', color: '#fff', border: 'none' }}>
          {status === 'processing' ? '处理中' : status === 'acknowledged' ? '已确认' : status}
        </Tag>
      ),
    },
  ];

  return (
    <div className="noc-root">
      {/* ==================== 全局状态条 ==================== */}
      <div className="noc-status-bar">
        <div className="noc-status-left">
          <span className="noc-status-title">OpsNexus NOC</span>
          <span className="noc-status-item">
            系统健康: {healthIcon}{' '}
            <span className="noc-status-value" style={{ color: healthColor }}>{healthLabel}</span>
          </span>
          <span className="noc-status-item">
            <ThunderboltOutlined style={{ color: '#DB6D28', marginRight: 4 }} />
            活跃事件: <span className="noc-status-value">{systemStatus?.activeIncidents ?? 0}</span>
          </span>
          <span className="noc-status-item">
            <AlertOutlined style={{ color: '#F85149', marginRight: 4 }} />
            今日告警: <span className="noc-status-value">{systemStatus?.todayAlerts ?? 0}</span>
          </span>
          <span className="noc-status-item">
            <SafetyCertificateOutlined style={{ color: '#3FB950', marginRight: 4 }} />
            SLA: <span className="noc-status-value">{systemStatus?.slaRate ?? '--'}%</span>
          </span>
        </div>
        <div className="noc-status-right">
          <Tag className="noc-service-tag">
            {systemStatus?.onlineServices ?? 0}/{systemStatus?.totalServices ?? 0} 服务在线
          </Tag>
          <span className="noc-clock">
            <ClockCircleOutlined style={{ marginRight: 4 }} />
            {currentTime.toLocaleString()}
          </span>
        </div>
      </div>

      {/* ==================== 三栏主内容区 ==================== */}
      <div className="noc-main-grid">
        {/* ===== 左侧 25%：告警瀑布流 ===== */}
        <div className="noc-panel">
          <div className="noc-panel-header">
            <span className="noc-panel-title">
              <AlertOutlined style={{ marginRight: 6, color: '#F85149' }} />
              告警瀑布流
            </span>
            <span className="noc-live-badge">LIVE</span>
          </div>
          <div className="noc-panel-body">
            <AlertWaterfall alerts={realtimeAlerts} />
          </div>
        </div>

        {/* ===== 中间 40%：业务健康矩阵 ===== */}
        <div className="noc-panel">
          <div className="noc-panel-header">
            <span className="noc-panel-title">业务健康矩阵</span>
            <div className="noc-legend-row">
              {Object.entries(HEALTH_COLORS).map(([key, color]) => (
                <span key={key} className="noc-legend-item">
                  <span className="noc-legend-dot" style={{ background: color }} />
                  {key === 'healthy' ? '健康' : key === 'warning' ? '警告' : key === 'degraded' ? '降级' : '严重'}
                </span>
              ))}
            </div>
          </div>
          <div className="noc-panel-body">
            <BusinessHealthMatrix data={healthMatrix} />
          </div>
        </div>

        {/* ===== 右侧 35%：资源曲线 ===== */}
        <div className="noc-panel">
          <div className="noc-panel-header noc-panel-header--start">
            <span className="noc-panel-title">资源趋势曲线</span>
          </div>
          <div className="noc-panel-body">
            <ResourceChart data={resourceData} />
          </div>
        </div>
      </div>

      {/* ==================== 底部：事件驾驶舱精简版 ==================== */}
      <div className="noc-cockpit-panel">
        <div className="noc-panel-header">
          <span className="noc-panel-title">
            <ThunderboltOutlined style={{ marginRight: 6, color: '#DB6D28' }} />
            事件驾驶舱 - 活跃 P0/P1 事件
          </span>
          <span className="noc-cockpit-count">共 {activeIncidents.length} 个活跃事件</span>
        </div>
        <div className="noc-table">
          <Table
            columns={incidentColumns}
            dataSource={activeIncidents}
            rowKey="id"
            size="small"
            pagination={false}
            locale={{ emptyText: '当前无活跃 P0/P1 事件' }}
          />
        </div>
      </div>

      {/* ==================== 底部滚动字幕栏 ==================== */}
      <MarqueeBar />
    </div>
  );
};

export default NOCScreen;
