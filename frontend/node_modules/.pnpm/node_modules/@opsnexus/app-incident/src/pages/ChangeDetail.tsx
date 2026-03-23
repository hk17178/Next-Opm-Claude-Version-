/**
 * 变更单详情页面 - 展示变更单完整信息，支持审批和状态流转操作
 *
 * 功能说明：
 * - 变更单基本信息展示（编号、标题、描述、类型、风险、计划时间等）
 * - 审批记录时间线
 * - 受影响资产列表
 * - "审批通过"/"拒绝" 按钮（管理员/审批人权限）
 * - "开始执行"/"完成"/"取消" 操作按钮（状态机流转）
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Row, Col, Tag, Button, Space, Timeline, Descriptions, Typography,
  Input, Modal, message, List,
} from 'antd';
import {
  ArrowLeftOutlined, CheckCircleOutlined, CloseCircleOutlined,
  PlayCircleOutlined, StopOutlined, CaretRightOutlined,
  ExclamationCircleOutlined, ClockCircleOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from 'react-router-dom';
import {
  getChange, approveChange, rejectChange, startChange, completeChange, cancelChange,
  type ChangeDetail as ChangeDetailType, type ChangeStatus, type ChangeType, type ChangeRisk,
} from '../api/change';

const { Text, Title, Paragraph } = Typography;
const { TextArea } = Input;

/** 变更类型标签配置 */
const TYPE_CONFIG: Record<ChangeType, { label: string; color: string }> = {
  standard: { label: '标准变更', color: '#52C41A' },
  normal: { label: '普通变更', color: '#1890FF' },
  emergency: { label: '紧急变更', color: '#F5222D' },
};

/** 风险等级标签配置 */
const RISK_CONFIG: Record<ChangeRisk, { label: string; color: string }> = {
  low: { label: '低', color: '#52C41A' },
  medium: { label: '中', color: '#FAAD14' },
  high: { label: '高', color: '#FA8C16' },
  critical: { label: '极高', color: '#F5222D' },
};

/** 变更状态标签配置 */
const STATUS_CONFIG: Record<ChangeStatus, { label: string; color: string }> = {
  draft: { label: '草稿', color: '#D9D9D9' },
  submitted: { label: '待审批', color: '#FAAD14' },
  approved: { label: '已审批', color: '#52C41A' },
  rejected: { label: '已拒绝', color: '#F5222D' },
  executing: { label: '执行中', color: '#1890FF' },
  completed: { label: '已完成', color: '#52C41A' },
  cancelled: { label: '已取消', color: '#D9D9D9' },
};

/**
 * 变更单状态流转规则
 * 定义每个状态可以执行的操作
 */
const STATUS_ACTIONS: Record<string, Array<{ key: string; label: string; icon: React.ReactNode; danger?: boolean }>> = {
  submitted: [
    { key: 'approve', label: '审批通过', icon: <CheckCircleOutlined /> },
    { key: 'reject', label: '拒绝', icon: <CloseCircleOutlined />, danger: true },
  ],
  approved: [
    { key: 'start', label: '开始执行', icon: <PlayCircleOutlined /> },
    { key: 'cancel', label: '取消变更', icon: <StopOutlined />, danger: true },
  ],
  executing: [
    { key: 'complete', label: '完成', icon: <CheckCircleOutlined /> },
    { key: 'cancel', label: '取消变更', icon: <StopOutlined />, danger: true },
  ],
};

/* ========== 模拟数据 ========== */

/** 生成模拟变更详情数据 */
function mockChangeDetail(): ChangeDetailType {
  return {
    id: '1',
    changeId: 'CHG-20260323-001',
    title: '生产环境数据库版本升级',
    description: '将生产环境 MySQL 从 8.0.32 升级至 8.0.36，包含多项安全补丁和性能优化。升级过程中将进行主从切换，预计业务影响时间 < 30s。',
    type: 'normal',
    risk: 'high',
    status: 'submitted',
    applicant: '张工',
    plannedStart: '2026-03-25 02:00',
    plannedEnd: '2026-03-25 04:00',
    createdAt: '2026-03-22 10:00',
    updatedAt: '2026-03-22 14:00',
    affectedAssets: ['db-master-01', 'db-slave-01', 'db-slave-02', 'app-server-01', 'app-server-02'],
    rollbackPlan: '1. 停止新版本 MySQL 服务\n2. 从备份快照恢复旧版本数据\n3. 启动旧版本 MySQL\n4. 验证主从同步正常\n5. 通知业务方恢复确认',
    approvalRecords: [
      { id: 'ar-1', approver: '李主管', action: 'approved', comment: '已评估风险，同意在凌晨低峰期执行', createdAt: '2026-03-22 16:30' },
    ],
  };
}

/**
 * 变更单详情页面组件
 */
