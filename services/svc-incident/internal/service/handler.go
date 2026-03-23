// Package service 提供事件管理领域的 HTTP API 处理器和 gRPC 服务实现。
package service

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
	"github.com/opsnexus/opsnexus/pkg/httputil"
	"github.com/opsnexus/svc-incident/internal/biz"
)

// Handler 实现事件管理的 REST API 处理器，提供事件 CRUD、时间线、变更工单和值班排班等接口。
type Handler struct {
	uc       *biz.IncidentUsecase
	changeUC *biz.ChangeOrderUsecase
	logger   *zap.Logger
}

// NewHandler 创建一个新的事件 HTTP 处理器实例。
func NewHandler(uc *biz.IncidentUsecase, logger *zap.Logger) *Handler {
	return &Handler{uc: uc, logger: logger}
}

// SetChangeOrderUsecase 注入变更工单用例，支持可选注入避免破坏现有构造逻辑。
func (h *Handler) SetChangeOrderUsecase(coUC *biz.ChangeOrderUsecase) {
	h.changeUC = coUC
}

// RegisterRoutes 将事件管理相关路由注册到 chi 路由器上。
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/incident", func(r chi.Router) {
		// 事件管理接口
		r.Get("/incidents", h.listIncidents)
		r.Post("/incidents", h.createIncident)
		r.Get("/incidents/{incident_id}", h.getIncident)
		r.Patch("/incidents/{incident_id}", h.updateIncident)
		r.Post("/incidents/{incident_id}/assign", h.assignIncident)
		r.Post("/incidents/{incident_id}/escalate", h.escalateIncident)
		r.Post("/incidents/{incident_id}/resolve", h.resolveIncident)
		r.Post("/incidents/{incident_id}/postmortem", h.addPostmortem)
		r.Get("/incidents/{incident_id}/metrics", h.getMetrics)

		// 事件时间线接口
		r.Get("/incidents/{incident_id}/timeline", h.getTimeline)
		r.Post("/incidents/{incident_id}/timeline", h.addTimelineEntry)

		// 事件变更工单关联接口
		// POST  /api/v1/incident/incidents/{incident_id}/changes  关联一个变更工单到事件
		// GET   /api/v1/incident/incidents/{incident_id}/changes  查询事件关联的所有变更工单
		r.Post("/incidents/{incident_id}/changes", h.addIncidentChange)
		r.Get("/incidents/{incident_id}/changes", h.listIncidentChanges)

		// 变更工单接口
		r.Get("/changes", h.listChangeOrders)
		r.Post("/changes", h.createChangeOrder)
		r.Get("/changes/{change_id}", h.getChangeOrder)
		r.Patch("/changes/{change_id}", h.updateChangeOrder)
		r.Post("/changes/{change_id}/link-incident", h.linkIncidentToChange)

		// 值班排班接口
		r.Get("/schedules", h.listSchedules)
		r.Post("/schedules", h.createSchedule)
		r.Get("/schedules/{schedule_id}", h.getSchedule)
		r.Put("/schedules/{schedule_id}", h.updateSchedule)
		r.Delete("/schedules/{schedule_id}", h.deleteSchedule)
	})
}

// --- 事件管理 ---

// listIncidents 查询事件列表，支持按状态、严重等级、负责人和业务单元过滤。
func (h *Handler) listIncidents(w http.ResponseWriter, r *http.Request) {
	p := httputil.ParsePagination(r)
	f := biz.ListFilter{
		Page:     p.Page,
		PageSize: p.PageSize,
	}

	if v := r.URL.Query().Get("status"); v != "" {
		s := biz.Status(v)
		f.Status = &s
	}
	if v := r.URL.Query().Get("severity"); v != "" {
		f.Severity = &v
	}
	if v := r.URL.Query().Get("assignee_id"); v != "" {
		f.AssigneeID = &v
	}
	if v := r.URL.Query().Get("business_unit"); v != "" {
		f.BusinessUnit = &v
	}
	sortBy, sortOrder := httputil.ParseSortParams(r, map[string]bool{
		"created_at": true, "detected_at": true, "severity": true, "status": true, "updated_at": true,
	}, "created_at")
	f.SortBy = sortBy
	f.SortOrder = sortOrder

	incidents, total, err := h.uc.ListIncidents(r.Context(), f)
	if err != nil {
		h.handleError(w, err)
		return
	}

	httputil.PagedJSON(w, incidents, p.Page, p.PageSize, total)
}

