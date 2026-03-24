/**
 * 版本管理页面（页面 21）- 系统版本信息展示、检查更新、上传升级包、版本历史
 *
 * 功能模块（严格按设计文档）：
 * - 当前版本信息卡片（版本号/构建时间/Git SHA/运行时长）
 * - 检查更新按钮 + 上传升级包按钮
 * - 版本历史表格（版本号/发布日期/变更摘要/操作：查看 Changelog/回滚）
 * - Changelog 详情 Drawer
 */
import React, { useState, useCallback } from 'react';
import {
  Card, Row, Col, Typography, Table, Tag, Button, Space, Upload, Drawer,
  Timeline, Popconfirm, Badge, Statistic, Divider, message, Spin,
} from 'antd';
import {
  CloudUploadOutlined, CloudDownloadOutlined, RocketOutlined, HistoryOutlined,
  CheckCircleOutlined, RollbackOutlined, FileTextOutlined, CodeOutlined,
  ClockCircleOutlined, TagOutlined, UploadOutlined, InfoCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text, Title, Paragraph } = Typography;

// ==================== 类型定义 ====================

/** 版本历史记录 */
interface VersionRecord {
  key: string;           // 唯一标识
  version: string;       // 版本号
  releaseDate: string;   // 发布日期
  summary: string;       // 变更摘要
  status: 'current' | 'available' | 'archived'; // 版本状态
  changelog: ChangelogItem[]; // 变更日志详情
  canRollback: boolean;  // 是否可回滚
}

/** 变更日志条目 */
interface ChangelogItem {
  type: 'feature' | 'fix' | 'improvement' | 'breaking'; // 变更类型
  description: string;   // 变更描述
}

// ==================== Mock 数据 ====================

/** 当前版本信息 */
const currentVersion = {
  version: 'v2.4.1',
  buildTime: '2026-03-20 14:30:00',
  gitSha: 'a3f8c2e',
  uptime: '4 天 8 小时 32 分钟',
  environment: 'Production',
  nodeVersion: 'v20.11.0',
};

/** Mock 版本历史列表 */
const mockVersionHistory: VersionRecord[] = [
  {
    key: '1',
    version: 'v2.4.1',
    releaseDate: '2026-03-20',
    summary: '修复告警降噪误判、优化 ES 查询性能',
    status: 'current',
    canRollback: false,
    changelog: [
      { type: 'fix', description: '修复告警降噪算法在高并发场景下的误判问题' },
      { type: 'improvement', description: '优化 Elasticsearch 复合查询性能，响应时间降低 40%' },
      { type: 'fix', description: '修复 CMDB 资产拓扑图在 Firefox 下的渲染异常' },
    ],
  },
  {
    key: '2',
    version: 'v2.4.0',
    releaseDate: '2026-03-10',
    summary: '新增自动化编排引擎、AI Copilot 多轮对话',
    status: 'archived',
    canRollback: true,
    changelog: [
      { type: 'feature', description: '新增自动化编排引擎，支持 YAML 定义工作流' },
      { type: 'feature', description: 'AI Copilot 支持多轮对话，带上下文记忆' },
      { type: 'improvement', description: '告警统计面板新增趋势对比图表' },
      { type: 'fix', description: '修复 SLA 报告导出时的时区偏移问题' },
    ],
  },
  {
    key: '3',
    version: 'v2.3.2',
    releaseDate: '2026-02-28',
    summary: '安全补丁：修复 LDAP 注入漏洞',
    status: 'archived',
    canRollback: true,
    changelog: [
      { type: 'fix', description: '修复 LDAP 查询参数注入漏洞（CVE-2026-XXXX）' },
      { type: 'fix', description: '修复 Session 过期后未正确清理 Token 的问题' },
      { type: 'improvement', description: '增强密码策略，支持自定义复杂度规则' },
    ],
  },
  {
    key: '4',
    version: 'v2.3.0',
    releaseDate: '2026-02-15',
    summary: '新增知识库模块、事件指挥驾驶舱',
    status: 'archived',
    canRollback: true,
    changelog: [
      { type: 'feature', description: '新增知识库模块，支持 Markdown 文档管理与智能搜索' },
      { type: 'feature', description: '新增事件指挥驾驶舱，集成翻牌卡片和健康矩阵' },
      { type: 'breaking', description: 'API 接口版本升级至 v2，v1 接口将在 v3.0 移除' },
      { type: 'improvement', description: '重构粒子背景渲染引擎，性能提升 60%' },
    ],
  },
  {
    key: '5',
    version: 'v2.2.0',
    releaseDate: '2026-01-20',
    summary: '新增 CMDB 资产管理、SLA 报告模块',
    status: 'archived',
    canRollback: false,
    changelog: [
      { type: 'feature', description: '新增 CMDB 资产管理，支持服务器/数据库/中间件多类型' },
      { type: 'feature', description: '新增 SLA 报告模块，自动计算可用性和 MTTR' },
      { type: 'improvement', description: '优化全局搜索 Cmd+K，支持模糊匹配和热键导航' },
    ],
  },
];

