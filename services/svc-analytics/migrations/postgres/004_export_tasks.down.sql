-- svc-analytics: 回滚异步数据导出任务表
-- 逆向操作 004_export_tasks.up.sql

DROP TABLE IF EXISTS export_tasks CASCADE;
