-- 回滚 api_tokens 表及其索引
DROP INDEX IF EXISTS idx_api_tokens_created_by;
DROP INDEX IF EXISTS idx_api_tokens_hash;
DROP TABLE IF EXISTS api_tokens;
