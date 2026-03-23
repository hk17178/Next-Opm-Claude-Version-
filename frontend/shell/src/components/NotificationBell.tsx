/**
 * NotificationBell - 站内通知铃铛组件
 *
 * 用途：在 Header 右上角显示通知入口
 * 功能：
 * - 铃铛图标 + 未读数角标（Badge）
 * - 点击展开下拉面板，显示最近 10 条通知
 * - 通知按类型用不同颜色标签区分：告警/事件/系统/审批
 * - 支持已读/未读状态切换
 * - "全部标为已读" 按钮
 *
 * 当前使用 Mock 数据，后期接入真实 API 替换
 *
 * 使用方式：
 * ```tsx
 * <NotificationBell />
 * ```
 */
import React, { useState, useMemo } from 'react';
import { Badge, Dropdown, Button, Tag, Typography, Space, Divider } from 'antd';
import { BellOutlined, CheckOutlined } from '@ant-design/icons';

const { Text } = Typography;

/** 通知类型枚举 */
type NotificationType = 'alert' | 'incident' | 'system' | 'approval';

/** 通知数据结构 */
interface NotificationItem {
  /** 通知唯一标识 */
  id: string;
  /** 通知类型：告警 / 事件 / 系统 / 审批 */
  type: NotificationType;
  /** 通知标题 */
  title: string;
  /** 通知内容摘要 */
  content: string;
  /** 通知时间 */
  time: string;
  /** 是否已读 */
  read: boolean;
}

/** 通知类型对应的颜色和中文标签 */
const NOTIFICATION_TYPE_MAP: Record<NotificationType, { color: string; label: string }> = {
  alert: { color: '#F53F3F', label: '告警' },
  incident: { color: '#FF7D00', label: '事件' },
  system: { color: '#165DFF', label: '系统' },
  approval: { color: '#722ED1', label: '审批' },
};

/** Mock 通知数据（后期替换为真实 API 调用） */
const MOCK_NOTIFICATIONS: NotificationItem[] = [
  {
    id: '1',
    type: 'alert',
    title: 'P0 告警触发',
    content: '生产环境 API 网关响应时间超过 5 秒阈值',
    time: '2 分钟前',
    read: false,
  },
  {
    id: '2',
    type: 'incident',
    title: '新事件创建',
    content: 'INC-2024-0328：支付服务异常，已分派给张三处理',
    time: '10 分钟前',
    read: false,
  },
  {
    id: '3',
    type: 'system',
    title: '系统维护通知',
    content: '今晚 02:00-04:00 将进行数据库升级维护',
    time: '30 分钟前',
    read: false,
  },
  {
    id: '4',
    type: 'approval',
    title: '审批待处理',
    content: '李四申请查看生产环境日志权限，等待审批',
    time: '1 小时前',
    read: true,
  },
  {
    id: '5',
    type: 'alert',
    title: 'P2 告警触发',
    content: '磁盘使用率超过 85% - web-server-03',
    time: '1 小时前',
    read: true,
  },
  {
    id: '6',
    type: 'incident',
    title: '事件状态变更',
    content: 'INC-2024-0325 已从 "处理中" 变更为 "待复盘"',
    time: '2 小时前',
    read: true,
  },
  {
    id: '7',
    type: 'system',
    title: '版本更新',
    content: 'OpsNexus v2.1.0 已发布，新增日志分析功能',
    time: '3 小时前',
    read: true,
  },
  {
    id: '8',
    type: 'alert',
    title: 'P1 告警恢复',
    content: '数据库连接池告警已自动恢复',
    time: '4 小时前',
    read: true,
  },
  {
    id: '9',
    type: 'approval',
    title: '审批已通过',
    content: '王五的 RBAC 角色变更已审批通过',
    time: '5 小时前',
    read: true,
  },
  {
    id: '10',
    type: 'system',
    title: '安全提醒',
    content: '检测到 3 次异常登录尝试，已自动锁定账户',
    time: '6 小时前',
    read: true,
  },
];

