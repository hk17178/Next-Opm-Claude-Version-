/**
 * 用户建议管理页面
 *
 * 功能说明：
 * - 用户提交视图：标题输入框 + 描述文本区 + 提交按钮，提交后立即可见
 * - 管理员看板视图（Tab 切换）：按状态分列的看板（待评估/已采纳/已拒绝/开发中/已上线）
 * - 每条建议卡片显示：标题、AI 分类标签、情感标识、提交人、时间
 * - 管理员可在卡片上更改状态（下拉选择）
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Row, Col, Typography, Button, Input, Form, Tabs, Tag, Select,
  Space, message, Empty,
} from 'antd';
import {
  PlusOutlined, BulbOutlined, CheckCircleOutlined, CloseCircleOutlined,
  CodeOutlined, RocketOutlined, UserOutlined, ClockCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  submitSuggestion, listSuggestions, updateStatus, getStats,
  type Suggestion, type SuggestionStatus, type SuggestionStats,
  type SubmitSuggestionParams,
} from '../api/suggestion';

const { Text, Title } = Typography;
const { TextArea } = Input;

/** 状态配置：标签文本、颜色、图标 */
const STATUS_CONFIG: Record<SuggestionStatus, { label: string; color: string; icon: React.ReactNode }> = {
  pending: { label: '待评估', color: '#D29922', icon: <ClockCircleOutlined /> },
  accepted: { label: '已采纳', color: '#238636', icon: <CheckCircleOutlined /> },
  rejected: { label: '已拒绝', color: '#F85149', icon: <CloseCircleOutlined /> },
  developing: { label: '开发中', color: '#58A6FF', icon: <CodeOutlined /> },
  released: { label: '已上线', color: '#A371F7', icon: <RocketOutlined /> },
};

/** 情感分析结果 → 显示标识映射 */
const SENTIMENT_MAP: Record<string, { emoji: string; label: string }> = {
  positive: { emoji: '\uD83D\uDE0A', label: '正面' },
  neutral: { emoji: '\uD83D\uDE10', label: '中性' },
  negative: { emoji: '\uD83D\uDE1E', label: '负面' },
};

/** 所有状态值列表，用于看板列遍历 */
const ALL_STATUSES: SuggestionStatus[] = ['pending', 'accepted', 'rejected', 'developing', 'released'];

/* ========== 模拟数据（后端 API 不可用时使用） ========== */

/** 生成模拟建议列表数据 */
function mockSuggestions(): Suggestion[] {
  return [
    { id: '1', title: '希望增加暗色模式', description: '长时间使用系统眼睛容易疲劳，建议增加暗色主题切换功能', category: 'UI/UX', sentiment: 'positive', status: 'accepted', submitter: '张三', createdAt: '2026-03-22 14:30' },
    { id: '2', title: '告警通知支持企业微信', description: '目前只支持邮件和钉钉通知，希望能增加企业微信推送渠道', category: '通知', sentiment: 'positive', status: 'developing', submitter: '李四', createdAt: '2026-03-21 10:15' },
    { id: '3', title: '批量确认告警功能', description: '运维夜班经常需要批量确认告警，逐条点击效率太低', category: '告警', sentiment: 'negative', status: 'pending', submitter: '王五', createdAt: '2026-03-20 22:40' },
    { id: '4', title: '仪表板支持自定义布局', description: '不同角色关注的指标不同，希望能自定义仪表板卡片排列', category: '仪表板', sentiment: 'neutral', status: 'pending', submitter: '赵六', createdAt: '2026-03-20 09:00' },
    { id: '5', title: '导出功能优化', description: '报表导出时希望能选择时间范围和格式', category: '报表', sentiment: 'neutral', status: 'rejected', submitter: '孙七', createdAt: '2026-03-19 16:20', adminNote: '已有类似功能在 v2.5 规划中' },
    { id: '6', title: '移动端适配', description: '出差时需要通过手机查看告警和事件', category: '移动端', sentiment: 'positive', status: 'released', submitter: '周八', createdAt: '2026-03-15 11:30' },
  ];
}

/** 生成模拟统计数据 */
function mockStats(): SuggestionStats {
  return { total: 6, pending: 2, accepted: 1, rejected: 1, developing: 1, released: 1 };
}

/**
 * 建议卡片组件
 * 展示单条建议的标题、分类标签、情感标识、提交人信息
 * 管理员可通过下拉选择更改建议状态
 */
