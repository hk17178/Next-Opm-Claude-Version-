import React, { useRef, useEffect } from 'react';

interface ParticleCanvasProps {
  isDark?: boolean;
  className?: string;
  style?: React.CSSProperties;
}

interface ThemeColors {
  particle: [number, number, number];
  streamerA: [number, number, number];
  streamerB: [number, number, number];
  lineAlpha: number;
  particleCount: number;
}

const DARK_THEME: ThemeColors = {
  particle: [77, 166, 255],
  streamerA: [0, 229, 160],
  streamerB: [77, 166, 255],
  lineAlpha: 0.04,
  particleCount: 70,
};

const LIGHT_THEME: ThemeColors = {
  particle: [37, 99, 235],
  streamerA: [16, 185, 129],
  streamerB: [37, 99, 235],
  lineAlpha: 0.03,
  particleCount: 65,
};

interface Particle {
  x: number;
  y: number;
  vx: number;
  vy: number;
  r: number;
}

interface Streamer {
  x: number;
  y: number;
  vx: number;
  vy: number;
  speed: number;
  life: number;
  maxLife: number;
  colorIdx: number;
}

class ParticleEngine {
  private canvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;
  private theme: ThemeColors;
  private particles: Particle[] = [];
  private streamers: Streamer[] = [];
  private animId = 0;
  private running = false;
  private mouseX = -1000;
  private mouseY = -1000;
  private radarAngle = 0;
  private lowPerf: boolean;
  private onMouseMove: (e: MouseEvent) => void;
  private onMouseLeave: () => void;

  constructor(canvas: HTMLCanvasElement, isDark: boolean) {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d')!;
    this.theme = isDark ? DARK_THEME : LIGHT_THEME;
    this.lowPerf = (navigator.hardwareConcurrency ?? 4) < 4;

    this.onMouseMove = (e: MouseEvent) => {
      const rect = canvas.getBoundingClientRect();
      this.mouseX = e.clientX - rect.left;
      this.mouseY = e.clientY - rect.top;
    };
    this.onMouseLeave = () => {
      this.mouseX = -1000;
      this.mouseY = -1000;
    };

    this.resize();
    this.initParticles();
    this.initStreamers();

    window.addEventListener('mousemove', this.onMouseMove);
    window.addEventListener('mouseleave', this.onMouseLeave);
  }

  private get W() {
    return this.canvas.parentElement?.clientWidth ?? this.canvas.width;
  }

  private get H() {
    return this.canvas.parentElement?.clientHeight ?? this.canvas.height;
  }

  private initParticles() {
    const count = this.lowPerf
      ? Math.floor(this.theme.particleCount / 2)
      : this.theme.particleCount;
    this.particles = [];
    for (let i = 0; i < count; i++) {
      const angle = Math.random() * Math.PI * 2;
      const speed = 0.15 + Math.random() * 0.1;
      this.particles.push({
        x: Math.random() * this.W,
        y: Math.random() * this.H,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed,
        r: 0.3 + Math.random() * 1.2,
      });
    }
  }

  private initStreamers() {
    if (this.lowPerf) {
      this.streamers = [];
      return;
    }
    this.streamers = [];
    for (let i = 0; i < 20; i++) {
      this.spawnStreamer();
    }
  }

  private spawnStreamer() {
    const angle = Math.random() * Math.PI * 2;
    const speed = 0.15 + Math.random() * 0.1;
    this.streamers.push({
      x: Math.random() * this.W,
      y: Math.random() * this.H,
      vx: Math.cos(angle) * speed * 2,
      vy: Math.sin(angle) * speed * 2,
      speed: speed * 2,
      life: 0,
      maxLife: 50 + Math.random() * 130,
      colorIdx: this.streamers.length % 2,
    });
  }

  resize() {
    const parent = this.canvas.parentElement;
    if (!parent) return;
    const dpr = window.devicePixelRatio || 1;
    const w = parent.clientWidth;
    const h = parent.clientHeight;
    this.canvas.width = w * dpr;
    this.canvas.height = h * dpr;
    this.canvas.style.width = `${w}px`;
    this.canvas.style.height = `${h}px`;
    this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }

