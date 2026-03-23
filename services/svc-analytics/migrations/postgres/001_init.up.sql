-- svc-analytics: PostgreSQL initial schema
-- SLA configs, dashboards, reports, knowledge base

-- Enable pgvector extension for knowledge base embeddings (FR-17-003)
CREATE EXTENSION IF NOT EXISTS vector;

-- SLA configurations (FR-10-001 ~ FR-10-007)
CREATE TABLE IF NOT EXISTS sla_configs (
    config_id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(200) NOT NULL,
    dimension           VARCHAR(30) NOT NULL,        -- infra_layer/business_unit/asset_group/asset_grade/region/asset/global
    dimension_value     VARCHAR(200) NOT NULL,        -- value for the dimension (e.g., "payment", "S")
    target_percentage   DOUBLE PRECISION NOT NULL,    -- e.g., 99.95
    window              VARCHAR(20) DEFAULT 'monthly', -- monthly/quarterly/yearly/weekly
    exclude_planned     BOOLEAN DEFAULT true,         -- FR-10-002: exclude planned maintenance
    created_at          TIMESTAMPTZ DEFAULT now(),
    updated_at          TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_sla_configs_dimension ON sla_configs(dimension);
CREATE INDEX idx_sla_configs_dim_value ON sla_configs(dimension_value);

-- SLA calculation snapshots (cached results for FR-10-004 error budget tracking)
CREATE TABLE IF NOT EXISTS sla_snapshots (
    snapshot_id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    config_id             UUID NOT NULL REFERENCES sla_configs(config_id) ON DELETE CASCADE,
    period_start          TIMESTAMPTZ NOT NULL,
    period_end            TIMESTAMPTZ NOT NULL,
    actual_percentage     DOUBLE PRECISION NOT NULL,
    compliance            BOOLEAN NOT NULL,
    downtime_seconds      DOUBLE PRECISION DEFAULT 0,
    incident_count        INT DEFAULT 0,
    error_budget_remaining_pct DOUBLE PRECISION DEFAULT 100,
    calculated_at         TIMESTAMPTZ DEFAULT now(),
    UNIQUE (config_id, period_start, period_end)
);

CREATE INDEX idx_sla_snapshots_config ON sla_snapshots(config_id);
CREATE INDEX idx_sla_snapshots_period ON sla_snapshots(period_start, period_end);

-- Dashboards (OpenAPI spec)
CREATE TABLE IF NOT EXISTS dashboards (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    panels          JSONB DEFAULT '[]',           -- array of Panel objects
    owner_id        UUID,
    is_public       BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_dashboards_owner ON dashboards(owner_id);
CREATE INDEX idx_dashboards_public ON dashboards(is_public);

-- Scheduled reports (OpenAPI spec)
CREATE TABLE IF NOT EXISTS reports (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    schedule        VARCHAR(50) NOT NULL,          -- cron expression
    query           TEXT NOT NULL,                  -- SQL-like analytics query
    format          VARCHAR(10) DEFAULT 'json',     -- pdf/csv/json
    recipients      JSONB DEFAULT '[]',             -- notification targets
    status          VARCHAR(20) DEFAULT 'pending',  -- pending/running/completed/failed
    last_run_at     TIMESTAMPTZ,
    created_by      UUID,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_reports_status ON reports(status);
CREATE INDEX idx_reports_created_by ON reports(created_by);

-- Knowledge base articles (FR-17-001 ~ FR-17-008)
-- Entity fields per ON-002: kb_id, type, title, content, tags, quality_score, related_incident, embedding_vector
CREATE TABLE IF NOT EXISTS knowledge_articles (
    article_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type              VARCHAR(30) NOT NULL,           -- case_study/runbook/faq/architecture/vendor_doc (FR-17-002)
    title             VARCHAR(500) NOT NULL,
    content           TEXT NOT NULL,
    tags              JSONB DEFAULT '[]',
    quality_score     DOUBLE PRECISION DEFAULT 0,     -- FR-17-006: user feedback score
    related_incident  VARCHAR(100),                    -- FR-17-001: linked incident ID
    status            VARCHAR(20) DEFAULT 'draft',     -- draft/published/archived
    embedding         vector(1536),                    -- pgvector for semantic search (FR-17-003)
    created_by        UUID,
    created_at        TIMESTAMPTZ DEFAULT now(),
    updated_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_knowledge_type ON knowledge_articles(type);
CREATE INDEX idx_knowledge_status ON knowledge_articles(status);
CREATE INDEX idx_knowledge_tags ON knowledge_articles USING GIN (tags);
CREATE INDEX idx_knowledge_incident ON knowledge_articles(related_incident);
CREATE INDEX idx_knowledge_embedding ON knowledge_articles USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
