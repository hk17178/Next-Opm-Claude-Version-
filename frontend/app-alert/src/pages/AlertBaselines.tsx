import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  Card, Row, Col, Table, Button, Space, Typography, Tag, Select,
  Modal, Form, Input, InputNumber, message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  PlusOutlined, ReloadOutlined, CheckCircleOutlined,
  SyncOutlined, ExclamationCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import * as echarts from 'echarts';

const { Text } = Typography;

/* ================================================================== */
/*  类型定义                                                           */
/* ================================================================== */

/** 基线记录状态 */
type BaselineStatus = 'normal' | 'learning' | 'anomaly';

/** 基线阈值类型 */
type ThresholdType = 'static' | 'dynamic' | 'percentile';

/** 基线训练算法类型 */
type AlgorithmType = 'moving_average' | 'exponential_smoothing' | 'prophet' | 'std_deviation';

/** 基线记录 */
interface BaselineRecord {
  key: string;
  /** 指标名称 */
  metricName: string;
  /** 阈值类型 */
  thresholdType: ThresholdType;
  /** 当前值 */
  currentValue: number;
  /** 基线值 */
  baselineValue: number;
  /** 偏差率 */
  deviationRate: string;
  /** 基线状态 */
  status: BaselineStatus;
  /** 训练算法 */
  algorithm: AlgorithmType;
  /** 训练周期（天） */
  trainingPeriod: number;
  /** 更新时间 */
  updatedAt: string;
}

/* ================================================================== */
/*  Mock 数据                                                          */
/* ================================================================== */

/** 基线列表 Mock 数据 */
const BASELINE_DATA: BaselineRecord[] = [
  { key: '1', metricName: 'cpu_usage_percent', thresholdType: 'dynamic', currentValue: 68.5, baselineValue: 55.2, deviationRate: '+24.1%', status: 'anomaly', algorithm: 'exponential_smoothing', trainingPeriod: 14, updatedAt: '2026-03-24 10:30' },
  { key: '2', metricName: 'memory_usage_percent', thresholdType: 'dynamic', currentValue: 72.3, baselineValue: 70.8, deviationRate: '+2.1%', status: 'normal', algorithm: 'moving_average', trainingPeriod: 7, updatedAt: '2026-03-24 10:30' },
  { key: '3', metricName: 'disk_io_latency_ms', thresholdType: 'percentile', currentValue: 12.8, baselineValue: 8.5, deviationRate: '+50.6%', status: 'anomaly', algorithm: 'std_deviation', trainingPeriod: 30, updatedAt: '2026-03-24 10:25' },
  { key: '4', metricName: 'http_request_rate', thresholdType: 'dynamic', currentValue: 1250, baselineValue: 1180, deviationRate: '+5.9%', status: 'normal', algorithm: 'prophet', trainingPeriod: 14, updatedAt: '2026-03-24 10:20' },
  { key: '5', metricName: 'redis_connection_count', thresholdType: 'static', currentValue: 85, baselineValue: 80, deviationRate: '+6.3%', status: 'normal', algorithm: 'moving_average', trainingPeriod: 7, updatedAt: '2026-03-24 10:15' },
  { key: '6', metricName: 'kafka_consumer_lag', thresholdType: 'dynamic', currentValue: 5200, baselineValue: 3800, deviationRate: '+36.8%', status: 'anomaly', algorithm: 'exponential_smoothing', trainingPeriod: 14, updatedAt: '2026-03-24 10:10' },
  { key: '7', metricName: 'jvm_gc_pause_ms', thresholdType: 'percentile', currentValue: 120, baselineValue: 95, deviationRate: '+26.3%', status: 'learning', algorithm: 'std_deviation', trainingPeriod: 30, updatedAt: '2026-03-24 10:05' },
  { key: '8', metricName: 'network_packet_loss', thresholdType: 'dynamic', currentValue: 0.05, baselineValue: 0.02, deviationRate: '+150%', status: 'anomaly', algorithm: 'moving_average', trainingPeriod: 7, updatedAt: '2026-03-24 10:00' },
  { key: '9', metricName: 'db_query_duration_ms', thresholdType: 'dynamic', currentValue: 45, baselineValue: 42, deviationRate: '+7.1%', status: 'normal', algorithm: 'prophet', trainingPeriod: 14, updatedAt: '2026-03-24 09:55' },
  { key: '10', metricName: 'container_restart_count', thresholdType: 'static', currentValue: 2, baselineValue: 0, deviationRate: '+200%', status: 'learning', algorithm: 'moving_average', trainingPeriod: 7, updatedAt: '2026-03-24 09:50' },
];

