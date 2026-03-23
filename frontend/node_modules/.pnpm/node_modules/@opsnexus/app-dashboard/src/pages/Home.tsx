import React, { useEffect, useRef } from 'react';
import {
  MetricFlipCard,
  HealthMatrix,
  NoiseFunnel,
  AITypewriter,
  LiveDot,
  FlipCard,
} from '@opsnexus/ui-kit';
import type { HealthCell } from '@opsnexus/ui-kit';
import * as echarts from 'echarts';

/* ------------------------------------------------------------------ */
/*  Mock data                                                         */
/* ------------------------------------------------------------------ */

/** 生成24小时模拟告警数据 */
function generateHourlyData(): { hours: string[]; raw: number[]; effective: number[] } {
  const hours: string[] = [];
  const raw: number[] = [];
  const effective: number[] = [];
  const now = new Date();
  for (let i = 23; i >= 0; i--) {
    const h = new Date(now.getTime() - i * 3600_000);
    hours.push(`${h.getHours().toString().padStart(2, '0')}:00`);
    const r = Math.floor(Math.random() * 20) + 5;
    raw.push(r);
    effective.push(Math.floor(r * (0.3 + Math.random() * 0.4)));
  }
  return { hours, raw, effective };
}

/** 业务健康矩阵 mock */
function generateHealthCells(): HealthCell[] {
  const rows = ['前端', '后端', '数据库', '缓存', '消息队列'];
  const cols = ['P0', 'P1', 'P2', 'P3', 'P4'];
  const statuses: HealthCell['status'][] = ['ok', 'degraded', 'critical'];
  return rows.flatMap((row, ri) =>
    cols.map((col, ci) => {
      const rand = Math.random();
      const status = rand < 0.7 ? 'ok' : rand < 0.9 ? 'degraded' : 'critical';
      return {
        id: `${ri}-${ci}`,
        label: `${row}/${col}`,
        status,
        details: {
          cpu: Math.floor(Math.random() * 60 + 20),
          mem: Math.floor(Math.random() * 50 + 30),
          disk: Math.floor(Math.random() * 40 + 10),
          conn: Math.floor(Math.random() * 200 + 50),
        },
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

interface TimelineEvent {
  id: string;
  time: string;
  title: string;
  severity: 'P0' | 'P1' | 'P2';
  status: string;
}

const recentEvents: TimelineEvent[] = [
  { id: '1', time: '14:32', title: '核心交易数据库主从延迟超阈值', severity: 'P0', status: '处理中' },
  { id: '2', time: '13:15', title: 'API 网关 5xx 错误率飙升至 2.3%', severity: 'P0', status: '已解决' },
  { id: '3', time: '11:48', title: '缓存集群节点 redis-07 内存使用 95%', severity: 'P1', status: '处理中' },
  { id: '4', time: '10:22', title: '消息队列消费延迟超过 30s', severity: 'P1', status: '已解决' },
  { id: '5', time: '09:05', title: 'CDN 边缘节点证书即将过期', severity: 'P2', status: '待处理' },
];

const aiSummaryText =
  '今日系统整体运行平稳，共产生 175 条原始告警，经降噪处理后有效告警 47 条。' +
  '重点关注：核心交易数据库在 14:32 出现主从延迟异常，已自动触发故障转移流程；' +
  'API 网关 5xx 错误率在 13:15 短暂飙升后恢复正常，根因定位为上游服务滚动更新导致。' +
  '当前 SLA 维持在 99.97%，MTTR 较昨日下降 4 分钟至 18 分钟，降噪效率稳步提升。';

interface OnCallMember {
  name: string;
  role: string;
  roleLabel: string;
  status: string;
  statusColor: string;
  phone: string;
  responseTime: string;
  todayCount: number;
}

const onCallTeam: OnCallMember[] = [
  { name: '张伟', role: 'primary', roleLabel: '主值班', status: 'P0 处理中', statusColor: '#ff6b6b', phone: '138****1234', responseTime: '2min', todayCount: 5 },
  { name: '陈静', role: 'backup', roleLabel: '副值班', status: '空闲', statusColor: '#00e5a0', phone: '139****5678', responseTime: '5min', todayCount: 2 },
  { name: '李明', role: 'supervisor', roleLabel: '主管', status: '在线', statusColor: '#4da6ff', phone: '137****9012', responseTime: '10min', todayCount: 0 },
];

/* ------------------------------------------------------------------ */
/*  Severity colors for timeline                                       */
/* ------------------------------------------------------------------ */

const SEVERITY_LINE_COLORS: Record<string, string> = {
  P0: '#ff6b6b',
  P1: '#ffaa33',
  P2: '#4da6ff',
};

const STATUS_TAG_COLORS: Record<string, { bg: string; fg: string }> = {
  '处理中': { bg: 'rgba(255,170,51,0.15)', fg: '#ffaa33' },
  '已解决': { bg: 'rgba(0,229,160,0.15)', fg: '#00e5a0' },
  '待处理': { bg: 'rgba(77,166,255,0.15)', fg: '#4da6ff' },
};

/* ------------------------------------------------------------------ */
/*  Shared styles                                                      */
/* ------------------------------------------------------------------ */

const panelStyle: React.CSSProperties = {
  background: 'var(--bg-card, rgba(255,255,255,0.04))',
  border: '1px solid var(--border-color, rgba(255,255,255,0.08))',
  borderRadius: 12,
  padding: 16,
};

const sectionTitle: React.CSSProperties = {
  fontSize: 14,
  fontWeight: 600,
  color: 'var(--text-primary, #e2e8f0)',
  marginBottom: 12,
};

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

const Home: React.FC = () => {
  const chartRef = useRef<HTMLDivElement>(null);
  const chartInstanceRef = useRef<echarts.ECharts | null>(null);

  useEffect(() => {
    if (!chartRef.current) return;

    const el = chartRef.current;
    const chart = echarts.init(el);
    chartInstanceRef.current = chart;

    const computed = getComputedStyle(el);
    const borderColor = computed.getPropertyValue('--border-color').trim() || 'rgba(255,255,255,0.08)';
    const textSecondary = computed.getPropertyValue('--text-secondary').trim() || '#8899a6';
    const primaryColor = computed.getPropertyValue('--color-primary').trim() || '#4da6ff';

    const { hours, raw, effective } = generateHourlyData();

    chart.setOption({
      grid: { top: 16, right: 16, bottom: 28, left: 40 },
      xAxis: {
        type: 'category',
        data: hours,
        axisLine: { lineStyle: { color: borderColor } },
        axisLabel: { color: textSecondary, fontSize: 10 },
        axisTick: { show: false },
      },
      yAxis: {
        type: 'value',
        splitLine: { lineStyle: { color: borderColor, type: 'dashed' } },
        axisLabel: { color: textSecondary, fontSize: 10 },
      },
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(20,20,30,0.9)',
        borderColor,
        textStyle: { color: '#e2e8f0', fontSize: 12 },
      },
      series: [
        {
          name: '原始告警',
          type: 'line',
          data: raw,
          smooth: true,
          symbol: 'none',
          lineStyle: { color: 'rgba(140,170,210,0.5)', width: 1.5 },
          itemStyle: { color: 'rgba(140,170,210,0.5)' },
        },
        {
          name: '有效告警',
          type: 'line',
          data: effective,
          smooth: true,
          symbol: 'none',
          lineStyle: { color: primaryColor, width: 2 },
          itemStyle: { color: primaryColor },
          areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: primaryColor.replace(')', ',0.3)').replace('rgb', 'rgba') },
            { offset: 1, color: 'rgba(77,166,255,0)' },
          ]) },
        },
      ],
    });

    const handleResize = () => chart.resize();
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      chart.dispose();
      chartInstanceRef.current = null;
    };
  }, []);

  const healthCells = React.useMemo(() => generateHealthCells(), []);

  return (
    <div
      style={{
        background: 'var(--bg-primary, #0d1117)',
        minHeight: '100vh',
        padding: 24,
        color: 'var(--text-primary, #e2e8f0)',
      }}
    >
      {/* -------- A. 4 张翻牌指标卡 -------- */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16, marginBottom: 24 }}>
        <MetricFlipCard
          label="当前告警"
          value={12}
          trend="up"
          trendValue="+3"
          backItems={[
            { label: 'P0', value: 3 },
            { label: 'P1', value: 5 },
            { label: 'P2', value: 4 },
          ]}
        />
        <MetricFlipCard
          label="今日事件"
          value={5}
          trend="flat"
          backItems={[
            { label: '处理中', value: 2 },
            { label: '已解决', value: 3 },
          ]}
        />
        <MetricFlipCard
          label="SLA"
          value={99.97}
          suffix="%"
          trend="up"
          backItems={[
            { label: 'API', value: '99.9%' },
            { label: 'Web', value: '100%' },
            { label: 'DB', value: '99.8%' },
          ]}
        />
        <MetricFlipCard
          label="MTTR"
          value={18}
          suffix="min"
          trend="down"
          trendValue="-4min"
          backItems={[
            { label: '检测', value: '3min' },
            { label: '定位', value: '8min' },
            { label: '修复', value: '7min' },
          ]}
        />
      </div>

      {/* -------- B. 告警趋势 + 健康矩阵 -------- */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 24 }}>
        <div style={panelStyle}>
          <div style={sectionTitle}>告警趋势 24h</div>
          <div ref={chartRef} style={{ width: '100%', height: 220, color: 'var(--text-secondary)' }} />
        </div>
        <div style={panelStyle}>
          <div style={sectionTitle}>业务健康矩阵</div>
          <HealthMatrix cells={healthCells} rows={5} cols={5} cellHeight={36} />
        </div>
      </div>

      {/* -------- C. 降噪漏斗 -------- */}
      <div style={{ ...panelStyle, marginBottom: 24 }}>
        <div style={sectionTitle}>降噪漏斗</div>
        <NoiseFunnel layers={funnelData} />
      </div>

      {/* -------- D. 最近事件流 + AI 洞察摘要 -------- */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 24 }}>
        {/* 事件时间线 */}
        <div style={panelStyle}>
          <div style={sectionTitle}>最近事件</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
            {recentEvents.map((evt, idx) => {
              const lineColor = SEVERITY_LINE_COLORS[evt.severity] || '#4da6ff';
              const tagColor = STATUS_TAG_COLORS[evt.status] || STATUS_TAG_COLORS['待处理'];
              return (
                <div
                  key={evt.id}
                  style={{
                    display: 'flex',
                    alignItems: 'flex-start',
                    gap: 12,
                    padding: '10px 0',
                    borderBottom: idx < recentEvents.length - 1 ? '1px solid var(--border-color, rgba(255,255,255,0.06))' : 'none',
                  }}
                >
                  {/* 左侧竖线 + 圆点 */}
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', width: 12, flexShrink: 0, paddingTop: 2 }}>
                    <div style={{ width: 8, height: 8, borderRadius: '50%', background: lineColor, flexShrink: 0 }} />
                    {idx < recentEvents.length - 1 && (
                      <div style={{ width: 2, flex: 1, minHeight: 20, background: lineColor, opacity: 0.3, marginTop: 4 }} />
                    )}
                  </div>
                  {/* 内容 */}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                      <span style={{ fontSize: 11, color: 'var(--text-secondary, #8899a6)', fontVariantNumeric: 'tabular-nums' }}>
                        {evt.time}
                      </span>
                      <span
                        style={{
                          fontSize: 10,
                          fontWeight: 600,
                          color: lineColor,
                          background: `${lineColor}20`,
                          padding: '1px 6px',
                          borderRadius: 4,
                        }}
                      >
                        {evt.severity}
                      </span>
                    </div>
                    <div style={{ fontSize: 13, color: 'var(--text-primary, #e2e8f0)', lineHeight: 1.4, marginBottom: 4 }}>
                      {evt.title}
                    </div>
                    <span
                      style={{
                        fontSize: 11,
                        fontWeight: 500,
                        color: tagColor.fg,
                        background: tagColor.bg,
                        padding: '2px 8px',
                        borderRadius: 4,
                      }}
                    >
                      {evt.status}
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* AI 洞察摘要 */}
        <div style={panelStyle}>
          <div style={{ ...sectionTitle, display: 'flex', alignItems: 'center', gap: 8 }}>
            <span>今日运维摘要</span>
            <LiveDot label="AI" size={4} />
          </div>
          <div style={{ fontSize: 13, lineHeight: 1.8, color: 'var(--text-secondary, #8899a6)' }}>
            <AITypewriter text={aiSummaryText} speed={20} />
          </div>
        </div>
      </div>

      {/* -------- E. 值班团队 -------- */}
      <div style={{ ...panelStyle }}>
        <div style={sectionTitle}>值班团队</div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16 }}>
          {onCallTeam.map((member) => (
            <div key={member.name} style={{ height: 100 }}>
              <FlipCard
                front={
                  <div
                    style={{
                      ...panelStyle,
                      height: '100%',
                      display: 'flex',
                      flexDirection: 'column',
                      justifyContent: 'center',
                      alignItems: 'center',
                      gap: 6,
                      boxSizing: 'border-box',
                    }}
                  >
                    <div style={{ fontSize: 16, fontWeight: 600, color: 'var(--text-primary, #e2e8f0)' }}>
                      {member.name}
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span
                        style={{
                          fontSize: 11,
                          padding: '2px 8px',
                          borderRadius: 4,
                          background: 'rgba(77,166,255,0.15)',
                          color: '#4da6ff',
                          fontWeight: 500,
                        }}
                      >
                        {member.roleLabel}
                      </span>
                      <span style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11 }}>
                        <span style={{ width: 6, height: 6, borderRadius: '50%', background: member.statusColor, display: 'inline-block' }} />
                        <span style={{ color: member.statusColor }}>{member.status}</span>
                      </span>
                    </div>
                  </div>
                }
                back={
                  <div
                    style={{
                      ...panelStyle,
                      height: '100%',
                      display: 'flex',
                      flexDirection: 'column',
                      justifyContent: 'center',
                      gap: 6,
                      boxSizing: 'border-box',
                      fontSize: 12,
                    }}
                  >
                    {[
                      { label: '手机', value: member.phone },
                      { label: '响应时间', value: member.responseTime },
                      { label: '今日处理', value: `${member.todayCount} 条` },
                    ].map((item) => (
                      <div key={item.label} style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <span style={{ color: 'var(--text-secondary, #8899a6)' }}>{item.label}</span>
                        <span style={{ color: 'var(--text-primary, #e2e8f0)', fontWeight: 600 }}>{item.value}</span>
                      </div>
                    ))}
                  </div>
                }
              />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

export default Home;
