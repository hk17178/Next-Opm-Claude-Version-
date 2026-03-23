/**
 * useColumnConfig - 表格列配置 Hook
 *
 * 功能：
 * - 列显示/隐藏切换
 * - 列顺序调整（上移/下移）
 * - 配置持久化到 localStorage（按 tableKey 隔离）
 * - 一键重置为默认配置
 * - 提供 ColumnConfigButton 渲染节点（点击弹出配置面板）
 *
 * 使用方式：
 * ```tsx
 * const { columns, ColumnConfigButton } = useColumnConfig('alert-list', defaultColumns);
 * ```
 */
import React, { useState, useCallback, useMemo } from 'react';
import { Button, Popover, Checkbox, Space, Divider, Tooltip } from 'antd';
import {
  SettingOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
  UndoOutlined,
} from '@ant-design/icons';
import type { ColumnDef } from '../components/VirtualTable';

/** localStorage 键名前缀 */
const STORAGE_PREFIX = 'opsnexus-col-config-';

/**
 * 持久化的列配置数据结构
 * 只存储列的可见性和顺序，不存储完整列定义
 */
interface ColumnConfigData {
  /** 隐藏的列 key 列表 */
  hiddenKeys: string[];
  /** 列顺序（key 数组） */
  order: string[];
}

/**
 * 从 localStorage 读取列配置
 * @param tableKey - 表格唯一标识
 * @returns 已保存的列配置，或 null（无保存值时）
 */
function loadConfig(tableKey: string): ColumnConfigData | null {
  try {
    const raw = localStorage.getItem(`${STORAGE_PREFIX}${tableKey}`);
    if (raw) {
      return JSON.parse(raw) as ColumnConfigData;
    }
  } catch {
    // localStorage 不可用或数据损坏时忽略
  }
  return null;
}

/**
 * 将列配置保存到 localStorage
 * @param tableKey - 表格唯一标识
 * @param config - 列配置数据
 */
function saveConfig(tableKey: string, config: ColumnConfigData): void {
  try {
    localStorage.setItem(`${STORAGE_PREFIX}${tableKey}`, JSON.stringify(config));
  } catch {
    // localStorage 不可用时忽略
  }
}

/**
 * 列配置面板组件属性
 */
interface ColumnConfigPanelProps {
  /** 所有列的 key 和标题 */
  allColumns: { key: string; title: React.ReactNode }[];
  /** 当前隐藏的列 key 列表 */
  hiddenKeys: string[];
  /** 当前列顺序 */
  order: string[];
  /** 切换列显示/隐藏 */
  onToggle: (key: string) => void;
  /** 上移列 */
  onMoveUp: (key: string) => void;
  /** 下移列 */
  onMoveDown: (key: string) => void;
  /** 重置为默认配置 */
  onReset: () => void;
}

/**
 * 列配置面板 - 展示在 Popover 中的列管理界面
 * 支持勾选显示/隐藏、上下移动调整顺序、重置为默认
 */
const ColumnConfigPanel: React.FC<ColumnConfigPanelProps> = ({
  allColumns,
  hiddenKeys,
  order,
  onToggle,
  onMoveUp,
  onMoveDown,
  onReset,
}) => {
  /** 按当前 order 排序列 */
  const sortedColumns = useMemo(() => {
    const orderMap = new Map(order.map((key, idx) => [key, idx]));
    return [...allColumns].sort(
      (a, b) => (orderMap.get(a.key) ?? 999) - (orderMap.get(b.key) ?? 999),
    );
  }, [allColumns, order]);

  return (
    <div style={{ width: 240, maxHeight: 400, overflow: 'auto' }}>
      {/* 标题栏：标题 + 重置按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        <span style={{ fontWeight: 600, fontSize: 13 }}>列配置</span>
        <Tooltip title="重置为默认">
          <Button type="text" size="small" icon={<UndoOutlined />} onClick={onReset} />
        </Tooltip>
      </div>
      <Divider style={{ margin: '4px 0 8px' }} />
      {/* 列列表：每行一个列，含勾选框和上下移动按钮 */}
      {sortedColumns.map((col, idx) => (
        <div
          key={col.key}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '4px 0',
            borderBottom: '1px solid #f0f0f0',
          }}
        >
          {/* 勾选框：控制列的显示/隐藏 */}
          <Checkbox
            checked={!hiddenKeys.includes(col.key)}
            onChange={() => onToggle(col.key)}
            style={{ fontSize: 13 }}
          >
            {col.title}
          </Checkbox>
          {/* 上下移动按钮 */}
          <Space size={2}>
            <Button
              type="text"
              size="small"
              icon={<ArrowUpOutlined style={{ fontSize: 11 }} />}
              disabled={idx === 0}
              onClick={() => onMoveUp(col.key)}
            />
            <Button
              type="text"
              size="small"
              icon={<ArrowDownOutlined style={{ fontSize: 11 }} />}
              disabled={idx === sortedColumns.length - 1}
              onClick={() => onMoveDown(col.key)}
            />
          </Space>
        </div>
      ))}
    </div>
  );
};

