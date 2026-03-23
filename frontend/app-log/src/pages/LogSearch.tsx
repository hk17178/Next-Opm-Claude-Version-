import React, { useState, useCallback, useEffect, useRef } from 'react';
import {
  Table, Button, Space, Tag, Typography, Card, Select, Input, DatePicker,
  Row, Col, Drawer, Descriptions, Modal, Radio, message,
} from 'antd';
import {
  ExportOutlined, SaveOutlined, SearchOutlined, CopyOutlined,
  EyeOutlined, AlertOutlined, ReloadOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { Skeleton as AntSkeleton, Empty as AntEmpty } from 'antd';
import { VirtualTable, type ColumnDef } from '@opsnexus/ui-kit';
import { searchLogs, exportLogs, type LogEntry, type LogSearchParams } from '../api/log';

const { Text } = Typography;
const { RangePicker } = DatePicker;

/** 日志级别对应的颜色映射 */
const LOG_LEVEL_COLORS: Record<string, string> = {
  ERROR: '#F53F3F',
  WARN: '#FF7D00',
  INFO: '#86909C',
  DEBUG: '#C9CDD4',
};

/** 时间快捷选项列表 */
const TIME_PRESETS = ['15min', '1h', '6h', '24h', '7d', '30d'];

/** 启用虚拟滚动的数据量阈值 */
const VIRTUAL_SCROLL_THRESHOLD = 1000;

/**
 * 日志检索页面组件
 * 功能：关键词搜索、时间范围过滤、级别/主机/服务过滤、分页展示、行展开详情、上下文抽屉、导出
 * 当结果超过 1000 条时自动启用虚拟滚动以保持流畅性能
 */
const LogSearch: React.FC = () => {
  const { t } = useTranslation('log');
  // ---- 数据状态 ----
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<LogEntry[]>([]);       // 日志列表数据
  const [total, setTotal] = useState(0);                   // 匹配总数
  const [took, setTook] = useState(0);                     // 查询耗时（秒）
  const [page, setPage] = useState(1);                     // 当前页码
  const [pageSize] = useState(100);                        // 每页条数

  // ---- 过滤条件状态 ----
  const [searchValue, setSearchValue] = useState('');                              // 搜索关键词
  const [timePreset, setTimePreset] = useState<string | undefined>();              // 时间快捷选项
  const [timeRange, setTimeRange] = useState<[string, string] | null>(null);       // 自定义时间范围
  const [levelFilter, setLevelFilter] = useState<string | undefined>();            // 日志级别过滤
  const [hostFilter, setHostFilter] = useState<string | undefined>();              // 主机名过滤
  const [serviceFilter, setServiceFilter] = useState<string | undefined>();        // 服务名过滤
  const [sourceTypeFilter, setSourceTypeFilter] = useState<string | undefined>();  // 来源类型过滤

  // ---- UI 交互状态 ----
  const [expandedRowKeys, setExpandedRowKeys] = useState<string[]>([]);  // 当前展开的日志行
  const [exportModalOpen, setExportModalOpen] = useState(false);         // 导出弹窗可见性
  const [exportFormat, setExportFormat] = useState('csv');               // 导出格式
  const [exportMaxRows, setExportMaxRows] = useState(10000);            // 导出最大行数
  const [exportLoading, setExportLoading] = useState(false);            // 导出操作加载状态
  const [detailDrawerOpen, setDetailDrawerOpen] = useState(false);       // 上下文抽屉可见性
  const [selectedLog, setSelectedLog] = useState<LogEntry | null>(null); // 当前选中的日志条目

  /** 用于取消正在进行的请求，防止竞态 */
  const abortRef = useRef<AbortController | null>(null);

  /**
   * 从后端获取日志数据
   * 自动取消上一个未完成的请求以防止竞态条件
   * @param currentPage - 要查询的页码，默认第 1 页
   */
  const fetchData = useCallback(async (currentPage = 1) => {
    // 取消上一个进行中的请求
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    const params: LogSearchParams = {
      keyword: searchValue || undefined,
      level: levelFilter,
      host: hostFilter,
      service: serviceFilter,
      sourceType: sourceTypeFilter,
      timePreset,
      startTime: timeRange?.[0],
      endTime: timeRange?.[1],
      page: currentPage,
      pageSize,
    };

    setLoading(true);
    try {
      const result = await searchLogs(params);
      if (!controller.signal.aborted) {
        setData(result.list);
        setTotal(result.total);
        setTook(result.took);
        setPage(currentPage);
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        // 后端 API 尚未就绪时，显示空状态
        setData([]);
        setTotal(0);
        setTook(0);
      }
    } finally {
      if (!controller.signal.aborted) {
        setLoading(false);
        setInitialLoaded(true);
      }
    }
  }, [searchValue, levelFilter, hostFilter, serviceFilter, sourceTypeFilter, timePreset, timeRange, pageSize]);

  /** 标记是否已完成首次数据加载（用于显示骨架屏） */
  const [initialLoaded, setInitialLoaded] = useState(false);

  // 组件卸载时取消未完成的请求
  useEffect(() => {
    return () => { abortRef.current?.abort(); };
  }, []);

  /** 触发搜索，重置到第 1 页 */
  const handleSearch = useCallback(() => {
    fetchData(1);
  }, [fetchData]);

  /**
   * 处理导出操作
   * 调用 GET /api/logs/export 接口，将当前搜索条件的日志导出为文件并触发浏览器下载
   */
  const handleExport = useCallback(async () => {
    setExportLoading(true);
    try {
      await exportLogs({
        format: exportFormat,
        keyword: searchValue || undefined,
        level: levelFilter,
        host: hostFilter,
        service: serviceFilter,
        sourceType: sourceTypeFilter,
        timePreset,
        startTime: timeRange?.[0],
        endTime: timeRange?.[1],
        maxRows: exportMaxRows,
      });
      message.success(t('search.exportDialog.success'));
      setExportModalOpen(false);
    } catch {
      message.error(t('search.exportDialog.failed'));
    } finally {
      setExportLoading(false);
    }
  }, [exportFormat, exportMaxRows, searchValue, levelFilter, hostFilter, serviceFilter, sourceTypeFilter, timePreset, timeRange, t]);

  /** 打开日志上下文抽屉，展示日志详细信息 */
  const handleViewContext = useCallback((record: LogEntry) => {
    setSelectedLog(record);
    setDetailDrawerOpen(true);
  }, []);

  /** 复制日志内容到剪贴板 */
  const handleCopy = useCallback((record: LogEntry) => {
    navigator.clipboard.writeText(record.fullMessage || record.message).then(() => {
      message.success(t('search.copy'));
    });
  }, [t]);

  /** 表格列定义 */
  const columns: ColumnDef<LogEntry>[] = [
    {
      title: t('search.column.timestamp'),
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
      sorter: true,
    },
    {
      title: t('search.column.level'),
      dataIndex: 'level',
      key: 'level',
      width: 80,
      render: (level: unknown) => (
        <Tag color={LOG_LEVEL_COLORS[level as string] || '#86909C'}>{level as string}</Tag>
      ),
    },
    {
      title: t('search.column.host'),
      dataIndex: 'host',
      key: 'host',
      width: 160,
    },
    {
      title: t('search.column.service'),
      dataIndex: 'service',
      key: 'service',
      width: 160,
    },
    {
      title: t('search.column.message'),
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
    },
  ];

  /** 行展开后显示的详情内容：完整消息、字段信息、操作按钮 */
  const expandedRowRender = (record: LogEntry) => (
    <div style={{ padding: '8px 0' }}>
      {record.fullMessage && (
        <div style={{ marginBottom: 12 }}>
          <Text strong>{t('search.detail.fullMessage')}</Text>
          <pre style={{
            background: '#F7F8FA',
            padding: 12,
            borderRadius: 6,
            fontSize: 12,
            whiteSpace: 'pre-wrap',
            marginTop: 4,
          }}>
            {record.fullMessage}
          </pre>
        </div>
      )}
      <div style={{ marginBottom: 12 }}>
        <Text strong>{t('search.detail.fields')}</Text>
        <div style={{ marginTop: 4, display: 'flex', gap: 24 }}>
          {record.traceId && <span>trace_id: {record.traceId}</span>}
          {record.sourceType && <span>source_type: {record.sourceType}</span>}
        </div>
        {record.tags && (
          <div style={{ marginTop: 4 }}>
            tags: {JSON.stringify(record.tags)}
          </div>
        )}
      </div>
      <Space>
        <Button
          size="small"
          icon={<EyeOutlined />}
          onClick={() => handleViewContext(record)}
        >
          {t('search.context', { count: 50 })}
        </Button>
        <Button size="small" icon={<CopyOutlined />} onClick={() => handleCopy(record)}>
          {t('search.copy')}
        </Button>
        <Button size="small" icon={<AlertOutlined />}>
          {t('search.relatedAlert')}
        </Button>
      </Space>
    </div>
  );

  /** 是否启用虚拟滚动（数据量超过阈值时启用） */
  const useVirtualScroll = data.length >= VIRTUAL_SCROLL_THRESHOLD;

  /* 首次加载时显示页面级骨架屏 */
  if (loading && !initialLoaded) {
    return (
      <div>
        {/* 标题区域骨架 */}
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
          <AntSkeleton.Input active style={{ width: 200, height: 28 }} />
          <div style={{ display: 'flex', gap: 8 }}>
            <AntSkeleton.Button active style={{ width: 80 }} />
            <AntSkeleton.Button active style={{ width: 80 }} />
          </div>
        </div>
        {/* 搜索栏骨架 */}
        <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 16 }}>
          <div style={{ display: 'flex', gap: 12 }}>
            <AntSkeleton.Input active style={{ width: 160 }} />
            <AntSkeleton.Input active style={{ width: '100%' }} />
          </div>
        </Card>
        {/* 表格行骨架 */}
        <Card style={{ borderRadius: 8 }} bodyStyle={{ padding: '12px 16px' }}>
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} style={{ display: 'flex', gap: 16, marginBottom: 16 }}>
              <AntSkeleton.Input active style={{ width: 140, height: 16 }} />
              <AntSkeleton.Input active style={{ width: 60, height: 16 }} />
              <AntSkeleton.Input active style={{ width: 120, height: 16 }} />
              <AntSkeleton.Input active style={{ width: '100%', height: 16, flex: 1 }} />
            </div>
          ))}
        </Card>
      </div>
    );
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('search.title')}</Text>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => fetchData(page)}>
            {t('search.refresh')}
          </Button>
          <Button icon={<ExportOutlined />} onClick={() => setExportModalOpen(true)}>
            {t('search.export')}
          </Button>
          <Button icon={<SaveOutlined />}>{t('search.saveQuery')}</Button>
        </Space>
      </div>

      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 16 }}>
        <Row gutter={[12, 12]}>
          <Col flex="160px">
            <Select
              style={{ width: '100%' }}
              placeholder={t('search.timePreset')}
              options={TIME_PRESETS.map((p) => ({ value: p, label: p }))}
              value={timePreset}
              onChange={(v) => { setTimePreset(v); setTimeRange(null); }}
              allowClear
            />
          </Col>
          <Col flex="auto">
            <Input.Search
              prefix={<SearchOutlined />}
              placeholder={t('search.placeholder')}
              value={searchValue}
              onChange={(e) => setSearchValue(e.target.value)}
              onSearch={handleSearch}
              enterButton={t('search.searchBtn')}
              allowClear
              style={{ borderRadius: 6 }}
            />
          </Col>
        </Row>
        <Row gutter={[12, 12]} style={{ marginTop: 12 }}>
          <Col>
            <RangePicker
              showTime
              style={{ borderRadius: 6 }}
              onChange={(dates) => {
                if (dates && dates[0] && dates[1]) {
                  setTimeRange([dates[0].toISOString(), dates[1].toISOString()]);
                  setTimePreset(undefined);
                } else {
                  setTimeRange(null);
                }
              }}
            />
          </Col>
          <Col>
            <Select
              placeholder={t('search.filter.sourceType')}
              style={{ width: 140 }}
              allowClear
              value={sourceTypeFilter}
              onChange={setSourceTypeFilter}
            />
          </Col>
          <Col>
            <Select
              placeholder={t('search.filter.host')}
              style={{ width: 140 }}
              allowClear
              value={hostFilter}
              onChange={setHostFilter}
            />
          </Col>
          <Col>
            <Select
              placeholder={t('search.filter.service')}
              style={{ width: 140 }}
              allowClear
              value={serviceFilter}
              onChange={setServiceFilter}
            />
          </Col>
          <Col>
            <Select
              placeholder={t('search.filter.level')}
              style={{ width: 140 }}
              allowClear
              value={levelFilter}
              onChange={setLevelFilter}
              options={['ERROR', 'WARN', 'INFO', 'DEBUG'].map((l) => ({ value: l, label: l }))}
            />
          </Col>
        </Row>
      </Card>

      <div style={{ marginBottom: 8, color: '#86909C', fontSize: 13 }}>
        {t('search.resultCount', { count: total, time: took.toFixed(1) })}
        {/* 虚拟滚动启用提示 */}
        {useVirtualScroll && (
          <Tag color="blue" style={{ marginLeft: 8 }}>虚拟滚动已启用</Tag>
        )}
      </div>

      {/* 根据数据量决定使用虚拟滚动表格还是普通表格 */}
      {useVirtualScroll ? (
        <VirtualTable<LogEntry>
          columns={columns}
          dataSource={data}
          loading={loading}
          height={Math.max(400, window.innerHeight - 420)}
          rowHeight={48}
          rowKey="id"
          emptyText="未找到匹配的日志，请调整搜索条件"
          onRowClick={(record) => handleViewContext(record)}
        />
      ) : (
        <Table
          columns={columns}
          dataSource={data}
          loading={loading}
          pagination={{
            current: page,
            pageSize,
            total,
            showTotal: (t) => `${t}`,
            onChange: (p) => fetchData(p),
          }}
          scroll={{ y: 'calc(100vh - 420px)' }}
          locale={{
            emptyText: (
              <AntEmpty
                image={AntEmpty.PRESENTED_IMAGE_SIMPLE}
                description="未找到匹配的日志，请调整搜索条件"
              />
            ),
          }}
          expandable={{
            expandedRowKeys,
            onExpand: (expanded, record) => {
              setExpandedRowKeys(expanded ? [record.id] : []);
            },
            expandedRowRender,
          }}
          rowKey="id"
          size="middle"
          style={{ borderRadius: 8 }}
        />
      )}

      {/* 导出配置弹窗：选择格式和最大行数后触发浏览器下载 */}
      <Modal
        title={t('search.exportDialog.title')}
        open={exportModalOpen}
        onCancel={() => setExportModalOpen(false)}
        onOk={handleExport}
        confirmLoading={exportLoading}
      >
        <div style={{ marginBottom: 16 }}>
          <Text>{t('search.exportDialog.format')}</Text>
          <Radio.Group
            value={exportFormat}
            onChange={(e) => setExportFormat(e.target.value)}
            style={{ marginLeft: 12 }}
          >
            <Radio value="csv">CSV</Radio>
            <Radio value="json">JSON</Radio>
          </Radio.Group>
        </div>
        <div>
          <Text>{t('search.exportDialog.maxRows')}</Text>
          <Input
            type="number"
            value={exportMaxRows}
            onChange={(e) => setExportMaxRows(Number(e.target.value) || 10000)}
            style={{ width: 150, marginLeft: 12, borderRadius: 6 }}
          />
        </div>
      </Modal>

      <Drawer
        title={t('search.context', { count: 50 })}
        open={detailDrawerOpen}
        onClose={() => setDetailDrawerOpen(false)}
        width={720}
      >
        {selectedLog && (
          <Descriptions column={2} bordered size="small">
            <Descriptions.Item label={t('search.column.timestamp')}>{selectedLog.timestamp}</Descriptions.Item>
            <Descriptions.Item label={t('search.column.level')}>
              <Tag color={LOG_LEVEL_COLORS[selectedLog.level]}>{selectedLog.level}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t('search.column.host')}>{selectedLog.host}</Descriptions.Item>
            <Descriptions.Item label={t('search.column.service')}>{selectedLog.service}</Descriptions.Item>
            <Descriptions.Item label={t('search.column.message')} span={2}>{selectedLog.message}</Descriptions.Item>
            {selectedLog.fullMessage && (
              <Descriptions.Item label={t('search.detail.fullMessage')} span={2}>
                <pre style={{ margin: 0, whiteSpace: 'pre-wrap', fontSize: 12 }}>
                  {selectedLog.fullMessage}
                </pre>
              </Descriptions.Item>
            )}
            {selectedLog.traceId && (
              <Descriptions.Item label="Trace ID">{selectedLog.traceId}</Descriptions.Item>
            )}
            {selectedLog.sourceType && (
              <Descriptions.Item label={t('search.filter.sourceType')}>{selectedLog.sourceType}</Descriptions.Item>
            )}
          </Descriptions>
        )}
      </Drawer>
    </div>
  );
};

export default LogSearch;