// createIncident 创建新的事件工单。
func (h *Handler) createIncident(w http.ResponseWriter, r *http.Request) {
	var req biz.CreateIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	if req.Title == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "title is required"))
		return
	}
	if req.Severity == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "severity is required"))
		return
	}

	inc, err := h.uc.CreateIncident(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, inc)
}

// getIncident 根据 ID 查询单条事件详情。
func (h *Handler) getIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "incident_id")
	inc, err := h.uc.GetIncident(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, inc)
}

// updateIncident 更新事件状态（PATCH），当前仅支持状态流转操作。
func (h *Handler) updateIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "incident_id")
	var body struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	if body.Status != "" {
		inc, err := h.uc.UpdateStatus(r.Context(), id, biz.Status(body.Status), body.Note)
		if err != nil {
			h.handleError(w, err)
			return
		}
		httputil.JSON(w, http.StatusOK, inc)
		return
	}

	httputil.Error(w, apperrors.BadRequest("VALIDATION", "status is required for update"))
}

// assignIncident 分配事件处理人。
func (h *Handler) assignIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "incident_id")
	var body struct {
		AssigneeID   string `json:"assignee_id"`
		AssigneeName string `json:"assignee_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if body.AssigneeID == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "assignee_id is required"))
		return
	}

	inc, err := h.uc.AssignIncident(r.Context(), id, body.AssigneeID, body.AssigneeName)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, inc)
}

// escalateIncident 升级事件严重等级。
func (h *Handler) escalateIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "incident_id")
	var body struct {
		Severity string `json:"severity"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if body.Severity == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "severity is required"))
		return
	}

	inc, err := h.uc.EscalateIncident(r.Context(), id, body.Severity, body.Reason)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, inc)
}

// resolveIncident 将事件标记为已解决。
func (h *Handler) resolveIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "incident_id")
	var body struct {
		ResolutionNote string `json:"resolution_note"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	inc, err := h.uc.UpdateStatus(r.Context(), id, biz.StatusResolved, body.ResolutionNote)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, inc)
}

// addPostmortem 为事件添加复盘报告。
func (h *Handler) addPostmortem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "incident_id")
	var pm biz.Postmortem
	if err := json.NewDecoder(r.Body).Decode(&pm); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if pm.RootCause == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "root_cause is required"))
		return
	}

	inc, err := h.uc.AddPostmortem(r.Context(), id, &pm)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, inc)
}

// getMetrics 查询事件的 MTTA/MTTI/MTTR 时效指标。
func (h *Handler) getMetrics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "incident_id")
	m, err := h.uc.GetMetrics(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, m)
}

// --- 事件时间线 ---

// getTimeline 查询指定事件的完整时间线。
func (h *Handler) getTimeline(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "incident_id")
	entries, err := h.uc.GetTimeline(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// addTimelineEntry 向事件时间线添加一条手动记录。
func (h *Handler) addTimelineEntry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "incident_id")
	var body struct {
		Type    string `json:"type"`
		Content any    `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	entry := &biz.TimelineEntry{
		IncidentID: id,
		Timestamp:  time.Now(),
		EntryType:  body.Type,
		Source:     "human",
		Content:    body.Content,
	}
	if err := h.uc.AddTimelineEntry(r.Context(), id, entry); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, entry)
}

// --- 事件变更工单关联 ---

// addIncidentChange 将一个变更工单关联到指定事件。
//
// 请求路径：POST /api/v1/incident/incidents/{incident_id}/changes
//
// 请求体字段：
//   - change_order_id (string, 必填): 变更工单 ID，如 CHG-20250101-001
//   - description     (string, 可选): 关联原因描述，说明该变更与事件的关系
//   - operator_id     (string, 可选): 操作人用户 ID
//   - operator_name   (string, 可选): 操作人姓名
//
// 响应：201 Created，返回创建的关联记录。
func (h *Handler) addIncidentChange(w http.ResponseWriter, r *http.Request) {
	incidentID := chi.URLParam(r, "incident_id")

	// 解析请求体
	var req biz.AddIncidentChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	// change_order_id 为必填字段
	if req.ChangeOrderID == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "change_order_id is required"))
		return
	}

	change, err := h.uc.AddIncidentChange(r.Context(), incidentID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, change)
}

