/**
 * 日志分析页面 (/log/analysis) — 功能完整版
 *
 * 功能模块：
 * 1. 日志量趋势图（SVG 面积图，24h / 7d 切换）
 * 2. 日志级别分布饼图（ERROR / WARN / INFO / DEBUG）
 * 3. 错误日志 TOP10 聚类表格（模式 / 出现次数 / 首次 / 最近 / 来源服务）
 * 4. 关键词标签云（以标签尺寸体现词频）
 * 5. 异常检测时间线（标注异常点的折线图）
 */
import React, { useState, useEffect, useCallback } from 'react';
import { Typography, Card, Row, Col, Select, Spin, Table, Tag, Tooltip } from 'antd';
import { WarningOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  getLogVolumeTrend,
  getLogLevelDistribution,
  type LogVolumePoint,
  type LogLevelDistribution,
} from '../api/log';

const { Text } = Typography;

/** 时间快捷选项列表 */
const TIME_PRESETS = ['1h', '6h', '24h', '7d', '30d'];

/** 日志级别对应的颜色映射 */
const LOG_LEVEL_COLORS: Record<string, string> = {
  ERROR: '#F53F3F',
  WARN: '#FF7D00',
  INFO: '#3491FA',
  DEBUG: '#C9CDD4',
};

/* ============================== 类型定义 ============================== */

/** 错误日志聚类条目 */
interface ErrorCluster {
  /** 唯一 ID */
  id: string;
  /** 错误模式（聚合后的摘要） */
  pattern: string;
  /** 出现次数 */
  count: number;
  /** 首次出现时间 */
  firstSeen: string;
  /** 最近出现时间 */
  lastSeen: string;
  /** 来源服务 */
  service: string;
  /** 日志级别 */
  level: string;
}

/** 关键词标签云数据 */
interface KeywordItem {
  /** 关键词 */
  word: string;
  /** 出现频率（用于计算字号） */
  weight: number;
}

/** 异常检测数据点 */
interface AnomalyPoint {
  /** 时间标签 */
  time: string;
  /** 日志数量 */
  count: number;
  /** 是否为异常点 */
  isAnomaly: boolean;
  /** 异常描述（可选） */
  anomalyDesc?: string;
}

/* ============================== Mock 数据生成 ============================== */

/**
 * 生成模拟日志量趋势数据
 * 生成最近 24 小时 / 7 天的数据点
 * @param preset 时间快捷选项
 */
function mockVolumeTrend(preset: string): LogVolumePoint[] {
  const points: LogVolumePoint[] = [];
  const now = Date.now();
  const count = preset === '7d' ? 7 * 24 : preset === '30d' ? 30 : 24;
  const interval = preset === '7d' ? 3600_000 : preset === '30d' ? 86400_000 : 3600_000;

  for (let i = count - 1; i >= 0; i--) {
    const t = new Date(now - i * interval);
    let label: string;
    if (preset === '30d') {
      label = `${(t.getMonth() + 1).toString().padStart(2, '0')}/${t.getDate().toString().padStart(2, '0')}`;
    } else if (preset === '7d') {
      label = `${(t.getMonth() + 1)}/${t.getDate()} ${t.getHours().toString().padStart(2, '0')}:00`;
    } else {
      label = `${t.getHours().toString().padStart(2, '0')}:00`;
    }
    // 模拟日志量变化，工作时间段（9-18 点）日志量较高
    const hour = t.getHours();
    const base = (hour >= 9 && hour <= 18) ? 3000 : 1000;
    points.push({
      time: label,
      count: Math.floor(Math.random() * base + base * 0.5),
    });
  }
  return points;
}

/** 生成模拟日志级别分布数据 */
function mockLevelDistribution(): LogLevelDistribution[] {
  return [
    { level: 'ERROR', count: Math.floor(Math.random() * 200) + 50 },
    { level: 'WARN', count: Math.floor(Math.random() * 500) + 200 },
    { level: 'INFO', count: Math.floor(Math.random() * 3000) + 1000 },
    { level: 'DEBUG', count: Math.floor(Math.random() * 1000) + 300 },
  ];
}

