-- 005_import_tasks.up.sql
-- 数据导入任务表（FR-29-002）
CREATE TABLE import_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    data_type VARCHAR(50) NOT NULL,
    format VARCHAR(10) NOT NULL,
    file_url TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    total_rows INTEGER NOT NULL DEFAULT 0,
    imported_rows INTEGER NOT NULL DEFAULT 0,
    failed_rows INTEGER NOT NULL DEFAULT 0,
    errors JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);
