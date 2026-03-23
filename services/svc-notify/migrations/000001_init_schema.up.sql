-- svc-notify initial schema
-- Bots (channels), notification logs, broadcast rules

CREATE TABLE IF NOT EXISTS bots (
    bot_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    channel_type    VARCHAR(50) NOT NULL,
    config          JSONB NOT NULL,
    scope           JSONB,
    template_type   VARCHAR(50),
    health_status   VARCHAR(20) DEFAULT 'unknown',
    last_health_check TIMESTAMPTZ,
    failure_count   INT DEFAULT 0,
    enabled         BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notification_logs (
    log_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_id          UUID REFERENCES bots(bot_id),
    channel_type    VARCHAR(50) NOT NULL,
    recipient       VARCHAR(500),
    message_type    VARCHAR(50),
    event_node      VARCHAR(50),
    incident_id     UUID,
    alert_id        UUID,
    content_hash    VARCHAR(64),
    content_preview TEXT,
    status          VARCHAR(20) NOT NULL,
    error_message   TEXT,
    retry_count     INT DEFAULT 0,
    sent_at         TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notif_status ON notification_logs(status, sent_at);
CREATE INDEX IF NOT EXISTS idx_notif_incident ON notification_logs(incident_id);
CREATE INDEX IF NOT EXISTS idx_notif_dedup ON notification_logs(content_hash, sent_at);

CREATE TABLE IF NOT EXISTS broadcast_rules (
    rule_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_id          UUID REFERENCES bots(bot_id),
    event_node      VARCHAR(50) NOT NULL,
    severity_filter JSONB,
    frequency       JSONB,
    template        TEXT,
    enabled         BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_broadcast_node ON broadcast_rules(event_node, enabled);

CREATE TABLE IF NOT EXISTS templates (
    template_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    channel         VARCHAR(50) NOT NULL,
    subject_template TEXT,
    body_template   TEXT NOT NULL,
    variables       JSONB DEFAULT '[]',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS contacts (
    contact_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    email           VARCHAR(300),
    phone           VARCHAR(50),
    im_accounts     JSONB DEFAULT '{}',
    groups          JSONB DEFAULT '[]',
    created_at      TIMESTAMPTZ DEFAULT now()
);
