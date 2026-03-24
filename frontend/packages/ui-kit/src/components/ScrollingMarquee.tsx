import React, { useRef, useEffect, useState } from 'react';
import { injectGlobalStyles } from '../utils/injectGlobalStyles';

export interface ScrollingMarqueeItem {
  text: string;
  severity?: 'critical' | 'warning' | 'info' | string;
  color?: string;
}

export interface ScrollingMarqueeProps {
  items: ScrollingMarqueeItem[];
  /** Scroll speed — animation duration in seconds for one full cycle (default 20) */
  speed?: number;
  /** Container height in px (default 36) */
  height?: number;
}

const SEVERITY_COLOR_MAP: Record<string, string> = {
  critical: 'var(--color-danger, #ff4d4f)',
  warning: 'var(--color-warning, #faad14)',
  info: 'var(--color-info, #1890ff)',
};

injectGlobalStyles('uikit-scrolling-marquee-keyframes', `
@keyframes uikit-marqueeScroll {
  0%   { transform: translateX(0); }
  100% { transform: translateX(-50%); }
}
@media (prefers-reduced-motion: reduce) {
  .uikit-marquee-track { animation: none !important; }
}
`);

export const ScrollingMarquee: React.FC<ScrollingMarqueeProps> = ({
  items,
  speed = 20,
  height = 36,
}) => {
  const trackRef = useRef<HTMLDivElement>(null);
  const [animDuration, setAnimDuration] = useState(speed);

  // Recalculate duration when items or speed change
  useEffect(() => {
    setAnimDuration(speed);
  }, [speed, items]);

  if (!items.length) return null;

  const resolveColor = (item: ScrollingMarqueeItem): string => {
    if (item.color) return item.color;
    if (item.severity && SEVERITY_COLOR_MAP[item.severity]) {
      return SEVERITY_COLOR_MAP[item.severity];
    }
    return 'var(--color-warning, #faad14)';
  };

  // Duplicate items so the scroll loops seamlessly
  const renderItems = (key: string) =>
    items.map((item, i) => (
      <span
        key={`${key}-${i}`}
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 6,
          whiteSpace: 'nowrap',
          padding: '0 24px',
          fontSize: 13,
          fontWeight: 500,
          color: 'var(--text-primary, #e6e6e6)',
        }}
      >
        <span
          style={{
            display: 'inline-block',
            width: 8,
            height: 8,
            borderRadius: '50%',
            backgroundColor: resolveColor(item),
            flexShrink: 0,
            boxShadow: `0 0 6px ${resolveColor(item)}`,
          }}
        />
        {item.text}
      </span>
    ));

  return (
    <div
      style={{
        width: '100%',
        height,
        overflow: 'hidden',
        display: 'flex',
        alignItems: 'center',
        background: 'var(--bg-marquee, rgba(0,0,0,0.3))',
        borderRadius: 4,
      }}
    >
      <div
        ref={trackRef}
        className="uikit-marquee-track"
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          whiteSpace: 'nowrap',
          animation: `uikit-marqueeScroll ${animDuration}s linear infinite`,
        }}
      >
        {renderItems('a')}
        {renderItems('b')}
      </div>
    </div>
  );
};
