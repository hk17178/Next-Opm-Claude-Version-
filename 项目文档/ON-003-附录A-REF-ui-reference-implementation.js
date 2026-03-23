/**
 * OpsNexus UI 参考实现代码
 * 文档编号: ON-003-附录A-REF
 * 用途: 配合 ON-003-附录A UI设计规格使用，前端工程师可直接复用
 *
 * 包含:
 * 1. ParticleNetwork — 粒子网络+流光+雷达+鼠标交互 (Canvas)
 * 2. ThemeEngine — 深色/浅色双主题切换
 * 3. FlipCard — 翻牌效果 (CSS + React)
 * 4. ScanLine — 扫描线动画 (CSS)
 * 5. NumberFlip — 数字翻牌弹入 (JS)
 * 6. AITypewriter — AI打字机效果 (JS)
 * 7. SparkLine — 实时曲线逐点绘制 (Canvas)
 * 8. RingGauge — 环形仪表从0增长 (Canvas)
 * 9. NoiseFunnel — 降噪漏斗流光 (CSS)
 * 10. 完整页面组装示例 (HTML)
 */

// ============================================================
// 1. ParticleNetwork — 粒子网络引擎
// ============================================================

class ParticleNetwork {
  constructor(canvas, options = {}) {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d');
    this.isDark = options.isDark !== false;
    this.mouseX = 0;
    this.mouseY = 0;
    this.mouseInside = false;
    this.particles = [];
    this.flows = [];
    this.radarAngle = 0;
    this.animationId = null;

    // 可配置参数 (对应附录A §1.4)
    this.config = {
      particleCount: this.isDark ? 70 : 65,
      particleMinRadius: 0.3,
      particleMaxRadius: 1.5,
      particleMinSpeed: 0.15,
      particleMaxSpeed: 0.25,
      connectionDistance: 100,
      connectionAlphaBase: this.isDark ? 0.04 : 0.03,
      flowCount: 20,
      flowMinSpeed: 0.15,
      flowMaxSpeed: 0.5,
      flowMinLife: 50,
      flowMaxLife: 180,
      flowTailMultiplier: 10,
      mouseAttractionRange: 140,
      mouseAttractionForce: 0.000025,
      mouseGlowRadius: 150,
      mouseGlowAlpha: 0.04,
      radarX: null,  // 默认 W-50
      radarY: 50,
      radarRadius: 35,
      radarSpeed: 0.012,
      radarAlpha: 0.05,
      ...options
    };

    this.resize();
    this.initParticles();
    this.initFlows();
    this.bindEvents();
  }

  // 颜色配置 (跟随主题)
  get colors() {
    if (this.isDark) {
      return {
        particle: [77, 166, 255],
        flowA: [0, 229, 160],
        flowB: [77, 166, 255],
        radar: [77, 166, 255]
      };
    } else {
      return {
        particle: [37, 99, 235],
        flowA: [16, 185, 129],
        flowB: [37, 99, 235],
        radar: [37, 99, 235]
      };
    }
  }

  resize() {
    const rect = this.canvas.parentElement.getBoundingClientRect();
    this.W = this.canvas.width = rect.width;
    this.H = this.canvas.height = rect.height;
    if (!this.config.radarX) this.config.radarX = this.W - 50;
  }

  initParticles() {
    this.particles = [];
    for (let i = 0; i < this.config.particleCount; i++) {
      this.particles.push({
        x: Math.random() * this.W,
        y: Math.random() * this.H,
        vx: (Math.random() - 0.5) * (this.config.particleMinSpeed + Math.random() * (this.config.particleMaxSpeed - this.config.particleMinSpeed)) * 2,
        vy: (Math.random() - 0.5) * (this.config.particleMinSpeed + Math.random() * (this.config.particleMaxSpeed - this.config.particleMinSpeed)) * 2,
        r: this.config.particleMinRadius + Math.random() * (this.config.particleMaxRadius - this.config.particleMinRadius),
        alpha: Math.random() * (this.isDark ? 0.18 : 0.12) + 0.03
      });
    }
  }

  initFlows() {
    this.flows = [];
    for (let i = 0; i < this.config.flowCount; i++) {
      this.flows.push(this.createFlow());
    }
  }

