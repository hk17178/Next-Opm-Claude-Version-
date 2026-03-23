/**
 * 工作流列表页面 - 展示所有编排工作流，支持管理操作
 *
 * 功能：
 * - 顶部统计卡片（总数/活跃/本周执行次数/成功率）
 * - 工作流表格（名称/触发类型/步骤数/启用状态/最近执行）
 * - 操作按钮：新建/编辑/删除/触发执行/查看执行历史
 * - 从模板创建：Modal 展示预置模板卡片
 * - 启用/禁用切换 Switch
 */
import React, { useState, useCallback, useEffect } from 'react';
import {
  Table, Card, Row, Col, Typography, Button, Space, Modal, Switch, Tag, Input,
  message, Popconfirm, Tooltip,
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined, PlayCircleOutlined,
  HistoryOutlined, ThunderboltOutlined, ReloadOutlined,
  ClockCircleOutlined, AlertOutlined, ScheduleOutlined,
  SyncOutlined, ClearOutlined, FileTextOutlined, RollbackOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import type { ColumnsType } from 'antd/es/table';
import {
  listWorkflows, deleteWorkflow, updateWorkflow, triggerExecution, createFromTemplate,
  type Workflow, type WorkflowTemplate, type ExecutionStatus,
} from '../api/orchestration';

const { Text } = Typography;

/** 触发类型标签颜色映射 */
const TRIGGER_TYPE_MAP: Record<string, { label: string; color: string }> = {
  manual: { label: '手动触发', color: 'blue' },
  alert: { label: '告警触发', color: 'orange' },
  cron: { label: '定时触发', color: 'green' },
};

/** 执行状态颜色映射 */
const EXECUTION_STATUS_MAP: Record<string, { label: string; color: string }> = {
  pending: { label: '等待中', color: 'default' },
  running: { label: '执行中', color: 'processing' },
  paused: { label: '已暂停', color: 'warning' },
  completed: { label: '已完成', color: 'success' },
  failed: { label: '失败', color: 'error' },
  cancelled: { label: '已取消', color: 'default' },
};

/** 预置模板数据（前端静态定义，也可从后端获取） */
const PRESET_TEMPLATES: WorkflowTemplate[] = [
  {
    id: 'tpl-restart',
    name: '服务重启',
    description: '自动重启指定服务，包含健康检查和通知',
    icon: 'SyncOutlined',
    steps: [
      { name: '执行重启脚本', type: 'script', script: 'systemctl restart ${service_name}', timeout: 60 },
      { name: '等待服务启动', type: 'wait', waitMinutes: 1 },
      { name: '健康检查', type: 'script', script: 'curl -f http://localhost:${port}/health', timeout: 30 },
      { name: '通知相关人员', type: 'notify', notifyTitle: '服务重启完成', notifyContent: '${service_name} 已成功重启', notifyChannels: ['dingtalk'] },
    ],
    variables: [
      { name: 'service_name', type: 'string', description: '服务名称' },
      { name: 'port', type: 'string', defaultValue: '8080', description: '服务端口' },
    ],
  },
  {
    id: 'tpl-disk-cleanup',
    name: '磁盘清理',
    description: '清理磁盘空间，删除过期日志和临时文件',
    icon: 'ClearOutlined',
    steps: [
      { name: '检查磁盘使用率', type: 'script', script: 'df -h ${mount_point}', timeout: 10 },
      { name: '审批确认', type: 'approval', approvers: ['admin'], timeout: 3600, approvalTimeoutAction: 'auto_reject' },
      { name: '清理过期日志', type: 'script', script: 'find ${log_dir} -name "*.log" -mtime +${retention_days} -delete', timeout: 300 },
      { name: '清理临时文件', type: 'script', script: 'rm -rf /tmp/app-cache/*', timeout: 60 },
      { name: '通知完成', type: 'notify', notifyTitle: '磁盘清理完成', notifyContent: '${mount_point} 清理完成', notifyChannels: ['email'] },
    ],
    variables: [
      { name: 'mount_point', type: 'string', defaultValue: '/', description: '挂载点' },
      { name: 'log_dir', type: 'string', defaultValue: '/var/log', description: '日志目录' },
      { name: 'retention_days', type: 'number', defaultValue: '30', description: '日志保留天数' },
    ],
  },
  {
    id: 'tpl-log-rotate',
    name: '日志轮转',
    description: '执行日志轮转，压缩历史日志并清理过期文件',
    icon: 'FileTextOutlined',
    steps: [
      { name: '执行日志轮转', type: 'script', script: 'logrotate -f /etc/logrotate.d/${config_name}', timeout: 120 },
      { name: '压缩历史日志', type: 'script', script: 'gzip ${log_dir}/*.log.*', timeout: 300 },
      { name: '通知完成', type: 'notify', notifyTitle: '日志轮转完成', notifyContent: '日志轮转任务已完成', notifyChannels: ['dingtalk'] },
    ],
    variables: [
      { name: 'config_name', type: 'string', defaultValue: 'app', description: 'logrotate 配置名' },
      { name: 'log_dir', type: 'string', defaultValue: '/var/log/app', description: '日志目录' },
    ],
  },
  {
    id: 'tpl-config-rollback',
    name: '配置回滚',
    description: '回滚配置到上一个版本，需审批确认',
    icon: 'RollbackOutlined',
    steps: [
      { name: '备份当前配置', type: 'script', script: 'cp ${config_path} ${config_path}.bak.$(date +%s)', timeout: 30 },
      { name: '审批回滚操作', type: 'approval', approvers: ['admin', 'ops-lead'], timeout: 1800, approvalTimeoutAction: 'auto_reject' },
      { name: '执行回滚', type: 'script', script: 'git -C ${config_repo} checkout HEAD~1 -- ${config_file}', timeout: 60 },
      { name: '重载配置', type: 'script', script: 'systemctl reload ${service_name}', timeout: 30 },
      { name: '验证服务状态', type: 'script', script: 'curl -f http://localhost:${port}/health', timeout: 30 },
      { name: '通知相关人员', type: 'notify', notifyTitle: '配置回滚完成', notifyContent: '${service_name} 配置已回滚', notifyChannels: ['dingtalk', 'email'] },
    ],
    variables: [
      { name: 'config_path', type: 'string', description: '配置文件路径' },
      { name: 'config_repo', type: 'string', description: '配置仓库路径' },
      { name: 'config_file', type: 'string', description: '配置文件名' },
      { name: 'service_name', type: 'string', description: '服务名称' },
      { name: 'port', type: 'string', defaultValue: '8080', description: '服务端口' },
    ],
  },
];

/** 模板图标组件映射 */
const TEMPLATE_ICONS: Record<string, React.ReactNode> = {
  SyncOutlined: <SyncOutlined style={{ fontSize: 32, color: '#1890ff' }} />,
  ClearOutlined: <ClearOutlined style={{ fontSize: 32, color: '#52c41a' }} />,
  FileTextOutlined: <FileTextOutlined style={{ fontSize: 32, color: '#722ed1' }} />,
  RollbackOutlined: <RollbackOutlined style={{ fontSize: 32, color: '#fa8c16' }} />,
};

const WorkflowList: React.FC = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<Workflow[]>([]);
  const [total, setTotal] = useState(0);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 20 });
  const [keyword, setKeyword] = useState('');
  const [templateModalOpen, setTemplateModalOpen] = useState(false);
  const [templateLoading, setTemplateLoading] = useState(false);

  /** 统计数据 */
  const [stats, setStats] = useState({ total: 0, active: 0, weekExecutions: 0, successRate: '--' });

  /**
   * 加载工作流列表数据
   */
  const loadData = useCallback(async (page?: number, pageSize?: number, kw?: string) => {
    setLoading(true);
    try {
      const result = await listWorkflows({
        page: page || pagination.current,
        pageSize: pageSize || pagination.pageSize,
        keyword: kw !== undefined ? kw : keyword,
      });
      setData(result.list || []);
      setTotal(result.total || 0);
      // 简单计算统计数据
      const list = result.list || [];
      const activeCount = list.filter((w) => w.enabled).length;
      setStats({
        total: result.total || 0,
        active: activeCount,
        weekExecutions: 0, // 需后端提供
        successRate: '--',
      });
    } catch {
      // API 尚未就绪时静默处理
    } finally {
      setLoading(false);
    }
  }, [pagination, keyword]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  /**
   * 切换工作流启用/禁用状态
   * @param record 目标工作流
   * @param checked 目标状态
   */
  const handleToggleEnabled = async (record: Workflow, checked: boolean) => {
    try {
      await updateWorkflow(record.id, { enabled: checked });
      message.success(checked ? '已启用' : '已禁用');
      loadData();
    } catch {
      message.error('操作失败');
    }
  };

  /**
   * 删除工作流
   * @param id 工作流 ID
   */
  const handleDelete = async (id: string) => {
    try {
      await deleteWorkflow(id);
      message.success('删除成功');
      loadData();
    } catch {
      message.error('删除失败');
    }
  };

  /**
   * 触发工作流执行
   * @param workflowId 工作流 ID
   */
  const handleTrigger = async (workflowId: string) => {
    try {
      await triggerExecution(workflowId);
      message.success('已触发执行');
    } catch {
      message.error('触发失败');
    }
  };

  /**
   * 从模板创建工作流
   * @param template 选择的模板
   */
  const handleCreateFromTemplate = async (template: WorkflowTemplate) => {
    setTemplateLoading(true);
    try {
      await createFromTemplate(template.id, `${template.name}-${Date.now()}`);
      message.success('从模板创建成功');
      setTemplateModalOpen(false);
      loadData();
    } catch {
      message.error('创建失败');
    } finally {
      setTemplateLoading(false);
    }
  };

  /** 表格列定义 */
  const columns: ColumnsType<Workflow> = [
    {
      title: '工作流名称',
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (name: string, record) => (
        <a onClick={() => navigate(`/workflows/${record.id}/edit`)}>{name}</a>
      ),
    },
    {
      title: '触发类型',
      dataIndex: 'triggerType',
      key: 'triggerType',
      width: 120,
      render: (type: string) => {
        const info = TRIGGER_TYPE_MAP[type] || { label: type, color: 'default' };
        return <Tag color={info.color}>{info.label}</Tag>;
      },
    },
    {
      title: '步骤数',
      key: 'stepCount',
      width: 80,
      align: 'center',
      render: (_: unknown, record) => record.steps?.length || 0,
    },
    {
      title: '启用状态',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 100,
      align: 'center',
      render: (enabled: boolean, record) => (
        <Switch
          checked={enabled}
          size="small"
          onChange={(checked) => handleToggleEnabled(record, checked)}
        />
      ),
    },
    {
      title: '最近执行',
      key: 'lastExecution',
      width: 160,
      render: (_: unknown, record) => {
        if (!record.lastExecutedAt) return <Text type="secondary">暂无执行</Text>;
        const statusInfo = EXECUTION_STATUS_MAP[record.lastExecutionStatus || ''] || { label: '-', color: 'default' };
        return (
          <Space size={4}>
            <Tag color={statusInfo.color}>{statusInfo.label}</Tag>
            <Text type="secondary" style={{ fontSize: 12 }}>{record.lastExecutedAt}</Text>
          </Space>
        );
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 240,
      render: (_: unknown, record) => (
        <Space size={4}>
          <Tooltip title="编辑">
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => navigate(`/workflows/${record.id}/edit`)}
            />
          </Tooltip>
          <Tooltip title="触发执行">
            <Button
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              onClick={() => handleTrigger(record.id)}
            />
          </Tooltip>
          <Tooltip title="执行历史">
            <Button
              type="link"
              size="small"
              icon={<HistoryOutlined />}
              onClick={() => navigate(`/workflows/${record.id}/executions`)}
            />
          </Tooltip>
          <Popconfirm
            title="确认删除此工作流？"
            onConfirm={() => handleDelete(record.id)}
            okText="确认"
            cancelText="取消"
          >
            <Tooltip title="删除">
              <Button type="link" size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  /** 统计卡片数据 */
  const statCards = [
    { key: 'total', label: '工作流总数', value: stats.total, icon: <ScheduleOutlined /> },
    { key: 'active', label: '活跃工作流', value: stats.active, icon: <ThunderboltOutlined /> },
    { key: 'weekExec', label: '本周执行次数', value: stats.weekExecutions, icon: <ClockCircleOutlined /> },
    { key: 'successRate', label: '成功率', value: stats.successRate, icon: <AlertOutlined /> },
  ];

  return (
    <div>
      {/* 页面标题与操作按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>自动化编排</Text>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => loadData()}>刷新</Button>
          <Button icon={<ThunderboltOutlined />} onClick={() => setTemplateModalOpen(true)}>从模板创建</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/workflows/new')}>
            新建工作流
          </Button>
        </Space>
      </div>

      {/* 统计卡片行 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {statCards.map((card) => (
          <Col flex={1} key={card.key}>
            <Card
              bordered
              style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
              bodyStyle={{ padding: '16px 20px', textAlign: 'center' }}
            >
              <div style={{ color: '#86909C', fontSize: 14 }}>{card.icon} {card.label}</div>
              <div style={{ fontSize: 24, fontWeight: 600, marginTop: 4 }}>{card.value}</div>
            </Card>
          </Col>
        ))}
      </Row>

      {/* 搜索栏 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Input.Search
          placeholder="搜索工作流名称..."
          style={{ width: 300 }}
          allowClear
          onSearch={(value) => {
            setKeyword(value);
            setPagination((prev) => ({ ...prev, current: 1 }));
            loadData(1, undefined, value);
          }}
        />
      </Card>

      {/* 工作流表格 */}
      <Table
        columns={columns}
        dataSource={data}
        loading={loading}
        rowKey="id"
        size="middle"
        pagination={{
          current: pagination.current,
          pageSize: pagination.pageSize,
          total,
          showSizeChanger: true,
          showQuickJumper: true,
          onChange: (page, pageSize) => {
            setPagination({ current: page, pageSize });
            loadData(page, pageSize);
          },
        }}
      />

      {/* 从模板创建 Modal */}
      <Modal
        title="从模板创建工作流"
        open={templateModalOpen}
        onCancel={() => setTemplateModalOpen(false)}
        footer={null}
        width={720}
        destroyOnClose
      >
        <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
          {PRESET_TEMPLATES.map((tpl) => (
            <Col span={12} key={tpl.id}>
              <Card
                hoverable
                style={{ borderRadius: 8 }}
                bodyStyle={{ padding: '20px', textAlign: 'center' }}
                onClick={() => handleCreateFromTemplate(tpl)}
              >
                {/* 模板图标 */}
                <div style={{ marginBottom: 12 }}>
                  {TEMPLATE_ICONS[tpl.icon] || <ThunderboltOutlined style={{ fontSize: 32, color: '#1890ff' }} />}
                </div>
                {/* 模板名称 */}
                <div style={{ fontSize: 16, fontWeight: 600, marginBottom: 8 }}>{tpl.name}</div>
                {/* 模板描述 */}
                <div style={{ color: '#86909C', fontSize: 13 }}>{tpl.description}</div>
                {/* 步骤数量 */}
                <Tag style={{ marginTop: 8 }}>{tpl.steps.length} 个步骤</Tag>
              </Card>
            </Col>
          ))}
        </Row>
        {templateLoading && (
          <div style={{ textAlign: 'center', marginTop: 16, color: '#86909C' }}>正在创建...</div>
        )}
      </Modal>
    </div>
  );
};

export default WorkflowList;
