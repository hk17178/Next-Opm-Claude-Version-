// Package main 是 svc-orchestration 编排引擎服务的入口包。
// svc-orchestration 负责自动化工作流编排，支持脚本执行、人工审批、条件分支、
// 并行执行、定时触发等能力。
// 启动顺序：日志 → 配置 → Postgres → 仓库层 → 业务层 → HTTP 服务器 → 优雅关闭。
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/opsnexus/svc-orchestration/internal/biz"
	"github.com/opsnexus/svc-orchestration/internal/config"
	"github.com/opsnexus/svc-orchestration/internal/data"
	"github.com/opsnexus/svc-orchestration/internal/service"
)

// main 是 svc-orchestration 服务的启动入口函数。
func main() {
	// 初始化生产级日志记录器
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	log := logger.Sugar()

	// 加载配置
	cfg := config.Load()

	// 连接 PostgreSQL 数据库
	pool, err := data.NewPostgres(context.Background(), cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()

	// 初始化数据仓库层
	workflowRepo := data.NewWorkflowRepo(pool)
	executionRepo := data.NewExecutionRepo(pool)

	// 初始化业务层
	workflowUC := biz.NewWorkflowUseCase(workflowRepo, log)
	executionUC := biz.NewExecutionUseCase(executionRepo, workflowRepo, log)

	// 初始化定时调度管理器
	scheduler := biz.NewScheduleManager(log, func(workflowID string) {
		// 定时触发时自动执行工作流
		_, err := executionUC.TriggerExecution(workflowID, "schedule", "system", nil)
		if err != nil {
			log.Errorf("定时触发执行失败: workflowID=%s, %v", workflowID, err)
		}
	})
	scheduler.Start()
	defer scheduler.Stop()

	// 构建 HTTP 路由
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)

	// 注册 HTTP 路由处理器
	handler := service.NewHandler(workflowUC, executionUC, log)
	handler.RegisterRoutes(r)

	// 启动 HTTP 服务器
	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Infof("svc-orchestration HTTP 监听: %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务器错误: %v", err)
		}
	}()

	// 优雅关闭：监听 SIGINT/SIGTERM 信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Infof("收到信号 %v，开始关闭", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Errorf("HTTP 服务器关闭错误: %v", err)
	}

	fmt.Println("svc-orchestration 已停止")
}
