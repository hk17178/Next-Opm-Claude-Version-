import React, { useState, useCallback, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Card, Row, Col, Tabs, Button, Space, Typography, Select, Input, Modal, Tag, message,
} from 'antd';
import {
  PlusOutlined, LockOutlined, ArrowUpOutlined, ArrowDownOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { Skeleton as AntSkeleton, Empty as AntEmpty } from 'antd';
import { VirtualTable, useColumnConfig, type ColumnDef } from '@opsnexus/ui-kit';
import {
  fetchAlerts, acknowledgeAlert,
  type AlertRecord, type AlertListResult,
} from '../api/alert';

const { Text } = Typography;

/** 告警严重程度颜色映射（P0 最高 ~ P4 最低） */
const SEVERITY_COLORS: Record<string, string> = {
  P0: '#F53F3F',
  P1: '#FF7D00',
  P2: '#3491FA',
  P3: '#86909C',
  P4: '#C9CDD4',
};

/** 自动轮询刷新间隔：30 秒 */
const POLL_INTERVAL = 30_000;

/**
 * 告警列表页面组件
 * 功能：告警列表展示（虚拟滚动）、列配置管理、状态 Tab 切换、多维过滤、告警确认（ACK）、30 秒自动轮询刷新
 */
const AlertList: React.FC = () => {
  const { t } = useTranslation('alert');
  const navigate = useNavigate();
  // ---- 数据状态 ----
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<AlertRecord[]>([]);                               // 告警列表数据
  const [total, setTotal] = useState(0);                                              // 匹配总数
  const [page, setPage] = useState(1);                                                // 当前页码
  const [pageSize] = useState(20);                                                    // 每页条数
  const [stats, setStats] = useState<AlertListResult['stats'] | null>(null);          // 统计卡片数据
  const [activeTab, setActiveTab] = useState('firing');                                // 当前激活的状态 Tab
  const [confirmModalOpen, setConfirmModalOpen] = useState(false);                    // 确认弹窗可见性
  const [selectedAlert, setSelectedAlert] = useState<AlertRecord | null>(null);       // 当前待确认的告警
  const [ackLoading, setAckLoading] = useState(false);                                // 确认操作加载状态

  // ---- 过滤条件状态 ----
  const [severityFilter, setSeverityFilter] = useState<string | undefined>();   // 严重程度过滤
  const [sourceFilter, setSourceFilter] = useState<string | undefined>();       // 来源过滤
  const [businessFilter, setBusinessFilter] = useState<string | undefined>();   // 业务板块过滤
  const [layerFilter, setLayerFilter] = useState<number | undefined>();         // 告警层级过滤
  const [keyword, setKeyword] = useState('');                                   // 关键词搜索

  /** 标记是否已完成首次数据加载（用于显示骨架屏） */
  const [initialLoaded, setInitialLoaded] = useState(false);

  /** 轮询定时器引用 */
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  /** 默认列定义 */
  const defaultColumns: ColumnDef<AlertRecord>[] = [
    {
      title: t('list.column.severity'),
      dataIndex: 'severity',
      key: 'severity',
      width: 80,
      render: (severity: unknown) => (
        <Tag
          style={{
            background: SEVERITY_COLORS[severity as string],
            color: '#fff',
            borderRadius: 4,
            height: 22,
            lineHeight: '20px',
            fontWeight: 600,
            fontSize: 12,
            border: 'none',
          }}
        >
          {severity as string}
        </Tag>
      ),
    },
    {
      title: t('list.column.content'),
      dataIndex: 'content',
      key: 'content',
      ellipsis: true,
      render: (text: unknown, record: AlertRecord) => (
        <Space>
          {record.isIronRule && <LockOutlined style={{ color: '#F53F3F', fontSize: 12 }} />}
          <span>{text as string}</span>
        </Space>
      ),
    },
    {
      title: t('list.column.source'),
      dataIndex: 'source',
      key: 'source',
      width: 130,
    },
    {
      title: t('list.column.triggerTime'),
      dataIndex: 'triggerTime',
      key: 'triggerTime',
      width: 160,
      sorter: true,
    },
    {
      title: t('list.column.duration'),
      dataIndex: 'duration',
      key: 'duration',
      width: 100,
    },
    {
      title: t('list.column.actions'),
      key: 'actions',
      width: 100,
      render: (_: unknown, record: AlertRecord) => (
        <Space size={0}>
          <Button type="link" size="small" onClick={() => navigate(`/detail/${record.id}`)}>详情</Button>
          <Button
            type="link"
            size="small"
            onClick={(e) => {
              e.stopPropagation();
              handleAcknowledge(record);
            }}
            disabled={record.status === 'acknowledged'}
          >
            {t('list.action.acknowledge')}
          </Button>
        </Space>
      ),
    },
  ];

  /** 使用 useColumnConfig 管理列配置（支持列显示/隐藏、排序、持久化） */
  const { columns, ColumnConfigButton } = useColumnConfig<AlertRecord>(
    'alert-list',
    defaultColumns,
  );

  /**
   * 从后端获取告警列表数据
   * @param currentPage - 要查询的页码，默认第 1 页
   */
  const fetchData = useCallback(async (currentPage = 1) => {
    setLoading(true);
    try {
      // "全部" Tab 不传 status 过滤
      const result = await fetchAlerts({
        status: activeTab === 'all' ? undefined : activeTab,
        severity: severityFilter,
        source: sourceFilter,
        business: businessFilter,
        layer: layerFilter,
        keyword: keyword || undefined,
        page: currentPage,
        pageSize,
      });
      setData(result.list);
      setTotal(result.total);
      setStats(result.stats);
      setPage(currentPage);
    } catch {
      // 后端 API 尚未就绪，显示空状态
      setData([]);
      setTotal(0);
    } finally {
      setLoading(false);
      setInitialLoaded(true);
    }
  }, [activeTab, severityFilter, sourceFilter, businessFilter, layerFilter, keyword, pageSize]);

  // 初始加载数据 + 启动 30 秒轮询定时器
  useEffect(() => {
    fetchData(1);

    // 每 30 秒自动刷新告警列表，确保实时性
    pollRef.current = setInterval(() => {
      fetchData(1);
    }, POLL_INTERVAL);

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [fetchData]);

  /** 点击确认按钮，打开确认弹窗 */
  const handleAcknowledge = useCallback((record: AlertRecord) => {
    setSelectedAlert(record);
    setConfirmModalOpen(true);
  }, []);

  /** 确认 ACK 操作，调用后端 PATCH /api/alerts/{id}/ack 接口 */
  const handleConfirmAcknowledge = useCallback(async () => {
    if (!selectedAlert) return;
    setAckLoading(true);
    try {
      await acknowledgeAlert(selectedAlert.id);
      message.success(t('list.action.ackSuccess'));
      setConfirmModalOpen(false);
      setSelectedAlert(null);
      fetchData(page);
    } catch {
      message.error(t('list.action.ackFailed'));
    } finally {
      setAckLoading(false);
    }
  }, [selectedAlert, fetchData, page, t]);

  /** 顶部统计卡片配置：触发中、今日新增、今日已解决、降噪抑制 */
  const statCards = [
    {
      key: 'firing',
      label: t('list.stat.firing'),
      value: stats?.firing ?? 0,
      trend: stats?.firingTrend,
      direction: stats?.firingDirection ?? 'up',
    },
    {
      key: 'todayNew',
      label: t('list.stat.todayNew'),
      value: stats?.todayNew ?? 0,
      trend: stats?.todayNewTrend,
      direction: stats?.todayNewDirection ?? 'down',
    },
    {
      key: 'todayResolved',
      label: t('list.stat.todayResolved'),
      value: stats?.todayResolved ?? 0,
      trend: stats?.todayResolvedTrend,
      direction: stats?.todayResolvedDirection ?? 'up',
    },
    {
      key: 'suppressed',
      label: t('list.stat.suppressed'),
      value: stats?.suppressed ?? 0,
      suffix: t('list.stat.noiseRate', { rate: stats?.noiseRate ?? '0%' }),
    },
  ];

  /** 状态切换 Tab 配置 */
  const tabItems = [
    { key: 'firing', label: t('list.tab.firing') },
    { key: 'acknowledged', label: t('list.tab.acknowledged') },
    { key: 'all', label: t('list.tab.all') },
    { key: 'suppressed', label: t('list.tab.suppressed') },
  ];

  /* 首次加载时显示页面级骨架屏 */
  if (loading && !initialLoaded) {
    return (
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
          <AntSkeleton.Input active style={{ width: 200, height: 28 }} />
          <div style={{ display: 'flex', gap: 8 }}>
            <AntSkeleton.Button active style={{ width: 40 }} />
            <AntSkeleton.Button active style={{ width: 100 }} />
          </div>
        </div>
        {/* 统计卡片骨架 */}
        <Row gutter={16} style={{ marginBottom: 16 }}>
          {[1, 2, 3, 4].map((i) => (
            <Col span={6} key={i}>
              <Card style={{ borderRadius: 8 }} bodyStyle={{ padding: '16px 20px' }}>
                <AntSkeleton.Input active style={{ width: 80, height: 14, marginBottom: 8 }} />
                <AntSkeleton.Input active style={{ width: 60, height: 28 }} />
              </Card>
            </Col>
          ))}
        </Row>
        {/* 表格骨架 */}
        <Card style={{ borderRadius: 8 }} bodyStyle={{ padding: '12px 16px' }}>
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} style={{ display: 'flex', gap: 16, marginBottom: 16 }}>
              <AntSkeleton.Input active style={{ width: 60, height: 16 }} />
              <AntSkeleton.Input active style={{ width: '100%', height: 16, flex: 1 }} />
              <AntSkeleton.Input active style={{ width: 100, height: 16 }} />
              <AntSkeleton.Input active style={{ width: 120, height: 16 }} />
            </div>
          ))}
        </Card>
      </div>
    );
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('list.title')}</Text>
        <Space>
          {/* 列配置按钮 */}
          {ColumnConfigButton}
          <Button icon={<ReloadOutlined />} onClick={() => fetchData(page)} />
          <Button type="primary" icon={<PlusOutlined />}>{t('list.createRule')}</Button>
        </Space>
      </div>

      <Row gutter={16} style={{ marginBottom: 16 }}>
        {statCards.map((card) => (
          <Col span={6} key={card.key}>
            <Card
              bordered
              style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
              bodyStyle={{ padding: '16px 20px' }}
            >
              <div style={{ color: '#86909C', fontSize: 14 }}>{card.label}</div>
              <div style={{ fontSize: 28, fontWeight: 600, marginTop: 4 }}>{card.value}</div>
              {'trend' in card && card.trend !== undefined && (
                <div style={{
                  fontSize: 12,
                  color: card.direction === 'up' ? '#F53F3F' : '#00B42A',
                  marginTop: 4,
                }}>
                  {card.direction === 'up' ? <ArrowUpOutlined /> : <ArrowDownOutlined />}
                  {' '}{card.trend} {t('list.stat.vsYesterday')}
                </div>
              )}
              {'suffix' in card && card.suffix && (
                <div style={{ fontSize: 12, color: '#86909C', marginTop: 4 }}>{card.suffix}</div>
              )}
            </Card>
          </Col>
        ))}
      </Row>

      <Tabs items={tabItems} activeKey={activeTab} onChange={(key) => { setActiveTab(key); setPage(1); }} />

      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Space wrap>
          <Select
            placeholder={t('list.filter.severity')}
            style={{ width: 120 }}
            allowClear
            value={severityFilter}
            onChange={setSeverityFilter}
            options={['P0', 'P1', 'P2', 'P3', 'P4'].map((s) => ({ value: s, label: s }))}
          />
          <Select
            placeholder={t('list.filter.business')}
            style={{ width: 140 }}
            allowClear
            value={businessFilter}
            onChange={setBusinessFilter}
          />
          <Select
            placeholder={t('list.filter.source')}
            style={{ width: 140 }}
            allowClear
            value={sourceFilter}
            onChange={setSourceFilter}
          />
          <Select
            placeholder={t('list.filter.layer')}
            style={{ width: 120 }}
            allowClear
            value={layerFilter}
            onChange={setLayerFilter}
            options={[0, 1, 2, 3, 4, 5].map((l) => ({ value: l, label: `Layer ${l}` }))}
          />
          <Input.Search
            placeholder={t('list.filter.keyword')}
            style={{ width: 200 }}
            allowClear
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onSearch={() => fetchData(1)}
          />
        </Space>
      </Card>

      {/* 使用虚拟滚动表格展示告警列表 */}
      <VirtualTable<AlertRecord>
        columns={columns}
        dataSource={data}
        loading={loading}
        height={500}
        rowHeight={48}
        rowKey="id"
        emptyText="暂无告警，系统运行正常"
      />

      <Modal
        title={t('list.action.acknowledge')}
        open={confirmModalOpen}
        onCancel={() => setConfirmModalOpen(false)}
        onOk={handleConfirmAcknowledge}
        confirmLoading={ackLoading}
        okText={t('list.confirmDialog.confirm')}
        cancelText={t('list.confirmDialog.cancel')}
      >
        <p>{t('list.confirmDialog.message')}</p>
        {selectedAlert && (
          <div>
            <Tag color={SEVERITY_COLORS[selectedAlert.severity]}>{selectedAlert.severity}</Tag>
            <span>{selectedAlert.content}</span>
          </div>
        )}
      </Modal>

      <style>{`
        .alert-row-p0 {
          border-left: 4px solid #F53F3F !important;
          background-color: #FFF5F5 !important;
        }
        .alert-row-p0:hover > td {
          background-color: #FFF0F0 !important;
        }
      `}</style>
    </div>
  );
};

export default AlertList;
