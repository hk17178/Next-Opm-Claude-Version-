import React from 'react';
import {
  MetricFlipCard,
  HealthMatrix,
  NoiseFunnel,
  AITypewriter,
  LiveDot,
  FlipCard,
  RingGauge,
} from '@opsnexus/ui-kit';
import type { HealthCell } from '@opsnexus/ui-kit';

/* ------------------------------------------------------------------ */
/*  Mock data                                                         */
/* ------------------------------------------------------------------ */

/** 业务健康矩阵 mock — 匹配 demo 的 3行×5列 */
function generateHealthCells(): HealthCell[] {
  const rows = ['pay', 'shop', 'risk'];
  const cols = ['NET', 'HOST', 'APP', 'DB', 'MW'];
  // 模拟 demo 中的固定状态分布
  const statusMap: Record<string, HealthCell['status']> = {
    'pay-NET': 'critical', 'pay-HOST': 'critical', 'pay-APP': 'degraded', 'pay-DB': 'degraded', 'pay-MW': 'ok',
    'shop-NET': 'ok', 'shop-HOST': 'ok', 'shop-APP': 'ok', 'shop-DB': 'ok', 'shop-MW': 'ok',
    'risk-NET': 'ok', 'risk-HOST': 'ok', 'risk-APP': 'ok', 'risk-DB': 'ok', 'risk-MW': 'ok',
  };
  const detailsMap: Record<string, HealthCell['details']> = {
    'pay-NET': { cpu: 0 }, 'pay-HOST': { cpu: 95 }, 'pay-APP': { conn: 200 }, 'pay-DB': { disk: 87 }, 'pay-MW': { conn: 3 },
    'shop-NET': { cpu: 100 }, 'shop-HOST': { cpu: 42 }, 'shop-APP': { conn: 80 }, 'shop-DB': { disk: 61 }, 'shop-MW': { conn: 2 },
    'risk-NET': { cpu: 100 }, 'risk-HOST': { cpu: 38 }, 'risk-APP': { conn: 45 }, 'risk-DB': { disk: 55 }, 'risk-MW': { conn: 1 },
  };

  return rows.flatMap((row) =>
    cols.map((col) => {
      const key = `${row}-${col}`;
      return {
        id: key,
        label: `${row}/${col}`,
        status: statusMap[key] || 'ok',
        details: detailsMap[key],
      } satisfies HealthCell;
    }),
  );
}

const funnelData = [
  { label: 'raw', value: 175 },
  { label: 'filtered', value: 140 },
  { label: 'merged', value: 72 },
  { label: 'AI cut', value: 47 },
  { label: 'human', value: 47 },
];

interface AlertRow {
  id: string;
  severity: 'P0' | 'P1' | 'P2' | 'P3';
  title: string;
  host: string;
  duration: string;
}

const activeAlerts: AlertRow[] = [
  { id: '1', severity: 'P0', title: 'server-pay-01 unreachable', host: 'payment', duration: '15m' },
  { id: '2', severity: 'P1', title: 'CPU 95.3% > threshold', host: 'payment', duration: '15m' },
  { id: '3', severity: 'P1', title: 'MySQL slow queries 200/min', host: 'payment', duration: '17m' },
  { id: '4', severity: 'P2', title: 'disk usage +35% baseline', host: 'analytics', duration: '19m' },
  { id: '5', severity: 'P3', title: 'API P99 latency WoW +25%', host: 'e-comm', duration: '29m' },
];

const aiSummaryText =
  'Root cause: human-triggered cascade. Operator wanghao executed interface shutdown on switch-core-01 GE0/0/1 at 10:12:03 without change order. ' +
  'This caused connectivity loss for 3 downstream payment hosts, triggering CPU spike from retry storms. ' +
  'Recommend: immediate port re-enable + post-incident review.';

interface OnCallMember {
  initials: string;
  name: string;
  role: string;
  status: string;
  statusColor: string;
  avatarBg: string;
  phone: string;
  backLabel2: string;
  backValue2: string;
  backLabel3: string;
  backValue3: string;
}

