import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import SLAReport from './SLAReport';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const map: Record<string, string> = {
        'sla.overview.sla': '综合 SLA',
        'sla.overview.target': `目标: ${params?.value ?? ''}`,
        'sla.overview.errorBudget': '错误预算剩余',
        'sla.overview.healthy': '健康',
        'sla.overview.incidents': '本月中断事件',
        'sla.overview.downtime': `总停机: ${params?.time ?? ''}`,
        'sla.filter.period': '时间范围',
        'sla.filter.week': '本周',
        'sla.filter.month': '本月',
        'sla.filter.quarter': '本季度',
        'sla.filter.year': '本年',
        'sla.filter.business': '业务',
        'sla.filter.tier': '层级',
        'sla.filter.grade': '分级',
        'sla.byBusiness': '按业务板块 SLA',
        'sla.table.business': '业务板块',
        'sla.table.target': '目标',
        'sla.table.status': '状态',
        'sla.table.errorBudget': '错误预算',
        'sla.table.downtime': '停机时间',
        'sla.status.met': '达标',
        'sla.status.nearMiss': '接近',
        'sla.trend': 'SLA 趋势图 (12 个月)',
        'sla.trendPlaceholder': 'ECharts 折线图待集成',
        'sla.noData': '暂无 SLA 数据',
      };
      return map[key] ?? key;
    },
    i18n: { language: 'zh', changeLanguage: vi.fn() },
  }),
}));

describe('SLAReport', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders filter bar with period, business, tier, and grade selects', () => {
    render(<SLAReport />);
    expect(screen.getByText('本月')).toBeInTheDocument(); // default value for period
    expect(screen.getByText('业务')).toBeInTheDocument();
    expect(screen.getByText('层级')).toBeInTheDocument();
    expect(screen.getByText('分级')).toBeInTheDocument();
  });

  it('renders 3 overview cards with correct labels', () => {
    render(<SLAReport />);
    expect(screen.getByText('综合 SLA')).toBeInTheDocument();
    expect(screen.getByText('错误预算剩余')).toBeInTheDocument();
    expect(screen.getByText('本月中断事件')).toBeInTheDocument();
  });

  it('renders overview card sub-text with target and health info', () => {
    render(<SLAReport />);
    expect(screen.getByText(/目标: 99.95%/)).toBeInTheDocument();
    expect(screen.getByText('健康')).toBeInTheDocument();
  });

  it('renders SLA by business table with correct column headers', () => {
    render(<SLAReport />);
    expect(screen.getByText('按业务板块 SLA')).toBeInTheDocument();
    expect(screen.getByText('业务板块')).toBeInTheDocument();
    expect(screen.getByText('错误预算')).toBeInTheDocument();
    expect(screen.getByText('停机时间')).toBeInTheDocument();
  });

  it('shows empty state for SLA table', () => {
    render(<SLAReport />);
    expect(screen.getByText('暂无 SLA 数据')).toBeInTheDocument();
  });

  it('renders trend chart section with placeholder', () => {
    render(<SLAReport />);
    expect(screen.getByText('SLA 趋势图 (12 个月)')).toBeInTheDocument();
    expect(screen.getByText('ECharts 折线图待集成')).toBeInTheDocument();
  });
});