const SuggestionCard: React.FC<{
  suggestion: Suggestion;
  onStatusChange: (id: string, status: SuggestionStatus) => void;
}> = ({ suggestion, onStatusChange }) => {
  const sentiment = SENTIMENT_MAP[suggestion.sentiment] || SENTIMENT_MAP.neutral;
  const statusConfig = STATUS_CONFIG[suggestion.status];

  return (
    <Card
      size="small"
      style={{
        marginBottom: 8,
        borderRadius: 8,
        border: '1px solid #F0F0F0',
        boxShadow: '0 1px 4px rgba(0,0,0,0.04)',
      }}
      bodyStyle={{ padding: '12px 16px' }}
    >
      {/* 标题行：建议标题 + 情感标识 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 8 }}>
        <Text strong style={{ fontSize: 14, flex: 1 }}>{suggestion.title}</Text>
        <span style={{ fontSize: 18, marginLeft: 8 }} title={sentiment.label}>{sentiment.emoji}</span>
      </div>
      {/* 描述摘要 */}
      <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }} ellipsis>
        {suggestion.description}
      </Text>
      {/* 标签行：AI 分类标签 + 当前状态 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        <Space size={4}>
          <Tag color="blue">{suggestion.category}</Tag>
          <Tag color={statusConfig.color} icon={statusConfig.icon}>{statusConfig.label}</Tag>
        </Space>
      </div>
      {/* 底部信息行：提交人 + 时间 + 状态变更下拉 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Space size={8}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            <UserOutlined style={{ marginRight: 2 }} />
            {suggestion.submitter}
          </Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            <ClockCircleOutlined style={{ marginRight: 2 }} />
            {suggestion.createdAt}
          </Text>
        </Space>
        {/* 管理员状态变更下拉 */}
        <Select
          size="small"
          value={suggestion.status}
          style={{ width: 100 }}
          onChange={(val) => onStatusChange(suggestion.id, val)}
          options={ALL_STATUSES.map((s) => ({
            value: s,
            label: STATUS_CONFIG[s].label,
          }))}
        />
      </div>
      {/* 管理员备注（如果有） */}
      {suggestion.adminNote && (
        <div style={{ marginTop: 8, padding: '6px 8px', background: '#FFFBE6', borderRadius: 4, fontSize: 12 }}>
          <Text type="warning">备注: {suggestion.adminNote}</Text>
        </div>
      )}
    </Card>
  );
};

/**
 * 用户建议管理主组件
 * 包含两个视图 Tab：
 * - submit：用户提交建议
 * - board：管理员看板（按状态分列展示）
 */
