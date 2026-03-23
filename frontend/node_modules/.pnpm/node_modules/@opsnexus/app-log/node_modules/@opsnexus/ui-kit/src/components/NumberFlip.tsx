import React, { useMemo } from 'react';
import { injectGlobalStyles } from '../utils/injectGlobalStyles';

interface NumberFlipProps {
  value: number | string;
  suffix?: string;
  prefix?: string;
  duration?: number;
  fontSize?: number;
  fontWeight?: number;
  color?: string;
  className?: string;
}

injectGlobalStyles('uikit-numberflip-keyframes', `
@keyframes uikit-flipIn {
  from {
    transform: translateY(100%) rotateX(-80deg);
    opacity: 0;
  }
  to {
    transform: translateY(0) rotateX(0deg);
    opacity: 1;
  }
}
@media (prefers-reduced-motion: reduce) {
  .uikit-number-char { animation: none !important; opacity: 1 !important; }
}
`);

export const NumberFlip: React.FC<NumberFlipProps> = ({
  value,
  suffix,
  prefix,
  duration = 80,
  fontSize = 28,
  fontWeight = 700,
  color = 'var(--text-primary, #e2e8f0)',
  className = '',
}) => {
  const chars = useMemo(() => String(value).split(''), [value]);

  return (
    <span
      className={className}
      style={{
        display: 'inline-flex',
        alignItems: 'baseline',
        fontSize,
        fontWeight,
        color,
        fontFamily: 'Inter, "JetBrains Mono", sans-serif',
        fontVariantNumeric: 'tabular-nums',
        letterSpacing: '-0.5px',
        perspective: '400px',
      }}
    >
      {prefix && <span>{prefix}</span>}
      {chars.map((ch, i) => (
        <span
          key={`${i}-${ch}`}
          className="uikit-number-char"
          style={{
            display: 'inline-block',
            transformOrigin: 'bottom center',
            animation: `uikit-flipIn 0.6s cubic-bezier(0.16,1,0.3,1) both`,
            animationDelay: `${i * duration}ms`,
          }}
        >
          {ch}
        </span>
      ))}
      {suffix && (
        <span style={{ fontSize: fontSize * 0.55, fontWeight: 400, marginLeft: 3, letterSpacing: 0 }}>
          {suffix}
        </span>
      )}
    </span>
  );
};
