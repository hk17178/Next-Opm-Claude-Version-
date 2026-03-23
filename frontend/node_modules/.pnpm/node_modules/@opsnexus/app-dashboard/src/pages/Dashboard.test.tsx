import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import Dashboard from './Dashboard';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'status.title': 'OpsNexus NOC',
        'status.activeAlerts': '活跃告警',
        'status.activeIncidents': '活跃事件',
        'status.normal': '正常',
        'card.alertsToday': '今日告警',
        'card.incidentsToday': '今日事件',
        'card.resolved': '已解决',
        'card.avgMTTR': '平均MTTR',
        'alertWaterfall': '告警瀑布流',
        'businessHealth': '业务健康矩阵',
        'resourceCurves': '资源趋势曲线',
        'miniCockpit': '事件驾驶舱精简版',
        'placeholder.alertStream': '实时告警流待集成 (WebSocket)',
        'placeholder.healthMatrix': '业务健康矩阵待集成 (ECharts)',
        'placeholder.resourceCharts': '资源曲线图待集成 (ECharts)',
        'placeholder.cockpitMini': '驾驶舱精简版待集成',
      };
      return map[key] ?? key;
    },
    i18n: { language: 'zh', changeLanguage: vi.fn() },
  }),
}));

describe('Dashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders with dark background (#141414)', () => {
    const { container } = render(<Dashboard />);
    const root = container.firstElementChild as HTMLElement;
    expect(root.style.background).toBe('rgb(20, 20, 20)');
  });

  it('renders 4 stat cards with correct labels', () => {
    render(<Dashboard />);
    expect(screen.getByText('今日告警')).toBeInTheDocument();
    expect(screen.getByText('今日事件')).toBeInTheDocument();
    expect(screen.getByText('已解决')).toBeInTheDocument();
    expect(screen.getByText('平均MTTR')).toBeInTheDocument();
  });

  it('renders alert waterfall section', () => {
    render(<Dashboard />);
    expect(screen.getByText('告警瀑布流')).toBeInTheDocument();
    expect(screen.getByText('实时告警流待集成 (WebSocket)')).toBeInTheDocument();
  });

  it('renders business health matrix section', () => {
    render(<Dashboard />);
    expect(screen.getByText('业务健康矩阵')).toBeInTheDocument();
    expect(screen.getByText('业务健康矩阵待集成 (ECharts)')).toBeInTheDocument();
  });

  it('renders global status bar with NOC title and indicators', () => {
    render(<Dashboard />);
    expect(screen.getByText('OpsNexus NOC')).toBeInTheDocument();
    expect(screen.getByText('活跃告警:')).toBeInTheDocument();
    expect(screen.getByText('活跃事件:')).toBeInTheDocument();
    expect(screen.getByText('正常')).toBeInTheDocument();
  });

  it('renders resource curves and mini cockpit sections', () => {
    render(<Dashboard />);
    expect(screen.getByText('资源趋势曲线')).toBeInTheDocument();
    expect(screen.getByText('事件驾驶舱精简版')).toBeInTheDocument();
  });
});
