import React, { useEffect, useRef } from 'react';
import * as echarts from 'echarts';
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
  { label: '原始告警', value: 175 },
  { label: '维护过滤', value: 140 },
  { label: '合并聚合', value: 72 },
  { label: 'AI 抑制', value: 47 },
  { label: '人工处理', value: 47 },
];

interface AlertRow {
  id: string;
  severity: 'P0' | 'P1' | 'P2' | 'P3';
  title: string;
  host: string;
  duration: string;
}

const activeAlerts: AlertRow[] = [
  { id: '1', severity: 'P0', title: '支付服务器 pay-01 不可达', host: 'payment', duration: '15m' },
  { id: '2', severity: 'P1', title: 'CPU 95.3% 超过阈值', host: 'payment', duration: '15m' },
  { id: '3', severity: 'P1', title: 'MySQL 慢查询 200次/分钟', host: 'payment', duration: '17m' },
  { id: '4', severity: 'P2', title: '磁盘使用量超基线 +35%', host: 'analytics', duration: '19m' },
  { id: '5', severity: 'P3', title: 'API P99 延迟环比 +25%', host: 'e-comm', duration: '29m' },
];

/* 24h 告警趋势 mock 数据 — 每小时一个点 */
const hours24 = Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:00`);
const rawAlertData = [
  12, 8, 6, 5, 4, 3, 5, 14, 28, 35, 32, 26, 22, 18, 20, 25, 30, 27, 22, 18, 15, 14, 13, 12,
];
const effectiveAlertData = [
  5, 3, 2, 2, 1, 1, 2, 6, 12, 18, 16, 12, 10, 8, 9, 11, 14, 12, 10, 8, 6, 5, 5, 5,
];

/* 最近事件时间线 mock 数据 */
interface TimelineEvent {
  id: string;
  time: string;
  title: string;
  severity: 'P0' | 'P1' | 'P2';
  status: '处理中' | '已解决' | '待处理';
}

const recentEvents: TimelineEvent[] = [
  { id: 'e1', time: '10:12', title: '支付服务器 pay-01 不可达 — 网络端口被关闭', severity: 'P0', status: '处理中' },
  { id: 'e2', time: '10:15', title: '支付集群 CPU 飙升 95.3% — 重试风暴级联', severity: 'P1', status: '处理中' },
  { id: 'e3', time: '09:47', title: 'pay-db-02 MySQL 慢查询超过 200次/分钟', severity: 'P1', status: '已解决' },
  { id: 'e4', time: '09:30', title: '分析平台磁盘使用量异常增长 +35%', severity: 'P2', status: '待处理' },
  { id: 'e5', time: '08:55', title: '电商 API P99 延迟环比上升 25%', severity: 'P2', status: '已解决' },
];

const TIMELINE_DOT_COLORS: Record<string, string> = {
  P0: '#ff6b6b',
  P1: '#ffaa33',
  P2: '#4da6ff',
};

const TIMELINE_STATUS_STYLES: Record<string, { bg: string; fg: string }> = {
  '处理中': { bg: 'rgba(255,170,50,0.1)', fg: '#ffaa33' },
  '已解决': { bg: 'rgba(0,229,160,0.1)', fg: '#00e5a0' },
  '待处理': { bg: 'rgba(77,166,255,0.1)', fg: '#4da6ff' },
};

const aiSummaryText =
  '根因分析：人为触发的级联故障。运维人员王浩在 10:12:03 未提交变更工单的情况下，对核心交换机 switch-core-01 的 GE0/0/1 端口执行了关闭操作。' +
  '这导致 3 台下游支付服务器失去连接，引发重试风暴导致 CPU 飙升。' +
  '建议：立即重新启用端口 + 启动事后复盘流程。';

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
  { initials: 'ZW', name: '张伟', role: '主值班', status: 'P0', statusColor: 'var(--color-danger, #ff6b6b)', avatarBg: 'rgba(255,70,70,0.06)', phone: '138****6721', backLabel2: '最近响应', backValue2: '2 分钟', backLabel3: '今日', backValue3: '处理 6 条' },
  { initials: 'CJ', name: '陈静', role: '副值班', status: '待命', statusColor: 'var(--color-success, #00e5a0)', avatarBg: 'rgba(0,229,160,0.06)', phone: '139****8834', backLabel2: '最近响应', backValue2: '8 分钟', backLabel3: '今日', backValue3: '处理 2 条' },
  { initials: 'LM', name: '李明', role: '主管', status: '在线', statusColor: 'var(--color-primary, #4da6ff)', avatarBg: 'rgba(77,166,255,0.06)', phone: '137****4412', backLabel2: '已升级', backValue2: '1 P0', backLabel3: '已审批', backValue3: '3 个变更' },
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

  /* ---------- 24h 告警趋势 ECharts ---------- */
  const chartRef = useRef<HTMLDivElement>(null);
  const chartInstance = useRef<echarts.ECharts | null>(null);

  useEffect(() => {
    if (!chartRef.current) return;
    const chart = echarts.init(chartRef.current);
    chartInstance.current = chart;

    chart.setOption({
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(10,16,28,0.9)',
        borderColor: 'rgba(60,140,255,0.15)',
        textStyle: { color: '#e2e8f0', fontSize: 11 },
      },
      legend: {
        data: ['原始告警', '有效告警'],
        top: 6,
        right: 14,
        textStyle: { color: 'rgba(140,170,210,0.5)', fontSize: 9 },
        itemWidth: 12,
        itemHeight: 3,
      },
      grid: { left: 36, right: 14, top: 34, bottom: 22 },
      xAxis: {
        type: 'category',
        data: hours24,
        boundaryGap: false,
        axisLine: { lineStyle: { color: 'rgba(60,140,255,0.08)' } },
        axisLabel: { color: 'rgba(140,170,210,0.4)', fontSize: 8, interval: 3 },
        axisTick: { show: false },
      },
      yAxis: {
        type: 'value',
        splitLine: { lineStyle: { color: 'rgba(60,140,255,0.05)' } },
        axisLine: { show: false },
        axisLabel: { color: 'rgba(140,170,210,0.4)', fontSize: 8 },
      },
      series: [
        {
          name: '原始告警',
          type: 'line',
          data: rawAlertData,
          smooth: true,
          symbol: 'none',
          lineStyle: { color: 'rgba(140,170,210,0.5)', width: 1.5 },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(140,170,210,0.12)' },
              { offset: 1, color: 'rgba(140,170,210,0.01)' },
            ]),
          },
        },
        {
          name: '有效告警',
          type: 'line',
          data: effectiveAlertData,
          smooth: true,
          symbol: 'none',
          lineStyle: { color: '#4da6ff', width: 2 },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(77,166,255,0.25)' },
              { offset: 1, color: 'rgba(77,166,255,0.02)' },
            ]),
          },
        },
      ],
    });

    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);
    return () => {
      window.removeEventListener('resize', handleResize);
      chart.dispose();
    };
  }, []);

  return (
    <div style={{ padding: 12 }}>
      {/* -------- A. 4 张翻牌指标卡 — demo .cards -------- */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 8, marginBottom: 10 }}>
        <MetricFlipCard
          label="当前告警"
          value={12}
          trend="up"
          trendValue="+3 较昨日"
          color="var(--color-danger, #ff6b6b)"
          scanColor="rgba(255,70,70,0.4)"
          backItems={[
            { label: 'P0', value: 1 },
            { label: 'P1', value: 4 },
            { label: 'P2', value: 5 },
            { label: 'P3', value: 2 },
            { label: '今日峰值', value: '18 (09:30)' },
          ]}
        />
        <MetricFlipCard
          label="今日已解决"
          value={35}
          trend="down"
          trendValue="+5 较昨日"
          color="var(--color-success, #00e5a0)"
          scanColor="rgba(0,229,160,0.4)"
          backItems={[
            { label: '自动恢复', value: 22 },
            { label: '人工确认', value: 13 },
            { label: '平均解决', value: '14 min' },
            { label: 'MTTR 趋势', value: '-18%' },
          ]}
        />
        <MetricFlipCard
          label="降噪率"
          value="73%"
          color="var(--color-primary, #4da6ff)"
          scanColor="rgba(77,166,255,0.4)"
          backItems={[
            { label: '维护窗口', value: 35 },
            { label: '收敛合并', value: 68 },
            { label: '基线过滤', value: 12 },
            { label: 'AI 抑制', value: 13 },
            { label: '误报率', value: '2.1%' },
          ]}
        />
        <MetricFlipCard
          label="MTTR (AVG)"
          value="18min"
          color="var(--color-success, #00e5a0)"
          scanColor="rgba(0,229,160,0.4)"
          backItems={[
            { label: '检测', value: '2 min' },
            { label: '定位', value: '5 min' },
            { label: '修复', value: '8 min' },
            { label: '验证', value: '3 min' },
            { label: 'P0 MTTR', value: '32 min' },
          ]}
        />
      </div>

      {/* -------- B. 降噪漏斗流水线 — demo .fn -------- */}
      <NoiseFunnel layers={funnelData} />

      {/* -------- C. 主内容区 两栏布局 — demo .main -------- */}
      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 520px', gap: 8, marginTop: 8 }}>
        {/* 左栏：趋势图 + 告警表格 + 事件时间线 + AI 分析 */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>

          {/* D. 24h 告警趋势折线图 */}
          <div style={panelStyle}>
            <div style={panelHeader}>
              <span style={panelTitle}>告警趋势 24h</span>
              <span style={mutedText}>原始 vs 有效</span>
            </div>
            <div ref={chartRef} style={{ width: '100%', height: 180 }} />
          </div>

          {/* E. 告警表格 + 事件时间线 */}
          <div style={panelStyle}>
            <div style={panelHeader}>
              <span style={panelTitle}>活跃告警</span>
              <span
                style={{
                  fontSize: 8,
                  padding: '2px 9px',
                  borderRadius: 10,
                  background: 'rgba(255,70,70,0.06)',
                  color: 'var(--color-danger, #ff6b6b)',
                }}
              >
                12 条触发中
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

            {/* F. 最近事件时间线 */}
            <div style={{ borderTop: '1px solid var(--border-color, rgba(60,140,255,0.04))', padding: '8px 0 4px' }}>
              <div style={{ ...panelHeader, borderBottom: 'none', paddingBottom: 4 }}>
                <span style={panelTitle}>最近事件</span>
                <span style={mutedText}>最近 5 条</span>
              </div>
              <div style={{ padding: '0 14px 8px', position: 'relative' }}>
                {recentEvents.map((evt, idx) => {
                  const dotColor = TIMELINE_DOT_COLORS[evt.severity] || '#4da6ff';
                  const statusStyle = TIMELINE_STATUS_STYLES[evt.status] || TIMELINE_STATUS_STYLES['待处理'];
                  const isLast = idx === recentEvents.length - 1;
                  return (
                    <div
                      key={evt.id}
                      style={{
                        display: 'flex',
                        alignItems: 'flex-start',
                        gap: 10,
                        position: 'relative',
                        paddingBottom: isLast ? 0 : 10,
                        cursor: 'pointer',
                      }}
                    >
                      {/* 竖线 + 圆点 */}
                      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', width: 12, flexShrink: 0, paddingTop: 2 }}>
                        <div
                          style={{
                            width: 8,
                            height: 8,
                            borderRadius: '50%',
                            background: dotColor,
                            boxShadow: `0 0 6px ${dotColor}55`,
                            flexShrink: 0,
                          }}
                        />
                        {!isLast && (
                          <div
                            style={{
                              width: 1.5,
                              flex: 1,
                              minHeight: 16,
                              background: `linear-gradient(to bottom, ${dotColor}44, rgba(60,140,255,0.06))`,
                            }}
                          />
                        )}
                      </div>
                      {/* 事件内容 */}
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 2 }}>
                          <span style={{ fontSize: 8, fontFamily: 'ui-monospace, monospace', color: 'var(--text-secondary, rgba(140,170,210,0.4))' }}>
                            {evt.time}
                          </span>
                          <span
                            style={{
                              fontSize: 7,
                              padding: '1px 5px',
                              borderRadius: 3,
                              fontWeight: 600,
                              background: (SEVERITY_STYLES[evt.severity] || SEVERITY_STYLES.P2).bg,
                              color: (SEVERITY_STYLES[evt.severity] || SEVERITY_STYLES.P2).fg,
                            }}
                          >
                            {evt.severity}
                          </span>
                          <span
                            style={{
                              fontSize: 7,
                              padding: '1px 5px',
                              borderRadius: 3,
                              background: statusStyle.bg,
                              color: statusStyle.fg,
                            }}
                          >
                            {evt.status}
                          </span>
                        </div>
                        <div
                          style={{
                            fontSize: 9,
                            color: 'var(--text-primary, rgba(200,220,240,0.75))',
                            lineHeight: 1.4,
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {evt.title}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>

          {/* AI 根因分析 — demo .ai */}
          <div
            style={{
              ...panelStyle,
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
              <span>AI 根因分析</span>
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
            <span style={panelTitle}>平台 SLA</span>
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
                <div style={{ fontSize: 8, color: 'var(--text-secondary, rgba(140,170,210,0.35))' }}>% 可用率</div>
              </div>
            </div>
          </div>

          {/* 业务健康矩阵 — demo .hg */}
          <div style={panelStyle}>
            <div style={panelHeader}>
              <span style={panelTitle}>业务健康矩阵</span>
              <span style={mutedText}>悬停翻转</span>
            </div>
            <HealthMatrix
              cells={healthCells}
              rows={3}
              cols={5}
              cellHeight={24}
              colLabels={['NET', 'HOST', 'APP', 'DB', 'MW']}
              rowLabels={['支付', '电商', '风控']}
            />
          </div>

          {/* 值班团队 — demo .tm-flip */}
          <div style={panelStyle}>
            <div style={panelHeader}>
              <span style={panelTitle}>值班团队</span>
              <span style={mutedText}>悬停翻转</span>
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
                        <div style={{ fontSize: 7, color: 'var(--text-secondary, rgba(140,170,210,0.35))' }}>电话</div>
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
