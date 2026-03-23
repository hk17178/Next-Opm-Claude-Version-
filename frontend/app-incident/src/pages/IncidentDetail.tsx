/**
 * 事件详情页面 - 展示单个事件的完整信息，支持状态流转、评论互动、复盘填写
 * 包含：事件头部信息卡片、时间线+评论区、AI 分析面板、关联信息、效率指标、复盘表单
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Row, Col, Tag, Button, Space, Timeline, Descriptions, Typography,
  Tabs, Input, Form, message, List, Avatar,
} from 'antd';
import {
  ArrowLeftOutlined, ArrowUpOutlined, SwapOutlined, CloseCircleOutlined,
  RobotOutlined, UserOutlined, DesktopOutlined, CheckCircleOutlined,
  WarningOutlined, SendOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from 'react-router-dom';
import {
  fetchIncidentDetail, updateIncidentStatus, addComment, fetchComments,
  type Incident, type IncidentComment,
} from '../api/incident';

const { Text, Title, Paragraph } = Typography;
const { TextArea } = Input;

/** 严重级别对应的颜色映射 */
const SEVERITY_COLORS: Record<string, string> = {
  P0: '#F53F3F', P1: '#FF7D00', P2: '#3491FA', P3: '#86909C',
};

/**
 * 状态流转规则映射
 * 定义每个状态可以流转到哪些目标状态
 * open → acknowledged → resolved → closed
 * processing → pending_review → closed
 */
const STATUS_FLOW: Record<string, string[]> = {
  open: ['acknowledged'],
  acknowledged: ['resolved'],
  resolved: ['closed'],
  processing: ['pending_review'],
  pending_review: ['closed'],
};

/** 状态对应的颜色映射 */
const STATUS_COLORS: Record<string, string> = {
  open: '#F53F3F',          // 红色 - 待处理
  acknowledged: '#FF7D00',  // 橙色 - 已确认
  processing: '#FF7D00',    // 橙色 - 处理中
  resolved: '#00B42A',      // 绿色 - 已解决
  pending_review: '#3491FA',// 蓝色 - 待复盘
  closed: '#86909C',        // 灰色 - 已关闭
};

/** 时间线事件类型对应的图标 */
const TIMELINE_ICONS: Record<string, React.ReactNode> = {
  system: <DesktopOutlined style={{ color: '#3491FA' }} />,     // 系统事件
  human: <UserOutlined style={{ color: '#722ED1' }} />,         // 人工操作
  ai: <RobotOutlined style={{ color: '#2E75B6' }} />,           // AI 分析
  recovery: <CheckCircleOutlined style={{ color: '#00B42A' }} />, // 恢复事件
};

/**
 * 事件详情组件
 * 布局结构：
 * - 顶部：事件头部卡片（基本信息、状态流转按钮、操作按钮组）
 * - 中间左列（60%）：事件时间线 + 评论区
 * - 中间右列（40%）：AI 分析、关联信息、效率指标
 * - 底部：复盘/处置/通知 Tab 面板
 */
