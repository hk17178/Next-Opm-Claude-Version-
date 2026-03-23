import React, { useRef, useEffect, useCallback } from 'react';

interface SparkLineProps {
  data: number[];
  color?: string;
  showPulse?: boolean;
  width?: number;
  height?: number;
  animated?: boolean;
  strokeWidth?: number;
}

export const SparkLine: React.FC<SparkLineProps> = ({
  data,
  color = '#4da6ff',
  showPulse = true,
  width,
  height = 32,
  animated = true,
  strokeWidth = 1.5,
}) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const animFrameRef = useRef(0);

  const draw = useCallback(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container || data.length < 2) return;

    const dpr = window.devicePixelRatio || 1;
    const W = width || container.clientWidth;
    const H = height;
    canvas.width = W * dpr;
    canvas.height = H * dpr;
    canvas.style.width = `${W}px`;
    canvas.style.height = `${H}px`;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.scale(dpr, dpr);

    const min = Math.min(...data);
    const max = Math.max(...data);
    const range = max - min || 1;
    const padY = 4;

    const getX = (i: number, count: number) => (i / (count - 1)) * W;
    const getY = (v: number) => padY + (1 - (v - min) / range) * (H - padY * 2);

    let pointCount = data.length;
    let frame = 0;

    const render = () => {
      ctx.clearRect(0, 0, W, H);
      const count = animated ? Math.min(frame + 1, pointCount) : pointCount;
      if (count < 2) {
        frame++;
        animFrameRef.current = requestAnimationFrame(render);
        return;
      }

      // Area fill
      ctx.beginPath();
      ctx.moveTo(getX(0, count), getY(data[0]));
      for (let i = 1; i < count; i++) ctx.lineTo(getX(i, count), getY(data[i]));
      ctx.lineTo(getX(count - 1, count), H);
      ctx.lineTo(getX(0, count), H);
      ctx.closePath();
      const grad = ctx.createLinearGradient(0, 0, 0, H);
      grad.addColorStop(0, color + '4D'); // 30% alpha
      grad.addColorStop(1, color + '00');
      ctx.fillStyle = grad;
      ctx.fill();

      // Line
      ctx.beginPath();
      ctx.moveTo(getX(0, count), getY(data[0]));
      for (let i = 1; i < count; i++) ctx.lineTo(getX(i, count), getY(data[i]));
      ctx.strokeStyle = color;
      ctx.lineWidth = strokeWidth;
      ctx.lineJoin = 'round';
      ctx.lineCap = 'round';
      ctx.stroke();

      // Pulse dot at end
      if (showPulse && count === pointCount) {
        const ex = getX(count - 1, count);
        const ey = getY(data[count - 1]);
        const t = (Date.now() % 1500) / 1500;
        const pulseR = 4 + t * 4;
        const pulseA = 1 - t;

        ctx.beginPath();
        ctx.arc(ex, ey, pulseR, 0, Math.PI * 2);
        ctx.fillStyle = color + Math.round(pulseA * 80).toString(16).padStart(2, '0');
        ctx.fill();

        ctx.beginPath();
        ctx.arc(ex, ey, 3, 0, Math.PI * 2);
        ctx.fillStyle = color;
        ctx.fill();
      }

      if (animated && count < pointCount) {
        frame++;
        setTimeout(() => { animFrameRef.current = requestAnimationFrame(render); }, 35);
      } else if (showPulse) {
        animFrameRef.current = requestAnimationFrame(render);
      }
    };

    frame = animated ? 0 : pointCount;
    animFrameRef.current = requestAnimationFrame(render);

    return () => cancelAnimationFrame(animFrameRef.current);
  }, [data, color, showPulse, width, height, animated, strokeWidth]);

  useEffect(() => {
    const cleanup = draw();
    return () => {
      cancelAnimationFrame(animFrameRef.current);
      cleanup?.();
    };
  }, [draw]);

  return (
    <div ref={containerRef} style={{ width: width || '100%', height, lineHeight: 0 }}>
      <canvas ref={canvasRef} style={{ display: 'block' }} />
    </div>
  );
};