/**
 * useColumnConfig Hook
 *
 * 管理表格列的显示/隐藏、顺序调整，并持久化到 localStorage
 *
 * @param tableKey - 表格唯一标识，用作 localStorage 的 key 后缀
 * @param defaultColumns - 默认列定义数组
 * @returns columns - 当前可见且已排序的列配置
 * @returns hiddenKeys - 当前隐藏的列 key 数组
 * @returns toggleColumn - 切换某列显示/隐藏
 * @returns resetColumns - 重置为默认列配置
 * @returns ColumnConfigButton - 列配置按钮（包含 Popover 面板）
 */
export function useColumnConfig<T>(
  tableKey: string,
  defaultColumns: ColumnDef<T>[],
): {
  columns: ColumnDef<T>[];
  hiddenKeys: string[];
  toggleColumn: (key: string) => void;
  resetColumns: () => void;
  ColumnConfigButton: React.ReactNode;
} {
  /** 默认隐藏的列（标记了 defaultHidden 的列） */
  const defaultHiddenKeys = useMemo(
    () => defaultColumns.filter((c) => c.defaultHidden).map((c) => c.key),
    [defaultColumns],
  );

  /** 默认列顺序 */
  const defaultOrder = useMemo(
    () => defaultColumns.map((c) => c.key),
    [defaultColumns],
  );

  /** 从 localStorage 加载已保存的配置，若无则使用默认值 */
  const savedConfig = useMemo(() => loadConfig(tableKey), [tableKey]);

  /** 隐藏的列 key 列表 */
  const [hiddenKeys, setHiddenKeys] = useState<string[]>(
    savedConfig?.hiddenKeys ?? defaultHiddenKeys,
  );

  /** 列显示顺序 */
  const [order, setOrder] = useState<string[]>(
    savedConfig?.order ?? defaultOrder,
  );

  /**
   * 持久化当前配置到 localStorage
   */
  const persistConfig = useCallback(
    (newHidden: string[], newOrder: string[]) => {
      saveConfig(tableKey, { hiddenKeys: newHidden, order: newOrder });
    },
    [tableKey],
  );

  /**
   * 切换列的显示/隐藏状态
   * @param key - 列的唯一标识
   */
  const toggleColumn = useCallback(
    (key: string) => {
      setHiddenKeys((prev) => {
        const next = prev.includes(key)
          ? prev.filter((k) => k !== key)
          : [...prev, key];
        persistConfig(next, order);
        return next;
      });
    },
    [order, persistConfig],
  );

  /**
   * 上移指定列（在顺序中与前一列交换位置）
   * @param key - 列的唯一标识
   */
  const moveUp = useCallback(
    (key: string) => {
      setOrder((prev) => {
        const idx = prev.indexOf(key);
        if (idx <= 0) return prev;
        const next = [...prev];
        [next[idx - 1], next[idx]] = [next[idx], next[idx - 1]];
        persistConfig(hiddenKeys, next);
        return next;
      });
    },
    [hiddenKeys, persistConfig],
  );

  /**
   * 下移指定列（在顺序中与后一列交换位置）
   * @param key - 列的唯一标识
   */
  const moveDown = useCallback(
    (key: string) => {
      setOrder((prev) => {
        const idx = prev.indexOf(key);
        if (idx < 0 || idx >= prev.length - 1) return prev;
        const next = [...prev];
        [next[idx], next[idx + 1]] = [next[idx + 1], next[idx]];
        persistConfig(hiddenKeys, next);
        return next;
      });
    },
    [hiddenKeys, persistConfig],
  );

  /**
   * 重置为默认列配置
   * 清除 localStorage 中的持久化数据
   */
  const resetColumns = useCallback(() => {
    setHiddenKeys(defaultHiddenKeys);
    setOrder(defaultOrder);
    try {
      localStorage.removeItem(`${STORAGE_PREFIX}${tableKey}`);
    } catch {
      // 忽略
    }
  }, [defaultHiddenKeys, defaultOrder, tableKey]);

  /**
   * 根据当前隐藏状态和顺序，计算最终可见的列配置
   * 过滤掉隐藏的列，并按 order 排序
   */
  const columns = useMemo(() => {
    const orderMap = new Map(order.map((key, idx) => [key, idx]));
    return defaultColumns
      .filter((col) => !hiddenKeys.includes(col.key))
      .sort((a, b) => (orderMap.get(a.key) ?? 999) - (orderMap.get(b.key) ?? 999));
  }, [defaultColumns, hiddenKeys, order]);

  /** 所有列的 key 和标题，用于配置面板展示 */
  const allColumnsMeta = useMemo(
    () => defaultColumns.map((c) => ({ key: c.key, title: c.title })),
    [defaultColumns],
  );

  /**
   * 列配置按钮 - 包含一个 Popover，点击后弹出列配置面板
   */
  const ColumnConfigButton = useMemo(
    () => (
      <Popover
        trigger="click"
        placement="bottomRight"
        content={
          <ColumnConfigPanel
            allColumns={allColumnsMeta}
            hiddenKeys={hiddenKeys}
            order={order}
            onToggle={toggleColumn}
            onMoveUp={moveUp}
            onMoveDown={moveDown}
            onReset={resetColumns}
          />
        }
      >
        <Tooltip title="列配置">
          <Button icon={<SettingOutlined />} />
        </Tooltip>
      </Popover>
    ),
    [allColumnsMeta, hiddenKeys, order, toggleColumn, moveUp, moveDown, resetColumns],
  );

  return {
    columns,
    hiddenKeys,
    toggleColumn,
    resetColumns,
    ColumnConfigButton,
  };
}

export default useColumnConfig;