  createFlow() {
    return {
      x: Math.random() * this.W,
      y: Math.random() * this.H,
      speed: this.config.flowMinSpeed + Math.random() * (this.config.flowMaxSpeed - this.config.flowMinSpeed),
      angle: Math.random() * Math.PI * 2,
      life: 0,
      maxLife: this.config.flowMinLife + Math.random() * (this.config.flowMaxLife - this.config.flowMinLife),
      size: Math.random() * 1.2 + 0.4,
      colorIndex: Math.random() > 0.5 ? 0 : 1  // 0=flowA(绿), 1=flowB(蓝)
    };
  }

  bindEvents() {
    const parent = this.canvas.parentElement;
    parent.addEventListener('mousemove', (e) => {
      const rect = parent.getBoundingClientRect();
      this.mouseX = e.clientX - rect.left;
      this.mouseY = e.clientY - rect.top;
      this.mouseInside = true;
    });
    parent.addEventListener('mouseleave', () => {
      this.mouseInside = false;
    });

    // document.hidden 时暂停 (性能优化)
    document.addEventListener('visibilitychange', () => {
      if (document.hidden) {
        this.stop();
      } else {
        this.start();
      }
    });
  }

  // 主题切换 (实时生效)
  setTheme(isDark) {
    this.isDark = isDark;
    this.config.particleCount = isDark ? 70 : 65;
    this.config.connectionAlphaBase = isDark ? 0.04 : 0.03;
    // 粒子数量变化时重建
    if (this.particles.length !== this.config.particleCount) {
      this.initParticles();
    }
  }

  animate() {
    const ctx = this.ctx;
    const W = this.W, H = this.H;
    const c = this.colors;
    const pc = c.particle.join(',');
    const alphaMultiplier = this.isDark ? 1 : 0.7;

    ctx.clearRect(0, 0, W, H);

    // === 粒子运动 + 绘制 ===
    this.particles.forEach(p => {
      // 鼠标吸引力
      if (this.mouseInside) {
        const dx = this.mouseX - p.x;
        const dy = this.mouseY - p.y;
        const d = Math.sqrt(dx * dx + dy * dy);
        if (d < this.config.mouseAttractionRange) {
          p.vx += dx * this.config.mouseAttractionForce;
          p.vy += dy * this.config.mouseAttractionForce;
        }
      }

      p.x += p.vx;
      p.y += p.vy;
      if (p.x < 0 || p.x > W) p.vx *= -1;
      if (p.y < 0 || p.y > H) p.vy *= -1;

      ctx.beginPath();
      ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${pc},${p.alpha * alphaMultiplier})`;
      ctx.fill();
    });

    // === 粒子连线 ===
    const dist = this.config.connectionDistance;
    const alphaBase = this.config.connectionAlphaBase;
    for (let i = 0; i < this.particles.length; i++) {
      for (let j = i + 1; j < this.particles.length; j++) {
        const dx = this.particles[i].x - this.particles[j].x;
        const dy = this.particles[i].y - this.particles[j].y;
        const d = Math.sqrt(dx * dx + dy * dy);
        if (d < dist) {
          ctx.beginPath();
          ctx.moveTo(this.particles[i].x, this.particles[i].y);
          ctx.lineTo(this.particles[j].x, this.particles[j].y);
          ctx.strokeStyle = `rgba(${pc},${alphaBase * (1 - d / dist)})`;
          ctx.lineWidth = 0.5;
          ctx.stroke();
        }
      }
    }

    // === 数据流光 ===
    const flowColors = [c.flowA, c.flowB];
    const flowAlphaBase = this.isDark ? 0.25 : 0.15;
    const dotAlphaBase = this.isDark ? 0.4 : 0.2;

    this.flows.forEach(f => {
      f.x += Math.cos(f.angle) * f.speed;
      f.y += Math.sin(f.angle) * f.speed;
      f.life++;

      if (f.life > f.maxLife || f.x < 0 || f.x > W || f.y < 0 || f.y > H) {
        Object.assign(f, this.createFlow());
      }

      const alpha = 1 - f.life / f.maxLife;
      const col = flowColors[f.colorIndex];
      const colStr = col.join(',');
      const tailLen = f.speed * this.config.flowTailMultiplier;

      // 拖尾渐变线
      const grd = ctx.createLinearGradient(
        f.x, f.y,
        f.x - Math.cos(f.angle) * tailLen,
        f.y - Math.sin(f.angle) * tailLen
      );
      grd.addColorStop(0, `rgba(${colStr},${alpha * flowAlphaBase})`);
      grd.addColorStop(1, 'transparent');
      ctx.beginPath();
      ctx.moveTo(f.x, f.y);
      ctx.lineTo(
        f.x - Math.cos(f.angle) * tailLen,
        f.y - Math.sin(f.angle) * tailLen
      );
      ctx.strokeStyle = grd;
      ctx.lineWidth = f.size;
      ctx.stroke();

      // 头部亮点
      ctx.beginPath();
      ctx.arc(f.x, f.y, f.size * 0.35, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(${colStr},${alpha * dotAlphaBase})`;
      ctx.fill();
    });

