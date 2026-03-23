/**
 * 指标报告页面 - 展示关联分析和交易分析数据，支持导出 CSV 报表
 * 包含：过滤条件栏、概览卡片行、关联分析/交易分析 Tab 切换、导出按钮
 * 数据来源：GET /api/analytics/metrics
 * 导出接口：GET /api/analytics/reports/{id}/export?format=csv
 */
import React, { useState, useEffect, useCallback, useRef } from 'react';
import * as echarts from 'echarts';
import {
  Card, Row, Col, Table, Select, Space, Typography, Tabs, Tag, Button,
  message,
} from 'antd';
import { ExportOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  fetchMetrics, exportReport,
  type CorrelationRecord, type TransactionRecord, type MetricsSummary, type MetricsResult,
} from '../api/analytics';

const { Text } = Typography;

/** 风险评分等级对应的颜色映射 */
const RISK_COLOR: Record<string, string> = {
  high: '#F53F3F',    // 高风险 - 红色
  medium: '#FF7D00',  // 中风险 - 橙色
  low: '#00B42A',     // 低风险 - 绿色
};

/**
 * 指标报告组件
 * - 顶部：页面标题 + 导出按钮
 * - 过滤栏：时间周期、业务板块、资产类型
 * - 概览卡片行：告警总数、事件总数、最高风险资产、平均错误率
 * - Tab 切换：关联分析（资产-告警-事件关联表）、交易分析（服务 QPS/P99/错误率）
 */