const onCallTeam: OnCallMember[] = [
  { initials: 'ZW', name: 'zhangwei', role: 'primary', status: 'P0', statusColor: 'var(--color-danger, #ff6b6b)', avatarBg: 'rgba(255,70,70,0.06)', phone: '138****6721', backLabel2: 'last resp', backValue2: '2 min', backLabel3: 'today', backValue3: '6 handled' },
  { initials: 'CJ', name: 'chenjing', role: 'backup', status: 'standby', statusColor: 'var(--color-success, #00e5a0)', avatarBg: 'rgba(0,229,160,0.06)', phone: '139****8834', backLabel2: 'last resp', backValue2: '8 min', backLabel3: 'today', backValue3: '2 handled' },
  { initials: 'LM', name: 'liming', role: 'supervisor', status: 'online', statusColor: 'var(--color-primary, #4da6ff)', avatarBg: 'rgba(77,166,255,0.06)', phone: '137****4412', backLabel2: 'escalated', backValue2: '1 P0', backLabel3: 'approved', backValue3: '3 changes' },
];

/* ------------------------------------------------------------------ */
/*  Severity styling                                                   */
/* ------------------------------------------------------------------ */

const SEVERITY_STYLES: Record<string, { bg: string; fg: string }> = {
  P0: { bg: 'rgba(255,70,70,0.1)', fg: 'var(--color-danger, #ff6b6b)' },
  P1: { bg: 'rgba(255,170,50,0.08)', fg: 'var(--color-warning, #ffaa33)' },
  P2: { bg: 'rgba(77,166,255,0.08)', fg: 'var(--color-primary, #60a5fa)' },
  P3: { bg: 'rgba(100,130,170,0.06)', fg: 'var(--text-secondary, rgba(140,170,210,0.35))' },
};

/* ------------------------------------------------------------------ */
/*  Shared styles                                                      */
/* ------------------------------------------------------------------ */

const panelStyle: React.CSSProperties = {
  background: 'var(--bg-card, rgba(10,16,28,0.5))',
  border: '1px solid var(--border-color, rgba(60,140,255,0.04))',
  borderRadius: 12,
  overflow: 'hidden',
};

const panelHeader: React.CSSProperties = {
  padding: '10px 14px',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  borderBottom: '1px solid var(--border-color, rgba(60,140,255,0.04))',
};

const panelTitle: React.CSSProperties = {
  fontSize: 11,
  fontWeight: 500,
  color: 'var(--text-primary, rgba(200,220,240,0.6))',
};

const mutedText: React.CSSProperties = {
  fontSize: 8,
  color: 'var(--text-secondary, rgba(140,170,210,0.3))',
};

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

