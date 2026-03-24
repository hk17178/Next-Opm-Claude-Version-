/**
 * 跨维度关联分析页面（页面 15）
 * 路由: /analytics/correlation
 *
 * 功能模块：
 * - 控制栏：业务线选择、时间范围、维度选择、图表类型切换
 * - 主图表区：支持叠加折线图、双Y柱线混合、散点图三种图表动态切换（ECharts）
 * - 右侧 AI 洞察面板（240px 宽，使用 AITypewriter 打字机效果输出分析结论）
 * - 维度对比选择器（CPU vs 告警数、延迟 vs 错误率等）
 *
 * 数据来源：Mock 数据（后端就绪后替换）
 */
import React, { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import * as echarts from 'echarts';
import {
  Card, Row, Col, Select, Space, Typography, Button, Checkbox, Tooltip, Tag,
} from 'antd';
import {
  LineChartOutlined, BarChartOutlined, DotChartOutlined,
  FullscreenOutlined, ExportOutlined, PlusOutlined,
  BulbOutlined, CheckOutlined, CloseOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { AITypewriter } from '@opsnexus/ui-kit';

const { Text, Title } = Typography;

/* ==================== 类型定义 ==================== */

/** 图表类型枚举 */
type ChartType = 'line' | 'bar-line' | 'scatter';

/** 维度对比选项 */
interface DimensionPair {
  key: string;
  labelX: string;
  labelY: string;
}

/* ==================== Mock 数据 ==================== */

/** 生成近 7 天日期标签 */
const generateDates = (days: number): string[] => {
  const dates: string[] = [];
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date();
    d.setDate(d.getDate() - i);
    dates.push(
      `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    );
  }
  return dates;
};

/** Mock: CPU 使用率数据（百分比） */
const mockCpuData = [65, 72, 58, 80, 75, 68, 71];

/** Mock: 告警数量数据 */
const mockAlertData = [12, 18, 8, 25, 20, 15, 17];

/** Mock: 延迟数据（ms） */
const mockLatencyData = [120, 145, 98, 180, 160, 130, 142];

/** Mock: 错误率数据（百分比） */
const mockErrorRateData = [0.5, 0.8, 0.3, 1.2, 1.0, 0.6, 0.7];

/** Mock: 内存使用率数据（百分比） */
const mockMemoryData = [55, 60, 52, 70, 65, 58, 62];

/** Mock: 事件数量数据 */
const mockIncidentData = [3, 5, 2, 8, 6, 4, 5];

/** 维度对比选项列表 */
const DIMENSION_PAIRS: DimensionPair[] = [
  { key: 'cpu-alert', labelX: 'CPU 使用率', labelY: '告警数' },
  { key: 'latency-error', labelX: '延迟 (ms)', labelY: '错误率 (%)' },
  { key: 'memory-incident', labelX: '内存使用率', labelY: '事件数' },
  { key: 'cpu-latency', labelX: 'CPU 使用率', labelY: '延迟 (ms)' },
];

/** 根据维度对获取对应的 Mock 数据 */
const getDimensionData = (pairKey: string): { dataX: number[]; dataY: number[] } => {
  switch (pairKey) {
    case 'cpu-alert':
      return { dataX: mockCpuData, dataY: mockAlertData };
    case 'latency-error':
      return { dataX: mockLatencyData, dataY: mockErrorRateData };
    case 'memory-incident':
      return { dataX: mockMemoryData, dataY: mockIncidentData };
    case 'cpu-latency':
      return { dataX: mockCpuData, dataY: mockLatencyData };
    default:
      return { dataX: mockCpuData, dataY: mockAlertData };
  }
};

/** AI 洞察文本 — 根据选中维度动态生成 */
const AI_INSIGHTS: Record<string, string> = {
  'cpu-alert':
    '分析发现：CPU 使用率与告警数量存在强正相关（r=0.87）。当 CPU 使用率超过 75% 时，告警数量显著增加。建议：\n\n1. 对 CPU 使用率超过 70% 的主机设置预警阈值\n2. 排查第 4 天 CPU 峰值（80%）对应的 25 条告警根因\n3. 考虑对高负载服务进行水平扩容',
  'latency-error':
    '分析发现：延迟与错误率呈显著正相关（r=0.92）。延迟超过 150ms 时错误率急剧上升。建议：\n\n1. 设置延迟 >140ms 的早期预警\n2. 排查第 4 天延迟峰值（180ms）的链路瓶颈\n3. 优化慢查询和连接池配置',
  'memory-incident':
    '分析发现：内存使用率与事件数量中度相关（r=0.73）。内存超过 65% 后事件概率明显上升。建议：\n\n1. 优化内存泄漏排查机制\n2. 设置内存 >60% 的梯度告警\n3. 检查 JVM 堆内存配置是否合理',
  'cpu-latency':
    '分析发现：CPU 使用率与服务延迟正相关（r=0.81）。CPU 负载升高直接导致响应时间增加。建议：\n\n1. 实施 CPU 限流保护策略\n2. 评估是否需要增加计算资源\n3. 优化热点代码路径减少 CPU 消耗',
};

/* ==================== 图表配置生成器 ==================== */

/**
 * 构建叠加折线图配置
 * 两条折线叠加展示，使用双 Y 轴
 */
const buildLineOption = (
  dates: string[],
  dataX: number[],
  dataY: number[],
  labelX: string,
  labelY: string,
): echarts.EChartsOption => ({
  tooltip: { trigger: 'axis' },
  legend: { data: [labelX, labelY], top: 4 },
  grid: { left: 60, right: 60, top: 40, bottom: 30 },
  xAxis: { type: 'category', data: dates },
  yAxis: [
    { type: 'value', name: labelX, position: 'left' },
    { type: 'value', name: labelY, position: 'right' },
  ],
  series: [
    {
      name: labelX,
      type: 'line',
      smooth: true,
      data: dataX,
      yAxisIndex: 0,
      lineStyle: { color: '#4da6ff', width: 2 },
      itemStyle: { color: '#4da6ff' },
      areaStyle: {
        color: {
          type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: 'rgba(77,166,255,0.25)' },
            { offset: 1, color: 'rgba(77,166,255,0)' },
          ],
        },
      },
    },
    {
      name: labelY,
      type: 'line',
      smooth: true,
      data: dataY,
      yAxisIndex: 1,
      lineStyle: { color: '#00e5a0', width: 2 },
      itemStyle: { color: '#00e5a0' },
      areaStyle: {
        color: {
          type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: 'rgba(0,229,160,0.2)' },
            { offset: 1, color: 'rgba(0,229,160,0)' },
          ],
        },
      },
    },
  ],
});

/**
 * 构建双Y柱线混合图配置
 * 左轴柱状图 + 右轴折线图
 */
const buildBarLineOption = (
  dates: string[],
  dataX: number[],
  dataY: number[],
  labelX: string,
  labelY: string,
): echarts.EChartsOption => ({
  tooltip: { trigger: 'axis' },
  legend: { data: [labelX, labelY], top: 4 },
  grid: { left: 60, right: 60, top: 40, bottom: 30 },
  xAxis: { type: 'category', data: dates },
  yAxis: [
    { type: 'value', name: labelX, position: 'left' },
    { type: 'value', name: labelY, position: 'right' },
  ],
  series: [
    {
      name: labelX,
      type: 'bar',
      data: dataX,
      yAxisIndex: 0,
      barMaxWidth: 30,
      itemStyle: {
        color: {
          type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [
            { offset: 0, color: '#4da6ff' },
            { offset: 1, color: 'rgba(77,166,255,0.3)' },
          ],
        },
        borderRadius: [4, 4, 0, 0],
      },
    },
    {
      name: labelY,
      type: 'line',
      smooth: true,
      data: dataY,
      yAxisIndex: 1,
      lineStyle: { color: '#ff6b6b', width: 2 },
      itemStyle: { color: '#ff6b6b' },
    },
  ],
});

/**
 * 构建散点图配置
 * X/Y 轴分别为两个维度的数值
 */
const buildScatterOption = (
  dataX: number[],
  dataY: number[],
  labelX: string,
  labelY: string,
): echarts.EChartsOption => ({
  tooltip: {
    trigger: 'item',
    formatter: (params: any) => `${labelX}: ${params.value[0]}<br/>${labelY}: ${params.value[1]}`,
  },
  grid: { left: 60, right: 30, top: 30, bottom: 40 },
  xAxis: { type: 'value', name: labelX, nameLocation: 'center', nameGap: 28 },
  yAxis: { type: 'value', name: labelY },
  series: [
    {
      type: 'scatter',
      data: dataX.map((x, i) => [x, dataY[i]]),
      symbolSize: 14,
      itemStyle: {
        color: {
          type: 'radial', x: 0.5, y: 0.5, r: 0.5,
          colorStops: [
            { offset: 0, color: 'rgba(77,166,255,0.9)' },
            { offset: 1, color: 'rgba(77,166,255,0.3)' },
          ],
        },
        shadowBlur: 8,
        shadowColor: 'rgba(77,166,255,0.4)',
      },
    },
  ],
});

/* ==================== 图表类型按钮配置 ==================== */

/** 图表类型切换按钮列表 */
const CHART_TYPES: { key: ChartType; icon: React.ReactNode; label: string }[] = [
  { key: 'line', icon: <LineChartOutlined />, label: '折线图' },
  { key: 'bar-line', icon: <BarChartOutlined />, label: '柱线混合' },
  { key: 'scatter', icon: <DotChartOutlined />, label: '散点图' },
];

/* ==================== 主组件 ==================== */

/**
 * 跨维度关联分析页面组件
 * 提供多维度指标对比分析能力，支持图表类型动态切换和 AI 智能洞察
 */
const CorrelationAnalysis: React.FC = () => {
  const { t } = useTranslation('analytics');

  /* ---------- 状态管理 ---------- */
  /** 当前业务线 */
  const [business, setBusiness] = useState<string>('all');
  /** 时间范围 */
  const [timeRange, setTimeRange] = useState<string>('7d');
  /** 当前选中的维度对 */
  const [dimensionPair, setDimensionPair] = useState<string>('cpu-alert');
  /** 当前图表类型 */
  const [chartType, setChartType] = useState<ChartType>('line');
  /** 是否显示周同比 */
  const [showWeekCompare, setShowWeekCompare] = useState(false);
  /** 是否显示月环比 */
  const [showMonthCompare, setShowMonthCompare] = useState(false);
  /** AI 洞察文本（用于打字机效果） */
  const [aiText, setAiText] = useState<string>('');
  /** AI 洞察是否已完成输出 */
  const [aiDone, setAiDone] = useState(false);

  /** ECharts 图表容器引用 */
  const chartRef = useRef<HTMLDivElement>(null);
  /** ECharts 实例引用 */
  const chartInstanceRef = useRef<echarts.ECharts | null>(null);

  /* ---------- 计算派生数据 ---------- */

  /** 日期标签 */
  const dates = useMemo(() => {
    const dayMap: Record<string, number> = { '24h': 1, '7d': 7, '30d': 30, '90d': 90 };
    return generateDates(dayMap[timeRange] || 7);
  }, [timeRange]);

  /** 当前维度对的配置信息 */
  const currentPair = useMemo(
    () => DIMENSION_PAIRS.find((p) => p.key === dimensionPair) || DIMENSION_PAIRS[0],
    [dimensionPair],
  );

  /** 当前维度对的 Mock 数据 */
  const { dataX, dataY } = useMemo(
    () => getDimensionData(dimensionPair),
    [dimensionPair],
  );

  /* ---------- 图表渲染 ---------- */

  /** 根据图表类型和维度数据构建 ECharts 配置 */
  const buildOption = useCallback((): echarts.EChartsOption => {
    switch (chartType) {
      case 'line':
        return buildLineOption(dates, dataX, dataY, currentPair.labelX, currentPair.labelY);
      case 'bar-line':
        return buildBarLineOption(dates, dataX, dataY, currentPair.labelX, currentPair.labelY);
      case 'scatter':
        return buildScatterOption(dataX, dataY, currentPair.labelX, currentPair.labelY);
      default:
        return buildLineOption(dates, dataX, dataY, currentPair.labelX, currentPair.labelY);
    }
  }, [chartType, dates, dataX, dataY, currentPair]);

  /** 初始化或更新 ECharts 图表 */
  useEffect(() => {
    if (!chartRef.current) return;

    // 如果实例不存在则初始化
    if (!chartInstanceRef.current) {
      chartInstanceRef.current = echarts.init(chartRef.current);
    }

    // 应用图表配置
    chartInstanceRef.current.setOption(buildOption(), true);

    // 窗口尺寸变化时自适应
    const handleResize = () => chartInstanceRef.current?.resize();
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
    };
  }, [buildOption]);

  /** 组件卸载时销毁图表实例 */
  useEffect(() => {
    return () => {
      chartInstanceRef.current?.dispose();
      chartInstanceRef.current = null;
    };
  }, []);

  /* ---------- AI 洞察 ---------- */

  /** 维度切换时重新生成 AI 洞察文本 */
  useEffect(() => {
    setAiDone(false);
    // 模拟 AI 分析延迟
    const timer = setTimeout(() => {
      setAiText(AI_INSIGHTS[dimensionPair] || AI_INSIGHTS['cpu-alert']);
    }, 500);
    return () => clearTimeout(timer);
  }, [dimensionPair]);

  /* ---------- 渲染 ---------- */

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('correlation.title')}</Text>
      </div>

      {/* 控制栏：业务线、时间范围、维度选择、图表类型切换 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
          {/* 左侧：筛选条件 */}
          <Space wrap>
            {/* 业务线选择 */}
            <Select
              value={business}
              onChange={setBusiness}
              style={{ width: 140 }}
              options={[
                { value: 'all', label: t('correlation.filter.allBusiness') },
                { value: 'payment', label: t('correlation.filter.payment') },
                { value: 'order', label: t('correlation.filter.order') },
                { value: 'user', label: t('correlation.filter.user') },
              ]}
            />
            {/* 时间范围选择 */}
            <Select
              value={timeRange}
              onChange={setTimeRange}
              style={{ width: 120 }}
              options={[
                { value: '24h', label: t('correlation.filter.24h') },
                { value: '7d', label: t('correlation.filter.7d') },
                { value: '30d', label: t('correlation.filter.30d') },
                { value: '90d', label: t('correlation.filter.90d') },
              ]}
            />
            {/* 维度对比选择 */}
            <Select
              value={dimensionPair}
              onChange={setDimensionPair}
              style={{ width: 200 }}
              options={DIMENSION_PAIRS.map((p) => ({
                value: p.key,
                label: `${p.labelX} vs ${p.labelY}`,
              }))}
            />
            {/* 对比开关 */}
            <Checkbox checked={showWeekCompare} onChange={(e) => setShowWeekCompare(e.target.checked)}>
              {t('correlation.filter.weekCompare')}
            </Checkbox>
            <Checkbox checked={showMonthCompare} onChange={(e) => setShowMonthCompare(e.target.checked)}>
              {t('correlation.filter.monthCompare')}
            </Checkbox>
          </Space>

          {/* 右侧：操作按钮 */}
          <Space>
            <Tooltip title={t('correlation.action.addMetric')}>
              <Button icon={<PlusOutlined />} />
            </Tooltip>
            <Tooltip title={t('correlation.action.export')}>
              <Button icon={<ExportOutlined />} />
            </Tooltip>
            <Tooltip title={t('correlation.action.fullscreen')}>
              <Button icon={<FullscreenOutlined />} />
            </Tooltip>
          </Space>
        </div>
      </Card>

      {/* 主内容区：图表 + AI 洞察面板 */}
      <Row gutter={16}>
        {/* 左侧：图表区域 */}
        <Col flex="1">
          <Card
            style={{ borderRadius: 8, minHeight: 440 }}
            title={
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span>{currentPair.labelX} vs {currentPair.labelY}</span>
                {/* 图表类型切换按钮组 */}
                <Space>
                  {CHART_TYPES.map((ct) => (
                    <Tooltip key={ct.key} title={ct.label}>
                      <Button
                        type={chartType === ct.key ? 'primary' : 'default'}
                        icon={ct.icon}
                        size="small"
                        onClick={() => setChartType(ct.key)}
                      />
                    </Tooltip>
                  ))}
                </Space>
              </div>
            }
          >
            {/* ECharts 图表容器 */}
            <div ref={chartRef} style={{ height: 360, width: '100%' }} />
          </Card>
        </Col>

        {/* 右侧：AI 洞察面板（固定宽度 240px） */}
        <Col flex="280px">
          <Card
            style={{
              borderRadius: 8,
              minHeight: 440,
              background: 'linear-gradient(135deg, rgba(77,166,255,0.03), rgba(0,229,160,0.03))',
            }}
            title={
              <Space>
                <BulbOutlined style={{ color: '#4da6ff' }} />
                <span>{t('correlation.aiInsight.title')}</span>
                <Tag color="blue">{t('correlation.aiInsight.tag')}</Tag>
              </Space>
            }
          >
            {/* AI 分析结果 - 使用打字机效果 */}
            <div style={{ fontSize: 13, lineHeight: 1.8, whiteSpace: 'pre-wrap', minHeight: 280 }}>
              {aiText ? (
                <AITypewriter
                  text={aiText}
                  speed={20}
                  onComplete={() => setAiDone(true)}
                />
              ) : (
                <Text type="secondary">{t('correlation.aiInsight.analyzing')}</Text>
              )}
            </div>

            {/* AI 建议操作按钮 */}
            {aiDone && (
              <div style={{ marginTop: 16, display: 'flex', gap: 8 }}>
                <Button type="primary" size="small" icon={<CheckOutlined />}>
                  {t('correlation.aiInsight.accept')}
                </Button>
                <Button size="small" icon={<CloseOutlined />}>
                  {t('correlation.aiInsight.dismiss')}
                </Button>
              </div>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default CorrelationAnalysis;
