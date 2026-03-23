import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import MetricsReport from './MetricsReport';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'metrics.title': '关联分析',
        'metrics.tab.correlation': '告警-资产关联',
        'metrics.tab.transaction': '链路分析',
        'metrics.filter.period': '时间范围',
        'metrics.filter.24h': '24小时',
        'metrics.filter.7d': '7天',
        'metrics.filter.30d': '30天',
        'metrics.filter.90d': '90天',
        'metrics.filter.business': '业务',
        'metrics.filter.assetType': '资产类型',
        'metrics.summary.totalAlerts': '总告警数',
        'metrics.summary.totalIncidents': '总事件数',
        'metrics.summary.topRisk': '最高风险',
        'metrics.summary.avgErrorRate': '平均错误率',
        'metrics.correlation.asset': '资产',
        'metrics.correlation.alertCount': '告警数',
        'metrics.correlation.incidentCount': '事件数',
        'metrics.correlation.avgMTTR': '平均MTTR',
        'metrics.correlation.riskScore': '风险分',
        'metrics.correlation.chartPlaceholder': '关联分析图待集成',
        'metrics.transaction.service': '服务',
        'metrics.transaction.qps': 'QPS',
        'metrics.transaction.p99': 'P99',
        'metrics.transaction.errorRate': '错误率',
        'metrics.transaction.trend': '趋势',
        'metrics.transaction.chartPlaceholder': '链路分析图待集成',
        'metrics.noData': '暂无数据',
      };
      return map[key] ?? key;
    },
    i18n: { language: 'zh', changeLanguage: vi.fn() },
  }),
}));

describe('MetricsReport', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page title and filter bar', () => {
    render(<MetricsReport />);
    expect(screen.getByText('关联分析')).toBeInTheDocument();
    expect(screen.getByText('7天')).toBeInTheDocument(); // default period value
    expect(screen.getByText('业务')).toBeInTheDocument();
    expect(screen.getByText('资产类型')).toBeInTheDocument();
  });

  it('renders 4 summary cards', () => {
    render(<MetricsReport />);
    expect(screen.getByText('总告警数')).toBeInTheDocument();
    expect(screen.getByText('总事件数')).toBeInTheDocument();
    expect(screen.getByText('最高风险')).toBeInTheDocument();
    expect(screen.getByText('平均错误率')).toBeInTheDocument();
  });

  it('renders correlation tab with table columns', () => {
    render(<MetricsReport />);
    expect(screen.getByText('告警-资产关联')).toBeInTheDocument();
    expect(screen.getByText('资产')).toBeInTheDocument();
    expect(screen.getByText('告警数')).toBeInTheDocument();
    expect(screen.getByText('风险分')).toBeInTheDocument();
  });

  it('renders tabs for correlation and transaction views', () => {
    render(<MetricsReport />);
    const tabs = screen.getAllByRole('tab');
    expect(tabs).toHaveLength(2);
    expect(tabs[0].textContent).toBe('告警-资产关联');
    expect(tabs[1].textContent).toBe('链路分析');
  });

  it('switches to transaction tab on click', async () => {
    const user = userEvent.setup();
    render(<MetricsReport />);
    const transactionTab = screen.getByRole('tab', { name: '链路分析' });
    await user.click(transactionTab);
    expect(transactionTab).toHaveAttribute('aria-selected', 'true');
  });
});
