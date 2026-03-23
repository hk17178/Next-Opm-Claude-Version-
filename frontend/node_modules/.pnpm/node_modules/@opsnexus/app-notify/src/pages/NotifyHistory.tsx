/**
 * 通知历史记录页面 - 展示所有通知发送记录，支持时间范围/渠道/状态过滤、重试失败通知
 * 包含：过滤条件栏、通知记录表格、通知详情弹窗、导出功能
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Table, Card, Space, Typography, Select, Tag, Input, DatePicker, Button, Tooltip, Modal,
  message,
} from 'antd';
import {
  CheckCircleOutlined, CloseCircleOutlined, ClockCircleOutlined,
  ReloadOutlined, EyeOutlined, ExportOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { fetchNotifyHistory, retryNotification } from '../api/notify';

const { Text, Paragraph } = Typography;
const { RangePicker } = DatePicker;

/** 通知发送状态枚举 */
type NotifyStatus = 'success' | 'fail' | 'pending';

/** 渠道类型枚举 */
type ChannelType = 'wecom_webhook' | 'wecom_app' | 'sms' | 'email' | 'voice_tts' | 'webhook';

/** 通知记录数据结构 */
interface NotifyRecord {
  key: string;                    // 记录唯一标识
  time: string;                   // 发送时间
  receiver: string;               // 接收人
  channel: string;                // 渠道名称
  channelType: ChannelType;       // 渠道类型
  contentSummary: string;         // 内容摘要
  fullContent?: string;           // 完整内容（可选）
  status: NotifyStatus;           // 发送状态
  retryCount: number;             // 重试次数
  errorMessage?: string;          // 错误信息（可选，失败时）
  relatedAlertId?: string;        // 关联告警 ID（可选）
  relatedIncidentId?: string;     // 关联事件 ID（可选）
}

/** 通知状态对应的颜色和图标配置 */
const STATUS_CONFIG: Record<NotifyStatus, { color: string; icon: React.ReactNode }> = {
  success: { color: '#00B42A', icon: <CheckCircleOutlined /> },   // 成功 - 绿色
  fail: { color: '#F53F3F', icon: <CloseCircleOutlined /> },      // 失败 - 红色
  pending: { color: '#FF7D00', icon: <ClockCircleOutlined /> },   // 等待中 - 橙色
};

/** 渠道类型对应的颜色映射 */
const CHANNEL_TYPE_COLORS: Record<ChannelType, string> = {
  wecom_webhook: '#2E75B6',
  wecom_app: '#4C9AE6',
  sms: '#00B42A',
  email: '#FF7D00',
  voice_tts: '#F53F3F',
  webhook: '#722ED1',
};

/** 所有渠道类型列表（用于下拉过滤） */
const ALL_CHANNEL_TYPES: ChannelType[] = [
  'wecom_webhook', 'wecom_app', 'sms', 'email', 'voice_tts', 'webhook',
];

/**
 * 通知历史组件
 * - 顶部：页面标题 + 导出按钮
 * - 过滤栏：时间范围选择、渠道类型、发送状态、关键词搜索
 * - 表格：发送时间、接收人、渠道、类型、内容摘要、状态、重试次数、操作
 * - 详情弹窗：展示通知完整内容、错误信息、关联告警等
 */
