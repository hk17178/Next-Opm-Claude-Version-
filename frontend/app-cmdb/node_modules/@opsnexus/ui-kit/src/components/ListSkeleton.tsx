/**
 * ListSkeleton - 列表级骨架屏组件
 *
 * 用途：在列表数据加载时显示 N 行占位骨架
 * 适用于：通知列表、消息列表、事件流等场景
 *
 * 使用方式：
 * ```tsx
 * {loading ? <ListSkeleton rows={5} /> : <RealList data={items} />}
 * ```
 *
 * Props：
 * @param rows - 骨架行数，默认 5
 * @param showAvatar - 每行是否显示头像占位，默认 false
 * @param showAction - 每行是否显示操作按钮占位，默认 false
 */
import React from 'react';
import { Skeleton, Card } from 'antd';

export interface ListSkeletonProps {
  /** 骨架行数，默认 5 */
  rows?: number;
  /** 每行是否显示头像占位，默认 false */
  showAvatar?: boolean;
  /** 每行是否显示操作按钮占位，默认 false */
  showAction?: boolean;
}

export const ListSkeleton: React.FC<ListSkeletonProps> = ({
  rows = 5,
  showAvatar = false,
  showAction = false,
}) => {
  return (
    <Card style={{ borderRadius: 8 }} bodyStyle={{ padding: '8px 16px' }}>
      {Array.from({ length: rows }).map((_, i) => (
        <div
          key={i}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            padding: '12px 0',
            borderBottom: i < rows - 1 ? '1px solid #f0f0f0' : 'none',
          }}
        >
          {/* 头像占位（可选） */}
          {showAvatar && (
            <Skeleton.Avatar active size={36} />
          )}

          {/* 文本内容占位 */}
          <div style={{ flex: 1 }}>
            <Skeleton.Input active style={{ width: '60%', height: 16, marginBottom: 8 }} />
            <Skeleton.Input active style={{ width: '40%', height: 12 }} />
          </div>

          {/* 操作按钮占位（可选） */}
          {showAction && (
            <Skeleton.Button active size="small" style={{ width: 60 }} />
          )}
        </div>
      ))}
    </Card>
  );
};

export default ListSkeleton;
