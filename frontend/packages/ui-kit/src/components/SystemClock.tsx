import React, { useState, useEffect } from 'react';
import { injectGlobalStyles } from '../utils/injectGlobalStyles';

export interface SystemClockProps {
  /** System launch date — used to calculate uptime days */
  startDate?: Date | string;
  /** Clock font size in px (default 48) */
  fontSize?: number;
}

injectGlobalStyles('uikit-system-clock-keyframes', `
@keyframes uikit-colonBlink {
  0%, 100% { opacity: 1; }
  50%      { opacity: 0; }
}
@media (prefers-reduced-motion: reduce) {
  .uikit-clock-colon { animation: none !important; }
}
`);

function formatTime(date: Date): { h: string; m: string; s: string } {
  return {
    h: String(date.getHours()).padStart(2, '0'),
    m: String(date.getMinutes()).padStart(2, '0'),
    s: String(date.getSeconds()).padStart(2, '0'),
  };
}

function calcUptimeDays(start: Date): number {
  const diff = Date.now() - start.getTime();
  return Math.max(0, Math.floor(diff / (1000 * 60 * 60 * 24)));
}

export const SystemClock: React.FC<SystemClockProps> = ({
  startDate,
  fontSize = 48,
}) => {
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(id);
  }, []);

  const { h, m, s } = formatTime(now);

  const start = startDate
    ? typeof startDate === 'string'
      ? new Date(startDate)
      : startDate
    : null;

  const uptimeDays = start ? calcUptimeDays(start) : null;

  const digitStyle: React.CSSProperties = {
    fontFamily: '"JetBrains Mono", "Fira Code", "SF Mono", "Cascadia Code", monospace',
    fontVariantNumeric: 'tabular-nums',
    fontSize,
    fontWeight: 700,
    color: 'var(--text-primary, #e6e6e6)',
    letterSpacing: '0.04em',
  };

  const colonStyle: React.CSSProperties = {
    ...digitStyle,
    animation: 'uikit-colonBlink 1s step-end infinite',
    padding: '0 2px',
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 8,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'baseline' }}>
        <span style={digitStyle}>{h}</span>
        <span className="uikit-clock-colon" style={colonStyle}>:</span>
        <span style={digitStyle}>{m}</span>
        <span className="uikit-clock-colon" style={colonStyle}>:</span>
        <span style={digitStyle}>{s}</span>
      </div>
      {uptimeDays !== null && (
        <span
          style={{
            fontSize: Math.max(12, fontSize * 0.28),
            fontWeight: 500,
            color: 'var(--text-secondary, #8899a6)',
            letterSpacing: '0.06em',
          }}
        >
          已运行 {uptimeDays.toLocaleString()} 天
        </span>
      )}
    </div>
  );
};
