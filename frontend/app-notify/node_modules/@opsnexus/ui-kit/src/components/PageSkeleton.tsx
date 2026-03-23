/**
 * PageSkeleton - 页面级骨架屏组件
 *
 * 用途：在页面数据加载时显示占位骨架，避免白屏闪烁，提升用户体验
 * 包含：标题区域占位 + 筛选栏占位 + 表格行占位
 *
 * 使用方式：
 * ```tsx
 * if (loading && !data.length) return <PageSkeleton />;
 * ```
 *
 * Props：
 * @param rows - 表格骨架行数，默认 8 行
 * @param showFilter - 是否显示筛选栏骨架，默认 true
 * @param showStats - 是否显示统计卡片骨架，默认 false
 */
import React from 'react';
import { Skeleton, Card, Row, Col, Space } from 'antd';

export interface PageSkeletonProps {
  /** 表格骨架行数，默认 8 */
  rows?: number;
  /** 是否显示筛选栏骨架，默认 true */
  showFilter?: boolean;
  /** 是否显示统计卡片骨架，默认 false */
  showStats?: boolean;
  /** 统计卡片数量，默认 4 */
  statCards?: number;
}

export const PageSkeleton: React.FC<PageSkeletonProps> = ({
  rows = 8,
  showFilter = true,
  showStats = false,
  statCards = 4,
}) => {
  return (
    <div>
      {/* 页面标题区域骨架 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Skeleton.Input active style={{ width: 200, height: 28 }} />
        <Space>
          <Skeleton.Button active style={{ width: 80 }} />
          <Skeleton.Button active style={{ width: 80 }} />
        </Space>
      </div>

      {/* 统计卡片骨架（可选） */}
      {showStats && (
        <Row gutter={16} style={{ marginBottom: 16 }}>
          {Array.from({ length: statCards }).map((_, i) => (
            <Col span={24 / statCards} key={i}>
              <Card style={{ borderRadius: 8 }} bodyStyle={{ padding: '16px 20px' }}>
                <Skeleton.Input active style={{ width: 80, height: 14, marginBottom: 8 }} />
                <Skeleton.Input active style={{ width: 60, height: 28 }} />
              </Card>
            </Col>
          ))}
        </Row>
      )}

      {/* 筛选栏骨架（可选） */}
      {showFilter && (
        <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
          <Space>
            <Skeleton.Input active style={{ width: 140 }} />
            <Skeleton.Input active style={{ width: 140 }} />
            <Skeleton.Input active style={{ width: 200 }} />
          </Space>
        </Card>
      )}

      {/* 表格行骨架 */}
      <Card style={{ borderRadius: 8 }} bodyStyle={{ padding: '12px 16px' }}>
        {/* 表头占位 */}
        <div style={{ display: 'flex', gap: 16, marginBottom: 12, paddingBottom: 12, borderBottom: '1px solid #f0f0f0' }}>
          <Skeleton.Input active style={{ width: 60, height: 16 }} />
          <Skeleton.Input active style={{ width: 120, height: 16 }} />
          <Skeleton.Input active style={{ width: 200, height: 16, flex: 1 }} />
          <Skeleton.Input active style={{ width: 100, height: 16 }} />
          <Skeleton.Input active style={{ width: 80, height: 16 }} />
        </div>
        {/* 数据行占位 */}
        {Array.from({ length: rows }).map((_, i) => (
          <div key={i} style={{ display: 'flex', gap: 16, marginBottom: 16 }}>
            <Skeleton.Input active style={{ width: 60, height: 16 }} />
            <Skeleton.Input active style={{ width: 120, height: 16 }} />
            <Skeleton.Input active style={{ width: '100%', height: 16, flex: 1 }} />
            <Skeleton.Input active style={{ width: 100, height: 16 }} />
            <Skeleton.Input active style={{ width: 80, height: 16 }} />
          </div>
        ))}
      </Card>
    </div>
  );
};

export default PageSkeleton;
