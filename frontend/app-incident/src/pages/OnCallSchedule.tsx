/**
 * 值班安排页面 - 展示值班日历、当前值班人员、排班表格、换班申请与值班统计
 * 对应路由：/incident/schedules
 *
 * 功能模块：
 * 1. 值班日历视图（antd Calendar 周视图 + 值班人员标记）
 * 2. 当前值班人员卡片（头像/姓名/角色/电话/在线状态）
 * 3. 值班排班表格（日期/主值班/副值班/主管）
 * 4. 换班申请弹窗
 * 5. 值班统计（本月值班次数/平均响应时间/处理事件数）
 */
import React, { useState } from 'react';
import {
  Card, Row, Col, Table, Tag, Typography, Space, Calendar, Badge, Avatar,
  Modal, Form, Input, Select, DatePicker, Button, Statistic, message, Tooltip,
} from 'antd';
import {
  UserOutlined, PhoneOutlined, SwapOutlined, CalendarOutlined,
  TeamOutlined, ClockCircleOutlined, CheckCircleOutlined,
  AlertOutlined, FieldTimeOutlined, SafetyCertificateOutlined,
  ManOutlined, WomanOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import dayjs, { type Dayjs } from 'dayjs';

const { Text, Title } = Typography;
const { TextArea } = Input;

/* ===================== 类型定义 ===================== */

/** 值班人员数据结构 */
interface OnCallMember {
  id: string;
  /** 姓名 */
  name: string;
  /** 头像颜色（Mock 用） */
  avatarColor: string;
  /** 角色：主值班/副值班/主管 */
  role: 'primary' | 'backup' | 'supervisor';
  /** 电话号码 */
  phone: string;
  /** 在线状态 */
  status: 'online' | 'busy' | 'offline';
  /** 今日处理事件数 */
  todayHandled: number;
  /** 平均响应时间（分钟） */
  avgResponseTime: number;
  /** 性别 */
  gender: 'male' | 'female';
}

/** 排班记录数据结构 */
interface ScheduleRecord {
  key: string;
  /** 日期 */
  date: string;
  /** 星期 */
  weekday: string;
  /** 主值班人员 */
  primary: string;
  /** 副值班人员 */
  backup: string;
  /** 主管 */
  supervisor: string;
  /** 是否为今天 */
  isToday?: boolean;
  /** 是否为节假日 */
  isHoliday?: boolean;
}

/** 换班申请数据结构 */
interface SwapRequest {
  /** 申请人 */
  applicant: string;
  /** 原日期 */
  originalDate: string;
  /** 目标日期 */
  targetDate: string;
  /** 换班对象 */
  targetPerson: string;
  /** 换班原因 */
  reason: string;
}

/** 日历值班标记数据 */
interface CalendarDayInfo {
  /** 主值班 */
  primary: string;
  /** 副值班 */
  backup: string;
  /** 当天事件数 */
  incidents: number;
  /** 是否为节假日 */
  isHoliday?: boolean;
}

/* ===================== Mock 数据 ===================== */

/** 当前值班团队成员 */
const CURRENT_TEAM: OnCallMember[] = [
  {
    id: 'U001', name: '张伟', avatarColor: '#3491FA', role: 'primary',
    phone: '138-0001-0001', status: 'online', todayHandled: 3,
    avgResponseTime: 2, gender: 'male',
  },
  {
    id: 'U002', name: '陈静', avatarColor: '#722ED1', role: 'backup',
    phone: '139-0002-0002', status: 'online', todayHandled: 1,
    avgResponseTime: 4, gender: 'female',
  },
  {
    id: 'U003', name: '李明', avatarColor: '#FF7D00', role: 'supervisor',
    phone: '137-0003-0003', status: 'busy', todayHandled: 0,
    avgResponseTime: 5, gender: 'male',
  },
];

/** 所有可选值班人员（用于换班选择） */
const ALL_MEMBERS = ['张伟', '陈静', '李明', '王芳', '刘洋', '赵磊', '孙悦', '周强'];

/** 排班表数据（当月） */
const generateScheduleData = (): ScheduleRecord[] => {
  /** 值班轮转人员组 */
  const primaryList = ['张伟', '王芳', '刘洋', '赵磊'];
  const backupList = ['陈静', '孙悦', '周强', '张伟'];
  const supervisorList = ['李明', '李明', '王芳', '陈静'];
  const weekdays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
  const today = dayjs();
  const records: ScheduleRecord[] = [];

  /** 生成当月每天的排班记录 */
  for (let i = 1; i <= today.daysInMonth(); i++) {
    const date = today.date(i);
    const dayOfWeek = date.day();
    const idx = (i - 1) % primaryList.length;
    records.push({
      key: `${i}`,
      date: date.format('YYYY-MM-DD'),
      weekday: weekdays[dayOfWeek],
      primary: primaryList[idx],
      backup: backupList[idx],
      supervisor: supervisorList[idx],
      isToday: date.isSame(today, 'day'),
      isHoliday: dayOfWeek === 0 || dayOfWeek === 6,
    });
  }
  return records;
};

const SCHEDULE_DATA = generateScheduleData();

/** 日历值班标记映射（当月） */
const generateCalendarMap = (): Record<string, CalendarDayInfo> => {
  const map: Record<string, CalendarDayInfo> = {};
  SCHEDULE_DATA.forEach((record) => {
    map[record.date] = {
      primary: record.primary,
      backup: record.backup,
      incidents: Math.floor(Math.random() * 5),
      isHoliday: record.isHoliday,
    };
  });
  return map;
};

const CALENDAR_MAP = generateCalendarMap();

/** 在线状态颜色映射 */
const STATUS_CONFIG: Record<string, { color: string; text: string }> = {
  online: { color: '#00B42A', text: '在线' },
  busy: { color: '#FF7D00', text: '忙碌' },
  offline: { color: '#C9CDD4', text: '离线' },
};

/** 角色标签颜色映射 */
const ROLE_CONFIG: Record<string, { color: string; label: string }> = {
  primary: { color: '#3491FA', label: '主值班' },
  backup: { color: '#722ED1', label: '副值班' },
  supervisor: { color: '#FF7D00', label: '主管' },
};

/* ===================== 值班人员卡片子组件 ===================== */

/**
 * OnCallMemberCard 值班人员卡片
 * 正面显示姓名/角色/状态，hover 翻转显示联系方式/响应时间/处理数
 */
const OnCallMemberCard: React.FC<{ member: OnCallMember }> = ({ member }) => {
  const [flipped, setFlipped] = useState(false);
  const statusInfo = STATUS_CONFIG[member.status];
  const roleInfo = ROLE_CONFIG[member.role];

  return (
    <div
      style={{ perspective: 800, cursor: 'pointer', height: 180 }}
      onMouseEnter={() => setFlipped(true)}
      onMouseLeave={() => setFlipped(false)}
    >
      <div
        style={{
          position: 'relative', width: '100%', height: '100%',
          transition: 'transform 0.5s', transformStyle: 'preserve-3d',
          transform: flipped ? 'rotateY(180deg)' : 'rotateY(0deg)',
        }}
      >
        {/* ---- 正面：基本信息 ---- */}
        <Card
          style={{
            position: 'absolute', width: '100%', height: '100%',
            backfaceVisibility: 'hidden', borderRadius: 8,
            boxShadow: '0 2px 8px rgba(0,0,0,0.06)',
            borderTop: `3px solid ${roleInfo.color}`,
          }}
          bodyStyle={{ padding: '20px 16px', textAlign: 'center' }}
        >
          {/* 头像 + 在线状态指示灯 */}
          <div style={{ position: 'relative', display: 'inline-block', marginBottom: 12 }}>
            <Avatar size={48} style={{ backgroundColor: member.avatarColor }} icon={<UserOutlined />} />
            <div style={{
              position: 'absolute', bottom: 2, right: 2,
              width: 12, height: 12, borderRadius: '50%',
              backgroundColor: statusInfo.color, border: '2px solid #fff',
            }} />
          </div>
          <div>
            <Text strong style={{ fontSize: 16 }}>{member.name}</Text>
            {member.gender === 'male' ? (
              <ManOutlined style={{ marginLeft: 4, color: '#3491FA', fontSize: 12 }} />
            ) : (
              <WomanOutlined style={{ marginLeft: 4, color: '#F53F3F', fontSize: 12 }} />
            )}
          </div>
          <Tag color={roleInfo.color} style={{ borderRadius: 4, marginTop: 8, border: 'none' }}>
            {roleInfo.label}
          </Tag>
          <div style={{ marginTop: 8 }}>
            <Badge color={statusInfo.color} text={<Text style={{ fontSize: 12, color: '#86909C' }}>{statusInfo.text}</Text>} />
          </div>
        </Card>

        {/* ---- 背面：详细数据 ---- */}
        <Card
          style={{
            position: 'absolute', width: '100%', height: '100%',
            backfaceVisibility: 'hidden', borderRadius: 8,
            transform: 'rotateY(180deg)',
            boxShadow: '0 2px 8px rgba(0,0,0,0.06)',
            background: 'linear-gradient(135deg, #f5f5f5 0%, #ffffff 100%)',
          }}
          bodyStyle={{ padding: '16px' }}
        >
          <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 12 }}>{member.name} 详情</Text>
          <div style={{ marginBottom: 8 }}>
            <PhoneOutlined style={{ color: '#3491FA', marginRight: 6 }} />
            <Text style={{ fontSize: 12 }}>{member.phone}</Text>
          </div>
          <div style={{ marginBottom: 8 }}>
            <ClockCircleOutlined style={{ color: '#FF7D00', marginRight: 6 }} />
            <Text style={{ fontSize: 12 }}>平均响应: {member.avgResponseTime} min</Text>
          </div>
          <div style={{ marginBottom: 8 }}>
            <AlertOutlined style={{ color: '#F53F3F', marginRight: 6 }} />
            <Text style={{ fontSize: 12 }}>今日处理: {member.todayHandled} 件</Text>
          </div>
          <div>
            <SafetyCertificateOutlined style={{ color: '#00B42A', marginRight: 6 }} />
            <Text style={{ fontSize: 12 }}>状态: {statusInfo.text}</Text>
          </div>
        </Card>
      </div>
    </div>
  );
};

