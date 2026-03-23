-- svc-change: 初始化变更管理数据库表结构
-- 包含变更单表、审批记录表和序列号生成表

-- 变更单主表：记录每次变更请求的完整信息
CREATE TABLE IF NOT EXISTS change_tickets (
    id               VARCHAR(50) PRIMARY KEY,       -- 变更单编号，格式 CHG-YYYYMMDD-NNN
    title            VARCHAR(500) NOT NULL,          -- 变更标题
    type             VARCHAR(20) NOT NULL,           -- 变更类型：standard/normal/emergency/major
    risk_level       VARCHAR(20) NOT NULL,           -- 风险级别：low/medium/high/critical
    status           VARCHAR(30) NOT NULL DEFAULT 'draft', -- 状态：draft/pending_approval/approved/in_progress/completed/cancelled/rejected
    requester        VARCHAR(100),                   -- 申请人
    approvers        JSONB DEFAULT '[]',             -- 审批人列表
    executor_id      VARCHAR(100),                   -- 执行人 ID
    affected_assets  JSONB DEFAULT '[]',             -- 影响的资产 ID 列表
    rollback_plan    TEXT,                           -- 回滚方案
    scheduled_start  TIMESTAMPTZ NOT NULL,           -- 计划开始时间
    scheduled_end    TIMESTAMPTZ NOT NULL,           -- 计划结束时间
    actual_start     TIMESTAMPTZ,                    -- 实际开始时间
    actual_end       TIMESTAMPTZ,                    -- 实际结束时间
    description      TEXT,                           -- 变更描述
    ai_risk_summary  TEXT,                           -- AI 风险评估摘要
    related_change_ids JSONB DEFAULT '[]',           -- 关联变更单 ID 列表
    maintenance_id   VARCHAR(100),                   -- 关联的维护模式 ID
    cancel_reason    TEXT,                           -- 取消原因
    created_at       TIMESTAMPTZ DEFAULT now(),
    updated_at       TIMESTAMPTZ DEFAULT now()
);

COMMENT ON TABLE change_tickets IS '变更单主表，记录变更请求的完整生命周期信息';
COMMENT ON COLUMN change_tickets.id IS '变更单编号，格式 CHG-YYYYMMDD-NNN';
COMMENT ON COLUMN change_tickets.type IS '变更类型：standard（标准）/normal（常规）/emergency（紧急）/major（重大）';
COMMENT ON COLUMN change_tickets.risk_level IS '风险级别：low/medium/high/critical，影响审批路由';
COMMENT ON COLUMN change_tickets.status IS '变更状态：draft/pending_approval/approved/in_progress/completed/cancelled/rejected';
COMMENT ON COLUMN change_tickets.affected_assets IS '受影响资产 ID 列表，JSON 数组格式，用于冲突检测';

-- 变更单索引
CREATE INDEX idx_change_tickets_status ON change_tickets(status);
CREATE INDEX idx_change_tickets_type ON change_tickets(type);
CREATE INDEX idx_change_tickets_risk_level ON change_tickets(risk_level);
CREATE INDEX idx_change_tickets_requester ON change_tickets(requester);
CREATE INDEX idx_change_tickets_scheduled ON change_tickets(scheduled_start, scheduled_end);
CREATE INDEX idx_change_tickets_created ON change_tickets(created_at);
-- GIN 索引用于加速 JSONB 数组的交集查询（冲突检测）
CREATE INDEX idx_change_tickets_assets ON change_tickets USING GIN (affected_assets);

-- 审批记录表：保存每一次审批决策的详细信息
CREATE TABLE IF NOT EXISTS approval_records (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),  -- 审批记录 ID
    change_id        VARCHAR(50) NOT NULL REFERENCES change_tickets(id) ON DELETE CASCADE, -- 关联的变更单 ID
    approver_id      VARCHAR(100) NOT NULL,           -- 审批人 ID
    decision         VARCHAR(20) NOT NULL,             -- 决策：approved/rejected
    comment          TEXT,                             -- 审批意见
    decided_at       TIMESTAMPTZ NOT NULL              -- 决策时间
);

COMMENT ON TABLE approval_records IS '审批记录表，记录变更单审批流程中的每次决策';
COMMENT ON COLUMN approval_records.decision IS '审批决策：approved（通过）/rejected（拒绝）';

CREATE INDEX idx_approval_records_change ON approval_records(change_id);
CREATE INDEX idx_approval_records_approver ON approval_records(approver_id);

-- 变更单序列号生成表：基于日期的自增序列，用于生成 CHG-YYYYMMDD-NNN 格式的 ID
CREATE TABLE IF NOT EXISTS change_sequences (
    date_key         DATE PRIMARY KEY,
    last_seq         INT DEFAULT 0
);

COMMENT ON TABLE change_sequences IS '变更单序列号生成表，按日期自增生成唯一 ID';
