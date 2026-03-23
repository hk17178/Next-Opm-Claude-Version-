/**
 * 告警详情页 - 展示告警的完整信息、AI 根因分析、关联日志/指标/审计
 * 路由：/alert/detail/:id
 * 数据来源：GET /api/alerts/{id}
 */
import React, { useState, useEffect } from 'react';
import {
  Card, Row, Col, Tag, Button, Tabs, Table, Space, Descriptions,
  Typography, Progress, Skeleton, Empty, message,
} from 'antd';
import {
  ArrowLeftOutlined, CheckCircleOutlined, BellOutlined,
  RobotOutlined, LikeOutlined, DislikeOutlined,
} from '@ant-design/icons';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { fetchAlertDetail, acknowledgeAlert, type AlertDetail } from '../api/alert';

const { Text, Title } = Typography;

const SEVERITY_COLORS: Record<string, string> = {
  P0: '#F53F3F', P1: '#FF7D00', P2: '#3491FA', P3: '#86909C', P4: '#C9CDD4',
};
const STATUS_COLOR: Record<string, string> = {
  firing: '#F53F3F', acknowledged: '#FF7D00', resolved: '#00B42A', suppressed: '#86909C',
};

const AlertDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation('alert');
  const [loading, setLoading] = useState(true);
  const [detail, setDetail] = useState<AlertDetail | null>(null);
  const [ackLoading, setAckLoading] = useState(false);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    fetchAlertDetail(id)
      .then(setDetail)
      .catch(() => {/* API 未就绪，保持空状态 */})
      .finally(() => setLoading(false));
  }, [id]);

  const handleAck = async () => {
    if (!id) return;
    setAckLoading(true);
    try {
      await acknowledgeAlert(id);
      message.success(t('list.action.ackSuccess'));
      setDetail(prev => prev ? { ...prev, status: 'acknowledged' } : prev);
    } catch {
      message.error(t('list.action.ackFailed'));
    } finally {
      setAckLoading(false);
    }
  };

  if (loading) return <Card><Skeleton active paragraph={{ rows: 8 }} /></Card>;
  if (!detail) return (
    <Card>
      <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(-1)} style={{ marginBottom: 16 }}>
        {t('detail.back')}
      </Button>
      <Empty description={t('detail.noData')} />
    </Card>
  );

  const ai = detail.aiAnalysis;

  return (
    <div style={{ padding: '0 0 24px' }}>
      {/* 顶部导航栏 */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(-1)}>
            {t('detail.back')}
          </Button>
          <Title level={4} style={{ margin: 0 }}>{t('detail.title')}</Title>
        </Space>
        <Space>
          {detail.status === 'firing' && (
            <Button
              type="primary"
              icon={<CheckCircleOutlined />}
              loading={ackLoading}
              onClick={handleAck}
            >
              {t('detail.acknowledge')}
            </Button>
          )}
          <Button icon={<BellOutlined />}>{t('detail.silence')}</Button>
        </Space>
      </div>

      {/* 严重程度 + 状态 + 内容标题 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
          <Tag
            style={{
              background: SEVERITY_COLORS[detail.severity],
              color: '#fff',
              border: 'none',
              fontSize: 16,
              fontWeight: 700,
              padding: '4px 12px',
              borderRadius: 4,
              flexShrink: 0,
            }}
          >
            {detail.severity}
          </Tag>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 18, fontWeight: 600, marginBottom: 8 }}>{detail.content}</div>
            <Tag color={STATUS_COLOR[detail.status]}>{detail.status}</Tag>
            {detail.isIronRule && <Tag color="red" style={{ marginLeft: 4 }}>铁律</Tag>}
          </div>
        </div>
      </Card>

      <Row gutter={16}>
        {/* 左列：基本信息 + Tabs */}
        <Col span={16}>
          {/* 基本信息 */}
          <Card title={t('detail.title')} style={{ marginBottom: 16, borderRadius: 8 }}>
            <Descriptions column={2} size="small">
              <Descriptions.Item label={t('detail.info.source')}>{detail.source}</Descriptions.Item>
              <Descriptions.Item label={t('detail.info.service')}>{detail.service || '--'}</Descriptions.Item>
              <Descriptions.Item label={t('detail.info.rule')}>{detail.rule || '--'}</Descriptions.Item>
              <Descriptions.Item label={t('detail.info.triggerTime')}>{detail.triggerTime}</Descriptions.Item>
              <Descriptions.Item label={t('detail.info.duration')}>{detail.duration}</Descriptions.Item>
              <Descriptions.Item label={t('detail.info.business')}>{detail.business || '--'}</Descriptions.Item>
              <Descriptions.Item label={t('detail.info.assetGrade')}>{detail.assetGrade || '--'}</Descriptions.Item>
              {detail.incidentId && (
                <Descriptions.Item label={t('detail.info.incident')}>
                  <Text type="warning">{detail.incidentId}</Text>
                </Descriptions.Item>
              )}
              {detail.tags && detail.tags.length > 0 && (
                <Descriptions.Item label={t('detail.info.tags')} span={2}>
                  {detail.tags.map(tag => <Tag key={tag}>{tag}</Tag>)}
                </Descriptions.Item>
              )}
            </Descriptions>
          </Card>

          {/* 关联数据 Tabs */}
          <Card style={{ borderRadius: 8 }}>
            <Tabs
              items={[
                {
                  key: 'logs',
                  label: t('detail.tab.relatedLogs'),
                  children: (
                    <Table
                      size="small"
                      dataSource={detail.relatedLogs || []}
                      pagination={false}
                      locale={{ emptyText: t('detail.noData') }}
                      columns={[
                        { title: '时间', dataIndex: 'time', key: 'time', width: 160 },
                        {
                          title: t('detail.relatedLogs.level'), dataIndex: 'level', key: 'level', width: 70,
                          render: (lv: string) => (
                            <Tag color={lv === 'ERROR' ? 'red' : lv === 'WARN' ? 'orange' : 'blue'}>{lv}</Tag>
                          ),
                        },
                        { title: t('detail.relatedLogs.host'), dataIndex: 'host', key: 'host', width: 120 },
                        { title: t('detail.relatedLogs.message'), dataIndex: 'message', key: 'message' },
                      ]}
                    />
                  ),
                },
                {
                  key: 'metrics',
                  label: t('detail.tab.relatedMetrics'),
                  children: (
                    <Table
                      size="small"
                      dataSource={detail.relatedMetrics || []}
                      pagination={false}
                      locale={{ emptyText: t('detail.noData') }}
                      columns={[
                        { title: t('detail.relatedMetrics.name'), dataIndex: 'name', key: 'name' },
                        { title: t('detail.relatedMetrics.current'), dataIndex: 'current', key: 'current', width: 120 },
                        { title: t('detail.relatedMetrics.baseline'), dataIndex: 'baseline', key: 'baseline', width: 120 },
                        {
                          title: t('detail.relatedMetrics.deviation'), dataIndex: 'deviation', key: 'deviation', width: 120,
                          render: (v: string) => <Text type="danger">{v}</Text>,
                        },
                      ]}
                    />
                  ),
                },
                {
                  key: 'audit',
                  label: t('detail.tab.audit'),
                  children: (
                    <Table
                      size="small"
                      dataSource={detail.auditLog || []}
                      pagination={false}
                      locale={{ emptyText: t('detail.noData') }}
                      columns={[
                        { title: '时间', dataIndex: 'time', key: 'time', width: 160 },
                        { title: t('detail.audit.user'), dataIndex: 'user', key: 'user', width: 100 },
                        { title: t('detail.audit.action'), dataIndex: 'action', key: 'action', width: 120 },
                        { title: t('detail.audit.note'), dataIndex: 'note', key: 'note' },
                      ]}
                    />
                  ),
                },
              ]}
            />
          </Card>
        </Col>

        {/* 右列：AI 分析 */}
        <Col span={8}>
          <Card
            title={
              <Space>
                <RobotOutlined style={{ color: '#3491FA' }} />
                {t('detail.ai.title')}
              </Space>
            }
            style={{ borderRadius: 8 }}
          >
            {ai ? (
              <div>
                {/* 置信度 */}
                <div style={{ marginBottom: 16 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                    <Text type="secondary">{t('detail.ai.confidence')}</Text>
                    <Text strong>{ai.confidence}%</Text>
                  </div>
                  <Progress
                    percent={ai.confidence}
                    strokeColor={ai.confidence > 80 ? '#00B42A' : ai.confidence > 50 ? '#FF7D00' : '#F53F3F'}
                    showInfo={false}
                    size="small"
                  />
                </div>

                {/* 分类 + 根因 */}
                <div style={{ marginBottom: 12 }}>
                  <Text type="secondary">{t('detail.ai.category')}：</Text>
                  <Tag color="blue">{ai.category}</Tag>
                </div>
                <div style={{ marginBottom: 12 }}>
                  <Text type="secondary">{t('detail.ai.rootCause')}：</Text>
                  <div style={{ marginTop: 4, padding: '8px 12px', background: 'rgba(0,0,0,0.03)', borderRadius: 4 }}>
                    {ai.rootCause}
                  </div>
                </div>

                {/* 证据 */}
                {ai.evidence.length > 0 && (
                  <div style={{ marginBottom: 12 }}>
                    <Text type="secondary">{t('detail.ai.evidence')}：</Text>
                    <ul style={{ marginTop: 4, paddingLeft: 16, marginBottom: 0 }}>
                      {ai.evidence.map((e, i) => (
                        <li key={i} style={{ fontSize: 13, color: '#4E5969' }}>{e}</li>
                      ))}
                    </ul>
                  </div>
                )}

                {/* 建议处置 */}
                <div style={{ marginBottom: 12 }}>
                  <Text type="secondary">{t('detail.ai.suggestion')}：</Text>
                  <div style={{ marginTop: 4, padding: '8px 12px', background: '#F0F9FF', borderRadius: 4, borderLeft: '3px solid #3491FA' }}>
                    {ai.suggestion}
                  </div>
                </div>

                {/* 预计恢复 + 分析耗时 */}
                <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
                  <Text type="secondary">{t('detail.ai.estimatedRecovery')}：<Text>{ai.estimatedRecovery}</Text></Text>
                  <Text type="secondary">{t('detail.ai.analysisDuration')}：<Text>{ai.analysisDuration}</Text></Text>
                </div>

                {/* 反馈按钮 */}
                <Space>
                  <Button size="small" icon={<LikeOutlined />}>{t('detail.ai.helpful')}</Button>
                  <Button size="small" icon={<DislikeOutlined />}>{t('detail.ai.notHelpful')}</Button>
                </Space>
              </div>
            ) : (
              <Empty description={t('detail.noData')} />
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default AlertDetailPage;
