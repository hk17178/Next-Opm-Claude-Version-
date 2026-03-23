// Package service 实现编排服务的 HTTP API 层，提供工作流管理和执行控制的 REST 接口。
package service

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/opsnexus/svc-orchestration/internal/biz"
)

// Handler 实现编排服务的 HTTP REST API。
type Handler struct {
	workflowUC  *biz.WorkflowUseCase
	executionUC *biz.ExecutionUseCase
	log         *zap.SugaredLogger
}

// NewHandler 创建 HTTP 处理器实例。
func NewHandler(workflowUC *biz.WorkflowUseCase, executionUC *biz.ExecutionUseCase, log *zap.SugaredLogger) *Handler {
	return &Handler{
		workflowUC:  workflowUC,
		executionUC: executionUC,
		log:         log,
	}
}

// RegisterRoutes 注册所有 HTTP 路由。
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1", func(r chi.Router) {
		// 健康检查
		r.Get("/healthz", h.health)

		// 工作流管理
		r.Post("/workflows", h.createWorkflow)
		r.Get("/workflows", h.listWorkflows)
		r.Get("/workflows/templates", h.listTemplates)
		r.Post("/workflows/from-template", h.createFromTemplate)
		r.Get("/workflows/{id}", h.getWorkflow)
		r.Put("/workflows/{id}", h.updateWorkflow)
		r.Delete("/workflows/{id}", h.deleteWorkflow)

		// 执行管理
		r.Post("/executions", h.triggerExecution)
		r.Get("/executions", h.listExecutions)
		r.Get("/executions/{id}", h.getExecution)
		r.Get("/executions/{id}/logs", h.getExecutionLog)
		r.Post("/executions/{id}/cancel", h.cancelExecution)
		r.Post("/executions/{id}/steps/{stepIndex}/approve", h.approveStep)
		r.Post("/executions/{id}/steps/{stepIndex}/reject", h.rejectStep)
	})
}

// --- 健康检查 ---

// health 返回服务健康状态。
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "svc-orchestration"})
}

// --- 工作流 API ---

// createWorkflow 创建新的工作流模板。
func (h *Handler) createWorkflow(w http.ResponseWriter, r *http.Request) {
	var wf biz.Workflow
	if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}

	if err := h.workflowUC.CreateWorkflow(&wf); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, wf)
}

// listWorkflows 分页查询工作流模板列表。
func (h *Handler) listWorkflows(w http.ResponseWriter, r *http.Request) {
	pageSize, offset := parsePagination(r)

	var isActive *bool
	if v := r.URL.Query().Get("is_active"); v != "" {
		b := v == "true"
		isActive = &b
	}

	workflows, total, err := h.workflowUC.ListWorkflows(isActive, pageSize, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": workflows,
		"total": total,
	})
}

// listTemplates 返回所有预置工作流模板。
func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) {
	templates := h.workflowUC.ListTemplates()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": templates,
		"total": len(templates),
	})
}

// createFromTemplate 从预置模板创建工作流。
func (h *Handler) createFromTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TemplateID string `json:"template_id"`
		Name       string `json:"name"`
		CreatedBy  string `json:"created_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}

	wf, err := h.workflowUC.CreateFromTemplate(req.TemplateID, req.Name, req.CreatedBy)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, wf)
}

// getWorkflow 根据 ID 查询工作流详情。
func (h *Handler) getWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	wf, err := h.workflowUC.GetWorkflow(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "工作流不存在")
		return
	}
	writeJSON(w, http.StatusOK, wf)
}

// updateWorkflow 更新指定工作流。
func (h *Handler) updateWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var wf biz.Workflow
	if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	wf.ID = id

	if err := h.workflowUC.UpdateWorkflow(&wf); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, wf)
}

// deleteWorkflow 删除指定工作流。
func (h *Handler) deleteWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.workflowUC.DeleteWorkflow(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已删除"})
}

// --- 执行 API ---

// triggerExecution 触发工作流执行。
func (h *Handler) triggerExecution(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkflowID    string          `json:"workflow_id"`
		TriggerSource string          `json:"trigger_source"`
		Variables     json.RawMessage `json:"variables"`
		CreatedBy     string          `json:"created_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}

	exec, err := h.executionUC.TriggerExecution(req.WorkflowID, req.TriggerSource, req.CreatedBy, req.Variables)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, exec)
}

// listExecutions 分页查询执行记录列表。
func (h *Handler) listExecutions(w http.ResponseWriter, r *http.Request) {
	pageSize, offset := parsePagination(r)
	workflowID := r.URL.Query().Get("workflow_id")

	executions, total, err := h.executionUC.ListExecutions(workflowID, pageSize, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": executions,
		"total": total,
	})
}

// getExecution 根据 ID 查询执行记录详情。
func (h *Handler) getExecution(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	exec, err := h.executionUC.GetExecution(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "执行记录不存在")
		return
	}
	writeJSON(w, http.StatusOK, exec)
}

// getExecutionLog 获取完整的执行日志。
func (h *Handler) getExecutionLog(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	steps, err := h.executionUC.GetExecutionLog(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"execution_id": id,
		"steps":        steps,
	})
}

// cancelExecution 取消执行。
func (h *Handler) cancelExecution(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.executionUC.CancelExecution(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已取消"})
}

// approveStep 审批通过指定步骤。
func (h *Handler) approveStep(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stepIndexStr := chi.URLParam(r, "stepIndex")
	stepIndex, err := strconv.Atoi(stepIndexStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "无效的步骤索引")
		return
	}

	if err := h.executionUC.ApproveStep(id, stepIndex); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已审批通过"})
}

// rejectStep 拒绝指定步骤。
func (h *Handler) rejectStep(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stepIndexStr := chi.URLParam(r, "stepIndex")
	stepIndex, err := strconv.Atoi(stepIndexStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "无效的步骤索引")
		return
	}

	if err := h.executionUC.RejectStep(id, stepIndex); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "已拒绝"})
}

// --- 工具函数 ---

// writeJSON 将数据序列化为 JSON 并写入 HTTP 响应。
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应。
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// parsePagination 从请求参数中解析分页信息。
func parsePagination(r *http.Request) (pageSize int, offset int) {
	pageSize = 20
	offset = 0

	if v := r.URL.Query().Get("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}
