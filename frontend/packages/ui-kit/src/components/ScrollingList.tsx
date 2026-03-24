import React, { useState, useEffect, useRef, useCallback } from 'react';
import { injectGlobalStyles } from '../utils/injectGlobalStyles';

export interface ScrollingListItem {
  id: string;
  content: React.ReactNode;
}

export interface ScrollingListProps {
  items: ScrollingListItem[];
  /** Scroll interval in milliseconds (default 3000) */
  interval?: number;
  /** Container height in px (default 300) */
  height?: number;
  /** Number of visible items (default 5) */
  visibleCount?: number;
}

injectGlobalStyles('uikit-scrolling-list-styles', `
@media (prefers-reduced-motion: reduce) {
  .uikit-scrolling-list-inner { transition: none !important; }
}
`);

export const ScrollingList: React.FC<ScrollingListProps> = ({
  items,
  interval = 3000,
  height = 300,
  visibleCount = 5,
}) => {
  const [offset, setOffset] = useState(0);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const itemHeight = height / visibleCount;

  const clearTimer = useCallback(() => {
    if (timerRef.current) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  useEffect(() => {
    if (items.length <= visibleCount) return;

    clearTimer();
    timerRef.current = setInterval(() => {
      setOffset((prev) => {
        const next = prev + 1;
        // When we've scrolled past all items, reset to 0
        if (next >= items.length) {
          return 0;
        }
        return next;
      });
    }, interval);

    return clearTimer;
  }, [items.length, visibleCount, interval, clearTimer]);

  // Reset offset when items change
  useEffect(() => {
    setOffset(0);
  }, [items]);

  if (!items.length) return null;

  // Build a display list: original items + duplicate enough for seamless wrapping
  const displayItems = items.length > visibleCount
    ? [...items, ...items.slice(0, visibleCount)]
    : items;

  // When offset would show duplicated tail items and we need to jump back
  const shouldTransition = offset <= items.length;
  const translateY = -(offset * itemHeight);

  return (
    <div
      style={{
        width: '100%',
        height,
        overflow: 'hidden',
        borderRadius: 6,
        background: 'var(--bg-card, rgba(255,255,255,0.04))',
      }}
    >
      <div
        className="uikit-scrolling-list-inner"
        style={{
          transform: `translateY(${translateY}px)`,
          transition: shouldTransition ? 'transform 0.5s ease-in-out' : 'none',
        }}
      >
        {displayItems.map((item, index) => (
          <div
            key={`${item.id}-${index}`}
            style={{
              height: itemHeight,
              display: 'flex',
              alignItems: 'center',
              padding: '0 16px',
              borderBottom: '1px solid var(--border-color, rgba(255,255,255,0.06))',
              boxSizing: 'border-box',
              fontSize: 13,
              color: 'var(--text-primary, #e6e6e6)',
            }}
          >
            {item.content}
          </div>
        ))}
      </div>
    </div>
  );
};
