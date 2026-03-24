/**
 * 事件统计分析页面 - 展示事件全局统计数据、趋势分析、根因分布、高频故障与改进看板
 * 对应设计文档：页面 13 (/incident/analytics)
 *
 * 功能模块：
 * 1. 4 张翻牌卡片（活跃事件 / MTTR 均值 / 复发率 / 改进闭环率）
 * 2. MTTR/MTTA 双轴趋势图（ECharts，12 个月数据）
 * 3. 根因分类分布饼图
 * 4. P0-P4 严重级别堆叠面积图（12 个月）
 * 5. 高频故障 TOP5 表格
 * 6. 改进项看板（待办 / 进行中 / 已完成 三列卡片布局）
 */
import React, { useEffect, useRef, useState } from 'react';
import {
  Card, Row, Col, Table, Tag, Typography, Space, Select, Badge, Statistic,
} from 'antd';
import {
  ArrowUpOutlined, ArrowDownOutlined, FireOutlined,
  ClockCircleOutlined, ReloadOutlined, CheckCircleOutlined,
  FieldTimeOutlined, BugOutlined, ThunderboltOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import * as echarts from 'echarts';

const { Text, Title } = Typography;

/* ===================== 类型定义 ===================== */

/** 翻牌卡片数据结构 */
interface FlipCardData {
  key: string;
  /** 卡片标题 */
  title: string;
  /** 主数值 */
  value: number | string;
  /** 数值后缀（如 min、%） */
  suffix?: string;
  /** 趋势：上升/下降 */
  trend: 'up' | 'down';
  /** 环比变化百分比 */
  changePercent: number;
  /** 图标 */
  icon: React.ReactNode;
  /** 主色调 */
  color: string;
  /** 翻转后显示的背面数据 */
  backInfo: { label: string; value: string }[];
}

/** 高频故障 TOP5 数据结构 */
interface FrequentFault {
  key: string;
  /** 故障名称 */
  faultName: string;
  /** 发生次数 */
  count: number;
  /** 影响时长（分钟） */
  impactDuration: string;
  /** 关联服务列表 */
  relatedServices: string[];
  /** 是否为复发故障 */
  isRecurrent: boolean;
}

/** 改进项数据结构 */
interface ImprovementItem {
  id: string;
  /** 改进项标题 */
  title: string;
  /** 优先级 */
  priority: 'P0' | 'P1' | 'P2' | 'P3';
  /** 负责人 */
  owner: string;
  /** 截止日期 */
  deadline: string;
  /** 来源事件 ID */
  sourceIncident: string;
}

/* ===================== Mock 数据 ===================== */

/** 12 个月标签 */
const MONTHS = [
  '2025-04', '2025-05', '2025-06', '2025-07', '2025-08', '2025-09',
  '2025-10', '2025-11', '2025-12', '2026-01', '2026-02', '2026-03',
];

/** MTTR 数据（分钟），12 个月 */
const MTTR_DATA = [32, 28, 25, 22, 30, 18, 20, 16, 19, 15, 17, 18];

/** MTTA 数据（分钟），12 个月 */
const MTTA_DATA = [8, 7, 6, 5, 9, 4, 5, 3, 4, 3, 4, 3];

/** MTTR 目标线（分钟） */
const MTTR_TARGET = 20;

/** 根因分类分布数据 */
const ROOT_CAUSE_DATA = [
  { name: '人为操作', value: 40, color: '#722ED1' },
  { name: '容量不足', value: 25, color: '#FF7D00' },
  { name: '代码缺陷', value: 20, color: '#F53F3F' },
  { name: '外部依赖', value: 15, color: '#3491FA' },
];

/** P0-P4 各级别堆叠面积图数据，12 个月 */
const SEVERITY_STACK_DATA = {
  P0: [3, 2, 4, 1, 2, 1, 0, 1, 2, 0, 1, 1],
  P1: [8, 7, 6, 9, 5, 6, 4, 5, 3, 4, 3, 3],
  P2: [15, 13, 14, 12, 11, 10, 9, 8, 10, 7, 8, 7],
  P3: [22, 20, 18, 19, 16, 15, 14, 12, 13, 11, 10, 9],
  P4: [12, 10, 11, 9, 8, 7, 6, 5, 6, 4, 5, 4],
};

/** 严重级别对应颜色 */
const SEVERITY_COLORS: Record<string, string> = {
  P0: '#F53F3F', P1: '#FF7D00', P2: '#FAAD14', P3: '#3491FA', P4: '#C9CDD4',
};

/** 高频故障 TOP5 数据 */
const FREQUENT_FAULTS: FrequentFault[] = [
  { key: '1', faultName: '支付网关超时', count: 12, impactDuration: '4h 32min', relatedServices: ['pay-gateway', 'order-service'], isRecurrent: true },
  { key: '2', faultName: '数据库连接池耗尽', count: 8, impactDuration: '2h 15min', relatedServices: ['user-service', 'auth-service'], isRecurrent: true },
  { key: '3', faultName: 'Redis 集群主从切换', count: 6, impactDuration: '1h 48min', relatedServices: ['cache-service', 'session-service'], isRecurrent: false },
  { key: '4', faultName: 'K8s Pod OOM Killed', count: 5, impactDuration: '3h 05min', relatedServices: ['ml-inference', 'data-pipeline'], isRecurrent: true },
  { key: '5', faultName: 'CDN 节点异常', count: 4, impactDuration: '0h 52min', relatedServices: ['static-assets', 'media-service'], isRecurrent: false },
];

/** 改进项看板数据 - 待办 */
const TODO_ITEMS: ImprovementItem[] = [
  { id: 'IMP-001', title: '支付网关增加熔断降级', priority: 'P0', owner: '张伟', deadline: '2026-04-15', sourceIncident: 'INC-2024-0312' },
  { id: 'IMP-002', title: '数据库连接池监控告警', priority: 'P1', owner: '李明', deadline: '2026-04-20', sourceIncident: 'INC-2024-0298' },
  { id: 'IMP-003', title: 'OOM 自动扩容策略', priority: 'P1', owner: '王芳', deadline: '2026-04-25', sourceIncident: 'INC-2024-0285' },
];

/** 改进项看板数据 - 进行中 */
const IN_PROGRESS_ITEMS: ImprovementItem[] = [
  { id: 'IMP-004', title: 'Redis 哨兵模式升级', priority: 'P0', owner: '陈静', deadline: '2026-04-10', sourceIncident: 'INC-2024-0276' },
  { id: 'IMP-005', title: '全链路压测覆盖支付流程', priority: 'P1', owner: '张伟', deadline: '2026-04-08', sourceIncident: 'INC-2024-0312' },
  { id: 'IMP-006', title: 'CDN 多活容灾部署', priority: 'P2', owner: '刘洋', deadline: '2026-04-18', sourceIncident: 'INC-2024-0261' },
  { id: 'IMP-007', title: '日志采集延迟优化', priority: 'P2', owner: '赵磊', deadline: '2026-04-22', sourceIncident: 'INC-2024-0250' },
  { id: 'IMP-008', title: '变更发布灰度策略完善', priority: 'P1', owner: '孙悦', deadline: '2026-04-12', sourceIncident: 'INC-2024-0245' },
];

/** 改进项看板数据 - 已完成 */
const DONE_ITEMS: ImprovementItem[] = [
  { id: 'IMP-009', title: '告警收敛规则优化', priority: 'P1', owner: '李明', deadline: '2026-03-20', sourceIncident: 'INC-2024-0230' },
  { id: 'IMP-010', title: '核心服务 SLO 定义', priority: 'P0', owner: '张伟', deadline: '2026-03-15', sourceIncident: 'INC-2024-0218' },
  { id: 'IMP-011', title: 'K8s HPA 阈值调优', priority: 'P2', owner: '王芳', deadline: '2026-03-10', sourceIncident: 'INC-2024-0205' },
  { id: 'IMP-012', title: 'Runbook 自动化脚本集成', priority: 'P1', owner: '陈静', deadline: '2026-03-08', sourceIncident: 'INC-2024-0198' },
];

/** 翻牌卡片 Mock 数据 */
const FLIP_CARDS: FlipCardData[] = [
  {
    key: 'active',
    title: '活跃事件',
    value: 5,
    trend: 'down',
    changePercent: 28.5,
    icon: <FireOutlined />,
    color: '#F53F3F',
    backInfo: [
      { label: '环比上月', value: '-28.5%' },
      { label: '同比去年', value: '-45.2%' },
      { label: '本月峰值', value: '12' },
      { label: 'P0/P1 占比', value: '40%' },
    ],
  },
  {
    key: 'mttr',
    title: 'MTTR 均值',
    value: 18,
    suffix: 'min',
    trend: 'down',
    changePercent: 15.3,
    icon: <ClockCircleOutlined />,
    color: '#00B42A',
    backInfo: [
      { label: '环比上月', value: '-15.3%' },
      { label: '同比去年', value: '-32.1%' },
      { label: '最快恢复', value: '3 min' },
      { label: 'P0 平均', value: '25 min' },
    ],
  },
  {
    key: 'recurrence',
    title: '复发率',
    value: '8',
    suffix: '%',
    trend: 'down',
    changePercent: 5.2,
    icon: <ReloadOutlined />,
    color: '#FF7D00',
    backInfo: [
      { label: '环比上月', value: '-5.2%' },
      { label: '复发事件数', value: '3' },
      { label: '最高复发', value: '支付网关' },
      { label: '30 天均值', value: '9.5%' },
    ],
  },
  {
    key: 'improvement',
    title: '改进闭环率',
    value: '91',
    suffix: '%',
    trend: 'up',
    changePercent: 8.7,
    icon: <CheckCircleOutlined />,
    color: '#3491FA',
    backInfo: [
      { label: '环比上月', value: '+8.7%' },
      { label: '待办项', value: `${TODO_ITEMS.length}` },
      { label: '进行中', value: `${IN_PROGRESS_ITEMS.length}` },
      { label: '已完成', value: `${DONE_ITEMS.length}` },
    ],
  },
];

/** 优先级颜色映射 */
const PRIORITY_COLORS: Record<string, string> = {
  P0: '#F53F3F', P1: '#FF7D00', P2: '#3491FA', P3: '#86909C',
};

/* ===================== 翻牌卡片子组件 ===================== */

/**
 * FlipCard 翻牌卡片组件
 * - 正面：显示数值、趋势箭头和迷你变化百分比
 * - 背面：显示环比/同比/峰值/均值等详细数据
 * - hover 时 CSS 3D 翻转
 */
const FlipCard: React.FC<{ data: FlipCardData }> = ({ data }) => {
  const [flipped, setFlipped] = useState(false);

  return (
    <div
      style={{ perspective: 800, cursor: 'pointer', height: 140 }}
      onMouseEnter={() => setFlipped(true)}
      onMouseLeave={() => setFlipped(false)}
    >
      <div
        style={{
          position: 'relative',
          width: '100%',
          height: '100%',
          transition: 'transform 0.6s',
          transformStyle: 'preserve-3d',
          transform: flipped ? 'rotateY(180deg)' : 'rotateY(0deg)',
        }}
      >
        {/* ---- 正面 ---- */}
        <Card
          style={{
            position: 'absolute', width: '100%', height: '100%',
            backfaceVisibility: 'hidden', borderRadius: 8,
            boxShadow: '0 2px 8px rgba(0,0,0,0.06)',
          }}
          bodyStyle={{ padding: '16px 20px' }}
        >
          <Space align="center" style={{ marginBottom: 8 }}>
            <span style={{ fontSize: 20, color: data.color }}>{data.icon}</span>
            <Text style={{ color: '#86909C', fontSize: 14 }}>{data.title}</Text>
          </Space>
          <div style={{ fontSize: 32, fontWeight: 700, color: data.color }}>
            {data.value}
            {data.suffix && <span style={{ fontSize: 14, fontWeight: 400, marginLeft: 4 }}>{data.suffix}</span>}
          </div>
          <Space style={{ marginTop: 4 }}>
            {data.trend === 'up' ? (
              <ArrowUpOutlined style={{ color: data.key === 'improvement' ? '#00B42A' : '#F53F3F', fontSize: 12 }} />
            ) : (
              <ArrowDownOutlined style={{ color: data.key === 'improvement' ? '#F53F3F' : '#00B42A', fontSize: 12 }} />
            )}
            <Text style={{ fontSize: 12, color: '#86909C' }}>{data.changePercent}%</Text>
          </Space>
        </Card>

        {/* ---- 背面 ---- */}
        <Card
          style={{
            position: 'absolute', width: '100%', height: '100%',
            backfaceVisibility: 'hidden', borderRadius: 8,
            transform: 'rotateY(180deg)',
            boxShadow: '0 2px 8px rgba(0,0,0,0.06)',
            background: 'linear-gradient(135deg, #f5f5f5 0%, #ffffff 100%)',
          }}
          bodyStyle={{ padding: '12px 16px' }}
        >
          <Text strong style={{ fontSize: 13, color: '#1D2129', marginBottom: 8, display: 'block' }}>
            {data.title} 详情
          </Text>
          {data.backInfo.map((info) => (
            <div key={info.label} style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
              <Text style={{ fontSize: 12, color: '#86909C' }}>{info.label}</Text>
              <Text style={{ fontSize: 12, fontWeight: 600 }}>{info.value}</Text>
            </div>
          ))}
        </Card>
      </div>
    </div>
  );
};

/* ===================== 改进项看板列子组件 ===================== */

/**
 * KanbanColumn 看板列组件
 * 渲染单列改进项卡片，带标题与计数 Badge
 */
const KanbanColumn: React.FC<{
  title: string;
  color: string;
  items: ImprovementItem[];
}> = ({ title, color, items }) => (
  <Col span={8}>
    <div style={{ marginBottom: 12 }}>
      <Badge count={items.length} style={{ backgroundColor: color }} offset={[8, 0]}>
        <Text strong style={{ fontSize: 14 }}>{title}</Text>
      </Badge>
    </div>
    <div style={{
      background: '#FAFAFA', borderRadius: 8, padding: 8,
      minHeight: 200, border: `2px solid ${color}20`,
      borderTop: `3px solid ${color}`,
    }}>
      {items.map((item) => (
        <Card
          key={item.id}
          size="small"
          style={{ marginBottom: 8, borderRadius: 6, boxShadow: '0 1px 4px rgba(0,0,0,0.04)' }}
          bodyStyle={{ padding: '10px 12px' }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
            <Tag color={PRIORITY_COLORS[item.priority]} style={{ borderRadius: 4, border: 'none', fontWeight: 600, fontSize: 11 }}>
              {item.priority}
            </Tag>
            <Text style={{ fontSize: 11, color: '#86909C' }}>{item.id}</Text>
          </div>
          <Text style={{ fontSize: 13, fontWeight: 500, display: 'block', marginBottom: 4 }}>{item.title}</Text>
          <div style={{ display: 'flex', justifyContent: 'space-between' }}>
            <Text style={{ fontSize: 11, color: '#86909C' }}>{item.owner}</Text>
            <Text style={{ fontSize: 11, color: '#86909C' }}>截止: {item.deadline}</Text>
          </div>
        </Card>
      ))}
    </div>
  </Col>
);

/* ===================== 主组件 ===================== */

/**
 * IncidentAnalytics 事件统计分析主组件
 * 按设计文档页面 13 布局：
 * - 第一行：4 张翻牌卡片
 * - 第二行左：MTTR/MTTA 双轴趋势图；右：根因分类饼图
 * - 第三行左：P0-P4 堆叠面积图；右：高频故障 TOP5 表格
 * - 第四行：改进项看板（三列）
 */
const IncidentAnalytics: React.FC = () => {
  const { t } = useTranslation('incident');

  /** 时间周期筛选 */
  const [period, setPeriod] = useState<string>('12m');

  /* ---- ECharts DOM 引用 ---- */
  /** MTTR/MTTA 双轴趋势图容器 */
  const mttrChartRef = useRef<HTMLDivElement>(null);
  /** 根因分类饼图容器 */
  const rootCauseChartRef = useRef<HTMLDivElement>(null);
  /** P0-P4 堆叠面积图容器 */
  const severityChartRef = useRef<HTMLDivElement>(null);

  /* ---- 存储 ECharts 实例，用于 resize 和销毁 ---- */
  const mttrChartInstance = useRef<echarts.ECharts | null>(null);
  const rootCauseChartInstance = useRef<echarts.ECharts | null>(null);
  const severityChartInstance = useRef<echarts.ECharts | null>(null);

  /**
   * 初始化 MTTR/MTTA 双轴趋势图
   * 左轴 = MTTR（柱状），右轴 = MTTA（折线）+ 目标线
   */
  useEffect(() => {
    if (!mttrChartRef.current) return;
    const chart = echarts.init(mttrChartRef.current);
    mttrChartInstance.current = chart;

    chart.setOption({
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'cross' },
        formatter: (params: any) => {
          const items = Array.isArray(params) ? params : [params];
          let result = `<strong>${items[0]?.axisValue}</strong><br/>`;
          items.forEach((item: any) => {
            result += `${item.marker} ${item.seriesName}: ${item.value} min<br/>`;
          });
          return result;
        },
      },
      legend: { data: ['MTTR', 'MTTA', '目标线'], top: 4 },
      grid: { left: 50, right: 50, top: 40, bottom: 30 },
      xAxis: {
        type: 'category',
        data: MONTHS,
        axisLine: { lineStyle: { color: '#E5E6EB' } },
        axisLabel: { color: '#86909C' },
      },
      yAxis: [
        {
          type: 'value',
          name: 'MTTR (min)',
          nameTextStyle: { color: '#86909C' },
          axisLine: { lineStyle: { color: '#E5E6EB' } },
          splitLine: { lineStyle: { color: '#F2F3F5' } },
        },
        {
          type: 'value',
          name: 'MTTA (min)',
          nameTextStyle: { color: '#86909C' },
          axisLine: { lineStyle: { color: '#E5E6EB' } },
          splitLine: { show: false },
        },
      ],
      series: [
        {
          name: 'MTTR',
          type: 'bar',
          yAxisIndex: 0,
          data: MTTR_DATA.map((v) => ({
            value: v,
            /** 超过目标线的月份标红 */
            itemStyle: { color: v > MTTR_TARGET ? '#F53F3F' : '#3491FA' },
          })),
          barWidth: 20,
          animationDuration: 800,
        },
        {
          name: 'MTTA',
          type: 'line',
          yAxisIndex: 1,
          data: MTTA_DATA,
          smooth: true,
          lineStyle: { color: '#00B42A', width: 2 },
          itemStyle: { color: '#00B42A' },
          areaStyle: { color: 'rgba(0,180,42,0.08)' },
          animationDuration: 1000,
        },
        {
          name: '目标线',
          type: 'line',
          yAxisIndex: 0,
          data: MONTHS.map(() => MTTR_TARGET),
          lineStyle: { color: '#F53F3F', width: 1, type: 'dashed' },
          itemStyle: { opacity: 0 },
          symbol: 'none',
          tooltip: { show: false },
        },
      ],
    });

    /** 窗口大小变化时自适应 */
    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      chart.dispose();
    };
  }, []);

  /**
   * 初始化根因分类饼图
   * 展示各根因类型占比
   */
  useEffect(() => {
    if (!rootCauseChartRef.current) return;
    const chart = echarts.init(rootCauseChartRef.current);
    rootCauseChartInstance.current = chart;

    chart.setOption({
      tooltip: {
        trigger: 'item',
        formatter: '{b}: {c}% ({d}%)',
      },
      legend: {
        orient: 'vertical',
        right: 20,
        top: 'center',
        textStyle: { color: '#4E5969' },
      },
      series: [
        {
          type: 'pie',
          radius: ['40%', '70%'],
          center: ['40%', '50%'],
          avoidLabelOverlap: true,
          itemStyle: { borderRadius: 6, borderColor: '#fff', borderWidth: 2 },
          label: {
            show: true,
            formatter: '{b}\n{d}%',
            fontSize: 12,
          },
          emphasis: {
            label: { show: true, fontSize: 14, fontWeight: 'bold' },
            itemStyle: { shadowBlur: 10, shadowOffsetX: 0, shadowColor: 'rgba(0,0,0,0.2)' },
          },
          data: ROOT_CAUSE_DATA.map((item) => ({
            name: item.name,
            value: item.value,
            itemStyle: { color: item.color },
          })),
          animationType: 'scale',
          animationDuration: 800,
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

  /**
   * 初始化 P0-P4 严重级别堆叠面积图
   * 12 个月数据，总体应呈下降趋势
   */
  useEffect(() => {
    if (!severityChartRef.current) return;
    const chart = echarts.init(severityChartRef.current);
    severityChartInstance.current = chart;

    chart.setOption({
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'cross' },
      },
      legend: {
        data: ['P0', 'P1', 'P2', 'P3', 'P4'],
        top: 4,
      },
      grid: { left: 50, right: 20, top: 40, bottom: 30 },
      xAxis: {
        type: 'category',
        data: MONTHS,
        boundaryGap: false,
        axisLine: { lineStyle: { color: '#E5E6EB' } },
        axisLabel: { color: '#86909C' },
      },
      yAxis: {
        type: 'value',
        name: '事件数',
        nameTextStyle: { color: '#86909C' },
        axisLine: { lineStyle: { color: '#E5E6EB' } },
        splitLine: { lineStyle: { color: '#F2F3F5' } },
      },
      series: Object.entries(SEVERITY_STACK_DATA).map(([level, data]) => ({
        name: level,
        type: 'line',
        stack: 'severity',
        areaStyle: { opacity: 0.4 },
        emphasis: { focus: 'series' },
        smooth: true,
        lineStyle: { width: 1.5, color: SEVERITY_COLORS[level] },
        itemStyle: { color: SEVERITY_COLORS[level] },
        data,
        animationDuration: 1000,
      })),
    });

    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      chart.dispose();
    };
  }, []);

  /** 高频故障 TOP5 表格列定义 */
  const faultColumns = [
    {
      title: t('analytics.faultTable.rank'),
      key: 'rank',
      width: 60,
      /** 渲染排名序号，前 3 名高亮 */
      render: (_: unknown, __: unknown, index: number) => (
        <Badge
          count={index + 1}
          style={{
            backgroundColor: index < 3 ? '#F53F3F' : '#86909C',
            fontSize: 11,
          }}
        />
      ),
    },
    {
      title: t('analytics.faultTable.name'),
      dataIndex: 'faultName',
      key: 'faultName',
      /** 复发故障名前加图标标记 */
      render: (text: string, record: FrequentFault) => (
        <Space>
          <Text>{text}</Text>
          {record.isRecurrent && (
            <Tag color="#FF7D00" style={{ borderRadius: 4, fontSize: 11, border: 'none' }}>
              <ReloadOutlined /> {t('analytics.faultTable.recurrent')}
            </Tag>
          )}
        </Space>
      ),
    },
    {
      title: t('analytics.faultTable.count'),
      dataIndex: 'count',
      key: 'count',
      width: 80,
      /** 高发次数加粗红色 */
      render: (val: number) => (
        <Text strong style={{ color: val >= 10 ? '#F53F3F' : '#1D2129' }}>{val}</Text>
      ),
    },
    {
      title: t('analytics.faultTable.impact'),
      dataIndex: 'impactDuration',
      key: 'impactDuration',
      width: 120,
    },
    {
      title: t('analytics.faultTable.services'),
      dataIndex: 'relatedServices',
      key: 'relatedServices',
      /** 渲染关联服务标签列表 */
      render: (services: string[]) => (
        <Space size={4} wrap>
          {services.map((s) => (
            <Tag key={s} color="blue" style={{ borderRadius: 4, fontSize: 11 }}>{s}</Tag>
          ))}
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* ===== 页面标题与时间筛选 ===== */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Space>
          <FieldTimeOutlined style={{ fontSize: 22, color: '#3491FA' }} />
          <Text strong style={{ fontSize: 20 }}>{t('analytics.title')}</Text>
        </Space>
        <Select
          value={period}
          onChange={setPeriod}
          style={{ width: 140 }}
          options={[
            { value: '3m', label: t('analytics.period.3m') },
            { value: '6m', label: t('analytics.period.6m') },
            { value: '12m', label: t('analytics.period.12m') },
          ]}
        />
      </div>

      {/* ===== 第一行：4 张翻牌卡片 ===== */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {FLIP_CARDS.map((card) => (
          <Col span={6} key={card.key}>
            <FlipCard data={card} />
          </Col>
        ))}
      </Row>

      {/* ===== 第二行：MTTR/MTTA 双轴趋势 + 根因分类饼图 ===== */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={14}>
          <Card
            title={
              <Space>
                <ClockCircleOutlined style={{ color: '#3491FA' }} />
                <span>{t('analytics.mttrTrend')}</span>
              </Space>
            }
            bordered
            style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
            bodyStyle={{ padding: '12px 16px' }}
          >
            {/* MTTR/MTTA 双轴趋势图容器 */}
            <div ref={mttrChartRef} style={{ width: '100%', height: 320 }} />
          </Card>
        </Col>
        <Col span={10}>
          <Card
            title={
              <Space>
                <BugOutlined style={{ color: '#722ED1' }} />
                <span>{t('analytics.rootCause')}</span>
              </Space>
            }
            bordered
            style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
            bodyStyle={{ padding: '12px 16px' }}
          >
            {/* 根因分类饼图容器 */}
            <div ref={rootCauseChartRef} style={{ width: '100%', height: 320 }} />
          </Card>
        </Col>
      </Row>

      {/* ===== 第三行：P0-P4 堆叠面积图 + 高频故障 TOP5 表格 ===== */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={14}>
          <Card
            title={
              <Space>
                <ThunderboltOutlined style={{ color: '#FF7D00' }} />
                <span>{t('analytics.severityTrend')}</span>
              </Space>
            }
            bordered
            style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
            bodyStyle={{ padding: '12px 16px' }}
          >
            {/* P0-P4 堆叠面积图容器 */}
            <div ref={severityChartRef} style={{ width: '100%', height: 320 }} />
          </Card>
        </Col>
        <Col span={10}>
          <Card
            title={
              <Space>
                <FireOutlined style={{ color: '#F53F3F' }} />
                <span>{t('analytics.topFaults')}</span>
              </Space>
            }
            bordered
            style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
            bodyStyle={{ padding: '8px 0' }}
          >
            {/* 高频故障 TOP5 表格 */}
            <Table
              columns={faultColumns}
              dataSource={FREQUENT_FAULTS}
              pagination={false}
              size="small"
              rowKey="key"
            />
          </Card>
        </Col>
      </Row>

      {/* ===== 第四行：改进项看板（三列） ===== */}
      <Card
        title={
          <Space>
            <CheckCircleOutlined style={{ color: '#00B42A' }} />
            <span>{t('analytics.improvement')}</span>
          </Space>
        }
        bordered
        style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
        bodyStyle={{ padding: 16 }}
      >
        <Row gutter={16}>
          <KanbanColumn title={t('analytics.kanban.todo')} color="#FF7D00" items={TODO_ITEMS} />
          <KanbanColumn title={t('analytics.kanban.inProgress')} color="#3491FA" items={IN_PROGRESS_ITEMS} />
          <KanbanColumn title={t('analytics.kanban.done')} color="#00B42A" items={DONE_ITEMS} />
        </Row>
      </Card>
    </div>
  );
};

export default IncidentAnalytics;
