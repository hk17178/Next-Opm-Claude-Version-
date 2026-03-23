import React, { useRef, useEffect } from 'react';

interface ChartCardProps {
  title: React.ReactNode;
  extra?: React.ReactNode;
  height?: number | string;
  option: Record<string, unknown>;
  style?: React.CSSProperties;
  className?: string;
  onFullscreen?: () => void;
}

const FullscreenIcon: React.FC<{ onClick?: () => void }> = ({ onClick }) => (
  <svg
    onClick={onClick}
    width="16"
    height="16"
    viewBox="0 0 16 16"
    fill="none"
    style={{ cursor: 'pointer', opacity: 0.65 }}
  >
    <path
      d="M2 6V2h4M10 2h4v4M14 10v4h-4M6 14H2v-4"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
);

export const ChartCard: React.FC<ChartCardProps> = ({
  title,
  extra,
  height = 240,
  option,
  style,
  className = '',
  onFullscreen,
}) => {
  const chartRef = useRef<HTMLDivElement>(null);
  const chartInstanceRef = useRef<{ setOption: (o: unknown, opts?: unknown) => void; resize: () => void; dispose: () => void } | null>(null);

  useEffect(() => {
    const el = chartRef.current;
    if (!el) return;

    let chart: typeof chartInstanceRef.current = null;

    try {
      // eslint-disable-next-line @typescript-eslint/no-require-imports
      const echarts = require('echarts');
      chart = echarts.init(el);
      chartInstanceRef.current = chart;
      chart!.setOption(option);
    } catch {
      return;
    }

    const ro = new ResizeObserver(() => {
      chart?.resize();
    });
    ro.observe(el);

    return () => {
      ro.disconnect();
      chart?.dispose();
      chartInstanceRef.current = null;
    };
  }, []);

  useEffect(() => {
    chartInstanceRef.current?.setOption(option, { notMerge: true });
  }, [option]);

  return (
    <div
      className={`uikit-chart-card ${className}`}
      style={{
        background: 'var(--bg-card)',
        border: '1px solid var(--border-color)',
        borderRadius: 12,
        overflow: 'hidden',
        ...style,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '12px 16px 8px',
        }}
      >
        <div style={{ fontWeight: 600, fontSize: 14 }}>{title}</div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {extra}
          {onFullscreen && <FullscreenIcon onClick={onFullscreen} />}
        </div>
      </div>
      <div
        ref={chartRef}
        style={{
          height: typeof height === 'number' ? `${height}px` : height,
          padding: '0 8px 8px',
        }}
      />
    </div>
  );
};
