/**
 * 工作流执行历史页面 - 展示执行记录列表，支持查看执行详情
 *
 * 功能：
 * - 执行记录表格（工作流名/触发来源/状态/开始时间/耗时/操作）
 * - 状态彩色 Tag
 * - 右侧 Drawer 展示执行详情：
 *   - 基本信息：触发源、变量、开始/结束时间
 *   - 步骤时间线（Timeline 组件）
 *   - 审批操作按钮
 */
import React, { useState, useCallback, useEffect } from 'react';
import {
  Table, Tag, Typography, Button, Space, Drawer, Timeline, Card, Descriptions,
  message, Select, Input, Modal, Tooltip,
} from 'antd';
import {
  EyeOutlined, StopOutlined, ReloadOutlined, CodeOutlined,
  AuditOutlined, BranchesOutlined, ClockCircleOutlined, BellOutlined,
  CheckCircleOutlined, CloseCircleOutlined, LoadingOutlined,
  MinusCircleOutlined, ExclamationCircleOutlined, PauseCircleOutlined,
} from '@ant-design/icons';
import { useParams, useNavigate } from 'react-router-dom';
import type { ColumnsType } from 'antd/es/table';
import {
  listExecutions, getExecution, cancelExecution, approveStep, rejectStep,
  type WorkflowExecution, type ExecutionStatus, type ExecutionStepRecord, type StepType,
} from '../api/orchestration';

const { Text, Title } = Typography;

/** 执行状态配置映射（标签文本、颜色、图标） */
const STATUS_CONFIG: Record<ExecutionStatus, { label: string; color: string; icon: React.ReactNode }> = {
  pending: { label: '等待中', color: 'default', icon: <MinusCircleOutlined /> },
  running: { label: '执行中', color: 'processing', icon: <LoadingOutlined /> },
  paused: { label: '已暂停', color: 'warning', icon: <PauseCircleOutlined /> },
  completed: { label: '已完成', color: 'success', icon: <CheckCircleOutlined /> },
  failed: { label: '失败', color: 'error', icon: <CloseCircleOutlined /> },
  cancelled: { label: '已取消', color: 'default', icon: <MinusCircleOutlined /> },
};

/** 步骤类型图标映射 */
const STEP_ICONS: Record<StepType, React.ReactNode> = {
  script: <CodeOutlined />,
  approval: <AuditOutlined />,
  condition: <BranchesOutlined />,
  wait: <ClockCircleOutlined />,
  notify: <BellOutlined />,
};

/** 步骤状态到 Timeline 节点颜色的映射 */
const STEP_STATUS_COLORS: Record<string, string> = {
  pending: 'gray',
  running: 'blue',
  success: 'green',
  failed: 'red',
  skipped: 'gray',
  waiting_approval: 'orange',
};

/**
 * 格式化耗时（秒转为人类可读格式）
 * @param seconds 耗时秒数
 */
