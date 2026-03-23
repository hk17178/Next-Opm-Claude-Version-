import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { initReactI18next } from 'react-i18next';
import i18n from 'i18next';
import zhCmdb from './locales/zh/cmdb.json';
import enCmdb from './locales/en/cmdb.json';
import AssetList from './pages/AssetList';
import Topology from './pages/Topology';
import { renderWithQiankun, qiankunWindow } from 'vite-plugin-qiankun/dist/helper';
import { ConfigProvider } from 'antd';
import { useSubAppTheme } from '@opsnexus/ui-kit';

i18n.use(initReactI18next).init({
  resources: {
    zh: { cmdb: zhCmdb },
    en: { cmdb: enCmdb },
  },
  lng: 'zh',
  fallbackLng: 'en',
  defaultNS: 'cmdb',
  interpolation: { escapeValue: false },
});

/** 密度感知包装层：读取 Shell 密度设置，应用到本子应用的 antd 组件 */
const ThemeWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const theme = useSubAppTheme();
  return <ConfigProvider theme={theme}>{children}</ConfigProvider>;
};

const App: React.FC = () => (
  <Routes>
    <Route path="/assets" element={<AssetList />} />
    <Route path="/groups" element={<Topology />} />
    <Route path="/discovery" element={<div />} />
    <Route path="/" element={<Navigate to="/assets" replace />} />
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
        <BrowserRouter basename="/cmdb">
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