  setTheme(isDark: boolean) {
    this.theme = isDark ? DARK_THEME : LIGHT_THEME;
  }

  start() {
    if (this.running) return;
    this.running = true;
    this.loop();
  }

  stop() {
    this.running = false;
    if (this.animId) {
      cancelAnimationFrame(this.animId);
      this.animId = 0;
    }
  }

  destroy() {
    this.stop();
    window.removeEventListener('mousemove', this.onMouseMove);
    window.removeEventListener('mouseleave', this.onMouseLeave);
  }

  private loop = () => {
    if (!this.running) return;
    this.update();
    this.draw();
    this.animId = requestAnimationFrame(this.loop);
  };

  private update() {
    const w = this.W;
    const h = this.H;

    for (const p of this.particles) {
      const dx = this.mouseX - p.x;
      const dy = this.mouseY - p.y;
      const dist = Math.sqrt(dx * dx + dy * dy);
      if (dist < 140 && dist > 0) {
        p.vx += dx * 0.000025;
        p.vy += dy * 0.000025;
      }

      p.x += p.vx;
      p.y += p.vy;

      if (p.x < 0) p.x += w;
      if (p.x > w) p.x -= w;
      if (p.y < 0) p.y += h;
      if (p.y > h) p.y -= h;
    }

    for (let i = this.streamers.length - 1; i >= 0; i--) {
      const s = this.streamers[i];
      s.x += s.vx;
      s.y += s.vy;
      s.life++;
      if (s.life >= s.maxLife || s.x < 0 || s.x > w || s.y < 0 || s.y > h) {
        this.streamers.splice(i, 1);
        this.spawnStreamer();
      }
    }

    this.radarAngle += 0.012;
  }

  private draw() {
    const w = this.W;
    const h = this.H;
    const ctx = this.ctx;
    const [pr, pg, pb] = this.theme.particle;

    ctx.clearRect(0, 0, w, h);

    // Connection lines
    for (let i = 0; i < this.particles.length; i++) {
      for (let j = i + 1; j < this.particles.length; j++) {
        const a = this.particles[i];
        const b = this.particles[j];
        const dx = a.x - b.x;
        const dy = a.y - b.y;
        const d = Math.sqrt(dx * dx + dy * dy);
        if (d < 100) {
          const alpha = this.theme.lineAlpha * (1 - d / 100);
          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.lineTo(b.x, b.y);
          ctx.strokeStyle = `rgba(${pr},${pg},${pb},${alpha})`;
          ctx.lineWidth = 0.5;
          ctx.stroke();
        }
      }
    }

    // Particles
    for (const p of this.particles) {
      ctx.beginPath();
      ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${pr},${pg},${pb},0.6)`;
      ctx.fill();
    }

    // Streamers with trails
    for (const s of this.streamers) {
      const color = s.colorIdx === 0 ? this.theme.streamerA : this.theme.streamerB;
      const [sr, sg, sb] = color;
      const tailLen = s.speed * 10;
      const tailX = s.x - (s.vx / s.speed) * tailLen;
      const tailY = s.y - (s.vy / s.speed) * tailLen;
      const lifeRatio = 1 - s.life / s.maxLife;

      const grad = ctx.createLinearGradient(tailX, tailY, s.x, s.y);
      grad.addColorStop(0, `rgba(${sr},${sg},${sb},0)`);
      grad.addColorStop(1, `rgba(${sr},${sg},${sb},${0.6 * lifeRatio})`);

      ctx.beginPath();
      ctx.moveTo(tailX, tailY);
      ctx.lineTo(s.x, s.y);
      ctx.strokeStyle = grad;
      ctx.lineWidth = 1.5;
      ctx.stroke();

      ctx.beginPath();
      ctx.arc(s.x, s.y, 1.5, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${sr},${sg},${sb},${0.8 * lifeRatio})`;
      ctx.fill();
    }

