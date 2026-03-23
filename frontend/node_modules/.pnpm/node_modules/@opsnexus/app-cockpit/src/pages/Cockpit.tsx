/**
 * 作战大屏页面 - 实时展示当前活跃事件、处置进度、AI 洞察、值班人员信息
 * 采用三栏布局：左侧活跃事件列表、中间事件时间线、右侧 AI 分析+值班团队
 *
 * 数据来源：
 * - 活跃事件列表：通过 WebSocket 实时推送（useRealtimeIncidents Hook）
 * - 事件时间线：按需从 REST API 获取
 * - 仪表盘指标：从分析 API 获取（MTTR、SLA 等）
 */
import React, { useState, useEffect, useCallback } from 'react';
import { Card, List, Tag, Badge, Typography, Row, Col, Button, Modal, Timeline, Avatar, Skeleton, Alert } from 'antd';
import {
  UserOutlined, PhoneOutlined, RobotOutlined,
  CheckCircleOutlined, ClockCircleOutlined,
  AlertOutlined, BellOutlined, DesktopOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useRealtimeIncidents } from '../api/useRealtimeIncidents';
import { getIncidentTimeline } from '../api/incident';
import { getSLASummary, getDashboardMetrics } from '../api/analytics';
import type { TimelineEntry } from '../api/incident';
import type { SLASummary, DashboardMetrics } from '../api/analytics';

const { Text, Title, Paragraph } = Typography;

/** 严重级别对应的颜色映射 */
const SEVERITY_COLORS: Record<string, string> = {
  P0: '#F53F3F', P1: '#FF7D00', P2: '#3491FA', P3: '#86909C',
};

/** 值班人员数据结构 */
interface OnCallMember {
  name: string;          // 姓名
  role: string;          // 角色（primary 主值班 / backup 备岗 / supervisor 主管）
  status: string;        // 状态（busy 忙碌 / idle 空闲）
  currentTask?: string;  // 当前处理的事件编号（可选）
}

/** 模拟值班人员数据（待值班 API 就绪后替换） */
const MOCK_ON_CALL: OnCallMember[] = [
  { name: '张明', role: 'primary', status: 'busy', currentTask: 'INC-20260322-001' },
  { name: '李伟', role: 'backup', status: 'busy', currentTask: 'INC-20260322-002' },
  { name: '赵磊', role: 'supervisor', status: 'idle' },
];

/** 时间线事件类型对应的图标 */
const TIMELINE_ICONS: Record<string, React.ReactNode> = {
  system: <DesktopOutlined style={{ color: '#3491FA' }} />,     // 系统事件
  human: <UserOutlined style={{ color: '#722ED1' }} />,         // 人工操作
  ai: <RobotOutlined style={{ color: '#2E75B6' }} />,           // AI 分析
  ai_analysis: <RobotOutlined style={{ color: '#2E75B6' }} />,  // AI 分析（API 格式）
  recovery: <CheckCircleOutlined style={{ color: '#00B42A' }} />, // 恢复事件
  comment: <UserOutlined style={{ color: '#722ED1' }} />,       // 评论
  status_change: <ClockCircleOutlined style={{ color: '#FF7D00' }} />, // 状态变更
  assignment: <UserOutlined style={{ color: '#3491FA' }} />,    // 指派
  escalation: <AlertOutlined style={{ color: '#F53F3F' }} />,   // 升级
  notification: <BellOutlined style={{ color: '#FF7D00' }} />,  // 通知
};

/**
 * 作战大屏组件
 * - 顶部状态栏：活跃事件数、各级别统计、SLA、今日告警数
 * - 指标行：MTTR、今日解决、今日告警、已抑制
 * - 三栏布局：活跃事件列表（30%）| 处置时间线（45%）| AI+值班（25%）
 */
