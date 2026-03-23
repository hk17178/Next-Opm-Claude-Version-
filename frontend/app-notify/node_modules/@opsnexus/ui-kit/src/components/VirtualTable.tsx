/**
 * VirtualTable 虚拟滚动表格组件
 *
 * 适用于 10K+ 行的大数据列表，只渲染可视区域内的 DOM 节点，
 * 保持流畅的 60fps 滚动性能。
 *
 * 实现方案：基于 Ant Design 5.x Table 的 virtual 属性 + 自定义滚动容器
 * - 滚动时动态渲染/销毁 DOM 节点
 * - 支持行点击、行选中、行 hover 高亮
 * - 支持滚动到底部触发回调（用于分页加载）
 * - 支持空状态展示
 */
import React, { useCallback, useRef, useEffect, useMemo } from 'react';
import { Table, Empty, Spin } from 'antd';
import type { TableProps } from 'antd';

/**
 * 列定义类型
 * 与 Ant Design Table columns 兼容，扩展了 key 必填约束
 */
export interface ColumnDef<T = unknown> {
  /** 列唯一标识 */
  key: string;
  /** 列标题 */
  title: React.ReactNode;
  /** 数据字段名 */
  dataIndex?: string | string[];
  /** 列宽度（像素） */
  width?: number;
  /** 是否固定列 */
  fixed?: 'left' | 'right' | boolean;
  /** 文本溢出省略 */
  ellipsis?: boolean;
  /** 排序支持 */
  sorter?: boolean | ((a: T, b: T) => number);
  /** 自定义渲染函数 */
  render?: (value: unknown, record: T, index: number) => React.ReactNode;
  /** 列对齐方式 */
  align?: 'left' | 'center' | 'right';
  /** 列是否默认隐藏（用于 useColumnConfig） */
  defaultHidden?: boolean;
}

/**
 * VirtualTable 组件属性
 */
export interface VirtualTableProps<T> {
  /** 数据源 */
  dataSource: T[];
  /** 列定义（与 Ant Design Table columns 兼容） */
  columns: ColumnDef<T>[];
  /** 行高（默认 48px） */
  rowHeight?: number;
  /** 容器高度（默认 600px） */
  height?: number;
  /** 滚动到底部时触发（用于分页加载） */
  onEndReached?: () => void;
  /** 加载状态 */
  loading?: boolean;
  /** 行唯一键字段名（默认 'id'） */
  rowKey?: string | ((record: T) => string);
  /** 行点击事件 */
  onRowClick?: (record: T, index: number) => void;
  /** 行选中变化回调 */
  onSelectionChange?: (selectedKeys: React.Key[], selectedRows: T[]) => void;
  /** 是否启用行选中 */
  selectable?: boolean;
  /** 空状态描述文字 */
  emptyText?: string;
  /** 触底阈值（距离底部多少像素触发 onEndReached，默认 100） */
  endReachedThreshold?: number;
  /** 表格额外的 className */
  className?: string;
  /** 表格额外的 style */
  style?: React.CSSProperties;
}

/**
 * VirtualTable 虚拟滚动表格
 *
 * 使用 Ant Design 5.x 内置的 virtual 属性实现虚拟滚动，
 * 仅渲染可视区域内的行，适合大数据量场景。
 *
 * @example
 * ```tsx
 * <VirtualTable
 *   dataSource={logs}
 *   columns={columns}
 *   height={600}
 *   rowHeight={48}
 *   onEndReached={loadMore}
 *   loading={isLoading}
 * />
 * ```
 */
