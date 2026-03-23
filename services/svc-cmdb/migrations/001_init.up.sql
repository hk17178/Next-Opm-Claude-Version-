-- svc-cmdb: Initial schema
-- CMDB domain tables

-- Assets
CREATE TABLE IF NOT EXISTS assets (
    asset_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hostname        VARCHAR(200),
    ip              VARCHAR(45),
    asset_type      VARCHAR(50) NOT NULL,       -- server/network_device/database/middleware/application/storage/loadbalancer/firewall/...
    asset_subtype   VARCHAR(50),
    business_units  JSONB DEFAULT '[]',         -- multi-value: ["payment", "payment.checkout"]
    organization    VARCHAR(200),               -- LDAP sync
    environment     VARCHAR(20),                -- prod/staging/test/dev
    region          VARCHAR(100),
    datacenter      VARCHAR(100),
    grade           CHAR(1) CHECK (grade IN ('S','A','B','C','D')),
    grade_score     JSONB,                      -- scoring details
    status          VARCHAR(20) DEFAULT 'active',  -- active/idle/maintenance/retired
    tags            JSONB DEFAULT '{}',         -- Key-Value custom tags
    custom_dimensions JSONB DEFAULT '{}',       -- custom dimension values
    discovered_by   VARCHAR(50),                -- scan/snmp/cloud_api/dhcp/log_source/manual
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_assets_type ON assets(asset_type);
CREATE INDEX idx_assets_grade ON assets(grade);
CREATE INDEX idx_assets_status ON assets(status);
CREATE INDEX idx_assets_hostname ON assets(hostname);
CREATE INDEX idx_assets_ip ON assets(ip);
CREATE INDEX idx_assets_environment ON assets(environment);
CREATE INDEX idx_assets_business ON assets USING GIN (business_units);
CREATE INDEX idx_assets_tags ON assets USING GIN (tags);
CREATE INDEX idx_assets_custom_dims ON assets USING GIN (custom_dimensions);

-- Custom dimension definitions
CREATE TABLE IF NOT EXISTS custom_dimensions (
    dim_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL UNIQUE,
    display_name    VARCHAR(200) NOT NULL,
    dim_type        VARCHAR(20) NOT NULL,       -- enum/tree/text/date/numeric/reference
    config          JSONB,                      -- enum values/tree structure/constraints
    required        BOOLEAN DEFAULT false,
    sortable        BOOLEAN DEFAULT true,
    filterable      BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- Asset relations (topology dependencies)
CREATE TABLE IF NOT EXISTS asset_relations (
    relation_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_asset_id UUID NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    target_asset_id UUID NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    relation_type   VARCHAR(50) NOT NULL,       -- depends_on/deployed_on/connected_to/child_of
    metadata        JSONB,
    created_at      TIMESTAMPTZ DEFAULT now(),
    UNIQUE (source_asset_id, target_asset_id, relation_type)
);

CREATE INDEX idx_relations_source ON asset_relations(source_asset_id);
CREATE INDEX idx_relations_target ON asset_relations(target_asset_id);
CREATE INDEX idx_relations_type ON asset_relations(relation_type);

-- Asset groups
CREATE TABLE IF NOT EXISTS asset_groups (
    group_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    group_type      VARCHAR(20) NOT NULL,       -- dynamic/static/hybrid
    conditions      JSONB,                      -- dynamic conditions (JSON query expression)
    static_members  JSONB DEFAULT '[]',         -- static member ID list
    member_count    INT DEFAULT 0,              -- cached member count
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

-- Auto-discovery records
CREATE TABLE IF NOT EXISTS discovery_records (
    record_id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    discovery_method VARCHAR(50) NOT NULL,       -- scan/snmp/cloud_api/dhcp/log_source
    ip              VARCHAR(45),
    hostname        VARCHAR(200),
    detected_type   VARCHAR(50),
    detected_grade  CHAR(1),
    status          VARCHAR(20) DEFAULT 'pending',  -- pending/approved/ignored/blacklisted
    matched_asset_id UUID,                      -- if matched to existing asset
    raw_data        JSONB,
    discovered_at   TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_discovery_status ON discovery_records(status);
CREATE INDEX idx_discovery_ip ON discovery_records(ip);

-- Maintenance windows (CMDB-side tracking for topology cascade)
CREATE TABLE IF NOT EXISTS maintenance_windows (
    mw_id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    status          VARCHAR(20) DEFAULT 'scheduled',  -- scheduled/active/expired/cancelled
    start_time      TIMESTAMPTZ NOT NULL,
    end_time        TIMESTAMPTZ NOT NULL,
    assets          JSONB DEFAULT '[]',
    asset_groups    JSONB DEFAULT '[]',
    cascade         BOOLEAN DEFAULT true,
    change_order_id VARCHAR(50),
    created_by      UUID,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_mw_status ON maintenance_windows(status);