/** 基线趋势图 Mock 数据：最近 24 小时 */
const generateTrendLabels = (): string[] =>
  Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:00`);

/** 基线值（稳定线） */
const BASELINE_TREND_VALUES = [55, 54, 53, 52, 52, 53, 55, 58, 60, 62, 63, 64, 63, 62, 60, 58, 56, 55, 54, 53, 53, 54, 55, 55];
/** 实际值（波动线） */
const ACTUAL_TREND_VALUES = [52, 50, 48, 47, 49, 55, 62, 70, 75, 68, 65, 67, 72, 64, 58, 56, 53, 52, 50, 48, 50, 53, 56, 58];
/** 置信区间上界 */
const UPPER_BOUND_VALUES = BASELINE_TREND_VALUES.map((v) => v + 12);
/** 置信区间下界 */
const LOWER_BOUND_VALUES = BASELINE_TREND_VALUES.map((v) => v - 12);

/* ================================================================== */
/*  常量定义                                                           */
/* ================================================================== */

/** 状态颜色映射 */
const STATUS_CONFIG: Record<BaselineStatus, { color: string; icon: React.ReactNode; label: string }> = {
  normal: { color: 'success', icon: <CheckCircleOutlined />, label: 'baselines.status.normal' },
  learning: { color: 'processing', icon: <SyncOutlined spin />, label: 'baselines.status.learning' },
  anomaly: { color: 'error', icon: <ExclamationCircleOutlined />, label: 'baselines.status.anomaly' },
};

/** 可选指标列表 */
const METRIC_OPTIONS = [
  'cpu_usage_percent',
  'memory_usage_percent',
  'disk_io_latency_ms',
  'http_request_rate',
  'redis_connection_count',
  'kafka_consumer_lag',
  'jvm_gc_pause_ms',
  'network_packet_loss',
  'db_query_duration_ms',
  'container_restart_count',
];

/* ================================================================== */
/*  组件                                                               */
/* ================================================================== */

/**
 * 告警基线管理页面
 *
 * 功能模块：
 * 1. 基线列表表格（指标名/阈值类型/当前值/基线值/偏差率/状态/操作）
 * 2. 创建基线弹窗（选择指标+算法+训练周期）
 * 3. 基线趋势图（ECharts 折线+置信区间带）
 * 4. 状态筛选（正常/学习中/异常）
 */
const AlertBaselines: React.FC = () => {
  const { t } = useTranslation('alert');

  // ---- 状态管理 ----
  /** 基线列表数据 */
  const [data, setData] = useState<BaselineRecord[]>(BASELINE_DATA);
  /** 状态筛选 */
  const [statusFilter, setStatusFilter] = useState<BaselineStatus | undefined>();
  /** 创建弹窗可见性 */
  const [createModalOpen, setCreateModalOpen] = useState(false);
  /** 创建表单实例 */
  const [form] = Form.useForm();
  /** 当前选中查看趋势的基线记录 */
  const [selectedBaseline, setSelectedBaseline] = useState<BaselineRecord | null>(BASELINE_DATA[0]);

  // ---- ECharts 引用 ----
  /** 基线趋势图容器 */
  const trendChartRef = useRef<HTMLDivElement>(null);
  const trendChartInstance = useRef<echarts.ECharts | null>(null);

  /** 根据状态筛选过滤数据 */
  const filteredData = statusFilter
    ? data.filter((item) => item.status === statusFilter)
    : data;

  /** 处理创建基线提交 */
  const handleCreateBaseline = useCallback(async () => {
    try {
      const values = await form.validateFields();
      // 模拟创建成功
      const newBaseline: BaselineRecord = {
        key: String(Date.now()),
        metricName: values.metric,
        thresholdType: 'dynamic',
        currentValue: 0,
        baselineValue: 0,
        deviationRate: '0%',
        status: 'learning',
        algorithm: values.algorithm,
        trainingPeriod: values.trainingPeriod,
        updatedAt: new Date().toLocaleString('zh-CN'),
      };
      setData((prev) => [newBaseline, ...prev]);
      message.success(t('baselines.createSuccess'));
      setCreateModalOpen(false);
      form.resetFields();
    } catch {
      // 表单校验失败，antd 自动显示错误提示
    }
  }, [form, t]);

  /** 处理删除基线 */
  const handleDeleteBaseline = useCallback((key: string) => {
    setData((prev) => prev.filter((item) => item.key !== key));
    message.success(t('baselines.deleteSuccess'));
  }, [t]);

  /* ---------- 基线趋势图初始化（折线 + 置信区间带） ---------- */
  useEffect(() => {
    if (!trendChartRef.current) return;

    if (trendChartInstance.current) {
      trendChartInstance.current.dispose();
    }

    const chart = echarts.init(trendChartRef.current);
    trendChartInstance.current = chart;

    const labels = generateTrendLabels();

    chart.setOption({
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(10,16,28,0.9)',
        borderColor: 'rgba(60,140,255,0.15)',
        textStyle: { color: '#e2e8f0', fontSize: 11 },
      },
      legend: {
        data: [
          t('baselines.chart.actual'),
          t('baselines.chart.baseline'),
          t('baselines.chart.confidenceBand'),
        ],
        top: 6,
        right: 14,
        textStyle: { color: 'rgba(140,170,210,0.5)', fontSize: 10 },
        itemWidth: 12,
        itemHeight: 3,
      },
      grid: { left: 40, right: 14, top: 40, bottom: 28 },
      xAxis: {
        type: 'category',
        data: labels,
        boundaryGap: false,
        axisLine: { lineStyle: { color: 'rgba(60,140,255,0.08)' } },
        axisLabel: { color: 'rgba(140,170,210,0.4)', fontSize: 9, interval: 3 },
        axisTick: { show: false },
      },
      yAxis: {
        type: 'value',
        splitLine: { lineStyle: { color: 'rgba(60,140,255,0.05)' } },
        axisLine: { show: false },
        axisLabel: { color: 'rgba(140,170,210,0.4)', fontSize: 9 },
      },
      series: [
        // 置信区间上界（不可见，用于堆叠区域的顶部）
        {
          name: t('baselines.chart.confidenceBand'),
          type: 'line',
          data: UPPER_BOUND_VALUES,
          smooth: true,
          symbol: 'none',
          lineStyle: { opacity: 0 },
          // 置信区间带：上界与下界之间的区域
          areaStyle: {
            color: 'rgba(77,166,255,0.1)',
          },
          stack: 'confidence',
        },
        // 置信区间下界（不可见，用于堆叠区域的底部）
        {
          name: '下界',
          type: 'line',
          data: LOWER_BOUND_VALUES,
          smooth: true,
          symbol: 'none',
          lineStyle: { opacity: 0 },
          areaStyle: {
            color: 'rgba(255,255,255,0)', // 透明底部
          },
          stack: 'confidence',
          // 隐藏图例
          showSymbol: false,
        },
        // 基线值（稳定线）
        {
          name: t('baselines.chart.baseline'),
          type: 'line',
          data: BASELINE_TREND_VALUES,
          smooth: true,
          symbol: 'none',
          lineStyle: { color: '#4da6ff', width: 2, type: 'dashed' },
        },
        // 实际值（波动线）
        {
          name: t('baselines.chart.actual'),
          type: 'line',
          data: ACTUAL_TREND_VALUES,
          smooth: true,
          symbol: 'none',
          lineStyle: { color: '#00e5a0', width: 2 },
          // 实际值超出置信区间时高亮显示
          markPoint: {
            data: ACTUAL_TREND_VALUES
              .map((v, i) => {
                if (v > UPPER_BOUND_VALUES[i] || v < LOWER_BOUND_VALUES[i]) {
                  return {
                    coord: [labels[i], v],
                    symbol: 'circle',
                    symbolSize: 6,
                    itemStyle: { color: '#F53F3F' },
                  };
                }
                return null;
              })
              .filter(Boolean) as echarts.MarkPointComponentOption['data'],
          },
        },
      ],
    });

    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      chart.dispose();
      trendChartInstance.current = null;
    };
  }, [selectedBaseline, t]);

  /* ---------- 表格列定义 ---------- */
  const columns: ColumnsType<BaselineRecord> = [
    {
      title: t('baselines.column.metricName'),
      dataIndex: 'metricName',
      key: 'metricName',
      render: (name: string) => (
        <Text code style={{ fontSize: 12 }}>{name}</Text>
      ),
    },
    {
      title: t('baselines.column.thresholdType'),
      dataIndex: 'thresholdType',
      key: 'thresholdType',
      width: 100,
      render: (type: ThresholdType) => {
        const typeColors: Record<ThresholdType, string> = {
          static: 'default',
          dynamic: 'blue',
          percentile: 'purple',
        };
        return <Tag color={typeColors[type]}>{t(`baselines.thresholdType.${type}`)}</Tag>;
      },
    },
    {
      title: t('baselines.column.currentValue'),
      dataIndex: 'currentValue',
      key: 'currentValue',
      width: 100,
      render: (val: number) => <Text strong>{val}</Text>,
    },
    {
      title: t('baselines.column.baselineValue'),
      dataIndex: 'baselineValue',
      key: 'baselineValue',
      width: 100,
    },
    {
      title: t('baselines.column.deviationRate'),
      dataIndex: 'deviationRate',
      key: 'deviationRate',
      width: 100,
      render: (rate: string) => {
        /** 解析偏差率数值，判断是否超阈值 */
        const numVal = parseFloat(rate.replace(/[+%]/g, ''));
        const color = numVal > 20 ? '#F53F3F' : numVal > 10 ? '#FF7D00' : '#00B42A';
        return <Text style={{ color, fontWeight: 600 }}>{rate}</Text>;
      },
    },
    {
      title: t('baselines.column.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: BaselineStatus) => {
        const config = STATUS_CONFIG[status];
        return (
          <Tag icon={config.icon} color={config.color}>
            {t(config.label)}
          </Tag>
        );
      },
    },
    {
      title: t('baselines.column.updatedAt'),
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 150,
    },
    {
      title: t('baselines.column.actions'),
      key: 'actions',
      width: 140,
      render: (_: unknown, record: BaselineRecord) => (
        <Space size={0}>
          <Button
            type="link"
            size="small"
            onClick={() => setSelectedBaseline(record)}
          >
            {t('baselines.action.viewTrend')}
          </Button>
          <Button
            type="link"
            size="small"
            danger
            onClick={() => handleDeleteBaseline(record.key)}
          >
            {t('baselines.action.delete')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* ---- 页面标题和操作栏 ---- */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('baselines.title')}</Text>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => setData([...BASELINE_DATA])} />
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateModalOpen(true)}
          >
            {t('baselines.createBaseline')}
          </Button>
        </Space>
      </div>

      {/* ---- 状态筛选 ---- */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Space>
          <Text style={{ color: '#86909C' }}>{t('baselines.filterStatus')}:</Text>
          <Select
            placeholder={t('baselines.filterStatus')}
            style={{ width: 150 }}
            allowClear
            value={statusFilter}
            onChange={setStatusFilter}
            options={[
              { value: 'normal', label: t('baselines.status.normal') },
              { value: 'learning', label: t('baselines.status.learning') },
              { value: 'anomaly', label: t('baselines.status.anomaly') },
            ]}
          />
          {/* 统计概览 */}
          <Tag color="success">{t('baselines.status.normal')}: {data.filter((d) => d.status === 'normal').length}</Tag>
          <Tag color="processing">{t('baselines.status.learning')}: {data.filter((d) => d.status === 'learning').length}</Tag>
          <Tag color="error">{t('baselines.status.anomaly')}: {data.filter((d) => d.status === 'anomaly').length}</Tag>
        </Space>
      </Card>

      <Row gutter={16}>
        {/* ---- 左侧：基线列表表格 ---- */}
        <Col span={14}>
          <Card
            title={t('baselines.listTitle')}
            bordered
            style={{ borderRadius: 8 }}
            bodyStyle={{ padding: '8px 16px' }}
          >
            <Table<BaselineRecord>
              columns={columns}
              dataSource={filteredData}
              pagination={{ pageSize: 10, size: 'small' }}
              size="small"
              rowKey="key"
              onRow={(record) => ({
                onClick: () => setSelectedBaseline(record),
                style: {
                  cursor: 'pointer',
                  background: selectedBaseline?.key === record.key ? 'rgba(77,166,255,0.06)' : undefined,
                },
              })}
            />
          </Card>
        </Col>

        {/* ---- 右侧：基线趋势图 ---- */}
        <Col span={10}>
          <Card
            title={
              <Space>
                <span>{t('baselines.trendTitle')}</span>
                {selectedBaseline && (
                  <Tag color="blue">{selectedBaseline.metricName}</Tag>
                )}
              </Space>
            }
            bordered
            style={{ borderRadius: 8 }}
            bodyStyle={{ padding: '8px 12px' }}
          >
            <div ref={trendChartRef} style={{ width: '100%', height: 380 }} />
            {/* 当前选中基线的详细信息 */}
            {selectedBaseline && (
              <div style={{ padding: '12px 8px 4px', borderTop: '1px solid rgba(60,140,255,0.08)' }}>
                <Row gutter={16}>
                  <Col span={8}>
                    <div style={{ fontSize: 12, color: '#86909C' }}>{t('baselines.detail.algorithm')}</div>
                    <div style={{ fontSize: 14, fontWeight: 600 }}>
                      {t(`baselines.algorithm.${selectedBaseline.algorithm}`)}
                    </div>
                  </Col>
                  <Col span={8}>
                    <div style={{ fontSize: 12, color: '#86909C' }}>{t('baselines.detail.trainingPeriod')}</div>
                    <div style={{ fontSize: 14, fontWeight: 600 }}>
                      {selectedBaseline.trainingPeriod} {t('baselines.detail.days')}
                    </div>
                  </Col>
                  <Col span={8}>
                    <div style={{ fontSize: 12, color: '#86909C' }}>{t('baselines.detail.status')}</div>
                    <Tag
                      icon={STATUS_CONFIG[selectedBaseline.status].icon}
                      color={STATUS_CONFIG[selectedBaseline.status].color}
                    >
                      {t(STATUS_CONFIG[selectedBaseline.status].label)}
                    </Tag>
                  </Col>
                </Row>
              </div>
            )}
          </Card>
        </Col>
      </Row>

      {/* ---- 创建基线弹窗 ---- */}
      <Modal
        title={t('baselines.createBaseline')}
        open={createModalOpen}
        onCancel={() => {
          setCreateModalOpen(false);
          form.resetFields();
        }}
        onOk={handleCreateBaseline}
        okText={t('baselines.form.submit')}
        cancelText={t('baselines.form.cancel')}
        width={520}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {/* 选择指标 */}
          <Form.Item
            name="metric"
            label={t('baselines.form.metric')}
            rules={[{ required: true, message: t('baselines.form.metricRequired') }]}
          >
            <Select
              placeholder={t('baselines.form.metricPlaceholder')}
              options={METRIC_OPTIONS.map((m) => ({ value: m, label: m }))}
              showSearch
            />
          </Form.Item>

          {/* 选择算法 */}
          <Form.Item
            name="algorithm"
            label={t('baselines.form.algorithm')}
            rules={[{ required: true, message: t('baselines.form.algorithmRequired') }]}
          >
            <Select
              placeholder={t('baselines.form.algorithmPlaceholder')}
              options={[
                { value: 'moving_average', label: t('baselines.algorithm.moving_average') },
                { value: 'exponential_smoothing', label: t('baselines.algorithm.exponential_smoothing') },
                { value: 'prophet', label: t('baselines.algorithm.prophet') },
                { value: 'std_deviation', label: t('baselines.algorithm.std_deviation') },
              ]}
            />
          </Form.Item>

          {/* 训练周期 */}
          <Form.Item
            name="trainingPeriod"
            label={t('baselines.form.trainingPeriod')}
            rules={[{ required: true, message: t('baselines.form.trainingPeriodRequired') }]}
            initialValue={14}
          >
            <InputNumber
              min={1}
              max={90}
              addonAfter={t('baselines.detail.days')}
              style={{ width: '100%' }}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default AlertBaselines;
