/**
 * BasicLayout - 应用主布局组件 (v2.0)
 *
 * 功能：
 * - 纯 flex 布局（Header + Sidebar + Content），不依赖 ProLayout
 * - 深/浅双主题切换（useTheme hook）
 * - 信息密度切换（useDensity hook）
 * - 粒子网络背景（ParticleCanvas，待 ui-kit 就绪后自动生效）
 * - 可折叠侧边栏（220px ↔ 48px）
 * - 集成通知铃铛、连接状态指示器、语言切换、用户头像
 */
import React, { useState, useMemo } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import {
  Input, Avatar, Dropdown, Tooltip, Button, ConfigProvider,
  theme as antdTheme,
} from 'antd';
import {
  SearchOutlined,
  UserOutlined,
  SunOutlined,
  MoonFilled,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  HomeOutlined,
  FundProjectionScreenOutlined,
  ControlOutlined,
  FileTextOutlined,
  AlertOutlined,
  WarningOutlined,
  DatabaseOutlined,
  BarChartOutlined,
  BellOutlined,
  SettingOutlined,
  GlobalOutlined,
  SwapOutlined,
  ApiOutlined,
  BookOutlined,
  ClusterOutlined,
  SafetyCertificateOutlined,
  KeyOutlined,
  LockOutlined,
  RobotOutlined,
  EditOutlined,
  BulbOutlined,
  SkinOutlined,
  UserSwitchOutlined,
  HeartOutlined,
  CloudServerOutlined,
  TagsOutlined,
  ExportOutlined,
  ScheduleOutlined,
  ApartmentOutlined,
  RadarChartOutlined,
  AuditOutlined,
  ThunderboltOutlined,
  FilterOutlined,
  LineChartOutlined,
  FileSearchOutlined,
  EyeOutlined,
  TeamOutlined,
  ToolOutlined,
  ProfileOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useTheme } from '../hooks/useTheme';
import { useDensity } from '../hooks/useDensity';
import NotificationBell from '../components/NotificationBell';
import ConnectionIndicator from '../components/ConnectionIndicator';

// ParticleCanvas 通过 React.lazy + dynamic import 加载
// 避免 ui-kit 加载失败时整个 shell 崩溃
const LazyParticleCanvas = React.lazy(() =>
  import('@opsnexus/ui-kit')
    .then(mod => ({ default: mod.ParticleCanvas }))
    .catch(() => ({ default: (() => null) as React.FC<{ isDark?: boolean }> }))
);

/* ─── 菜单数据结构 ─── */

interface MenuItem {
  key: string;
  path: string;
  nameKey: string;
  icon: React.ReactNode;
}

interface MenuGroup {
  groupKey: string;
  nameKey: string;
  items: MenuItem[];
}

