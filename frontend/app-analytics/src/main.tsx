import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { initReactI18next } from 'react-i18next';
import i18n from 'i18next';
import zhAnalytics from './locales/zh/analytics.json';
import enAnalytics from './locales/en/analytics.json';
import SLAReport from './pages/SLAReport';
import SLADashboard from './pages/SLADashboard';
import MetricsReport from './pages/MetricsReport';
import CorrelationAnalysis from './pages/CorrelationAnalysis';
import ReportCenter from './pages/ReportCenter';
import AuditAnalytics from './pages/AuditAnalytics';
import KnowledgeBase from './pages/KnowledgeBase';
import { renderWithQiankun, qiankunWindow } from 'vite-plugin-qiankun/dist/helper';
import { ConfigProvider } from 'antd';
import { useSubAppTheme } from '@opsnexus/ui-kit';

i18n.use(initReactI18next).init({
  resources: {
    zh: { analytics: zhAnalytics },
    en: { analytics: enAnalytics },
  },
  lng: 'zh',
  fallbackLng: 'en',
  defaultNS: 'analytics',
  interpolation: { escapeValue: false },
});

/** 密度感知包装层：读取 Shell 密度设置，应用到本子应用的 antd 组件 */
const ThemeWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const theme = useSubAppTheme();
  return <ConfigProvider theme={theme}>{children}</ConfigProvider>;
};

const App: React.FC = () => (
  <Routes>
    <Route path="/sla" element={<SLAReport />} />
    <Route path="/sla-dashboard" element={<SLADashboard />} />
    <Route path="/correlation" element={<CorrelationAnalysis />} />
    <Route path="/reports" element={<ReportCenter />} />
    <Route path="/knowledge" element={<KnowledgeBase />} />
    <Route path="/audit/analytics" element={<AuditAnalytics />} />
    <Route path="/metrics" element={<MetricsReport />} />
    <Route path="/" element={<Navigate to="/sla-dashboard" replace />} />
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
        <BrowserRouter basename="/analytics">
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
