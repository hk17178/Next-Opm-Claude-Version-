/**
 * 会话管理页面
 * 展示系统中所有活跃会话，支持踢出单个会话或批量踢出其他设备
 *
 * 核心交互逻辑：
 * - 表格展示所有活跃会话（用户、IP、浏览器、登录时间、最后活跃时间）
 * - 当前会话高亮标记，不可被踢出
 * - 每行有"踢出"按钮（需二次确认弹窗）
 * - 顶部"踢出全部其他设备"按钮（需二次确认）
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Table, Button, Typography, Tag, Space, message, Popconfirm, Skeleton, Modal,
} from 'antd';
import {
  DisconnectOutlined, DesktopOutlined, ExclamationCircleOutlined,
} from '@ant-design/icons';
import { listSessions, revokeSession, revokeAll } from '../api/session';
import type { Session } from '../api/session';

const { Text } = Typography;

/**
 * 会话管理组件
 * - 顶部：页面标题 + "踢出全部其他设备"按钮
 * - 主体：会话列表表格（当前会话高亮）
 */
const SessionManagement: React.FC = () => {
  /** 会话列表数据 */
  const [sessions, setSessions] = useState<Session[]>([]);
  /** 页面加载状态 */
  const [loading, setLoading] = useState(true);

  /**
   * 加载会话列表
   */
  const fetchSessions = useCallback(async () => {
    setLoading(true);
    try {
      const list = await listSessions();
      setSessions(list);
    } catch {
      message.error('加载会话列表失败');
    } finally {
      setLoading(false);
    }
  }, []);

  /** 页面首次加载获取数据 */
  useEffect(() => {
    fetchSessions();
  }, [fetchSessions]);

  /**
   * 踢出指定会话
   * @param id - 会话 ID
   */
  const handleRevoke = useCallback(async (id: string) => {
    try {
      await revokeSession(id);
      message.success('已踢出该会话');
      fetchSessions();
    } catch {
      message.error('踢出会话失败');
    }
  }, [fetchSessions]);

  /**
   * 踢出所有其他会话
   * 通过 Modal.confirm 进行二次确认
   */
  const handleRevokeAll = useCallback(() => {
    Modal.confirm({
      title: '踢出全部其他设备',
      icon: <ExclamationCircleOutlined />,
      content: '确定要踢出除当前设备外的所有活跃会话吗？其他设备上的用户将被要求重新登录。',
      okText: '确定踢出',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await revokeAll();
          message.success('已踢出所有其他会话');
          fetchSessions();
        } catch {
          message.error('操作失败');
        }
      },
    });
  }, [fetchSessions]);

  /** 会话列表表格列定义 */
  const columns = [
    {
      title: '用户',
      dataIndex: 'username',
      key: 'username',
      width: 120,
      /** 渲染用户名，当前会话带"当前"标签 */
      render: (username: string, record: Session) => (
        <Space>
          <span>{username}</span>
          {record.isCurrent && <Tag color="blue">当前</Tag>}
        </Space>
      ),
    },
    {
      title: 'IP 地址',
      dataIndex: 'ip',
      key: 'ip',
      width: 140,
    },
    {
      title: '浏览器 / 设备',
      dataIndex: 'userAgent',
      key: 'userAgent',
      /** 渲染浏览器信息，带桌面图标 */
      render: (ua: string) => (
        <Space>
          <DesktopOutlined />
          <span>{ua}</span>
        </Space>
      ),
    },
    {
      title: '登录时间',
      dataIndex: 'loginAt',
      key: 'loginAt',
      width: 170,
      render: (val: string) => new Date(val).toLocaleString('zh-CN'),
    },
    {
      title: '最后活跃',
      dataIndex: 'lastActiveAt',
      key: 'lastActiveAt',
      width: 170,
      render: (val: string) => new Date(val).toLocaleString('zh-CN'),
    },
    {
      title: '操作',
      key: 'actions',
      width: 100,
      /** 渲染踢出按钮（当前会话不可踢出） */
      render: (_: unknown, record: Session) => (
        record.isCurrent ? (
          <Text type="secondary">--</Text>
        ) : (
          <Popconfirm
            title="确定要踢出此会话吗？"
            description="该会话对应的用户将被要求重新登录。"
            onConfirm={() => handleRevoke(record.id)}
            okText="确定"
            cancelText="取消"
            okButtonProps={{ danger: true }}
          >
            <Button type="link" danger icon={<DisconnectOutlined />} size="small">
              踢出
            </Button>
          </Popconfirm>
        )
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与全部踢出按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>会话管理</Text>
        <Button
          danger
          icon={<DisconnectOutlined />}
          onClick={handleRevokeAll}
          disabled={sessions.filter((s) => !s.isCurrent).length === 0}
        >
          踢出全部其他设备
        </Button>
      </div>

      {/* 会话列表 */}
      <Card style={{ borderRadius: 8 }}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 4 }} />
        ) : (
          <Table
            columns={columns}
            dataSource={sessions}
            rowKey="id"
            pagination={false}
            size="middle"
            locale={{ emptyText: '暂无活跃会话' }}
            /** 当前会话行高亮样式 */
            rowClassName={(record) => record.isCurrent ? 'current-session-row' : ''}
          />
        )}
      </Card>

      {/* 当前会话行高亮 CSS */}
      <style>{`
        .current-session-row {
          background-color: #E8F3FF !important;
        }
        .current-session-row:hover > td {
          background-color: #D6EAFF !important;
        }
      `}</style>
    </div>
  );
};

export default SessionManagement;
