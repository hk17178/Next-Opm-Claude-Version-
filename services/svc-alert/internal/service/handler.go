package service

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/opsnexus/svc-alert/internal/biz"
)

// Handler 实现告警服务的 HTTP REST API，提供规则 CRUD、告警查询/确认/解决、
// 静默管理以及手动评估等接口，路由挂载在 /api/v1/alert 下。
type Handler struct {
	ruleUC      *biz.RuleUseCase
	engine      *biz.AlertEngine
	alertRepo   biz.AlertRepository
	silenceRepo biz.SilenceRepository
	// silenceUC 封装了沉默规则的业务逻辑，包括校验、ID 生成和持久化
	silenceUC   *biz.SilenceUseCase
	log         *zap.SugaredLogger
}

// NewHandler 创建 HTTP 处理器实例。
//
// 参数：
//   - ruleUC：告警规则用例，处理规则 CRUD 和启用/禁用逻辑。
//   - engine：告警评估引擎，处理指标/日志事件的规则匹配。
//   - alertRepo：告警实例仓储，用于查询和状态更新。
//   - silenceRepo：沉默规则仓储接口，直接用于列表查询。
//   - silenceUC：沉默规则用例，封装创建/删除的业务校验逻辑。
//   - log：结构化日志记录器。
func NewHandler(
	ruleUC *biz.RuleUseCase,
	engine *biz.AlertEngine,
	alertRepo biz.AlertRepository,
	silenceRepo biz.SilenceRepository,
	silenceUC *biz.SilenceUseCase,
	log *zap.SugaredLogger,
) *Handler {
	return &Handler{
		ruleUC:      ruleUC,
		engine:      engine,
		alertRepo:   alertRepo,
		silenceRepo: silenceRepo,
		silenceUC:   silenceUC,
		log:         log,
	}
}

// RegisterRoutes 按 OpenAPI 规范在 /api/v1/alert 下注册所有路由。
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/alert", func(r chi.Router) {
		// Health
		r.Get("/health", h.health)

		// Rules CRUD
		r.Route("/rules", func(r chi.Router) {
			r.Get("/", h.listRules)
			r.Post("/", h.createRule)
			r.Get("/{rule_id}", h.getRule)
			r.Put("/{rule_id}", h.updateRule)
			r.Delete("/{rule_id}", h.deleteRule)
			r.Post("/{rule_id}/enable", h.enableRule)
			r.Post("/{rule_id}/disable", h.disableRule)
		})

		// Alerts
		r.Route("/alerts", func(r chi.Router) {
			r.Get("/", h.listAlerts)
			r.Get("/{alert_id}", h.getAlert)
			r.Post("/{alert_id}/acknowledge", h.acknowledgeAlert)
			r.Post("/{alert_id}/resolve", h.resolveAlert)
		})

		// Silences — 沉默规则持久化到 PostgreSQL silences 表，重启后不丢失
		r.Route("/silences", func(r chi.Router) {
			r.Get("/", h.listSilences)
			r.Post("/", h.createSilence)
			// DELETE /silences/{silence_id}：提前解除静默（维护窗口提前结束等场景）
			r.Delete("/{silence_id}", h.deleteSilence)
		})

		// Manual evaluation endpoint (for testing/debugging)
		r.Post("/evaluate/metric", h.evaluateMetric)
		r.Post("/evaluate/log", h.evaluateLog)
	})
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "svc-alert"})
}

// --- 规则管理 ---

// createRule 创建告警规则（POST /rules）。
func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	var rule biz.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if err := h.ruleUC.CreateRule(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, rule)
}

// getRule 按 ID 查询规则（GET /rules/{rule_id}）。
func (h *Handler) getRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "rule_id")
	rule, err := h.ruleUC.GetRule(id)
	if err != nil || rule == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "rule not found")
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

// updateRule 更新告警规则（PUT /rules/{rule_id}）。
func (h *Handler) updateRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "rule_id")
	var rule biz.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	rule.RuleID = id

	if err := h.ruleUC.UpdateRule(&rule); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// deleteRule 删除告警规则（DELETE /rules/{rule_id}）。
func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "rule_id")
	if err := h.ruleUC.DeleteRule(id); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// enableRule 启用告警规则（POST /rules/{rule_id}/enable），立即生效。
func (h *Handler) enableRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "rule_id")
	if err := h.ruleUC.EnableRule(id); err != nil {
		if err.Error() == "rule not found" {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

// disableRule 禁用告警规则（POST /rules/{rule_id}/disable），立即生效。
func (h *Handler) disableRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "rule_id")
	if err := h.ruleUC.DisableRule(id); err != nil {
		if err.Error() == "rule not found" {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

// listRules 分页查询规则列表（GET /rules），支持 enabled 过滤。
func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	pageSize := queryInt(r, "page_size", 20)
	pageToken := r.URL.Query().Get("page_token")

	var enabled *bool
	if e := r.URL.Query().Get("enabled"); e != "" {
		b, _ := strconv.ParseBool(e)
		enabled = &b
	}

	rules, nextToken, err := h.ruleUC.ListRules(enabled, pageSize, pageToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := map[string]interface{}{"rules": rules}
	if nextToken != "" {
		resp["next_page_token"] = nextToken
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- 告警查询与操作 ---

// listAlerts 分页查询告警列表（GET /alerts），支持 status 和 severity 过滤。
func (h *Handler) listAlerts(w http.ResponseWriter, r *http.Request) {
	pageSize := queryInt(r, "page_size", 20)
	pageToken := r.URL.Query().Get("page_token")

	var status *biz.AlertStatus
	if s := r.URL.Query().Get("status"); s != "" {
		st := biz.AlertStatus(s)
		status = &st
	}
	var severity *biz.Severity
	if s := r.URL.Query().Get("severity"); s != "" {
		sv := biz.Severity(s)
		severity = &sv
	}

	alerts, nextToken, err := h.alertRepo.List(status, severity, pageSize, pageToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := map[string]interface{}{"alerts": alerts}
	if nextToken != "" {
		resp["next_page_token"] = nextToken
	}
	writeJSON(w, http.StatusOK, resp)
}

// getAlert 按 ID 查询单条告警（GET /alerts/{alert_id}）。
func (h *Handler) getAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "alert_id")
	alert, err := h.alertRepo.GetByID(id)
	if err != nil || alert == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "alert not found")
		return
	}
	writeJSON(w, http.StatusOK, alert)
}

// acknowledgeAlert 确认告警（POST /alerts/{alert_id}/acknowledge），将状态更新为 acknowledged。
func (h *Handler) acknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "alert_id")

	alert, err := h.alertRepo.GetByID(id)
	if err != nil || alert == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "alert not found")
		return
	}

	now := time.Now()
	if err := h.alertRepo.UpdateStatus(id, biz.AlertStatusAcknowledged, &now, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}

