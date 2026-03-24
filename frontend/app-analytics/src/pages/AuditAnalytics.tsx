/**
 * 操作审计统计页面（页面 16）
 * 路由: /audit/analytics
 *
 * 功能模块：
 * - 4 张统计卡片（总操作数 / 高危操作 / 违规操作 / 审计覆盖率）
 * - 操作时间热力图（ECharts heatmap，7天 x 24小时，标红非工作时间高危操作）
 * - 高危操作 TOP10 水平柱状图（ECharts）
 * - 操作类型分布饼图
 * - 违规趋势折线图（12 个月）
 * - 最近审计日志表格
 *
 * 数据来源：Mock 数据（后端就绪后替换）
 */
import React, { useEffect, useRef } from 'react';
import * as echarts from 'echarts';
import {
  Card, Row, Col, Table, Tag, Typography, Space, Badge,
} from 'antd';
import {
  AuditOutlined, WarningOutlined, StopOutlined,
  SafetyCertificateOutlined, UserOutlined, ClockCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

/* ==================== Mock 数据 ==================== */

/** 统计卡片数据 */
const STAT_CARDS = [
  {
    key: 'totalOps',
    icon: <AuditOutlined />,
    color: '#4da6ff',
    value: '8,432',
    trend: '+12.3%',
    trendUp: true,
  },
  {
    key: 'highRisk',
    icon: <WarningOutlined />,
    color: '#ffaa33',
    value: '127',
    trend: '-5.2%',
    trendUp: false,
  },
  {
    key: 'violations',
    icon: <StopOutlined />,
    color: '#ff6b6b',
    value: '14',
    trend: '-18.7%',
    trendUp: false,
  },
  {
    key: 'coverage',
    icon: <SafetyCertificateOutlined />,
    color: '#00e5a0',
    value: '94.2%',
    trend: '+2.1%',
    trendUp: true,
  },
];

/**
 * 操作时间热力图数据
 * 格式：[小时(0-23), 星期(0=周一, 6=周日), 操作次数]
 * 非工作时间的高危操作用较高数值标记
 */
const generateHeatmapData = (): number[][] => {
  const data: number[][] = [];
  // 星期（0=周一 ~ 6=周日）
  for (let day = 0; day < 7; day++) {
    for (let hour = 0; hour < 24; hour++) {
      // 工作时间（周一至周五 9:00-18:00）操作较多
      const isWorkTime = day < 5 && hour >= 9 && hour < 18;
      // 非工作时间也有少量操作（模拟高危场景）
      let value: number;
      if (isWorkTime) {
        value = Math.floor(Math.random() * 30) + 20; // 20-50
      } else if (hour >= 0 && hour < 6) {
        // 凌晨时段 — 少量但可能是高危操作
        value = Math.floor(Math.random() * 8) + 1;   // 1-8
      } else {
        value = Math.floor(Math.random() * 15) + 3;   // 3-18
      }
      data.push([hour, day, value]);
    }
  }
  return data;
};

/** 热力图 Mock 数据 */
const HEATMAP_DATA = generateHeatmapData();

/** 高危操作 TOP10 Mock 数据 */
const TOP10_DATA = [
  { name: '删除生产数据库', value: 42 },
  { name: '权限变更(root)', value: 38 },
  { name: '配置修改(防火墙)', value: 28 },
  { name: '密钥轮换', value: 22 },
  { name: '服务重启(核心)', value: 19 },
  { name: '安全组变更', value: 16 },
  { name: '批量用户禁用', value: 14 },
  { name: '存储卷删除', value: 11 },
  { name: '网络ACL修改', value: 9 },
  { name: '证书替换', value: 7 },
];

/** 操作类型分布 Mock 数据 */
const OPERATION_TYPES = [
  { name: '配置变更', value: 2845 },
  { name: '权限管理', value: 1920 },
  { name: '服务操作', value: 1560 },
  { name: '数据操作', value: 1240 },
  { name: '安全审计', value: 867 },
];

/** 违规趋势 Mock 数据（近 12 个月） */
const VIOLATION_MONTHS = [
  '2025-04', '2025-05', '2025-06', '2025-07', '2025-08', '2025-09',
  '2025-10', '2025-11', '2025-12', '2026-01', '2026-02', '2026-03',
];
const VIOLATION_DATA = [28, 32, 25, 22, 19, 24, 18, 15, 20, 16, 12, 14];
/** 无工单操作数据（用红色标记） */
const NO_TICKET_DATA = [8, 12, 7, 6, 5, 9, 4, 3, 6, 4, 2, 3];

/** 审计日志 Mock 数据 */
interface AuditLog {
  key: string;
  time: string;
  operator: string;
  operation: string;
  target: string;
  result: 'success' | 'failed' | 'blocked';
  riskLevel: 'high' | 'medium' | 'low';
  hasTicket: boolean;
}

const AUDIT_LOGS: AuditLog[] = [
  { key: '1', time: '2026-03-24 10:32:15', operator: '张三', operation: '配置修改', target: 'nginx-prod-01', result: 'success', riskLevel: 'medium', hasTicket: true },
  { key: '2', time: '2026-03-24 10:28:42', operator: '李四', operation: '权限变更', target: 'db-master-02', result: 'blocked', riskLevel: 'high', hasTicket: false },
  { key: '3', time: '2026-03-24 10:15:03', operator: '王五', operation: '服务重启', target: 'api-gateway-01', result: 'success', riskLevel: 'high', hasTicket: true },
  { key: '4', time: '2026-03-24 09:58:21', operator: '赵六', operation: '数据导出', target: 'mysql-slave-03', result: 'success', riskLevel: 'medium', hasTicket: true },
  { key: '5', time: '2026-03-24 09:45:10', operator: '钱七', operation: '密钥轮换', target: 'vault-prod', result: 'success', riskLevel: 'high', hasTicket: true },
  { key: '6', time: '2026-03-24 03:12:08', operator: '孙八', operation: '删除操作', target: 'temp-storage-05', result: 'failed', riskLevel: 'high', hasTicket: false },
  { key: '7', time: '2026-03-24 02:45:33', operator: '周九', operation: '安全组变更', target: 'vpc-prod-sg', result: 'blocked', riskLevel: 'high', hasTicket: false },
  { key: '8', time: '2026-03-23 23:18:45', operator: '吴十', operation: '配置修改', target: 'redis-cluster-01', result: 'success', riskLevel: 'low', hasTicket: true },
];

/* ==================== 颜色配置 ==================== */

/** 风险等级颜色 */
const RISK_COLORS: Record<string, string> = {
  high: '#ff6b6b',
  medium: '#ffaa33',
  low: '#00e5a0',
};

/** 操作结果颜色 */
const RESULT_COLORS: Record<string, string> = {
  success: '#00e5a0',
  failed: '#ff6b6b',
  blocked: '#ffaa33',
};

/* ==================== 主组件 ==================== */

/**
 * 操作审计统计页面组件
 * 展示操作审计的多维度统计分析，包括时间分布、高危排行、类型分布、违规趋势
 */
const AuditAnalytics: React.FC = () => {
  const { t } = useTranslation('analytics');

  /* ---------- ECharts 容器引用 ---------- */
  /** 热力图容器 */
  const heatmapRef = useRef<HTMLDivElement>(null);
  /** 高危 TOP10 柱状图容器 */
  const top10Ref = useRef<HTMLDivElement>(null);
  /** 操作类型饼图容器 */
  const pieRef = useRef<HTMLDivElement>(null);
  /** 违规趋势折线图容器 */
  const trendRef = useRef<HTMLDivElement>(null);

  /* ---------- 热力图渲染 ---------- */

  useEffect(() => {
    if (!heatmapRef.current) return;
    const chart = echarts.init(heatmapRef.current);

    /** 星期标签 */
    const weekDays = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];
    /** 小时标签（0-23） */
    const hours = Array.from({ length: 24 }, (_, i) => `${i}:00`);

    chart.setOption({
      tooltip: {
        position: 'top',
        formatter: (params: any) => {
          const [hour, day, value] = params.value;
          return `${weekDays[day]} ${hour}:00<br/>操作次数: ${value}`;
        },
      },
      grid: { left: 60, right: 30, top: 10, bottom: 40 },
      xAxis: {
        type: 'category',
        data: hours,
        splitArea: { show: true },
        axisLabel: { fontSize: 10 },
      },
      yAxis: {
        type: 'category',
        data: weekDays,
        splitArea: { show: true },
      },
      visualMap: {
        min: 0,
        max: 50,
        calculable: true,
        orient: 'horizontal',
        left: 'center',
        bottom: 0,
        inRange: {
          color: ['#ebedf0', '#9be9a8', '#40c463', '#30a14e', '#216e39'],
        },
        textStyle: { fontSize: 11 },
      },
      series: [
        {
          type: 'heatmap',
          data: HEATMAP_DATA,
          label: { show: false },
          emphasis: {
            itemStyle: {
              shadowBlur: 10,
              shadowColor: 'rgba(0, 0, 0, 0.5)',
            },
          },
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

  /* ---------- 高危操作 TOP10 柱状图渲染 ---------- */

  useEffect(() => {
    if (!top10Ref.current) return;
    const chart = echarts.init(top10Ref.current);

    /** 按操作数量倒序排列 */
    const sortedData = [...TOP10_DATA].reverse();

    chart.setOption({
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
      grid: { left: 130, right: 30, top: 10, bottom: 20 },
      xAxis: { type: 'value' },
      yAxis: {
        type: 'category',
        data: sortedData.map((d) => d.name),
        axisLabel: { fontSize: 11 },
      },
      series: [
        {
          type: 'bar',
          data: sortedData.map((d) => ({
            value: d.value,
            itemStyle: {
              color: d.value >= 30
                ? '#ff6b6b'       // 高危 - 红色
                : d.value >= 15
                  ? '#ffaa33'     // 中危 - 橙色
                  : '#4da6ff',    // 一般 - 蓝色
              borderRadius: [0, 4, 4, 0],
            },
          })),
          barMaxWidth: 20,
          label: {
            show: true,
            position: 'right',
            formatter: '{c}',
            fontSize: 11,
          },
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

  /* ---------- 操作类型分布饼图渲染 ---------- */

  useEffect(() => {
    if (!pieRef.current) return;
    const chart = echarts.init(pieRef.current);

    /** 饼图颜色 */
    const PIE_COLORS = ['#4da6ff', '#00e5a0', '#ffaa33', '#ff6b6b', '#6366f1'];

    chart.setOption({
      tooltip: {
        trigger: 'item',
        formatter: '{b}: {c} ({d}%)',
      },
      legend: {
        orient: 'vertical',
        right: 10,
        top: 'center',
        textStyle: { fontSize: 12 },
      },
      series: [
        {
          type: 'pie',
          radius: ['40%', '70%'],
          center: ['40%', '50%'],
          avoidLabelOverlap: false,
          itemStyle: {
            borderRadius: 6,
            borderColor: '#fff',
            borderWidth: 2,
          },
          label: { show: false },
          emphasis: {
            label: { show: true, fontSize: 14, fontWeight: 'bold' },
          },
          data: OPERATION_TYPES.map((item, index) => ({
            ...item,
            itemStyle: { color: PIE_COLORS[index] },
          })),
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

  /* ---------- 违规趋势折线图渲染 ---------- */

  useEffect(() => {
    if (!trendRef.current) return;
    const chart = echarts.init(trendRef.current);

    chart.setOption({
      tooltip: { trigger: 'axis' },
      legend: { data: ['违规操作', '无工单操作'], top: 4 },
      grid: { left: 50, right: 20, top: 40, bottom: 30 },
      xAxis: {
        type: 'category',
        data: VIOLATION_MONTHS,
        axisLabel: { fontSize: 10 },
      },
      yAxis: { type: 'value', name: '次数' },
      series: [
        {
          name: '违规操作',
          type: 'line',
          smooth: true,
          data: VIOLATION_DATA,
          lineStyle: { color: '#ffaa33', width: 2 },
          itemStyle: { color: '#ffaa33' },
          areaStyle: {
            color: {
              type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
              colorStops: [
                { offset: 0, color: 'rgba(255,170,51,0.2)' },
                { offset: 1, color: 'rgba(255,170,51,0)' },
              ],
            },
          },
        },
        {
          name: '无工单操作',
          type: 'line',
          smooth: true,
          data: NO_TICKET_DATA,
          lineStyle: { color: '#ff6b6b', width: 2 },
          itemStyle: { color: '#ff6b6b' },
          areaStyle: {
            color: {
              type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
              colorStops: [
                { offset: 0, color: 'rgba(255,107,107,0.15)' },
                { offset: 1, color: 'rgba(255,107,107,0)' },
              ],
            },
          },
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

  /* ---------- 审计日志表格列定义 ---------- */

  const auditColumns = [
    {
      title: t('audit.table.time'),
      dataIndex: 'time',
      key: 'time',
      width: 170,
      render: (time: string) => (
        <Space size={4}>
          <ClockCircleOutlined style={{ color: '#86909C' }} />
          <Text>{time}</Text>
        </Space>
      ),
    },
    {
      title: t('audit.table.operator'),
      dataIndex: 'operator',
      key: 'operator',
      width: 100,
      render: (name: string) => (
        <Space size={4}>
          <UserOutlined />
          <Text>{name}</Text>
        </Space>
      ),
    },
    {
      title: t('audit.table.operation'),
      dataIndex: 'operation',
      key: 'operation',
      width: 120,
    },
    {
      title: t('audit.table.target'),
      dataIndex: 'target',
      key: 'target',
      width: 160,
      render: (target: string) => <Text code>{target}</Text>,
    },
    {
      title: t('audit.table.riskLevel'),
      dataIndex: 'riskLevel',
      key: 'riskLevel',
      width: 90,
      /** 渲染风险等级标签 */
      render: (level: string) => (
        <Tag color={RISK_COLORS[level]}>
          {t(`audit.risk.${level}`)}
        </Tag>
      ),
    },
    {
      title: t('audit.table.result'),
      dataIndex: 'result',
      key: 'result',
      width: 90,
      /** 渲染操作结果标签 */
      render: (result: string) => (
        <Tag color={RESULT_COLORS[result]}>
          {t(`audit.result.${result}`)}
        </Tag>
      ),
    },
    {
      title: t('audit.table.ticket'),
      dataIndex: 'hasTicket',
      key: 'hasTicket',
      width: 80,
      /** 有工单显示绿色点，无工单显示红色警告 */
      render: (has: boolean) => (
        has
          ? <Badge status="success" text={t('audit.ticket.yes')} />
          : <Badge status="error" text={t('audit.ticket.no')} />
      ),
    },
  ];

  /* ---------- 渲染 ---------- */

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('audit.title')}</Text>
      </div>

      {/* 统计卡片行 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {STAT_CARDS.map((card) => (
          <Col span={6} key={card.key}>
            <Card
              bordered
              style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
              bodyStyle={{ padding: '16px 20px' }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <div>
                  <div style={{ color: '#86909C', fontSize: 14 }}>{t(`audit.stats.${card.key}`)}</div>
                  <div style={{ fontSize: 32, fontWeight: 600, marginTop: 4, color: card.color }}>
                    {card.value}
                  </div>
                  <div style={{ fontSize: 12, marginTop: 4 }}>
                    <Text type={card.trendUp ? 'danger' : 'success'}>
                      {card.trend}
                    </Text>
                    <Text type="secondary" style={{ marginLeft: 4 }}>
                      {t('audit.stats.vsLastWeek')}
                    </Text>
                  </div>
                </div>
                <div
                  style={{
                    width: 40,
                    height: 40,
                    borderRadius: 8,
                    backgroundColor: `${card.color}15`,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 20,
                    color: card.color,
                  }}
                >
                  {card.icon}
                </div>
              </div>
            </Card>
          </Col>
        ))}
      </Row>

      {/* 热力图 + 高危 TOP10 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={14}>
          <Card
            title={t('audit.heatmap.title')}
            style={{ borderRadius: 8 }}
          >
            <div ref={heatmapRef} style={{ height: 280, width: '100%' }} />
          </Card>
        </Col>
        <Col span={10}>
          <Card
            title={t('audit.top10.title')}
            style={{ borderRadius: 8 }}
          >
            <div ref={top10Ref} style={{ height: 280, width: '100%' }} />
          </Card>
        </Col>
      </Row>

      {/* 操作类型饼图 + 违规趋势折线图 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={10}>
          <Card
            title={t('audit.pie.title')}
            style={{ borderRadius: 8 }}
          >
            <div ref={pieRef} style={{ height: 280, width: '100%' }} />
          </Card>
        </Col>
        <Col span={14}>
          <Card
            title={t('audit.trend.title')}
            style={{ borderRadius: 8 }}
          >
            <div ref={trendRef} style={{ height: 280, width: '100%' }} />
          </Card>
        </Col>
      </Row>

      {/* 最近审计日志表格 */}
      <Card title={t('audit.logs.title')} style={{ borderRadius: 8 }}>
        <Table<AuditLog>
          columns={auditColumns}
          dataSource={AUDIT_LOGS}
          pagination={{ pageSize: 10 }}
          size="middle"
        />
      </Card>
    </div>
  );
};

export default AuditAnalytics;
