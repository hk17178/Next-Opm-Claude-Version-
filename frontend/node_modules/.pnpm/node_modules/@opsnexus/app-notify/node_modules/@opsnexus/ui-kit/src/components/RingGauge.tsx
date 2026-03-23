import React, { useRef, useEffect } from 'react';

interface RingGaugeProps {
  value: number;
  max?: number;
  label?: string;
  size?: number;
  strokeWidth?: number;
  colors?: [string, string];
}

function easeOut(t: number): number {
  return 1 - Math.pow(1 - t, 3);
}

export const RingGauge: React.FC<RingGaugeProps> = ({
  value,
  max = 100,
  label,
  size = 110,
  strokeWidth = 8,
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

      // Track
      ctx.beginPath();
      ctx.arc(cx, cy, radius, 0, Math.PI * 2);
      ctx.strokeStyle = 'rgba(255,255,255,0.08)';
      ctx.lineWidth = strokeWidth;
      ctx.lineCap = 'round';
      ctx.stroke();

      // Progress arc
      if (pct > 0) {
        const startAngle = -Math.PI / 2;
        const endAngle = startAngle + Math.PI * 2 * pct;

        const grad = ctx.createConicGradient(startAngle, cx, cy);
        grad.addColorStop(0, colors[0]);
        grad.addColorStop(pct, colors[1]);
        grad.addColorStop(1, colors[1]);

        ctx.beginPath();
        ctx.arc(cx, cy, radius, startAngle, endAngle);
        ctx.strokeStyle = grad;
        ctx.lineWidth = strokeWidth;
        ctx.lineCap = 'round';
        ctx.stroke();
      }

      // Center text
      const displayPct = Math.round(pct * max);
      ctx.fillStyle = 'var(--text-primary)';
      ctx.font = `600 18px Inter, sans-serif`;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillStyle = '#e2e8f0';
      ctx.fillText(`${displayPct}%`, cx, label ? cy - 6 : cy);

      if (label) {
        ctx.font = `12px Inter, sans-serif`;
        ctx.fillStyle = '#8899a6';
        ctx.fillText(label, cx, cy + 12);
      }

      if (t < 1) {
        animRef.current = requestAnimationFrame(render);
      }
    };

    animRef.current = requestAnimationFrame(render);
    return () => cancelAnimationFrame(animRef.current);
  }, [value, max, label, size, strokeWidth, colors]);

  return (
    <canvas
      ref={canvasRef}
      style={{ display: 'block', width: size, height: size }}
    />
  );
};
