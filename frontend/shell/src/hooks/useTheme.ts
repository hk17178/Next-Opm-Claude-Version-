import { useState, useEffect, useCallback } from 'react';

export type ThemeKey = 'dark' | 'light';

export interface ThemeOption {
  key: ThemeKey;
  label: string;
  isDark: boolean;
}

const THEME_KEY = 'opsnexus-theme';

const themeOptions: ThemeOption[] = [
  { key: 'dark',  label: '深色', isDark: true  },
  { key: 'light', label: '浅色', isDark: false },
];

function readTheme(): ThemeKey {
  try {
    const v = localStorage.getItem(THEME_KEY);
    if (v === 'dark' || v === 'light') return v;
  } catch { /* ignore */ }
  return 'dark'; // 默认深色
}

function applyTheme(theme: ThemeKey) {
  document.documentElement.setAttribute('data-theme', theme);
  try { localStorage.setItem(THEME_KEY, theme); } catch { /* ignore */ }
}

export function useTheme() {
  const [theme, setThemeState] = useState<ThemeKey>(readTheme);

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  // 初始化时立即应用（避免闪烁）
  useEffect(() => {
    applyTheme(readTheme());
  }, []);

  const setTheme = useCallback((t: ThemeKey) => {
    setThemeState(t);
    applyTheme(t);
  }, []);

  const toggleTheme = useCallback(() => {
    setTheme(theme === 'dark' ? 'light' : 'dark');
  }, [theme, setTheme]);

  return {
    theme,
    setTheme,
    toggleTheme,
    isDark: theme === 'dark',
    themes: themeOptions,
  };
}

export default useTheme;
