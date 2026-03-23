import React, { useState, useEffect, useCallback } from 'react';
import { Typography, Card, Row, Col, Select, Spin, Empty } from 'antd';
import { useTranslation } from 'react-i18next';
import { getLogVolumeTrend, getLogLevelDistribution, type LogVolumePoint, type LogLevelDistribution } from '../api/log';

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

/**
 * 日志量趋势折线图组件（轻量 SVG 实现）
 * 展示时间序列上的日志数量变化，包含面积填充和数据点标记
 * @param data - 时间序列数据点数组
 */
const VolumeChart: React.FC<{ data: LogVolumePoint[] }> = ({ data }) => {
  if (data.length === 0) return <Empty />;

  // SVG 画布尺寸和内边距
  const width = 600;
  const height = 200;
  const padding = { top: 20, right: 20, bottom: 30, left: 50 };
  const chartW = width - padding.left - padding.right;
  const chartH = height - padding.top - padding.bottom;

  // 计算数据点的 SVG 坐标
  const maxCount = Math.max(...data.map((d) => d.count), 1);
  const points = data.map((d, i) => ({
    x: padding.left + (i / Math.max(data.length - 1, 1)) * chartW,
    y: padding.top + chartH - (d.count / maxCount) * chartH,
  }));
  const pathD = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x},${p.y}`).join(' ');
  const areaD = `${pathD} L${points[points.length - 1].x},${padding.top + chartH} L${points[0].x},${padding.top + chartH} Z`;

  return (
    <svg viewBox={`0 0 ${width} ${height}`} style={{ width: '100%', maxHeight: 240 }}>
      <defs>
        <linearGradient id="volumeGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#3491FA" stopOpacity={0.3} />
          <stop offset="100%" stopColor="#3491FA" stopOpacity={0.02} />
        </linearGradient>
      </defs>
      {/* Y axis labels */}
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
      {/* X axis labels */}
      {data.length > 0 && [0, Math.floor(data.length / 2), data.length - 1].map((idx) => (
        <text
          key={idx}
          x={padding.left + (idx / Math.max(data.length - 1, 1)) * chartW}
          y={height - 5}
          textAnchor="middle"
          fontSize={10}
          fill="#86909C"
        >
          {data[idx].time}
        </text>
      ))}
      <path d={areaD} fill="url(#volumeGrad)" />
      <path d={pathD} fill="none" stroke="#3491FA" strokeWidth={2} />
      {points.map((p, i) => (
        <circle key={i} cx={p.x} cy={p.y} r={2.5} fill="#3491FA" />
      ))}
    </svg>
  );
};

/**
 * 日志级别分布饼图组件（轻量 SVG 实现）
 * 展示各级别（ERROR/WARN/INFO/DEBUG）日志的占比分布
 * @param data - 各级别日志数量数组
 */
const LevelPieChart: React.FC<{ data: LogLevelDistribution[] }> = ({ data }) => {
  if (data.length === 0) return <Empty />;

  const totalCount = data.reduce((s, d) => s + d.count, 0);
  if (totalCount === 0) return <Empty />;

  const size = 200;
  const cx = size / 2;
  const cy = size / 2;
  const radius = 70;

  // 从 12 点钟方向（-90 度）开始绘制扇区
  let startAngle = -Math.PI / 2;
  const slices = data.map((d) => {
    const angle = (d.count / totalCount) * 2 * Math.PI;
    const start = startAngle;
    const end = startAngle + angle;
    startAngle = end;
    const largeArc = angle > Math.PI ? 1 : 0;
    const x1 = cx + radius * Math.cos(start);
    const y1 = cy + radius * Math.sin(start);
    const x2 = cx + radius * Math.cos(end);
    const y2 = cy + radius * Math.sin(end);
    return {
      level: d.level,
      count: d.count,
      pct: ((d.count / totalCount) * 100).toFixed(1),
      path: `M${cx},${cy} L${x1},${y1} A${radius},${radius} 0 ${largeArc},1 ${x2},${y2} Z`,
      color: LOG_LEVEL_COLORS[d.level] || '#86909C',
    };
  });

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 32 }}>
      <svg viewBox={`0 0 ${size} ${size}`} style={{ width: 200, height: 200 }}>
        {slices.map((s) => (
          <path key={s.level} d={s.path} fill={s.color} stroke="#fff" strokeWidth={1} />
        ))}
      </svg>
      <div>
        {slices.map((s) => (
          <div key={s.level} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
            <span style={{ display: 'inline-block', width: 12, height: 12, borderRadius: 2, background: s.color }} />
            <span style={{ fontSize: 14 }}>{s.level}</span>
            <span style={{ color: '#86909C', fontSize: 13 }}>{s.count} ({s.pct}%)</span>
          </div>
        ))}
      </div>
    </div>
  );
};

/**
 * 生成模拟日志量趋势数据（后端 API 不可用时使用）
 * 生成最近 24 小时的模拟数据点
 */
function mockVolumeTrend(): LogVolumePoint[] {
  const points: LogVolumePoint[] = [];
  const now = Date.now();
  for (let i = 23; i >= 0; i--) {
    const t = new Date(now - i * 3600_000);
    points.push({
      time: `${t.getHours().toString().padStart(2, '0')}:00`,
      count: Math.floor(Math.random() * 5000) + 500,
    });
  }
  return points;
}

/** 生成模拟日志级别分布数据（后端 API 不可用时使用） */
function mockLevelDistribution(): LogLevelDistribution[] {
  return [
    { level: 'ERROR', count: Math.floor(Math.random() * 200) + 50 },
    { level: 'WARN', count: Math.floor(Math.random() * 500) + 200 },
    { level: 'INFO', count: Math.floor(Math.random() * 3000) + 1000 },
    { level: 'DEBUG', count: Math.floor(Math.random() * 1000) + 300 },
  ];
}

/**
 * 日志分析页面组件
 * 功能：日志量趋势折线图 + 日志级别分布饼图，支持按时间范围切换
 */
const LogAnalysis: React.FC = () => {
  const { t } = useTranslation('log');
  const [timePreset, setTimePreset] = useState('24h');                         // 当前时间范围选项
  const [volumeData, setVolumeData] = useState<LogVolumePoint[]>([]);          // 日志量趋势数据
  const [levelData, setLevelData] = useState<LogLevelDistribution[]>([]);      // 日志级别分布数据
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
      setVolumeData(mockVolumeTrend());
      setLevelData(mockLevelDistribution());
    } finally {
      setLoading(false);
    }
  }, [timePreset]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return (
    <div>
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
        <Row gutter={16}>
          <Col span={16}>
            <Card
              title={t('analysis.volume')}
              style={{ borderRadius: 8, marginBottom: 16 }}
              bodyStyle={{ padding: 16 }}
            >
              <VolumeChart data={volumeData} />
            </Card>
          </Col>
          <Col span={8}>
            <Card
              title={t('analysis.dimension.level')}
              style={{ borderRadius: 8, marginBottom: 16 }}
              bodyStyle={{ padding: 16 }}
            >
              <LevelPieChart data={levelData} />
            </Card>
          </Col>
        </Row>
      </Spin>
    </div>
  );
};

export default LogAnalysis;