function VirtualTableInner<T extends Record<string, unknown>>(
  props: VirtualTableProps<T>,
) {
  const {
    dataSource,
    columns,
    rowHeight = 48,
    height = 600,
    onEndReached,
    loading = false,
    rowKey = 'id',
    onRowClick,
    onSelectionChange,
    selectable = false,
    emptyText = '暂无数据',
    endReachedThreshold = 100,
    className,
    style,
  } = props;

  /** 滚动容器引用，用于监听滚动事件判断是否触底 */
  const scrollContainerRef = useRef<HTMLDivElement | null>(null);
  /** 防止重复触发 onEndReached 的标记 */
  const endReachedFiredRef = useRef(false);

  /**
   * 监听虚拟表格滚动事件
   * 当滚动到距离底部 endReachedThreshold 像素时，触发 onEndReached 回调
   */
  const handleScroll = useCallback(
    (e: React.UIEvent<HTMLDivElement>) => {
      if (!onEndReached) return;
      const target = e.currentTarget;
      const { scrollTop, scrollHeight, clientHeight } = target;
      const distanceToBottom = scrollHeight - scrollTop - clientHeight;

      if (distanceToBottom <= endReachedThreshold && !endReachedFiredRef.current) {
        endReachedFiredRef.current = true;
        onEndReached();
      }

      // 当用户向上滚动一定距离后，重置触底标记，允许再次触发
      if (distanceToBottom > endReachedThreshold * 2) {
        endReachedFiredRef.current = false;
      }
    },
    [onEndReached, endReachedThreshold],
  );

  /** 数据源变化时重置触底标记 */
  useEffect(() => {
    endReachedFiredRef.current = false;
  }, [dataSource.length]);

  /**
   * 注册滚动监听
   * Ant Design 虚拟表格的滚动容器为 .ant-table-body，需要在挂载后查找并绑定事件
   */
  useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container) return;

    const scrollBody = container.querySelector('.ant-table-body') as HTMLDivElement | null;
    if (!scrollBody || !onEndReached) return;

    const handleNativeScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = scrollBody;
      const distanceToBottom = scrollHeight - scrollTop - clientHeight;

      if (distanceToBottom <= endReachedThreshold && !endReachedFiredRef.current) {
        endReachedFiredRef.current = true;
        onEndReached();
      }
      if (distanceToBottom > endReachedThreshold * 2) {
        endReachedFiredRef.current = false;
      }
    };

    scrollBody.addEventListener('scroll', handleNativeScroll, { passive: true });
    return () => scrollBody.removeEventListener('scroll', handleNativeScroll);
  }, [onEndReached, endReachedThreshold, dataSource]);

  /** 行选择配置（仅在 selectable 为 true 时启用） */
  const rowSelection: TableProps<T>['rowSelection'] = selectable
    ? {
        onChange: (selectedRowKeys: React.Key[], selectedRows: T[]) => {
          onSelectionChange?.(selectedRowKeys, selectedRows);
        },
      }
    : undefined;

  /** 转换列定义为 Ant Design Table 兼容格式 */
  const antColumns = useMemo(
    () =>
      columns.map((col) => ({
        ...col,
        // 确保每列都有 width，虚拟滚动模式下需要固定列宽
        width: col.width || 150,
      })),
    [columns],
  );

  return (
    <div ref={scrollContainerRef} className={className} style={style}>
      <Spin spinning={loading}>
        <Table<T>
          columns={antColumns as TableProps<T>['columns']}
          dataSource={dataSource}
          pagination={false}
          virtual
          scroll={{ y: height, x: 'max-content' }}
          rowKey={rowKey}
          rowSelection={rowSelection}
          size="middle"
          onRow={(record, index) => ({
            onClick: () => onRowClick?.(record, index ?? 0),
            style: { cursor: onRowClick ? 'pointer' : 'default', height: rowHeight },
          })}
          locale={{
            emptyText: (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={emptyText}
              />
            ),
          }}
        />
      </Spin>
    </div>
  );
}

/**
 * 导出的 VirtualTable 组件
 * 使用类型断言保留泛型支持
 */
export const VirtualTable = VirtualTableInner as <T extends Record<string, unknown>>(
  props: VirtualTableProps<T>,
) => React.ReactElement;

export default VirtualTable;