    // === 雷达扫描 ===
    this.radarAngle += this.config.radarSpeed;
    const rcx = this.config.radarX || (W - 50);
    const rcy = this.config.radarY;
    const rr = this.config.radarRadius;
    const rAlpha = this.config.radarAlpha;
    const rc = c.radar.join(',');

    // 扫描扇形
    const grad = ctx.createConicGradient(this.radarAngle, rcx, rcy);
    grad.addColorStop(0, `rgba(${rc},${rAlpha})`);
    grad.addColorStop(0.12, 'transparent');
    grad.addColorStop(1, 'transparent');
    ctx.beginPath();
    ctx.arc(rcx, rcy, rr, 0, Math.PI * 2);
    ctx.fillStyle = grad;
    ctx.fill();

    // 同心圆
    ctx.beginPath();
    ctx.arc(rcx, rcy, rr, 0, Math.PI * 2);
    ctx.strokeStyle = `rgba(${rc},0.04)`;
    ctx.lineWidth = 0.5;
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(rcx, rcy, rr * 0.5, 0, Math.PI * 2);
    ctx.stroke();

    // 扫描线
    const sx = rcx + Math.cos(this.radarAngle) * rr;
    const sy = rcy + Math.sin(this.radarAngle) * rr;
    ctx.beginPath();
    ctx.moveTo(rcx, rcy);
    ctx.lineTo(sx, sy);
    ctx.strokeStyle = `rgba(${rc},0.08)`;
    ctx.lineWidth = 0.8;
    ctx.stroke();

    this.animationId = requestAnimationFrame(() => this.animate());
  }

  start() {
    if (!this.animationId) this.animate();
  }

  stop() {
    if (this.animationId) {
      cancelAnimationFrame(this.animationId);
      this.animationId = null;
    }
  }

  destroy() {
    this.stop();
  }
}


// ============================================================
// 2. ThemeEngine — 深色/浅色主题切换
// ============================================================

class ThemeEngine {
  constructor(rootElement, defaultTheme = 'dark') {
    this.root = rootElement;
    this.current = defaultTheme;

    this.themes = {
      dark: {
        '--bg-primary': '#060a12',
        '--bg-card': 'rgba(10,16,28,0.7)',
        '--border-color': 'rgba(60,140,255,0.05)',
        '--text-primary': '#b0c4de',
        '--text-secondary': 'rgba(140,170,210,0.35)',
        '--color-primary': '#4da6ff',
        '--color-success': '#00e5a0',
        '--color-warning': '#ffaa33',
        '--color-danger': '#ff6b6b',
        '--color-scan': 'rgba(77,166,255,0.5)',
        '--color-p0': '#ff6b6b',
        '--color-p1': '#ffaa33',
        '--color-p2': '#60a5fa',
        '--color-p3': 'rgba(140,170,210,0.35)'
      },
      light: {
        '--bg-primary': '#f4f7fc',
        '--bg-card': 'rgba(255,255,255,0.75)',
        '--border-color': 'rgba(56,120,220,0.06)',
        '--text-primary': '#2d3748',
        '--text-secondary': 'rgba(100,116,139,0.45)',
        '--color-primary': '#2563eb',
        '--color-success': '#059669',
        '--color-warning': '#ea580c',
        '--color-danger': '#dc2626',
        '--color-scan': 'rgba(37,99,235,0.25)',
        '--color-p0': '#dc2626',
        '--color-p1': '#ea580c',
        '--color-p2': '#2563eb',
        '--color-p3': 'rgba(100,116,139,0.4)'
      }
    };

    this.apply(this.current);
  }

  apply(theme) {
    this.current = theme;
    const vars = this.themes[theme];
    Object.entries(vars).forEach(([key, value]) => {
      this.root.style.setProperty(key, value);
    });
    this.root.classList.remove('dark', 'light');
    this.root.classList.add(theme);

    // 持久化
    try { localStorage.setItem('opsnexus-theme', theme); } catch(e) {}
  }

  toggle() {
    this.apply(this.current === 'dark' ? 'light' : 'dark');
    return this.current;
  }

