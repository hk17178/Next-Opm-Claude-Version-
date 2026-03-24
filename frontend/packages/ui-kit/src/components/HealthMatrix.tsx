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
  /** 列标题，如 ['NET','HOST','APP','DB','MW'] */
  colLabels?: string[];
  /** 行标题，如 ['pay','shop','risk'] */
  rowLabels?: string[];
}

const STATUS_COLORS: Record<HealthCell['status'], string> = {
  ok: 'var(--color-success, rgba(0,229,160,0.06))',
  degraded: 'var(--color-warning, rgba(255,170,50,0.06))',
  critical: 'var(--color-danger, rgba(255,70,70,0.06))',
  unknown: 'rgba(140,170,210,0.06)',
};

const STATUS_TEXT_COLORS: Record<HealthCell['status'], string> = {
  ok: 'var(--color-success, #00e5a0)',
  degraded: 'var(--color-warning, #ffaa33)',
  critical: 'var(--color-danger, #ff6b6b)',
  unknown: 'rgba(140,170,210,0.3)',
};

const STATUS_BORDER: Record<HealthCell['status'], string> = {
  ok: 'var(--color-success, rgba(0,229,160,0.06))',
  degraded: 'var(--color-warning, rgba(255,170,50,0.08))',
  critical: 'var(--color-danger, rgba(255,70,70,0.08))',
  unknown: 'rgba(140,170,210,0.06)',
};

injectGlobalStyles('uikit-health-pulse-keyframes', `
@keyframes uikit-health-pulse {
  0%, 100% { box-shadow: inset 0 0 6px rgba(255,70,70,0.06); }
  50% { box-shadow: inset 0 0 14px rgba(255,70,70,0.15); }
}
@media (prefers-reduced-motion: reduce) {
  .uikit-health-critical { animation: none !important; }
}
`);

/**
 * CellFront — 严格匹配 demo .hc-f 样式
 * 圆角方块，背景色半透明 + 边框，居中显示状态
 */
const CellFront: React.FC<{ cell: HealthCell; height: number }> = ({ cell, height }) => {
  const isCritical = cell.status === 'critical';

  return (
    <div
      className={isCritical ? 'uikit-health-critical' : undefined}
      style={{
        width: '100%',
        height,
        background: STATUS_COLORS[cell.status],
        border: `1px solid ${STATUS_BORDER[cell.status]}`,
        color: STATUS_TEXT_COLORS[cell.status],
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        borderRadius: 5,
        fontSize: 7,
        fontWeight: 500,
        cursor: 'pointer',
        animation: isCritical ? 'uikit-health-pulse 3s ease infinite' : undefined,
      }}
    />
  );
};

/**
 * CellBack — 严格匹配 demo .hc-b 样式
 * 翻转后显示关键指标数值
 */
const CellBack: React.FC<{ cell: HealthCell; height: number }> = ({ cell, height }) => {
  const d = cell.details;
  // 根据 details 选择最关键的一行显示
  let summary = '--';
  if (d?.cpu != null && d.cpu > 80) summary = `${d.cpu}% cpu`;
  else if (d?.disk != null && d.disk > 70) summary = `${d.disk}% disk`;
  else if (d?.mem != null && d.mem > 80) summary = `${d.mem}% mem`;
  else if (d?.conn != null) summary = `${d.conn} conn`;
  else if (d?.cpu != null) summary = `OK ${d.cpu}%`;
  else summary = cell.status === 'ok' ? 'OK' : '--';

  return (
    <div
      style={{
        width: '100%',
        height,
        background: STATUS_COLORS[cell.status],
        border: `1px solid ${STATUS_BORDER[cell.status]}`,
        color: STATUS_TEXT_COLORS[cell.status],
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        borderRadius: 5,
        padding: '0 3px',
        fontFamily: 'ui-monospace, monospace',
        fontSize: 6,
        letterSpacing: '0.2px',
        fontWeight: 500,
      }}
    >
      {summary}
    </div>
  );
};

/**
 * HealthMatrix — 严格匹配 demo .hg 网格布局
 *
 * 带可选行标签（第一列）和列标题（第一行），
 * gridTemplateColumns: [labelCol] repeat(cols, 1fr)
 */
export const HealthMatrix: React.FC<HealthMatrixProps> = ({
  cells,
  rows: _rows = 5,
  cols = 5,
  cellHeight = 24,
  colLabels,
  rowLabels,
}) => {
  const hasLabels = !!(colLabels || rowLabels);
  const gridCols = hasLabels
    ? `46px repeat(${cols}, 1fr)`
    : `repeat(${cols}, 1fr)`;

  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: gridCols,
        gap: 3,
        padding: '8px 10px',
      }}
    >
      {/* 列标题行 */}
      {colLabels && (
        <>
          {/* 左上角空白 */}
          <div />
          {colLabels.map((label) => (
            <div
              key={label}
              style={{
                fontSize: 7,
                textAlign: 'center',
                padding: '2px 0',
                fontWeight: 500,
                color: 'var(--text-secondary, rgba(140,170,210,0.3))',
              }}
            >
              {label}
            </div>
          ))}
        </>
      )}

      {/* 数据行 */}
      {cells.map((cell, i) => {
        const colIdx = i % cols;
        const rowIdx = Math.floor(i / cols);

        return (
          <React.Fragment key={cell.id}>
            {/* 行标签 */}
            {colIdx === 0 && rowLabels && rowLabels[rowIdx] && (
              <div
                style={{
                  fontSize: 8,
                  display: 'flex',
                  alignItems: 'center',
                  color: 'var(--text-secondary, rgba(140,170,210,0.3))',
                }}
              >
                {rowLabels[rowIdx]}
              </div>
            )}
            {colIdx === 0 && hasLabels && !rowLabels?.[rowIdx] && <div />}
            {/* 翻转格子 */}
            <div style={{ height: cellHeight }}>
              <FlipCard
                front={<CellFront cell={cell} height={cellHeight} />}
                back={<CellBack cell={cell} height={cellHeight} />}
                duration={400}
              />
            </div>
          </React.Fragment>
        );
      })}
    </div>
  );
};
