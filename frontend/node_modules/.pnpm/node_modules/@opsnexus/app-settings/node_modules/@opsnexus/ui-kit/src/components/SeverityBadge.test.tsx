import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SeverityBadge } from './SeverityBadge';
import { SEVERITY_COLORS } from '../theme';
import type { Severity } from '../types';

describe('SeverityBadge', () => {
  it('renders severity text inside the tag', () => {
    render(<SeverityBadge severity="P0" />);
    expect(screen.getByText('P0')).toBeInTheDocument();
  });

  it('renders all severity levels without error', () => {
    const severities: Severity[] = ['P0', 'P1', 'P2', 'P3', 'P4'];
    severities.forEach((severity) => {
      const { unmount } = render(<SeverityBadge severity={severity} />);
      expect(screen.getByText(severity)).toBeInTheDocument();
      unmount();
    });
  });

  it('renders as an Ant Design Tag element', () => {
    const { container } = render(<SeverityBadge severity="P1" />);
    const tag = container.querySelector('.ant-tag');
    expect(tag).toBeTruthy();
    expect(tag?.textContent).toBe('P1');
  });

  it('applies fontWeight 600 style for bold display', () => {
    const { container } = render(<SeverityBadge severity="P2" />);
    const tag = container.querySelector('.ant-tag');
    expect(tag).toBeTruthy();
    expect(tag?.style.fontWeight).toBe('600');
  });

  it('applies border-radius 4px style', () => {
    const { container } = render(<SeverityBadge severity="P3" />);
    const tag = container.querySelector('.ant-tag');
    expect(tag).toBeTruthy();
    expect(tag?.style.borderRadius).toBe('4px');
  });
});
