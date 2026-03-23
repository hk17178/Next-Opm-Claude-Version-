-- svc-incident: 迁移 002 回滚 —— 移除 MTTR 列和变更工单关联表

-- 先删除依赖索引，再删除表和列
DROP INDEX IF EXISTS idx_incident_changes_order;
DROP INDEX IF EXISTS idx_incident_changes_incident;
DROP TABLE IF EXISTS incident_changes;

DROP INDEX IF EXISTS idx_incidents_mttr;
ALTER TABLE incidents DROP COLUMN IF EXISTS mttr_seconds;
