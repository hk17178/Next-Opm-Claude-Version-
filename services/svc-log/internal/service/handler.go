// Package service 提供日志服务的传输层实现，包括 gRPC 服务端、HTTP 路由处理和 Kafka 消费者。
package service

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/opsnexus/svc-log/internal/biz"
	"go.uber.org/zap"
)

// Handler 提供日志服务的 HTTP 请求处理器，包含摄入、搜索、导出等端点。
type Handler struct {
	ingestSvc *biz.IngestService
	searchSvc *biz.SearchService
	logger    *zap.Logger
}

// NewHandler 创建一个新的 HTTP Handler 实例。
func NewHandler(ingestSvc *biz.IngestService, searchSvc *biz.SearchService, logger *zap.Logger) *Handler {
	return &Handler{
		ingestSvc: ingestSvc,
		searchSvc: searchSvc,
		logger:    logger,
	}
}

// NewRouter 创建 chi 路由并注册所有 HTTP 路由。
// 路由与 ON-003 API 表和 OpenAPI svc-log.yaml 对齐。
// API 网关会剥离服务前缀，内部挂载在 /api/v1/log 下。
func NewRouter(h *Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	r.Get("/healthz", h.Healthz)

	// OpenAPI base path: /api/v1/log
	r.Route("/api/v1/log", func(r chi.Router) {
		// Log ingestion (ON-003: POST /api/v1/logs/ingest)
		r.Post("/ingest", h.IngestLogs)
		r.Post("/ingest/webhook", h.IngestLogs)

		// Log search (ON-003: POST /api/v1/logs/search)
		r.Post("/search", h.SearchLogs)

		// Streams CRUD (OpenAPI: /streams)
		r.Route("/streams", func(r chi.Router) {
			r.Get("/", h.ListStreams)
			r.Post("/", h.CreateStream)
			r.Get("/{stream_id}", h.GetStream)
			r.Delete("/{stream_id}", h.DeleteStream)
		})
	})

	// Additional ON-003 endpoints under /api/v1/logs (kept for compatibility)
	r.Route("/api/v1/logs", func(r chi.Router) {
		r.Post("/ingest", h.IngestLogs)
		r.Post("/ingest/webhook", h.IngestLogs)
		r.Post("/search", h.SearchLogs)
		r.Get("/{id}/context", h.LogContext)
		// POST /export：请求体携带完整查询条件（JSON Body 方式，向后兼容）
		r.Post("/export", h.ExportLogs)
		// GET /export：通过 URL 查询参数指定格式，适合浏览器直接下载场景
		// 支持参数：format=csv|json，query，start_time，end_time
		r.Get("/export", h.ExportLogsGET)
		r.Get("/stats", h.StatsLogs)
	})

	// CRUD: log-sources, parse-rules, masking-rules, retention-policies (ON-003)
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/log-sources", func(r chi.Router) {
			r.Get("/", h.ListLogSources)
			r.Post("/", h.CreateLogSource)
			r.Get("/{id}", h.GetLogSource)
			r.Put("/{id}", h.UpdateLogSource)
			r.Delete("/{id}", h.DeleteLogSource)
		})
		r.Route("/parse-rules", func(r chi.Router) {
			r.Get("/", h.ListParseRules)
			r.Post("/", h.CreateParseRule)
			r.Get("/{id}", h.GetParseRule)
			r.Put("/{id}", h.UpdateParseRule)
			r.Delete("/{id}", h.DeleteParseRule)
		})
		r.Route("/masking-rules", func(r chi.Router) {
			r.Get("/", h.ListMaskingRules)
			r.Post("/", h.CreateMaskingRule)
			r.Get("/{id}", h.GetMaskingRule)
			r.Put("/{id}", h.UpdateMaskingRule)
			r.Delete("/{id}", h.DeleteMaskingRule)
		})
		r.Route("/retention-policies", func(r chi.Router) {
			r.Get("/", h.ListRetentionPolicies)
			r.Post("/", h.CreateRetentionPolicy)
			r.Get("/{id}", h.GetRetentionPolicy)
			r.Put("/{id}", h.UpdateRetentionPolicy)
			r.Delete("/{id}", h.DeleteRetentionPolicy)
		})
	})

	return r
}