const menuGroups: MenuGroup[] = [
  {
    groupKey: 'monitor',
    nameKey: 'menu.monitor',
    items: [
      { key: 'home',      path: '/home',      nameKey: 'menu.monitor.home',      icon: <HomeOutlined /> },
      { key: 'dashboard', path: '/dashboard',  nameKey: 'menu.monitor.dashboard', icon: <FundProjectionScreenOutlined /> },
      { key: 'cockpit',   path: '/cockpit',    nameKey: 'menu.monitor.cockpit',   icon: <ControlOutlined /> },
    ],
  },
  {
    groupKey: 'log',
    nameKey: 'menu.log',
    items: [
      { key: 'log-search',   path: '/log/search',   nameKey: 'menu.log.search',   icon: <FileSearchOutlined /> },
      { key: 'log-analysis', path: '/log/analysis',  nameKey: 'menu.log.analysis', icon: <LineChartOutlined /> },
    ],
  },
  {
    groupKey: 'alert',
    nameKey: 'menu.alert',
    items: [
      { key: 'alert-list',            path: '/alert/list',            nameKey: 'menu.alert.list',           icon: <AlertOutlined /> },
      { key: 'alert-analytics',       path: '/alert/analytics',       nameKey: 'menu.alert.analytics',      icon: <BarChartOutlined /> },
      { key: 'alert-rules',           path: '/alert/rules',           nameKey: 'menu.alert.rules',          icon: <ToolOutlined /> },
      { key: 'alert-baselines',       path: '/alert/baselines',       nameKey: 'menu.alert.baselines',      icon: <RadarChartOutlined /> },
      { key: 'alert-noise-reduction', path: '/alert/noise-reduction', nameKey: 'menu.alert.noiseReduction', icon: <FilterOutlined /> },
    ],
  },
  {
    groupKey: 'incident',
    nameKey: 'menu.incident',
    items: [
      { key: 'incident-list',      path: '/incident/list',       nameKey: 'menu.incident.list',      icon: <WarningOutlined /> },
      { key: 'incident-analytics', path: '/incident/analytics',  nameKey: 'menu.incident.analytics', icon: <BarChartOutlined /> },
      { key: 'incident-changes',   path: '/incident/changes',    nameKey: 'menu.incident.changes',   icon: <SwapOutlined /> },
      { key: 'incident-schedules', path: '/incident/schedules',  nameKey: 'menu.incident.schedules', icon: <ScheduleOutlined /> },
      { key: 'automation-workflows', path: '/automation/workflows', nameKey: 'menu.incident.workflows', icon: <ApartmentOutlined /> },
    ],
  },
  {
    groupKey: 'cmdb',
    nameKey: 'menu.cmdb',
    items: [
      { key: 'cmdb-assets',    path: '/cmdb/assets',    nameKey: 'menu.cmdb.assets',    icon: <DatabaseOutlined /> },
      { key: 'cmdb-overview',  path: '/cmdb/overview',  nameKey: 'menu.cmdb.overview',  icon: <EyeOutlined /> },
      { key: 'cmdb-groups',    path: '/cmdb/groups',    nameKey: 'menu.cmdb.groups',    icon: <TeamOutlined /> },
      { key: 'cmdb-discovery', path: '/cmdb/discovery', nameKey: 'menu.cmdb.discovery', icon: <ThunderboltOutlined /> },
    ],
  },
  {
    groupKey: 'analytics',
    nameKey: 'menu.analytics',
    items: [
      { key: 'analytics-sla',         path: '/analytics/sla',         nameKey: 'menu.analytics.sla',         icon: <ProfileOutlined /> },
      { key: 'analytics-correlation', path: '/analytics/correlation', nameKey: 'menu.analytics.correlation', icon: <RadarChartOutlined /> },
      { key: 'analytics-reports',     path: '/analytics/reports',     nameKey: 'menu.analytics.reports',     icon: <FileTextOutlined /> },
      { key: 'audit-analytics',       path: '/audit/analytics',       nameKey: 'menu.analytics.audit',       icon: <AuditOutlined /> },
      { key: 'analytics-knowledge',   path: '/analytics/knowledge',   nameKey: 'menu.analytics.knowledge',   icon: <BookOutlined /> },
    ],
  },
  {
    groupKey: 'notify',
    nameKey: 'menu.notify',
    items: [
      { key: 'notify-bots',    path: '/notify/bots',          nameKey: 'menu.notify.bots',      icon: <BellOutlined /> },
      { key: 'notify-logs',    path: '/notify/logs',          nameKey: 'menu.notify.logs',      icon: <FileTextOutlined /> },
      { key: 'webhooks',       path: '/settings/webhooks',    nameKey: 'menu.notify.webhooks',  icon: <ApiOutlined /> },
      { key: 'api-tokens',     path: '/settings/api-tokens',  nameKey: 'menu.notify.apiTokens', icon: <KeyOutlined /> },
      { key: 'api-docs',       path: '/api/docs',             nameKey: 'menu.notify.apiDocs',   icon: <BookOutlined /> },
    ],
  },
  {
    groupKey: 'settings',
    nameKey: 'menu.settings',
    items: [
      { key: 'settings-rbac',               path: '/settings/rbac',               nameKey: 'menu.settings.rbac',              icon: <SafetyCertificateOutlined /> },
      { key: 'settings-ldap',               path: '/settings/ldap',               nameKey: 'menu.settings.ldap',              icon: <ClusterOutlined /> },
      { key: 'settings-identity-providers', path: '/settings/identity-providers', nameKey: 'menu.settings.identityProviders', icon: <UserSwitchOutlined /> },
      { key: 'settings-security',           path: '/settings/security',           nameKey: 'menu.settings.security',          icon: <LockOutlined /> },
      { key: 'settings-ldap-config',        path: '/settings/ldap-config',        nameKey: 'menu.settings.ldapConfig',        icon: <ClusterOutlined /> },
      { key: 'settings-ai-models',          path: '/settings/ai-models',          nameKey: 'menu.settings.aiModels',          icon: <RobotOutlined /> },
      { key: 'settings-prompts',            path: '/settings/prompts',            nameKey: 'menu.settings.prompts',           icon: <EditOutlined /> },
      { key: 'settings-brand',              path: '/settings/brand',              nameKey: 'menu.settings.brand',             icon: <SkinOutlined /> },
      { key: 'settings-preferences',        path: '/settings/preferences',        nameKey: 'menu.settings.preferences',       icon: <SettingOutlined /> },
      { key: 'settings-suggestions',        path: '/settings/suggestions',        nameKey: 'menu.settings.suggestions',       icon: <BulbOutlined /> },
      { key: 'settings-system-health',      path: '/settings/system-health',      nameKey: 'menu.settings.systemHealth',      icon: <HeartOutlined /> },
      { key: 'settings-cluster',            path: '/settings/cluster',            nameKey: 'menu.settings.cluster',           icon: <CloudServerOutlined /> },
      { key: 'settings-version',            path: '/settings/version',            nameKey: 'menu.settings.version',           icon: <TagsOutlined /> },
      { key: 'settings-data-export',        path: '/settings/data-export',        nameKey: 'menu.settings.dataExport',        icon: <ExportOutlined /> },
    ],
  },
];

