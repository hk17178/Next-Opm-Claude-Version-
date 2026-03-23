-- svc-analytics: 用户建议管理表（FR-16-001~006）
-- 存储用户改进建议，支持 AI 分类、关键词提取、情感分析和相似建议去重

CREATE TABLE IF NOT EXISTS suggestions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title           VARCHAR(500) NOT NULL,                          -- 建议标题
    description     TEXT NOT NULL,                                  -- 详细描述
    submitted_by    UUID NOT NULL,                                  -- 提交人 ID
    status          VARCHAR(20) DEFAULT 'pending' NOT NULL,         -- 状态：pending/accepted/rejected/in_progress/launched
    category        VARCHAR(50),                                    -- AI 分类：功能/体验/性能/安全/流程
    keywords        JSONB DEFAULT '[]',                             -- AI 提取的关键词
    sentiment       VARCHAR(20),                                    -- AI 情感识别：positive/neutral/negative
    similar_ids     JSONB DEFAULT '[]',                             -- 相似建议 ID 列表（AI 去重）
    admin_note      TEXT DEFAULT '',                                -- 管理员备注
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

-- 按状态查询索引（列表筛选高频场景）
CREATE INDEX idx_suggestions_status ON suggestions(status);
-- 按提交人查询索引
CREATE INDEX idx_suggestions_submitted_by ON suggestions(submitted_by);
-- 按创建时间排序索引
CREATE INDEX idx_suggestions_created_at ON suggestions(created_at DESC);
-- AI 分类查询索引
CREATE INDEX idx_suggestions_category ON suggestions(category);