// Healthz 返回服务健康状态，供负载均衡器和 Kubernetes 探针使用。
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "svc-log"})
}

// --- 日志摄入 ---

// IngestLogs 处理 POST /ingest 请求，接收批量日志条目。
func (h *Handler) IngestLogs(w http.ResponseWriter, r *http.Request) {
	var req biz.LogIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid JSON body, expected {entries: [...]}"})
		return
	}

	resp, err := h.ingestSvc.IngestHTTP(r.Context(), req)
	if err != nil {
		h.logger.Error("ingest failed", zap.Error(err))
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "ingest failed"})
		return
	}

	writeJSON(w, http.StatusAccepted, resp)
}

// --- 日志搜索 ---

// SearchLogs 处理 POST /search 请求，执行全文日志搜索。
func (h *Handler) SearchLogs(w http.ResponseWriter, r *http.Request) {
	var req biz.LogSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid query body"})
		return
	}

	result, err := h.searchSvc.Search(r.Context(), req)
	if err != nil {
		h.logger.Error("search failed", zap.Error(err))
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "search failed"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// --- 日志上下文 ---

// LogContext 处理 GET /{id}/context 请求，查询指定日志条目的前后上下文。
func (h *Handler) LogContext(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	before := 20
	after := 20

	if bs := r.URL.Query().Get("before"); bs != "" {
		if parsed, err := strconv.Atoi(bs); err == nil {
			before = parsed
		}
	}
	if as := r.URL.Query().Get("after"); as != "" {
		if parsed, err := strconv.Atoi(as); err == nil {
			after = parsed
		}
	}

	result, err := h.searchSvc.Context(r.Context(), id, before, after)
	if err != nil {
		h.logger.Error("context failed", zap.Error(err))
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "context query failed"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// --- 日志导出 ---

// ExportLogs 处理 POST /export 请求，支持同步流式导出（CSV/JSON 格式）。
// 查询日志后逐条写入响应流，适合大数据量导出场景。
func (h *Handler) ExportLogs(w http.ResponseWriter, r *http.Request) {
	var req biz.LogExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid export request"})
		return
	}

	if req.Format == "" {
		req.Format = "json"
	}

	switch req.Format {
	case "csv":
		h.exportCSV(w, r, req)
	case "json":
		h.exportJSON(w, r, req)
	default:
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "unsupported format, use csv or json"})
	}
}

// exportCSV 以 CSV 格式流式导出日志。
func (h *Handler) exportCSV(w http.ResponseWriter, r *http.Request, req biz.LogExportRequest) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=logs_export.csv")
	w.WriteHeader(http.StatusOK)

	csvWriter := csv.NewWriter(w)
	// 写入表头
	csvWriter.Write([]string{"id", "timestamp", "level", "message", "service_name", "host_id", "source_type", "source_host", "source_service", "trace_id", "span_id"})
	csvWriter.Flush()

	err := h.searchSvc.ExportStream(r.Context(), req, func(entry biz.LogEntry) error {
		record := []string{
			entry.ID,
			entry.Timestamp.Format("2006-01-02T15:04:05Z"),
			string(entry.Level),
			entry.Message,
			entry.ServiceName,
			entry.HostID,
			entry.SourceType,
			entry.SourceHost,
			entry.SourceService,
			entry.TraceID,
			entry.SpanID,
		}
		if err := csvWriter.Write(record); err != nil {
			return fmt.Errorf("csv write: %w", err)
		}
		csvWriter.Flush()
		return csvWriter.Error()
	})

	if err != nil {
		h.logger.Error("csv export failed", zap.Error(err))
	}
}

// exportJSON 以 JSON Lines 格式流式导出日志（每行一条 JSON 记录）。
func (h *Handler) exportJSON(w http.ResponseWriter, r *http.Request, req biz.LogExportRequest) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", "attachment; filename=logs_export.jsonl")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	err := h.searchSvc.ExportStream(r.Context(), req, func(entry biz.LogEntry) error {
		return encoder.Encode(entry)
	})

	if err != nil {
		h.logger.Error("json export failed", zap.Error(err))
	}
}

