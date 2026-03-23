/**
 * EmptyState - 通用空状态引导组件
 *
 * 用途：当列表、表格等数据为空时，展示友好的空状态提示
 * 包含可选的图标/插图、标题文案、描述文案和操作按钮
 *
 * 使用方式：
 * ```tsx
 * <EmptyState
 *   title="暂无告警"
 *   description="系统运行正常"
 *   icon={<CheckCircleOutlined />}
 *   action={<Button type="primary">刷新</Button>}
 * />
 * ```
 *
 * Props：
 * @param icon - 自定义图标或插图（ReactNode）
 * @param title - 标题文案（必填）
 * @param description - 描述信息（可选）
 * @param action - 操作按钮区域（可选）
 */
import React, { type ReactNode } from 'react';
import { Empty, Typography } from 'antd';

const { Text, Title } = Typography;

export interface EmptyStateProps {
  /** 自定义图标或插图 */
  icon?: ReactNode;
  /** 标题文案 */
  title: string;
  /** 描述信息 */
  description?: string;
  /** 操作按钮区域 */
  action?: ReactNode;
}

export const EmptyState: React.FC<EmptyStateProps> = ({
  icon,
  title,
  description,
  action,
}) => {
  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      padding: '60px 24px',
      textAlign: 'center',
    }}>
      {/* 图标/插图区域 */}
      {icon ? (
        <div style={{ fontSize: 64, color: '#C9CDD4', marginBottom: 24 }}>
          {icon}
        </div>
      ) : (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={false}
          style={{ marginBottom: 16 }}
        />
      )}

      {/* 标题 */}
      <Title level={5} style={{ color: 'var(--text-primary, #1D2129)', marginBottom: 8 }}>
        {title}
      </Title>

      {/* 描述 */}
      {description && (
        <Text style={{ color: 'var(--text-tertiary, #86909C)', marginBottom: 24, maxWidth: 400 }}>
          {description}
        </Text>
      )}

      {/* 操作按钮 */}
      {action && (
        <div style={{ marginTop: 16 }}>
          {action}
        </div>
      )}
    </div>
  );
};

export default EmptyState;