const Cockpit: React.FC = () => {
  const { t } = useTranslation('cockpit');
  const [selectedIncident, setSelectedIncident] = useState<string | null>(null); // 当前选中的事件编号
  const [contactModalOpen, setContactModalOpen] = useState(false);  // 联系确认弹窗是否打开
  const [contactTarget, setContactTarget] = useState<OnCallMember | null>(null); // 联系目标人员

  // 通过 WebSocket 实时获取活跃事件列表
  const { incidents, connected, error: wsError } = useRealtimeIncidents();

  // 时间线数据状态
  const [currentTimeline, setCurrentTimeline] = useState<TimelineEntry[]>([]);
  const [timelineLoading, setTimelineLoading] = useState(false);

  // SLA 摘要和仪表盘指标
  const [sla, setSla] = useState<SLASummary | null>(null);
  const [metrics, setMetrics] = useState<DashboardMetrics | null>(null);
  const [metricsLoading, setMetricsLoading] = useState(true);

  const onCallTeam = MOCK_ON_CALL;

  /**
   * 加载仪表盘指标数据
   * 组件挂载时调用，获取 SLA 和关键运维指标
   */
  useEffect(() => {
    async function loadMetrics() {
      setMetricsLoading(true);
      try {
        const [slaData, metricsData] = await Promise.all([
          getSLASummary(),
          getDashboardMetrics(),
        ]);
        setSla(slaData);
        setMetrics(metricsData);
      } catch {
        // 指标加载失败时静默处理，页面仍可使用
      } finally {
        setMetricsLoading(false);
      }
    }
    loadMetrics();
  }, []);

  /**
   * 选中事件变化时，加载对应的时间线数据
   */
  useEffect(() => {
    if (!selectedIncident) {
      setCurrentTimeline([]);
      return;
    }

    let cancelled = false;
    async function loadTimeline() {
      setTimelineLoading(true);
      try {
        const entries = await getIncidentTimeline(selectedIncident!);
        if (!cancelled) {
          setCurrentTimeline(entries);
        }
      } catch {
        if (!cancelled) {
          setCurrentTimeline([]);
        }
      } finally {
        if (!cancelled) {
          setTimelineLoading(false);
        }
      }
    }
    loadTimeline();

    return () => {
      cancelled = true;
    };
  }, [selectedIncident]);

  /**
   * 处理联系值班人员操作
   * 打开确认弹窗，显示目标人员信息
   * @param member 值班人员信息
   */
  const handleContact = (member: OnCallMember) => {
    setContactTarget(member);
    setContactModalOpen(true);
  };

  // 各严重级别事件数量统计
  const p0Count = incidents.filter((i) => i.severity === 'P0').length;
  const p1Count = incidents.filter((i) => i.severity === 'P1').length;
  const p2Count = incidents.filter((i) => i.severity === 'P2').length;

  return (
    <div>
      {/* FR-26-006: WebSocket 断线时显示橙色警告条 */}
      {!connected && (
        <Alert
          message="实时连接已断开，正在尝试重连..."
          type="warning"
          banner
          showIcon
          style={{ marginBottom: 0 }}
        />
      )}

      {/* 顶部状态栏 - 深色背景，展示全局概览信息 */}
      <Card
        bodyStyle={{
          padding: '12px 24px',
          background: '#1A1A1A',
          color: '#E5E6EB',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          borderRadius: 0,
        }}
        bordered={false}
      >
        {/* 左侧：活跃事件总数及各级别统计 */}
        <div style={{ display: 'flex', gap: 24, alignItems: 'center' }}>
          <span>
            <AlertOutlined style={{ marginRight: 6 }} />
            {t('status.active')}: <Text strong style={{ color: '#E5E6EB', fontSize: 18 }}>{incidents.length}</Text>
          </span>
          {[
            { level: 'P0', count: p0Count },
            { level: 'P1', count: p1Count },
            { level: 'P2', count: p2Count },
          ].map(({ level, count }) => (
            <span key={level}>
              {level}:{' '}
              <Tag color={SEVERITY_COLORS[level]} style={{ margin: 0, fontWeight: 600 }}>
                {count}
              </Tag>
            </span>
          ))}
        </div>
        {/* 右侧：SLA 达成率、今日告警数、系统健康状态 */}
        <div style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
          <span>SLA: <Text strong style={{ color: sla?.met ? '#00B42A' : '#F53F3F' }}>
            {sla ? `${sla.rate}%` : '--'}
          </Text></span>
          <span><BellOutlined /> {t('status.todayAlerts')}: <Text strong style={{ color: '#E5E6EB' }}>
            {metrics ? metrics.today_alerts : '--'}
          </Text></span>
          <Badge status={connected ? 'success' : 'warning'} />
        </div>
      </Card>

      {/* 指标卡片行 - 展示关键运维指标 */}
      <Row gutter={16} style={{ margin: '16px 0' }}>
        {metricsLoading ? (
          // FR-24-003: 加载中显示骨架屏
          [1, 2, 3, 4].map((key) => (
            <Col span={6} key={key}>
              <Card bordered style={{ borderRadius: 8 }} bodyStyle={{ padding: '12px 16px' }}>
                <Skeleton active paragraph={false} title={{ width: '60%' }} />
              </Card>
            </Col>
          ))
        ) : (
          [
            { title: t('metrics.mttr'), value: metrics ? `${metrics.mttr} min` : '--', color: '#2E75B6' },
            { title: t('metrics.todayResolved'), value: metrics ? String(metrics.today_resolved) : '--', color: '#00B42A' },
            { title: t('metrics.todayAlerts'), value: metrics ? String(metrics.today_alerts) : '--', color: '#FF7D00' },
            { title: t('metrics.suppressed'), value: metrics ? String(metrics.suppressed) : '--', color: '#86909C' },
          ].map((m) => (
            <Col span={6} key={m.title}>
              <Card bordered style={{ borderRadius: 8, borderTop: `3px solid ${m.color}` }}
                bodyStyle={{ padding: '12px 16px', textAlign: 'center' }}
              >
                <div style={{ color: '#86909C', fontSize: 13 }}>{m.title}</div>
                <div style={{ fontSize: 24, fontWeight: 600, color: m.color }}>{m.value}</div>
              </Card>
            </Col>
          ))
        )}
      </Row>

      {/* 三栏布局主体 */}
      <Row gutter={0} style={{ marginTop: 0 }}>
        {/* 左栏：活跃事件列表（30%） */}
        <Col span={7} style={{ borderRight: '1px solid #E5E6EB' }}>
          <Card
            title={t('incidents.title')}
            bordered={false}
            bodyStyle={{ padding: 0 }}
            style={{ height: 'calc(100vh - 300px)' }}
          >
            {incidents.length === 0 && !wsError ? (
              // FR-24-004: 无事件时显示空状态
              <div style={{ textAlign: 'center', padding: 48, color: '#86909C' }}>
                <CheckCircleOutlined style={{ fontSize: 48, color: '#00B42A', display: 'block', marginBottom: 16 }} />
                {t('incidents.noActive')}
              </div>
            ) : (
              <List
                dataSource={incidents}
                locale={{ emptyText: t('incidents.noActive') }}
                renderItem={(item) => (
                  <List.Item
                    onClick={() => setSelectedIncident(item.id)}
                    style={{
                      cursor: 'pointer',
                      padding: '12px 16px',
                      // 选中的事件高亮背景
                      background: selectedIncident === item.id ? '#F0F2F5' : 'transparent',
                      // P0 事件左侧红色边线标记
                      borderLeft: item.severity === 'P0' ? '4px solid #F53F3F' : 'none',
                    }}
                  >
                    <List.Item.Meta
                      title={
                        <div>
                          <Tag color={SEVERITY_COLORS[item.severity]}>{item.severity}</Tag>
                          <Text>{item.title}</Text>
                        </div>
                      }
                      description={`${item.handler || item.assignee_id || '--'} · ${item.duration || '--'}`}
                    />
                  </List.Item>
                )}
              />
            )}
          </Card>
        </Col>

        {/* 中栏：处置进度时间线（45%） */}
        <Col span={11} style={{ borderRight: '1px solid #E5E6EB' }}>
          <Card
            title={t('progress.title')}
            bordered={false}
            style={{ height: 'calc(100vh - 300px)', overflow: 'auto' }}
          >
            {selectedIncident ? (
              <>
                {/* 选中事件的编号和标题 */}
                <div style={{ marginBottom: 16 }}>
                  <Text strong>{selectedIncident}</Text>
                  <Text type="secondary" style={{ marginLeft: 12 }}>
                    {incidents.find((i) => i.id === selectedIncident)?.title}
                  </Text>
                </div>
                {timelineLoading ? (
                  // 时间线加载中显示骨架屏
                  <Skeleton active paragraph={{ rows: 4 }} />
                ) : (
                  // 渲染时间线条目，每条显示时间、标题、描述
                  <Timeline
                    items={currentTimeline.map((entry) => ({
                      dot: TIMELINE_ICONS[entry.type] || TIMELINE_ICONS['system'],
                      children: (
                        <div>
                          <Text type="secondary" style={{ fontSize: 12 }}>{entry.time || entry.created_at}</Text>
                          <div><Text strong>{entry.title || entry.content}</Text></div>
                          {entry.description && <Text type="secondary">{entry.description}</Text>}
                        </div>
                      ),
                    }))}
                  />
                )}
              </>
            ) : (
              // 未选中事件时的提示
              <div style={{ textAlign: 'center', padding: 48, color: '#86909C' }}>
                {t('progress.selectIncident')}
              </div>
            )}
          </Card>
        </Col>

        {/* 右栏：AI 洞察 + 值班团队（25%） */}
        <Col span={6}>
          <div style={{ height: 'calc(100vh - 300px)', overflow: 'auto' }}>
            {/* AI 智能洞察面板 */}
            <Card
              title={<span><RobotOutlined /> {t('ai.title')}</span>}
              bordered={false}
              bodyStyle={{ padding: '12px 16px' }}
            >
              {selectedIncident ? (
                <div>
                  {/* AI 根因分析结果 */}
                  <Paragraph>
                    <Text strong>{t('ai.rootCause')}:</Text> {selectedIncident === 'INC-20260322-001'
                      ? '下游银行接口超时导致支付网关积压请求'
                      : '大事务阻塞导致主从同步延迟'}
                  </Paragraph>
                  {/* AI 建议操作 */}
                  <Paragraph>
                    <Text strong>{t('ai.suggestion')}:</Text> {selectedIncident === 'INC-20260322-001'
                      ? '建议启用支付降级方案，切换至备用通道'
                      : '建议 kill 长事务并检查慢查询日志'}
                  </Paragraph>
                  {/* AI 置信度 */}
                  <Tag color="#2E75B6">{t('ai.confidence')}: 87%</Tag>
                </div>
              ) : (
                // 未选中事件时的提示
                <Paragraph style={{ color: '#86909C', fontStyle: 'italic' }}>
                  {t('ai.noInsight')}
                </Paragraph>
              )}
            </Card>

            {/* 值班团队列表 */}
            <Card
              title={t('onCall.title')}
              bordered={false}
              bodyStyle={{ padding: 0 }}
            >
              <List
                dataSource={onCallTeam}
                locale={{ emptyText: t('onCall.noData') }}
                renderItem={(member) => (
                  <List.Item
                    actions={[
                      // 联系按钮，点击弹出确认弹窗
                      <Button
                        key="contact"
                        size="small"
                        icon={<PhoneOutlined />}
                        onClick={() => handleContact(member)}
                      >
                        {t('onCall.contact')}
                      </Button>,
                    ]}
                    style={{ padding: '8px 16px' }}
                  >
                    <List.Item.Meta
                      avatar={<Avatar size="small" icon={<UserOutlined />} />}
                      title={
                        <span>
                          {member.name}{' '}
                          <Tag style={{ fontSize: 11 }}>{t(`onCall.${member.role}`)}</Tag>
                        </span>
                      }
                      description={
                        <span>
                          {/* 根据状态显示当前任务或空闲状态 */}
                          {member.status === 'busy' ? (
                            <><ClockCircleOutlined style={{ color: '#FF7D00' }} /> {member.currentTask}</>
                          ) : (
                            <><CheckCircleOutlined style={{ color: '#00B42A' }} /> {t('onCall.idle')}</>
                          )}
                        </span>
                      }
                    />
                  </List.Item>
                )}
              />
            </Card>
          </div>
        </Col>
      </Row>

      {/* 联系值班人员确认弹窗 */}
      <Modal
        title={t('onCall.contactConfirm.title')}
        open={contactModalOpen}
        onCancel={() => setContactModalOpen(false)}
        onOk={() => setContactModalOpen(false)}
        okText={t('onCall.contactConfirm.ok')}
        cancelText={t('onCall.contactConfirm.cancel')}
      >
        {contactTarget && (
          <p>{t('onCall.contactConfirm.message', { name: contactTarget.name, task: contactTarget.currentTask })}</p>
        )}
      </Modal>
    </div>
  );
};

export default Cockpit;
