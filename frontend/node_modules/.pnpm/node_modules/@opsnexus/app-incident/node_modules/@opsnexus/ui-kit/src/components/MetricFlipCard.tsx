import React, { useState } from 'react';
import { FlipCard } from './FlipCard';
import { ScanLine } from './ScanLine';
import { NumberFlip } from './NumberFlip';

interface MetricFlipCardProps {
  label: string;
  value: number | string;
  suffix?: string;
  trend?: 'up' | 'down' | 'flat';
  trendValue?: string;
  icon?: React.ReactNode;
  back?: React.ReactNode;
  backItems?: Array<{ label: string; value: string | number }>;
  color?: string;
  animate?: boolean;
}

const trendArrows: Record<string, string> = { up: '\u2191', down: '\u2193', flat: '\u2192' };
const trendColors: Record<string, string> = { up: '#00e5a0', down: '#ff6b6b', flat: 'var(--text-secondary, #8899a6)' };

// overflow:hidden and backdropFilter are intentionally excluded from cardStyle.
// Both create new stacking contexts on elements inside a preserve-3d hierarchy,
// which causes browsers to flatten the 3D rendering context and break
// backface-visibility + the flip animation.
// overflow:hidden is applied on the outer wrapper OUTSIDE preserve-3d instead.
const cardStyle: React.CSSProperties = {
  background: 'var(--bg-card, rgba(255,255,255,0.04))',
  border: '1px solid var(--border-color, rgba(255,255,255,0.08))',
  borderRadius: 12,
  position: 'relative',
  height: 108,
  padding: '12px 16px',
  boxSizing: 'border-box',
};

const FrontFace: React.FC<MetricFlipCardProps> = ({ label, value, suffix, trend, trendValue, icon, animate = true }) => (
  <div style={{ ...cardStyle, display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
    <ScanLine />
    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
      {icon && <span style={{ fontSize: 14, lineHeight: 1 }}>{icon}</span>}
      <span style={{ color: 'var(--text-secondary, #8899a6)', fontSize: 12 }}>{label}</span>
    </div>
    <div>
      {animate ? (
        <NumberFlip value={value} suffix={suffix} fontSize={28} fontWeight={700} />
      ) : (
        <span style={{ fontSize: 28, fontWeight: 700, fontFamily: 'Inter, sans-serif', fontVariantNumeric: 'tabular-nums', color: 'var(--text-primary, #e2e8f0)' }}>
          {value}{suffix && <span style={{ fontSize: 16, fontWeight: 400, marginLeft: 2 }}>{suffix}</span>}
        </span>
      )}
    </div>
    {trend && (
      <div style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
        <span style={{ color: trendColors[trend], fontWeight: 600 }}>{trendArrows[trend]}</span>
        {trendValue && <span style={{ color: trendColors[trend] }}>{trendValue}</span>}
      </div>
    )}
    {!trend && <div style={{ height: 16 }} />}
  </div>
);

const BackFace: React.FC<Pick<MetricFlipCardProps, 'label' | 'back' | 'backItems'>> = ({ label, back, backItems }) => (
  <div style={{ ...cardStyle, display: 'flex', flexDirection: 'column', padding: '10px 14px' }}>
    <div style={{ fontSize: 10, color: 'var(--text-secondary, #8899a6)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
      {label}
    </div>
    {back ? (
      <div style={{ flex: 1, overflow: 'auto' }}>{back}</div>
    ) : backItems ? (
      <div style={{ flex: 1, overflow: 'auto' }}>
        {backItems.map((item, i) => (
          <div
            key={i}
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              padding: '4px 0',
              borderBottom: i < backItems.length - 1 ? '1px solid var(--border-color, rgba(255,255,255,0.06))' : 'none',
              fontSize: 12,
            }}
          >
            <span style={{ color: 'var(--text-secondary, #8899a6)' }}>{item.label}</span>
            <span style={{ color: 'var(--text-primary, #e2e8f0)', fontWeight: 600, fontVariantNumeric: 'tabular-nums' }}>{item.value}</span>
          </div>
        ))}
      </div>
    ) : null}
  </div>
);

export const MetricFlipCard: React.FC<MetricFlipCardProps> = (props) => {
  const [hovered, setHovered] = useState(false);

  return (
    <div
      style={{
        height: 108,
        borderRadius: 12,
        overflow: 'hidden',
        transition: 'transform 0.3s, box-shadow 0.3s',
        transform: hovered ? 'translateY(-2px)' : 'translateY(0)',
        boxShadow: hovered ? '0 8px 24px rgba(77,166,255,0.12)' : 'none',
      }}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <FlipCard
        front={<FrontFace {...props} />}
        back={<BackFace label={props.label} back={props.back} backItems={props.backItems} />}
      />
    </div>
  );
};
