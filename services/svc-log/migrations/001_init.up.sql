-- svc-log: initial schema aligned with ON-003 system design document.
-- Log entries are stored in Elasticsearch; this schema holds configuration metadata.

-- 日志解析规则 (must be created before log_sources due to FK)
CREATE TABLE IF NOT EXISTS parse_rules (
    rule_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    format_type     VARCHAR(50) NOT NULL,     -- json/syslog/cef/clf/grok/regex/lua
    pattern         TEXT,                      -- Grok/正则表达式
    multiline_rule  JSONB,                     -- 多行合并配置
    field_mapping   JSONB,                     -- 字段映射到标准 Schema
    sample_log      TEXT,                      -- 示例日志 (用于测试)
    enabled         BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- 日志源配置
CREATE TABLE IF NOT EXISTS log_sources (
    source_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(100) NOT NULL,
    source_type       VARCHAR(50) NOT NULL,   -- host/middleware/network/security/transaction/database/k8s
    collection_method VARCHAR(50) NOT NULL,    -- syslog_tcp/syslog_udp/http_push/kafka_consume/webhook
    config            JSONB NOT NULL,          -- 接收端点配置 (端口/Topic/路径/认证)
    parse_rule_id     UUID REFERENCES parse_rules(rule_id),
    enabled           BOOLEAN DEFAULT true,
    created_at        TIMESTAMPTZ DEFAULT now(),
    updated_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_log_sources_type ON log_sources (source_type);

-- 脱敏规则
CREATE TABLE IF NOT EXISTS masking_rules (
    rule_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    pattern         VARCHAR(500) NOT NULL,     -- 正则匹配
    replacement     VARCHAR(200) NOT NULL,     -- 替换模式
    priority        INT DEFAULT 0,
    enabled         BOOLEAN DEFAULT true
);

-- 留存策略
CREATE TABLE IF NOT EXISTS retention_policies (
    policy_id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    source_type     VARCHAR(50),
    log_level       VARCHAR(20),               -- 按级别设置不同策略
    hot_days        INT DEFAULT 7,
    warm_days       INT DEFAULT 30,
    cold_days       INT DEFAULT 365,
    enabled         BOOLEAN DEFAULT true
);

-- 日志流 (对应 OpenAPI /streams 资源)
CREATE TABLE IF NOT EXISTS streams (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    filter          TEXT DEFAULT '',
    retention_days  INT DEFAULT 30,
    created_at      TIMESTAMPTZ DEFAULT now()
);
