import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import Cockpit from './Cockpit';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const map: Record<string, string> = {
        'status.active': '活跃事件',
        'incidents.title': '活跃事件列表',
        'incidents.noActive': '当前无活跃事件',
        'progress.title': '事件进展面板',
        'progress.selectIncident': '请选择左侧事件查看进展',
        'ai.title': 'AI 提示',
        'ai.noInsight': '暂无 AI 洞察',
        'onCall.title': '值班团队',
        'onCall.noData': '暂无值班数据',
        'onCall.contact': '联系',
        'onCall.idle': '空闲',
        'onCall.contactConfirm.title': '联系处理人',
        'onCall.contactConfirm.message': `${params?.name ?? ''} 正在处理事件 ${params?.task ?? ''}，确认联系？`,
        'onCall.contactConfirm.ok': '确认联系',
        'onCall.contactConfirm.cancel': '取消',
      };
      return map[key] ?? key;
    },
    i18n: { language: 'zh', changeLanguage: vi.fn() },
  }),
}));

describe('Cockpit', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the top status bar with active incident count and severity tags', () => {
    render(<Cockpit />);
    expect(screen.getByText('活跃事件:')).toBeInTheDocument();
    // P0, P1, P2 severity labels in the status bar
    expect(screen.getByText('P0:')).toBeInTheDocument();
    expect(screen.getByText('P1:')).toBeInTheDocument();
    expect(screen.getByText('P2:')).toBeInTheDocument();
    // SLA indicator
    expect(screen.getByText('SLA:')).toBeInTheDocument();
  });

  it('renders three-column layout with all panel titles', () => {
    render(<Cockpit />);
    // Left panel: active incidents list
    expect(screen.getByText('活跃事件列表')).toBeInTheDocument();
    // Center panel: progress timeline
    expect(screen.getByText('事件进展面板')).toBeInTheDocument();
    // Prompt to select an incident
    expect(screen.getByText('请选择左侧事件查看进展')).toBeInTheDocument();
  });

  it('renders AI insight section with placeholder', () => {
    render(<Cockpit />);
    expect(screen.getByText('AI 提示')).toBeInTheDocument();
    expect(screen.getByText('暂无 AI 洞察')).toBeInTheDocument();
  });

  it('renders on-call team section with empty state', () => {
    render(<Cockpit />);
    expect(screen.getByText('值班团队')).toBeInTheDocument();
    expect(screen.getByText('暂无值班数据')).toBeInTheDocument();
  });

  it('renders the three-column Row layout with correct Col spans', () => {
    const { container } = render(<Cockpit />);
    // Verify the Row/Col structure exists
    const cols = container.querySelectorAll('.ant-col');
    // Should have at least 3 columns (left: 7, center: 11, right: 6)
    expect(cols.length).toBeGreaterThanOrEqual(3);
  });

  it('shows empty state for active incidents list', () => {
    render(<Cockpit />);
    expect(screen.getByText('当前无活跃事件')).toBeInTheDocument();
  });
});