/* ─── 布局常量 ─── */

const HEADER_HEIGHT = 56;
const SIDEBAR_WIDTH = 220;
const SIDEBAR_COLLAPSED_WIDTH = 48;

const BasicLayout: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { t, i18n } = useTranslation();

  const { isDark, toggleTheme } = useTheme();
  const { density } = useDensity();

  const [collapsed, setCollapsed] = useState(false);
  const [wsConnected] = useState(true);

  /* ─── 全局字体大小调节 ─── */
  const FONT_SIZE_KEY = 'opsnexus-font-size';
  const FONT_SIZES = [
    { key: '12', label: '小' },
    { key: '13', label: '标准' },
    { key: '14', label: '大' },
    { key: '16', label: '特大' },
  ];
  const [globalFontSize, setGlobalFontSize] = useState<string>(() => {
    try { return localStorage.getItem(FONT_SIZE_KEY) || '13'; } catch { return '13'; }
  });
  React.useEffect(() => {
    document.documentElement.style.fontSize = `${globalFontSize}px`;
    try { localStorage.setItem(FONT_SIZE_KEY, globalFontSize); } catch {}
  }, [globalFontSize]);

  const sidebarWidth = collapsed ? SIDEBAR_COLLAPSED_WIDTH : SIDEBAR_WIDTH;
  const densityFontSize = density === 'compact' ? 12 : density === 'comfortable' ? 14 : 13;

  /** 判断菜单项是否激活 */
  const isActive = (path: string) => location.pathname === path || location.pathname.startsWith(path + '/');

  /** 所有菜单项扁平化，用于快速查找 */
  const allItems = useMemo(
    () => menuGroups.flatMap((g) => g.items),
    [],
  );

  /** 当前激活的菜单项 key */
  const activeKey = useMemo(() => {
    // 精确匹配优先，然后前缀匹配（取最长匹配）
    let best: MenuItem | undefined;
    for (const item of allItems) {
      if (location.pathname === item.path) return item.key;
      if (location.pathname.startsWith(item.path + '/')) {
        if (!best || item.path.length > best.path.length) best = item;
      }
    }
    return best?.key;
  }, [location.pathname, allItems]);

  return (
    <ConfigProvider
      theme={{
        algorithm: isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          colorPrimary: isDark ? '#4da6ff' : '#2563eb',
          colorBgBase: isDark ? '#060a12' : '#f4f7fc',
          fontFamily: 'Inter, "Noto Sans SC", -apple-system, sans-serif',
          fontSize: densityFontSize,
          borderRadius: 6,
        },
      }}
    >
      {/* WebSocket 断线提示条 */}
      <ConnectionIndicator connected={wsConnected} />

      {/* 粒子网络背景 */}
      <React.Suspense fallback={null}>
        <div style={{ position: 'fixed', inset: 0, zIndex: 0, pointerEvents: 'none' }}>
          <LazyParticleCanvas isDark={isDark} />
        </div>
      </React.Suspense>

      {/* ─── Header ─── */}
      <header
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          zIndex: 100,
          height: HEADER_HEIGHT,
          background: 'var(--header-bg)',
          borderBottom: '1px solid var(--header-border)',
          display: 'flex',
          alignItems: 'center',
          padding: '0 16px',
          gap: 16,
          transition: 'all 0.3s ease',
        }}
      >
        {/* Logo */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 2,
            cursor: 'pointer',
            flexShrink: 0,
            minWidth: collapsed ? SIDEBAR_COLLAPSED_WIDTH - 16 : SIDEBAR_WIDTH - 16,
            transition: 'min-width 0.3s ease',
          }}
          onClick={() => navigate('/home')}
        >
          <span
            style={{
              fontSize: 20,
              fontWeight: 700,
              color: isDark ? '#4da6ff' : '#2563eb',
              letterSpacing: -0.5,
            }}
          >
            OpsNexus
          </span>
          <span
            style={{
              fontSize: 20,
              fontWeight: 700,
              color: isDark ? '#4da6ff' : '#2563eb',
            }}
          >
            {'\u00b7'}
          </span>
        </div>

        {/* 全局搜索 */}
        <div style={{ flex: 1, display: 'flex', justifyContent: 'center' }}>
          <Input
            prefix={<SearchOutlined style={{ color: 'var(--text-secondary)' }} />}
            placeholder={t('header.globalSearch', '全局搜索 Cmd+K')}
            style={{
              width: 320,
              borderRadius: 6,
              background: 'var(--bg-card)',
              borderColor: 'var(--border-color)',
            }}
            allowClear
          />
        </div>

        {/* 右侧操作区 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 }}>
          {/* 通知铃铛 */}
          <NotificationBell />

          {/* 主题切换 */}
          <Tooltip title={isDark ? t('header.switchToLight', '切换到浅色') : t('header.switchToDark', '切换到深色')}>
            <Button
              type="text"
              size="small"
              icon={isDark ? <SunOutlined /> : <MoonFilled />}
              onClick={toggleTheme}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: 'var(--text-primary)',
              }}
            />
          </Tooltip>

          {/* 全局字体大小调节 */}
          <Dropdown
            trigger={['click']}
            menu={{
              items: FONT_SIZES.map((fs) => ({
                key: fs.key,
                label: `${globalFontSize === fs.key ? '✓ ' : '   '}${fs.label}`,
              })),
              onClick: ({ key }) => setGlobalFontSize(key),
            }}
            placement="bottomRight"
          >
            <Tooltip title={t('header.fontSize', '字体大小')}>
              <Button
                type="text"
                size="small"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: 'var(--text-primary)',
                  fontSize: 13,
                  fontWeight: 700,
                  minWidth: 28,
                }}
              >
                A
              </Button>
            </Tooltip>
          </Dropdown>

          {/* 语言切换 */}
          <Dropdown
            trigger={['click']}
            menu={{
              items: [
                { key: 'zh', label: '中文' },
                { key: 'en', label: 'English' },
              ],
              onClick: ({ key }) => i18n.changeLanguage(key),
            }}
          >
            <Button
              type="text"
              size="small"
              icon={<GlobalOutlined />}
              style={{ color: 'var(--text-primary)' }}
            >
              {i18n.language === 'zh' ? '中文' : 'EN'}
            </Button>
          </Dropdown>

          {/* 用户头像 */}
          <Dropdown
            menu={{
              items: [
                { key: 'profile', label: t('header.profile', '个人中心') },
                { key: 'preferences', label: t('header.preferences', '偏好设置'), onClick: () => navigate('/settings/preferences') },
                { type: 'divider' },
                { key: 'logout', label: t('header.logout', '退出登录') },
              ],
            }}
          >
            <Avatar
              size="small"
              icon={<UserOutlined />}
              style={{ cursor: 'pointer', flexShrink: 0 }}
            />
          </Dropdown>
        </div>
      </header>

      {/* ─── Sidebar ─── */}
      <aside
        style={{
          position: 'fixed',
          top: HEADER_HEIGHT,
          left: 0,
          bottom: 0,
          width: sidebarWidth,
          background: 'var(--sidebar-bg)',
          borderRight: '1px solid var(--border-color)',
          zIndex: 90,
          display: 'flex',
          flexDirection: 'column',
          transition: 'width 0.3s ease',
          overflow: 'hidden',
        }}
      >
        {/* 菜单滚动区 */}
        <nav
          className="shell-sidebar-nav"
          style={{
            flex: 1,
            overflowY: 'auto',
            overflowX: 'hidden',
            padding: collapsed ? '8px 4px' : '8px',
          }}
        >
          {menuGroups.map((group) => (
            <div key={group.groupKey} style={{ marginBottom: 8 }}>
              {/* 分组标题 */}
              {!collapsed && (
                <div
                  style={{
                    padding: '8px 12px 4px',
                    fontSize: 11,
                    fontWeight: 600,
                    color: 'var(--text-tertiary)',
                    textTransform: 'uppercase',
                    letterSpacing: 0.5,
                    whiteSpace: 'nowrap',
                    overflow: 'hidden',
                  }}
                >
                  {t(group.nameKey)}
                </div>
              )}

              {/* 菜单项 */}
              {group.items.map((item) => {
                const active = activeKey === item.key;
                return (
                  <Tooltip
                    key={item.key}
                    title={collapsed ? t(item.nameKey) : undefined}
                    placement="right"
                  >
                    <div
                      onClick={() => navigate(item.path)}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 10,
                        height: 40,
                        padding: collapsed ? '0 12px' : '0 12px',
                        margin: collapsed ? '2px 0' : '2px 0',
                        borderRadius: 6,
                        cursor: 'pointer',
                        color: active ? 'var(--color-primary)' : 'var(--text-primary)',
                        background: active
                          ? (isDark ? 'rgba(77, 166, 255, 0.1)' : 'rgba(37, 99, 235, 0.08)')
                          : 'transparent',
                        fontWeight: active ? 600 : 400,
                        fontSize: 13,
                        whiteSpace: 'nowrap',
                        overflow: 'hidden',
                        transition: 'all 0.2s ease',
                        justifyContent: collapsed ? 'center' : 'flex-start',
                      }}
                      onMouseEnter={(e) => {
                        if (!active) {
                          e.currentTarget.style.background = isDark
                            ? 'rgba(255,255,255,0.06)'
                            : 'rgba(0,0,0,0.04)';
                        }
                      }}
                      onMouseLeave={(e) => {
                        if (!active) {
                          e.currentTarget.style.background = 'transparent';
                        }
                      }}
                    >
                      <span style={{ fontSize: 16, flexShrink: 0, display: 'flex', alignItems: 'center' }}>
                        {item.icon}
                      </span>
                      {!collapsed && (
                        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>
                          {t(item.nameKey)}
                        </span>
                      )}
                    </div>
                  </Tooltip>
                );
              })}
            </div>
          ))}
        </nav>

        {/* 折叠/展开按钮 */}
        <div
          style={{
            borderTop: '1px solid var(--border-color)',
            padding: '8px',
            display: 'flex',
            justifyContent: collapsed ? 'center' : 'flex-end',
          }}
        >
          <Button
            type="text"
            size="small"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed(!collapsed)}
            style={{ color: 'var(--text-secondary)' }}
          />
        </div>
      </aside>

      {/* ─── Content Area ─── */}
      <main
        style={{
          marginTop: HEADER_HEIGHT,
          marginLeft: sidebarWidth,
          minHeight: `calc(100vh - ${HEADER_HEIGHT}px)`,
          background: 'var(--bg-primary)',
          position: 'relative',
          zIndex: 1,
          transition: 'margin-left 0.3s ease',
        }}
      >
        <div id="micro-app-container" style={{ minHeight: `calc(100vh - ${HEADER_HEIGHT}px)` }} />
      </main>
    </ConfigProvider>
  );
};

export default BasicLayout;
