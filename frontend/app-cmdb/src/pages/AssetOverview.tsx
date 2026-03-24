/**
 * 资产概览仪表盘页面 — 页面 14 (/cmdb/overview)
 *
 * 功能模块：
 * 1. 4 张翻牌统计卡片（总资产数 / 健康率 / 过保资产 / 闲置资产）
 * 2. 资产分类 Treemap（SVG 实现，按类型分：服务器 / 网络 / 存储 / 容器）
 * 3. 健康度热力矩阵（使用 @opsnexus/ui-kit HealthMatrix 组件）
 * 4. 资源利用率散点图（SVG 实现，X=CPU Y=内存 气泡大小=磁盘）
 * 5. 过保资产列表（表格：资产名 / 类型 / 过保日期 / 剩余天数 / 负责人）
 */
import React, { useState, useEffect } from 'react';
import { Typography, Card, Row, Col, Table, Tag, Tooltip } from 'antd';
import {
  CloudServerOutlined,
  ApiOutlined,
  DatabaseOutlined,
  ContainerOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { MetricFlipCard, HealthMatrix } from '@opsnexus/ui-kit';
import type { HealthCell } from '@opsnexus/ui-kit';

const { Text } = Typography;

/* ============================== Mock 数据 ============================== */

/** 资产分类数据结构 */
interface AssetCategory {
  /** 分类名称 */
  name: string;
  /** 资产数量 */
  value: number;
  /** 显示颜色 */
  color: string;
  /** 图标 */
  icon: React.ReactNode;
}

/** 资源利用率散点数据 */
interface ResourcePoint {
  /** 主机名 */
  hostname: string;
  /** CPU 利用率 (%) */
  cpu: number;
  /** 内存利用率 (%) */
  mem: number;
  /** 磁盘利用率 (%) — 映射为气泡大小 */
  disk: number;
  /** 状态标记 */
  status: 'normal' | 'overload' | 'idle';
}

/** 过保资产数据结构 */
interface ExpiredAsset {
  /** 资产唯一 ID */
  id: string;
  /** 资产名称 */
  name: string;
  /** 资产类型 */
  type: string;
  /** 过保日期 */
  expireDate: string;
  /** 剩余天数（负数表示已过保） */
  remainDays: number;
  /** 负责人 */
  owner: string;
}

/** 模拟资产分类数据 */
const MOCK_CATEGORIES: AssetCategory[] = [
  { name: '服务器', value: 562, color: '#4da6ff', icon: <CloudServerOutlined /> },
  { name: '网络设备', value: 249, color: '#00e5a0', icon: <ApiOutlined /> },
  { name: '存储设备', value: 187, color: '#ffaa33', icon: <DatabaseOutlined /> },
  { name: '容器实例', value: 249, color: '#60a5fa', icon: <ContainerOutlined /> },
];

/** 模拟资源利用率散点数据 */
const MOCK_RESOURCE_POINTS: ResourcePoint[] = [
  { hostname: 'web-01', cpu: 85, mem: 78, disk: 60, status: 'overload' },
  { hostname: 'web-02', cpu: 72, mem: 65, disk: 45, status: 'normal' },
  { hostname: 'db-master', cpu: 90, mem: 88, disk: 75, status: 'overload' },
  { hostname: 'db-slave', cpu: 45, mem: 52, disk: 70, status: 'normal' },
  { hostname: 'cache-01', cpu: 30, mem: 40, disk: 20, status: 'normal' },
  { hostname: 'cache-02', cpu: 15, mem: 18, disk: 15, status: 'idle' },
  { hostname: 'api-gw-01', cpu: 68, mem: 72, disk: 35, status: 'normal' },
  { hostname: 'api-gw-02', cpu: 55, mem: 60, disk: 30, status: 'normal' },
  { hostname: 'mq-01', cpu: 42, mem: 38, disk: 55, status: 'normal' },
  { hostname: 'mq-02', cpu: 12, mem: 10, disk: 8, status: 'idle' },
  { hostname: 'monitor-01', cpu: 78, mem: 82, disk: 65, status: 'overload' },
  { hostname: 'log-01', cpu: 62, mem: 70, disk: 80, status: 'normal' },
  { hostname: 'k8s-node-01', cpu: 58, mem: 55, disk: 40, status: 'normal' },
  { hostname: 'k8s-node-02', cpu: 8, mem: 12, disk: 10, status: 'idle' },
  { hostname: 'k8s-node-03', cpu: 92, mem: 85, disk: 72, status: 'overload' },
  { hostname: 'storage-01', cpu: 25, mem: 30, disk: 90, status: 'normal' },
];

/** 模拟过保资产列表 */
const MOCK_EXPIRED_ASSETS: ExpiredAsset[] = [
  { id: '1', name: 'Dell R730 #A12', type: '服务器', expireDate: '2026-02-15', remainDays: -37, owner: '张明' },
  { id: '2', name: 'Cisco 3750 #N05', type: '网络设备', expireDate: '2026-03-01', remainDays: -23, owner: '李华' },
  { id: '3', name: 'HP DL380 #A08', type: '服务器', expireDate: '2026-03-10', remainDays: -14, owner: '王强' },
  { id: '4', name: 'NetApp FAS2700', type: '存储设备', expireDate: '2026-04-01', remainDays: 8, owner: '赵雪' },
  { id: '5', name: 'Dell R740 #A15', type: '服务器', expireDate: '2026-04-10', remainDays: 17, owner: '刘洋' },
  { id: '6', name: 'H3C S6520 #N12', type: '网络设备', expireDate: '2026-04-20', remainDays: 27, owner: '陈飞' },
  { id: '7', name: 'Huawei CE6850', type: '网络设备', expireDate: '2026-05-05', remainDays: 42, owner: '孙磊' },
  { id: '8', name: 'Lenovo SR650 #A20', type: '服务器', expireDate: '2026-05-15', remainDays: 52, owner: '周婷' },
];

/**
 * 生成模拟健康度热力矩阵数据
 * 5 行 x 5 列，行=业务线，列=监控维度
 */
function generateMockHealthCells(): HealthCell[] {
  const statuses: HealthCell['status'][] = ['ok', 'ok', 'ok', 'degraded', 'critical', 'ok', 'ok', 'unknown'];
  const cells: HealthCell[] = [];
  const biz = ['pay', 'order', 'user', 'risk', 'infra'];
  const dims = ['NET', 'HOST', 'APP', 'DB', 'MW'];
  for (let r = 0; r < 5; r++) {
    for (let c = 0; c < 5; c++) {
      const st = statuses[Math.floor(Math.random() * statuses.length)];
      cells.push({
        id: `${biz[r]}-${dims[c]}`,
        label: `${biz[r]}-${dims[c]}`,
        status: st,
        details: {
          cpu: Math.floor(Math.random() * 60 + 20),
          mem: Math.floor(Math.random() * 60 + 20),
          disk: Math.floor(Math.random() * 50 + 10),
          conn: Math.floor(Math.random() * 500 + 50),
        },
      });
    }
  }
  return cells;
}

/* ============================== SVG 图表组件 ============================== */

/**
 * 资产分类 Treemap 组件（SVG 实现）
 * 按面积比例展示各类资产数量占比
 */
const AssetTreemap: React.FC<{ data: AssetCategory[] }> = ({ data }) => {
  const width = 560;
  const height = 200;
  const total = data.reduce((s, d) => s + d.value, 0);

  /** 使用简化的 Squarify 布局算法计算 Treemap 矩形位置 */
  const rects: Array<{ x: number; y: number; w: number; h: number; item: AssetCategory }> = [];
  let currentX = 0;

  data.forEach((item) => {
    const ratio = item.value / total;
    const w = width * ratio;
    rects.push({ x: currentX, y: 0, w, h: height, item });
    currentX += w;
  });

  return (
    <svg viewBox={`0 0 ${width} ${height}`} style={{ width: '100%', maxHeight: 240 }}>
      {rects.map((r, i) => (
        <g key={i}>
          {/* 矩形区块 */}
          <rect
            x={r.x + 1}
            y={1}
            width={Math.max(r.w - 2, 0)}
            height={r.h - 2}
            rx={6}
            fill={r.item.color}
            opacity={0.18}
            stroke={r.item.color}
            strokeWidth={1}
            strokeOpacity={0.4}
          />
          {/* 分类名称标签 */}
          <text
            x={r.x + r.w / 2}
            y={r.h / 2 - 12}
            textAnchor="middle"
            fontSize={14}
            fontWeight={600}
            fill={r.item.color}
          >
            {r.item.name}
          </text>
          {/* 数量值 */}
          <text
            x={r.x + r.w / 2}
            y={r.h / 2 + 10}
            textAnchor="middle"
            fontSize={22}
            fontWeight={700}
            fill={r.item.color}
            fontFamily="Inter, sans-serif"
          >
            {r.item.value}
          </text>
          {/* 占比标签 */}
          <text
            x={r.x + r.w / 2}
            y={r.h / 2 + 32}
            textAnchor="middle"
            fontSize={12}
            fill={r.item.color}
            opacity={0.7}
          >
            {((r.item.value / total) * 100).toFixed(1)}%
          </text>
        </g>
      ))}
    </svg>
  );
};

/**
 * 资源利用率散点图组件（SVG 实现）
 * X 轴 = CPU 利用率，Y 轴 = 内存利用率，气泡大小 = 磁盘利用率
 * 红色区域 = 过载（>80%），灰色区域 = 闲置（<20%）
 */
const ResourceScatter: React.FC<{ data: ResourcePoint[] }> = ({ data }) => {
  const width = 560;
  const height = 280;
  const padding = { top: 20, right: 30, bottom: 40, left: 50 };
  const chartW = width - padding.left - padding.right;
  const chartH = height - padding.top - padding.bottom;

  /** 将百分比值映射到画布坐标 */
  const scaleX = (v: number) => padding.left + (v / 100) * chartW;
  const scaleY = (v: number) => padding.top + chartH - (v / 100) * chartH;

  /** 气泡半径：磁盘利用率映射到 4~18px */
  const scaleR = (disk: number) => 4 + (disk / 100) * 14;

  /** 状态对应的颜色 */
  const statusColor: Record<string, string> = {
    overload: '#ff6b6b',
    normal: '#4da6ff',
    idle: '#86909C',
  };

  return (
    <svg viewBox={`0 0 ${width} ${height}`} style={{ width: '100%', maxHeight: 320 }}>
      {/* 过载区域背景（CPU>80 或 MEM>80） */}
      <rect
        x={scaleX(80)}
        y={padding.top}
        width={scaleX(100) - scaleX(80)}
        height={chartH}
        fill="#ff6b6b"
        opacity={0.04}
      />
      <rect
        x={padding.left}
        y={scaleY(100)}
        width={chartW}
        height={scaleY(80) - scaleY(100)}
        fill="#ff6b6b"
        opacity={0.04}
      />

      {/* 闲置区域背景（CPU<20 且 MEM<20） */}
      <rect
        x={padding.left}
        y={scaleY(20)}
        width={scaleX(20) - padding.left}
        height={scaleY(0) - scaleY(20)}
        fill="#86909C"
        opacity={0.06}
      />

      {/* 网格线 */}
      {[0, 20, 40, 60, 80, 100].map((v) => (
        <g key={`grid-${v}`}>
          {/* 横向网格线 */}
          <line
            x1={padding.left}
            y1={scaleY(v)}
            x2={padding.left + chartW}
            y2={scaleY(v)}
            stroke="#E5E6EB"
            strokeDasharray="3,3"
            opacity={0.5}
          />
          {/* Y 轴标签 */}
          <text x={padding.left - 8} y={scaleY(v) + 4} textAnchor="end" fontSize={10} fill="#86909C">
            {v}%
          </text>
          {/* 纵向网格线 */}
          <line
            x1={scaleX(v)}
            y1={padding.top}
            x2={scaleX(v)}
            y2={padding.top + chartH}
            stroke="#E5E6EB"
            strokeDasharray="3,3"
            opacity={0.5}
          />
          {/* X 轴标签 */}
          <text x={scaleX(v)} y={height - 10} textAnchor="middle" fontSize={10} fill="#86909C">
            {v}%
          </text>
        </g>
      ))}

      {/* 轴标题 */}
      <text x={width / 2} y={height - 0} textAnchor="middle" fontSize={11} fill="#86909C">
        CPU 利用率
      </text>
      <text
        x={12}
        y={height / 2}
        textAnchor="middle"
        fontSize={11}
        fill="#86909C"
        transform={`rotate(-90, 12, ${height / 2})`}
      >
        内存利用率
      </text>

      {/* 散点气泡 */}
      {data.map((point, i) => (
        <Tooltip key={i} title={`${point.hostname}: CPU ${point.cpu}% / MEM ${point.mem}% / Disk ${point.disk}%`}>
          <circle
            cx={scaleX(point.cpu)}
            cy={scaleY(point.mem)}
            r={scaleR(point.disk)}
            fill={statusColor[point.status]}
            opacity={0.7}
            stroke={statusColor[point.status]}
            strokeWidth={1}
            strokeOpacity={0.3}
            style={{ cursor: 'pointer' }}
          />
        </Tooltip>
      ))}
    </svg>
  );
};

/* ============================== 主组件 ============================== */

/**
 * 资产概览仪表盘页面
 * 顶部翻牌卡片 → 中间双栏（Treemap + 健康矩阵）→ 散点图 → 过保资产表格
 */
const AssetOverview: React.FC = () => {
  const { t } = useTranslation('cmdb');

  /** 健康度矩阵数据 */
  const [healthCells, setHealthCells] = useState<HealthCell[]>([]);

  /** 组件挂载时生成模拟数据 */
  useEffect(() => {
    setHealthCells(generateMockHealthCells());
  }, []);

  /** 过保资产表格列定义 */
  const expiredColumns = [
    {
      title: t('overview.expired.name'),
      dataIndex: 'name',
      key: 'name',
      width: 200,
    },
    {
      title: t('overview.expired.type'),
      dataIndex: 'type',
      key: 'type',
      width: 100,
      /** 根据类型渲染不同颜色的标签 */
      render: (type: string) => {
        const colorMap: Record<string, string> = {
          '服务器': '#4da6ff',
          '网络设备': '#00e5a0',
          '存储设备': '#ffaa33',
        };
        return <Tag color={colorMap[type] || '#86909C'}>{type}</Tag>;
      },
    },
    {
      title: t('overview.expired.date'),
      dataIndex: 'expireDate',
      key: 'expireDate',
      width: 120,
    },
    {
      title: t('overview.expired.remain'),
      dataIndex: 'remainDays',
      key: 'remainDays',
      width: 100,
      /** 剩余天数为负数时标红，正数显示橙色警告 */
      render: (days: number) => (
        <Text style={{ color: days < 0 ? '#ff6b6b' : days < 30 ? '#ffaa33' : '#86909C', fontWeight: 600 }}>
          {days < 0 ? `${t('overview.expired.overdue')} ${Math.abs(days)} ${t('overview.expired.days')}` : `${days} ${t('overview.expired.days')}`}
        </Text>
      ),
    },
    {
      title: t('overview.expired.owner'),
      dataIndex: 'owner',
      key: 'owner',
      width: 100,
    },
  ];

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('overview.title')}</Text>
      </div>

      {/* ---- 第一行：4 张翻牌统计卡片 ---- */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <MetricFlipCard
            label={t('overview.card.total')}
            value={1247}
            icon={<CloudServerOutlined />}
            trend="up"
            trendValue="+12 本周"
            color="#4da6ff"
            backItems={[
              { label: '环比', value: '+3.2%' },
              { label: '同比', value: '+15.8%' },
              { label: '峰值', value: '1280' },
              { label: '均值', value: '1195' },
            ]}
          />
        </Col>
        <Col span={6}>
          <MetricFlipCard
            label={t('overview.card.healthy')}
            value={91}
            suffix="%"
            trend="up"
            trendValue="+0.5%"
            color="#00e5a0"
            scanColor="rgba(0,229,160,0.5)"
            backItems={[
              { label: '环比', value: '+0.5%' },
              { label: '同比', value: '+2.1%' },
              { label: '最低', value: '87.3%' },
              { label: '异常次数', value: '14' },
            ]}
          />
        </Col>
        <Col span={6}>
          <MetricFlipCard
            label={t('overview.card.expired')}
            value={23}
            trend="down"
            trendValue="-3 已处理"
            color="#ff6b6b"
            scanColor="rgba(255,107,107,0.5)"
            backItems={[
              { label: '已过保', value: '8' },
              { label: '30天内', value: '6' },
              { label: '90天内', value: '9' },
              { label: '待处理', value: '15' },
            ]}
          />
        </Col>
        <Col span={6}>
          <MetricFlipCard
            label={t('overview.card.idle')}
            value={45}
            trend="down"
            trendValue="-5 已回收"
            color="#ffaa33"
            scanColor="rgba(255,170,51,0.5)"
            backItems={[
              { label: '服务器', value: '18' },
              { label: '网络', value: '12' },
              { label: '存储', value: '8' },
              { label: '容器', value: '7' },
            ]}
          />
        </Col>
      </Row>

      {/* ---- 第二行：资产分类 Treemap + 健康度热力矩阵 ---- */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={14}>
          <Card
            title={t('overview.treemap.title')}
            style={{ borderRadius: 12 }}
            styles={{ body: { padding: 16 } }}
          >
            <AssetTreemap data={MOCK_CATEGORIES} />
          </Card>
        </Col>
        <Col span={10}>
          <Card
            title={t('overview.health.title')}
            style={{ borderRadius: 12 }}
            styles={{ body: { padding: 8 } }}
          >
            <HealthMatrix
              cells={healthCells}
              rows={5}
              cols={5}
              cellHeight={28}
              colLabels={['NET', 'HOST', 'APP', 'DB', 'MW']}
              rowLabels={['支付', '订单', '用户', '风控', '基础设施']}
            />
          </Card>
        </Col>
      </Row>

      {/* ---- 第三行：资源利用率散点图 ---- */}
      <Card
        title={t('overview.scatter.title')}
        style={{ borderRadius: 12, marginBottom: 16 }}
        styles={{ body: { padding: 16 } }}
        extra={
          <div style={{ display: 'flex', gap: 16, fontSize: 12 }}>
            <span><span style={{ display: 'inline-block', width: 10, height: 10, borderRadius: '50%', background: '#ff6b6b', marginRight: 4 }} />{t('overview.scatter.overload')}</span>
            <span><span style={{ display: 'inline-block', width: 10, height: 10, borderRadius: '50%', background: '#4da6ff', marginRight: 4 }} />{t('overview.scatter.normal')}</span>
            <span><span style={{ display: 'inline-block', width: 10, height: 10, borderRadius: '50%', background: '#86909C', marginRight: 4 }} />{t('overview.scatter.idle')}</span>
          </div>
        }
      >
        <ResourceScatter data={MOCK_RESOURCE_POINTS} />
      </Card>

      {/* ---- 第四行：过保资产列表 ---- */}
      <Card
        title={t('overview.expired.title')}
        style={{ borderRadius: 12 }}
        styles={{ body: { padding: 0 } }}
      >
        <Table
          columns={expiredColumns}
          dataSource={MOCK_EXPIRED_ASSETS}
          rowKey="id"
          size="middle"
          pagination={false}
          locale={{ emptyText: t('assets.noData') }}
        />
      </Card>
    </div>
  );
};

export default AssetOverview;
