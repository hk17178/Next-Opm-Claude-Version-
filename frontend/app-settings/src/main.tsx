import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { initReactI18next } from 'react-i18next';
import i18n from 'i18next';
import zhSettings from './locales/zh/settings.json';
import enSettings from './locales/en/settings.json';
import Settings from './pages/Settings';
import UserManagement from './pages/UserManagement';
import SystemConfig from './pages/SystemConfig';
import APITokens from './pages/APITokens';
import MFASetup from './pages/MFASetup';
import SessionManagement from './pages/SessionManagement';
import IPWhitelist from './pages/IPWhitelist';
import SetupWizard from './pages/SetupWizard';
import { renderWithQiankun, qiankunWindow } from 'vite-plugin-qiankun/dist/helper';
import { ConfigProvider } from 'antd';
import { useSubAppTheme } from '@opsnexus/ui-kit';

i18n.use(initReactI18next).init({
  resources: {
    zh: { settings: zhSettings },
    en: { settings: enSettings },
  },
  lng: 'zh',
  fallbackLng: 'en',
  defaultNS: 'settings',
  interpolation: { escapeValue: false },
});

/** 密度感知包装层：读取 Shell 密度设置，应用到本子应用的 antd 组件 */
const ThemeWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const theme = useSubAppTheme();
  return <ConfigProvider theme={theme}>{children}</ConfigProvider>;
};

const App: React.FC = () => (
  <Routes>
    <Route path="/rbac" element={<UserManagement />} />
    <Route path="/ldap" element={<SystemConfig />} />
    <Route path="/ai-models" element={<Settings />} />
    <Route path="/prompts" element={<div />} />
    <Route path="/suggestions" element={<div />} />
    <Route path="/api-tokens" element={<APITokens />} />
    <Route path="/mfa" element={<MFASetup />} />
    <Route path="/sessions" element={<SessionManagement />} />
    <Route path="/ip-whitelist" element={<IPWhitelist />} />
    <Route path="/setup" element={<SetupWizard />} />
    <Route path="/" element={<Navigate to="/ai-models" replace />} />
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
        <BrowserRouter basename="/settings">
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
