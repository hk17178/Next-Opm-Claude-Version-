import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  Card, Row, Col, Typography, Table, Tag, Radio, DatePicker,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import * as echarts from 'echarts';
import { NoiseFunnel } from '@opsnexus/ui-kit';

const { Text } = Typography;
const { RangePicker } = DatePicker;

/* ================================================================== */
/*  类型定义                                                           */
/* ================================================================== */

/** 时间范围快捷选项类型 */
type TimeRange = '24h' | '7d' | '30d' | 'custom';

/** 误报 TOP10 规则表格行 */
interface FalsePositiveRule {
  key: string;
  /** 规则名称 */
  ruleName: string;
  /** 误报次数 */
  falseCount: number;
  /** 误报率（百分比） */
  falseRate: string;
  /** 优化建议 */
  suggestion: string;
}

/* ================================================================== */
/*  Mock 数据                                                          */
/* ================================================================== */

/** 生成 24 小时标签 */
const generateHourLabels = (): string[] =>
  Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:00`);

/** 生成 7 天日期标签 */
const generate7DayLabels = (): string[] => {
  const labels: string[] = [];
  for (let i = 6; i >= 0; i--) {
    const d = new Date();
    d.setDate(d.getDate() - i);
    labels.push(`${d.getMonth() + 1}/${d.getDate()}`);
  }
  return labels;
};

/** 告警趋势堆叠面积图数据（P0-P3 按级别堆叠） */
const TREND_DATA_24H = {
  labels: generateHourLabels(),
  P0: [2, 1, 0, 0, 1, 3, 5, 4, 2, 1, 0, 0, 1, 2, 3, 4, 6, 3, 2, 1, 0, 1, 2, 1],
  P1: [8, 6, 5, 4, 7, 12, 18, 15, 10, 8, 6, 5, 7, 9, 14, 16, 20, 14, 10, 7, 5, 6, 8, 7],
  P2: [15, 12, 10, 8, 14, 22, 30, 28, 20, 16, 12, 10, 14, 18, 25, 28, 35, 24, 18, 14, 10, 12, 15, 13],
  P3: [10, 8, 7, 5, 9, 15, 20, 18, 14, 10, 8, 7, 9, 12, 18, 20, 25, 16, 12, 9, 7, 8, 10, 9],
};

const TREND_DATA_7D = {
  labels: generate7DayLabels(),
  P0: [8, 12, 5, 15, 7, 3, 10],
  P1: [45, 62, 38, 78, 50, 28, 55],
  P2: [120, 145, 98, 180, 130, 85, 135],
  P3: [80, 95, 65, 110, 88, 60, 90],
};

/** 严重级别分布饼图数据 */
const SEVERITY_DIST = [
  { value: 62, name: 'P0', itemStyle: { color: '#F53F3F' } },
  { value: 274, name: 'P1', itemStyle: { color: '#FF7D00' } },
  { value: 536, name: 'P2', itemStyle: { color: '#3491FA' } },
  { value: 375, name: 'P3', itemStyle: { color: '#86909C' } },
];

/** 来源 TOP10 水平柱状图数据 */
const SOURCE_TOP10 = [
  { source: 'server-pay-01', count: 142, severity: 'P0' },
  { source: 'server-shop-03', count: 118, severity: 'P1' },
  { source: 'db-master-01', count: 95, severity: 'P1' },
  { source: 'redis-cluster-02', count: 87, severity: 'P2' },
  { source: 'kafka-broker-01', count: 76, severity: 'P2' },
  { source: 'nginx-lb-01', count: 65, severity: 'P2' },
  { source: 'server-risk-02', count: 54, severity: 'P2' },
  { source: 'es-node-03', count: 48, severity: 'P3' },
  { source: 'server-auth-01', count: 42, severity: 'P3' },
  { source: 'mq-consumer-05', count: 35, severity: 'P3' },
];

/** 降噪漏斗层级数据 */
const NOISE_FUNNEL_LAYERS = [
  { label: 'L0 铁律', value: 12 },
  { label: 'L1 静态', value: 35 },
  { label: 'L2 基线', value: 68 },
  { label: 'L3 趋势', value: 12 },
  { label: 'L4 AI', value: 13 },
  { label: '有效告警', value: 47 },
];

/** 误报 TOP10 规则表格 Mock 数据 */
const FALSE_POSITIVE_RULES: FalsePositiveRule[] = [
  { key: '1', ruleName: 'CPU 使用率 > 80%', falseCount: 45, falseRate: '32%', suggestion: '调整阈值至 85%' },
  { key: '2', ruleName: '磁盘 IO 延迟告警', falseCount: 38, falseRate: '28%', suggestion: '增加持续时间窗口至 5min' },
  { key: '3', ruleName: '内存使用超限', falseCount: 32, falseRate: '25%', suggestion: '启用动态基线替代固定阈值' },
  { key: '4', ruleName: '网络丢包率告警', falseCount: 28, falseRate: '22%', suggestion: '排除维护窗口时段' },
  { key: '5', ruleName: 'HTTP 5xx 错误率', falseCount: 24, falseRate: '18%', suggestion: '增加采样率至 10 次/分钟' },
  { key: '6', ruleName: 'Redis 连接数告警', falseCount: 20, falseRate: '16%', suggestion: '调整阈值至连接池 90%' },
  { key: '7', ruleName: 'Kafka 消费延迟', falseCount: 18, falseRate: '14%', suggestion: '增加容忍延迟至 30s' },
  { key: '8', ruleName: 'JVM GC 暂停时间', falseCount: 15, falseRate: '12%', suggestion: '排除 Full GC 后的短暂波动' },
  { key: '9', ruleName: '容器重启告警', falseCount: 12, falseRate: '10%', suggestion: '排除滚动更新场景' },
  { key: '10', ruleName: 'DNS 解析超时', falseCount: 10, falseRate: '8%', suggestion: '增加重试次数检测' },
];

/* ================================================================== */
/*  告警严重程度颜色常量                                                 */
/* ================================================================== */

/** 告警严重程度颜色映射（P0 最高 ~ P3 最低） */
const SEVERITY_COLORS: Record<string, string> = {
  P0: '#F53F3F',
  P1: '#FF7D00',
  P2: '#3491FA',
  P3: '#86909C',
};

/* ================================================================== */
/*  组件                                                               */
/* ================================================================== */

/**
 * 告警统计分析页面（页面 12）
 *
 * 功能模块：
 * 1. 时间选择器（24h/7天/30天/自定义）
 * 2. 4 张统计卡片（总告警数/有效率/平均确认时间 MTTA/规则覆盖率）
 * 3. 告警趋势堆叠面积图（ECharts，按 P0-P3 堆叠，X 轴时间）
 * 4. 严重级别分布环形饼图（ECharts）
 * 5. 告警来源 TOP10 水平柱状图（ECharts）
 * 6. 降噪漏斗展示（NoiseFunnel from @opsnexus/ui-kit）
 * 7. 误报 TOP10 规则表格
 */
const AlertAnalytics: React.FC = () => {
  const { t } = useTranslation('alert');

  // ---- 状态管理 ----
  /** 当前选中的时间范围 */
  const [timeRange, setTimeRange] = useState<TimeRange>('24h');
  /** 是否显示自定义日期范围选择器 */
  const [showCustomRange, setShowCustomRange] = useState(false);

  // ---- ECharts 图表引用 ----
  /** 告警趋势堆叠面积图容器 */
  const trendChartRef = useRef<HTMLDivElement>(null);
  const trendChartInstance = useRef<echarts.ECharts | null>(null);

  /** 严重级别分布环形饼图容器 */
  const pieChartRef = useRef<HTMLDivElement>(null);
  const pieChartInstance = useRef<echarts.ECharts | null>(null);

  /** 告警来源 TOP10 水平柱状图容器 */
  const barChartRef = useRef<HTMLDivElement>(null);
  const barChartInstance = useRef<echarts.ECharts | null>(null);

  /** 根据时间范围获取趋势数据 */
  const getTrendData = useCallback(() => {
    switch (timeRange) {
      case '7d':
        return TREND_DATA_7D;
      case '30d':
        // 30 天使用 7 天数据的倍数模拟
        return TREND_DATA_7D;
      default:
        return TREND_DATA_24H;
    }
  }, [timeRange]);

  /** 处理时间范围切换 */
  const handleTimeRangeChange = useCallback((value: TimeRange) => {
    setTimeRange(value);
    setShowCustomRange(value === 'custom');
  }, []);

  /* ---------- 告警趋势堆叠面积图初始化 ---------- */
  useEffect(() => {
    if (!trendChartRef.current) return;

    // 销毁已有实例，避免重复初始化
    if (trendChartInstance.current) {
      trendChartInstance.current.dispose();
    }

    const chart = echarts.init(trendChartRef.current);
    trendChartInstance.current = chart;

    const data = getTrendData();

    chart.setOption({
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(10,16,28,0.9)',
        borderColor: 'rgba(60,140,255,0.15)',
        textStyle: { color: '#e2e8f0', fontSize: 11 },
      },
      legend: {
        data: ['P0', 'P1', 'P2', 'P3'],
        top: 6,
        right: 14,
        textStyle: { color: 'rgba(140,170,210,0.5)', fontSize: 10 },
        itemWidth: 12,
        itemHeight: 3,
      },
      grid: { left: 40, right: 14, top: 40, bottom: 28 },
      xAxis: {
        type: 'category',
        data: data.labels,
        boundaryGap: false,
        axisLine: { lineStyle: { color: 'rgba(60,140,255,0.08)' } },
        axisLabel: { color: 'rgba(140,170,210,0.4)', fontSize: 9, interval: timeRange === '24h' ? 3 : 0 },
        axisTick: { show: false },
      },
      yAxis: {
        type: 'value',
        splitLine: { lineStyle: { color: 'rgba(60,140,255,0.05)' } },
        axisLine: { show: false },
        axisLabel: { color: 'rgba(140,170,210,0.4)', fontSize: 9 },
      },
      series: [
        {
          name: 'P0',
          type: 'line',
          stack: 'total',
          data: data.P0,
          smooth: true,
          symbol: 'none',
          lineStyle: { color: SEVERITY_COLORS.P0, width: 1.5 },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(245,63,63,0.3)' },
              { offset: 1, color: 'rgba(245,63,63,0.02)' },
            ]),
          },
        },
        {
          name: 'P1',
          type: 'line',
          stack: 'total',
          data: data.P1,
          smooth: true,
          symbol: 'none',
          lineStyle: { color: SEVERITY_COLORS.P1, width: 1.5 },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(255,125,0,0.25)' },
              { offset: 1, color: 'rgba(255,125,0,0.02)' },
            ]),
          },
        },
        {
          name: 'P2',
          type: 'line',
          stack: 'total',
          data: data.P2,
          smooth: true,
          symbol: 'none',
          lineStyle: { color: SEVERITY_COLORS.P2, width: 1.5 },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(52,145,250,0.25)' },
              { offset: 1, color: 'rgba(52,145,250,0.02)' },
            ]),
          },
        },
        {
          name: 'P3',
          type: 'line',
          stack: 'total',
          data: data.P3,
          smooth: true,
          symbol: 'none',
          lineStyle: { color: SEVERITY_COLORS.P3, width: 1.5 },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(134,144,156,0.2)' },
              { offset: 1, color: 'rgba(134,144,156,0.02)' },
            ]),
          },
        },
      ],
    });

    // 监听窗口 resize 自适应
    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      chart.dispose();
      trendChartInstance.current = null;
    };
  }, [timeRange, getTrendData]);

  /* ---------- 严重级别分布环形饼图初始化 ---------- */
  useEffect(() => {
    if (!pieChartRef.current) return;

    if (pieChartInstance.current) {
      pieChartInstance.current.dispose();
    }

    const chart = echarts.init(pieChartRef.current);
    pieChartInstance.current = chart;

    /** 计算告警总数（显示在饼图中心） */
    const total = SEVERITY_DIST.reduce((sum, item) => sum + item.value, 0);

    chart.setOption({
      tooltip: {
        trigger: 'item',
        backgroundColor: 'rgba(10,16,28,0.9)',
        borderColor: 'rgba(60,140,255,0.15)',
        textStyle: { color: '#e2e8f0', fontSize: 11 },
        formatter: '{b}: {c} ({d}%)',
      },
      legend: {
        orient: 'vertical',
        right: 10,
        top: 'center',
        textStyle: { color: 'rgba(140,170,210,0.6)', fontSize: 11 },
        itemWidth: 10,
        itemHeight: 10,
      },
      // 中心文字显示总数
      graphic: [
        {
          type: 'text',
          left: 'center',
          top: '42%',
          style: {
            text: String(total),
            fontSize: 24,
            fontWeight: 700,
            fill: 'var(--text-primary, #e2e8f0)',
            textAlign: 'center',
          },
        },
        {
          type: 'text',
          left: 'center',
          top: '56%',
          style: {
            text: t('analytics.totalAlerts'),
            fontSize: 11,
            fill: 'rgba(140,170,210,0.5)',
            textAlign: 'center',
          },
        },
      ],
      series: [
        {
          type: 'pie',
          radius: ['50%', '72%'],
          center: ['40%', '50%'],
          avoidLabelOverlap: false,
          label: { show: false },
          emphasis: {
            label: { show: true, fontSize: 12, fontWeight: 'bold' },
          },
          labelLine: { show: false },
          data: SEVERITY_DIST,
        },
      ],
    });

    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      chart.dispose();
      pieChartInstance.current = null;
    };
  }, [t]);

  /* ---------- 来源 TOP10 水平柱状图初始化 ---------- */
  useEffect(() => {
    if (!barChartRef.current) return;

    if (barChartInstance.current) {
      barChartInstance.current.dispose();
    }

    const chart = echarts.init(barChartRef.current);
    barChartInstance.current = chart;

    /** 反转数据使最大值在顶部 */
    const reversed = [...SOURCE_TOP10].reverse();

    chart.setOption({
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        backgroundColor: 'rgba(10,16,28,0.9)',
        borderColor: 'rgba(60,140,255,0.15)',
        textStyle: { color: '#e2e8f0', fontSize: 11 },
      },
      grid: { left: 120, right: 30, top: 10, bottom: 20 },
      xAxis: {
        type: 'value',
        splitLine: { lineStyle: { color: 'rgba(60,140,255,0.05)' } },
        axisLine: { show: false },
        axisLabel: { color: 'rgba(140,170,210,0.4)', fontSize: 9 },
      },
      yAxis: {
        type: 'category',
        data: reversed.map((item) => item.source),
        axisLine: { lineStyle: { color: 'rgba(60,140,255,0.08)' } },
        axisLabel: {
          color: 'rgba(140,170,210,0.6)',
          fontSize: 10,
          width: 110,
          overflow: 'truncate',
        },
        axisTick: { show: false },
      },
      series: [
        {
          type: 'bar',
          data: reversed.map((item) => ({
            value: item.count,
            itemStyle: {
              // P0 来源使用红色标注，其他使用品牌蓝
              color: item.severity === 'P0'
                ? new echarts.graphic.LinearGradient(0, 0, 1, 0, [
                    { offset: 0, color: 'rgba(245,63,63,0.6)' },
                    { offset: 1, color: 'rgba(245,63,63,0.2)' },
                  ])
                : new echarts.graphic.LinearGradient(0, 0, 1, 0, [
                    { offset: 0, color: 'rgba(77,166,255,0.6)' },
                    { offset: 1, color: 'rgba(77,166,255,0.2)' },
                  ]),
              borderRadius: [0, 3, 3, 0],
            },
          })),
          barWidth: 14,
          label: {
            show: true,
            position: 'right',
            color: 'rgba(140,170,210,0.6)',
            fontSize: 10,
          },
        },
      ],
    });

    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      chart.dispose();
      barChartInstance.current = null;
    };
  }, []);

  /* ---------- 统计卡片配置 ---------- */
  const statCards = [
    {
      key: 'totalAlerts',
      label: t('analytics.totalAlerts'),
      value: '1,247',
      color: '#4da6ff',
    },
    {
      key: 'effectiveRate',
      label: t('analytics.effectiveRate'),
      value: '73%',
      color: '#00e5a0',
    },
    {
      key: 'mtta',
      label: t('analytics.mtta'),
      value: '2.1min',
      color: '#ffaa33',
    },
    {
      key: 'ruleCoverage',
      label: t('analytics.ruleCoverage'),
      value: '89%',
      color: '#3491FA',
    },
  ];

  /* ---------- 误报 TOP10 表格列定义 ---------- */
  const falsePositiveColumns: ColumnsType<FalsePositiveRule> = [
    {
      title: t('analytics.fpTable.ruleName'),
      dataIndex: 'ruleName',
      key: 'ruleName',
      ellipsis: true,
    },
    {
      title: t('analytics.fpTable.falseCount'),
      dataIndex: 'falseCount',
      key: 'falseCount',
      width: 100,
      sorter: (a, b) => a.falseCount - b.falseCount,
      render: (val: number) => <Text strong style={{ color: '#F53F3F' }}>{val}</Text>,
    },
    {
      title: t('analytics.fpTable.falseRate'),
      dataIndex: 'falseRate',
      key: 'falseRate',
      width: 100,
      render: (val: string) => <Tag color="red">{val}</Tag>,
    },
    {
      title: t('analytics.fpTable.suggestion'),
      dataIndex: 'suggestion',
      key: 'suggestion',
      render: (val: string) => (
        <Tag color="blue" style={{ cursor: 'pointer' }}>{val}</Tag>
      ),
    },
  ];

  return (
    <div>
      {/* ---- 页面标题和时间选择器 ---- */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('analytics.title')}</Text>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          {/* 时间范围快捷选项 */}
          <Radio.Group
            value={timeRange}
            onChange={(e) => handleTimeRangeChange(e.target.value)}
            optionType="button"
            buttonStyle="solid"
            size="small"
          >
            <Radio.Button value="24h">{t('analytics.time.24h')}</Radio.Button>
            <Radio.Button value="7d">{t('analytics.time.7d')}</Radio.Button>
            <Radio.Button value="30d">{t('analytics.time.30d')}</Radio.Button>
            <Radio.Button value="custom">{t('analytics.time.custom')}</Radio.Button>
          </Radio.Group>
          {/* 自定义日期范围选择器 */}
          {showCustomRange && (
            <RangePicker size="small" />
          )}
        </div>
      </div>

      {/* ---- 4 张统计卡片 ---- */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {statCards.map((card) => (
          <Col span={6} key={card.key}>
            <Card
              bordered
              style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
              bodyStyle={{ padding: '16px 20px' }}
            >
              <div style={{ color: '#86909C', fontSize: 14 }}>{card.label}</div>
              <div style={{ fontSize: 28, fontWeight: 600, marginTop: 4, color: card.color }}>
                {card.value}
              </div>
            </Card>
          </Col>
        ))}
      </Row>

      {/* ---- 上半部分：告警趋势 + 级别分布 ---- */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {/* 左上：告警趋势堆叠面积图 */}
        <Col span={14}>
          <Card
            title={t('analytics.trendChart')}
            bordered
            style={{ borderRadius: 8 }}
            bodyStyle={{ padding: '8px 12px' }}
          >
            <div ref={trendChartRef} style={{ width: '100%', height: 300 }} />
          </Card>
        </Col>
        {/* 右上：严重级别分布环形饼图 */}
        <Col span={10}>
          <Card
            title={t('analytics.severityDist')}
            bordered
            style={{ borderRadius: 8 }}
            bodyStyle={{ padding: '8px 12px' }}
          >
            <div ref={pieChartRef} style={{ width: '100%', height: 300 }} />
          </Card>
        </Col>
      </Row>

      {/* ---- 下半部分：来源 TOP10 + 降噪漏斗 ---- */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {/* 左下：告警来源 TOP10 水平柱状图 */}
        <Col span={14}>
          <Card
            title={t('analytics.sourceTop10')}
            bordered
            style={{ borderRadius: 8 }}
            bodyStyle={{ padding: '8px 12px' }}
          >
            <div ref={barChartRef} style={{ width: '100%', height: 320 }} />
          </Card>
        </Col>
        {/* 右下：降噪漏斗可视化（纵向） */}
        <Col span={10}>
          <Card
            title={t('analytics.noiseFunnel')}
            bordered
            style={{ borderRadius: 8 }}
            bodyStyle={{ padding: '16px 12px' }}
          >
            {/* 使用 ui-kit 的 NoiseFunnel 组件展示降噪过程 */}
            <NoiseFunnel layers={NOISE_FUNNEL_LAYERS} height={6} />
            {/* 降噪效果统计文字 */}
            <div style={{ marginTop: 20, padding: '0 14px' }}>
              <Row gutter={16}>
                <Col span={8} style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 24, fontWeight: 700, color: '#4da6ff' }}>175</div>
                  <div style={{ fontSize: 12, color: '#86909C' }}>{t('analytics.funnel.rawAlerts')}</div>
                </Col>
                <Col span={8} style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 24, fontWeight: 700, color: '#00e5a0' }}>47</div>
                  <div style={{ fontSize: 12, color: '#86909C' }}>{t('analytics.funnel.effectiveAlerts')}</div>
                </Col>
                <Col span={8} style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 24, fontWeight: 700, color: '#ffaa33' }}>73%</div>
                  <div style={{ fontSize: 12, color: '#86909C' }}>{t('analytics.funnel.noiseRate')}</div>
                </Col>
              </Row>
            </div>
          </Card>
        </Col>
      </Row>

      {/* ---- 底部：误报 TOP10 规则表格 ---- */}
      <Card
        title={t('analytics.fpTable.title')}
        bordered
        style={{ borderRadius: 8 }}
        bodyStyle={{ padding: '8px 16px' }}
      >
        <Table<FalsePositiveRule>
          columns={falsePositiveColumns}
          dataSource={FALSE_POSITIVE_RULES}
          pagination={false}
          size="small"
          rowKey="key"
        />
      </Card>
    </div>
  );
};

export default AlertAnalytics;
