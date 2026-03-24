/**
 * 数据导出页面（页面 21）- 系统数据导出管理
 *
 * 功能模块（严格按设计文档）：
 * - 导出范围选择（Checkbox Group：告警/事件/资产/知识库/配置/审计日志）
 * - 时间范围选择（DateRangePicker）
 * - 格式选择（Radio：JSON/CSV）
 * - 加密选项（Switch + 密码输入）
 * - 导出按钮（异步任务）
 * - 历史导出任务列表（文件名/大小/创建时间/状态/下载）
 */
import React, { useState, useCallback } from 'react';
import {
  Card, Row, Col, Typography, Table, Tag, Button, Space, Checkbox, Radio,
  Switch, Input, DatePicker, Divider, Progress, message, Tooltip,
} from 'antd';
import {
  ExportOutlined, DownloadOutlined, FileZipOutlined, LockOutlined,
  CheckCircleOutlined, LoadingOutlined, CloseCircleOutlined,
  ClockCircleOutlined, DeleteOutlined, FileTextOutlined,
  DatabaseOutlined, SafetyCertificateOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;
const { RangePicker } = DatePicker;

// ==================== 类型定义 ====================

/** 导出范围选项 */
type ExportScope = 'alerts' | 'events' | 'assets' | 'knowledge' | 'config' | 'audit';

/** 导出格式 */
type ExportFormat = 'json' | 'csv';

/** 导出任务状态 */
type ExportStatus = 'pending' | 'processing' | 'completed' | 'failed';

/** 导出任务记录 */
interface ExportTask {
  key: string;            // 唯一标识
  fileName: string;       // 文件名
  fileSize: string;       // 文件大小
  createdAt: string;      // 创建时间
  status: ExportStatus;   // 任务状态
  progress: number;       // 处理进度（百分比）
  format: ExportFormat;   // 导出格式
  scopes: ExportScope[];  // 导出范围
  encrypted: boolean;     // 是否加密
  downloadUrl?: string;   // 下载链接（完成后可用）
}

// ==================== Mock 数据 ====================

/** 导出范围选项配置 */
const scopeOptions: { value: ExportScope; labelKey: string; icon: React.ReactNode }[] = [
  { value: 'alerts', labelKey: 'dataExport.scope.alerts', icon: <ExportOutlined /> },
  { value: 'events', labelKey: 'dataExport.scope.events', icon: <ClockCircleOutlined /> },
  { value: 'assets', labelKey: 'dataExport.scope.assets', icon: <DatabaseOutlined /> },
  { value: 'knowledge', labelKey: 'dataExport.scope.knowledge', icon: <FileTextOutlined /> },
  { value: 'config', labelKey: 'dataExport.scope.config', icon: <SafetyCertificateOutlined /> },
  { value: 'audit', labelKey: 'dataExport.scope.audit', icon: <LockOutlined /> },
];

/** Mock 历史导出任务列表 */
const mockExportTasks: ExportTask[] = [
  {
    key: '1',
    fileName: 'opsnexus-export-2026-03-24-alerts-events.json.gz',
    fileSize: '12.5 MB',
    createdAt: '2026-03-24 09:30:00',
    status: 'completed',
    progress: 100,
    format: 'json',
    scopes: ['alerts', 'events'],
    encrypted: false,
    downloadUrl: '#',
  },
  {
    key: '2',
    fileName: 'opsnexus-export-2026-03-23-full.csv.enc.gz',
    fileSize: '85.2 MB',
    createdAt: '2026-03-23 18:00:00',
    status: 'completed',
    progress: 100,
    format: 'csv',
    scopes: ['alerts', 'events', 'assets', 'knowledge', 'config', 'audit'],
    encrypted: true,
    downloadUrl: '#',
  },
  {
    key: '3',
    fileName: 'opsnexus-export-2026-03-24-assets.json.gz',
    fileSize: '--',
    createdAt: '2026-03-24 10:15:00',
    status: 'processing',
    progress: 67,
    format: 'json',
    scopes: ['assets'],
    encrypted: false,
  },
  {
    key: '4',
    fileName: 'opsnexus-export-2026-03-22-audit.csv.gz',
    fileSize: '3.1 MB',
    createdAt: '2026-03-22 14:00:00',
    status: 'completed',
    progress: 100,
    format: 'csv',
    scopes: ['audit'],
    encrypted: false,
    downloadUrl: '#',
  },
  {
    key: '5',
    fileName: 'opsnexus-export-2026-03-21-knowledge.json.gz',
    fileSize: '--',
    createdAt: '2026-03-21 11:30:00',
    status: 'failed',
    progress: 35,
    format: 'json',
    scopes: ['knowledge'],
    encrypted: false,
  },
];

// ==================== 辅助函数 ====================

/**
 * 根据导出状态返回标签配置
 */
const getStatusConfig = (status: ExportStatus) => {
  const configs = {
    pending: { color: 'default', icon: <ClockCircleOutlined />, textKey: 'dataExport.status.pending' },
    processing: { color: 'processing', icon: <LoadingOutlined />, textKey: 'dataExport.status.processing' },
    completed: { color: 'success', icon: <CheckCircleOutlined />, textKey: 'dataExport.status.completed' },
    failed: { color: 'error', icon: <CloseCircleOutlined />, textKey: 'dataExport.status.failed' },
  };
  return configs[status];
};

// ==================== 组件实现 ====================

/**
 * 数据导出组件
 * 包含导出配置表单和历史任务列表
 */
const DataExport: React.FC = () => {
  const { t } = useTranslation('settings');

  // 导出配置状态
  const [selectedScopes, setSelectedScopes] = useState<ExportScope[]>([]); // 导出范围
  const [exportFormat, setExportFormat] = useState<ExportFormat>('json');   // 导出格式
  const [encrypted, setEncrypted] = useState(false);                        // 是否加密
  const [encryptPassword, setEncryptPassword] = useState('');               // 加密密码
  const [dateRange, setDateRange] = useState<[any, any] | null>(null);     // 时间范围
  const [exporting, setExporting] = useState(false);                        // 导出中状态

  /**
   * 执行导出操作
   * 校验参数后创建异步导出任务
   */
  const handleExport = useCallback(() => {
    // 参数校验
    if (selectedScopes.length === 0) {
      message.warning(t('dataExport.validation.scopeRequired'));
      return;
    }
    if (!dateRange || !dateRange[0] || !dateRange[1]) {
      message.warning(t('dataExport.validation.dateRequired'));
      return;
    }
    if (encrypted && !encryptPassword) {
      message.warning(t('dataExport.validation.passwordRequired'));
      return;
    }

    setExporting(true);
    // 模拟创建导出任务
    setTimeout(() => {
      setExporting(false);
      message.success(t('dataExport.exportStarted'));
      // TODO: 对接导出任务创建 API
    }, 1500);
  }, [selectedScopes, dateRange, encrypted, encryptPassword, t]);

  /** 历史导出任务表格列定义 */
  const taskColumns = [
    {
      title: t('dataExport.task.column.fileName'),
      dataIndex: 'fileName',
      key: 'fileName',
      ellipsis: true,
      /** 渲染文件名，带文件图标 */
      render: (name: string, record: ExportTask) => (
        <Space>
          <FileZipOutlined />
          <Tooltip title={name}>
            <Text style={{ maxWidth: 280 }} ellipsis>{name}</Text>
          </Tooltip>
          {record.encrypted && (
            <Tag icon={<LockOutlined />} color="orange">{t('dataExport.encrypted')}</Tag>
          )}
        </Space>
      ),
    },
    {
      title: t('dataExport.task.column.fileSize'),
      dataIndex: 'fileSize',
      key: 'fileSize',
      width: 100,
    },
    {
      title: t('dataExport.task.column.format'),
      dataIndex: 'format',
      key: 'format',
      width: 80,
      /** 渲染导出格式标签 */
      render: (format: ExportFormat) => (
        <Tag>{format.toUpperCase()}</Tag>
      ),
    },
    {
      title: t('dataExport.task.column.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 170,
    },
    {
      title: t('dataExport.task.column.status'),
      dataIndex: 'status',
      key: 'status',
      width: 140,
      /** 渲染导出状态，处理中显示进度条 */
      render: (status: ExportStatus, record: ExportTask) => {
        const config = getStatusConfig(status);
        return (
          <Space direction="vertical" size={4} style={{ width: '100%' }}>
            <Tag icon={config.icon} color={config.color as string}>
              {t(config.textKey)}
            </Tag>
            {status === 'processing' && (
              <Progress percent={record.progress} size="small" />
            )}
          </Space>
        );
      },
    },
    {
      title: t('dataExport.task.column.actions'),
      key: 'actions',
      width: 120,
      /** 渲染操作按钮：下载（已完成可用）、删除 */
      render: (_: unknown, record: ExportTask) => (
        <Space>
          {record.status === 'completed' && record.downloadUrl && (
            <Button type="link" size="small" icon={<DownloadOutlined />} onClick={() => message.info(t('dataExport.downloadStart'))}>
              {t('dataExport.task.download')}
            </Button>
          )}
          <Button type="link" size="small" danger icon={<DeleteOutlined />} onClick={() => message.success(t('dataExport.taskDeleted'))}>
            {t('dataExport.task.delete')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('dataExport.title')}</Text>
      </div>

      {/* 导出配置区域 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} title={t('dataExport.configTitle')}>
        {/* 导出范围选择 */}
        <div style={{ marginBottom: 20 }}>
          <Title level={5}>{t('dataExport.scopeTitle')}</Title>
          <Checkbox.Group
            value={selectedScopes}
            onChange={(values) => setSelectedScopes(values as ExportScope[])}
          >
            <Row gutter={[16, 12]}>
              {scopeOptions.map((opt) => (
                <Col xs={12} sm={8} md={4} key={opt.value}>
                  <Checkbox value={opt.value}>
                    <Space size={4}>
                      {opt.icon}
                      <span>{t(opt.labelKey)}</span>
                    </Space>
                  </Checkbox>
                </Col>
              ))}
            </Row>
          </Checkbox.Group>
        </div>

        <Divider />

        {/* 时间范围与格式选择 */}
        <Row gutter={24}>
          <Col xs={24} md={12}>
            <Title level={5}>{t('dataExport.dateRange')}</Title>
            <RangePicker
              style={{ width: '100%' }}
              placeholder={[t('dataExport.startDate'), t('dataExport.endDate')]}
              onChange={(dates) => setDateRange(dates as [any, any])}
            />
          </Col>
          <Col xs={24} md={12}>
            <Title level={5}>{t('dataExport.formatTitle')}</Title>
            <Radio.Group value={exportFormat} onChange={(e) => setExportFormat(e.target.value)}>
              <Radio.Button value="json">JSON</Radio.Button>
              <Radio.Button value="csv">CSV</Radio.Button>
            </Radio.Group>
          </Col>
        </Row>

        <Divider />

        {/* 加密选项 */}
        <Row gutter={24} align="middle">
          <Col xs={24} md={12}>
            <Space size={16}>
              <div>
                <Title level={5} style={{ margin: 0 }}>{t('dataExport.encryptTitle')}</Title>
                <Text type="secondary" style={{ fontSize: 12 }}>{t('dataExport.encryptHint')}</Text>
              </div>
              <Switch
                checked={encrypted}
                onChange={setEncrypted}
                checkedChildren={<LockOutlined />}
              />
            </Space>
          </Col>
          {encrypted && (
            <Col xs={24} md={12}>
              <Input.Password
                placeholder={t('dataExport.passwordPlaceholder')}
                value={encryptPassword}
                onChange={(e) => setEncryptPassword(e.target.value)}
                style={{ maxWidth: 300 }}
              />
            </Col>
          )}
        </Row>

        <Divider />

        {/* 导出按钮 */}
        <div style={{ textAlign: 'right' }}>
          <Button
            type="primary"
            icon={<ExportOutlined />}
            size="large"
            onClick={handleExport}
            loading={exporting}
          >
            {t('dataExport.startExport')}
          </Button>
        </div>
      </Card>

      {/* 历史导出任务列表 */}
      <Card
        title={t('dataExport.historyTitle')}
        style={{ borderRadius: 8 }}
      >
        <Table<ExportTask>
          columns={taskColumns}
          dataSource={mockExportTasks}
          rowKey="key"
          size="middle"
          pagination={{ pageSize: 10, showTotal: (total) => t('dataExport.total', { count: total }) }}
        />
      </Card>
    </div>
  );
};

export default DataExport;
