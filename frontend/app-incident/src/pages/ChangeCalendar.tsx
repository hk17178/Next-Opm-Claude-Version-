/**
 * 变更日历页面 - 使用日历视图展示未来 30 天的变更排期
 *
 * 功能说明：
 * - 使用 Ant Design Calendar 组件展示变更排期
 * - 有变更的日期显示红色角标和数量
 * - 点击日期展示当天变更列表
 * - 冲突日期高亮警告
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Calendar, Card, Typography, Badge, Tag, Modal, List, Space, Tooltip,
} from 'antd';
import {
  CalendarOutlined, WarningOutlined, SwapOutlined,
  ClockCircleOutlined, UserOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import {
  getCalendar,
  type CalendarEntry, type Change, type ChangeType, type ChangeRisk, type ChangeStatus,
} from '../api/change';

const { Text, Title } = Typography;

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

/* ========== 模拟数据 ========== */

/** 生成模拟日历数据（未来 30 天内随机分布变更） */
function mockCalendarData(): CalendarEntry[] {
  const entries: CalendarEntry[] = [];
  const today = dayjs();

  /** 模拟变更数据模板 */
  const mockChanges: Array<Omit<Change, 'id' | 'plannedStart' | 'plannedEnd' | 'createdAt' | 'updatedAt'>> = [
    { changeId: 'CHG-001', title: '数据库版本升级', description: '', type: 'normal', risk: 'high', status: 'approved', applicant: '张工', affectedAssets: ['db-01'], rollbackPlan: '' },
    { changeId: 'CHG-002', title: 'Nginx 配置优化', description: '', type: 'standard', risk: 'low', status: 'approved', applicant: '李工', affectedAssets: ['nginx-01'], rollbackPlan: '' },
    { changeId: 'CHG-003', title: 'K8s 集群扩容', description: '', type: 'normal', risk: 'medium', status: 'submitted', applicant: '王工', affectedAssets: ['k8s-prod'], rollbackPlan: '' },
    { changeId: 'CHG-004', title: '支付服务热修复', description: '', type: 'emergency', risk: 'critical', status: 'executing', applicant: '赵工', affectedAssets: ['pay-api'], rollbackPlan: '' },
    { changeId: 'CHG-005', title: 'Redis 集群迁移', description: '', type: 'normal', risk: 'high', status: 'approved', applicant: '孙工', affectedAssets: ['redis-cluster'], rollbackPlan: '' },
  ];

  // 在未来 30 天内随机分布变更
  for (let i = 0; i < 30; i++) {
    const date = today.add(i, 'day');
    const dateStr = date.format('YYYY-MM-DD');
    // 约 40% 的天数有变更
    if (Math.random() > 0.6) {
      const count = Math.floor(Math.random() * 3) + 1;
      const dayChanges: Change[] = [];
      for (let j = 0; j < count; j++) {
        const template = mockChanges[Math.floor(Math.random() * mockChanges.length)];
        dayChanges.push({
          ...template,
          id: `${dateStr}-${j}`,
          plannedStart: `${dateStr} 02:00`,
          plannedEnd: `${dateStr} 04:00`,
          createdAt: dateStr,
          updatedAt: dateStr,
        });
      }
      // 同一天有 2 个及以上变更视为冲突
      entries.push({
        date: dateStr,
        count,
        changes: dayChanges,
        hasConflict: count >= 2,
      });
    }
  }
  return entries;
}

/**
 * 变更日历页面组件
 */
