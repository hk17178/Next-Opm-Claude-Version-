import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import LogSearch from './LogSearch';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const map: Record<string, string> = {
        'search.title': '日志检索',
        'search.export': '导出',
        'search.saveQuery': '保存查询',
        'search.noResults': '暂无日志数据',
        'search.resultCount': `找到 ${params?.count ?? 0} 条结果 (耗时 ${params?.time ?? '0.0'}s)`,
        'search.placeholder': '输入 Lucene 查询语法搜索日志...',
        'search.timePreset': '时间范围',
        'search.column.timestamp': '时间戳',
        'search.column.level': '级别',
        'search.column.host': '主机名',
        'search.column.service': '服务名',
        'search.column.message': '消息',
        'search.filter.sourceType': '来源类型',
        'search.filter.host': '主机名',
        'search.filter.service': '服务名',
        'search.filter.level': '日志级别',
        'search.context': `查看上下文 (前后${params?.count ?? 50}条)`,
        'search.copy': '复制',
        'search.relatedAlert': '关联告警搜索',
        'search.detail.fullMessage': '完整消息',
        'search.detail.fields': '字段',
        'search.exportDialog.title': '导出日志',
        'search.exportDialog.format': '导出格式',
        'search.exportDialog.maxRows': '最大行数',
        'common:common.search': '搜索',
      };
      return map[key] ?? key;
    },
    i18n: { language: 'zh', changeLanguage: vi.fn() },
  }),
}));

describe('LogSearch', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the page title and action buttons', () => {
    render(<LogSearch />);
    expect(screen.getByText('日志检索')).toBeInTheDocument();
    expect(screen.getByText('导出')).toBeInTheDocument();
    expect(screen.getByText('保存查询')).toBeInTheDocument();
  });

  it('renders the search input with Lucene placeholder', () => {
    render(<LogSearch />);
    const searchInput = screen.getByPlaceholderText('输入 Lucene 查询语法搜索日志...');
    expect(searchInput).toBeInTheDocument();
  });

  it('renders time preset select with quick options', () => {
    render(<LogSearch />);
    // The time preset select is rendered with placeholder
    const timePresetPlaceholder = screen.getByText('时间范围');
    expect(timePresetPlaceholder).toBeInTheDocument();
  });

  it('handles search input value changes', async () => {
    const user = userEvent.setup();
    render(<LogSearch />);
    const searchInput = screen.getByPlaceholderText('输入 Lucene 查询语法搜索日志...');
    await user.type(searchInput, 'error AND host:web-01');
    expect(searchInput).toHaveValue('error AND host:web-01');
  });

  it('renders the table with correct column headers', () => {
    render(<LogSearch />);
    expect(screen.getByText('时间戳')).toBeInTheDocument();
    // "级别" appears in both the column header and a filter select
    const levelTexts = screen.getAllByText('级别');
    expect(levelTexts.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('消息')).toBeInTheDocument();
  });

  it('renders filter selects for source type, host, service, and level', () => {
    render(<LogSearch />);
    expect(screen.getByText('来源类型')).toBeInTheDocument();
    // Multiple "主机名" elements (column header + filter)
    const hostTexts = screen.getAllByText('主机名');
    expect(hostTexts.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('日志级别')).toBeInTheDocument();
  });

  it('shows result count message', () => {
    render(<LogSearch />);
    expect(screen.getByText(/找到 0 条结果/)).toBeInTheDocument();
  });

  it('shows empty state when no log data', () => {
    render(<LogSearch />);
    expect(screen.getByText('暂无日志数据')).toBeInTheDocument();
  });

  it('opens export modal on export button click', async () => {
    const user = userEvent.setup();
    render(<LogSearch />);
    const exportBtn = screen.getByText('导出');
    await user.click(exportBtn);
    expect(screen.getByText('导出日志')).toBeInTheDocument();
    expect(screen.getByText('导出格式')).toBeInTheDocument();
    expect(screen.getByText('最大行数')).toBeInTheDocument();
  });

  it('renders CSV and JSON radio options in export modal', async () => {
    const user = userEvent.setup();
    render(<LogSearch />);
    await user.click(screen.getByText('导出'));
    expect(screen.getByText('CSV')).toBeInTheDocument();
    expect(screen.getByText('JSON')).toBeInTheDocument();
  });
});
