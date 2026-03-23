import React from 'react';
import { injectGlobalStyles } from '../utils/injectGlobalStyles';

interface LiveDotProps {
  label?: string;
  color?: string;
  size?: number;
}

injectGlobalStyles('uikit-livedot-keyframes', `
@keyframes uikit-livePulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 10px var(--color-success, #00e5a0); }
  50%       { opacity: 0.4; box-shadow: 0 0 3px var(--color-success, #00e5a0); }
}
@media (prefers-reduced-motion: reduce) {
  .uikit-live-dot { animation: none !important; }
}
`);

export const LiveDot: React.FC<LiveDotProps> = ({
  label = 'LIVE',
  color = 'var(--color-success, #00e5a0)',
  size = 6,
}) => {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
      <span
        className="uikit-live-dot"
        style={{
          display: 'inline-block',
          width: size,
          height: size,
          borderRadius: '50%',
          backgroundColor: color,
          animation: 'uikit-livePulse 3s ease-in-out infinite',
        }}
      />
      {label && (
        <span style={{ color: 'var(--text-secondary, #8899a6)', fontSize: 11, fontWeight: 600, letterSpacing: '0.05em' }}>
          {label}
        </span>
      )}
    </span>
  );
};