  get isDark() { return this.current === 'dark'; }
}


// ============================================================
// 3. NumberFlip — 数字翻牌弹入动画
// ============================================================

function numberFlip(element, value, suffix = '', delayPerChar = 80) {
  element.innerHTML = '';
  element.style.display = 'inline-flex';
  element.style.overflow = 'hidden';
  element.style.height = element.parentElement ? 'auto' : '36px';
  element.style.alignItems = 'flex-end';

  const str = String(value) + suffix;
  str.split('').forEach((char, i) => {
    const span = document.createElement('span');
    span.textContent = char;
    span.style.display = 'inline-block';
    span.style.animation = `flipIn 0.6s cubic-bezier(0.16,1,0.3,1) both`;
    span.style.animationDelay = `${i * delayPerChar}ms`;
    span.style.transformOrigin = 'bottom';
    element.appendChild(span);
  });
}

// 需要配合的 CSS @keyframes:
// @keyframes flipIn {
//   from { transform: translateY(100%) rotateX(-80deg); opacity: 0; }
//   to { transform: translateY(0) rotateX(0); opacity: 1; }
// }


// ============================================================
// 4. AITypewriter — AI打字机逐字输出
// ============================================================

class AITypewriter {
  constructor(element, options = {}) {
    this.el = element;
    this.speed = options.speed || { min: 18, max: 35 };
    this.cursorClass = options.cursorClass || 'ai-cursor';
    this.index = 0;
    this.text = '';
    this.running = false;
  }

  // text 可以包含 HTML 标签 (如 <b>bold</b>)
  // 打字时安全截断: 不会截断到 HTML 标签内部
  start(text) {
    this.text = text;
    this.index = 0;
    this.running = true;
    this._tick();
  }

  _tick() {
    if (!this.running || this.index > this.text.length) {
      this.el.innerHTML = this.text;
      return;
    }

    let display = this.text.substring(0, this.index);

    // 安全截断: 如果截断到了未闭合的 HTML 标签内部,回退到标签开始前
    if (!display.endsWith('>')) {
      const lastOpen = display.lastIndexOf('<');
      const lastClose = display.lastIndexOf('>');
      if (lastOpen > lastClose) {
        display = this.text.substring(0, display.lastIndexOf('<'));
      }
    }

    this.el.innerHTML = display + `<span class="${this.cursorClass}"></span>`;
    this.index++;

    const delay = this.speed.min + Math.random() * (this.speed.max - this.speed.min);
    setTimeout(() => this._tick(), delay);
  }

  stop() {
    this.running = false;
    this.el.innerHTML = this.text;
  }
}

// 配合 CSS:
// .ai-cursor {
//   display: inline-block; width: 2px; height: 12px;
//   background: var(--color-success);
//   animation: blink 1s step-end infinite;
//   vertical-align: middle; margin-left: 2px;
// }
// @keyframes blink { 0%,100% { opacity:1 } 50% { opacity:0 } }


// ============================================================
// 5. SparkLine — 迷你折线逐点绘制 (Canvas)
// ============================================================

class SparkLine {
  constructor(canvas, options = {}) {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d');
    this.color = options.color || 'rgba(77,166,255,1)';
    this.fillAlpha = options.fillAlpha || 0.12;
    this.showPulse = options.showPulse !== false;
    this.drawSpeed = options.drawSpeed || 0.35;  // 每帧前进的数据点数
    this.data = options.data || [];
    this.progress = 0;
    this.animationId = null;
  }

  setData(data) {
    this.data = data;
    this.progress = 0;
    this.draw();
  }

