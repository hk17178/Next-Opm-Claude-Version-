/**
 * CardSkeleton - 卡片级骨架屏组件
 *
 * 用途：在卡片内容加载时显示占位骨架
 * 适用于：仪表盘卡片、详情卡片、统计卡片等场景
 *
 * 使用方式：
 * ```tsx
 * {loading ? <CardSkeleton /> : <RealCard data={data} />}
 * ```
 *
 * Props：
 * @param lines - 内容行数，默认 3
 * @param showAvatar - 是否显示头像占位，默认 false
 * @param showImage - 是否显示图片占位，默认 false
 */
import React from 'react';
import { Skeleton, Card } from 'antd';

export interface CardSkeletonProps {
  /** 内容行数，默认 3 */
  lines?: number;
  /** 是否显示头像占位，默认 false */
  showAvatar?: boolean;
  /** 是否显示图片占位，默认 false */
  showImage?: boolean;
  /** 卡片样式 */
  style?: React.CSSProperties;
}

export const CardSkeleton: React.FC<CardSkeletonProps> = ({
  lines = 3,
  showAvatar = false,
  showImage = false,
  style,
}) => {
  return (
    <Card
      style={{ borderRadius: 8, ...style }}
      bodyStyle={{ padding: '20px 24px' }}
    >
      {/* 图片区域占位 */}
      {showImage && (
        <Skeleton.Image
          active
          style={{ width: '100%', height: 160, marginBottom: 16 }}
        />
      )}

      {/* 标题与内容占位 */}
      <Skeleton
        active
        avatar={showAvatar}
        paragraph={{ rows: lines }}
      />
    </Card>
  );
};

export default CardSkeleton;
