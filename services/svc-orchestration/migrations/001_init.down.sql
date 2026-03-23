-- svc-orchestration: 回滚初始化数据库结构
-- 按依赖关系倒序删除

DROP TABLE IF EXISTS execution_steps;
DROP TABLE IF EXISTS workflow_executions;
DROP TABLE IF EXISTS workflow_templates;
DROP TABLE IF EXISTS workflows;
