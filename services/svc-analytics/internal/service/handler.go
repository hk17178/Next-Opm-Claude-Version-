// Package service 实现分析域（Analytics）的 HTTP REST API 处理器。
package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
	"github.com/opsnexus/opsnexus/pkg/httputil"
	"github.com/opsnexus/svc-analytics/internal/biz"
)

// Handler 实现分析域的 REST API，聚合 SLA、指标、报表、仪表盘、知识库、API Token、
// 建议管理、报告模板、数据导出、数据导入和诊断包收集等子用例。
type Handler struct {
	slaUC            *biz.SLAUsecase            // SLA 计算用例
	metricsUC        *biz.MetricsUsecase        // 指标查询与摄入用例
	reportUC         *biz.ReportUsecase         // 报表管理与执行用例
	dashboardUC      *biz.DashboardUsecase      // 仪表盘 CRUD 用例
	knowledgeUC      *biz.KnowledgeUsecase      // 知识库管理与搜索用例
	tokenUC          *biz.APITokenUsecase       // API Token 管理用例
	suggestionUC     *biz.SuggestionUsecase     // 用户建议管理用例
	reportTemplateUC *biz.ReportTemplateUsecase // 报告模板用例
	dataExportUC     *biz.DataExportUsecase     // 数据导出用例
	dataImportUC     *biz.DataImportUsecase     // 数据导入用例（FR-29-002）
	diagnosticsUC    *biz.DiagnosticsUsecase    // 诊断包收集用例（FR-29-004）
	logger           *zap.Logger                // 日志记录器
}

// NewHandler 创建分析域 HTTP 处理器实例，注入各子领域用例。
func NewHandler(
	slaUC *biz.SLAUsecase,
	metricsUC *biz.MetricsUsecase,
	reportUC *biz.ReportUsecase,
	dashboardUC *biz.DashboardUsecase,
	knowledgeUC *biz.KnowledgeUsecase,
	tokenUC *biz.APITokenUsecase,
	suggestionUC *biz.SuggestionUsecase,
	reportTemplateUC *biz.ReportTemplateUsecase,
	dataExportUC *biz.DataExportUsecase,
	dataImportUC *biz.DataImportUsecase,
	diagnosticsUC *biz.DiagnosticsUsecase,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		slaUC:            slaUC,
		metricsUC:        metricsUC,
		reportUC:         reportUC,
		dashboardUC:      dashboardUC,
		knowledgeUC:      knowledgeUC,
		tokenUC:          tokenUC,
		suggestionUC:     suggestionUC,
		reportTemplateUC: reportTemplateUC,
		dataExportUC:     dataExportUC,
		dataImportUC:     dataImportUC,
		diagnosticsUC:    diagnosticsUC,
		logger:           logger,
	}
}

