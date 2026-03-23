// Package service 提供变更管理领域的 HTTP API 处理器。
package service

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
	"github.com/opsnexus/opsnexus/pkg/httputil"
	"github.com/opsnexus/svc-change/internal/biz"
)

// Handler 实现变更管理的 REST API 处理器。
type Handler struct {
	changeUC   *biz.ChangeUsecase
	approvalUC *biz.ApprovalUsecase
	logger     *zap.Logger
}

// NewHandler 创建一个新的变更管理 HTTP 处理器实例。
func NewHandler(changeUC *biz.ChangeUsecase, approvalUC *biz.ApprovalUsecase, logger *zap.Logger) *Handler {
	return &Handler{
		changeUC:   changeUC,
		approvalUC: approvalUC,
		logger:     logger,
	}
}

// RegisterRoutes 将变更管理相关路由注册到 chi 路由器上。
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/changes", func(r chi.Router) {
		// 变更单 CRUD
		r.Post("/", h.createChange)
		r.Get("/", h.listChanges)
		r.Get("/calendar", h.getCalendar)
		r.Get("/conflicts", h.checkConflicts)

		// 单个变更单操作
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.getChange)
			r.Put("/", h.updateChange)
			r.Post("/submit", h.submitChange)
			r.Post("/approve", h.approveChange)
			r.Post("/reject", h.rejectChange)
			r.Post("/start", h.startExecution)
			r.Post("/complete", h.completeChange)
			r.Post("/cancel", h.cancelChange)
		})
	})
}

// createChange 创建新的变更单。
func (h *Handler) createChange(w http.ResponseWriter, r *http.Request) {
	var req biz.CreateChangeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	ticket, err := h.changeUC.Create(r.Context(), &req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, ticket)
}

// listChanges 查询变更单列表，支持分页和筛选。
func (h *Handler) listChanges(w http.ResponseWriter, r *http.Request) {
	p := httputil.ParsePagination(r)
	f := biz.ListFilter{
		Page:     p.Page,
		PageSize: p.PageSize,
	}

	// 解析筛选参数
	if v := r.URL.Query().Get("status"); v != "" {
		s := biz.ChangeStatus(v)
		f.Status = &s
	}
	if v := r.URL.Query().Get("type"); v != "" {
		t := biz.ChangeType(v)
		f.Type = &t
	}
	if v := r.URL.Query().Get("risk_level"); v != "" {
		rl := biz.RiskLevel(v)
		f.RiskLevel = &rl
	}
	if v := r.URL.Query().Get("requester"); v != "" {
		f.Requester = &v
	}

	// 解析排序参数
	sortBy, sortOrder := httputil.ParseSortParams(r, map[string]bool{
		"created_at": true, "scheduled_start": true, "risk_level": true, "status": true, "updated_at": true,
	}, "created_at")
	f.SortBy = sortBy
	f.SortOrder = sortOrder

	tickets, total, err := h.changeUC.List(r.Context(), f)
	if err != nil {
		h.handleError(w, err)
		return
	}

	httputil.PagedJSON(w, tickets, p.Page, p.PageSize, total)
}

// getChange 根据 ID 查询单条变更单详情。
func (h *Handler) getChange(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ticket, err := h.changeUC.GetByID(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, ticket)
}

// updateChange 更新变更单信息（仅 draft 状态允许）。
func (h *Handler) updateChange(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req biz.UpdateChangeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	ticket, err := h.changeUC.Update(r.Context(), id, &req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, ticket)
}

// submitChange 提交变更单进入审批流程。
func (h *Handler) submitChange(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.changeUC.Submit(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}

// approveChange 审批通过变更单。
func (h *Handler) approveChange(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		ApproverID string `json:"approver_id"`
		Comment    string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if body.ApproverID == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "approver_id is required"))
		return
	}

	if err := h.approvalUC.Approve(r.Context(), id, body.ApproverID, body.Comment); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// rejectChange 拒绝变更单。
func (h *Handler) rejectChange(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		ApproverID string `json:"approver_id"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if body.ApproverID == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "approver_id is required"))
		return
	}

	if err := h.approvalUC.Reject(r.Context(), id, body.ApproverID, body.Reason); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// startExecution 开始执行变更。
func (h *Handler) startExecution(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.changeUC.StartExecution(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"status": "in_progress"})
}

// completeChange 完成变更。
func (h *Handler) completeChange(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.changeUC.Complete(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// cancelChange 取消变更。
func (h *Handler) cancelChange(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if err := h.changeUC.Cancel(r.Context(), id, body.Reason); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// getCalendar 获取变更日历，返回未来 30 天的变更排期。
func (h *Handler) getCalendar(w http.ResponseWriter, r *http.Request) {
	tickets, err := h.changeUC.GetCalendar(r.Context(), 30)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"changes": tickets,
		"total":   len(tickets),
	})
}

// checkConflicts 检测变更冲突。
func (h *Handler) checkConflicts(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	excludeID := r.URL.Query().Get("exclude_id")

	if startStr == "" || endStr == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "start and end time are required"))
		return
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "invalid start time format, use RFC3339"))
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "invalid end time format, use RFC3339"))
		return
	}

	// 从查询参数获取资产列表
	assets := r.URL.Query()["assets"]

	conflicts, err := h.changeUC.CheckConflicts(r.Context(), start, end, assets, excludeID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"conflicts": conflicts,
		"total":     len(conflicts),
	})
}

// handleError 将 AppError 映射为对应的 HTTP 错误响应。
func (h *Handler) handleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		httputil.Error(w, appErr)
		return
	}
	httputil.Error(w, apperrors.Internal("INTERNAL", err.Error()))
}
