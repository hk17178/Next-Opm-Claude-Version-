-- svc-log: rollback initial schema
-- Drop in reverse dependency order.

DROP TABLE IF EXISTS streams;
DROP TABLE IF EXISTS retention_policies;
DROP TABLE IF EXISTS masking_rules;
DROP TABLE IF EXISTS log_sources;
DROP TABLE IF EXISTS parse_rules;
