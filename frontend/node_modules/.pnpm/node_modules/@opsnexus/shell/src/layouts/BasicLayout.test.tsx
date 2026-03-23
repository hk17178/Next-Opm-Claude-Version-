import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

const mockNavigate = vi.fn();
const mockChangeLanguage = vi.fn();

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => ({ pathname: '/log/search' }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'menu.log': '日志中心',
        'menu.log.search': '日志检索',
        'menu.log.analysis': '日志分析',
        'menu.alert': '告警中心',
        'menu.alert.list': '告警列表',
        'menu.alert.rules': '告警规则',
        'menu.alert.baselines': '基线管理',
        'menu.alert.noiseReduction': '降噪配置',
        'menu.incident': '事件管理',
        'menu.incident.list': '事件列表',
        'menu.incident.changes': '变更管理',
        'menu.incident.schedules': '值班排班',
        'menu.cockpit': '指挥驾驶舱',
        'menu.cmdb': '资产管理',
        'menu.cmdb.assets': '资产列表',
        'menu.cmdb.groups': '资产组',
        'menu.cmdb.discovery': '自动发现',
        'menu.notify': '通知中心',
        'menu.notify.bots': '机器人管理',
        'menu.notify.logs': '通知记录',
        'menu.analytics': '数据分析',
        'menu.analytics.sla': 'SLA 管理',
        'menu.analytics.correlation': '关联分析',
        'menu.analytics.reports': '事件报告',
        'menu.analytics.knowledge': '知识库',
        'menu.settings': '系统设置',
        'menu.settings.rbac': '角色权限',
        'menu.settings.ldap': 'LDAP 管理',
        'menu.settings.aiModels': 'AI 模型管理',
        'menu.settings.prompts': 'Prompt 管理',
        'menu.settings.suggestions': '建议管理',
        'menu.dashboard': '运维大屏',
        'header.globalSearch': '全局搜索: 主机、服务、告警、事件...',
        'header.profile': '个人中心',
        'header.logout': '退出登录',
      };
      return map[key] ?? key;
    },
    i18n: { language: 'zh', changeLanguage: mockChangeLanguage },
  }),
}));

vi.mock('@ant-design/pro-components', () => ({
  ProLayout: ({ children, route, actionsRender, headerContentRender, menuItemRender, ...rest }: any) => {
    const actions = actionsRender ? actionsRender() : [];
    const headerContent = headerContentRender ? headerContentRender() : null;

    const renderMenuItems = (items: any[], depth = 0) => {
      return items?.map((item: any, idx: number) => (
        <div key={idx} data-testid={`menu-item-${depth}-${idx}`}>
          {menuItemRender ? menuItemRender(item, <span>{item.name}</span>) : <span>{item.name}</span>}
          {item.children && renderMenuItems(item.children, depth + 1)}
        </div>
      ));
    };

    return (
      <div data-testid="pro-layout" data-title={rest.title}>
        <nav data-testid="sidebar">
          {route?.routes && renderMenuItems(route.routes)}
        </nav>
        <header data-testid="header">
          {headerContent}
          <div data-testid="header-actions">
            {actions.map((action: any, idx: number) => (
              <div key={idx} data-testid={`action-${idx}`}>{action}</div>
            ))}
          </div>
        </header>
        <main>{children}</main>
      </div>
    );
  },
}));

import BasicLayout from './BasicLayout';

describe('BasicLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the layout with OpsNexus title', () => {
    render(<BasicLayout />);
    const layout = screen.getByTestId('pro-layout');
    expect(layout).toHaveAttribute('data-title', 'OpsNexus');
  });

  it('renders all 9 top-level menu groups', () => {
    render(<BasicLayout />);
    expect(screen.getByText('日志中心')).toBeInTheDocument();
    expect(screen.getByText('告警中心')).toBeInTheDocument();
    expect(screen.getByText('事件管理')).toBeInTheDocument();
    expect(screen.getByText('指挥驾驶舱')).toBeInTheDocument();
    expect(screen.getByText('资产管理')).toBeInTheDocument();
    expect(screen.getByText('通知中心')).toBeInTheDocument();
    expect(screen.getByText('数据分析')).toBeInTheDocument();
    expect(screen.getByText('系统设置')).toBeInTheDocument();
    expect(screen.getByText('运维大屏')).toBeInTheDocument();
  });

  it('renders sub-menu items for nested menus', () => {
    render(<BasicLayout />);
    // Log sub-items
    expect(screen.getByText('日志检索')).toBeInTheDocument();
    expect(screen.getByText('日志分析')).toBeInTheDocument();
    // Alert sub-items
    expect(screen.getByText('告警列表')).toBeInTheDocument();
    expect(screen.getByText('告警规则')).toBeInTheDocument();
    // Settings sub-items
    expect(screen.getByText('角色权限')).toBeInTheDocument();
    expect(screen.getByText('AI 模型管理')).toBeInTheDocument();
  });

  it('renders global search input in header', () => {
    render(<BasicLayout />);
    const searchInput = screen.getByPlaceholderText('全局搜索: 主机、服务、告警、事件...');
    expect(searchInput).toBeInTheDocument();
  });

  it('renders language switcher showing current language', () => {
    render(<BasicLayout />);
    // Language switcher shows "中文" when language is zh
    expect(screen.getByText('中文')).toBeInTheDocument();
  });

  it('renders notification badge in header actions', () => {
    render(<BasicLayout />);
    const actionsContainer = screen.getByTestId('header-actions');
    expect(actionsContainer).toBeInTheDocument();
    // Badge with count 5 is rendered
    expect(actionsContainer.querySelector('.ant-badge')).toBeTruthy();
  });

  it('renders dark mode toggle switch', () => {
    render(<BasicLayout />);
    const switchEl = document.querySelector('.ant-switch');
    expect(switchEl).toBeTruthy();
  });

  it('renders user avatar dropdown with profile and logout', () => {
    render(<BasicLayout />);
    // The avatar icon is rendered within the actions
    const actionsContainer = screen.getByTestId('header-actions');
    expect(actionsContainer.querySelector('.ant-avatar')).toBeTruthy();
  });

  it('renders micro-app-container div', () => {
    render(<BasicLayout />);
    const container = document.getElementById('micro-app-container');
    expect(container).toBeTruthy();
  });
});
