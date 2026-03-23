/**
 * useDensity - 信息密度管理 Hook
 *
 * 功能：
 * - 管理三档信息密度模式（紧凑 / 标准 / 舒适）
 * - 将密度偏好持久化到 localStorage
 * - 在 document.documentElement 上设置 data-density 属性，驱动 CSS 变量切换
 *
 * 信息密度影响：
 * - 表格行高（--row-height）
 * - 基础字号（--font-size-base）
 * - 卡片间距（--card-gap）
 *
 * 使用方式：
 * ```tsx
 * const { density, setDensity } = useDensity();
 * ```
 */
import { useState, useEffect, useCallback } from 'react';

/** localStorage 中存储密度偏好的键名 */
const DENSITY_STORAGE_KEY = 'opsnexus-density';

/**
 * 信息密度模式类型
 * - compact: 紧凑模式，适合大量数据浏览
 * - normal: 标准模式，平衡可读性和信息量
 * - comfortable: 舒适模式，适合长时间阅读
 */
export type DensityMode = 'compact' | 'normal' | 'comfortable';

/** 密度模式配置信息，用于 UI 展示 */
export interface DensityOption {
  /** 模式标识 */
  key: DensityMode;
  /** 显示名称 */
  label: string;
  /** 图标提示说明 */
  description: string;
}

/** 三种密度模式的配置列表 */
export const DENSITY_OPTIONS: DensityOption[] = [
  { key: 'compact', label: '紧凑', description: '更小的行高和字号，适合浏览大量数据' },
  { key: 'normal', label: '标准', description: '默认密度，平衡可读性与信息量' },
  { key: 'comfortable', label: '舒适', description: '更大的行高和字号，适合长时间阅读' },
];

/**
 * 从 localStorage 读取已保存的密度偏好
 * 若无保存值则默认使用标准模式
 */
function getStoredDensity(): DensityMode {
  try {
    const stored = localStorage.getItem(DENSITY_STORAGE_KEY);
    if (stored === 'compact' || stored === 'normal' || stored === 'comfortable') {
      return stored;
    }
  } catch {
    // localStorage 不可用时忽略
  }
  return 'normal';
}

/**
 * 将密度模式应用到 DOM
 * CSS 变量会根据 data-density 的值自动切换（参见 theme.css）
 */
function applyDensityToDOM(mode: DensityMode) {
  document.documentElement.setAttribute('data-density', mode);
}

/**
 * 信息密度管理 Hook
 *
 * @returns density - 当前密度模式
 * @returns setDensity - 设置密度模式的函数
 * @returns densityOptions - 所有密度模式的配置列表
 */
export function useDensity() {
  const [density, setDensityState] = useState<DensityMode>(getStoredDensity);

  /** 初始化时将密度应用到 DOM */
  useEffect(() => {
    applyDensityToDOM(density);
  }, [density]);

  /** 设置密度并持久化 */
  const setDensity = useCallback((mode: DensityMode) => {
    setDensityState(mode);
    applyDensityToDOM(mode);
    try {
      localStorage.setItem(DENSITY_STORAGE_KEY, mode);
    } catch {
      // localStorage 不可用时忽略
    }
  }, []);

  return {
    /** 当前密度模式 */
    density,
    /** 设置密度模式 */
    setDensity,
    /** 所有密度模式配置列表 */
    densityOptions: DENSITY_OPTIONS,
  };
}

export default useDensity;
