import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MetricCard } from './MetricCard';

describe('MetricCard', () => {
  it('renders label and value', () => {
    render(<MetricCard label="Active Alerts" value={42} />);
    expect(screen.getByText('Active Alerts')).toBeInTheDocument();
    expect(screen.getByText('42')).toBeInTheDocument();
  });

  it('renders string values', () => {
    render(<MetricCard label="MTTR" value="12min" />);
    expect(screen.getByText('MTTR')).toBeInTheDocument();
    expect(screen.getByText('12min')).toBeInTheDocument();
  });

  it('renders as an Ant Design Card element', () => {
    const { container } = render(<MetricCard label="Test" value={0} />);
    const card = container.querySelector('.ant-card');
    expect(card).toBeTruthy();
  });

  it('renders trend indicator when trend prop is provided', () => {
    const { container } = render(
      <MetricCard label="Alerts" value={10} trend={3} trendDirection="up" />,
    );
    // Trend value is displayed
    expect(screen.getByText(/3/)).toBeInTheDocument();
    // Up arrow icon should be present
    const icon = container.querySelector('.anticon-arrow-up');
    expect(icon).toBeTruthy();
  });

  it('renders down trend with correct icon', () => {
    const { container } = render(
      <MetricCard label="Errors" value={5} trend={2} trendDirection="down" />,
    );
    const icon = container.querySelector('.anticon-arrow-down');
    expect(icon).toBeTruthy();
  });

  it('does not render trend when trend prop is omitted', () => {
    const { container } = render(<MetricCard label="SLA" value="99.9%" />);
    const upIcon = container.querySelector('.anticon-arrow-up');
    const downIcon = container.querySelector('.anticon-arrow-down');
    expect(upIcon).toBeNull();
    expect(downIcon).toBeNull();
  });

  it('renders suffix when provided', () => {
    render(<MetricCard label="Uptime" value={99} suffix="%" />);
    expect(screen.getByText('%')).toBeInTheDocument();
  });
});
