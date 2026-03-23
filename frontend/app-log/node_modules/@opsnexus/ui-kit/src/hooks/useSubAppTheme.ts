import { useState, useEffect } from 'react';
import type { ThemeConfig } from 'antd';
import { theme as antdTheme } from 'antd';

const DENSITY_KEY = 'opsnexus-density';
const FONT_KEY    = 'opsnexus-font';
const THEME_KEY   = 'opsnexus-theme';

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

export function useSubAppTheme(): ThemeConfig {
  const [density, setDensity] = useState<DensityMode>(() => readLS(DENSITY_KEY, ['compact','normal','comfortable'], 'normal'));
  const [fontKey, setFontKey] = useState<FontKey>(() => readLS(FONT_KEY, ['system','inter','mono'], 'inter'));
  const [themeMode, setThemeMode] = useState<ThemeMode>(() => readLS(THEME_KEY, ['dark','light'], 'dark'));

  useEffect(() => {
    const handler = (e: StorageEvent) => {
      if (e.key === DENSITY_KEY) setDensity(readLS(DENSITY_KEY, ['compact','normal','comfortable'], 'normal'));
      if (e.key === FONT_KEY)    setFontKey(readLS(FONT_KEY, ['system','inter','mono'], 'inter'));
      if (e.key === THEME_KEY)   setThemeMode(readLS(THEME_KEY, ['dark','light'], 'dark'));
    };
    window.addEventListener('storage', handler);
    return () => window.removeEventListener('storage', handler);
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