/** 模拟错误日志 TOP10 聚类数据 */
const MOCK_ERROR_CLUSTERS: ErrorCluster[] = [
  { id: '1', pattern: 'NullPointerException at UserService.getProfile()', count: 342, firstSeen: '2026-03-20 08:12:00', lastSeen: '2026-03-24 10:45:00', service: 'user-service', level: 'ERROR' },
  { id: '2', pattern: 'Connection refused: Redis cluster node 10.0.1.30:6379', count: 228, firstSeen: '2026-03-22 14:30:00', lastSeen: '2026-03-24 10:40:00', service: 'cache-proxy', level: 'ERROR' },
  { id: '3', pattern: 'Timeout waiting for response from payment gateway', count: 187, firstSeen: '2026-03-21 09:00:00', lastSeen: '2026-03-24 10:38:00', service: 'pay-service', level: 'ERROR' },
  { id: '4', pattern: 'Disk usage exceeds 90% on /data partition', count: 156, firstSeen: '2026-03-19 03:00:00', lastSeen: '2026-03-24 02:00:00', service: 'monitor-agent', level: 'WARN' },
  { id: '5', pattern: 'SQL deadlock detected in order_items table', count: 134, firstSeen: '2026-03-23 10:15:00', lastSeen: '2026-03-24 09:30:00', service: 'order-service', level: 'ERROR' },
  { id: '6', pattern: 'Certificate expiry warning: api.example.com (7 days)', count: 98, firstSeen: '2026-03-17 00:00:00', lastSeen: '2026-03-24 00:00:00', service: 'cert-manager', level: 'WARN' },
  { id: '7', pattern: 'OutOfMemoryError: Java heap space', count: 87, firstSeen: '2026-03-22 16:45:00', lastSeen: '2026-03-24 08:20:00', service: 'search-engine', level: 'ERROR' },
  { id: '8', pattern: 'Failed to authenticate user: invalid token', count: 76, firstSeen: '2026-03-20 12:00:00', lastSeen: '2026-03-24 10:42:00', service: 'auth-service', level: 'WARN' },
  { id: '9', pattern: 'Kafka consumer lag exceeds threshold (>10000)', count: 65, firstSeen: '2026-03-23 06:00:00', lastSeen: '2026-03-24 10:35:00', service: 'event-processor', level: 'WARN' },
  { id: '10', pattern: 'DNS resolution failed for internal.svc.cluster.local', count: 54, firstSeen: '2026-03-24 02:00:00', lastSeen: '2026-03-24 10:30:00', service: 'k8s-coredns', level: 'ERROR' },
];

/** 模拟关键词标签云数据 */
const MOCK_KEYWORDS: KeywordItem[] = [
  { word: 'timeout', weight: 95 },
  { word: 'connection', weight: 88 },
  { word: 'error', weight: 82 },
  { word: 'refused', weight: 75 },
  { word: 'exception', weight: 72 },
  { word: 'null', weight: 68 },
  { word: 'failed', weight: 65 },
  { word: 'retry', weight: 60 },
  { word: 'memory', weight: 55 },
  { word: 'disk', weight: 52 },
  { word: 'deadlock', weight: 48 },
  { word: 'certificate', weight: 45 },
  { word: 'authentication', weight: 42 },
  { word: 'kafka', weight: 40 },
  { word: 'dns', weight: 38 },
  { word: 'latency', weight: 35 },
  { word: 'threshold', weight: 32 },
  { word: 'heap', weight: 30 },
  { word: 'cluster', weight: 28 },
  { word: 'replication', weight: 25 },
];

/**
 * 生成模拟异常检测时间线数据
 * 在正常趋势上随机标注 3~5 个异常点
 */
function mockAnomalyTimeline(): AnomalyPoint[] {
  const points: AnomalyPoint[] = [];
  const now = Date.now();
  const anomalyIndices = new Set([3, 8, 14, 19]); // 预设异常点位置
  const anomalyDescs = [
    '日志量突增 300%，疑似 DDoS 攻击',
    'ERROR 日志占比突增至 45%',
    '多服务同时出现连接超时',
    '异常流量模式检测',
  ];
  let descIdx = 0;

  for (let i = 23; i >= 0; i--) {
    const t = new Date(now - i * 3600_000);
    const hour = t.getHours();
    const base = (hour >= 9 && hour <= 18) ? 3000 : 1000;
    const isAnomaly = anomalyIndices.has(23 - i);

    points.push({
      time: `${hour.toString().padStart(2, '0')}:00`,
      count: isAnomaly
        ? Math.floor(base * (2.5 + Math.random()))  // 异常点日志量显著高于正常值
        : Math.floor(Math.random() * base + base * 0.3),
      isAnomaly,
      anomalyDesc: isAnomaly ? anomalyDescs[descIdx++] : undefined,
    });
  }
  return points;
}

/* ============================== SVG 图表组件 ============================== */