const Home: React.FC = () => {
  const healthCells = React.useMemo(() => generateHealthCells(), []);

  return (
    <div style={{ padding: 12 }}>
      {/* -------- A. 4 张翻牌指标卡 — demo .cards -------- */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 8, marginBottom: 10 }}>
        <MetricFlipCard
          label="FIRING ALERTS"
          value={12}
          trend="up"
          trendValue="+3 vs yesterday"
          color="var(--color-danger, #ff6b6b)"
          scanColor="rgba(255,70,70,0.4)"
          backItems={[
            { label: 'P0', value: 1 },
            { label: 'P1', value: 4 },
            { label: 'P2', value: 5 },
            { label: 'P3', value: 2 },
            { label: 'peak today', value: '18 (09:30)' },
          ]}
        />
        <MetricFlipCard
          label="RESOLVED TODAY"
          value={35}
          trend="down"
          trendValue="+5 vs yesterday"
          color="var(--color-success, #00e5a0)"
          scanColor="rgba(0,229,160,0.4)"
          backItems={[
            { label: 'auto-resolved', value: 22 },
            { label: 'manual ack', value: 13 },
            { label: 'avg resolve', value: '14 min' },
            { label: 'MTTR trend', value: '-18%' },
          ]}
        />
        <MetricFlipCard
          label="NOISE REDUCTION"
          value="73%"
          color="var(--color-primary, #4da6ff)"
          scanColor="rgba(77,166,255,0.4)"
          backItems={[
            { label: 'maint window', value: 35 },
            { label: 'convergence', value: 68 },
            { label: 'baseline', value: 12 },
            { label: 'AI suppress', value: 13 },
            { label: 'false +', value: '2.1%' },
          ]}
        />
        <MetricFlipCard
          label="MTTR (AVG)"
          value="18min"
          color="var(--color-success, #00e5a0)"
          scanColor="rgba(0,229,160,0.4)"
          backItems={[
            { label: 'detect', value: '2 min' },
            { label: 'triage', value: '5 min' },
            { label: 'fix', value: '8 min' },
            { label: 'verify', value: '3 min' },
            { label: 'P0 MTTR', value: '32 min' },
          ]}
        />
      </div>

      {/* -------- B. 降噪漏斗流水线 — demo .fn -------- */}
      <NoiseFunnel layers={funnelData} />

      {/* -------- C. 主内容区 两栏布局 — demo .main -------- */}
      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 260px', gap: 8, marginTop: 8 }}>
        {/* 左栏：告警表格 + AI 分析 */}
        <div style={panelStyle}>
          <div style={panelHeader}>
            <span style={panelTitle}>active alerts</span>
            <span
              style={{
                fontSize: 8,
                padding: '2px 9px',
                borderRadius: 10,
                background: 'rgba(255,70,70,0.06)',
                color: 'var(--color-danger, #ff6b6b)',
              }}
            >
              12 firing
            </span>
          </div>
          {/* 告警表格 — demo .tbl */}
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <tbody>
              {activeAlerts.map((alert) => {
                const sev = SEVERITY_STYLES[alert.severity] || SEVERITY_STYLES.P3;
                return (
                  <tr
                    key={alert.id}
                    style={{
                      cursor: 'pointer',
                      boxShadow: alert.severity === 'P0' ? `inset 3px 0 0 ${sev.fg}` : undefined,
                    }}
                  >
                    <td style={{ padding: '7px 12px', fontSize: 10, borderBottom: '1px solid var(--border-color, rgba(60,140,255,0.025))' }}>
                      <span
                        style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          padding: '2px 8px',
                          borderRadius: 4,
                          fontSize: 8,
                          fontWeight: 600,
                          minWidth: 28,
                          background: sev.bg,
                          color: sev.fg,
                        }}
                      >
                        {alert.severity}
                      </span>
                    </td>
                    <td style={{ padding: '7px 12px', fontSize: 10, borderBottom: '1px solid var(--border-color, rgba(60,140,255,0.025))', color: 'var(--text-primary, #e2e8f0)' }}>
                      {alert.title}
                    </td>
                    <td style={{ padding: '7px 12px', fontSize: 9, borderBottom: '1px solid var(--border-color, rgba(60,140,255,0.025))', color: 'var(--color-primary, rgba(77,166,255,0.6))' }}>
                      {alert.host}
                    </td>
                    <td style={{ padding: '7px 12px', fontSize: 9, borderBottom: '1px solid var(--border-color, rgba(60,140,255,0.025))', fontFamily: 'ui-monospace, monospace', color: 'var(--text-secondary, rgba(255,170,50,0.5))' }}>
                      {alert.duration}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>

          {/* AI 根因分析 — demo .ai */}
          <div
            style={{
              margin: '8px 12px 10px',
              borderRadius: '0 10px 10px 0',
              padding: '10px 14px',
              position: 'relative',
              background: 'linear-gradient(135deg, rgba(0,229,160,0.03), var(--bg-card, rgba(10,16,28,0.6)))',
              border: '1px solid rgba(0,229,160,0.06)',
              borderLeft: '2px solid rgba(0,229,160,0.3)',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 10, fontWeight: 500, marginBottom: 5, color: 'var(--color-success, #00e5a0)' }}>
              <LiveDot size={5} label="" />
              <span>AI root cause analysis</span>
              <span
                style={{
                  fontSize: 8,
                  padding: '2px 7px',
                  borderRadius: 4,
                  fontWeight: 400,
                  background: 'rgba(0,229,160,0.08)',
                  color: 'var(--color-success, #00e5a0)',
                }}
              >
                95%
              </span>
            </div>
            <div style={{ fontSize: 10, lineHeight: 1.7, color: 'var(--text-secondary, rgba(160,185,215,0.55))' }}>
              <AITypewriter text={aiSummaryText} speed={20} />
            </div>
          </div>
        </div>

        {/* 右栏：SLA 环形图 + 健康矩阵 + 值班团队 */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {/* SLA 环形仪表盘 — demo .ring-box */}
          <div style={{ ...panelStyle, display: 'flex', flexDirection: 'column', alignItems: 'center', padding: 14 }}>
            <span style={panelTitle}>platform SLA</span>
            <div style={{ position: 'relative', width: 110, height: 110, margin: '4px 0' }}>
              <RingGauge value={99.97} max={100} size={110} strokeWidth={5} />
              <div
                style={{
                  position: 'absolute',
                  top: '50%',
                  left: '50%',
                  transform: 'translate(-50%, -50%)',
                  textAlign: 'center',
                  pointerEvents: 'none',
                }}
              >
                <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--color-success, #00e5a0)' }}>99.97</div>
                <div style={{ fontSize: 8, color: 'var(--text-secondary, rgba(140,170,210,0.35))' }}>% uptime</div>
              </div>
            </div>
          </div>

          {/* 业务健康矩阵 — demo .hg */}
          <div style={panelStyle}>
            <div style={panelHeader}>
              <span style={panelTitle}>business health matrix</span>
              <span style={mutedText}>hover to flip</span>
            </div>
            <HealthMatrix
              cells={healthCells}
              rows={3}
              cols={5}
              cellHeight={24}
              colLabels={['NET', 'HOST', 'APP', 'DB', 'MW']}
              rowLabels={['pay', 'shop', 'risk']}
            />
          </div>

          {/* 值班团队 — demo .tm-flip */}
          <div style={panelStyle}>
            <div style={panelHeader}>
              <span style={panelTitle}>on-call team</span>
              <span style={mutedText}>hover to flip</span>
            </div>
            {onCallTeam.map((m, i) => (
              <div key={m.name} style={{ height: 36, marginBottom: i < onCallTeam.length - 1 ? 0 : 6 }}>
                <FlipCard
                  duration={500}
                  front={
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '2px 12px', fontSize: 9, height: '100%' }}>
                      {/* 头像 */}
                      <div
                        style={{
                          width: 26,
                          height: 26,
                          borderRadius: 8,
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          fontSize: 9,
                          fontWeight: 600,
                          flexShrink: 0,
                          background: m.avatarBg,
                          color: m.statusColor,
                        }}
                      >
                        {m.initials}
                      </div>
                      {/* 名字 + 角色 */}
                      <div style={{ flex: 1 }}>
                        <b style={{ display: 'block', fontSize: 10, fontWeight: 500, color: 'var(--text-primary, rgba(200,220,240,0.8))' }}>{m.name}</b>
                        <span style={{ color: 'var(--text-secondary, rgba(180,200,225,0.5))' }}>{m.role}</span>
                      </div>
                      {/* 状态标签 */}
                      <span
                        style={{
                          fontSize: 8,
                          padding: '2px 7px',
                          borderRadius: 8,
                          background: m.avatarBg,
                          color: m.statusColor,
                        }}
                      >
                        {m.status}
                      </span>
                    </div>
                  }
                  back={
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 12, padding: '2px 12px', fontFamily: 'ui-monospace, monospace', fontSize: 9, height: '100%' }}>
                      <div style={{ textAlign: 'center' }}>
                        <div style={{ fontSize: 7, color: 'var(--text-secondary, rgba(140,170,210,0.35))' }}>phone</div>
                        <div style={{ color: 'var(--text-primary, rgba(200,220,240,0.7))' }}>{m.phone}</div>
                      </div>
                      <div style={{ textAlign: 'center' }}>
                        <div style={{ fontSize: 7, color: 'var(--text-secondary, rgba(140,170,210,0.35))' }}>{m.backLabel2}</div>
                        <div style={{ color: 'var(--text-primary, rgba(200,220,240,0.7))' }}>{m.backValue2}</div>
                      </div>
                      <div style={{ textAlign: 'center' }}>
                        <div style={{ fontSize: 7, color: 'var(--text-secondary, rgba(140,170,210,0.35))' }}>{m.backLabel3}</div>
                        <div style={{ color: 'var(--text-primary, rgba(200,220,240,0.7))' }}>{m.backValue3}</div>
                      </div>
                    </div>
                  }
                />
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};

export default Home;
