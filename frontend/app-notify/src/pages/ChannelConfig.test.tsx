import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ChannelConfig from './ChannelConfig';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const map: Record<string, string> = {
        'channel.title': '通知渠道配置',
        'channel.addChannel': '新建渠道',
        'channel.addTitle': '新建通知渠道',
        'channel.editTitle': '编辑通知渠道',
        'channel.deleteTitle': '删除渠道确认',
        'channel.deleteMessage': '确定要删除该通知渠道吗？',
        'channel.confirmDelete': '确认删除',
        'channel.save': '保存',
        'channel.cancel': '取消',
        'channel.noData': '暂无通知渠道',
        'channel.total': `共 ${params?.count ?? 0} 条`,
        'channel.type.wecom_webhook': '企微群 Webhook',
        'channel.type.wecom_app': '企微应用消息',
        'channel.type.sms': '短信',
        'channel.type.email': '邮件',
        'channel.type.voice_tts': '语音 TTS',
        'channel.type.webhook': '通用 Webhook',
        'channel.health.healthy': '健康',
        'channel.health.degraded': '降级',
        'channel.health.unavailable': '不可用',
        'channel.column.name': '渠道名称',
        'channel.column.type': '渠道类型',
        'channel.column.health': '健康状态',
        'channel.column.enabled': '启用',
        'channel.column.lastCheck': '最后检测时间',
        'channel.column.actions': '操作',
        'channel.action.edit': '编辑',
        'channel.action.test': '测试',
        'channel.action.delete': '删除',
        'channel.filter.type': '渠道类型',
        'channel.filter.health': '健康状态',
        'channel.filter.status': '启用状态',
        'channel.filter.enabled': '已启用',
        'channel.filter.disabled': '已禁用',
        'channel.form.name': '渠道名称',
        'channel.form.nameRequired': '请输入渠道名称',
        'channel.form.namePlaceholder': '输入渠道名称',
        'channel.form.type': '渠道类型',
        'channel.form.typeRequired': '请选择渠道类型',
        'channel.form.typePlaceholder': '选择渠道类型',
        'channel.form.description': '描述',
        'channel.form.descriptionPlaceholder': '输入渠道描述（可选）',
        'channel.form.enabled': '启用',
      };
      return map[key] ?? key;
    },
    i18n: { language: 'zh', changeLanguage: vi.fn() },
  }),
}));

describe('ChannelConfig', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page title and add channel button', () => {
    render(<ChannelConfig />);
    expect(screen.getByText('通知渠道配置')).toBeInTheDocument();
    expect(screen.getByText('新建渠道')).toBeInTheDocument();
  });

  it('renders filter bar with type, health, and status selects', () => {
    render(<ChannelConfig />);
    // Filter placeholders
    const typeTexts = screen.getAllByText('渠道类型');
    expect(typeTexts.length).toBeGreaterThanOrEqual(1); // filter + column header
    expect(screen.getByText('健康状态')).toBeInTheDocument();
    expect(screen.getByText('启用状态')).toBeInTheDocument();
  });

  it('renders table with correct column headers', () => {
    render(<ChannelConfig />);
    expect(screen.getByText('渠道名称')).toBeInTheDocument();
    expect(screen.getByText('最后检测时间')).toBeInTheDocument();
  });

  it('shows empty state when no channels', () => {
    render(<ChannelConfig />);
    expect(screen.getByText('暂无通知渠道')).toBeInTheDocument();
  });

  it('opens add channel modal on button click', async () => {
    const user = userEvent.setup();
    render(<ChannelConfig />);
    await user.click(screen.getByText('新建渠道'));
    expect(screen.getByText('新建通知渠道')).toBeInTheDocument();
    // Form fields
    expect(screen.getByText('描述')).toBeInTheDocument();
  });

  it('renders channel type options in add modal', async () => {
    const user = userEvent.setup();
    render(<ChannelConfig />);
    await user.click(screen.getByText('新建渠道'));
    // The form has a type select with placeholder
    expect(screen.getByText('选择渠道类型')).toBeInTheDocument();
  });
});
