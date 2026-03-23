import React, { useState } from 'react';

export interface FlipCardProps {
  front: React.ReactNode;
  back: React.ReactNode;
  duration?: number;
  direction?: 'horizontal' | 'vertical';
  className?: string;
  style?: React.CSSProperties;
}

const faceStyle: React.CSSProperties = {
  backfaceVisibility: 'hidden',
  WebkitBackfaceVisibility: 'hidden',
  position: 'absolute',
  top: 0, left: 0, width: '100%', height: '100%',
};

export const FlipCard: React.FC<FlipCardProps> = ({
  front,
  back,
  duration = 600,
  direction = 'horizontal',
  className = '',
  style,
}) => {
  const [flipped, setFlipped] = useState(false);
  const isVertical = direction === 'vertical';
  const flipTransform = isVertical ? 'rotateX(-180deg)' : 'rotateY(180deg)';

  // 使用 React state 代替 CSS hover 选择器，避免 qiankun experimentalStyleIsolation
  // 对 <style> 标签内选择器的重写导致 hover 失效
  const prefersReducedMotion =
    typeof window !== 'undefined' &&
    window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;

  // 必须用 rotateY(0deg) 而非 'none'，浏览器只能在两个同类型 transform 之间插值。
  // none → rotateY(180deg) 无法平滑过渡；rotateY(0deg) → rotateY(180deg) 可以。
  const neutralTransform = isVertical ? 'rotateX(0deg)' : 'rotateY(0deg)';

  return (
    <div
      className={`uikit-flip-card ${className}`}
      style={{ perspective: '800px', width: '100%', height: '100%', ...style }}
      onMouseEnter={() => setFlipped(true)}
      onMouseLeave={() => setFlipped(false)}
    >
      <div
        style={{
          transition: prefersReducedMotion ? 'none' : `transform ${duration}ms cubic-bezier(0.4,0,0.2,1)`,
          transformStyle: 'preserve-3d',
          WebkitTransformStyle: 'preserve-3d',
          transform: flipped ? flipTransform : neutralTransform,
          position: 'relative',
          width: '100%',
          height: '100%',
        }}
      >
        <div style={faceStyle}>{front}</div>
        <div style={{ ...faceStyle, transform: flipTransform }}>{back}</div>
      </div>
    </div>
  );
};
