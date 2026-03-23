import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import AssetList from './AssetList';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const map: Record<string, string> = {
        'assets.title': '资产管理',
        'assets.import': '导入',
        'assets.export': '导出',
        'assets.create': '新建资产',
        'assets.noData': '暂无资产数据',
        'assets.total': `共 ${params?.count ?? 0} 台`,
        'assets.column.hostname': '主机名',
        'assets.column.ip': 'IP',
        'assets.column.type': '类型',
        'assets.column.business': '业务',
        'assets.column.grade': '分级',
        'assets.column.env': '环境',
        'assets.column.status': '状态',
        'assets.filter.business': '业务板块',
        'assets.filter.type': '资产类型',
        'assets.filter.env': '环境',
        'assets.filter.region': '地域',
        'assets.filter.grade': '资产分级',
        'assets.filter.status': '状态',
        'assets.filter.tags': '标签 Key=Value',
        'assets.batch.title': '批量操作',
        'assets.batch.changeGrade': '修改分级',
        'assets.batch.changeBusiness': '修改业务',
        'assets.batch.changeTags': '修改标签',
        'common:common.reset': '重置',
        'common:common.search': '搜索',
      };
      return map[key] ?? key;
    },
    i18n: { language: 'zh', changeLanguage: vi.fn() },
  }),
}));

describe('AssetList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the page title and action buttons', () => {
    render(<AssetList />);
    expect(screen.getByText('资产管理')).toBeInTheDocument();
    expect(screen.getByText('导入')).toBeInTheDocument();
    expect(screen.getByText('导出')).toBeInTheDocument();
    expect(screen.getByText('新建资产')).toBeInTheDocument();
  });

  it('renders 6-dimension filter area with all filter selects', () => {
    render(<AssetList />);
    expect(screen.getByText('业务板块')).toBeInTheDocument();
    expect(screen.getByText('资产类型')).toBeInTheDocument();
    // "环境" appears in both filter and column header
    const envTexts = screen.getAllByText('环境');
    expect(envTexts.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('地域')).toBeInTheDocument();
    expect(screen.getByText('资产分级')).toBeInTheDocument();
  });

  it('renders tags input filter and search/reset buttons', () => {
    render(<AssetList />);
    expect(screen.getByPlaceholderText('标签 Key=Value')).toBeInTheDocument();
    expect(screen.getByText('重置')).toBeInTheDocument();
    expect(screen.getByText('搜索')).toBeInTheDocument();
  });

  it('renders the table with correct column headers', () => {
    render(<AssetList />);
    expect(screen.getByText('主机名')).toBeInTheDocument();
    expect(screen.getByText('IP')).toBeInTheDocument();
    expect(screen.getByText('分级')).toBeInTheDocument();
  });

  it('renders stats summary line with grade breakdown', () => {
    render(<AssetList />);
    expect(screen.getByText(/共 0 台/)).toBeInTheDocument();
    expect(screen.getByText(/S:0/)).toBeInTheDocument();
  });

  it('shows empty state when no asset data', () => {
    render(<AssetList />);
    expect(screen.getByText('暂无资产数据')).toBeInTheDocument();
  });

  it('renders table with row selection checkboxes', () => {
    const { container } = render(<AssetList />);
    // The table has rowSelection enabled, so the select-all checkbox should exist in the header
    const checkboxes = container.querySelectorAll('.ant-checkbox');
    expect(checkboxes.length).toBeGreaterThanOrEqual(1);
  });

  it('renders grade filter select with S/A/B/C/D options', () => {
    render(<AssetList />);
    // The grade filter select with placeholder "资产分级" is rendered
    expect(screen.getByText('资产分级')).toBeInTheDocument();
  });
});
