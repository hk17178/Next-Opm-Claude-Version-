/**
 * 个人通知偏好设置页面
 *
 * 功能：
 * - 各告警级别（P0~P4）的通知接收开关
 * - 静默时段设置（开始时间 ~ 结束时间）
 * - 偏好通知渠道选择（企微/邮件/短信，多选）
 * - 报告推送频率选择（实时/日汇总/周汇总，单选）
 * - 保存按钮提交到后端
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Switch, TimePicker, Checkbox, Radio, Button, Space, Typography, Row, Col,
  Divider, Spin, message, Tag,
} from 'antd';
import {
  BellOutlined, ClockCircleOutlined, SendOutlined, BarChartOutlined,
  SaveOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import {
  getPreferences,
  updatePreferences,
  type NotificationPreferences as PreferencesData,
} from '../api/preferences';

const { Title, Text } = Typography;

/** 告警级别列表及对应颜色 */
const SEVERITY_LEVELS = [
  { key: 'P0' as const, label: 'P0 - 致命', color: '#F53F3F', desc: '生产环境全面中断，需立即处理' },
  { key: 'P1' as const, label: 'P1 - 严重', color: '#FF7D00', desc: '核心功能受损，需紧急处理' },
  { key: 'P2' as const, label: 'P2 - 中等', color: '#3491FA', desc: '部分功能受影响，需尽快处理' },
  { key: 'P3' as const, label: 'P3 - 低', color: '#86909C', desc: '轻微问题，计划内处理' },
  { key: 'P4' as const, label: 'P4 - 信息', color: '#C9CDD4', desc: '提示信息，无需立即处理' },
];

/** 通知渠道选项 */
const CHANNEL_OPTIONS = [
  { value: 'wecom' as const, label: '企业微信' },
  { value: 'email' as const, label: '邮件' },
  { value: 'sms' as const, label: '短信' },
];

/** 报告频率选项 */
const FREQUENCY_OPTIONS = [
  { value: 'realtime' as const, label: '实时推送' },
  { value: 'daily' as const, label: '日汇总' },
  { value: 'weekly' as const, label: '周汇总' },
];

/** 默认偏好值 */
const DEFAULT_PREFERENCES: PreferencesData = {
  levelSettings: { P0: true, P1: true, P2: true, P3: true, P4: false },
  silentPeriod: { enabled: false, startTime: '22:00', endTime: '08:00' },
  channels: ['wecom', 'email'],
  reportFrequency: 'daily',
};

/**
 * 通知偏好设置页面组件
 */
