import React from 'react';
import { injectGlobalStyles } from '../utils/injectGlobalStyles';

interface NoiseFunnelLayer {
  label: string;
  value: number;
  color?: string;
}

interface NoiseFunnelProps {
  layers: NoiseFunnelLayer[];
  height?: number;
  showArrows?: boolean;
}

injectGlobalStyles('uikit-funnel-keyframes', `
@keyframes uikit-funnelFlow {
  0%   { left: -30%; }
  100% { left: 130%; }
}
@media (prefers-reduced-motion: reduce) {
  .uikit-funnel-glow { animation: none !important; }
}
`);

const LAYER_COLORS = ['#4da6ff', '#60a5fa', '#ffaa33', '#ff8c42', '#00e5a0'];

export const NoiseFunnel: React.FC<NoiseFunnelProps> = ({
  layers,
  height = 32,
  showArrows = true,
}) => {
  const maxValue = layers.length > 0 ? layers[0].value : 1;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
      {layers.map((layer, i) => {
        const widthPct = Math.max(8, (layer.value / maxValue) * 100);
        const reductionPct = i > 0 ? Math.round((1 - layer.value / layers[i - 1].value) * 100) : 0;
        const barColor = layer.color || LAYER_COLORS[i % LAYER_COLORS.length];

        return (
          <React.Fragment key={i}>
            {showArrows && i > 0 && (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 22, gap: 6 }}>
                <svg width="10" height="8" viewBox="0 0 10 8">
                  <polygon points="5,8 0,0 10,0" fill="var(--text-secondary, #8899a6)" opacity={0.45} />
                </svg>
                <span style={{ fontSize: 11, color: 'var(--color-success, #00e5a0)', fontWeight: 700 }}>
                  -{reductionPct}%
                </span>
              </div>
            )}
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, height }}>
              <span style={{
                width: 72,
                fontSize: 12,
                color: 'var(--text-secondary, #8899a6)',
                textAlign: 'right',
                flexShrink: 0,
              }}>
                {layer.label}
              </span>
              <div style={{
                flex: 1,
                position: 'relative',
                height: height * 0.65,
                borderRadius: 3,
                background: 'rgba(255,255,255,0.04)',
                overflow: 'hidden',
              }}>
                <div style={{
                  width: `${widthPct}%`,
                  height: '100%',
                  borderRadius: 3,
                  background: barColor,
                  opacity: 0.85,
                  position: 'relative',
                  overflow: 'hidden',
                  transition: 'width 0.8s cubic-bezier(0.4,0,0.2,1)',
                }}>
                  {/* 流光扫过效果 */}
                  <div
                    className="uikit-funnel-glow"
                    style={{
                      position: 'absolute',
                      top: 0,
                      left: '-30%',
                      width: '25%',
                      height: '100%',
                      background: 'linear-gradient(90deg, transparent, rgba(255,255,255,0.35), transparent)',
                      animation: `uikit-funnelFlow 2s linear infinite`,
                      animationDelay: `${i * 0.3}s`,
                    }}
                  />
                </div>
              </div>
              <span style={{
                width: 56,
                fontSize: 13,
                color: 'var(--text-primary, #e2e8f0)',
                fontWeight: 700,
                fontVariantNumeric: 'tabular-nums',
                flexShrink: 0,
              }}>
                {layer.value.toLocaleString()}
              </span>
            </div>
          </React.Fragment>
        );
      })}
    </div>
  );
};

export type { NoiseFunnelLayer, NoiseFunnelProps };