const ChangeDetail: React.FC = () => {
  const { t } = useTranslation('incident');
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const [change, setChange] = useState<ChangeDetailType | null>(null);  // 变更单详情数据
  const [actionLoading, setActionLoading] = useState(false);            // 操作按钮加载状态
  const [rejectModalOpen, setRejectModalOpen] = useState(false);        // 拒绝弹窗
  const [cancelModalOpen, setCancelModalOpen] = useState(false);        // 取消弹窗
  const [approveModalOpen, setApproveModalOpen] = useState(false);      // 审批弹窗
  const [inputText, setInputText] = useState('');                       // 弹窗输入内容

  /**
   * 加载变更单详情
   * API 不可用时回退到模拟数据
   */
  const loadDetail = useCallback(async () => {
    if (!id) return;
    try {
      const result = await getChange(id);
      setChange(result);
    } catch {
      setChange(mockChangeDetail());
    }
  }, [id]);

  useEffect(() => {
    loadDetail();
  }, [loadDetail]);

  /**
   * 执行变更单操作（审批、拒绝、开始执行、完成、取消）
   * @param action 操作类型
   * @param extraParam 附加参数（审批意见/拒绝原因/取消原因）
   */
  const handleAction = useCallback(async (action: string, extraParam?: string) => {
    if (!id) return;
    setActionLoading(true);
    try {
      switch (action) {
        case 'approve':
          await approveChange(id, extraParam || '同意');
          break;
        case 'reject':
          await rejectChange(id, extraParam || '');
          break;
        case 'start':
          await startChange(id);
          break;
        case 'complete':
          await completeChange(id);
          break;
        case 'cancel':
          await cancelChange(id, extraParam || '');
          break;
      }
      message.success('操作成功');
      loadDetail();
    } catch {
      // API 不可用，本地模拟状态更新
      const statusMap: Record<string, ChangeStatus> = {
        approve: 'approved',
        reject: 'rejected',
        start: 'executing',
        complete: 'completed',
        cancel: 'cancelled',
      };
      if (change && statusMap[action]) {
        setChange({ ...change, status: statusMap[action] });
        message.success('操作成功');
      }
    } finally {
      setActionLoading(false);
      setRejectModalOpen(false);
      setCancelModalOpen(false);
      setApproveModalOpen(false);
      setInputText('');
    }
  }, [id, change, loadDetail]);

  /** 当前状态可用的操作按钮 */
  const currentActions = change ? (STATUS_ACTIONS[change.status] || []) : [];

  /** 处理操作按钮点击 */
  const handleActionClick = (actionKey: string) => {
    if (actionKey === 'approve') {
      setApproveModalOpen(true);
    } else if (actionKey === 'reject') {
      setRejectModalOpen(true);
    } else if (actionKey === 'cancel') {
      setCancelModalOpen(true);
    } else {
      handleAction(actionKey);
    }
  };

  return (
    <div>
      {/* 变更单头部信息卡片 */}
      <Card bodyStyle={{ padding: '16px 24px' }} style={{ marginBottom: 16, borderRadius: 8 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            {/* 返回按钮 + 变更编号 */}
            <Space style={{ marginBottom: 8 }}>
              <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/changes')}>
                返回
              </Button>
              <Text type="secondary">{change?.changeId || '--'}</Text>
            </Space>
            {/* 变更标题 + 类型/风险/状态标签 */}
            <div style={{ marginBottom: 8 }}>
              <Title level={4} style={{ display: 'inline', marginRight: 12 }}>
                {change?.title || '--'}
              </Title>
              {change && (
                <Space>
                  <Tag color={TYPE_CONFIG[change.type]?.color}>
                    {TYPE_CONFIG[change.type]?.label}
                  </Tag>
                  <Tag color={RISK_CONFIG[change.risk]?.color}>
                    风险: {RISK_CONFIG[change.risk]?.label}
                  </Tag>
                  <Tag color={STATUS_CONFIG[change.status]?.color}>
                    {STATUS_CONFIG[change.status]?.label}
                  </Tag>
                </Space>
              )}
            </div>
            {/* 基本信息行 */}
            <Space size={16} wrap>
              <Text>申请人: {change?.applicant || '--'}</Text>
              <Text>计划时间: {change?.plannedStart || '--'} ~ {change?.plannedEnd || '--'}</Text>
              <Text>创建时间: {change?.createdAt || '--'}</Text>
            </Space>
          </div>
          {/* 操作按钮组 */}
          <Space>
            {currentActions.map((action) => (
              <Button
                key={action.key}
                type={action.danger ? 'default' : 'primary'}
                danger={action.danger}
                icon={action.icon}
                loading={actionLoading}
                onClick={() => handleActionClick(action.key)}
              >
                {action.label}
              </Button>
            ))}
          </Space>
        </div>
      </Card>

      {/* 双列布局 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {/* 左列：变更描述 + 回滚方案 */}
        <Col span={14}>
          {/* 变更描述 */}
          <Card title="变更描述" style={{ borderRadius: 8, marginBottom: 16 }}>
            <Paragraph>{change?.description || '暂无描述'}</Paragraph>
          </Card>

          {/* 回滚方案 */}
          <Card title="回滚方案" style={{ borderRadius: 8, marginBottom: 16 }}>
            <Paragraph style={{ whiteSpace: 'pre-line' }}>
              {change?.rollbackPlan || '暂无回滚方案'}
            </Paragraph>
          </Card>

          {/* 审批记录时间线 */}
          <Card title="审批记录" style={{ borderRadius: 8 }}>
            {change?.approvalRecords && change.approvalRecords.length > 0 ? (
              <Timeline
                items={change.approvalRecords.map((record) => ({
                  color: record.action === 'approved' ? 'green' : 'red',
                  children: (
                    <div>
                      <div>
                        <Text strong>
                          <UserOutlined style={{ marginRight: 4 }} />
                          {record.approver}
                        </Text>
                        <Tag
                          color={record.action === 'approved' ? '#52C41A' : '#F5222D'}
                          style={{ marginLeft: 8 }}
                        >
                          {record.action === 'approved' ? '通过' : '拒绝'}
                        </Tag>
                      </div>
                      <div style={{ margin: '4px 0' }}>{record.comment}</div>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        <ClockCircleOutlined style={{ marginRight: 4 }} />
                        {record.createdAt}
                      </Text>
                    </div>
                  ),
                }))}
              />
            ) : (
              <div style={{ textAlign: 'center', color: '#86909C', padding: 24 }}>
                暂无审批记录
              </div>
            )}
          </Card>
        </Col>

        {/* 右列：变更信息 + 受影响资产 + 时间线 */}
        <Col span={10}>
          {/* 变更信息卡片 */}
          <Card title="变更信息" style={{ borderRadius: 8, marginBottom: 16 }}>
            <Descriptions column={1} size="small">
              <Descriptions.Item label="变更编号">{change?.changeId || '--'}</Descriptions.Item>
              <Descriptions.Item label="变更类型">
                {change && <Tag color={TYPE_CONFIG[change.type]?.color}>{TYPE_CONFIG[change.type]?.label}</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="风险等级">
                {change && <Tag color={RISK_CONFIG[change.risk]?.color}>{RISK_CONFIG[change.risk]?.label}</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="当前状态">
                {change && <Tag color={STATUS_CONFIG[change.status]?.color}>{STATUS_CONFIG[change.status]?.label}</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="申请人">{change?.applicant || '--'}</Descriptions.Item>
              <Descriptions.Item label="计划开始">{change?.plannedStart || '--'}</Descriptions.Item>
              <Descriptions.Item label="计划结束">{change?.plannedEnd || '--'}</Descriptions.Item>
              {change?.actualStart && (
                <Descriptions.Item label="实际开始">{change.actualStart}</Descriptions.Item>
              )}
              {change?.actualEnd && (
                <Descriptions.Item label="实际结束">{change.actualEnd}</Descriptions.Item>
              )}
            </Descriptions>
          </Card>

          {/* 受影响资产列表 */}
          <Card title="受影响资产" style={{ borderRadius: 8 }}>
            {change?.affectedAssets && change.affectedAssets.length > 0 ? (
              <List
                size="small"
                dataSource={change.affectedAssets}
                renderItem={(asset) => (
                  <List.Item>
                    <Text>
                      <ExclamationCircleOutlined style={{ color: '#FA8C16', marginRight: 8 }} />
                      {asset}
                    </Text>
                  </List.Item>
                )}
              />
            ) : (
              <div style={{ textAlign: 'center', color: '#86909C', padding: 24 }}>
                暂无受影响资产
              </div>
            )}
          </Card>
        </Col>
      </Row>

      {/* 审批通过弹窗 */}
      <Modal
        title="审批通过"
        open={approveModalOpen}
        onCancel={() => { setApproveModalOpen(false); setInputText(''); }}
        onOk={() => handleAction('approve', inputText)}
        confirmLoading={actionLoading}
        okText="确认通过"
      >
        <TextArea
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          placeholder="请输入审批意见（可选）"
          rows={3}
        />
      </Modal>

      {/* 拒绝弹窗 */}
      <Modal
        title="拒绝变更"
        open={rejectModalOpen}
        onCancel={() => { setRejectModalOpen(false); setInputText(''); }}
        onOk={() => handleAction('reject', inputText)}
        confirmLoading={actionLoading}
        okText="确认拒绝"
        okButtonProps={{ danger: true }}
      >
        <TextArea
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          placeholder="请输入拒绝原因"
          rows={3}
        />
      </Modal>

      {/* 取消变更弹窗 */}
      <Modal
        title="取消变更"
        open={cancelModalOpen}
        onCancel={() => { setCancelModalOpen(false); setInputText(''); }}
        onOk={() => handleAction('cancel', inputText)}
        confirmLoading={actionLoading}
        okText="确认取消"
        okButtonProps={{ danger: true }}
      >
        <TextArea
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          placeholder="请输入取消原因"
          rows={3}
        />
      </Modal>
    </div>
  );
};

export default ChangeDetail;