// listIncidentChanges 查询指定事件关联的所有变更工单记录。
//
// 请求路径：GET /api/v1/incident/incidents/{incident_id}/changes
//
// 响应：200 OK，返回 {"changes": [...], "total": N} 格式。
func (h *Handler) listIncidentChanges(w http.ResponseWriter, r *http.Request) {
	incidentID := chi.URLParam(r, "incident_id")

	changes, err := h.uc.ListIncidentChanges(r.Context(), incidentID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"changes": changes,
		"total":   len(changes),
	})
}

// --- 变更工单 ---

// listChangeOrders 查询变更工单列表，支持按状态过滤和分页。
func (h *Handler) listChangeOrders(w http.ResponseWriter, r *http.Request) {
	if h.changeUC == nil {
		httputil.JSON(w, http.StatusOK, map[string]any{"changes": []any{}, "total": 0})
		return
	}
	p := httputil.ParsePagination(r)
	status := r.URL.Query().Get("status")

	orders, total, err := h.changeUC.ListChangeOrders(r.Context(), status, p.Page, p.PageSize)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.PagedJSON(w, orders, p.Page, p.PageSize, total)
}

// createChangeOrder 创建新的变更工单。
func (h *Handler) createChangeOrder(w http.ResponseWriter, r *http.Request) {
	if h.changeUC == nil {
		httputil.JSON(w, http.StatusCreated, map[string]string{"status": "placeholder"})
		return
	}
	var co biz.ChangeOrder
	if err := json.NewDecoder(r.Body).Decode(&co); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if co.Title == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "title is required"))
		return
	}
	if co.ChangeType == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "change_type is required"))
		return
	}
	if co.RiskLevel == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "risk_level is required"))
		return
	}

	if err := h.changeUC.CreateChangeOrder(r.Context(), &co); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, co)
}

// getChangeOrder 根据 ID 查询单条变更工单。
func (h *Handler) getChangeOrder(w http.ResponseWriter, r *http.Request) {
	if h.changeUC == nil {
		httputil.Error(w, apperrors.NotFound(apperrors.ErrChangeOrderNotFound, "change order service not configured"))
		return
	}
	id := chi.URLParam(r, "change_id")
	co, err := h.changeUC.GetChangeOrder(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, co)
}

// updateChangeOrder 更新变更工单字段。
func (h *Handler) updateChangeOrder(w http.ResponseWriter, r *http.Request) {
	if h.changeUC == nil {
		httputil.Error(w, apperrors.NotFound(apperrors.ErrChangeOrderNotFound, "change order service not configured"))
		return
	}
	id := chi.URLParam(r, "change_id")
	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	co, err := h.changeUC.UpdateChangeOrder(r.Context(), id, updates)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, co)
}

// linkIncidentToChange 将事件关联到变更工单。
func (h *Handler) linkIncidentToChange(w http.ResponseWriter, r *http.Request) {
	if h.changeUC == nil {
		httputil.Error(w, apperrors.NotFound(apperrors.ErrChangeOrderNotFound, "change order service not configured"))
		return
	}
	changeID := chi.URLParam(r, "change_id")
	var body struct {
		IncidentID string `json:"incident_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if body.IncidentID == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "incident_id is required"))
		return
	}

	if err := h.changeUC.LinkIncidentToChangeOrder(r.Context(), changeID, body.IncidentID); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"status": "linked"})
}

// --- 值班排班（占位 CRUD） ---

// listSchedules 查询值班排班列表（占位实现）。
func (h *Handler) listSchedules(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusOK, map[string]any{"schedules": []any{}})
}

// createSchedule 创建值班排班（占位实现）。
func (h *Handler) createSchedule(w http.ResponseWriter, r *http.Request) {
	httputil.JSON(w, http.StatusCreated, map[string]string{"status": "placeholder"})
}

// getSchedule 查询单条值班排班（占位实现）。
func (h *Handler) getSchedule(w http.ResponseWriter, r *http.Request) {
	httputil.Error(w, apperrors.NotFound(apperrors.ErrScheduleNotFound, "not implemented yet"))
}

// updateSchedule 更新值班排班（占位实现）。
func (h *Handler) updateSchedule(w http.ResponseWriter, r *http.Request) {
	httputil.Error(w, apperrors.NotFound(apperrors.ErrScheduleNotFound, "not implemented yet"))
}

// deleteSchedule 删除值班排班（占位实现）。
func (h *Handler) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// handleError 将 AppError 映射为对应的 HTTP 错误响应。
func (h *Handler) handleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		httputil.Error(w, appErr)
		return
	}
	httputil.Error(w, apperrors.Internal("INTERNAL", err.Error()))
}