// RegisterRoutes 将分析域所有路由挂载到 chi 路由器上。
// 路由路径遵循 docs/api/svc-analytics.yaml 中的 OpenAPI 契约定义。
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/analytics", func(r chi.Router) {
		// Ad-hoc query (OpenAPI: /query)
		r.Post("/query", h.executeQuery)

		// Metrics (OpenAPI: /metrics, /metrics/query)
		r.Post("/metrics", h.ingestMetrics)
		r.Post("/metrics/query", h.queryMetrics)

		// Dashboards (OpenAPI: /dashboards)
		r.Get("/dashboards", h.listDashboards)
		r.Post("/dashboards", h.createDashboard)
		r.Get("/dashboards/{dashboard_id}", h.getDashboard)
		r.Put("/dashboards/{dashboard_id}", h.updateDashboard)
		r.Delete("/dashboards/{dashboard_id}", h.deleteDashboard)

		// Reports (OpenAPI: /reports)
		r.Get("/reports", h.listReports)
		r.Post("/reports", h.createReport)
		r.Get("/reports/{report_id}", h.getReport)
		r.Delete("/reports/{report_id}", h.deleteReport)
		r.Post("/reports/{report_id}/run", h.runReport)

		// SLA (internal domain endpoints, not in OpenAPI but listed in ON-003 4.3)
		r.Get("/sla", h.listSLAConfigs)
		r.Post("/sla", h.createSLAConfig)
		r.Get("/sla/{config_id}", h.getSLAConfig)
		r.Put("/sla/{config_id}", h.updateSLAConfig)
		r.Delete("/sla/{config_id}", h.deleteSLAConfig)
		r.Post("/sla/calculate", h.calculateSLA)
		r.Post("/sla/report", h.slaReport)

		// Report export
		r.Get("/reports/{report_id}/export", h.exportReport)

		// Correlations (ON-003 4.3: correlations resource)
		r.Post("/correlations", h.correlateMetrics)

		// Knowledge (ON-003 4.3: knowledge resource)
		r.Get("/knowledge", h.listKnowledge)
		r.Post("/knowledge", h.createKnowledge)
		r.Get("/knowledge/{article_id}", h.getKnowledge)
		r.Put("/knowledge/{article_id}", h.updateKnowledge)
		r.Delete("/knowledge/{article_id}", h.deleteKnowledge)
		r.Post("/knowledge/search", h.searchKnowledge)
	})

	// 用户建议管理端点（FR-16-001~006）
	r.Route("/api/v1/suggestions", func(r chi.Router) {
		r.Post("/", h.submitSuggestion)              // 提交建议
		r.Get("/", h.listSuggestions)                 // 列表（支持状态筛选）
		r.Get("/stats", h.getSuggestionStats)         // 统计数据
		r.Put("/{id}/status", h.updateSuggestionStatus) // 更新状态
	})

	// 报告模板端点（FR-15-009~011, FR-15-013）
	r.Route("/api/v1/reports", func(r chi.Router) {
		r.Get("/templates", h.listReportTemplates)    // 列出所有预置模板
		r.Post("/generate", h.generateReport)         // 生成报告
		r.Get("/{id}", h.getGeneratedReport)          // 获取已生成报告
		r.Get("/{id}/export", h.exportGeneratedReport) // 导出报告
	})

	// 数据导出端点（FR-29-001）
	r.Route("/api/v1/exports", func(r chi.Router) {
		r.Post("/", h.createExportTask)    // 创建导出任务
		r.Get("/{id}", h.getExportTask)    // 查询导出任务状态
	})

	// API Token 管理端点（FR-28）
	r.Route("/api/v1/tokens", func(r chi.Router) {
		r.Post("/", h.createToken)       // 生成新 Token
		r.Get("/", h.listTokens)         // 列出当前用户的 Token
		r.Delete("/{id}", h.revokeToken) // 吊销指定 Token
	})

	// 数据导入端点（FR-29-002）
	r.Route("/api/v1/imports", func(r chi.Router) {
		r.Post("/", h.createImportTask)    // 创建导入任务
		r.Get("/{id}", h.getImportTask)    // 查询导入任务进度
	})

	// 诊断包收集端点（FR-29-004）
	r.Route("/api/v1/diagnostics", func(r chi.Router) {
		r.Post("/", h.collectDiagnostics)              // 触发收集诊断包
		r.Get("/{id}/download", h.getDiagnosticsDownload) // 获取下载链接
	})
}

// --- Ad-hoc Query ---

func (h *Handler) executeQuery(w http.ResponseWriter, r *http.Request) {
	var req biz.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if req.Query == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "query is required"))
		return
	}

	result, err := h.metricsUC.ExecuteQuery(r.Context(), req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, result)
}

// --- Metrics ---

func (h *Handler) ingestMetrics(w http.ResponseWriter, r *http.Request) {
	var req biz.MetricIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if len(req.Metrics) == 0 {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "at least one metric is required"))
		return
	}

	if err := h.metricsUC.Ingest(r.Context(), req); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) queryMetrics(w http.ResponseWriter, r *http.Request) {
	var q biz.MetricQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	result, err := h.metricsUC.Query(r.Context(), q)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, result)
}

// --- Dashboards ---

func (h *Handler) listDashboards(w http.ResponseWriter, r *http.Request) {
	filter := biz.DashboardListFilter{
		PageToken: r.URL.Query().Get("page_token"),
		PageSize:  20,
	}
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		fmt.Sscanf(ps, "%d", &filter.PageSize)
	}

	dashboards, nextToken, err := h.dashboardUC.List(r.Context(), filter)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"dashboards":      dashboards,
		"next_page_token": nextToken,
	})
}