// resolveAlert 解决告警（POST /alerts/{alert_id}/resolve），将状态更新为 resolved。
func (h *Handler) resolveAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "alert_id")

	alert, err := h.alertRepo.GetByID(id)
	if err != nil || alert == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "alert not found")
		return
	}

	now := time.Now()
	if err := h.alertRepo.UpdateStatus(id, biz.AlertStatusResolved, nil, &now); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

// --- 静默管理（持久化到 PostgreSQL silences 表）---
// 沉默规则通过 SilenceUseCase 处理业务校验，并最终写入 PostgreSQL。
// 重启后静默规则不会丢失，保证维护窗口内告警持续被正确抑制。

// listSilences 查询静默规则列表（GET /silences）。
// 返回所有静默规则（含已过期），按创建时间倒序排列。
func (h *Handler) listSilences(w http.ResponseWriter, r *http.Request) {
	// 通过 SilenceUseCase 查询，确保返回空切片而非 null
	silences, err := h.silenceUC.ListSilences()
	if err != nil {
		h.log.Errorw("failed to list silences", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list silences")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"silences": silences})
}

// createSilence 创建静默规则（POST /silences），通过 SilenceUseCase 完成业务校验后持久化到 PostgreSQL。
//
// 请求体（JSON）：
//
//	{
//	  "matchers": [{"label": "service", "value": "payment", "is_regex": false}],
//	  "starts_at": "2024-01-01T00:00:00Z",  // 可选，默认立即生效
//	  "ends_at":   "2024-01-01T08:00:00Z",  // 必填
//	  "comment":   "Planned maintenance",    // 可选
//	  "created_by": "ops-team"               // 可选
//	}
func (h *Handler) createSilence(w http.ResponseWriter, r *http.Request) {
	var silence biz.Silence
	if err := json.NewDecoder(r.Body).Decode(&silence); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	// 通过 SilenceUseCase 执行业务校验（matchers 非空、ends_at 必填、时间合法性检查）
	// 并自动完成 ID 生成、starts_at 默认值设置，然后写入 PostgreSQL
	if err := h.silenceUC.CreateSilence(&silence); err != nil {
		h.log.Errorw("failed to create silence", "error", err)
		// 区分校验错误（400）和内部错误（500）
		if isValidationError(err) {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create silence")
		}
		return
	}

	h.log.Infow("silence rule created via HTTP",
		"id", silence.ID,
		"ends_at", silence.EndsAt,
		"matchers", len(silence.Matchers),
	)
	writeJSON(w, http.StatusCreated, silence)
}

// deleteSilence 删除静默规则（DELETE /silences/{silence_id}），提前解除静默。
// 对应 OpenAPI DELETE /api/v1/alert/silences/{silence_id} 接口。
func (h *Handler) deleteSilence(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "silence_id")

	if err := h.silenceUC.DeleteSilence(id); err != nil {
		h.log.Errorw("failed to delete silence", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete silence")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// isValidationError 判断错误是否属于业务校验错误（用于区分 400 和 500 响应码）。
// 当前通过检查错误消息中的关键词来判断，与 SilenceUseCase 中的 errors.New 保持一致。
func isValidationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// SilenceUseCase.CreateSilence 中的校验错误消息特征
	return msg == "at least one matcher is required" ||
		msg == "ends_at is required" ||
		msg == "ends_at must be after starts_at" ||
		msg == "silence id is required"
}

// --- 手动评估（用于测试和调试）---

// evaluateMetric 手动提交指标数据点进行评估（POST /evaluate/metric）。
func (h *Handler) evaluateMetric(w http.ResponseWriter, r *http.Request) {
	var sample biz.MetricSample
	if err := json.NewDecoder(r.Body).Decode(&sample); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid metric sample")
		return
	}

	result, err := h.engine.EvaluateMetric(sample)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// evaluateLog 手动提交日志事件进行评估（POST /evaluate/log）。
func (h *Handler) evaluateLog(w http.ResponseWriter, r *http.Request) {
	var event biz.LogEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid log event")
		return
	}

	result, err := h.engine.EvaluateLog(event)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// --- 辅助函数 ---

// writeJSON 将数据序列化为 JSON 并写入 HTTP 响应。
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError 返回统一格式的错误 JSON 响应。
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"code": code, "message": msg})
}

// queryInt 从 URL 查询参数中解析整数值，无效时返回 fallback，上限 100。
func queryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fallback
	}
	if n > 100 {
		return 100
	}
	return n
}