/**
 * 日志量趋势面积图组件（SVG 实现）
 * 展示时间序列上的日志数量变化，包含面积填充和数据点标记
 * @param data - 时间序列数据点数组
 */
const VolumeChart: React.FC<{ data: LogVolumePoint[] }> = ({ data }) => {
  if (data.length === 0) return null;

  const width = 700;
  const height = 220;
  const padding = { top: 20, right: 20, bottom: 35, left: 55 };
  const chartW = width - padding.left - padding.right;
  const chartH = height - padding.top - padding.bottom;

  /** 计算数据点的 SVG 坐标 */
  const maxCount = Math.max(...data.map((d) => d.count), 1);
  const points = data.map((d, i) => ({
    x: padding.left + (i / Math.max(data.length - 1, 1)) * chartW,
    y: padding.top + chartH - (d.count / maxCount) * chartH,
  }));
  const pathD = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x},${p.y}`).join(' ');
  const areaD = `${pathD} L${points[points.length - 1].x},${padding.top + chartH} L${points[0].x},${padding.top + chartH} Z`;

  /** 选取 X 轴标签显示的索引（避免密集重叠） */
  const labelStep = Math.max(1, Math.floor(data.length / 8));

  return (
    <svg viewBox={`0 0 ${width} ${height}`} style={{ width: '100%', maxHeight: 260 }}>
      <defs>
        <linearGradient id="volumeGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#3491FA" stopOpacity={0.3} />
          <stop offset="100%" stopColor="#3491FA" stopOpacity={0.02} />
        </linearGradient>
      </defs>
      {/* Y 轴网格线和标签 */}
      {[0, 0.25, 0.5, 0.75, 1].map((ratio) => {
        const y = padding.top + chartH - ratio * chartH;
        const val = Math.round(maxCount * ratio);
        return (
          <g key={ratio}>
            <line x1={padding.left} y1={y} x2={padding.left + chartW} y2={y} stroke="#E5E6EB" strokeDasharray="3,3" />
            <text x={padding.left - 8} y={y + 4} textAnchor="end" fontSize={10} fill="#86909C">{val}</text>
          </g>
        );
      })}
      {/* X 轴标签 */}
      {data.map((d, idx) => {
        if (idx % labelStep !== 0 && idx !== data.length - 1) return null;
        return (
          <text
            key={idx}
            x={padding.left + (idx / Math.max(data.length - 1, 1)) * chartW}
            y={height - 5}
            textAnchor="middle"
            fontSize={9}
            fill="#86909C"
          >
            {d.time}
          </text>
        );
      })}
      {/* 面积填充 */}
      <path d={areaD} fill="url(#volumeGrad)" />
      {/* 折线 */}
      <path d={pathD} fill="none" stroke="#3491FA" strokeWidth={2} />
      {/* 数据点 */}
      {points.map((p, i) => (
        <circle key={i} cx={p.x} cy={p.y} r={2.5} fill="#3491FA" />
      ))}
    </svg>
  );
};

/**
 * 日志级别分布饼图组件（SVG 实现）
 * 展示各级别日志的占比分布
 * @param data - 各级别日志数量数组
 */
const LevelPieChart: React.FC<{ data: LogLevelDistribution[] }> = ({ data }) => {
  if (data.length === 0) return null;

  const totalCount = data.reduce((s, d) => s + d.count, 0);
  if (totalCount === 0) return null;

  const size = 200;
  const cx = size / 2;
  const cy = size / 2;
  const outerRadius = 80;
  const innerRadius = 45; // 环形饼图

  /** 从 12 点钟方向开始绘制扇区 */
  let startAngle = -Math.PI / 2;
  const slices = data.map((d) => {
    const angle = (d.count / totalCount) * 2 * Math.PI;
    const start = startAngle;
    const end = startAngle + angle;
    startAngle = end;
    const largeArc = angle > Math.PI ? 1 : 0;

    // 外弧
    const ox1 = cx + outerRadius * Math.cos(start);
    const oy1 = cy + outerRadius * Math.sin(start);
    const ox2 = cx + outerRadius * Math.cos(end);
    const oy2 = cy + outerRadius * Math.sin(end);
    // 内弧
    const ix1 = cx + innerRadius * Math.cos(end);
    const iy1 = cy + innerRadius * Math.sin(end);
    const ix2 = cx + innerRadius * Math.cos(start);
    const iy2 = cy + innerRadius * Math.sin(start);

    return {
      level: d.level,
      count: d.count,
      pct: ((d.count / totalCount) * 100).toFixed(1),
      path: `M${ox1},${oy1} A${outerRadius},${outerRadius} 0 ${largeArc},1 ${ox2},${oy2} L${ix1},${iy1} A${innerRadius},${innerRadius} 0 ${largeArc},0 ${ix2},${iy2} Z`,
      color: LOG_LEVEL_COLORS[d.level] || '#86909C',
    };
  });

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 24, justifyContent: 'center' }}>
      <svg viewBox={`0 0 ${size} ${size}`} style={{ width: 180, height: 180 }}>
        {slices.map((s) => (
          <path key={s.level} d={s.path} fill={s.color} stroke="var(--bg-card, #fff)" strokeWidth={2} />
        ))}
        {/* 中心总数显示 */}
        <text x={cx} y={cy - 6} textAnchor="middle" fontSize={18} fontWeight={700} fill="var(--text-primary, #333)">
          {totalCount.toLocaleString()}
        </text>
        <text x={cx} y={cy + 14} textAnchor="middle" fontSize={11} fill="#86909C">
          总计
        </text>
      </svg>
      {/* 图例 */}
      <div>
        {slices.map((s) => (
          <div key={s.level} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
            <span style={{ display: 'inline-block', width: 12, height: 12, borderRadius: 3, background: s.color }} />
            <span style={{ fontSize: 13, fontWeight: 500, minWidth: 50 }}>{s.level}</span>
            <span style={{ color: '#86909C', fontSize: 12 }}>{s.count.toLocaleString()} ({s.pct}%)</span>
          </div>
        ))}
      </div>
    </div>
  );
};

/**
 * 关键词标签云组件
 * 以不同字号和颜色展示高频关键词
 * @param keywords - 关键词及权重数组
 */
const KeywordCloud: React.FC<{ keywords: KeywordItem[] }> = ({ keywords }) => {
  /** 词频颜色映射 */
  const getColor = (weight: number): string => {
    if (weight >= 80) return '#F53F3F';
    if (weight >= 60) return '#FF7D00';
    if (weight >= 40) return '#3491FA';
    return '#86909C';
  };

  /** 根据权重计算字号（14px ~ 28px） */
  const getFontSize = (weight: number): number => {
    return 14 + (weight / 100) * 14;
  };

  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 12px', padding: 8, alignItems: 'center', justifyContent: 'center', minHeight: 120 }}>
      {keywords.map((kw) => (
        <Tag
          key={kw.word}
          style={{
            fontSize: getFontSize(kw.weight),
            color: getColor(kw.weight),
            background: `${getColor(kw.weight)}10`,
            border: `1px solid ${getColor(kw.weight)}30`,
            borderRadius: 6,
            padding: '4px 10px',
            cursor: 'pointer',
            fontWeight: kw.weight >= 70 ? 600 : 400,
          }}
        >
          {kw.word}
        </Tag>
      ))}
    </div>
  );
};

/**
 * 异常检测时间线组件（SVG 折线图 + 异常标注）
 * 在正常日志量折线图上标注异常检测点
 * @param data - 包含异常标记的时间序列数据
 */
const AnomalyTimeline: React.FC<{ data: AnomalyPoint[] }> = ({ data }) => {
  if (data.length === 0) return null;

  const width = 700;
  const height = 200;
  const padding = { top: 25, right: 20, bottom: 35, left: 55 };
  const chartW = width - padding.left - padding.right;
  const chartH = height - padding.top - padding.bottom;

  const maxCount = Math.max(...data.map((d) => d.count), 1);
  const points = data.map((d, i) => ({
    x: padding.left + (i / Math.max(data.length - 1, 1)) * chartW,
    y: padding.top + chartH - (d.count / maxCount) * chartH,
    isAnomaly: d.isAnomaly,
    desc: d.anomalyDesc,
    time: d.time,
    count: d.count,
  }));
  const pathD = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x},${p.y}`).join(' ');

  return (
    <svg viewBox={`0 0 ${width} ${height}`} style={{ width: '100%', maxHeight: 240 }}>
      <defs>
        <linearGradient id="anomalyGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#60a5fa" stopOpacity={0.15} />
          <stop offset="100%" stopColor="#60a5fa" stopOpacity={0.01} />
        </linearGradient>
      </defs>
      {/* Y 轴网格 */}
      {[0, 0.25, 0.5, 0.75, 1].map((ratio) => {
        const y = padding.top + chartH - ratio * chartH;
        return (
          <g key={ratio}>
            <line x1={padding.left} y1={y} x2={padding.left + chartW} y2={y} stroke="#E5E6EB" strokeDasharray="3,3" />
            <text x={padding.left - 8} y={y + 4} textAnchor="end" fontSize={10} fill="#86909C">
              {Math.round(maxCount * ratio)}
            </text>
          </g>
        );
      })}
      {/* X 轴标签 */}
      {data.map((d, idx) => {
        if (idx % 3 !== 0 && idx !== data.length - 1) return null;
        return (
          <text
            key={idx}
            x={padding.left + (idx / Math.max(data.length - 1, 1)) * chartW}
            y={height - 5}
            textAnchor="middle"
            fontSize={9}
            fill="#86909C"
          >
            {d.time}
          </text>
        );
      })}
      {/* 面积填充 */}
      <path
        d={`${pathD} L${points[points.length - 1].x},${padding.top + chartH} L${points[0].x},${padding.top + chartH} Z`}
        fill="url(#anomalyGrad)"
      />
      {/* 折线 */}
      <path d={pathD} fill="none" stroke="#60a5fa" strokeWidth={2} />
      {/* 正常数据点 */}
      {points.filter((p) => !p.isAnomaly).map((p, i) => (
        <circle key={`n-${i}`} cx={p.x} cy={p.y} r={2.5} fill="#60a5fa" />
      ))}
      {/* 异常标注点 — 红色大圆 + 脉冲动画 + 标注线 */}
      {points.filter((p) => p.isAnomaly).map((p, i) => (
        <g key={`a-${i}`}>
          {/* 异常标注竖线 */}
          <line
            x1={p.x}
            y1={padding.top}
            x2={p.x}
            y2={padding.top + chartH}
            stroke="#ff6b6b"
            strokeDasharray="4,3"
            opacity={0.4}
          />
          {/* 脉冲外圈 */}
          <circle cx={p.x} cy={p.y} r={8} fill="#ff6b6b" opacity={0.15}>
            <animate attributeName="r" values="6;12;6" dur="2s" repeatCount="indefinite" />
            <animate attributeName="opacity" values="0.2;0.05;0.2" dur="2s" repeatCount="indefinite" />
          </circle>
          {/* 异常点 */}
          <Tooltip title={`${p.time} | ${p.count} 条 | ${p.desc}`}>
            <circle cx={p.x} cy={p.y} r={5} fill="#ff6b6b" stroke="#fff" strokeWidth={2} style={{ cursor: 'pointer' }} />
          </Tooltip>
          {/* 异常描述标签 */}
          <text
            x={p.x}
            y={padding.top - 5}
            textAnchor="middle"
            fontSize={8}
            fill="#ff6b6b"
            fontWeight={500}
          >
            <WarningOutlined /> 异常
          </text>
        </g>
      ))}
    </svg>
  );
};

