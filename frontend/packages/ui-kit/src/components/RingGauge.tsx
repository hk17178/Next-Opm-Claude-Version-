import React, { useRef, useEffect } from 'react';

interface RingGaugeProps {
  /** 当前值 */
  value: number;
  /** 最大值，默认 100 */
  max?: number;
  /** 画布尺寸(px)，默认 110 */
  size?: number;
  /** 弧线宽度(px)，默认 5 */
  strokeWidth?: number;
  /** 渐变起止色 [start, end]，默认 ['#00e5a0', '#4da6ff'] */
  colors?: [string, string];
}

function easeOut(t: number): number {
  return 1 - Math.pow(1 - t, 3);
}

/**
 * 环形仪表盘 — 严格匹配 demo drawR() 函数
 *
 * 只绘制背景轨道 + 渐变弧线，中心文字由调用方用 DOM 覆盖层实现
 * （demo 使用 .ring-lb 做 DOM 文字，不在 canvas 上绘制文字）
 */
export const RingGauge: React.FC<RingGaugeProps> = ({
  value,
  max = 100,
  size = 110,
  strokeWidth = 5,
  colors = ['#00e5a0', '#4da6ff'],
}) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const animRef = useRef(0);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const dpr = window.devicePixelRatio || 1;
    canvas.width = size * dpr;
    canvas.height = size * dpr;
    canvas.style.width = `${size}px`;
    canvas.style.height = `${size}px`;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.scale(dpr, dpr);

    const cx = size / 2;
    const cy = size / 2;
    const radius = (size - strokeWidth) / 2 - 2;
    const targetPct = Math.min(value / max, 1);
    const duration = 800;
    const startTime = performance.now();

    const render = (now: number) => {
      const elapsed = now - startTime;
      const t = Math.min(elapsed / duration, 1);
      const pct = targetPct * easeOut(t);

      ctx.clearRect(0, 0, size, size);

      // 背景轨道 — demo: rgba(60,140,255,0.04)
      ctx.beginPath();
      ctx.arc(cx, cy, radius, 0, Math.PI * 2);
      ctx.strokeStyle = 'rgba(60,140,255,0.04)';
      ctx.lineWidth = strokeWidth;
      ctx.stroke();

      // 渐变弧线 — demo: createLinearGradient(10,10,100,100)
      if (pct > 0) {
        const startAngle = -Math.PI / 2;
        const endAngle = startAngle + Math.PI * 2 * pct;

        const grad = ctx.createLinearGradient(10, 10, size - 10, size - 10);
        grad.addColorStop(0, colors[0]);
        grad.addColorStop(1, colors[1]);

        ctx.beginPath();
        ctx.arc(cx, cy, radius, startAngle, endAngle);
        ctx.strokeStyle = grad;
        ctx.lineWidth = strokeWidth;
        ctx.lineCap = 'round';
        ctx.stroke();
      }

      if (t < 1) {
        animRef.current = requestAnimationFrame(render);
      }
    };

    animRef.current = requestAnimationFrame(render);
    return () => cancelAnimationFrame(animRef.current);
  }, [value, max, size, strokeWidth, colors]);

  return (
    <canvas
      ref={canvasRef}
      data-no-transition
      style={{ display: 'block', width: size, height: size }}
    />
  );
};
