/**
 * 通知配置页面（旧版） - 管理通知机器人和事件生命周期通知规则
 * 包含两个 Tab：通知机器人管理、事件生命周期通知配置
 * 注：新版渠道配置请使用 ChannelConfig 页面
 */
import React from 'react';
import { Card, Table, Button, Tag, Badge, Typography, Space, Tabs } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

/** 渠道类型对应的颜色映射 */
const CHANNEL_COLORS: Record<string, string> = {
  wecom_webhook: '#2E75B6',    // 企微机器人
  wecom_app: '#4C9AE6',       // 企微应用
  wecom_personal: '#3491FA',  // 企微个人
  sms: '#FF7D00',             // 短信
  voice: '#F53F3F',           // 语音
  email: '#86909C',           // 邮件
  webhook: '#722ED1',         // 自定义 Webhook
};

/**
 * 通知配置组件
 * - 通知机器人 Tab：展示已配置的通知机器人列表（名称、渠道、目标、模板、健康状态）
 * - 事件生命周期 Tab：展示各事件阶段的通知开关和渠道配置
 */
const NotifyConfig: React.FC = () => {
  const { t } = useTranslation('notify');

  /** 通知机器人表格列定义 */
  const botColumns = [
    { title: t('bots.column.name'), dataIndex: 'name', key: 'name' },
    {
      title: t('bots.column.channel'),
      dataIndex: 'channel',
      key: 'channel',
      width: 140,
      /** 渲染渠道类型标签 */
      render: (channel: string) => (
        <Tag color={CHANNEL_COLORS[channel] || '#86909C'}>{t(`channel.${channel}`)}</Tag>
      ),
    },
    { title: t('bots.column.target'), dataIndex: 'target', key: 'target' },
    { title: t('bots.column.template'), dataIndex: 'template', key: 'template', width: 120 },
    {
      title: t('bots.column.health'),
      dataIndex: 'healthy',
      key: 'healthy',
      width: 80,
      /** 渲染健康状态 Badge */
      render: (healthy: boolean) => (
        <Badge status={healthy ? 'success' : 'error'} text={healthy ? t('bots.healthy') : t('bots.unhealthy')} />
      ),
    },
    {
      title: t('bots.column.actions'),
      key: 'actions',
      width: 120,
      /** 渲染操作按钮：编辑、删除 */
      render: () => (
        <Space>
          <Button type="link" icon={<EditOutlined />} size="small" />
          <Button type="link" danger icon={<DeleteOutlined />} size="small" />
        </Space>
      ),
    },
  ];

  /** 事件生命周期通知表格列定义 */
  const lifecycleColumns = [
    { title: t('lifecycle.column.event'), dataIndex: 'event', key: 'event' },
    {
      title: t('lifecycle.column.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      /** 渲染启用/禁用状态 */
      render: (enabled: boolean) => (
        <Badge status={enabled ? 'success' : 'default'} text={enabled ? t('lifecycle.on') : t('lifecycle.off')} />
      ),
    },
    { title: t('lifecycle.column.channels'), dataIndex: 'channels', key: 'channels' },
  ];

  /** Tab 配置项 */
  const tabItems = [
    {
      key: 'bots',
      label: t('tab.bots'),
      children: (
        <div>
          {/* 添加机器人按钮 */}
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
            <Button type="primary" icon={<PlusOutlined />}>{t('bots.create')}</Button>
          </div>
          {/* 机器人列表表格 */}
          <Table
            columns={botColumns}
            dataSource={[]}
            locale={{ emptyText: t('bots.noData') }}
            size="middle"
          />
        </div>
      ),
    },
    {
      key: 'lifecycle',
      label: t('tab.lifecycle'),
      children: (
        /* 生命周期通知配置表格 */
        <Table
          columns={lifecycleColumns}
          dataSource={[]}
          locale={{ emptyText: t('lifecycle.noData') }}
          pagination={false}
          size="middle"
        />
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('title')}</Text>
      </div>

      <Card style={{ borderRadius: 8 }}>
        <Tabs items={tabItems} />
      </Card>
    </div>
  );
};

export default NotifyConfig;