  draw() {
    const ctx = this.ctx;
    const w = this.canvas.width = this.canvas.parentElement.offsetWidth;
    const h = this.canvas.height = this.canvas.parentElement.offsetHeight || 32;
    const data = this.data;

    if (data.length < 2) return;

    const visible = Math.min(Math.floor(this.progress), data.length);
    if (visible < 2) {
      this.progress += 0.5;
      this.animationId = requestAnimationFrame(() => this.draw());
      return;
    }

    ctx.clearRect(0, 0, w, h);

    // 折线
    ctx.beginPath();
    for (let i = 0; i < visible; i++) {
      const x = i / (data.length - 1) * w;
      const y = h - data[i] * h * 0.8 - h * 0.1;
      i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
    }
    ctx.strokeStyle = this.color;
    ctx.lineWidth = 1.5;
    ctx.stroke();

    // 面积填充
    const lastIdx = visible - 1;
    const lastX = lastIdx / (data.length - 1) * w;
    ctx.lineTo(lastX, h);
    ctx.lineTo(0, h);
    ctx.closePath();
    const gradient = ctx.createLinearGradient(0, 0, 0, h);
    gradient.addColorStop(0, this.color.replace('1)', `${this.fillAlpha})`).replace('rgb', 'rgba'));
    gradient.addColorStop(1, 'transparent');
    ctx.fillStyle = gradient;
    ctx.fill();

    // 末端脉冲点
    if (this.showPulse && visible === data.length) {
      const lx = lastX;
      const ly = h - data[lastIdx] * h * 0.8 - h * 0.1;
      ctx.beginPath();
      ctx.arc(lx, ly, 2.5, 0, Math.PI * 2);
      ctx.fillStyle = this.color;
      ctx.fill();
      // 光晕
      ctx.beginPath();
      ctx.arc(lx, ly, 5, 0, Math.PI * 2);
      ctx.fillStyle = this.color.replace('1)', '0.15)').replace('rgb', 'rgba');
      ctx.fill();
    }

    if (this.progress < data.length + 8) {
      this.progress += this.drawSpeed;
      this.animationId = requestAnimationFrame(() => this.draw());
    }
  }

  destroy() {
    if (this.animationId) cancelAnimationFrame(this.animationId);
  }
}


// ============================================================
// 6. RingGauge — 环形仪表从0增长 (Canvas)
// ============================================================

class RingGauge {
  constructor(canvas, options = {}) {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d');
    this.targetValue = options.value || 0.9997;  // 0~1
    this.currentValue = 0;
    this.growSpeed = options.growSpeed || 0.012;
    this.lineWidth = options.lineWidth || 5;
    this.radius = options.radius || 44;
    this.cx = canvas.width / 2;
    this.cy = canvas.height / 2;
    this.isDark = options.isDark !== false;
    this.labelElement = options.labelElement;
    this.animationId = null;
  }

  start() {
    this.currentValue = 0;
    this._draw();
  }

  _draw() {
    const ctx = this.ctx;
    const cx = this.cx, cy = this.cy, r = this.radius, lw = this.lineWidth;
    const pct = Math.min(this.currentValue, this.targetValue);

    ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

    // 底环
    ctx.beginPath();
    ctx.arc(cx, cy, r, 0, Math.PI * 2);
    ctx.strokeStyle = this.isDark ? 'rgba(60,140,255,0.04)' : 'rgba(37,99,235,0.06)';
    ctx.lineWidth = lw;
    ctx.stroke();

    // 进度环 (渐变色)
    const g = ctx.createLinearGradient(10, 10, this.canvas.width - 10, this.canvas.height - 10);
    g.addColorStop(0, this.isDark ? '#00e5a0' : '#10b981');
    g.addColorStop(1, this.isDark ? '#4da6ff' : '#2563eb');
    ctx.beginPath();
    ctx.arc(cx, cy, r, -Math.PI / 2, -Math.PI / 2 + Math.PI * 2 * pct);
    ctx.strokeStyle = g;
    ctx.lineWidth = lw;
    ctx.lineCap = 'round';
    ctx.stroke();

    // 更新文字
    if (this.labelElement) {
      this.labelElement.textContent = (pct * 100).toFixed(2);
    }

    if (this.currentValue < this.targetValue) {
      this.currentValue += this.growSpeed;
      this.animationId = requestAnimationFrame(() => this._draw());
    }
  }

  setTheme(isDark) {
    this.isDark = isDark;
  }

  destroy() {
    if (this.animationId) cancelAnimationFrame(this.animationId);
  }
}


// ============================================================
// 7. 鼠标跟随光晕
// ============================================================

class MouseGlow {
  constructor(glowElement, container, options = {}) {
    this.el = glowElement;
    this.container = container;
    this.size = options.size || 300;
    this.isDark = options.isDark !== false;

    this.el.style.cssText = `
      position: absolute;
      width: ${this.size}px; height: ${this.size}px;
      border-radius: 50%;
      pointer-events: none;
      z-index: 0;
      transition: opacity 0.3s;
      opacity: 0;
    `;
    this.updateTheme();

    this.container.addEventListener('mousemove', (e) => {
      const rect = this.container.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      this.el.style.left = (x - this.size / 2) + 'px';
      this.el.style.top = (y - this.size / 2) + 'px';
      this.el.style.opacity = '1';
    });

    this.container.addEventListener('mouseleave', () => {
      this.el.style.opacity = '0';
    });
  }