const IncidentDetail: React.FC = () => {
  const { t } = useTranslation('incident');
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();              // 从路由获取事件 ID
  const [activeTab, setActiveTab] = useState('postmortem'); // 底部活动 Tab
  const [incident, setIncident] = useState<Incident | null>(null);  // 事件详情数据
  const [comments, setComments] = useState<IncidentComment[]>([]);  // 评论列表
  const [commentText, setCommentText] = useState('');       // 评论输入内容
  const [submittingComment, setSubmittingComment] = useState(false); // 评论提交中状态
  const [statusLoading, setStatusLoading] = useState(false); // 状态流转加载状态

  /**
   * 加载事件详情
   * 根据路由中的 id 参数调用详情 API
   */
  const loadDetail = useCallback(async () => {
    if (!id) return;
    try {
      // request<T> 已自动解包，fetchIncidentDetail 直接返回 Incident
      const result = await fetchIncidentDetail(id);
      setIncident(result || null);
    } catch {
      // API 尚未就绪
    }
  }, [id]);

  /**
   * 加载事件评论列表
   * 根据路由中的 id 参数调用评论列表 API
   */
  const loadComments = useCallback(async () => {
    if (!id) return;
    try {
      // request<T> 已自动解包，fetchComments 直接返回 IncidentComment[]
      const result = await fetchComments(id);
      setComments(result || []);
    } catch {
      // API 尚未就绪
    }
  }, [id]);

  /** 组件挂载时加载详情和评论 */
  useEffect(() => {
    loadDetail();
    loadComments();
  }, [loadDetail, loadComments]);

  /**
   * 处理状态流转操作
   * @param newStatus 目标状态
   */
  const handleStatusChange = useCallback(async (newStatus: string) => {
    if (!id) return;
    setStatusLoading(true);
    try {
      await updateIncidentStatus(id, newStatus);
      message.success(t('detail.statusUpdateSuccess'));
      loadDetail(); // 状态更新成功后重新加载详情
    } catch {
      message.error(t('detail.statusUpdateFail'));
    } finally {
      setStatusLoading(false);
    }
  }, [id, loadDetail, t]);

  /**
   * 处理添加评论
   * 校验评论内容非空 → 调用添加评论 API → 成功后清空输入并刷新列表
   */
  const handleAddComment = useCallback(async () => {
    if (!id || !commentText.trim()) return;
    setSubmittingComment(true);
    try {
      await addComment(id, commentText.trim());
      setCommentText('');
      message.success(t('detail.commentSuccess'));
      loadComments(); // 评论成功后刷新评论列表
    } catch {
      message.error(t('detail.commentFail'));
    } finally {
      setSubmittingComment(false);
    }
  }, [id, commentText, loadComments, t]);

  // 当前事件状态及可流转的目标状态列表
  const currentStatus = incident?.status || 'open';
  const nextStatuses = STATUS_FLOW[currentStatus] || [];

  /** 底部 Tab 配置项 */
  const bottomTabs = [
    { key: 'postmortem', label: t('detail.tab.postmortem') },   // 复盘
    { key: 'handling', label: t('detail.tab.handling') },       // 处置记录
    { key: 'notifications', label: t('detail.tab.notifications') }, // 通知记录
  ];

  return (
    <div>
      {/* 事件头部信息卡片 */}
      <Card bodyStyle={{ padding: '16px 24px' }} style={{ marginBottom: 16, borderRadius: 8 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            {/* 返回按钮 + 事件编号 */}
            <Space style={{ marginBottom: 8 }}>
              <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/list')}>
                {t('detail.back')}
              </Button>
              <Text type="secondary">{incident?.incidentId || '--'}</Text>
            </Space>
            {/* 严重级别标签 + 事件标题 */}
            <div style={{ marginBottom: 8 }}>
              <Tag color={SEVERITY_COLORS[incident?.severity || 'P0']} style={{ fontWeight: 600 }}>
                {incident?.severity || 'P0'}
              </Tag>
              <Title level={4} style={{ display: 'inline', marginLeft: 8 }}>
                {incident?.title || '--'}
              </Title>
            </div>
            {/* 基本信息行：状态、处理人、发现时间、持续时长 */}
            <Space size={16} wrap>
              <Text>
                {t('detail.status')}:{' '}
                <Tag color={STATUS_COLORS[currentStatus]}>{t(`list.status.${currentStatus}`)}</Tag>
              </Text>
              <Text>{t('detail.handler')}: {incident?.handler || '--'}</Text>
              <Text>{t('detail.detected')}: {incident?.createdAt || '--'}</Text>
              <Text>{t('detail.duration')}: {incident?.duration || '--'}</Text>
            </Space>
            {/* 扩展信息行：所属业务、根因分类、关联告警数 */}
            <div style={{ marginTop: 8 }}>
              <Space>
                <Text>{t('detail.business')}: {incident?.business || '--'}</Text>
                <Text>{t('detail.rootCause')}: <Tag>{incident?.rootCause || '--'}</Tag></Text>
                <Text>{t('detail.relatedAlerts')}: {incident?.relatedAlerts ?? '--'}</Text>
              </Space>
            </div>
          </div>
          {/* 操作按钮组：状态流转、升级、转派、关闭 */}
          <Space>
            {/* 根据状态流转规则动态渲染可用的状态操作按钮 */}
            {nextStatuses.map((status) => (
              <Button
                key={status}
                type="primary"
                loading={statusLoading}
                onClick={() => handleStatusChange(status)}
              >
                {t(`detail.action.${status}`)}
              </Button>
            ))}
            <Button icon={<ArrowUpOutlined />}>{t('detail.escalate')}</Button>
            <Button icon={<SwapOutlined />}>{t('detail.reassign')}</Button>
            <Button danger icon={<CloseCircleOutlined />} onClick={() => handleStatusChange('closed')}>
              {t('detail.close')}
            </Button>
          </Space>
        </div>
      </Card>

      {/* 双列布局：左侧时间线+评论、右侧 AI 分析+关联信息 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {/* 左列：事件时间线（60%） */}
        <Col span={14}>
          <Card title={t('detail.timeline')} style={{ borderRadius: 8, minHeight: 400 }}>
            <Timeline items={[]} />
            <div style={{ textAlign: 'center', color: '#86909C' }}>
              {t('detail.noTimelineData')}
            </div>

            {/* 评论区 */}
            <div style={{ marginTop: 24, borderTop: '1px solid #E5E6EB', paddingTop: 16 }}>
              <Text strong style={{ marginBottom: 12, display: 'block' }}>{t('detail.comments')}</Text>
              {/* 评论列表 */}
              <List
                dataSource={comments}
                locale={{ emptyText: t('detail.noComments') }}
                renderItem={(item) => (
                  <List.Item>
                    <List.Item.Meta
                      avatar={<Avatar size="small" icon={<UserOutlined />} />}
                      title={<Text>{item.author} <Text type="secondary" style={{ fontSize: 12 }}>{item.createdAt}</Text></Text>}
                      description={item.content}
                    />
                  </List.Item>
                )}
              />
              {/* 评论输入框 + 发送按钮 */}
              <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
                <TextArea
                  value={commentText}
                  onChange={(e) => setCommentText(e.target.value)}
                  placeholder={t('detail.commentPlaceholder')}
                  rows={2}
                  style={{ flex: 1 }}
                />
                <Button
                  type="primary"
                  icon={<SendOutlined />}
                  loading={submittingComment}
                  onClick={handleAddComment}
                  disabled={!commentText.trim()}
                >
                  {t('detail.sendComment')}
                </Button>
              </div>
            </div>
          </Card>
        </Col>

        {/* 右列：AI 分析 + 关联信息 + 效率指标（40%） */}
        <Col span={10}>
          {/* AI 智能分析面板 */}
          <Card
            title={<span><RobotOutlined /> {t('detail.ai.summary')}</span>}
            style={{ borderLeft: '4px solid #3491FA', borderRadius: 8, marginBottom: 16 }}
          >
            <Paragraph type="secondary">{t('detail.ai.noAnalysis')}</Paragraph>
          </Card>

          {/* 关联信息卡片 */}
          <Card title={t('detail.relatedInfo')} style={{ borderRadius: 8, marginBottom: 16 }}>
            <Descriptions column={1} size="small">
              <Descriptions.Item label={t('detail.relatedAlerts')}>{incident?.relatedAlerts ?? '--'}</Descriptions.Item>
              <Descriptions.Item label={t('detail.relatedOps')}>--</Descriptions.Item>
              <Descriptions.Item label={t('detail.relatedChanges')}>--</Descriptions.Item>
              <Descriptions.Item label={t('detail.affectedAssets')}>--</Descriptions.Item>
              <Descriptions.Item label={t('detail.affectedBusiness')}>{incident?.business || '--'}</Descriptions.Item>
            </Descriptions>
          </Card>

          {/* 效率指标卡片（MTTA/MTTI/MTTR） */}
          <Card title={t('detail.metrics')} style={{ borderRadius: 8 }}>
            <Descriptions column={1} size="small">
              <Descriptions.Item label="MTTA">--</Descriptions.Item>
              <Descriptions.Item label="MTTI">--</Descriptions.Item>
              <Descriptions.Item label="MTTR">{incident?.mttr || '--'}</Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>
      </Row>

      {/* 底部 Tab 面板：复盘 / 处置记录 / 通知记录 */}
      <Card style={{ borderRadius: 8 }}>
        <Tabs items={bottomTabs} activeKey={activeTab} onChange={setActiveTab} />
        {/* 复盘 Tab 内容 */}
        {activeTab === 'postmortem' && (
          <div>
            {/* 复盘提醒提示 */}
            <div style={{ marginBottom: 8, color: '#FF7D00' }}>
              <WarningOutlined /> {t('detail.postmortem.required')}
            </div>
            <Form layout="vertical">
              {/* 根因分析文本域 */}
              <Form.Item label={t('detail.postmortem.rootCauseAnalysis')}>
                <TextArea rows={3} />
              </Form.Item>
              {/* 改进措施 */}
              <Form.Item label={t('detail.postmortem.improvements')}>
                <Button type="dashed">{t('detail.postmortem.addImprovement')}</Button>
              </Form.Item>
              {/* 经验教训 */}
              <Form.Item label={t('detail.postmortem.lessonsLearned')}>
                <TextArea rows={3} />
              </Form.Item>
              <Space>
                <Button>{t('detail.postmortem.saveDraft')}</Button>
                <Button type="primary">{t('detail.postmortem.submit')}</Button>
              </Space>
            </Form>
          </div>
        )}
        {/* 处置记录 Tab 内容（暂无数据） */}
        {activeTab === 'handling' && (
          <div style={{ color: '#86909C', textAlign: 'center', padding: 32 }}>
            {t('detail.noData')}
          </div>
        )}
        {/* 通知记录 Tab 内容（暂无数据） */}
        {activeTab === 'notifications' && (
          <div style={{ color: '#86909C', textAlign: 'center', padding: 32 }}>
            {t('detail.noData')}
          </div>
        )}
      </Card>
    </div>
  );
};

export default IncidentDetail;
