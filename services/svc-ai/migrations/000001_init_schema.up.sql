-- svc-ai initial schema
-- Models, scene bindings, prompts, call logs, and budget tracking

CREATE TABLE IF NOT EXISTS ai_models (
    model_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(200) NOT NULL,
    provider          VARCHAR(50) NOT NULL,
    deployment_type   VARCHAR(20) NOT NULL DEFAULT 'cloud',
    api_endpoint      VARCHAR(500) NOT NULL,
    api_key_encrypted BYTEA NOT NULL,
    local_endpoint    VARCHAR(500),
    local_model_name  VARCHAR(200),
    parameters        JSONB DEFAULT '{}',
    rate_limit_qps    INT DEFAULT 10,
    enabled           BOOLEAN DEFAULT true,
    health_status     VARCHAR(20) DEFAULT 'unknown',
    last_health_check TIMESTAMPTZ,
    created_at        TIMESTAMPTZ DEFAULT now(),
    updated_at        TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS scene_bindings (
    binding_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scene             VARCHAR(50) NOT NULL UNIQUE,
    primary_model_id  UUID REFERENCES ai_models(model_id),
    fallback_model_id UUID REFERENCES ai_models(model_id),
    routing_strategy  VARCHAR(20) NOT NULL DEFAULT 'cloud_first',
    prompt_version    VARCHAR(50),
    created_at        TIMESTAMPTZ DEFAULT now(),
    updated_at        TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS prompts (
    prompt_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scene          VARCHAR(50) NOT NULL,
    version        VARCHAR(50) NOT NULL,
    system_prompt  TEXT NOT NULL,
    user_prompt    TEXT NOT NULL,
    variables      JSONB DEFAULT '[]',
    is_active      BOOLEAN DEFAULT false,
    feedback_score DECIMAL,
    created_at     TIMESTAMPTZ DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_prompts_scene_version ON prompts(scene, version);

CREATE TABLE IF NOT EXISTS ai_call_logs (
    call_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id    UUID,
    model_id       UUID,
    scene          VARCHAR(50) NOT NULL,
    prompt_version VARCHAR(50),
    input_tokens   INT,
    output_tokens  INT,
    latency_ms     INT,
    status         VARCHAR(20),
    feedback       VARCHAR(20),
    input_hash     VARCHAR(64),
    output_summary TEXT,
    error_message  TEXT,
    created_at     TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ai_logs_scene ON ai_call_logs(scene, created_at);
CREATE INDEX IF NOT EXISTS idx_ai_logs_analysis ON ai_call_logs(analysis_id);

CREATE TABLE IF NOT EXISTS ai_budget (
    month        VARCHAR(7) NOT NULL,
    model_id     UUID NOT NULL REFERENCES ai_models(model_id),
    tokens_used  BIGINT DEFAULT 0,
    budget_limit BIGINT NOT NULL,
    alert_sent   BOOLEAN DEFAULT false,
    exhausted    BOOLEAN DEFAULT false,
    PRIMARY KEY (month, model_id)
);

CREATE TABLE IF NOT EXISTS analysis_tasks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type          VARCHAR(50) NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',
    incident_id   UUID,
    alert_ids     JSONB DEFAULT '[]',
    time_range    JSONB,
    context       JSONB DEFAULT '{}',
    result        JSONB,
    model_version VARCHAR(100),
    trigger_event_id VARCHAR(200),
    error_message TEXT,
    created_at    TIMESTAMPTZ DEFAULT now(),
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_analysis_status ON analysis_tasks(status, created_at);
CREATE INDEX IF NOT EXISTS idx_analysis_incident ON analysis_tasks(incident_id);

CREATE TABLE IF NOT EXISTS knowledge_entries (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title      VARCHAR(500) NOT NULL,
    content    TEXT NOT NULL,
    category   VARCHAR(100),
    tags       JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_knowledge_category ON knowledge_entries(category);
