package biz

import (
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// ScheduleManager 管理定时触发的工作流调度任务。
// 内部使用 robfig/cron 库实现 cron 表达式解析和定时执行。
type ScheduleManager struct {
	cron   *cron.Cron
	jobs   map[string]cron.EntryID // workflowID -> cron entryID
	mu     sync.RWMutex
	log    *zap.SugaredLogger
	onTick func(workflowID string) // 触发回调函数
}

// NewScheduleManager 创建定时调度管理器。
//
// 参数：
//   - log: 日志记录器
//   - onTick: 当定时任务触发时调用的回调函数，参数为工作流 ID
func NewScheduleManager(log *zap.SugaredLogger, onTick func(workflowID string)) *ScheduleManager {
	return &ScheduleManager{
		cron:   cron.New(cron.WithSeconds()),
		jobs:   make(map[string]cron.EntryID),
		log:    log,
		onTick: onTick,
	}
}

// Start 启动调度器，开始执行已注册的定时任务。
func (sm *ScheduleManager) Start() {
	sm.cron.Start()
	sm.log.Info("定时调度管理器已启动")
}

// Stop 优雅停止调度器，等待正在执行的任务完成。
func (sm *ScheduleManager) Stop() {
	ctx := sm.cron.Stop()
	<-ctx.Done()
	sm.log.Info("定时调度管理器已停止")
}

// AddJob 为指定工作流添加定时任务。
// 若该工作流已有定时任务，会先移除旧任务再添加新任务。
//
// 参数：
//   - workflowID: 工作流 ID
//   - cronExpr: cron 表达式（支持 6 位格式：秒 分 时 日 月 星期）
func (sm *ScheduleManager) AddJob(workflowID string, cronExpr string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 若已存在则先移除
	if entryID, exists := sm.jobs[workflowID]; exists {
		sm.cron.Remove(entryID)
		delete(sm.jobs, workflowID)
	}

	// 创建闭包捕获 workflowID
	wfID := workflowID
	entryID, err := sm.cron.AddFunc(cronExpr, func() {
		sm.log.Infof("定时任务触发: workflowID=%s", wfID)
		if sm.onTick != nil {
			sm.onTick(wfID)
		}
	})
	if err != nil {
		return fmt.Errorf("添加定时任务失败: %w", err)
	}

	sm.jobs[workflowID] = entryID
	sm.log.Infof("已添加定时任务: workflowID=%s, cron=%s", workflowID, cronExpr)
	return nil
}

// RemoveJob 移除指定工作流的定时任务。
func (sm *ScheduleManager) RemoveJob(workflowID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if entryID, exists := sm.jobs[workflowID]; exists {
		sm.cron.Remove(entryID)
		delete(sm.jobs, workflowID)
		sm.log.Infof("已移除定时任务: workflowID=%s", workflowID)
	}
}

// ListJobs 返回当前所有已注册定时任务的工作流 ID 列表。
func (sm *ScheduleManager) ListJobs() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	ids := make([]string, 0, len(sm.jobs))
	for id := range sm.jobs {
		ids = append(ids, id)
	}
	return ids
}
