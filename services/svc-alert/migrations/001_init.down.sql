-- svc-alert: rollback initial schema
-- Drop in reverse dependency order.

DROP SEQUENCE IF EXISTS alert_daily_seq;
DROP TABLE IF EXISTS silences;
DROP TABLE IF EXISTS maintenance_windows;
DROP TABLE IF EXISTS baseline_models;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS alert_rules;
