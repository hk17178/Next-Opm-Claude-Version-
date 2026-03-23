-- svc-orchestration 初始化数据库结构
-- 包含工作流模板、执行记录、执行步骤和预置模板四张表

-- 工作流模板表
CREATE TABLE workflows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    steps           JSONB NOT NULL,                              -- 步骤定义列表
    variables       JSONB DEFAULT '{}',                          -- 变量定义
    trigger_type    VARCHAR(20) NOT NULL DEFAULT 'manual',       -- manual/alert/schedule
    cron_expr       VARCHAR(100),                                -- 定时触发的 cron 表达式
    created_by      VARCHAR(200),
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now(),
    is_active       BOOLEAN DEFAULT true
);

CREATE INDEX idx_workflows_active ON workflows (is_active);
CREATE INDEX idx_workflows_trigger ON workflows (trigger_type);

-- 工作流执行记录表
CREATE TABLE workflow_executions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    trigger_type    VARCHAR(20) NOT NULL,                        -- manual/alert/schedule
    trigger_source  VARCHAR(200),                                -- 告警 ID / 事件 ID 等触发源
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',      -- pending/running/paused/completed/failed/cancelled
    variables       JSONB DEFAULT '{}',                          -- 执行时的变量值
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    created_by      VARCHAR(200)
);

CREATE INDEX idx_executions_workflow ON workflow_executions (workflow_id);
CREATE INDEX idx_executions_status ON workflow_executions (status);
CREATE INDEX idx_executions_started ON workflow_executions (started_at);

-- 执行步骤记录表
CREATE TABLE execution_steps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id    UUID NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    step_index      INT NOT NULL,                                -- 步骤顺序索引
    step_name       VARCHAR(200) NOT NULL,
    step_type       VARCHAR(20) NOT NULL,                        -- script/approval/condition/parallel/wait/notify
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',      -- pending/running/success/failed/skipped/approved/rejected
    input           JSONB,                                       -- 步骤输入参数
    output          JSONB,                                       -- 步骤执行输出
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    error_msg       TEXT
);

CREATE INDEX idx_steps_execution ON execution_steps (execution_id, step_index);

-- 预置模板表（存储模板元数据，实际模板定义在代码中）
CREATE TABLE workflow_templates (
    id              VARCHAR(100) PRIMARY KEY,
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    category        VARCHAR(50),                                 -- 模板分类
    steps           JSONB NOT NULL,                              -- 步骤定义
    variables       JSONB DEFAULT '{}',                          -- 默认变量
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- 插入 4 个预置模板
INSERT INTO workflow_templates (id, name, description, category, steps, variables) VALUES
('tpl-service-restart', '服务重启', '重启指定服务并验证恢复状态，适用于服务异常时的快速恢复', 'recovery',
 '[{"name":"确认重启","type":"approval","config":{"message":"确认要重启服务吗？"}},{"name":"停止服务","type":"script","config":{"script":"systemctl stop {{.ServiceName}}"}},{"name":"等待停止","type":"wait","config":{"duration":5}},{"name":"启动服务","type":"script","config":{"script":"systemctl start {{.ServiceName}}"}},{"name":"健康检查","type":"script","config":{"script":"systemctl is-active {{.ServiceName}}"}},{"name":"通知结果","type":"notify","config":{"channel":"ops","message":"服务已重启完成"}}]',
 '{"ServiceName":""}'),

('tpl-disk-cleanup', '磁盘清理', '清理临时文件和日志以释放磁盘空间，适用于磁盘告警场景', 'maintenance',
 '[{"name":"检查磁盘","type":"script","config":{"script":"df -h {{.MountPoint}}"}},{"name":"清理临时文件","type":"script","config":{"script":"find {{.MountPoint}}/tmp -type f -mtime +7 -delete 2>/dev/null; echo done"}},{"name":"清理旧日志","type":"script","config":{"script":"find {{.MountPoint}}/var/log -name *.gz -mtime +30 -delete 2>/dev/null; echo done"}},{"name":"确认结果","type":"script","config":{"script":"df -h {{.MountPoint}}"}},{"name":"通知","type":"notify","config":{"channel":"ops","message":"磁盘清理完成"}}]',
 '{"MountPoint":"/"}'),

('tpl-log-rotation', '日志轮转', '手动触发日志轮转并压缩归档，适用于日志文件过大的场景', 'maintenance',
 '[{"name":"执行轮转","type":"script","config":{"script":"logrotate -f /etc/logrotate.d/{{.LogConfig}}"}},{"name":"压缩归档","type":"script","config":{"script":"find /var/log -name *.1 -exec gzip {} \\; 2>/dev/null; echo done"}},{"name":"通知","type":"notify","config":{"channel":"ops","message":"日志轮转完成"}}]',
 '{"LogConfig":"syslog"}'),

('tpl-config-rollback', '配置回滚', '回滚服务配置到上一版本并重启服务，适用于配置变更导致异常的场景', 'recovery',
 '[{"name":"审批回滚","type":"approval","config":{"message":"确认要回滚配置吗？"}},{"name":"备份当前配置","type":"script","config":{"script":"cp {{.ConfigPath}} {{.ConfigPath}}.bak"}},{"name":"恢复旧配置","type":"script","config":{"script":"cp {{.BackupPath}} {{.ConfigPath}}"}},{"name":"重启服务","type":"script","config":{"script":"systemctl restart {{.ServiceName}}"}},{"name":"验证","type":"script","config":{"script":"systemctl is-active {{.ServiceName}}"}},{"name":"通知","type":"notify","config":{"channel":"ops","message":"配置回滚完成"}}]',
 '{"ServiceName":"","ConfigPath":"","BackupPath":""}');
