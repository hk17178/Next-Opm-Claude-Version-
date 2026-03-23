-- svc-alert schema initialization (aligned with ON-003 Section 3.2)

-- 告警规则
CREATE TABLE alert_rules (
    rule_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    layer           SMALLINT NOT NULL CHECK (layer BETWEEN 0 AND 5),
    rule_type       VARCHAR(50) NOT NULL,  -- threshold/keyword/frequency/baseline/trend/ai/business
    condition       JSONB NOT NULL,         -- 规则条件 (阈值/关键字/表达式)
    targets         JSONB,                  -- 关联资产/资产组
    severity        VARCHAR(10) NOT NULL,   -- critical/high/medium/low/info
    ironclad        BOOLEAN DEFAULT false,  -- 铁律告警标记
    enabled         BOOLEAN DEFAULT true,
    schedule        JSONB,                  -- 定时生效规则
    cooldown_minutes INT DEFAULT 5,
    notification_channels JSONB DEFAULT '[]',
    created_by      UUID,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_alert_rules_enabled ON alert_rules (enabled, layer);
CREATE INDEX idx_alert_rules_type    ON alert_rules (rule_type);

-- 告警实例
CREATE TABLE alerts (
    alert_id        VARCHAR(50) PRIMARY KEY,  -- ALT-YYYYMMDD-NNN
    rule_id         UUID REFERENCES alert_rules(rule_id),
    severity        VARCHAR(10) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'firing',  -- firing/acknowledged/resolved
    title           TEXT NOT NULL DEFAULT '',
    description     TEXT,
    source_host     VARCHAR(200),
    source_service  VARCHAR(200),
    source_asset_id UUID,
    message         TEXT NOT NULL,
    metric_value    DECIMAL,
    threshold_value DECIMAL,
    fingerprint     VARCHAR(64) NOT NULL,     -- 去重指纹 sha256:...
    layer           SMALLINT NOT NULL,
    ironclad        BOOLEAN DEFAULT false,
    suppressed      BOOLEAN DEFAULT false,
    suppressed_by   VARCHAR(50),              -- maintenance/convergence/ai
    triggered_at    TIMESTAMPTZ NOT NULL,
    acknowledged_at TIMESTAMPTZ,
    resolved_at     TIMESTAMPTZ,
    incident_id     VARCHAR(50),
    tags            JSONB
);

CREATE INDEX idx_alerts_status      ON alerts(status) WHERE status = 'firing';
CREATE INDEX idx_alerts_fingerprint ON alerts(fingerprint);
CREATE INDEX idx_alerts_triggered   ON alerts(triggered_at);

-- 基线模型
CREATE TABLE baseline_models (
    model_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id         UUID REFERENCES alert_rules(rule_id),
    metric_name     VARCHAR(200) NOT NULL,
    target          VARCHAR(200) NOT NULL,     -- 主机名/服务名
    sampling_interval VARCHAR(10) NOT NULL,    -- 1min/5min/15min/30min/1h
    learning_days   INT DEFAULT 14,
    deviation_pct   DECIMAL DEFAULT 30.0,
    peak_exemptions JSONB,                     -- 高峰豁免规则
    baseline_data   JSONB,                     -- 学习得到的基线数据
    status          VARCHAR(20) DEFAULT 'learning',  -- learning/active/paused
    created_at      TIMESTAMPTZ DEFAULT now(),
    last_trained_at TIMESTAMPTZ
);

-- 维护模式
CREATE TABLE maintenance_windows (
    mw_id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    status          VARCHAR(20) DEFAULT 'scheduled',  -- scheduled/active/expired
    start_time      TIMESTAMPTZ NOT NULL,
    end_time        TIMESTAMPTZ NOT NULL,
    max_duration_h  INT DEFAULT 24,
    assets          JSONB,
    asset_groups    JSONB,
    cascade         BOOLEAN DEFAULT true,
    change_order_id VARCHAR(50),
    created_by      UUID,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- 静默规则 (OpenAPI silences endpoint)
CREATE TABLE silences (
    silence_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    matchers        JSONB NOT NULL,
    starts_at       TIMESTAMPTZ NOT NULL,
    ends_at         TIMESTAMPTZ NOT NULL,
    comment         TEXT,
    created_by      VARCHAR(200),
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- 告警序号生成 (for ALT-YYYYMMDD-NNN format)
CREATE SEQUENCE alert_daily_seq START 1;
