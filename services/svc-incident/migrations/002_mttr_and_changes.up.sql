-- svc-incident: 迁移 002 —— 添加 MTTR 存储列和变更工单关联表
--
-- 变更说明：
--   1. incidents 表新增 mttr_seconds 列，在事件关闭时持久化 MTTR 指标（秒）。
--      之前 MTTR 仅在 API 响应时实时计算，无法在数据库层面做聚合统计和趋势分析。
--   2. 新建 incident_changes 表，记录触发或关联到某个事件的变更工单信息。
--      该表支持 POST /api/v1/incidents/{id}/changes 端点写入变更关联数据。

-- ① 在 incidents 表中新增 mttr_seconds 列，用于持久化平均修复时间（秒）。
--   - 数据含义：resolved_at - created_at（以秒为单位）
--   - 可空：事件未解决或未关闭时为 NULL
ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS mttr_seconds BIGINT;

COMMENT ON COLUMN incidents.mttr_seconds IS
    '平均修复时间（秒），在事件关闭时计算 resolved_at - created_at 并写入';

-- 为 MTTR 列建立索引，支持按修复时效统计分析
CREATE INDEX IF NOT EXISTS idx_incidents_mttr ON incidents(mttr_seconds)
    WHERE mttr_seconds IS NOT NULL;

-- ② 创建 incident_changes 表，记录事件关联的变更工单信息。
--   每条记录对应一次"将某个变更工单关联到事件"的操作，
--   支持多条（一个事件可关联多个变更工单），也支持查询某变更工单关联了哪些事件。
CREATE TABLE IF NOT EXISTS incident_changes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),  -- 关联记录唯一 ID
    incident_id     VARCHAR(50) NOT NULL                         -- 关联的事件 ID（如 INC-20250101-001）
                    REFERENCES incidents(incident_id) ON DELETE CASCADE,
    change_order_id VARCHAR(50) NOT NULL,                        -- 变更工单 ID（如 CHG-20250101-001）
    description     TEXT,                                        -- 变更描述，说明与该事件的关联背景
    operator_id     VARCHAR(100),                                -- 操作人用户 ID
    operator_name   VARCHAR(100),                                -- 操作人姓名（冗余存储，避免用户表联查）
    created_at      TIMESTAMPTZ DEFAULT now()                    -- 关联记录创建时间
);

-- 按事件 ID 查询该事件关联的所有变更工单（最常见查询路径）
CREATE INDEX IF NOT EXISTS idx_incident_changes_incident ON incident_changes(incident_id, created_at DESC);

-- 按变更工单 ID 反查关联了哪些事件
CREATE INDEX IF NOT EXISTS idx_incident_changes_order ON incident_changes(change_order_id);