func (h *Handler) createDashboard(w http.ResponseWriter, r *http.Request) {
	var d biz.Dashboard
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if d.Name == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "name is required"))
		return
	}

	if err := h.dashboardUC.Create(r.Context(), &d); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, d)
}

func (h *Handler) getDashboard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "dashboard_id")
	d, err := h.dashboardUC.Get(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, d)
}

func (h *Handler) updateDashboard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "dashboard_id")
	var d biz.Dashboard
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	d.ID = id
	if err := h.dashboardUC.Update(r.Context(), &d); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, d)
}

func (h *Handler) deleteDashboard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "dashboard_id")
	if err := h.dashboardUC.Delete(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Reports ---

func (h *Handler) listReports(w http.ResponseWriter, r *http.Request) {
	p := httputil.ParsePagination(r)
	filter := biz.ReportListFilter{Page: p.Page, PageSize: p.PageSize}
	if v := r.URL.Query().Get("status"); v != "" {
		filter.Status = &v
	}

	reports, total, err := h.reportUC.List(r.Context(), filter)
	if err != nil {
		h.handleError(w, err)
		return
	}
	// BUG-007 修复：total 由 ReportUsecase.List 返回 int 类型，
	// 但此处仅用于构建响应体，不涉及 PagedJSON 的 int64 参数，保持原逻辑。
	_ = total
	httputil.JSON(w, http.StatusOK, map[string]any{"reports": reports})
}

func (h *Handler) createReport(w http.ResponseWriter, r *http.Request) {
	var rpt biz.Report
	if err := json.NewDecoder(r.Body).Decode(&rpt); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if err := h.reportUC.Create(r.Context(), &rpt); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, rpt)
}

func (h *Handler) getReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "report_id")
	rpt, err := h.reportUC.Get(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, rpt)
}

func (h *Handler) deleteReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "report_id")
	if err := h.reportUC.Delete(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) runReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "report_id")
	if err := h.reportUC.Run(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// --- SLA ---

func (h *Handler) listSLAConfigs(w http.ResponseWriter, r *http.Request) {
	p := httputil.ParsePagination(r)
	filter := biz.SLAListFilter{Page: p.Page, PageSize: p.PageSize}
	if v := r.URL.Query().Get("dimension"); v != "" {
		filter.Dimension = &v
	}

	configs, total, err := h.slaUC.ListConfigs(r.Context(), filter)
	if err != nil {
		h.handleError(w, err)
		return
	}
	// BUG-007 修复：SLAUsecase.ListConfigs 返回 int 类型的 total，
	// 而 httputil.PagedJSON 的最后一个参数要求 int64，显式转换以消除类型不匹配编译错误。
	httputil.PagedJSON(w, configs, p.Page, p.PageSize, int64(total))
}

func (h *Handler) createSLAConfig(w http.ResponseWriter, r *http.Request) {
	var cfg biz.SLAConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if err := h.slaUC.CreateConfig(r.Context(), &cfg); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, cfg)
}

func (h *Handler) getSLAConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "config_id")
	cfg, err := h.slaUC.GetConfig(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, cfg)
}

func (h *Handler) updateSLAConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "config_id")
	var cfg biz.SLAConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	cfg.ConfigID = id
	if err := h.slaUC.UpdateConfig(r.Context(), &cfg); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, cfg)
}

