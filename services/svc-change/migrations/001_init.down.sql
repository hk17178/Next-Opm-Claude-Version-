-- svc-change: 回滚初始化表结构
-- 按依赖关系的反序删除

DROP TABLE IF EXISTS approval_records;
DROP TABLE IF EXISTS change_tickets;
DROP TABLE IF EXISTS change_sequences;
