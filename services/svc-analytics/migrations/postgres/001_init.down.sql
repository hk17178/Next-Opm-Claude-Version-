-- svc-analytics: PostgreSQL rollback
-- Reverse of 001_init.up.sql

DROP TABLE IF EXISTS knowledge_articles CASCADE;
DROP TABLE IF EXISTS reports CASCADE;
DROP TABLE IF EXISTS dashboards CASCADE;
DROP TABLE IF EXISTS sla_snapshots CASCADE;
DROP TABLE IF EXISTS sla_configs CASCADE;

DROP EXTENSION IF EXISTS vector;