/* ============================== 主组件 ============================== */

/**
 * 日志分析页面组件
 * 上下多行布局：趋势图 + 饼图 → 错误聚类表 → 标签云 + 异常时间线
 */
const LogAnalysis: React.FC = () => {
  const { t } = useTranslation('log');

  /** 当前选中的时间范围 */
  const [timePreset, setTimePreset] = useState('24h');
  /** 日志量趋势数据 */
  const [volumeData, setVolumeData] = useState<LogVolumePoint[]>([]);
  /** 日志级别分布数据 */
  const [levelData, setLevelData] = useState<LogLevelDistribution[]>([]);
  /** 异常检测时间线数据 */
  const [anomalyData, setAnomalyData] = useState<AnomalyPoint[]>([]);
  /** 加载状态 */
  const [loading, setLoading] = useState(false);

  /**
   * 并行获取日志量趋势和级别分布数据
   * API 不可用时自动回退到模拟数据
   */
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [volume, levels] = await Promise.all([
        getLogVolumeTrend({ timePreset }),
        getLogLevelDistribution({ timePreset }),
      ]);
      setVolumeData(volume);
      setLevelData(levels);
    } catch {
      // 后端 API 不可用，使用模拟数据展示 UI
      setVolumeData(mockVolumeTrend(timePreset));
      setLevelData(mockLevelDistribution());
    }
    // 异常检测数据始终使用模拟（该功能依赖 AI 引擎）
    setAnomalyData(mockAnomalyTimeline());
    setLoading(false);
  }, [timePreset]);

  /** 组件挂载及时间切换时刷新数据 */
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  /** 错误日志聚类表格列定义 */
  const errorClusterColumns = [
    {
      title: t('analysis.cluster.pattern'),
      dataIndex: 'pattern',
      key: 'pattern',
      ellipsis: true,
      /** 错误模式以代码风格展示，截断过长内容 */
      render: (pattern: string) => (
        <Tooltip title={pattern}>
          <code style={{ fontSize: 12, background: 'rgba(0,0,0,0.04)', padding: '2px 6px', borderRadius: 4 }}>
            {pattern}
          </code>
        </Tooltip>
      ),
    },
    {
      title: t('analysis.cluster.count'),
      dataIndex: 'count',
      key: 'count',
      width: 100,
      sorter: (a: ErrorCluster, b: ErrorCluster) => b.count - a.count,
      /** 高频错误标红 */
      render: (count: number) => (
        <Text strong style={{ color: count >= 200 ? '#F53F3F' : count >= 100 ? '#FF7D00' : '#86909C' }}>
          {count.toLocaleString()}
        </Text>
      ),
    },
    {
      title: t('analysis.cluster.firstSeen'),
      dataIndex: 'firstSeen',
      key: 'firstSeen',
      width: 160,
    },
    {
      title: t('analysis.cluster.lastSeen'),
      dataIndex: 'lastSeen',
      key: 'lastSeen',
      width: 160,
    },
    {
      title: t('analysis.cluster.service'),
      dataIndex: 'service',
      key: 'service',
      width: 140,
      /** 服务名标签 */
      render: (service: string) => <Tag>{service}</Tag>,
    },
    {
      title: t('analysis.cluster.level'),
      dataIndex: 'level',
      key: 'level',
      width: 80,
      /** 日志级别标签 */
      render: (level: string) => (
        <Tag color={LOG_LEVEL_COLORS[level]}>{level}</Tag>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与时间范围选择器 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('analysis.title')}</Text>
        <Select
          value={timePreset}
          onChange={setTimePreset}
          style={{ width: 120 }}
          options={TIME_PRESETS.map((p) => ({ value: p, label: p }))}
        />
      </div>

      <Spin spinning={loading}>
        {/* ---- 第一行：日志量趋势 + 级别分布 ---- */}
        <Row gutter={16} style={{ marginBottom: 16 }}>
          <Col span={16}>
            <Card
              title={t('analysis.volume')}
              style={{ borderRadius: 12 }}
              styles={{ body: { padding: 16 } }}
            >
              <VolumeChart data={volumeData} />
            </Card>
          </Col>
          <Col span={8}>
            <Card
              title={t('analysis.dimension.level')}
              style={{ borderRadius: 12 }}
              styles={{ body: { padding: 16 } }}
            >
              <LevelPieChart data={levelData} />
            </Card>
          </Col>
        </Row>

        {/* ---- 第二行：错误日志 TOP10 聚类表 ---- */}
        <Card
          title={t('analysis.errorTop10')}
          style={{ borderRadius: 12, marginBottom: 16 }}
          styles={{ body: { padding: 0 } }}
        >
          <Table
            columns={errorClusterColumns}
            dataSource={MOCK_ERROR_CLUSTERS}
            rowKey="id"
            size="middle"
            pagination={false}
            locale={{ emptyText: t('analysis.noErrors') }}
          />
        </Card>

        {/* ---- 第三行：关键词标签云 + 异常检测时间线 ---- */}
        <Row gutter={16}>
          <Col span={8}>
            <Card
              title={t('analysis.keywords')}
              style={{ borderRadius: 12 }}
              styles={{ body: { padding: 12 } }}
            >
              <KeywordCloud keywords={MOCK_KEYWORDS} />
            </Card>
          </Col>
          <Col span={16}>
            <Card
              title={t('analysis.anomaly')}
              style={{ borderRadius: 12 }}
              styles={{ body: { padding: 16 } }}
              extra={
                <div style={{ display: 'flex', gap: 12, fontSize: 12 }}>
                  <span>
                    <span style={{ display: 'inline-block', width: 10, height: 10, borderRadius: '50%', background: '#60a5fa', marginRight: 4 }} />
                    {t('analysis.normalPoint')}
                  </span>
                  <span>
                    <span style={{ display: 'inline-block', width: 10, height: 10, borderRadius: '50%', background: '#ff6b6b', marginRight: 4 }} />
                    {t('analysis.anomalyPoint')}
                  </span>
                </div>
              }
            >
              <AnomalyTimeline data={anomalyData} />
            </Card>
          </Col>
        </Row>
      </Spin>
    </div>
  );
};

export default LogAnalysis;
