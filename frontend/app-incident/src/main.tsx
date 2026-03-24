import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { initReactI18next } from 'react-i18next';
import i18n from 'i18next';
import zhIncident from './locales/zh/incident.json';
import enIncident from './locales/en/incident.json';
import IncidentList from './pages/IncidentList';
import IncidentDetail from './pages/IncidentDetail';
import ChangeList from './pages/ChangeList';
import ChangeDetail from './pages/ChangeDetail';
import ChangeCalendar from './pages/ChangeCalendar';
import WorkflowList from './pages/WorkflowList';
import WorkflowEditor from './pages/WorkflowEditor';
import WorkflowExecutions from './pages/WorkflowExecutions';
import IncidentAnalytics from './pages/IncidentAnalytics';
import OnCallSchedule from './pages/OnCallSchedule';
import { renderWithQiankun, qiankunWindow } from 'vite-plugin-qiankun/dist/helper';
import { ConfigProvider } from 'antd';
import { useSubAppTheme } from '@opsnexus/ui-kit';

i18n.use(initReactI18next).init({
  resources: {
    zh: { incident: zhIncident },
    en: { incident: enIncident },
  },
  lng: 'zh',
  fallbackLng: 'en',
  defaultNS: 'incident',
  interpolation: { escapeValue: false },
});

/** 密度感知包装层：读取 Shell 密度设置，应用到本子应用的 antd 组件 */
const ThemeWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const theme = useSubAppTheme();
  return <ConfigProvider theme={theme}>{children}</ConfigProvider>;
};

const App: React.FC = () => (
  <Routes>
    <Route path="/list" element={<IncidentList />} />
    <Route path="/detail/:id" element={<IncidentDetail />} />
    <Route path="/changes" element={<ChangeList />} />
    <Route path="/changes/:id" element={<ChangeDetail />} />
    <Route path="/change-calendar" element={<ChangeCalendar />} />
    <Route path="/workflows" element={<WorkflowList />} />
    <Route path="/workflows/new" element={<WorkflowEditor />} />
    <Route path="/workflows/:id/edit" element={<WorkflowEditor />} />
    <Route path="/workflows/:id/executions" element={<WorkflowExecutions />} />
    <Route path="/analytics" element={<IncidentAnalytics />} />
    <Route path="/schedules" element={<OnCallSchedule />} />
    <Route path="/" element={<Navigate to="/list" replace />} />
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
        <BrowserRouter basename="/incident">
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