export const NotificationBell: React.FC = () => {
  const [open, setOpen] = useState(false);
  const [notifications, setNotifications] = useState<NotificationItem[]>(MOCK_NOTIFICATIONS);

  /** 计算未读通知数量 */
  const unreadCount = useMemo(
    () => notifications.filter((n) => !n.read).length,
    [notifications],
  );

  /** 将单条通知标记为已读 */
  const markAsRead = (id: string) => {
    setNotifications((prev) =>
      prev.map((n) => (n.id === id ? { ...n, read: true } : n)),
    );
  };

  /** 全部标为已读 */
  const markAllAsRead = () => {
    setNotifications((prev) => prev.map((n) => ({ ...n, read: true })));
  };

  /** 下拉面板内容 */
  const dropdownContent = (
    // stopPropagation 防止面板内 click/mousedown 冒泡到 document。
    // antd rc-trigger 对外部点击的检测同时使用 click 和 mousedown 事件，
    // 两者都需要阻止冒泡才能避免面板内操作被误判为"点击外部"而关闭面板。
    <div
      onClick={(e) => e.stopPropagation()}
      onMouseDown={(e) => e.stopPropagation()}
      style={{
        width: 380,
        maxHeight: 480,
        overflow: 'auto',
        background: 'var(--card-bg, #FFFFFF)',
        borderRadius: 8,
        boxShadow: '0 6px 16px rgba(0, 0, 0, 0.12)',
      }}
    >
      {/* 面板头部 */}
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        padding: '12px 16px',
        borderBottom: '1px solid var(--border-primary, #E5E6EB)',
      }}>
        <Text strong style={{ fontSize: 15 }}>通知中心</Text>
        {unreadCount > 0 && (
          <Button
            type="link"
            size="small"
            icon={<CheckOutlined />}
            onClick={markAllAsRead}
          >
            全部已读
          </Button>
        )}
      </div>

      {/* 通知列表 */}
      {notifications.length === 0 ? (
        <div style={{ padding: 40, textAlign: 'center', color: '#86909C' }}>
          暂无通知
        </div>
      ) : (
        notifications.map((item) => {
          const typeInfo = NOTIFICATION_TYPE_MAP[item.type];
          return (
            <div
              key={item.id}
              onClick={() => markAsRead(item.id)}
              style={{
                padding: '10px 16px',
                cursor: 'pointer',
                borderBottom: '1px solid var(--border-secondary, #F2F3F5)',
                backgroundColor: item.read ? 'transparent' : 'rgba(22, 93, 255, 0.04)',
                transition: 'background-color 0.2s',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = 'var(--bg-hover, #F2F3F5)';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = item.read
                  ? 'transparent'
                  : 'rgba(22, 93, 255, 0.04)';
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                <Tag
                  color={typeInfo.color}
                  style={{ fontSize: 11, lineHeight: '18px', margin: 0, borderRadius: 3 }}
                >
                  {typeInfo.label}
                </Tag>
                <Text strong style={{ fontSize: 13, flex: 1 }}>
                  {item.title}
                </Text>
                {/* 未读指示点 */}
                {!item.read && (
                  <div style={{
                    width: 6,
                    height: 6,
                    borderRadius: '50%',
                    backgroundColor: '#F53F3F',
                    flexShrink: 0,
                  }} />
                )}
              </div>
              <Text
                style={{ fontSize: 12, color: 'var(--text-tertiary, #86909C)', display: 'block' }}
                ellipsis
              >
                {item.content}
              </Text>
              <Text style={{ fontSize: 11, color: 'var(--text-disabled, #C9CDD4)', marginTop: 2, display: 'block' }}>
                {item.time}
              </Text>
            </div>
          );
        })
      )}
    </div>
  );

  return (
    <Dropdown
      dropdownRender={() => dropdownContent}
      trigger={['click']}
      placement="bottomRight"
      open={open}
      onOpenChange={setOpen}
    >
      <div style={{ cursor: 'pointer', display: 'flex', alignItems: 'center' }}>
        <Badge count={unreadCount} size="small" offset={[-2, 2]}>
          <BellOutlined style={{ fontSize: 18, color: 'var(--text-secondary, #4E5969)' }} />
        </Badge>
      </div>
    </Dropdown>
  );
};

export default NotificationBell;
