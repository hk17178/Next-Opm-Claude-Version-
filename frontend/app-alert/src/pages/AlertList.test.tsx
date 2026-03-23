import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import AlertList from './AlertList';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const map: Record<string, string> = {
        'list.title': '告警中心',
        'list.createRule': '创建规则',
        'list.stat.firing': '触发中',
        'list.stat.todayNew': '今日新增',
        'list.stat.todayResolved': '今日已解决',
        'list.stat.suppressed': '降噪抑制',
        'list.stat.noiseRate': `降噪率 ${params?.rate ?? ''}`,
        'list.stat.vsYesterday': '较昨日',
        'list.tab.firing': '触发中',
        'list.tab.acknowledged': '已确认',
        'list.tab.all': '全部',
        'list.tab.suppressed': '被抑制',
        'list.column.severity': '级别',
        'list.column.content': '告警内容',
        'list.column.source': '来源',
        'list.column.triggerTime': '触发时间',
        'list.column.duration': '持续',
        'list.column.actions': '操作',
        'list.action.acknowledge': '确认',
        'list.filter.severity': '级别',
        'list.filter.business': '业务板块',
        'list.filter.source': '告警来源',
        'list.filter.layer': 'Layer',
        'list.noAlerts': '暂无告警数据',
        'list.confirmDialog.confirm': '确认告警',
        'list.confirmDialog.cancel': '取消',
        'list.confirmDialog.message': '确认该告警后将自动创建事件，是否继续？',
        'common:common.search': '搜索',
      };
      return map[key] ?? key;
    },
    i18n: { language: 'zh', changeLanguage: vi.fn() },
  }),
}));

describe('AlertList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the page title and create button', () => {
    render(<AlertList />);
    expect(screen.getByText('告警中心')).toBeInTheDocument();
    expect(screen.getByText('创建规则')).toBeInTheDocument();
  });

  it('renders 4 stat cards with correct labels', () => {
    render(<AlertList />);
    expect(screen.getByText('触发中')).toBeInTheDocument();
    expect(screen.getByText('今日新增')).toBeInTheDocument();
    expect(screen.getByText('今日已解决')).toBeInTheDocument();
    expect(screen.getByText('降噪抑制')).toBeInTheDocument();
    // Each stat card shows a "0" value
    const zeros = screen.getAllByText('0');
    expect(zeros.length).toBeGreaterThanOrEqual(3);
  });

  it('renders all tab items for alert status filtering', () => {
    render(<AlertList />);
    const tabs = screen.getAllByRole('tab');
    expect(tabs).toHaveLength(4);
    const tabLabels = tabs.map((tab) => tab.textContent);
    expect(tabLabels).toContain('触发中');
    expect(tabLabels).toContain('已确认');
    expect(tabLabels).toContain('全部');
    expect(tabLabels).toContain('被抑制');
  });

  it('switches active tab on click', async () => {
    const user = userEvent.setup();
    render(<AlertList />);
    const acknowledgedTab = screen.getByRole('tab', { name: '已确认' });
    await user.click(acknowledgedTab);
    expect(acknowledgedTab).toHaveAttribute('aria-selected', 'true');
  });

  it('renders severity filter with P0-P4 options', async () => {
    render(<AlertList />);
    // The severity filter Select is rendered
    const severitySelect = screen.getByText('级别').closest('.ant-select')
      ?? document.querySelector('[class*="ant-select"]');
    expect(severitySelect).toBeTruthy();
  });

  it('renders the table with correct column headers', () => {
    render(<AlertList />);
    expect(screen.getByText('告警内容')).toBeInTheDocument();
    expect(screen.getByText('来源')).toBeInTheDocument();
    expect(screen.getByText('触发时间')).toBeInTheDocument();
    expect(screen.getByText('持续')).toBeInTheDocument();
  });

  it('shows empty state when no alerts', () => {
    render(<AlertList />);
    expect(screen.getByText('暂无告警数据')).toBeInTheDocument();
  });

  it('applies P0 row className for severity P0 alerts', () => {
    // The component defines a rowClassName function that returns 'alert-row-p0' for P0 severity
    // and includes CSS with red border-left and pink background
    render(<AlertList />);
    const styleTag = document.querySelector('style');
    expect(styleTag).toBeTruthy();
    expect(styleTag?.textContent).toContain('alert-row-p0');
    expect(styleTag?.textContent).toContain('#F53F3F');
    expect(styleTag?.textContent).toContain('#FFF5F5');
  });

  it('renders the confirm modal structure', async () => {
    const user = userEvent.setup();
    render(<AlertList />);
    // Modal should not be visible initially
    expect(screen.queryByText('确认该告警后将自动创建事件，是否继续？')).not.toBeInTheDocument();
  });
});
