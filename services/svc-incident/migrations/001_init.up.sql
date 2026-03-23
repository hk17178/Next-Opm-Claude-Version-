-- svc-incident: Initial schema
-- Incident domain tables

CREATE TABLE IF NOT EXISTS incidents (
    incident_id     VARCHAR(50) PRIMARY KEY,  -- INC-YYYYMMDD-NNN
    title           VARCHAR(500) NOT NULL,
    severity        VARCHAR(10) NOT NULL,      -- P0/P1/P2/P3/P4
    status          VARCHAR(30) NOT NULL DEFAULT 'created',
    -- status values: created/triaging/analyzing/assigned/resolving/verifying/resolved/postmortem/closed
    root_cause_category VARCHAR(30),           -- human_action/change_caused/system_fault/external/unknown
    assignee_id     UUID,
    assignee_name   VARCHAR(100),
    detected_at     TIMESTAMPTZ NOT NULL,
    acknowledged_at TIMESTAMPTZ,
    resolved_at     TIMESTAMPTZ,
    closed_at       TIMESTAMPTZ,
    source_alerts   JSONB DEFAULT '[]',       -- related alert ID list
    affected_assets JSONB DEFAULT '[]',
    business_unit   VARCHAR(100),
    postmortem      JSONB,                     -- root cause / improvements / lessons
    improvement_items JSONB DEFAULT '[]',      -- improvement item list
    tags            JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_incidents_assignee ON incidents(assignee_id);
CREATE INDEX idx_incidents_detected ON incidents(detected_at);
CREATE INDEX idx_incidents_business ON incidents(business_unit);

-- Incident timeline
CREATE TABLE IF NOT EXISTS incident_timeline (
    entry_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id     VARCHAR(50) NOT NULL REFERENCES incidents(incident_id) ON DELETE CASCADE,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT now(),
    entry_type      VARCHAR(50) NOT NULL,      -- alert/status_change/ai_analysis/human_action/note/notification
    source          VARCHAR(50),               -- system/human/ai
    content         JSONB NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_timeline_incident ON incident_timeline(incident_id, timestamp);

-- Change orders
CREATE TABLE IF NOT EXISTS change_orders (
    change_id       VARCHAR(50) PRIMARY KEY,   -- CHG-YYYYMMDD-NNN
    title           VARCHAR(500) NOT NULL,
    change_type     VARCHAR(20) NOT NULL,       -- standard/normal/emergency/major
    risk_level      VARCHAR(20) NOT NULL,       -- low/medium/high/critical
    status          VARCHAR(20) DEFAULT 'draft', -- draft/pending/approved/implementing/completed/failed/cancelled
    requester_id    UUID,
    approver_id     UUID,
    executor_id     UUID,
    plan            JSONB NOT NULL,
    schedule        JSONB,
    result          JSONB,
    related_incidents JSONB DEFAULT '[]',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_change_orders_status ON change_orders(status);

-- On-call schedules
CREATE TABLE IF NOT EXISTS oncall_schedules (
    schedule_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    scope           JSONB NOT NULL,             -- associated business/asset groups/alert levels
    rotation        JSONB NOT NULL,             -- rotation config
    escalation      JSONB NOT NULL,             -- escalation policy
    overrides       JSONB DEFAULT '[]',         -- temporary overrides
    enabled         BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

-- Sequence table for incident ID generation
CREATE TABLE IF NOT EXISTS incident_sequences (
    date_key        DATE PRIMARY KEY,
    last_seq        INT DEFAULT 0
);