    // Mouse radial glow (300px radius)
    if (this.mouseX > 0 && this.mouseY > 0) {
      const glow = ctx.createRadialGradient(
        this.mouseX, this.mouseY, 0,
        this.mouseX, this.mouseY, 300,
      );
      glow.addColorStop(0, `rgba(${pr},${pg},${pb},0.08)`);
      glow.addColorStop(1, `rgba(${pr},${pg},${pb},0)`);
      ctx.fillStyle = glow;
      ctx.fillRect(0, 0, w, h);
    }

    // Radar (top-right)
    const radarX = w - 50;
    const radarY = 50;
    const radarR = 35;

    // Radar ring
    ctx.beginPath();
    ctx.arc(radarX, radarY, radarR, 0, Math.PI * 2);
    ctx.strokeStyle = `rgba(${pr},${pg},${pb},0.15)`;
    ctx.lineWidth = 1;
    ctx.stroke();

    // Inner ring
    ctx.beginPath();
    ctx.arc(radarX, radarY, radarR * 0.5, 0, Math.PI * 2);
    ctx.strokeStyle = `rgba(${pr},${pg},${pb},0.08)`;
    ctx.lineWidth = 0.5;
    ctx.stroke();

    // Radar sweep
    if (typeof ctx.createConicGradient === 'function') {
      const sweepGrad = ctx.createConicGradient(this.radarAngle, radarX, radarY);
      sweepGrad.addColorStop(0, `rgba(${pr},${pg},${pb},0.15)`);
      sweepGrad.addColorStop(0.1, `rgba(${pr},${pg},${pb},0)`);
      sweepGrad.addColorStop(1, `rgba(${pr},${pg},${pb},0)`);

      ctx.beginPath();
      ctx.moveTo(radarX, radarY);
      ctx.arc(radarX, radarY, radarR, this.radarAngle, this.radarAngle + Math.PI * 0.3);
      ctx.closePath();
      ctx.fillStyle = sweepGrad;
      ctx.fill();
    }

    // Radar scan line
    ctx.beginPath();
    ctx.moveTo(radarX, radarY);
    ctx.lineTo(
      radarX + Math.cos(this.radarAngle) * radarR,
      radarY + Math.sin(this.radarAngle) * radarR,
    );
    ctx.strokeStyle = `rgba(${pr},${pg},${pb},0.5)`;
    ctx.lineWidth = 1;
    ctx.stroke();

    // Crosshairs
    ctx.beginPath();
    ctx.moveTo(radarX - radarR, radarY);
    ctx.lineTo(radarX + radarR, radarY);
    ctx.moveTo(radarX, radarY - radarR);
    ctx.lineTo(radarX, radarY + radarR);
    ctx.strokeStyle = `rgba(${pr},${pg},${pb},0.08)`;
    ctx.lineWidth = 0.5;
    ctx.stroke();

    // Center dot
    ctx.beginPath();
    ctx.arc(radarX, radarY, 1.5, 0, Math.PI * 2);
    ctx.fillStyle = `rgba(${pr},${pg},${pb},0.5)`;
    ctx.fill();
  }
}

export const ParticleCanvas: React.FC<ParticleCanvasProps> = ({ isDark = true, className, style }) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const engineRef = useRef<ParticleEngine | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const engine = new ParticleEngine(canvas, isDark);
    engineRef.current = engine;
    engine.start();

    const ro = new ResizeObserver(() => engine.resize());
    if (canvas.parentElement) ro.observe(canvas.parentElement);

    const onVisibility = () => {
      if (document.hidden) engine.stop();
      else engine.start();
    };
    document.addEventListener('visibilitychange', onVisibility);

    return () => {
      engine.destroy();
      ro.disconnect();
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, []);

  useEffect(() => {
    engineRef.current?.setTheme(isDark ?? true);
  }, [isDark]);

  return (
    <canvas
      ref={canvasRef}
      className={className}
      style={{
        position: 'absolute',
        inset: 0,
        width: '100%',
        height: '100%',
        pointerEvents: 'none',
        ...style,
      }}
    />
  );
};
