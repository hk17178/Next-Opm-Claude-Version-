import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { initReactI18next } from 'react-i18next';
import i18n from 'i18next';
import zhDashboard from './locales/zh/dashboard.json';
import enDashboard from './locales/en/dashboard.json';
import Home from './pages/Home';
import Dashboard from './pages/Dashboard';
import NOCScreen from './pages/NOCScreen';
import Suggestions from './pages/Suggestions';
import { renderWithQiankun, qiankunWindow } from 'vite-plugin-qiankun/dist/helper';
import { ConfigProvider } from 'antd';
import { useSubAppTheme } from '@opsnexus/ui-kit';

i18n.use(initReactI18next).init({
  resources: {
    zh: { dashboard: zhDashboard },
    en: { dashboard: enDashboard },
  },
  lng: 'zh',
  fallbackLng: 'en',
  defaultNS: 'dashboard',
  interpolation: { escapeValue: false },
});

/** 密度感知包装层：读取 Shell 密度设置，应用到本子应用的 antd 组件 */
const ThemeWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const theme = useSubAppTheme();
  return <ConfigProvider theme={theme}>{children}</ConfigProvider>;
};

/**
 * 统一路由：使用绝对路径，不依赖 basename。
 * qiankun 的 activeRule 同时覆盖 /home 和 /dashboard，子应用在两个路径下
 * 只挂载一次，render() 不会被重复调用。用绝对路径可以在单次挂载内正确
 * 响应 shell 的路由切换，避免 basename 固化导致的空白页面。
 */
const App: React.FC = () => (
  <Routes>
    <Route path="/home" element={<Home />} />
    <Route path="/dashboard" element={<Dashboard />} />
    <Route path="/dashboard/noc" element={<NOCScreen />} />
    <Route path="/dashboard/suggestions" element={<Suggestions />} />
    <Route path="/" element={<Navigate to="/home" replace />} />
  </Routes>
);

let root: ReactDOM.Root | null = null;

function render(container?: HTMLElement) {
  let mountEl: HTMLElement;
  if (container) {
    let el = container.querySelector<HTMLElement>('#root');
    if (!el) {
      el = document.createElement('div');
      el.id = 'root';
      container.appendChild(el);
    }
    mountEl = el;
  } else {
    mountEl = document.getElementById('root')!;
  }

  root = ReactDOM.createRoot(mountEl);
  root.render(
    <React.StrictMode>
      <ThemeWrapper>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </ThemeWrapper>
    </React.StrictMode>,
  );
}

renderWithQiankun({
  mount(props: any) {
    render(props?.container as HTMLElement);
  },
  bootstrap() {},
  unmount() {
    root?.unmount();
    root = null;
  },
  update() {},
});

if (!qiankunWindow.__POWERED_BY_QIANKUN__) {
  render();
}
