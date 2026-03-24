/**
 * useSubAppTheme — 子应用主题同步 Hook
 *
 * 解决问题：qiankun 的 experimentalStyleIsolation 隔离了 CSS 变量作用域，
 * 导致子应用无法访问 shell 的 :root CSS 变量。
 *
 * 工作方式：
 * 1. 监听 localStorage 变化（主题/密度/字体）
 * 2. 同步设置 document.documentElement 的 data-theme 属性
 * 3. 通过 <style> 标签注入 CSS 变量到 document.head（绕过沙箱）
 * 4. 返回 antd ConfigProvider 的 ThemeConfig
 *
 * 因为 StorageEvent 只在跨标签页时触发，同标签页内通过
 * MutationObserver 监听 shell 对 <html> data-theme 的修改来实时同步。
 */
import { useState, useEffect } from 'react';
import type { ThemeConfig } from 'antd';
import { theme as antdTheme } from 'antd';

const DENSITY_KEY = 'opsnexus-density';
const FONT_KEY    = 'opsnexus-font';
const THEME_KEY   = 'opsnexus-theme';
const STYLE_ID    = 'opsnexus-subapp-theme-vars';

type DensityMode = 'compact' | 'normal' | 'comfortable';
type FontKey     = 'system' | 'inter' | 'mono';
type ThemeMode   = 'dark' | 'light';

const FONT_FAMILIES: Record<FontKey, string> = {
  system: `-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif`,
  inter:  `Inter, "Noto Sans SC", -apple-system, BlinkMacSystemFont, sans-serif`,
  mono:   `"JetBrains Mono", "Noto Sans Mono CJK SC", monospace`,
};

function getFontSize(d: DensityMode) { return d === 'compact' ? 12 : d === 'comfortable' ? 14 : 13; }
function readLS<T extends string>(key: string, allowed: T[], fallback: T): T {
  try { const v = localStorage.getItem(key); if (allowed.includes(v as T)) return v as T; } catch { /* ignore */ }
  return fallback;
}

/** 深色主题 CSS 变量 */
const DARK_VARS = `
  --bg-primary: #060a12;
  --bg-card: rgba(10,16,28,0.85);
  --bg-panel: rgba(10,16,28,0.7);
  --border-color: rgba(60,140,255,0.08);
  --border-glow: rgba(77,166,255,0.3);
  --text-primary: #b0c4de;
  --text-secondary: rgba(140,170,210,0.6);
  --text-muted: rgba(140,170,210,0.35);
  --color-primary: #4da6ff;
  --color-success: #00e5a0;
  --color-warning: #ffaa33;
  --color-danger: #ff6b6b;
  --color-scan: rgba(77,166,255,0.5);
  --color-p0: #ff6b6b;
  --color-p1: #ffaa33;
  --color-p2: #60a5fa;
  --color-p3: rgba(140,170,210,0.5);
  --color-p0-bg: rgba(255,107,107,0.10);
  --color-p1-bg: rgba(255,170,51,0.08);
  --color-p2-bg: rgba(96,165,250,0.08);
  --color-p3-bg: rgba(140,170,210,0.06);
  --card-bg: rgba(10,16,28,0.85);
  --border-primary: rgba(60,140,255,0.08);
  --bg-hover: rgba(77,166,255,0.06);
  --table-header-bg: rgba(10,16,28,0.6);
  --table-row-hover: rgba(77,166,255,0.04);
  --table-border: rgba(60,140,255,0.06);
  --input-bg: rgba(10,16,28,0.6);
  --input-border: rgba(60,140,255,0.15);
  --scrollbar-thumb: rgba(77,166,255,0.2);
`;

