import React from 'react';
import { injectGlobalStyles } from '../utils/injectGlobalStyles';

interface ScanLineProps {
  color?: string;
  duration?: number;
  delay?: number;
  height?: number;
}

// 注入到 document.head，绕过 qiankun experimentalStyleIsolation 沙箱
injectGlobalStyles('uikit-scanline-keyframes', `
@keyframes uikit-scanLine {
  0%   { left: -60%; opacity: 0; }
  10%  { opacity: 1; }
  90%  { opacity: 1; }
  100% { left: 160%; opacity: 0; }
}
@media (prefers-reduced-motion: reduce) {
  .uikit-scan-line { animation: none !important; }
}
`);

export const ScanLine: React.FC<ScanLineProps> = ({
  color = 'var(--color-scan, rgba(77,166,255,0.6))',
  duration = 4000,
  delay = 0,
  height = 2,
}) => {
  return (
    <div
      className="uikit-scan-line"
      style={{
        position: 'absolute',
        top: 0,
        left: '-60%',
        width: '60%',
        height,
        background: `linear-gradient(90deg, transparent, ${color}, transparent)`,
        animation: `uikit-scanLine ${duration}ms ease-in-out infinite`,
        animationDelay: `${delay}ms`,
        pointerEvents: 'none',
        zIndex: 1,
      }}
    />
  );
};
