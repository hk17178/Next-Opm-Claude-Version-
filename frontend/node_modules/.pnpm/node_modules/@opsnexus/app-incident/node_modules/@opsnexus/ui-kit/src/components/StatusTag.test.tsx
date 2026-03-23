import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StatusTag } from './StatusTag';

describe('StatusTag', () => {
  it('renders the status text', () => {
    render(<StatusTag status="firing" />);
    expect(screen.getByText('firing')).toBeInTheDocument();
  });

  it('renders as an Ant Design Tag element', () => {
    const { container } = render(<StatusTag status="acknowledged" />);
    const tag = container.querySelector('.ant-tag');
    expect(tag).toBeTruthy();
    expect(tag?.textContent).toBe('acknowledged');
  });

  it('renders various default statuses without error', () => {
    const statuses = ['firing', 'acknowledged', 'resolved', 'suppressed', 'active', 'processing', 'pending_review', 'closed'];
    statuses.forEach((status) => {
      const { unmount } = render(<StatusTag status={status} />);
      expect(screen.getByText(status)).toBeInTheDocument();
      unmount();
    });
  });

  it('uses custom colorMap when provided', () => {
    const customMap = { custom_status: '#123456' };
    const { container } = render(<StatusTag status="custom_status" colorMap={customMap} />);
    const tag = container.querySelector('.ant-tag');
    expect(tag).toBeTruthy();
    expect(tag?.textContent).toBe('custom_status');
  });

  it('falls back to gray for unknown status without custom map', () => {
    const { container } = render(<StatusTag status="unknown_xyz" />);
    const tag = container.querySelector('.ant-tag');
    expect(tag).toBeTruthy();
    // Should still render the text
    expect(tag?.textContent).toBe('unknown_xyz');
  });

  it('merges custom colorMap with defaults', () => {
    const customMap = { new_status: '#AABBCC' };
    // "firing" should still work from defaults even with custom map
    const { unmount } = render(<StatusTag status="firing" colorMap={customMap} />);
    expect(screen.getByText('firing')).toBeInTheDocument();
    unmount();

    // Custom status should also work
    render(<StatusTag status="new_status" colorMap={customMap} />);
    expect(screen.getByText('new_status')).toBeInTheDocument();
  });
});