// ==================== 组件实现 ====================

/**
 * 版本管理组件
 * 展示当前版本信息、版本历史、Changelog 详情
 */
const VersionManagement: React.FC = () => {
  const { t } = useTranslation('settings');
  const [checking, setChecking] = useState(false);                // 检查更新中状态
  const [drawerOpen, setDrawerOpen] = useState(false);             // Changelog Drawer 开关
  const [selectedVersion, setSelectedVersion] = useState<VersionRecord | null>(null); // 选中查看的版本

  /**
   * 检查系统更新
   * 模拟异步检查过程
   */
  const handleCheckUpdate = useCallback(() => {
    setChecking(true);
    setTimeout(() => {
      setChecking(false);
      message.info(t('version.checkResult'));
      // TODO: 对接版本更新检查 API
    }, 2000);
  }, [t]);

  /**
   * 查看版本 Changelog
   * 打开 Drawer 展示变更详情
   */
  const handleViewChangelog = useCallback((record: VersionRecord) => {
    setSelectedVersion(record);
    setDrawerOpen(true);
  }, []);

  /**
   * 回滚到指定版本
   * 二次确认后执行回滚操作
   */
  const handleRollback = useCallback((record: VersionRecord) => {
    message.loading(t('version.rollbackLoading'));
    // TODO: 对接版本回滚 API
    setTimeout(() => {
      message.success(t('version.rollbackSuccess', { version: record.version }));
    }, 1500);
  }, [t]);

  /**
   * 返回变更类型对应的 Tag 配色
   */
  const getChangeTypeTag = (type: ChangelogItem['type']) => {
    const configs = {
      feature: { color: '#2E75B6', label: t('version.changeType.feature') },
      fix: { color: '#F53F3F', label: t('version.changeType.fix') },
      improvement: { color: '#00B42A', label: t('version.changeType.improvement') },
      breaking: { color: '#FF7D00', label: t('version.changeType.breaking') },
    };
    return configs[type];
  };

  /** 版本历史表格列定义 */
  const versionColumns = [
    {
      title: t('version.column.version'),
      dataIndex: 'version',
      key: 'version',
      width: 120,
      /** 渲染版本号，当前版本带标记 */
      render: (version: string, record: VersionRecord) => (
        <Space>
          <TagOutlined />
          <Text strong>{version}</Text>
          {record.status === 'current' && (
            <Tag color="green">{t('version.status.current')}</Tag>
          )}
        </Space>
      ),
    },
    {
      title: t('version.column.releaseDate'),
      dataIndex: 'releaseDate',
      key: 'releaseDate',
      width: 130,
    },
    {
      title: t('version.column.summary'),
      dataIndex: 'summary',
      key: 'summary',
      ellipsis: true,
    },
    {
      title: t('version.column.actions'),
      key: 'actions',
      width: 220,
      /** 渲染操作按钮：查看 Changelog、回滚 */
      render: (_: unknown, record: VersionRecord) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<FileTextOutlined />}
            onClick={() => handleViewChangelog(record)}
          >
            {t('version.action.changelog')}
          </Button>
          {record.canRollback && (
            <Popconfirm
              title={t('version.rollbackConfirm', { version: record.version })}
              onConfirm={() => handleRollback(record)}
              okText={t('version.confirm')}
              cancelText={t('version.cancel')}
            >
              <Button type="link" size="small" danger icon={<RollbackOutlined />}>
                {t('version.action.rollback')}
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与操作按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('version.title')}</Text>
        <Space>
          {/* 检查更新按钮 */}
          <Button
            icon={<CloudDownloadOutlined />}
            onClick={handleCheckUpdate}
            loading={checking}
          >
            {t('version.checkUpdate')}
          </Button>
          {/* 上传升级包按钮 */}
          <Upload
            accept=".tar.gz,.zip"
            showUploadList={false}
            beforeUpload={(file) => {
              message.info(t('version.uploadStart', { name: file.name }));
              // TODO: 对接升级包上传 API
              return false;
            }}
          >
            <Button type="primary" icon={<CloudUploadOutlined />}>
              {t('version.uploadPackage')}
            </Button>
          </Upload>
        </Space>
      </div>

      {/* 当前版本信息卡片 */}
      <Card
        style={{ marginBottom: 16, borderRadius: 8 }}
        title={
          <Space>
            <RocketOutlined />
            {t('version.currentInfo')}
          </Space>
        }
      >
        <Row gutter={[24, 16]}>
          <Col xs={12} md={6}>
            <Statistic
              title={t('version.info.version')}
              value={currentVersion.version}
              prefix={<TagOutlined />}
              valueStyle={{ color: '#2E75B6', fontSize: 18 }}
            />
          </Col>
          <Col xs={12} md={6}>
            <Statistic
              title={t('version.info.buildTime')}
              value={currentVersion.buildTime}
              prefix={<ClockCircleOutlined />}
              valueStyle={{ fontSize: 14 }}
            />
          </Col>
          <Col xs={12} md={6}>
            <Statistic
              title={t('version.info.gitSha')}
              value={currentVersion.gitSha}
              prefix={<CodeOutlined />}
              valueStyle={{ fontSize: 14, fontFamily: 'JetBrains Mono, monospace' }}
            />
          </Col>
          <Col xs={12} md={6}>
            <Statistic
              title={t('version.info.uptime')}
              value={currentVersion.uptime}
              prefix={<ClockCircleOutlined />}
              valueStyle={{ fontSize: 14 }}
            />
          </Col>
        </Row>
      </Card>

      {/* 版本历史表格 */}
      <Card
        title={
          <Space>
            <HistoryOutlined />
            {t('version.historyTitle')}
          </Space>
        }
        style={{ borderRadius: 8 }}
      >
        <Table<VersionRecord>
          columns={versionColumns}
          dataSource={mockVersionHistory}
          rowKey="key"
          size="middle"
          pagination={{ pageSize: 10, showTotal: (total) => t('version.total', { count: total }) }}
        />
      </Card>

      {/* Changelog 详情 Drawer */}
      <Drawer
        title={
          <Space>
            <FileTextOutlined />
            {selectedVersion ? `${selectedVersion.version} ${t('version.changelogTitle')}` : ''}
          </Space>
        }
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={520}
      >
        {selectedVersion && (
          <>
            {/* 版本基本信息 */}
            <div style={{ marginBottom: 24 }}>
              <Space direction="vertical" style={{ width: '100%' }}>
                <div>
                  <Text type="secondary">{t('version.info.version')}:</Text>{' '}
                  <Text strong>{selectedVersion.version}</Text>
                </div>
                <div>
                  <Text type="secondary">{t('version.column.releaseDate')}:</Text>{' '}
                  <Text>{selectedVersion.releaseDate}</Text>
                </div>
                <div>
                  <Text type="secondary">{t('version.column.summary')}:</Text>{' '}
                  <Text>{selectedVersion.summary}</Text>
                </div>
              </Space>
            </div>

            <Divider />

            {/* 变更详情时间线 */}
            <Title level={5}>{t('version.changelogDetail')}</Title>
            <Timeline
              items={selectedVersion.changelog.map((item, index) => {
                const config = getChangeTypeTag(item.type);
                return {
                  key: index,
                  color: config.color,
                  children: (
                    <div>
                      <Tag color={config.color} style={{ marginBottom: 4 }}>{config.label}</Tag>
                      <div>{item.description}</div>
                    </div>
                  ),
                };
              })}
            />
          </>
        )}
      </Drawer>
    </div>
  );
};

export default VersionManagement;
