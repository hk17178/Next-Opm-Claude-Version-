import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { RootCauseBadge } from './RootCauseBadge';
import type { RootCauseCategory } from '../types';

describe('RootCauseBadge', () => {
  it('renders the label text', () => {
    render(<RootCauseBadge category="human_action" label="Human Action" />);
    expect(screen.getByText('Human Action')).toBeInTheDocument();
  });

  it('renders all root cause categories without error', () => {
    const categories: RootCauseCategory[] = [
      'human_action', 'system_fault', 'change_induced', 'external_dependency', 'pending',
    ];
    categories.forEach((category) => {
      const { unmount } = render(<RootCauseBadge category={category} label={category} />);
      expect(screen.getByText(category)).toBeInTheDocument();
      unmount();
    });
  });

  it('renders as an Ant Design Tag element', () => {
    const { container } = render(<RootCauseBadge category="system_fault" label="System Fault" />);
    const tag = container.querySelector('.ant-tag');
    expect(tag).toBeTruthy();
  });

  it('displays the label prop, not the category key', () => {
    render(<RootCauseBadge category="change_induced" label="变更引发" />);
    expect(screen.getByText('变更引发')).toBeInTheDocument();
    expect(screen.queryByText('change_induced')).not.toBeInTheDocument();
  });

  it('renders with different label for same category', () => {
    const { rerender } = render(<RootCauseBadge category="pending" label="Pending" />);
    expect(screen.getByText('Pending')).toBeInTheDocument();

    rerender(<RootCauseBadge category="pending" label="待定" />);
    expect(screen.getByText('待定')).toBeInTheDocument();
  });
});
