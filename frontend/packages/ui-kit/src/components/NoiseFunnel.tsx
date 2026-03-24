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

const LAYER_COLORS = ['rgba(255,70,70,0.2)', 'rgba(255,170,50,0.15)', 'rgba(77,166,255,0.15)', 'rgba(0,229,160,0.15)', 'rgba(0,229,160,0.35)'];

/**
 * 横向降噪漏斗 — 严格匹配 opsnexus-ui-demo.html 中的 .fn 流水线效果
 *
 * 布局：[stage] › [stage] › [stage] › [stage] › [stage]
 * 每个 stage: 进度条（含流光） + 数值 + 标签
 */
export const NoiseFunnel: React.FC<NoiseFunnelProps> = ({
  layers,
  height = 5,
}) => {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 3, padding: '5px 14px' }}>
      {layers.map((layer, i) => {
        const barColor = layer.color || LAYER_COLORS[i % LAYER_COLORS.length];
        const isLast = i === layers.length - 1;

        return (
          <React.Fragment key={i}>
            {/* 单个阶段 */}
            <div style={{ flex: 1, textAlign: 'center' }}>
              {/* 进度条 + 流光 */}
              <div
                style={{
                  height,
                  borderRadius: 3,
                  position: 'relative',
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    position: 'absolute',
                    inset: 0,
                    borderRadius: 3,
                    background: barColor,
                  }}
                />
                {/* 流光扫过效果 — demo .fn-flow */}
                <div
                  className="uikit-funnel-glow"
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: '-30%',
                    width: '30%',
                    height: '100%',
                    background: 'linear-gradient(90deg, transparent, rgba(255,255,255,0.2), transparent)',
                    animation: 'uikit-funnelFlow 2s linear infinite',
                    animationDelay: `${i * 0.4}s`,
                  }}
                />
              </div>
              {/* 数值 */}
              <div
                style={{
                  fontSize: 11,
                  fontWeight: 600,
                  marginTop: 3,
                  color: isLast ? 'var(--color-success, #00e5a0)' : 'var(--text-primary, rgba(200,220,240,0.75))',
                }}
              >
                {layer.value}
              </div>
              {/* 标签 */}
              <div
                style={{
                  fontSize: 7,
                  color: isLast ? 'var(--color-success-dim, rgba(0,229,160,0.35))' : 'var(--text-secondary, rgba(140,170,210,0.3))',
                }}
              >
                {layer.label}
              </div>
            </div>
            {/* 箭头分隔符 — demo .fn-a */}
            {i < layers.length - 1 && (
              <span
                style={{
                  fontSize: 11,
                  color: 'var(--text-secondary, rgba(60,140,255,0.1))',
                  flexShrink: 0,
                }}
              >
                &#8250;
              </span>
            )}
          </React.Fragment>
        );
      })}
    </div>
  );
};

export type { NoiseFunnelLayer, NoiseFunnelProps };