const NotificationPreferences: React.FC = () => {
  /** 加载状态 */
  const [loading, setLoading] = useState(true);
  /** 保存中状态 */
  const [saving, setSaving] = useState(false);
  /** 偏好数据 */
  const [preferences, setPreferences] = useState<PreferencesData>(DEFAULT_PREFERENCES);

  /**
   * 从后端加载偏好数据
   * 失败时使用默认值
   */
  const loadPreferences = useCallback(async () => {
    setLoading(true);
    try {
      const data = await getPreferences();
      setPreferences(data);
    } catch {
      // API 尚未就绪，使用默认值
      setPreferences(DEFAULT_PREFERENCES);
    } finally {
      setLoading(false);
    }
  }, []);

  /** 组件挂载时加载偏好数据 */
  useEffect(() => {
    loadPreferences();
  }, [loadPreferences]);

  /**
   * 切换指定告警级别的通知开关
   * @param level - 告警级别
   * @param checked - 是否开启
   */
  const handleLevelChange = useCallback((level: keyof PreferencesData['levelSettings'], checked: boolean) => {
    setPreferences((prev) => ({
      ...prev,
      levelSettings: { ...prev.levelSettings, [level]: checked },
    }));
  }, []);

  /**
   * 切换静默时段启用状态
   */
  const handleSilentToggle = useCallback((checked: boolean) => {
    setPreferences((prev) => ({
      ...prev,
      silentPeriod: { ...prev.silentPeriod, enabled: checked },
    }));
  }, []);

  /**
   * 更新静默时段的开始/结束时间
   * @param field - 'startTime' 或 'endTime'
   * @param time - 时间值
   */
  const handleSilentTimeChange = useCallback((field: 'startTime' | 'endTime', time: dayjs.Dayjs | null) => {
    if (!time) return;
    setPreferences((prev) => ({
      ...prev,
      silentPeriod: { ...prev.silentPeriod, [field]: time.format('HH:mm') },
    }));
  }, []);

  /**
   * 更新通知渠道选择
   */
  const handleChannelsChange = useCallback((channels: PreferencesData['channels']) => {
    setPreferences((prev) => ({ ...prev, channels }));
  }, []);

  /**
   * 更新报告推送频率
   */
  const handleFrequencyChange = useCallback((frequency: PreferencesData['reportFrequency']) => {
    setPreferences((prev) => ({ ...prev, reportFrequency: frequency }));
  }, []);

  /**
   * 保存偏好设置到后端
   */
  const handleSave = useCallback(async () => {
    setSaving(true);
    try {
      await updatePreferences(preferences);
      message.success('通知偏好保存成功');
    } catch {
      message.error('保存失败，请稍后重试');
    } finally {
      setSaving(false);
    }
  }, [preferences]);

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', padding: 80 }}>
        <Spin size="large" tip="加载通知偏好设置..." />
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 800 }}>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <Title level={4} style={{ margin: 0 }}>
          <BellOutlined style={{ marginRight: 8 }} />
          通知偏好设置
        </Title>
        <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={handleSave}>
          保存设置
        </Button>
      </div>

      {/* 告警级别通知开关 */}
      <Card
        title={<><BellOutlined style={{ marginRight: 8 }} />告警级别通知</>}
        style={{ marginBottom: 16, borderRadius: 8 }}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          选择需要接收通知的告警级别，关闭的级别将不会推送通知
        </Text>
        {SEVERITY_LEVELS.map((level) => (
          <div
            key={level.key}
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '10px 0',
              borderBottom: '1px solid #f0f0f0',
            }}
          >
            <Space>
              <Tag color={level.color} style={{ minWidth: 36, textAlign: 'center', border: 'none', fontWeight: 600 }}>
                {level.key}
              </Tag>
              <div>
                <div style={{ fontWeight: 500 }}>{level.label}</div>
                <Text type="secondary" style={{ fontSize: 12 }}>{level.desc}</Text>
              </div>
            </Space>
            <Switch
              checked={preferences.levelSettings[level.key]}
              onChange={(checked) => handleLevelChange(level.key, checked)}
            />
          </div>
        ))}
      </Card>

      {/* 静默时段设置 */}
      <Card
        title={<><ClockCircleOutlined style={{ marginRight: 8 }} />静默时段</>}
        style={{ marginBottom: 16, borderRadius: 8 }}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          在静默时段内，系统将暂停推送非 P0 级别的通知
        </Text>
        <Row align="middle" gutter={16}>
          <Col>
            <Space>
              <Text>启用静默时段</Text>
              <Switch
                checked={preferences.silentPeriod.enabled}
                onChange={handleSilentToggle}
              />
            </Space>
          </Col>
        </Row>
        {preferences.silentPeriod.enabled && (
          <Row style={{ marginTop: 16 }} gutter={16} align="middle">
            <Col>
              <Space>
                <Text>开始时间</Text>
                <TimePicker
                  format="HH:mm"
                  value={dayjs(preferences.silentPeriod.startTime, 'HH:mm')}
                  onChange={(time) => handleSilentTimeChange('startTime', time)}
                />
              </Space>
            </Col>
            <Col>
              <Text style={{ fontSize: 16 }}>~</Text>
            </Col>
            <Col>
              <Space>
                <Text>结束时间</Text>
                <TimePicker
                  format="HH:mm"
                  value={dayjs(preferences.silentPeriod.endTime, 'HH:mm')}
                  onChange={(time) => handleSilentTimeChange('endTime', time)}
                />
              </Space>
            </Col>
          </Row>
        )}
      </Card>

      {/* 通知渠道选择 */}
      <Card
        title={<><SendOutlined style={{ marginRight: 8 }} />通知渠道</>}
        style={{ marginBottom: 16, borderRadius: 8 }}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          选择接收通知的渠道，可多选
        </Text>
        <Checkbox.Group
          value={preferences.channels}
          onChange={(values) => handleChannelsChange(values as PreferencesData['channels'])}
        >
          <Space direction="vertical">
            {CHANNEL_OPTIONS.map((opt) => (
              <Checkbox key={opt.value} value={opt.value} style={{ fontSize: 14 }}>
                {opt.label}
              </Checkbox>
            ))}
          </Space>
        </Checkbox.Group>
      </Card>

      {/* 报告推送频率 */}
      <Card
        title={<><BarChartOutlined style={{ marginRight: 8 }} />报告推送频率</>}
        style={{ marginBottom: 16, borderRadius: 8 }}
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          选择接收运维报告的推送频率
        </Text>
        <Radio.Group
          value={preferences.reportFrequency}
          onChange={(e) => handleFrequencyChange(e.target.value)}
        >
          <Space direction="vertical">
            {FREQUENCY_OPTIONS.map((opt) => (
              <Radio key={opt.value} value={opt.value} style={{ fontSize: 14 }}>
                {opt.label}
              </Radio>
            ))}
          </Space>
        </Radio.Group>
      </Card>

      <Divider />

      {/* 底部保存按钮 */}
      <div style={{ textAlign: 'right' }}>
        <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={handleSave} size="large">
          保存通知偏好
        </Button>
      </div>
    </div>
  );
};

export default NotificationPreferences;