/* ===================== 主组件 ===================== */

/**
 * OnCallSchedule 值班安排主组件
 * 布局：
 * - 第一行左：值班日历视图；右：当前值班人员卡片（3 张）
 * - 第二行左：值班统计；右：换班申请按钮
 * - 第三行：排班表格
 * - 换班申请弹窗
 */
const OnCallSchedule: React.FC = () => {
  const { t } = useTranslation('incident');

  /** 换班申请弹窗可见性 */
  const [swapModalOpen, setSwapModalOpen] = useState(false);
  /** 换班申请提交加载状态 */
  const [swapLoading, setSwapLoading] = useState(false);
  /** 换班申请表单实例 */
  const [swapForm] = Form.useForm<SwapRequest>();
  /** 日历当前选中月份 */
  const [calendarValue, setCalendarValue] = useState<Dayjs>(dayjs());

  /**
   * 处理换班申请提交
   * 校验表单 → 模拟提交 → 成功后关闭弹窗
   */
  const handleSwapSubmit = async () => {
    try {
      await swapForm.validateFields();
      setSwapLoading(true);
      // 模拟 API 请求
      await new Promise((resolve) => setTimeout(resolve, 800));
      message.success(t('schedule.swapSuccess'));
      setSwapModalOpen(false);
      swapForm.resetFields();
    } catch {
      // 表单校验失败
    } finally {
      setSwapLoading(false);
    }
  };

  /**
   * 日历单元格渲染函数
   * 在日期下方显示主值班人员标记与事件数角标
   */
  const dateCellRender = (date: Dayjs) => {
    const key = date.format('YYYY-MM-DD');
    const info = CALENDAR_MAP[key];
    if (!info) return null;

    return (
      <div style={{ fontSize: 11 }}>
        {/* 主值班人员标记 */}
        <div>
          <Badge color="#3491FA" text={<Text style={{ fontSize: 11 }}>{info.primary}</Text>} />
        </div>
        {/* 事件数标记（仅有事件时显示） */}
        {info.incidents > 0 && (
          <div style={{ marginTop: 2 }}>
            <Tag color="red" style={{ fontSize: 10, padding: '0 4px', lineHeight: '16px', borderRadius: 4, border: 'none' }}>
              {info.incidents} 件事件
            </Tag>
          </div>
        )}
        {/* 节假日标记 */}
        {info.isHoliday && (
          <Tag color="orange" style={{ fontSize: 10, padding: '0 4px', lineHeight: '16px', borderRadius: 4, marginTop: 2, border: 'none' }}>
            休
          </Tag>
        )}
      </div>
    );
  };

  /** 排班表格列定义 */
  const scheduleColumns = [
    {
      title: t('schedule.table.date'),
      dataIndex: 'date',
      key: 'date',
      width: 120,
      /** 今天日期高亮显示 */
      render: (text: string, record: ScheduleRecord) => (
        <Space>
          <Text strong={record.isToday} style={{ color: record.isToday ? '#3491FA' : undefined }}>
            {text}
          </Text>
          {record.isToday && <Tag color="blue" style={{ borderRadius: 4, fontSize: 11, border: 'none' }}>今天</Tag>}
          {record.isHoliday && <Tag color="orange" style={{ borderRadius: 4, fontSize: 11, border: 'none' }}>休</Tag>}
        </Space>
      ),
    },
    {
      title: t('schedule.table.weekday'),
      dataIndex: 'weekday',
      key: 'weekday',
      width: 80,
      render: (text: string, record: ScheduleRecord) => (
        <Text style={{ color: record.isHoliday ? '#FF7D00' : '#4E5969' }}>{text}</Text>
      ),
    },
    {
      title: t('schedule.table.primary'),
      dataIndex: 'primary',
      key: 'primary',
      width: 120,
      /** 主值班人员带蓝色标签 */
      render: (text: string) => (
        <Tag color="#3491FA" style={{ borderRadius: 4, border: 'none' }}>
          <UserOutlined style={{ marginRight: 4 }} />{text}
        </Tag>
      ),
    },
    {
      title: t('schedule.table.backup'),
      dataIndex: 'backup',
      key: 'backup',
      width: 120,
      /** 副值班人员带紫色标签 */
      render: (text: string) => (
        <Tag color="#722ED1" style={{ borderRadius: 4, border: 'none' }}>
          <UserOutlined style={{ marginRight: 4 }} />{text}
        </Tag>
      ),
    },
    {
      title: t('schedule.table.supervisor'),
      dataIndex: 'supervisor',
      key: 'supervisor',
      width: 120,
      /** 主管带橙色标签 */
      render: (text: string) => (
        <Tag color="#FF7D00" style={{ borderRadius: 4, border: 'none' }}>
          <UserOutlined style={{ marginRight: 4 }} />{text}
        </Tag>
      ),
    },
  ];

  /** 值班统计 Mock 数据 */
  const stats = {
    /** 本月总值班次数 */
    totalShifts: 31,
    /** 平均响应时间（分钟） */
    avgResponseTime: 3.2,
    /** 本月处理事件数 */
    handledIncidents: 47,
    /** 换班申请数 */
    swapRequests: 3,
  };

  return (
    <div>
      {/* ===== 页面标题与操作按钮 ===== */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Space>
          <CalendarOutlined style={{ fontSize: 22, color: '#3491FA' }} />
          <Text strong style={{ fontSize: 20 }}>{t('schedule.title')}</Text>
        </Space>
        <Button type="primary" icon={<SwapOutlined />} onClick={() => setSwapModalOpen(true)}>
          {t('schedule.requestSwap')}
        </Button>
      </div>

      {/* ===== 值班统计卡片行 ===== */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card bordered style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }} bodyStyle={{ padding: '16px 20px' }}>
            <Statistic
              title={t('schedule.stats.totalShifts')}
              value={stats.totalShifts}
              suffix={t('schedule.stats.shiftUnit')}
              prefix={<CalendarOutlined style={{ color: '#3491FA' }} />}
              valueStyle={{ color: '#3491FA' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card bordered style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }} bodyStyle={{ padding: '16px 20px' }}>
            <Statistic
              title={t('schedule.stats.avgResponse')}
              value={stats.avgResponseTime}
              suffix="min"
              prefix={<ClockCircleOutlined style={{ color: '#00B42A' }} />}
              valueStyle={{ color: '#00B42A' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card bordered style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }} bodyStyle={{ padding: '16px 20px' }}>
            <Statistic
              title={t('schedule.stats.handled')}
              value={stats.handledIncidents}
              suffix={t('schedule.stats.incidentUnit')}
              prefix={<AlertOutlined style={{ color: '#FF7D00' }} />}
              valueStyle={{ color: '#FF7D00' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card bordered style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }} bodyStyle={{ padding: '16px 20px' }}>
            <Statistic
              title={t('schedule.stats.swapRequests')}
              value={stats.swapRequests}
              suffix={t('schedule.stats.requestUnit')}
              prefix={<SwapOutlined style={{ color: '#722ED1' }} />}
              valueStyle={{ color: '#722ED1' }}
            />
          </Card>
        </Col>
      </Row>

      {/* ===== 主体区域：日历 + 当前值班人员 ===== */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {/* 左侧：值班日历视图 */}
        <Col span={16}>
          <Card
            title={
              <Space>
                <CalendarOutlined style={{ color: '#3491FA' }} />
                <span>{t('schedule.calendar')}</span>
              </Space>
            }
            bordered
            style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
            bodyStyle={{ padding: '8px 16px' }}
          >
            <Calendar
              value={calendarValue}
              onChange={setCalendarValue}
              fullscreen={false}
              cellRender={(date) => dateCellRender(date as Dayjs)}
            />
          </Card>
        </Col>

        {/* 右侧：当前值班人员卡片 */}
        <Col span={8}>
          <Card
            title={
              <Space>
                <TeamOutlined style={{ color: '#722ED1' }} />
                <span>{t('schedule.currentTeam')}</span>
              </Space>
            }
            bordered
            style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
            bodyStyle={{ padding: 12 }}
          >
            <Space direction="vertical" style={{ width: '100%' }} size={12}>
              {CURRENT_TEAM.map((member) => (
                <OnCallMemberCard key={member.id} member={member} />
              ))}
            </Space>
          </Card>
        </Col>
      </Row>

      {/* ===== 排班表格 ===== */}
      <Card
        title={
          <Space>
            <FieldTimeOutlined style={{ color: '#FF7D00' }} />
            <span>{t('schedule.scheduleTable')}</span>
          </Space>
        }
        bordered
        style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
        bodyStyle={{ padding: '8px 0' }}
      >
        <Table
          columns={scheduleColumns}
          dataSource={SCHEDULE_DATA}
          pagination={{ pageSize: 10, showSizeChanger: true, showQuickJumper: true }}
          size="middle"
          rowKey="key"
          rowClassName={(record) => record.isToday ? 'on-call-today-row' : ''}
        />
      </Card>

      {/* ===== 换班申请弹窗 ===== */}
      <Modal
        title={
          <Space>
            <SwapOutlined style={{ color: '#3491FA' }} />
            <span>{t('schedule.swapTitle')}</span>
          </Space>
        }
        open={swapModalOpen}
        onCancel={() => { setSwapModalOpen(false); swapForm.resetFields(); }}
        onOk={handleSwapSubmit}
        confirmLoading={swapLoading}
        okText={t('schedule.swapSubmit')}
        cancelText={t('schedule.swapCancel')}
        width={520}
        destroyOnClose
      >
        <Form form={swapForm} layout="vertical" style={{ marginTop: 16 }}>
          {/* 申请人 */}
          <Form.Item
            name="applicant"
            label={t('schedule.swapForm.applicant')}
            rules={[{ required: true, message: t('schedule.swapForm.applicantRequired') }]}
          >
            <Select
              placeholder={t('schedule.swapForm.applicantPlaceholder')}
              options={ALL_MEMBERS.map((m) => ({ value: m, label: m }))}
            />
          </Form.Item>
          {/* 原值班日期 */}
          <Form.Item
            name="originalDate"
            label={t('schedule.swapForm.originalDate')}
            rules={[{ required: true, message: t('schedule.swapForm.originalDateRequired') }]}
          >
            <DatePicker style={{ width: '100%' }} placeholder={t('schedule.swapForm.selectDate')} />
          </Form.Item>
          {/* 换班对象 */}
          <Form.Item
            name="targetPerson"
            label={t('schedule.swapForm.targetPerson')}
            rules={[{ required: true, message: t('schedule.swapForm.targetPersonRequired') }]}
          >
            <Select
              placeholder={t('schedule.swapForm.targetPersonPlaceholder')}
              options={ALL_MEMBERS.map((m) => ({ value: m, label: m }))}
            />
          </Form.Item>
          {/* 目标日期 */}
          <Form.Item
            name="targetDate"
            label={t('schedule.swapForm.targetDate')}
            rules={[{ required: true, message: t('schedule.swapForm.targetDateRequired') }]}
          >
            <DatePicker style={{ width: '100%' }} placeholder={t('schedule.swapForm.selectDate')} />
          </Form.Item>
          {/* 换班原因 */}
          <Form.Item
            name="reason"
            label={t('schedule.swapForm.reason')}
            rules={[{ required: true, message: t('schedule.swapForm.reasonRequired') }]}
          >
            <TextArea rows={3} placeholder={t('schedule.swapForm.reasonPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 内联样式：今天行高亮 */}
      <style>{`
        .on-call-today-row td {
          background-color: #E8F3FF !important;
        }
      `}</style>
    </div>
  );
};

export default OnCallSchedule;
