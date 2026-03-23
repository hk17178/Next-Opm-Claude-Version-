-- api_tokens 表：存储 API 访问令牌
-- 令牌明文不存储，仅保存 SHA-256 哈希值用于验证
-- token_prefix 用于在 UI 上展示令牌的部分前缀，方便用户识别
CREATE TABLE IF NOT EXISTS api_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(200) NOT NULL,
    token_hash   VARCHAR(64) NOT NULL UNIQUE,  -- SHA-256 hex 编码，固定 64 字符
    token_prefix VARCHAR(12) NOT NULL,         -- 展示用前缀，如 "opn_abcd..."
    permissions  TEXT[] NOT NULL DEFAULT '{}',  -- 权限数组，如 {"read","write"}
    expires_at   TIMESTAMPTZ,                  -- 过期时间，NULL 表示永不过期
    last_used_at TIMESTAMPTZ,                  -- 最后使用时间
    last_used_ip VARCHAR(45),                  -- 最后使用来源 IP（支持 IPv6）
    call_count   BIGINT NOT NULL DEFAULT 0,    -- 累计调用次数
    created_by   VARCHAR(100) NOT NULL,        -- 创建人标识
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked      BOOLEAN NOT NULL DEFAULT FALSE -- 是否已吊销
);

-- 按哈希值查询索引（仅索引未吊销的令牌，提升认证查询性能）
CREATE INDEX idx_api_tokens_hash ON api_tokens(token_hash) WHERE NOT revoked;

-- 按创建人查询索引（用于列出用户的所有令牌）
CREATE INDEX idx_api_tokens_created_by ON api_tokens(created_by);