const ChangeCalendar: React.FC = () => {
  const { t } = useTranslation('incident');
  const navigate = useNavigate();
  const [calendarData, setCalendarData] = useState<CalendarEntry[]>([]);  // 日历数据
  const [selectedDate, setSelectedDate] = useState<string | null>(null);  // 选中的日期
  const [modalOpen, setModalOpen] = useState(false);                      // 变更列表弹窗
  const [selectedChanges, setSelectedChanges] = useState<Change[]>([]);   // 选中日期的变更列表

  /**
   * 加载日历数据
   * 获取当前月份及未来 30 天的变更排期
   * API 不可用时回退到模拟数据
   */
  const loadCalendar = useCallback(async () => {
    const startDate = dayjs().format('YYYY-MM-DD');
    const endDate = dayjs().add(30, 'day').format('YYYY-MM-DD');
    try {
      const data = await getCalendar(startDate, endDate);
      setCalendarData(data);
    } catch {
      setCalendarData(mockCalendarData());
    }
  }, []);

  useEffect(() => {
    loadCalendar();
  }, [loadCalendar]);

  /**
   * 根据日期查找日历条目
   * @param date 日期对象
   * @returns 对应的日历条目，未找到返回 undefined
   */
  const getEntryByDate = (date: Dayjs): CalendarEntry | undefined => {
    const dateStr = date.format('YYYY-MM-DD');
    return calendarData.find((entry) => entry.date === dateStr);
  };

  /**
   * 处理日期点击事件
   * 有变更的日期弹出变更列表弹窗
   */
  const handleDateSelect = (date: Dayjs) => {
    const entry = getEntryByDate(date);
    if (entry && entry.changes.length > 0) {
      setSelectedDate(date.format('YYYY-MM-DD'));
      setSelectedChanges(entry.changes);
      setModalOpen(true);
    }
  };

  /**
   * 自定义日历单元格渲染
   * 有变更的日期显示红色角标和数量，冲突日期高亮
   */
  const dateCellRender = (date: Dayjs) => {
    const entry = getEntryByDate(date);
    if (!entry) return null;

    return (
      <div style={{ position: 'relative' }}>
        {/* 变更数量角标 */}
        <Badge
          count={entry.count}
          style={{
            backgroundColor: entry.hasConflict ? '#F5222D' : '#1890FF',
            fontSize: 10,
          }}
        />
        {/* 冲突警告标识 */}
        {entry.hasConflict && (
          <Tooltip title="该日期存在变更冲突">
            <WarningOutlined
              style={{
                color: '#F5222D',
                fontSize: 12,
                marginLeft: 4,
                animation: 'pulse 2s infinite',
              }}
            />
          </Tooltip>
        )}
        {/* 变更列表预览（最多显示 2 条） */}
        <div style={{ marginTop: 4 }}>
          {entry.changes.slice(0, 2).map((change) => (
            <div
              key={change.id}
              style={{
                fontSize: 11,
                lineHeight: '18px',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                padding: '0 4px',
                marginBottom: 2,
                borderLeft: `2px solid ${TYPE_CONFIG[change.type]?.color || '#1890FF'}`,
                background: entry.hasConflict ? 'rgba(245,34,45,0.06)' : 'rgba(24,144,255,0.06)',
                borderRadius: '0 2px 2px 0',
              }}
            >
              {change.title}
            </div>
          ))}
          {entry.count > 2 && (
            <Text type="secondary" style={{ fontSize: 10, paddingLeft: 4 }}>
              +{entry.count - 2} 更多...
            </Text>
          )}
        </div>
      </div>
    );
  };

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>
          <CalendarOutlined style={{ marginRight: 8 }} />
          变更日历
        </Text>
        <Space>
          {/* 图例说明 */}
          <Tag color="#1890FF">普通变更</Tag>
          <Tag color="#52C41A">标准变更</Tag>
          <Tag color="#F5222D">紧急变更 / 冲突</Tag>
        </Space>
      </div>

      {/* 脉冲动画样式 */}
      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.4; }
        }
      `}</style>

      {/* 日历组件 */}
      <Card style={{ borderRadius: 8 }}>
        <Calendar
          onSelect={handleDateSelect}
          cellRender={(date) => dateCellRender(date as Dayjs)}
        />
      </Card>

      {/* 日期变更列表弹窗 */}
      <Modal
        title={
          <Space>
            <CalendarOutlined />
            <span>{selectedDate} 变更排期</span>
            {selectedChanges.length >= 2 && (
              <Tag color="#F5222D" icon={<WarningOutlined />}>存在冲突</Tag>
            )}
          </Space>
        }
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        footer={null}
        width={700}
      >
        <List
          dataSource={selectedChanges}
          renderItem={(change) => (
            <List.Item
              style={{ cursor: 'pointer' }}
              onClick={() => {
                setModalOpen(false);
                navigate(`/changes/${change.id}`);
              }}
              actions={[
                <Tag color={STATUS_CONFIG[change.status]?.color} key="status">
                  {STATUS_CONFIG[change.status]?.label}
                </Tag>,
              ]}
            >
              <List.Item.Meta
                title={
                  <Space>
                    <Text strong>{change.title}</Text>
                    <Tag color={TYPE_CONFIG[change.type]?.color}>
                      {TYPE_CONFIG[change.type]?.label}
                    </Tag>
                    <Tag color={RISK_CONFIG[change.risk]?.color}>
                      风险: {RISK_CONFIG[change.risk]?.label}
                    </Tag>
                  </Space>
                }
                description={
                  <Space size={16}>
                    <span>
                      <UserOutlined style={{ marginRight: 4 }} />
                      {change.applicant}
                    </span>
                    <span>
                      <ClockCircleOutlined style={{ marginRight: 4 }} />
                      {change.plannedStart} ~ {change.plannedEnd}
                    </span>
                    <span>
                      <SwapOutlined style={{ marginRight: 4 }} />
                      影响: {change.affectedAssets.join(', ')}
                    </span>
                  </Space>
                }
              />
            </List.Item>
          )}
        />
      </Modal>
    </div>
  );
};

export default ChangeCalendar;