function formatDuration(seconds?: number): string {
  if (seconds === undefined || seconds === null) return '-';
  if (seconds < 60) return `${seconds}秒`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}分${seconds % 60}秒`;
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return `${h}时${m}分`;
}

const WorkflowExecutions: React.FC = () => {
  const navigate = useNavigate();
  const { id: workflowId } = useParams<{ id: string }>();

  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<WorkflowExecution[]>([]);
  const [total, setTotal] = useState(0);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 20 });
  const [statusFilter, setStatusFilter] = useState<ExecutionStatus | undefined>(undefined);

  /** Drawer 状态 */
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [selectedExecution, setSelectedExecution] = useState<WorkflowExecution | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  /** 审批拒绝 Modal 状态 */
  const [rejectModalOpen, setRejectModalOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState('');
  const [rejectTarget, setRejectTarget] = useState<{ executionId: string; stepIndex: number } | null>(null);

  /**
   * 加载执行记录列表
   */
  const loadData = useCallback(async (page?: number, pageSize?: number, status?: ExecutionStatus) => {
    setLoading(true);
    try {
      const result = await listExecutions({
        page: page || pagination.current,
        pageSize: pageSize || pagination.pageSize,
        workflowId,
        status: status !== undefined ? status : statusFilter,
      });
      setData(result.list || []);
      setTotal(result.total || 0);
    } catch {
      // API 尚未就绪时静默处理
    } finally {
      setLoading(false);
    }
  }, [pagination, workflowId, statusFilter]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  /**
   * 打开执行详情 Drawer
   * @param executionId 执行记录 ID
   */
  const handleViewDetail = async (executionId: string) => {
    setDrawerOpen(true);
    setDetailLoading(true);
    try {
      const execution = await getExecution(executionId);
      setSelectedExecution(execution);
    } catch {
      message.error('加载执行详情失败');
    } finally {
      setDetailLoading(false);
    }
  };

  /**
   * 取消执行
   * @param executionId 执行记录 ID
   */
  const handleCancel = async (executionId: string) => {
    try {
      await cancelExecution(executionId);
      message.success('已取消执行');
      loadData();
      // 刷新详情
      if (selectedExecution?.id === executionId) {
        handleViewDetail(executionId);
      }
    } catch {
      message.error('取消失败');
    }
  };

  /**
   * 审批通过步骤
   * @param executionId 执行记录 ID
   * @param stepIndex 步骤序号
   */
  const handleApprove = async (executionId: string, stepIndex: number) => {
    try {
      await approveStep(executionId, stepIndex, '审批通过');
      message.success('已通过审批');
      handleViewDetail(executionId);
    } catch {
      message.error('审批失败');
    }
  };

  /**
   * 打开拒绝 Modal
   */
  const handleOpenRejectModal = (executionId: string, stepIndex: number) => {
    setRejectTarget({ executionId, stepIndex });
    setRejectReason('');
    setRejectModalOpen(true);
  };

  /**
   * 提交审批拒绝
   */
  const handleRejectSubmit = async () => {
    if (!rejectTarget) return;
    if (!rejectReason.trim()) {
      message.warning('请输入拒绝原因');
      return;
    }
    try {
      await rejectStep(rejectTarget.executionId, rejectTarget.stepIndex, rejectReason);
      message.success('已拒绝审批');
      setRejectModalOpen(false);
      handleViewDetail(rejectTarget.executionId);
    } catch {
      message.error('操作失败');
    }
  };

  /** 表格列定义 */
  const columns: ColumnsType<WorkflowExecution> = [
    {
      title: '工作流名称',
      dataIndex: 'workflowName',
      key: 'workflowName',
      ellipsis: true,
    },
    {
      title: '触发来源',
      dataIndex: 'triggerSource',
      key: 'triggerSource',
      width: 150,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (status: ExecutionStatus) => {
        const config = STATUS_CONFIG[status] || { label: status, color: 'default', icon: null };
        return (
          <Tag color={config.color} icon={config.icon}>
            {config.label}
          </Tag>
        );
      },
    },
    {
      title: '开始时间',
      dataIndex: 'startedAt',
      key: 'startedAt',
      width: 180,
    },
    {
      title: '耗时',
      dataIndex: 'duration',
      key: 'duration',
      width: 100,
      render: (duration: number) => formatDuration(duration),
    },
    {
      title: '操作',
      key: 'actions',
      width: 140,
      render: (_: unknown, record) => (
        <Space size={4}>
          <Tooltip title="查看详情">
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => handleViewDetail(record.id)}
            />
          </Tooltip>
          {(record.status === 'running' || record.status === 'paused') && (
            <Tooltip title="取消执行">
              <Button
                type="link"
                size="small"
                danger
                icon={<StopOutlined />}
                onClick={() => handleCancel(record.id)}
              />
            </Tooltip>
          )}
        </Space>
      ),
    },
  ];

  /**
   * 渲染步骤时间线节点
   * @param step 步骤执行记录
   */
  const renderStepTimeline = (step: ExecutionStepRecord) => {
    const statusLabel: Record<string, string> = {
      pending: '等待中',
      running: '执行中',
      success: '成功',
      failed: '失败',
      skipped: '已跳过',
      waiting_approval: '待审批',
    };

    return (
      <div>
        {/* 步骤标题行：类型图标 + 名称 + 状态标签 + 耗时 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <span>{STEP_ICONS[step.type]}</span>
          <Text strong>{step.name}</Text>
          <Tag color={STEP_STATUS_COLORS[step.status] || 'default'}>
            {statusLabel[step.status] || step.status}
          </Tag>
          {step.duration !== undefined && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              耗时: {formatDuration(step.duration)}
            </Text>
          )}
        </div>

        {/* 审批类型步骤：待审批时显示操作按钮 */}
        {step.type === 'approval' && step.status === 'waiting_approval' && selectedExecution && (
          <Space style={{ marginBottom: 8 }}>
            <Button
              type="primary"
              size="small"
              icon={<CheckCircleOutlined />}
              onClick={() => handleApprove(selectedExecution.id, step.index)}
            >
              通过
            </Button>
            <Button
              danger
              size="small"
              icon={<CloseCircleOutlined />}
              onClick={() => handleOpenRejectModal(selectedExecution.id, step.index)}
            >
              拒绝
            </Button>
          </Space>
        )}

        {/* 失败步骤显示错误信息 */}
        {step.status === 'failed' && step.error && (
          <div style={{
            background: '#fff2f0',
            border: '1px solid #ffccc7',
            borderRadius: 4,
            padding: '8px 12px',
            marginBottom: 8,
            color: '#cf1322',
            fontSize: 13,
          }}>
            <ExclamationCircleOutlined style={{ marginRight: 4 }} />
            {step.error}
          </div>
        )}

        {/* 可展开查看 Input/Output JSON */}
        {(step.input || step.output) && (
          <div style={{ fontSize: 12 }}>
            {step.input && Object.keys(step.input).length > 0 && (
              <details style={{ marginBottom: 4 }}>
                <summary style={{ cursor: 'pointer', color: '#86909C' }}>查看输入参数</summary>
                <pre style={{
                  background: '#f5f5f5',
                  padding: 8,
                  borderRadius: 4,
                  fontSize: 12,
                  maxHeight: 200,
                  overflow: 'auto',
                }}>
                  {JSON.stringify(step.input, null, 2)}
                </pre>
              </details>
            )}
            {step.output && Object.keys(step.output).length > 0 && (
              <details>
                <summary style={{ cursor: 'pointer', color: '#86909C' }}>查看输出结果</summary>
                <pre style={{
                  background: '#f5f5f5',
                  padding: 8,
                  borderRadius: 4,
                  fontSize: 12,
                  maxHeight: 200,
                  overflow: 'auto',
                }}>
                  {JSON.stringify(step.output, null, 2)}
                </pre>
              </details>
            )}
          </div>
        )}
      </div>
    );
  };

  return (
    <div>
      {/* 页面标题与操作按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>
          {workflowId ? '工作流执行历史' : '全部执行记录'}
        </Title>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => loadData()}>刷新</Button>
          {workflowId && (
            <Button onClick={() => navigate('/workflows')}>返回工作流列表</Button>
          )}
        </Space>
      </div>

      {/* 过滤栏 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Space>
          <Select
            placeholder="状态过滤"
            style={{ width: 140 }}
            allowClear
            value={statusFilter}
            onChange={(value) => {
              setStatusFilter(value);
              setPagination((prev) => ({ ...prev, current: 1 }));
              loadData(1, undefined, value);
            }}
            options={[
              { value: 'pending', label: '等待中' },
              { value: 'running', label: '执行中' },
              { value: 'paused', label: '已暂停' },
              { value: 'completed', label: '已完成' },
              { value: 'failed', label: '失败' },
              { value: 'cancelled', label: '已取消' },
            ]}
          />
        </Space>
      </Card>

      {/* 执行记录表格 */}
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

      {/* 执行详情 Drawer */}
      <Drawer
        title="执行详情"
        placement="right"
        width={640}
        open={drawerOpen}
        onClose={() => { setDrawerOpen(false); setSelectedExecution(null); }}
        destroyOnClose
      >
        {detailLoading ? (
          <div style={{ textAlign: 'center', padding: '40px 0' }}>
            <LoadingOutlined style={{ fontSize: 24 }} />
            <div style={{ marginTop: 8, color: '#86909C' }}>加载中...</div>
          </div>
        ) : selectedExecution ? (
          <div>
            {/* 基本信息 */}
            <Descriptions column={1} bordered size="small" style={{ marginBottom: 24 }}>
              <Descriptions.Item label="工作流名称">{selectedExecution.workflowName}</Descriptions.Item>
              <Descriptions.Item label="触发来源">{selectedExecution.triggerSource}</Descriptions.Item>
              <Descriptions.Item label="执行状态">
                <Tag color={STATUS_CONFIG[selectedExecution.status]?.color}>
                  {STATUS_CONFIG[selectedExecution.status]?.icon} {STATUS_CONFIG[selectedExecution.status]?.label}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="开始时间">{selectedExecution.startedAt}</Descriptions.Item>
              <Descriptions.Item label="结束时间">{selectedExecution.finishedAt || '-'}</Descriptions.Item>
              <Descriptions.Item label="总耗时">{formatDuration(selectedExecution.duration)}</Descriptions.Item>
              {selectedExecution.variables && Object.keys(selectedExecution.variables).length > 0 && (
                <Descriptions.Item label="执行变量">
                  <pre style={{ margin: 0, fontSize: 12 }}>
                    {JSON.stringify(selectedExecution.variables, null, 2)}
                  </pre>
                </Descriptions.Item>
              )}
              {selectedExecution.error && (
                <Descriptions.Item label="错误信息">
                  <Text type="danger">{selectedExecution.error}</Text>
                </Descriptions.Item>
              )}
            </Descriptions>

            {/* 步骤时间线 */}
            <Title level={5}>步骤执行时间线</Title>
            <Timeline
              items={
                (selectedExecution.steps || []).map((step) => ({
                  color: STEP_STATUS_COLORS[step.status] || 'gray',
                  dot: step.status === 'running' ? <LoadingOutlined /> : undefined,
                  children: renderStepTimeline(step),
                }))
              }
            />

            {/* 取消执行按钮 */}
            {(selectedExecution.status === 'running' || selectedExecution.status === 'paused') && (
              <div style={{ textAlign: 'center', marginTop: 16 }}>
                <Button
                  danger
                  icon={<StopOutlined />}
                  onClick={() => handleCancel(selectedExecution.id)}
                >
                  取消执行
                </Button>
              </div>
            )}
          </div>
        ) : (
          <div style={{ textAlign: 'center', color: '#86909C' }}>无数据</div>
        )}
      </Drawer>

      {/* 拒绝审批 Modal */}
      <Modal
        title="拒绝审批"
        open={rejectModalOpen}
        onCancel={() => setRejectModalOpen(false)}
        onOk={handleRejectSubmit}
        okText="确认拒绝"
        cancelText="取消"
        okButtonProps={{ danger: true }}
      >
        <div style={{ marginTop: 16 }}>
          <Text strong style={{ display: 'block', marginBottom: 8 }}>拒绝原因</Text>
          <Input.TextArea
            rows={3}
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
            placeholder="请输入拒绝原因..."
          />
        </div>
      </Modal>
    </div>
  );
};

export default WorkflowExecutions;