func (h *Handler) deleteSLAConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "config_id")
	if err := h.slaUC.DeleteConfig(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) calculateSLA(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConfigID string    `json:"config_id"`
		Start    time.Time `json:"start"`
		End      time.Time `json:"end"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if req.ConfigID == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "config_id is required"))
		return
	}

	result, err := h.slaUC.Calculate(r.Context(), req.ConfigID, req.Start, req.End)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, result)
}

// slaReport 处理 SLA 报告生成请求，按指定时间粒度（daily/weekly/monthly）聚合 SLA 数据。
// 若未指定 granularity 则默认按月聚合。
func (h *Handler) slaReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConfigID    string    `json:"config_id"`   // SLA 配置 ID
		Start       time.Time `json:"start"`       // 报告起始时间
		End         time.Time `json:"end"`         // 报告结束时间
		Granularity string    `json:"granularity"` // 时间粒度：daily / weekly / monthly
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if req.ConfigID == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "config_id is required"))
		return
	}
	if req.Granularity == "" {
		req.Granularity = "monthly"
	}

	report, err := h.slaUC.CalculateReport(r.Context(), req.ConfigID, req.Start, req.End, req.Granularity)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, report)
}

// exportReport 导出指定报表的数据，支持 CSV 和 JSON 两种格式。
// 默认导出 CSV 格式，通过 ?format=json 可切换为 JSON 格式。
// 导出时会重新执行报表查询以获取最新数据。
func (h *Handler) exportReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "report_id")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}

	// 获取报表配置
	rpt, err := h.reportUC.Get(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// 重新执行报表查询获取最新数据
	queryResult, err := h.metricsUC.ExecuteQuery(r.Context(), biz.QueryRequest{
		Query: rpt.Query,
		Limit: 10000,
	})
	if err != nil {
		h.handleError(w, err)
		return
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", rpt.Name))
		writeCSV(w, queryResult)
	case "json":
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.json", rpt.Name))
		httputil.JSON(w, http.StatusOK, queryResult)
	default:
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "unsupported format: "+format+", supported: csv, json"))
	}
}

// writeCSV 将查询结果以 CSV 格式写入 HTTP 响应。
// 先输出列名作为表头，再逐行输出数据，使用 CRLF 换行符。
func writeCSV(w http.ResponseWriter, result *biz.QueryResponse) {
	// 写入 CSV 表头（列名）
	for i, col := range result.Columns {
		if i > 0 {
			w.Write([]byte(","))
		}
		w.Write([]byte(csvEscape(col.Name)))
	}
	w.Write([]byte("\r\n"))

	// 写入数据行
	for _, row := range result.Rows {
		for i, val := range row {
			if i > 0 {
				w.Write([]byte(","))
			}
			w.Write([]byte(csvEscape(fmt.Sprintf("%v", val))))
		}
		w.Write([]byte("\r\n"))
	}
}

// csvEscape 对 CSV 字段进行转义：当字段包含逗号、双引号或换行符时，用双引号包裹并转义内部双引号。
func csvEscape(s string) string {
	needsQuote := false
	for _, c := range s {
		if c == ',' || c == '"' || c == '\n' || c == '\r' {
			needsQuote = true
			break
		}
	}
	if !needsQuote {
		return s
	}
	// 将字段内的双引号替换为两个双引号，并用双引号包裹整个字段
	escaped := "\""
	for _, c := range s {
		if c == '"' {
			escaped += "\"\""
		} else {
			escaped += string(c)
		}
	}
	escaped += "\""
	return escaped
}

// --- Correlations ---

func (h *Handler) correlateMetrics(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AssetID string    `json:"asset_id"`
		MetricA string    `json:"metric_a"`
		MetricB string    `json:"metric_b"`
		Start   time.Time `json:"start"`
		End     time.Time `json:"end"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	corr, err := h.metricsUC.Correlate(r.Context(), req.AssetID, req.MetricA, req.MetricB, req.Start, req.End)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, corr)
}

// --- Knowledge ---

func (h *Handler) listKnowledge(w http.ResponseWriter, r *http.Request) {
	p := httputil.ParsePagination(r)
	articleType := r.URL.Query().Get("type")

	articles, total, err := h.knowledgeUC.List(r.Context(), articleType, p.Page, p.PageSize)
	if err != nil {
		h.handleError(w, err)
		return
	}
	// BUG-007 修复：KnowledgeUsecase.List 返回 int 类型的 total，
	// 而 httputil.PagedJSON 的最后一个参数要求 int64，显式转换以消除类型不匹配编译错误。
	httputil.PagedJSON(w, articles, p.Page, p.PageSize, int64(total))
}

func (h *Handler) createKnowledge(w http.ResponseWriter, r *http.Request) {
	var article biz.KnowledgeArticle
	if err := json.NewDecoder(r.Body).Decode(&article); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if err := h.knowledgeUC.Create(r.Context(), &article); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, article)
}