/** 浅色主题 CSS 变量 */
const LIGHT_VARS = `
  --bg-primary: #f4f7fc;
  --bg-card: rgba(255,255,255,0.82);
  --bg-panel: rgba(255,255,255,0.75);
  --border-color: rgba(56,120,220,0.08);
  --border-glow: rgba(37,99,235,0.3);
  --text-primary: #2d3748;
  --text-secondary: rgba(100,116,139,0.7);
  --text-muted: rgba(100,116,139,0.45);
  --color-primary: #2563eb;
  --color-success: #059669;
  --color-warning: #ea580c;
  --color-danger: #dc2626;
  --color-scan: rgba(37,99,235,0.25);
  --color-p0: #dc2626;
  --color-p1: #ea580c;
  --color-p2: #2563eb;
  --color-p3: rgba(100,116,139,0.5);
  --color-p0-bg: rgba(220,38,38,0.08);
  --color-p1-bg: rgba(234,88,12,0.07);
  --color-p2-bg: rgba(37,99,235,0.07);
  --color-p3-bg: rgba(100,116,139,0.05);
  --card-bg: rgba(255,255,255,0.82);
  --border-primary: rgba(56,120,220,0.08);
  --bg-hover: rgba(37,99,235,0.04);
  --table-header-bg: rgba(244,247,252,0.8);
  --table-row-hover: rgba(37,99,235,0.04);
  --table-border: rgba(56,120,220,0.06);
  --input-bg: rgba(255,255,255,0.9);
  --input-border: rgba(56,120,220,0.2);
  --scrollbar-thumb: rgba(37,99,235,0.25);
`;

/**
 * 将 CSS 变量注入到 document.head（绕过 qiankun experimentalStyleIsolation 沙箱）
 * 同时设置 data-theme 属性以驱动已加载的 CSS 规则
 */
function applyThemeVars(mode: ThemeMode) {
  document.documentElement.setAttribute('data-theme', mode);

  const vars = mode === 'dark' ? DARK_VARS : LIGHT_VARS;
  const css = `:root, [data-theme="${mode}"] { ${vars} }`;

  let el = document.getElementById(STYLE_ID);
  if (!el) {
    el = document.createElement('style');
    el.id = STYLE_ID;
    document.head.appendChild(el);
  }
  el.textContent = css;
}

export function useSubAppTheme(): ThemeConfig {
  const [density, setDensity] = useState<DensityMode>(() => readLS(DENSITY_KEY, ['compact','normal','comfortable'], 'normal'));
  const [fontKey, setFontKey] = useState<FontKey>(() => readLS(FONT_KEY, ['system','inter','mono'], 'inter'));
  const [themeMode, setThemeMode] = useState<ThemeMode>(() => readLS(THEME_KEY, ['dark','light'], 'dark'));

  /** 初始化时立即注入 CSS 变量 */
  useEffect(() => {
    applyThemeVars(themeMode);
  }, [themeMode]);

  /** 监听 localStorage 变化（跨标签页同步） */
  useEffect(() => {
    const handler = (e: StorageEvent) => {
      if (e.key === DENSITY_KEY) setDensity(readLS(DENSITY_KEY, ['compact','normal','comfortable'], 'normal'));
      if (e.key === FONT_KEY)    setFontKey(readLS(FONT_KEY, ['system','inter','mono'], 'inter'));
      if (e.key === THEME_KEY)   setThemeMode(readLS(THEME_KEY, ['dark','light'], 'dark'));
    };
    window.addEventListener('storage', handler);
    return () => window.removeEventListener('storage', handler);
  }, []);

  /**
   * 监听 shell 对 <html> data-theme 属性的修改（同标签页内同步）
   * StorageEvent 只在跨标签页触发，同一标签页内 shell 调用
   * localStorage.setItem 不会触发 storage 事件。
   * 所以用 MutationObserver 监听 document.documentElement 的属性变化。
   */
  useEffect(() => {
    const observer = new MutationObserver((mutations) => {
      for (const m of mutations) {
        if (m.type === 'attributes' && m.attributeName === 'data-theme') {
          const newTheme = document.documentElement.getAttribute('data-theme');
          if (newTheme === 'dark' || newTheme === 'light') {
            setThemeMode(newTheme);
          }
        }
      }
    });
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    return () => observer.disconnect();
  }, []);

  return {
    algorithm: themeMode === 'dark' ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
    token: {
      colorPrimary:   themeMode === 'dark' ? '#4da6ff' : '#2563eb',
      colorSuccess:   themeMode === 'dark' ? '#00e5a0' : '#059669',
      colorWarning:   themeMode === 'dark' ? '#ffaa33' : '#ea580c',
      colorError:     themeMode === 'dark' ? '#ff6b6b' : '#dc2626',
      colorBgBase:    themeMode === 'dark' ? '#060a12' : '#f4f7fc',
      fontSize:       getFontSize(density),
      fontFamily:     FONT_FAMILIES[fontKey],
      borderRadius:   6,
    },
  };
}
