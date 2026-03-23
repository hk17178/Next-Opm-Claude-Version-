-- svc-analytics: 异步数据导出任务表（FR-29-001）
-- 存储导出任务状态、范围和下载链接信息

CREATE TABLE IF NOT EXISTS export_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope           JSONB NOT NULL DEFAULT '[]',                    -- 导出范围：alerts/incidents/assets/knowledge/audit_logs
    format          VARCHAR(10) NOT NULL DEFAULT 'json',            -- 输出格式：json / csv
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',         -- 任务状态：pending/processing/done/failed
    file_url        TEXT DEFAULT '',                                -- 下载链接（完成后生成，24 小时有效）
    error           TEXT DEFAULT '',                                -- 错误信息（失败时记录）
    created_by      UUID NOT NULL,                                  -- 创建人 ID
    created_at      TIMESTAMPTZ DEFAULT now(),
    expires_at      TIMESTAMPTZ                                     -- 下载链接过期时间
);

-- 按状态查询索引（轮询任务状态的高频场景）
CREATE INDEX idx_export_tasks_status ON export_tasks(status);
-- 按创建人查询索引
CREATE INDEX idx_export_tasks_created_by ON export_tasks(created_by);
-- 按创建时间排序索引
CREATE INDEX idx_export_tasks_created_at ON export_tasks(created_at DESC);
