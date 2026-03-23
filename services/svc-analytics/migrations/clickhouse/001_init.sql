-- svc-analytics: ClickHouse initial schema
-- Tables match ON-003 section 3.7 ClickHouse table design

-- SLA 计算基础表 (从事件域 ETL)
CREATE TABLE IF NOT EXISTS sla_incidents ON CLUSTER default (
    incident_id       String,
    severity          String,
    business_unit     String,
    infra_layer       String,       -- network/host/application/database/middleware
    asset_group       String,
    asset_grade       String,
    region            String,
    detected_at       DateTime64(3),
    resolved_at       DateTime64(3),
    downtime_seconds  UInt32,
    is_planned        UInt8,        -- 计划内维护排除 (FR-10-002)
    root_cause_category String
) ENGINE = ReplicatedMergeTree()
ORDER BY (business_unit, detected_at);

-- 业务指标表 (FR-12-001: 交易量/TPS/DAU/GMV/API调用量/成功率/P99延迟/支付成功率)
CREATE TABLE IF NOT EXISTS business_metrics ON CLUSTER default (
    timestamp         DateTime64(3),
    metric_name       String,
    metric_value      Float64,
    business_unit     String,
    service           String,
    tags              Map(String, String)
) ENGINE = ReplicatedMergeTree()
ORDER BY (business_unit, metric_name, timestamp);

-- 资源指标表 (FR-12-002: CPU/内存/带宽/磁盘IO/连接数/QPS)
CREATE TABLE IF NOT EXISTS resource_metrics ON CLUSTER default (
    timestamp         DateTime64(3),
    metric_name       String,
    metric_value      Float64,
    asset_id          String,
    asset_type        String,
    business_unit     String,
    region            String
) ENGINE = ReplicatedMergeTree()
ORDER BY (asset_id, metric_name, timestamp);

-- 5-minute rollups for business metrics
CREATE TABLE IF NOT EXISTS business_metrics_5m ON CLUSTER default (
    timestamp         DateTime64(3),
    metric_name       String,
    business_unit     String,
    service           String,
    avg_value         AggregateFunction(avg, Float64),
    max_value         AggregateFunction(max, Float64),
    min_value         AggregateFunction(min, Float64),
    count_value       AggregateFunction(count, UInt64)
) ENGINE = ReplicatedAggregatingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (business_unit, metric_name, timestamp);

CREATE MATERIALIZED VIEW IF NOT EXISTS business_metrics_5m_mv ON CLUSTER default
TO business_metrics_5m
AS SELECT
    toStartOfFiveMinutes(timestamp) AS timestamp,
    metric_name,
    business_unit,
    service,
    avgState(metric_value)   AS avg_value,
    maxState(metric_value)   AS max_value,
    minState(metric_value)   AS min_value,
    countState()             AS count_value
FROM business_metrics
GROUP BY timestamp, metric_name, business_unit, service;

-- 5-minute rollups for resource metrics
CREATE TABLE IF NOT EXISTS resource_metrics_5m ON CLUSTER default (
    timestamp         DateTime64(3),
    metric_name       String,
    asset_id          String,
    asset_type        String,
    business_unit     String,
    region            String,
    avg_value         AggregateFunction(avg, Float64),
    max_value         AggregateFunction(max, Float64),
    min_value         AggregateFunction(min, Float64),
    count_value       AggregateFunction(count, UInt64)
) ENGINE = ReplicatedAggregatingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (asset_id, metric_name, timestamp);

CREATE MATERIALIZED VIEW IF NOT EXISTS resource_metrics_5m_mv ON CLUSTER default
TO resource_metrics_5m
AS SELECT
    toStartOfFiveMinutes(timestamp) AS timestamp,
    metric_name,
    asset_id,
    asset_type,
    business_unit,
    region,
    avgState(metric_value)   AS avg_value,
    maxState(metric_value)   AS max_value,
    minState(metric_value)   AS min_value,
    countState()             AS count_value
FROM resource_metrics
GROUP BY timestamp, metric_name, asset_id, asset_type, business_unit, region;

-- 1-hour rollups for resource metrics (long retention)
CREATE TABLE IF NOT EXISTS resource_metrics_1h ON CLUSTER default (
    timestamp         DateTime64(3),
    metric_name       String,
    asset_id          String,
    asset_type        String,
    business_unit     String,
    region            String,
    avg_value         AggregateFunction(avg, Float64),
    max_value         AggregateFunction(max, Float64),
    min_value         AggregateFunction(min, Float64),
    count_value       AggregateFunction(count, UInt64)
) ENGINE = ReplicatedAggregatingMergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (asset_id, metric_name, timestamp);

CREATE MATERIALIZED VIEW IF NOT EXISTS resource_metrics_1h_mv ON CLUSTER default
TO resource_metrics_1h
AS SELECT
    toStartOfHour(timestamp) AS timestamp,
    metric_name,
    asset_id,
    asset_type,
    business_unit,
    region,
    avgState(metric_value)   AS avg_value,
    maxState(metric_value)   AS max_value,
    minState(metric_value)   AS min_value,
    countState()             AS count_value
FROM resource_metrics
GROUP BY timestamp, metric_name, asset_id, asset_type, business_unit, region;
