/**
 * useFont - 字体切换 Hook
 * 管理界面字体偏好，支持 3 种字体选项（FR-25-001）
 * 持久化到 localStorage，并通过 CSS 变量和 document style 应用到全局
 */
import { useState, useEffect, useCallback } from 'react';

const FONT_STORAGE_KEY = 'opsnexus-font';

export type FontKey = 'system' | 'inter' | 'mono';

export interface FontOption {
  key: FontKey;
  label: string;
  description: string;
  /** antd ConfigProvider fontFamily token 值 */
  fontFamily: string;
}

export const FONT_OPTIONS: FontOption[] = [
  {
    key: 'system',
    label: '系统默认',
    description: '使用操作系统的默认字体',
    fontFamily: `-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif`,
  },
  {
    key: 'inter',
    label: 'Inter + 思源黑体',
    description: '英文 Inter，中文 Noto Sans SC（推荐）',
    fontFamily: `'Inter', 'Noto Sans SC', -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`,
  },
  {
    key: 'mono',
    label: '等宽字体',
    description: '适合代码和日志阅读',
    fontFamily: `'JetBrains Mono', 'Noto Sans Mono CJK SC', 'Courier New', monospace`,
  },
];

function getStoredFont(): FontKey {
  try {
    const v = localStorage.getItem(FONT_STORAGE_KEY);
    if (v === 'system' || v === 'inter' || v === 'mono') return v;
  } catch { /* ignore */ }
  return 'inter';
}

function applyFontToDOM(font: FontOption) {
  document.documentElement.style.setProperty('--font-family-base', font.fontFamily);
  document.body.style.fontFamily = font.fontFamily;
}

export function useFont() {
  const [fontKey, setFontKeyState] = useState<FontKey>(getStoredFont);
  const currentFont = FONT_OPTIONS.find(f => f.key === fontKey) ?? FONT_OPTIONS[1];

  useEffect(() => {
    applyFontToDOM(currentFont);
  }, [currentFont]);

  const setFont = useCallback((key: FontKey) => {
    const font = FONT_OPTIONS.find(f => f.key === key) ?? FONT_OPTIONS[1];
    setFontKeyState(key);
    applyFontToDOM(font);
    try {
      localStorage.setItem(FONT_STORAGE_KEY, key);
    } catch { /* ignore */ }
  }, []);

  return {
    fontKey,
    currentFont,
    setFont,
    fontOptions: FONT_OPTIONS,
  };
}
