import React from 'react';
import { FlipCard } from './FlipCard';
import { injectGlobalStyles } from '../utils/injectGlobalStyles';

export interface HealthCell {
  id: string;
  label: string;
  status: 'ok' | 'degraded' | 'critical' | 'unknown';
  details?: {
    cpu?: number;
    mem?: number;
    disk?: number;
    conn?: number;
  };
}

interface HealthMatrixProps {
  cells: HealthCell[];
  rows?: number;
  cols?: number;
  cellHeight?: number;
}

const STATUS_COLORS: Record<HealthCell['status'], string> = {
  ok: 'var(--color-success)',
  degraded: 'var(--color-warning)',
  critical: 'var(--color-danger)',
  unknown: 'rgba(140,170,210,0.3)',
};

injectGlobalStyles('uikit-health-pulse-keyframes', `
@keyframes uikit-health-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.55; }
}
@media (prefers-reduced-motion: reduce) {
  .uikit-health-critical { animation: none !important; }
}
`);

const CellFront: React.FC<{ cell: HealthCell; height: number }> = ({ cell, height }) => {
  const bg = STATUS_COLORS[cell.status];
  const isCritical = cell.status === 'critical';

  return (
    <div
      className={isCritical ? 'uikit-health-critical' : undefined}
      style={{
        width: '100%',
        height,
        background: bg,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        borderRadius: 4,
        animation: isCritical ? 'uikit-health-pulse 3s ease-in-out infinite' : undefined,
      }}
    >
      <span
        style={{
          fontSize: 10,
          color: '#fff',
          fontWeight: 600,
          textShadow: '0 1px 2px rgba(0,0,0,0.3)',
          textAlign: 'center',
          padding: '0 2px',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
          maxWidth: '100%',
        }}
      >
        {cell.label}
      </span>
    </div>
  );
};

const CellBack: React.FC<{ cell: HealthCell; height: number }> = ({ cell, height }) => {
  const d = cell.details;
  const lines: [string, string][] = [];
  if (d?.cpu != null) lines.push(['CPU', `${d.cpu}%`]);
  if (d?.mem != null) lines.push(['内存', `${d.mem}%`]);
  if (d?.disk != null) lines.push(['磁盘', `${d.disk}%`]);
  if (d?.conn != null) lines.push(['连接', `${d.conn}`]);

  return (
    <div
      style={{
        width: '100%',
        height,
        background: 'var(--bg-card, #1a1a2e)',
        border: `1px solid ${STATUS_COLORS[cell.status]}`,
        borderRadius: 4,
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        padding: '2px 4px',
        boxSizing: 'border-box',
        fontSize: 9,
        lineHeight: '13px',
        color: 'var(--text-color, #ccc)',
        overflow: 'hidden',
      }}
    >
      {lines.map(([k, v]) => (
        <div key={k} style={{ display: 'flex', justifyContent: 'space-between' }}>
          <span style={{ opacity: 0.7 }}>{k}</span>
          <span style={{ fontWeight: 600 }}>{v}</span>
        </div>
      ))}
      {lines.length === 0 && (
        <span style={{ textAlign: 'center', opacity: 0.5 }}>--</span>
      )}
    </div>
  );
};

export const HealthMatrix: React.FC<HealthMatrixProps> = ({
  cells,
  rows: _rows = 5,
  cols = 5,
  cellHeight = 40,
}) => {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: `repeat(${cols}, 1fr)`,
        gap: 4,
      }}
    >
      {cells.map((cell) => (
        <div key={cell.id} style={{ height: cellHeight }}>
          <FlipCard
            front={<CellFront cell={cell} height={cellHeight} />}
            back={<CellBack cell={cell} height={cellHeight} />}
            duration={400}
          />
        </div>
      ))}
    </div>
  );
};