  updateTheme() {
    const color = this.isDark ? 'rgba(77,166,255,0.04)' : 'rgba(37,99,235,0.04)';
    this.el.style.background = `radial-gradient(circle, ${color} 0%, transparent 70%)`;
  }

  setTheme(isDark) {
    this.isDark = isDark;
    this.updateTheme();
  }
}


// ============================================================
// 8. CSS 动画集 (扫描线/漏斗流光/呼吸脉冲/翻牌)
//    直接写入 <style> 标签即可
// ============================================================

const OPSNEXUS_CSS_ANIMATIONS = `
/* 扫描线 — 卡片顶部 */
@keyframes scanline {
  0% { left: -60%; opacity: 0; }
  10% { opacity: 1; }
  90% { opacity: 1; }
  100% { left: 160%; opacity: 0; }
}
.cd-scan {
  position: absolute; top: 0; left: -80%; width: 60%; height: 2px;
  background: linear-gradient(90deg, transparent, var(--color-scan), transparent);
  animation: scanline 4s ease-in-out infinite;
}
.cd-scan-delay-1 { animation-delay: 1s; }
.cd-scan-delay-2 { animation-delay: 2s; }
.cd-scan-delay-3 { animation-delay: 3s; }

/* 数字翻牌弹入 */
@keyframes flipIn {
  from { transform: translateY(100%) rotateX(-80deg); opacity: 0; }
  to { transform: translateY(0) rotateX(0); opacity: 1; }
}

/* 漏斗流光 */
@keyframes funnelFlow {
  0% { left: -30%; }
  100% { left: 130%; }
}
.funnel-flow {
  position: absolute; top: 0; left: -30%; width: 30%; height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255,255,255,0.2), transparent);
  animation: funnelFlow 2s linear infinite;
}

/* LIVE呼吸灯 */
@keyframes breathe {
  0%, 100% { opacity: 1; box-shadow: 0 0 10px var(--color-success); }
  50% { opacity: 0.4; box-shadow: 0 0 3px var(--color-success); }
}
.live-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--color-success);
  animation: breathe 3s ease infinite;
}

/* P0呼吸脉冲 */
@keyframes cellPulse {
  0%, 100% { box-shadow: inset 0 0 6px rgba(255,70,70,0.06); }
  50% { box-shadow: inset 0 0 14px rgba(255,70,70,0.15); }
}

/* AI光标闪烁 */
@keyframes cursorBlink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}
.ai-cursor {
  display: inline-block; width: 2px; height: 12px;
  background: var(--color-success);
  animation: cursorBlink 1s step-end infinite;
  vertical-align: middle; margin-left: 2px;
}

/* 翻牌 */
.flip-card { perspective: 800px; }
.flip-inner {
  position: relative; width: 100%; height: 100%;
  transition: transform 0.6s cubic-bezier(0.4, 0, 0.2, 1);
  transform-style: preserve-3d;
}
.flip-card:hover .flip-inner { transform: rotateY(180deg); }
.flip-front, .flip-back {
  position: absolute; inset: 0;
  backface-visibility: hidden;
  border-radius: 12px;
}
.flip-back { transform: rotateY(180deg); }

/* 主题过渡 */
.app { transition: background 0.4s ease, color 0.4s ease; }
.app * { transition: background-color 0.4s ease, border-color 0.4s ease, color 0.4s ease, box-shadow 0.4s ease; }

/* prefers-reduced-motion */
@media (prefers-reduced-motion: reduce) {
  .cd-scan, .funnel-flow, .live-dot, .ai-cursor { animation: none !important; }
  .flip-inner { transition: none !important; }
  .flip-card:hover .flip-inner { transform: none !important; }
}
`;


// ============================================================
// 导出 (ES Module 或全局)
// ============================================================

if (typeof module !== 'undefined' && module.exports) {
  module.exports = {
    ParticleNetwork, ThemeEngine, NumberFlip: numberFlip,
    AITypewriter, SparkLine, RingGauge, MouseGlow,
    OPSNEXUS_CSS_ANIMATIONS
  };
}

if (typeof window !== 'undefined') {
  window.OpsNexusUI = {
    ParticleNetwork, ThemeEngine, NumberFlip: numberFlip,
    AITypewriter, SparkLine, RingGauge, MouseGlow,
    OPSNEXUS_CSS_ANIMATIONS
  };
}