func (h *Handler) getKnowledge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "article_id")
	article, err := h.knowledgeUC.Get(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, article)
}

func (h *Handler) updateKnowledge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "article_id")
	var article biz.KnowledgeArticle
	if err := json.NewDecoder(r.Body).Decode(&article); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	article.ArticleID = id
	if err := h.knowledgeUC.Update(r.Context(), &article); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, article)
}

func (h *Handler) deleteKnowledge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "article_id")
	if err := h.knowledgeUC.Delete(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) searchKnowledge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if req.Query == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "query is required"))
		return
	}

	results, err := h.knowledgeUC.Search(r.Context(), req.Query, req.Limit)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"results": results})
}

// --- API Token ---

// createToken 处理创建新 API Token 的请求。
// 请求体包含 name、permissions 和可选的 expires_in（秒数）。
// 返回的响应中包含明文令牌，该令牌仅在此次响应中可见。
func (h *Handler) createToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
		ExpiresIn   *int64   `json:"expires_in"` // 有效期，单位：秒，nil 表示永不过期
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if req.Name == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "name is required"))
		return
	}
	if len(req.Permissions) == 0 {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "at least one permission is required"))
		return
	}

	// 将秒数转换为 time.Duration
	var expiresIn *time.Duration
	if req.ExpiresIn != nil {
		d := time.Duration(*req.ExpiresIn) * time.Second
		expiresIn = &d
	}

	// 从 context 中获取当前用户（简化实现：取 X-User-ID 请求头）
	createdBy := r.Header.Get("X-User-ID")
	if createdBy == "" {
		createdBy = "anonymous"
	}

	result, err := h.tokenUC.Generate(r.Context(), req.Name, req.Permissions, expiresIn, createdBy)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, result)
}

// listTokens 列出当前用户创建的所有 API Token（不返回明文令牌）。
func (h *Handler) listTokens(w http.ResponseWriter, r *http.Request) {
	createdBy := r.Header.Get("X-User-ID")
	if createdBy == "" {
		createdBy = "anonymous"
	}

	tokens, err := h.tokenUC.List(r.Context(), createdBy)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

// revokeToken 吊销指定 ID 的 API Token。
func (h *Handler) revokeToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "token id is required"))
		return
	}

	operatorID := r.Header.Get("X-User-ID")
	if operatorID == "" {
		operatorID = "anonymous"
	}

	if err := h.tokenUC.Revoke(r.Context(), id, operatorID); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Suggestions（用户建议管理） ---

// submitSuggestion 处理提交新建议的请求（FR-16-001）。
func (h *Handler) submitSuggestion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if req.Title == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "title is required"))
		return
	}

	// 从请求头获取当前用户 ID
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "anonymous"
	}

	suggestion, err := h.suggestionUC.Submit(r.Context(), req.Title, req.Description, userID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, suggestion)
}

// listSuggestions 处理查询建议列表的请求，支持按状态筛选（FR-16-001）。
func (h *Handler) listSuggestions(w http.ResponseWriter, r *http.Request) {
	p := httputil.ParsePagination(r)
	status := r.URL.Query().Get("status")

	suggestions, total, err := h.suggestionUC.List(r.Context(), status, p.Page, p.PageSize)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.PagedJSON(w, suggestions, p.Page, p.PageSize, int64(total))
}