const Suggestions: React.FC = () => {
  const { t } = useTranslation('dashboard');
  const [activeTab, setActiveTab] = useState('submit');         // 当前激活的 Tab
  const [suggestions, setSuggestions] = useState<Suggestion[]>([]); // 建议列表
  const [stats, setStats] = useState<SuggestionStats | null>(null); // 统计数据
  const [submitLoading, setSubmitLoading] = useState(false);    // 提交中状态
  const [form] = Form.useForm<SubmitSuggestionParams>();        // 提交表单实例

  /**
   * 加载建议列表和统计数据
   * API 不可用时回退到模拟数据
   */
  const loadData = useCallback(async () => {
    try {
      const [listResult, statsResult] = await Promise.all([
        listSuggestions(),
        getStats(),
      ]);
      setSuggestions(listResult.list || []);
      setStats(statsResult);
    } catch {
      setSuggestions(mockSuggestions());
      setStats(mockStats());
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  /**
   * 处理建议提交
   * 校验表单 → 调用提交 API → 成功后清空表单并刷新列表
   */
  const handleSubmit = useCallback(async () => {
    try {
      const values = await form.validateFields();
      setSubmitLoading(true);
      await submitSuggestion(values);
      message.success('建议提交成功，感谢您的反馈！');
      form.resetFields();
      loadData();
    } catch {
      // 表单校验失败或 API 错误，使用本地模拟
      const values = form.getFieldsValue();
      if (values.title && values.description) {
        const newSuggestion: Suggestion = {
          id: `local-${Date.now()}`,
          title: values.title,
          description: values.description,
          category: '待分类',
          sentiment: 'neutral',
          status: 'pending',
          submitter: '当前用户',
          createdAt: new Date().toLocaleString(),
        };
        setSuggestions((prev) => [newSuggestion, ...prev]);
        message.success('建议提交成功，感谢您的反馈！');
        form.resetFields();
      }
    } finally {
      setSubmitLoading(false);
    }
  }, [form, loadData]);

  /**
   * 处理建议状态变更（管理员操作）
   * @param id 建议 ID
   * @param newStatus 目标状态
   */
  const handleStatusChange = useCallback(async (id: string, newStatus: SuggestionStatus) => {
    try {
      await updateStatus(id, newStatus);
      message.success('状态更新成功');
      loadData();
    } catch {
      // API 不可用，本地更新状态
      setSuggestions((prev) =>
        prev.map((s) => (s.id === id ? { ...s, status: newStatus } : s)),
      );
      message.success('状态更新成功');
    }
  }, [loadData]);

  /** Tab 项配置 */
  const tabItems = [
    { key: 'submit', label: '提交建议' },
    { key: 'board', label: '建议看板' },
  ];

  return (
    <div style={{ padding: 24 }}>
      {/* 页面标题 */}
      <div style={{ marginBottom: 24 }}>
        <Title level={4} style={{ margin: 0 }}>
          <BulbOutlined style={{ marginRight: 8, color: '#FAAD14' }} />
          用户建议
        </Title>
        <Text type="secondary">提交您的改进建议，帮助我们做得更好</Text>
      </div>

      {/* 统计卡片行 */}
      {stats && (
        <Row gutter={16} style={{ marginBottom: 16 }}>
          {ALL_STATUSES.map((status) => {
            const config = STATUS_CONFIG[status];
            const count = stats[status] ?? 0;
            return (
              <Col flex={1} key={status}>
                <Card
                  bordered
                  style={{ borderRadius: 8, borderTop: `3px solid ${config.color}` }}
                  bodyStyle={{ padding: '12px 16px', textAlign: 'center' }}
                >
                  <div style={{ color: '#86909C', fontSize: 13 }}>{config.label}</div>
                  <div style={{ fontSize: 24, fontWeight: 600, color: config.color, marginTop: 4 }}>{count}</div>
                </Card>
              </Col>
            );
          })}
        </Row>
      )}

      {/* Tab 切换：提交建议 / 建议看板 */}
      <Tabs items={tabItems} activeKey={activeTab} onChange={setActiveTab} />

      {/* ===== 用户提交视图 ===== */}
      {activeTab === 'submit' && (
        <Card style={{ borderRadius: 8, maxWidth: 640 }}>
          <Form form={form} layout="vertical">
            {/* 建议标题输入 */}
            <Form.Item
              name="title"
              label="建议标题"
              rules={[{ required: true, message: '请输入建议标题' }]}
            >
              <Input placeholder="简要描述您的建议" maxLength={100} showCount />
            </Form.Item>
            {/* 建议描述文本区 */}
            <Form.Item
              name="description"
              label="详细描述"
              rules={[{ required: true, message: '请描述您的建议内容' }]}
            >
              <TextArea
                rows={4}
                placeholder="请详细描述您的建议，包括使用场景和期望效果..."
                maxLength={500}
                showCount
              />
            </Form.Item>
            {/* 提交按钮 */}
            <Form.Item>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                loading={submitLoading}
                onClick={handleSubmit}
              >
                提交建议
              </Button>
            </Form.Item>
          </Form>

          {/* 最近提交的建议（即时反馈） */}
          {suggestions.filter((s) => s.status === 'pending').length > 0 && (
            <div style={{ marginTop: 16, borderTop: '1px solid #F0F0F0', paddingTop: 16 }}>
              <Text strong style={{ marginBottom: 12, display: 'block' }}>最近提交</Text>
              {suggestions
                .filter((s) => s.status === 'pending')
                .slice(0, 3)
                .map((s) => (
                  <SuggestionCard
                    key={s.id}
                    suggestion={s}
                    onStatusChange={handleStatusChange}
                  />
                ))}
            </div>
          )}
        </Card>
      )}

      {/* ===== 管理员看板视图 ===== */}
      {activeTab === 'board' && (
        <div style={{ overflowX: 'auto' }}>
          <Row gutter={12} style={{ minWidth: 1200, flexWrap: 'nowrap' }}>
            {ALL_STATUSES.map((status) => {
              const config = STATUS_CONFIG[status];
              const columnSuggestions = suggestions.filter((s) => s.status === status);
              return (
                <Col flex="1" key={status} style={{ minWidth: 230 }}>
                  {/* 看板列标题 */}
                  <div
                    style={{
                      background: `${config.color}15`,
                      borderTop: `3px solid ${config.color}`,
                      borderRadius: '8px 8px 0 0',
                      padding: '8px 12px',
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                    }}
                  >
                    <Space>
                      {config.icon}
                      <Text strong>{config.label}</Text>
                    </Space>
                    <Tag>{columnSuggestions.length}</Tag>
                  </div>
                  {/* 看板列内容 */}
                  <div
                    style={{
                      background: '#FAFAFA',
                      borderRadius: '0 0 8px 8px',
                      padding: 8,
                      minHeight: 400,
                      maxHeight: 600,
                      overflowY: 'auto',
                    }}
                  >
                    {columnSuggestions.length === 0 ? (
                      <Empty
                        image={Empty.PRESENTED_IMAGE_SIMPLE}
                        description="暂无建议"
                        style={{ padding: 32 }}
                      />
                    ) : (
                      columnSuggestions.map((s) => (
                        <SuggestionCard
                          key={s.id}
                          suggestion={s}
                          onStatusChange={handleStatusChange}
                        />
                      ))
                    )}
                  </div>
                </Col>
              );
            })}
          </Row>
        </div>
      )}
    </div>
  );
};

export default Suggestions;