const MetricsReport: React.FC = () => {
  const { t } = useTranslation('analytics');
  const [activeTab, setActiveTab] = useState('correlation');          // 当前活动 Tab
  const [loading, setLoading] = useState(false);                     // 数据加载状态
  const [correlationData, setCorrelationData] = useState<CorrelationRecord[]>([]); // 关联分析数据
  const [transactionData, setTransactionData] = useState<TransactionRecord[]>([]); // 交易分析数据
  const [summary, setSummary] = useState<MetricsSummary | null>(null); // 概览统计数据
  const [period, setPeriod] = useState('7d');                        // 当前选中的时间周期
  const [exporting, setExporting] = useState(false);                 // 导出进行中状态

  const riskChartRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!riskChartRef.current) return;
    const chart = echarts.init(riskChartRef.current);
    chart.setOption({
      grid: { left: 50, right: 20, top: 20, bottom: 40 },
      tooltip: { trigger: 'axis' },
      xAxis: { type: 'category', data: ['网络设备', '应用服务', '数据库', '存储', '中间件', '云主机'] },
      yAxis: { type: 'value', name: '告警次数' },
      series: [{
        type: 'bar',
        data: [
          { value: 42, itemStyle: { color: '#F53F3F' } },
          { value: 28, itemStyle: { color: '#FF7D00' } },
          { value: 19, itemStyle: { color: '#FF7D00' } },
          { value: 13, itemStyle: { color: '#00B42A' } },
          { value: 9,  itemStyle: { color: '#00B42A' } },
          { value: 6,  itemStyle: { color: '#00B42A' } },
        ],
        barMaxWidth: 40,
      }],
    });
    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);
    return () => { window.removeEventListener('resize', handleResize); chart.dispose(); };
  }, [activeTab]);

  /**
   * 加载指标分析数据
   * 调用 GET /api/analytics/metrics 获取概览和列表数据
   * @param currentPeriod 时间周期参数
   */
  const loadData = useCallback(async (currentPeriod?: string) => {
    setLoading(true);
    try {
      // request<T> 已自动解包 ApiResponse.data，直接获取 MetricsResult
      const result = await fetchMetrics({ period: currentPeriod || period });
      setCorrelationData(result.correlationList || []);
      setTransactionData(result.transactionList || []);
      setSummary(result.summary || null);
    } catch {
      // API 尚未就绪，保持空状态
    } finally {
      setLoading(false);
    }
  }, [period]);

  /** 组件挂载及依赖变化时加载数据 */
  useEffect(() => {
    loadData();
  }, [loadData]);

  /**
   * 处理时间周期切换
   * @param value 周期值（24h/7d/30d/90d）
   */
  const handlePeriodChange = (value: string) => {
    setPeriod(value);
    loadData(value);
  };

  /**
   * 处理导出报表
   * 调用 GET /api/analytics/reports/{id}/export?format=csv
   * 将返回的 Blob 创建为可下载链接并自动触发下载
   */
  const handleExport = useCallback(async () => {
    setExporting(true);
    try {
      // 使用当前 Tab 类型作为报表 ID
      const reportId = activeTab === 'correlation' ? 'correlation' : 'transaction';
      const blob = await exportReport(reportId, 'csv');
      // 创建下载链接并触发浏览器下载
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `${reportId}_report_${new Date().toISOString().slice(0, 10)}.csv`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
      message.success(t('metrics.exportSuccess'));
    } catch {
      message.error(t('metrics.exportFail'));
    } finally {
      setExporting(false);
    }
  }, [activeTab, t]);

  /** 关联分析表格列定义 */
  const correlationColumns = [
    { title: t('metrics.correlation.asset'), dataIndex: 'asset', key: 'asset' },
    { title: t('metrics.correlation.alertCount'), dataIndex: 'alertCount', key: 'alertCount', width: 100, sorter: true },
    { title: t('metrics.correlation.incidentCount'), dataIndex: 'incidentCount', key: 'incidentCount', width: 100, sorter: true },
    { title: t('metrics.correlation.avgMTTR'), dataIndex: 'avgMTTR', key: 'avgMTTR', width: 100 },
    {
      title: t('metrics.correlation.riskScore'),
      dataIndex: 'riskScore',
      key: 'riskScore',
      width: 120,
      sorter: true,
      /** 渲染风险评分标签，根据分值动态着色（>=80 高危，>=50 中危，<50 低危） */
      render: (score: number) => {
        const level = score >= 80 ? 'high' : score >= 50 ? 'medium' : 'low';
        return <Tag color={RISK_COLOR[level]}>{score}</Tag>;
      },
    },
  ];

  /** 交易分析表格列定义 */
  const transactionColumns = [
    { title: t('metrics.transaction.service'), dataIndex: 'service', key: 'service' },
    { title: t('metrics.transaction.qps'), dataIndex: 'qps', key: 'qps', width: 100, sorter: true },
    { title: t('metrics.transaction.p99'), dataIndex: 'p99', key: 'p99', width: 100 },
    { title: t('metrics.transaction.errorRate'), dataIndex: 'errorRate', key: 'errorRate', width: 100 },
    {
      title: t('metrics.transaction.trend'),
      dataIndex: 'trend',
      key: 'trend',
      width: 80,
      /** 渲染趋势方向，上升红色、下降绿色、稳定灰色 */
      render: (trend: string) => {
        const color = trend === 'up' ? '#F53F3F' : trend === 'down' ? '#00B42A' : '#86909C';
        return <Text style={{ color }}>{t(`metrics.transaction.trend_${trend}`)}</Text>;
      },
    },
  ];

  /** Tab 配置项：关联分析 + 交易分析 */
  const tabItems = [
    {
      key: 'correlation',
      label: t('metrics.tab.correlation'),
      children: (
        <div>
          {/* 关联风险分布图 */}
          <Card style={{ borderRadius: 8, marginBottom: 16 }}>
            <div ref={riskChartRef} style={{ height: 220, width: '100%', marginBottom: 16 }} />
          </Card>

          {/* 关联分析数据表格 */}
          <Table<CorrelationRecord>
            columns={correlationColumns}
            dataSource={correlationData}
            loading={loading}
            locale={{ emptyText: t('metrics.noData') }}
            pagination={false}
            size="middle"
          />
        </div>
      ),
    },
    {
      key: 'transaction',
      label: t('metrics.tab.transaction'),
      children: (
        <div>
          {/* 交易趋势图表占位 */}
          <Card style={{ borderRadius: 8, marginBottom: 16, minHeight: 200 }}>
            <div style={{ textAlign: 'center', color: '#86909C', padding: 48 }}>
              {t('metrics.transaction.chartPlaceholder')}
            </div>
          </Card>

          {/* 交易分析数据表格 */}
          <Table<TransactionRecord>
            columns={transactionColumns}
            dataSource={transactionData}
            loading={loading}
            locale={{ emptyText: t('metrics.noData') }}
            pagination={false}
            size="middle"
          />
        </div>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与导出按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('metrics.title')}</Text>
        <Button icon={<ExportOutlined />} loading={exporting} onClick={handleExport}>
          {t('metrics.export')}
        </Button>
      </div>

      {/* 过滤条件栏 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Space>
          {/* 时间周期选择 */}
          <Select placeholder={t('metrics.filter.period')} style={{ width: 140 }} value={period}
            onChange={handlePeriodChange}
            options={[
              { value: '24h', label: t('metrics.filter.24h') },
              { value: '7d', label: t('metrics.filter.7d') },
              { value: '30d', label: t('metrics.filter.30d') },
              { value: '90d', label: t('metrics.filter.90d') },
            ]}
          />
          {/* 业务板块过滤 */}
          <Select placeholder={t('metrics.filter.business')} style={{ width: 140 }} allowClear />
          {/* 资产类型过滤 */}
          <Select placeholder={t('metrics.filter.assetType')} style={{ width: 140 }} allowClear />
        </Space>
      </Card>

      {/* 概览统计卡片行 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {[
          { key: 'totalAlerts', label: t('metrics.summary.totalAlerts'), value: summary?.totalAlerts ?? '--' },
          { key: 'totalIncidents', label: t('metrics.summary.totalIncidents'), value: summary?.totalIncidents ?? '--' },
          { key: 'topRisk', label: t('metrics.summary.topRisk'), value: summary?.topRisk ?? '--' },
          { key: 'avgErrorRate', label: t('metrics.summary.avgErrorRate'), value: summary?.avgErrorRate ?? '--' },
        ].map((card) => (
          <Col span={6} key={card.key}>
            <Card bordered style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
              bodyStyle={{ padding: '16px 20px', textAlign: 'center' }}
            >
              <div style={{ color: '#86909C', fontSize: 14 }}>{card.label}</div>
              <div style={{ fontSize: 28, fontWeight: 600, marginTop: 4 }}>{card.value}</div>
            </Card>
          </Col>
        ))}
      </Row>

      {/* 关联分析 / 交易分析 Tab 面板 */}
      <Card style={{ borderRadius: 8 }}>
        <Tabs items={tabItems} activeKey={activeTab} onChange={setActiveTab} />
      </Card>
    </div>
  );
};

export default MetricsReport;