// updateSuggestionStatus 处理更新建议状态的请求（FR-16-001）。
func (h *Handler) updateSuggestionStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Status    string `json:"status"`
		AdminNote string `json:"admin_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if req.Status == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "status is required"))
		return
	}

	if err := h.suggestionUC.UpdateStatus(r.Context(), id, req.Status, req.AdminNote); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"message": "status updated"})
}

// getSuggestionStats 处理获取建议统计数据的请求（FR-16-006）。
func (h *Handler) getSuggestionStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.suggestionUC.GetStats(r.Context())
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, stats)
}

// --- Report Templates（报告模板） ---

// listReportTemplates 列出所有预置报告模板。
func (h *Handler) listReportTemplates(w http.ResponseWriter, r *http.Request) {
	templates := h.reportTemplateUC.ListTemplates()
	httputil.JSON(w, http.StatusOK, map[string]any{"templates": templates})
}

// generateReport 根据模板生成报告（FR-15-009~011, FR-15-013）。
func (h *Handler) generateReport(w http.ResponseWriter, r *http.Request) {
	var req biz.GenerateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if req.TemplateID == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "template_id is required"))
		return
	}

	var result interface{}
	var err error

	switch req.TemplateID {
	case biz.TemplateMonthlyOps:
		// 月度运营报告需要年月参数
		year := req.StartDate.Year()
		month := int(req.StartDate.Month())
		result, err = h.reportTemplateUC.GenerateMonthlyOpsReport(r.Context(), year, month)
	case biz.TemplateSLAReport:
		result, err = h.reportTemplateUC.GenerateSLAReport(r.Context(), req.StartDate, req.EndDate)
	case biz.TemplateIncidentTrend:
		result, err = h.reportTemplateUC.GenerateIncidentTrendReport(r.Context(), req.StartDate, req.EndDate)
	case biz.TemplateAlertQuality:
		result, err = h.reportTemplateUC.GenerateAlertQualityReport(r.Context(), req.StartDate, req.EndDate)
	default:
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "unknown template_id: "+req.TemplateID))
		return
	}

	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, result)
}

// getGeneratedReport 获取已生成的报告。
func (h *Handler) getGeneratedReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	report, err := h.reportTemplateUC.GetGeneratedReport(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, report)
}

// exportGeneratedReport 导出已生成的报告，支持 pdf/csv/json 格式。
func (h *Handler) exportGeneratedReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	report, err := h.reportTemplateUC.GetGeneratedReport(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	switch format {
	case "json":
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.json", report.Title))
		httputil.JSON(w, http.StatusOK, report)
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", report.Title))
		// CSV 导出简化处理：输出报告元数据
		w.Write([]byte(fmt.Sprintf("id,template_id,title,generated_at\r\n%s,%s,%s,%s\r\n",
			report.ID, report.TemplateID, report.Title, report.GeneratedAt.Format(time.RFC3339))))
	case "pdf":
		// PDF 导出占位：实际生产需集成 PDF 渲染库
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pdf", report.Title))
		w.Write([]byte("PDF export not yet implemented"))
	default:
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "unsupported format: "+format+", supported: pdf, csv, json"))
	}
}

// --- Data Export（数据导出） ---

// createExportTask 创建异步数据导出任务（FR-29-001）。
func (h *Handler) createExportTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope  []string `json:"scope"`
		Format string   `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "anonymous"
	}

	task, err := h.dataExportUC.CreateTask(r.Context(), req.Scope, req.Format, userID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, task)
}

// getExportTask 查询导出任务状态和下载链接（FR-29-001）。
func (h *Handler) getExportTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := h.dataExportUC.GetTask(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, task)
}

// --- Data Import（数据导入） ---

// createImportTask 创建异步数据导入任务（FR-29-002）。
func (h *Handler) createImportTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DataType string `json:"data_type"`
		Format   string `json:"format"`
		FileURL  string `json:"file_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	task, err := h.dataImportUC.CreateImportTask(r.Context(), req.DataType, req.Format, req.FileURL)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, task)
}

// getImportTask 查询导入任务进度（FR-29-002）。
func (h *Handler) getImportTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := h.dataImportUC.GetImportTask(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, task)
}

// --- Diagnostics（诊断包收集） ---

// collectDiagnostics 触发收集诊断包（FR-29-004）。
func (h *Handler) collectDiagnostics(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	bundle, err := h.diagnosticsUC.CollectDiagnostics(r.Context(), req.Summary)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, bundle)
}

// getDiagnosticsDownload 获取诊断包下载链接（FR-29-004）。
func (h *Handler) getDiagnosticsDownload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	url, err := h.diagnosticsUC.GetDiagnosticsDownloadURL(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]string{"download_url": url})
}

// handleError 将业务错误（AppError）映射为对应的 HTTP 错误响应。
func (h *Handler) handleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		httputil.Error(w, appErr)
		return
	}
	httputil.Error(w, apperrors.Internal("INTERNAL", err.Error()))
}
