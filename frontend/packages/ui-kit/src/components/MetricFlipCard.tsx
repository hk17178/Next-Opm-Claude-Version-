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
  /** 扫描线颜色（rgba 格式），不传则使用默认蓝色 */
  scanColor?: string;
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

const FrontFace: React.FC<MetricFlipCardProps> = ({ label, value, suffix, trend, trendValue, icon, animate = true, color, scanColor }) => (
  <div style={{ ...cardStyle, display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
    {/* 静态顶部渐变线 — demo .cd-top */}
    <div
      style={{
        position: 'absolute',
        top: 0,
        left: 0,
        right: 0,
        height: 1,
        background: 'linear-gradient(90deg, transparent 5%, var(--color-primary-alpha15, rgba(77,166,255,0.15)) 50%, transparent 95%)',
        pointerEvents: 'none',
      }}
    />
    {/* 动态扫描线 — demo .cd-scan */}
    <ScanLine color={scanColor} />
    {/* 标签 — demo .cd-l */}
    <div style={{ fontSize: 8, letterSpacing: '1.2px', fontWeight: 500, color: 'var(--text-secondary, rgba(140,170,210,0.35))' }}>
      {label}
    </div>
    {/* 数值 — demo .cd-v */}
    <div style={{ color: color || 'var(--text-primary, #e2e8f0)' }}>
      {animate ? (
        <NumberFlip value={value} suffix={suffix} fontSize={28} fontWeight={700} color={color || 'var(--text-primary, #e2e8f0)'} />
      ) : (
        <span style={{ fontSize: 28, fontWeight: 700, fontFamily: 'Inter, sans-serif', fontVariantNumeric: 'tabular-nums' }}>
          {value}{suffix && <span style={{ fontSize: 16, fontWeight: 400, marginLeft: 2 }}>{suffix}</span>}
        </span>
      )}
    </div>
    {/* 趋势 — demo .cd-t */}
    {trend && (
      <div style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 9, color: color || trendColors[trend] }}>
        <span style={{ fontWeight: 600 }}>{trendArrows[trend]}</span>
        {trendValue && <span>{trendValue}</span>}
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