const NotifyHistory: React.FC = () => {
  const { t } = useTranslation('notify');
  const [loading, setLoading] = useState(false);             // 表格加载状态
  const [data, setData] = useState<NotifyRecord[]>([]);      // 通知记录数据
  const [total, setTotal] = useState(0);                     // 数据总条数
  const [detailModalOpen, setDetailModalOpen] = useState(false); // 详情弹窗是否打开
  const [selectedRecord, setSelectedRecord] = useState<NotifyRecord | null>(null); // 当前查看的记录
  const [retryingKey, setRetryingKey] = useState<string | null>(null); // 正在重试的记录 key
  const [pagination, setPagination] = useState({ current: 1, pageSize: 20 }); // 分页参数

  /**
   * 加载通知历史记录数据
   * @param page 页码（可选）
   * @param pageSize 每页条数（可选）
   */
  const loadData = useCallback(async (page?: number, pageSize?: number) => {
    setLoading(true);
    try {
      // request<T> 已自动解包 ApiResponse.data，直接获取 NotifyHistoryResult
      const result = await fetchNotifyHistory({
        page: page || pagination.current,
        pageSize: pageSize || pagination.pageSize,
      });
      setData(result.list || []);
      setTotal(result.total || 0);
    } catch {
      // API 尚未就绪
    } finally {
      setLoading(false);
    }
  }, [pagination]);

  /** 组件挂载及依赖变化时加载数据 */
  useEffect(() => {
    loadData();
  }, [loadData]);

  /**
   * 查看通知详情 - 打开详情弹窗
   * @param record 通知记录
   */
  const handleViewDetail = (record: NotifyRecord) => {
    setSelectedRecord(record);
    setDetailModalOpen(true);
  };

  /**
   * 重试失败的通知
   * 调用重试 API → 成功后刷新列表
   * @param record 通知记录
   */
  const handleRetry = useCallback(async (record: NotifyRecord) => {
    setRetryingKey(record.key);
    try {
      await retryNotification(record.key);
      message.success(t('history.retrySuccess'));
      loadData();
    } catch {
      message.error(t('history.retryFail'));
    } finally {
      setRetryingKey(null);
    }
  }, [loadData, t]);

  /** 表格列定义 */
  const columns = [
    {
      title: t('history.column.time'),
      dataIndex: 'time',
      key: 'time',
      width: 180,
      sorter: true, // 支持按时间排序
    },
    {
      title: t('history.column.receiver'),
      dataIndex: 'receiver',
      key: 'receiver',
      width: 140,
      ellipsis: true,
    },
    {
      title: t('history.column.channel'),
      dataIndex: 'channel',
      key: 'channel',
      width: 140,
    },
    {
      title: t('history.column.type'),
      dataIndex: 'channelType',
      key: 'channelType',
      width: 130,
      /** 渲染渠道类型标签 */
      render: (type: ChannelType) => (
        <Tag color={CHANNEL_TYPE_COLORS[type]}>{t(`channel.type.${type}`)}</Tag>
      ),
    },
    {
      title: t('history.column.content'),
      dataIndex: 'contentSummary',
      key: 'contentSummary',
      ellipsis: true,
    },
    {
      title: t('history.column.status'),
      dataIndex: 'status',
      key: 'status',
      width: 110,
      /** 渲染发送状态标签（带图标） */
      render: (status: NotifyStatus) => {
        const cfg = STATUS_CONFIG[status];
        return <Tag color={cfg.color} icon={cfg.icon}>{t(`history.status.${status}`)}</Tag>;
      },
    },
    {
      title: t('history.column.retryCount'),
      dataIndex: 'retryCount',
      key: 'retryCount',
      width: 80,
      /** 重试次数 > 0 时使用警告色显示 */
      render: (count: number) => (
        <Text type={count > 0 ? 'warning' : 'secondary'}>{count}</Text>
      ),
    },
    {
      title: t('history.column.actions'),
      key: 'actions',
      width: 120,
      /** 渲染操作按钮：查看详情 + 重试（仅失败记录） */
      render: (_: unknown, record: NotifyRecord) => (
        <Space>
          <Tooltip title={t('history.action.detail')}>
            <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => handleViewDetail(record)} />
          </Tooltip>
          {/* 仅对失败的通知显示重试按钮 */}
          {record.status === 'fail' && (
            <Tooltip title={t('history.action.retry')}>
              <Button
                type="link" size="small" icon={<ReloadOutlined />}
                loading={retryingKey === record.key}
                onClick={() => handleRetry(record)}
              />
            </Tooltip>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与导出按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('history.title')}</Text>
        <Button icon={<ExportOutlined />}>{t('history.export')}</Button>
      </div>

      {/* 过滤条件栏 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Space wrap>
          {/* 时间范围选择器 */}
          <RangePicker showTime style={{ borderRadius: 6 }} />
          {/* 渠道类型过滤 */}
          <Select placeholder={t('history.filter.channelType')} style={{ width: 160 }} allowClear
            options={ALL_CHANNEL_TYPES.map((v) => ({ value: v, label: t(`channel.type.${v}`) }))} />
          {/* 发送状态过滤 */}
          <Select placeholder={t('history.filter.status')} style={{ width: 120 }} allowClear
            options={(['success', 'fail', 'pending'] as NotifyStatus[]).map((v) => ({
              value: v, label: t(`history.status.${v}`),
            }))} />
          {/* 关键词搜索 */}
          <Input.Search placeholder={t('history.filter.search')} style={{ width: 220 }} allowClear />
        </Space>
      </Card>

      {/* 通知记录表格 */}
      <Table<NotifyRecord>
        columns={columns}
        dataSource={data}
        loading={loading}
        locale={{ emptyText: t('history.noData') }}
        rowKey="key"
        size="middle"
        pagination={{
          current: pagination.current,
          pageSize: pagination.pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t2) => t('history.total', { count: t2 }),
          onChange: (page, pageSize) => {
            setPagination({ current: page, pageSize });
            loadData(page, pageSize);
          },
        }}
      />

      {/* 通知详情弹窗 */}
      <Modal
        title={t('history.detailTitle')}
        open={detailModalOpen}
        onCancel={() => { setDetailModalOpen(false); setSelectedRecord(null); }}
        footer={null}
        width={600}
      >
        {selectedRecord && (
          <div>
            {/* 发送时间 */}
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary">{t('history.column.time')}: </Text>
              <Text>{selectedRecord.time}</Text>
            </div>
            {/* 接收人 */}
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary">{t('history.column.receiver')}: </Text>
              <Text>{selectedRecord.receiver}</Text>
            </div>
            {/* 渠道类型和名称 */}
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary">{t('history.column.channel')}: </Text>
              <Tag color={CHANNEL_TYPE_COLORS[selectedRecord.channelType]}>
                {t(`channel.type.${selectedRecord.channelType}`)}
              </Tag>
              <Text>{selectedRecord.channel}</Text>
            </div>
            {/* 发送状态 */}
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary">{t('history.column.status')}: </Text>
              <Tag color={STATUS_CONFIG[selectedRecord.status].color} icon={STATUS_CONFIG[selectedRecord.status].icon}>
                {t(`history.status.${selectedRecord.status}`)}
              </Tag>
            </div>
            {/* 通知内容（优先显示完整内容，无则显示摘要） */}
            <div style={{ marginBottom: 12 }}>
              <Text type="secondary">{t('history.detail.content')}: </Text>
              <Paragraph style={{ marginTop: 4, padding: 12, background: '#F7F8FA', borderRadius: 6 }}>
                {selectedRecord.fullContent || selectedRecord.contentSummary}
              </Paragraph>
            </div>
            {/* 错误信息（仅失败时显示） */}
            {selectedRecord.errorMessage && (
              <div style={{ marginBottom: 12 }}>
                <Text type="secondary">{t('history.detail.errorMessage')}: </Text>
                <Paragraph type="danger" style={{ marginTop: 4 }}>
                  {selectedRecord.errorMessage}
                </Paragraph>
              </div>
            )}
            {/* 关联告警 ID（可选） */}
            {selectedRecord.relatedAlertId && (
              <div style={{ marginBottom: 12 }}>
                <Text type="secondary">{t('history.detail.relatedAlert')}: </Text>
                <Button type="link" size="small">{selectedRecord.relatedAlertId}</Button>
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
};

export default NotifyHistory;