// ExportLogsGET 处理 GET /export 请求，支持通过 URL 查询参数触发日志导出。
// 适合浏览器直接访问下载（无需设置 Content-Type: application/json 请求体）。
//
// 支持的查询参数：
//   - format=csv|json（默认 json）：导出格式
//   - query：全文搜索表达式（如 "level:ERROR"）
//   - start_time：时间范围开始（RFC3339 格式，如 2024-01-01T00:00:00Z）
//   - end_time：时间范围结束（RFC3339 格式）
//
// 响应：
//   - CSV 格式：Content-Type: text/csv，Content-Disposition: attachment; filename=logs_export.csv
//   - JSON 格式：Content-Type: application/x-ndjson，流式 JSON Lines 格式
func (h *Handler) ExportLogsGET(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// 解析导出格式，默认为 json
	format := q.Get("format")
	if format == "" {
		format = "json"
	}

	// 校验格式参数，仅支持 csv 和 json
	if format != "csv" && format != "json" {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{
			Error: fmt.Sprintf("unsupported format %q, use csv or json", format),
		})
		return
	}

	// 构造导出请求，从查询参数中提取搜索条件
	req := biz.LogExportRequest{
		Query:  q.Get("query"),
		Format: format,
	}

	// 如果 query 为空，使用通配符匹配所有日志
	if req.Query == "" {
		req.Query = "*"
	}

	// 解析可选的时间范围参数
	startStr := q.Get("start_time")
	endStr := q.Get("end_time")
	if startStr != "" || endStr != "" {
		tr := &biz.TimeRange{}
		if startStr != "" {
			// 解析 RFC3339 格式的开始时间
			if t, err := parseRFC3339(startStr); err == nil {
				tr.Start = &t
			} else {
				writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{
					Error: fmt.Sprintf("invalid start_time %q, use RFC3339 format", startStr),
				})
				return
			}
		}
		if endStr != "" {
			// 解析 RFC3339 格式的结束时间
			if t, err := parseRFC3339(endStr); err == nil {
				tr.End = &t
			} else {
				writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{
					Error: fmt.Sprintf("invalid end_time %q, use RFC3339 format", endStr),
				})
				return
			}
		}
		req.TimeRange = tr
	}

	// 根据格式分发到对应的流式输出处理器
	switch req.Format {
	case "csv":
		h.exportCSV(w, r, req)
	case "json":
		h.exportJSON(w, r, req)
	}
}

// parseRFC3339 解析 RFC3339 格式的时间字符串，返回 time.Time 和可能的错误。
// 支持带时区偏移（如 2024-01-01T00:00:00+08:00）和 UTC（如 2024-01-01T00:00:00Z）两种格式。
func parseRFC3339(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// --- 日志统计 ---

// StatsLogs 处理 GET /stats 请求，返回日志聚合统计数据。
func (h *Handler) StatsLogs(w http.ResponseWriter, r *http.Request) {
	var req biz.LogStatsRequest
	req.GroupBy = r.URL.Query().Get("group_by")
	req.Interval = r.URL.Query().Get("interval")

	result, err := h.searchSvc.Stats(r.Context(), req)
	if err != nil {
		h.logger.Error("stats failed", zap.Error(err))
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "stats failed"})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// --- 日志流 CRUD（OpenAPI） ---

func (h *Handler) ListStreams(w http.ResponseWriter, r *http.Request) {
	pageSize := 20
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil {
			pageSize = parsed
		}
	}
	pageToken := r.URL.Query().Get("page_token")

	streams, nextToken, err := h.ingestSvc.ListStreams(r.Context(), pageSize, pageToken)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to list streams"})
		return
	}
	writeJSON(w, http.StatusOK, biz.StreamListResponse{
		Streams:       streams,
		NextPageToken: nextToken,
	})
}

func (h *Handler) CreateStream(w http.ResponseWriter, r *http.Request) {
	var req biz.StreamCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid stream body"})
		return
	}

	stream := &biz.Stream{
		Name:          req.Name,
		Filter:        req.Filter,
		RetentionDays: req.RetentionDays,
	}
	if err := h.ingestSvc.CreateStream(r.Context(), stream); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to create stream"})
		return
	}
	writeJSON(w, http.StatusCreated, stream)
}

func (h *Handler) GetStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "stream_id")
	stream, err := h.ingestSvc.GetStream(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, biz.ErrorResponse{Error: "stream not found"})
		return
	}
	writeJSON(w, http.StatusOK, stream)
}

