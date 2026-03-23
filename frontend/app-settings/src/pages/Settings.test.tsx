import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import Settings from './Settings';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'ai.title': 'AI 模型管理',
        'ai.registerModel': '注册新模型',
        'ai.modelList': '模型列表',
        'ai.noModels': '暂无模型数据',
        'ai.column.name': '模型名称',
        'ai.column.provider': '供应商',
        'ai.column.status': '状态',
        'ai.column.todayCalls': '今日调用',
        'ai.column.todayTokens': '今日Token',
        'ai.column.actions': '操作',
        'ai.edit': '编辑',
        'ai.status.active': '正常',
        'ai.status.standby': '备用',
        'ai.status.error': '异常',
        'ai.scenarioBinding': '场景绑定',
        'ai.noScenarios': '暂无场景数据',
        'ai.scenario.name': '场景',
        'ai.scenario.primaryModel': '主模型',
        'ai.scenario.backupModel': '备模型',
        'ai.scenario.promptVersion': 'Prompt 版本',
        'ai.scenario.approvalRate': '好评率',
        'ai.usageMonitor': '用量监控',
        'ai.usagePlaceholder': 'ECharts 折线图 + 饼图待集成',
      };
      return map[key] ?? key;
    },
    i18n: { language: 'zh', changeLanguage: vi.fn() },
  }),
}));

describe('Settings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page title and register model button', () => {
    render(<Settings />);
    expect(screen.getByText('AI 模型管理')).toBeInTheDocument();
    expect(screen.getByText('注册新模型')).toBeInTheDocument();
  });

  it('renders model list table with correct column headers', () => {
    render(<Settings />);
    expect(screen.getByText('模型列表')).toBeInTheDocument();
    expect(screen.getByText('模型名称')).toBeInTheDocument();
    expect(screen.getByText('供应商')).toBeInTheDocument();
    expect(screen.getByText('今日调用')).toBeInTheDocument();
    expect(screen.getByText('今日Token')).toBeInTheDocument();
  });

  it('renders scenario binding table with correct column headers', () => {
    render(<Settings />);
    expect(screen.getByText('场景绑定')).toBeInTheDocument();
    expect(screen.getByText('场景')).toBeInTheDocument();
    expect(screen.getByText('主模型')).toBeInTheDocument();
    expect(screen.getByText('备模型')).toBeInTheDocument();
    expect(screen.getByText('Prompt 版本')).toBeInTheDocument();
    expect(screen.getByText('好评率')).toBeInTheDocument();
  });

  it('renders usage monitor section with placeholder', () => {
    render(<Settings />);
    expect(screen.getByText('用量监控')).toBeInTheDocument();
    expect(screen.getByText('ECharts 折线图 + 饼图待集成')).toBeInTheDocument();
  });

  it('shows empty state for both tables', () => {
    render(<Settings />);
    expect(screen.getByText('暂无模型数据')).toBeInTheDocument();
    expect(screen.getByText('暂无场景数据')).toBeInTheDocument();
  });
});
