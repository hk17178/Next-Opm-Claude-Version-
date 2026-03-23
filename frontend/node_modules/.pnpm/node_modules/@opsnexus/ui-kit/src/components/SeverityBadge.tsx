import React from 'react';
import { Tag } from 'antd';
import type { Severity } from '../types';
import { SEVERITY_COLORS } from '../theme';

interface SeverityBadgeProps {
  severity: Severity;
}

const styles = `
@keyframes uikit-severityPulse {
  0%, 100% { opacity: 1; }
  50%      { opacity: 0.75; }
}
@media (prefers-reduced-motion: reduce) {
  .uikit-severity-p0 { animation: none !important; }
}
`;

const SEVERITY_CSS_VARS: Record<string, string> = {
  P0: 'var(--color-p0, #F85149)',
  P1: 'var(--color-p1, #ff6b6b)',
  P2: 'var(--color-p2, #ffaa33)',
  P3: 'var(--color-p3, #4da6ff)',
  P4: 'var(--color-p4, #8899a6)',
};

export const SeverityBadge: React.FC<SeverityBadgeProps> = ({ severity }) => {
  const color = SEVERITY_CSS_VARS[severity] || SEVERITY_COLORS[severity];
  const isP0 = severity === 'P0';
  return (
    <>
      <style>{styles}</style>
      <Tag
        color={SEVERITY_COLORS[severity]}
        className={isP0 ? 'uikit-severity-p0' : undefined}
        style={{
          borderRadius: 4,
          height: 22,
          lineHeight: '20px',
          fontWeight: 600,
          boxShadow: isP0 ? `0 0 6px ${SEVERITY_COLORS[severity] || '#F85149'}` : undefined,
          animation: isP0 ? 'uikit-severityPulse 2s ease-in-out infinite' : undefined,
        }}
      >
        {severity}
      </Tag>
    </>
  );
};