func (h *Handler) DeleteStream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "stream_id")
	if err := h.ingestSvc.DeleteStream(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to delete stream"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- 日志采集源 CRUD（ON-003） ---

func (h *Handler) ListLogSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.ingestSvc.ListLogSources(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to list log sources"})
		return
	}
	writeJSON(w, http.StatusOK, sources)
}

func (h *Handler) CreateLogSource(w http.ResponseWriter, r *http.Request) {
	var src biz.LogSource
	if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid log source body"})
		return
	}
	if err := h.ingestSvc.CreateLogSource(r.Context(), &src); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to create log source"})
		return
	}
	writeJSON(w, http.StatusCreated, src)
}

func (h *Handler) GetLogSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	src, err := h.ingestSvc.GetLogSource(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, biz.ErrorResponse{Error: "log source not found"})
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (h *Handler) UpdateLogSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var src biz.LogSource
	if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid log source body"})
		return
	}
	src.SourceID = id
	if err := h.ingestSvc.UpdateLogSource(r.Context(), &src); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to update log source"})
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (h *Handler) DeleteLogSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.ingestSvc.DeleteLogSource(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to delete log source"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- 解析规则 CRUD（ON-003） ---

func (h *Handler) ListParseRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.ingestSvc.ListParseRules(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to list parse rules"})
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handler) CreateParseRule(w http.ResponseWriter, r *http.Request) {
	var rule biz.ParseRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid parse rule body"})
		return
	}
	if err := h.ingestSvc.CreateParseRule(r.Context(), &rule); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to create parse rule"})
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) GetParseRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rule, err := h.ingestSvc.GetParseRule(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, biz.ErrorResponse{Error: "parse rule not found"})
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handler) UpdateParseRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var rule biz.ParseRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid parse rule body"})
		return
	}
	rule.RuleID = id
	if err := h.ingestSvc.UpdateParseRule(r.Context(), &rule); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to update parse rule"})
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handler) DeleteParseRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.ingestSvc.DeleteParseRule(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to delete parse rule"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- 脱敏规则 CRUD（ON-003） ---

func (h *Handler) ListMaskingRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.ingestSvc.ListMaskingRules(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to list masking rules"})
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handler) CreateMaskingRule(w http.ResponseWriter, r *http.Request) {
	var rule biz.MaskingRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid masking rule body"})
		return
	}
	if err := h.ingestSvc.CreateMaskingRule(r.Context(), &rule); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to create masking rule"})
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) GetMaskingRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rule, err := h.ingestSvc.GetMaskingRule(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, biz.ErrorResponse{Error: "masking rule not found"})
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handler) UpdateMaskingRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var rule biz.MaskingRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid masking rule body"})
		return
	}
	rule.RuleID = id
	if err := h.ingestSvc.UpdateMaskingRule(r.Context(), &rule); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to update masking rule"})
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handler) DeleteMaskingRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.ingestSvc.DeleteMaskingRule(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to delete masking rule"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- 保留策略 CRUD（ON-003） ---

func (h *Handler) ListRetentionPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := h.ingestSvc.ListRetentionPolicies(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to list retention policies"})
		return
	}
	writeJSON(w, http.StatusOK, policies)
}

func (h *Handler) CreateRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	var policy biz.RetentionPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid retention policy body"})
		return
	}
	if err := h.ingestSvc.CreateRetentionPolicy(r.Context(), &policy); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to create retention policy"})
		return
	}
	writeJSON(w, http.StatusCreated, policy)
}

func (h *Handler) GetRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	policy, err := h.ingestSvc.GetRetentionPolicy(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, biz.ErrorResponse{Error: "retention policy not found"})
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (h *Handler) UpdateRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var policy biz.RetentionPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		writeJSON(w, http.StatusBadRequest, biz.ErrorResponse{Error: "invalid retention policy body"})
		return
	}
	policy.PolicyID = id
	if err := h.ingestSvc.UpdateRetentionPolicy(r.Context(), &policy); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to update retention policy"})
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (h *Handler) DeleteRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.ingestSvc.DeleteRetentionPolicy(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, biz.ErrorResponse{Error: "failed to delete retention policy"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeJSON 将数据序列化为 JSON 并写入 HTTP 响应。
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}
