package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AnalysisUseCase 编排完整的 AI 分析流水线，包含以下步骤：
//  1. 从多数据源收集上下文（告警、日志、变更、拓扑等）
//  2. 对敏感数据进行脱敏处理（PII/密钥等）
//  3. 压缩上下文以适应 token 预算（2000-5000）
//  4. 通过场景绑定（SceneBinding）和路由策略选择模型
//  5. 经熔断器保护后调用 AI 模型
//  6. 解析模型输出并存储结构化结果
//  7. 发送 analysis.done 事件通知下游
//
// 整个分析过程是异步执行的：CreateAnalysis 立即返回任务 ID，
// runAnalysis 在后台 goroutine 中完成实际的模型调用。
// 当主模型调用失败时，ModelManager 会自动 fallback 到备用模型。
type AnalysisUseCase struct {
	repo             AnalysisRepo
	modelManager     *ModelManager
	contextCollector *ContextCollector
	circuitBreaker   *CircuitBreaker
	promptRepo       PromptRepo
	callLogRepo      CallLogRepo
	logger           *zap.Logger
}

// NewAnalysisUseCase 创建分析用例实例，注入所有依赖组件。
func NewAnalysisUseCase(
	repo AnalysisRepo,
	modelManager *ModelManager,
	contextCollector *ContextCollector,
	circuitBreaker *CircuitBreaker,
	promptRepo PromptRepo,
	callLogRepo CallLogRepo,
	logger *zap.Logger,
) *AnalysisUseCase {
	return &AnalysisUseCase{
		repo:             repo,
		modelManager:     modelManager,
		contextCollector: contextCollector,
		circuitBreaker:   circuitBreaker,
		promptRepo:       promptRepo,
		callLogRepo:      callLogRepo,
		logger:           logger,
	}
}

// CreateAnalysis 创建新的分析任务并立即返回（异步执行）。
// 任务创建后会启动后台 goroutine 执行实际的 AI 分析流程。
func (uc *AnalysisUseCase) CreateAnalysis(ctx context.Context, req AnalysisCreateRequest) (*AnalysisTask, error) {
	task := &AnalysisTask{
		ID:        uuid.New(),
		Type:      req.Type,
		Status:    StatusPending,
		AlertIDs:  req.AlertIDs,
		TimeRange: req.TimeRange,
		Context:   req.Context,
		CreatedAt: time.Now(),
	}
	if req.IncidentID != nil {
		task.IncidentID = req.IncidentID
	}

	if err := uc.repo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("create analysis task: %w", err)
	}

	// Start async analysis
	go uc.runAnalysis(context.Background(), task)

	return task, nil
}

// AnalysisCreateRequest 对应 API 规范中的分析创建请求参数。
type AnalysisCreateRequest struct {
	Type       AnalysisType    `json:"type"`
	IncidentID *uuid.UUID      `json:"incident_id,omitempty"`
	AlertIDs   []uuid.UUID     `json:"alert_ids,omitempty"`
	TimeRange  *TimeRange      `json:"time_range,omitempty"`
	Context    json.RawMessage `json:"context,omitempty"`
}

// runAnalysis 在后台执行完整的分析流水线。
// 流程：熔断检查 → 场景映射 → 获取 prompt → 构建上下文 → 脱敏压缩 → 调用模型 → 解析结果 → 存储。
// 任何步骤失败都会将任务标记为 StatusFailed 并记录错误原因。
func (uc *AnalysisUseCase) runAnalysis(ctx context.Context, task *AnalysisTask) {
	start := time.Now()
	uc.logger.Info("starting analysis", zap.String("id", task.ID.String()), zap.String("type", string(task.Type)))

	// Update status to running
	_ = uc.repo.UpdateStatus(ctx, task.ID, StatusRunning, nil, "")

	// Step 1: Check circuit breaker
	if !uc.circuitBreaker.Allow() {
		uc.logger.Warn("circuit breaker open, skipping AI analysis", zap.String("id", task.ID.String()))
		_ = uc.repo.UpdateStatus(ctx, task.ID, StatusFailed, nil, "circuit breaker open: AI service degraded")
		return
	}

	// Step 2: Map analysis type to scene
	scene := uc.mapTypeToScene(task.Type)

	// Step 3: Get the active prompt for this scene
	prompt, err := uc.promptRepo.GetActive(ctx, scene)
	if err != nil {
		uc.logger.Error("no active prompt for scene", zap.String("scene", scene), zap.Error(err))
		_ = uc.repo.UpdateStatus(ctx, task.ID, StatusFailed, nil, "no active prompt for scene: "+scene)
		return
	}

	// Step 4: Build raw context from task data
	rawCtx := uc.buildRawContext(task)

	// Step 5: Collect, desensitize, and compress context
	analysisCtx, _ := uc.contextCollector.CollectAndCompress(rawCtx)

	// Step 6: Render prompt template
	vars := uc.buildPromptVars(task, analysisCtx)
	systemPrompt := uc.contextCollector.BuildPrompt(prompt.SystemPrompt, vars)
	userPrompt := uc.contextCollector.BuildPrompt(prompt.UserPrompt, vars)

	// Step 7: Call the model
	callReq := ChatRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}

	callResult, err := uc.modelManager.CallModel(ctx, scene, callReq)
	if err != nil {
		uc.circuitBreaker.RecordFailure()
		uc.logger.Error("model call failed", zap.String("id", task.ID.String()), zap.Error(err))
		_ = uc.repo.UpdateStatus(ctx, task.ID, StatusFailed, nil, err.Error())

		// Log the failed call
		uc.logCall(ctx, task.ID, nil, scene, prompt.Version, 0, 0, int(time.Since(start).Milliseconds()), "error", err.Error())
		return
	}

	uc.circuitBreaker.RecordSuccess()

	// Step 8: Parse the model output
	result, parseErr := uc.parseAnalysisResult(callResult.Response.Content, task.Type)
	status := StatusSuccess
	errMsg := ""
	if parseErr != nil {
		status = StatusPartial
		errMsg = parseErr.Error()
		uc.logger.Warn("partial parse of AI output", zap.String("id", task.ID.String()), zap.Error(parseErr))
	}

	// Step 9: Store result
	_ = uc.repo.UpdateStatus(ctx, task.ID, status, result, errMsg)

	// Step 10: Log the call
	uc.logCall(ctx, task.ID, &callResult.ModelID, scene, prompt.Version,
		callResult.Response.InputTokens, callResult.Response.OutputTokens,
		callResult.LatencyMs, string(status), errMsg)

	uc.logger.Info("analysis completed",
		zap.String("id", task.ID.String()),
		zap.String("status", string(status)),
		zap.Int("latency_ms", callResult.LatencyMs),
		zap.String("model", callResult.ModelName),
	)
}

// mapTypeToScene 将分析类型映射到 prompt 场景名称。
// 多个分析类型可能共享同一个场景（如根因分析、异常检测和关联分析都使用 alert_analysis）。
func (uc *AnalysisUseCase) mapTypeToScene(t AnalysisType) string {
	switch t {
	case AnalysisRootCause:
		return "alert_analysis"
	case AnalysisLogSummary:
		return "log_summary"
	case AnalysisAnomalyDetect:
		return "alert_analysis"
	case AnalysisPrediction:
		return "trend_insight"
	case AnalysisCorrelation:
		return "alert_analysis"
	default:
		return "alert_analysis"
	}
}

// buildRawContext 从任务的 Context JSON 字段中提取原始上下文数据。
func (uc *AnalysisUseCase) buildRawContext(task *AnalysisTask) RawContext {
	raw := RawContext{}

	// Extract context fields from the task's context JSON
	if task.Context != nil {
		var ctxMap map[string]interface{}
		if err := json.Unmarshal(task.Context, &ctxMap); err == nil {
			if v, ok := ctxMap["alert_info"].(string); ok {
				raw.AlertInfo = v
			}
			if v, ok := ctxMap["system_logs"].(string); ok {
				raw.SystemLogs = v
			}
			if v, ok := ctxMap["operation_logs"].(string); ok {
				raw.OperationLogs = v
			}
			if v, ok := ctxMap["change_records"].(string); ok {
				raw.ChangeRecords = v
			}
			if v, ok := ctxMap["topology_info"].(string); ok {
				raw.TopologyInfo = v
			}
			if v, ok := ctxMap["similar_alerts"].(string); ok {
				raw.SimilarAlerts = v
			}
		}
	}

	return raw
}

// buildPromptVars 构建 prompt 模板的变量替换映射表。
func (uc *AnalysisUseCase) buildPromptVars(task *AnalysisTask, ctx *AnalysisContext) map[string]string {
	vars := map[string]string{
		"alert_info":               ctx.AlertInfo,
		"system_context":           ctx.SystemContext,
		"human_context":            ctx.HumanContext,
		"change_context":           ctx.ChangeContext,
		"topology_context":         ctx.TopologyInfo,
		"similar_alerts":           ctx.SimilarAlerts,
		"context_window_minutes":   "30",
		"operation_window_minutes": "30",
		"change_window_hours":      "24",
	}

	if task.IncidentID != nil {
		vars["incident_id"] = task.IncidentID.String()
	}

	return vars
}

// parseAnalysisResult 解析 AI 模型的文本输出为结构化的 AnalysisResult。
// 如果模型返回的不是有效 JSON，则将原始文本作为摘要并标记为低置信度（0.3）。
func (uc *AnalysisUseCase) parseAnalysisResult(content string, analysisType AnalysisType) (*AnalysisResult, error) {
	result := &AnalysisResult{
		RawOutput: json.RawMessage(content),
	}

	// Try to parse as structured JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		// If not valid JSON, treat raw text as summary
		result.Summary = content
		result.Confidence = 0.3
		return result, fmt.Errorf("model output is not valid JSON: %w", err)
	}

	// Extract common fields
	if v, ok := parsed["root_cause"].(string); ok {
		result.Summary = v
	}
	if v, ok := parsed["summary"].(string); ok {
		result.Summary = v
	}
	if v, ok := parsed["confidence"].(float64); ok {
		result.Confidence = v
	}

	// Extract root causes
	if v, ok := parsed["root_cause_category"].(string); ok {
		rc := RootCause{
			Description: result.Summary,
			Category:    RootCauseCategory(v),
			Probability: result.Confidence,
		}
		if evidence, ok := parsed["evidence"].([]interface{}); ok {
			for _, e := range evidence {
				if s, ok := e.(string); ok {
					rc.Evidence = append(rc.Evidence, s)
				}
			}
		}
		result.RootCauses = []RootCause{rc}
	}

	// Extract recommendations
	if actions, ok := parsed["suggested_actions"].([]interface{}); ok {
		for _, a := range actions {
			if s, ok := a.(string); ok {
				result.Recommendations = append(result.Recommendations, s)
			}
		}
	}

	return result, nil
}

// logCall 记录一次 AI 模型调用日志，用于审计和成本追踪。
func (uc *AnalysisUseCase) logCall(ctx context.Context, analysisID uuid.UUID, modelID *uuid.UUID, scene, promptVersion string, inputTokens, outputTokens, latencyMs int, status, errMsg string) {
	log := &AICallLog{
		ID:            uuid.New(),
		AnalysisID:    &analysisID,
		ModelID:       modelID,
		Scene:         scene,
		PromptVersion: promptVersion,
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
		LatencyMs:     latencyMs,
		Status:        status,
		ErrorMessage:  errMsg,
		CreatedAt:     time.Now(),
	}
	if err := uc.callLogRepo.Create(ctx, log); err != nil {
		uc.logger.Warn("failed to log AI call", zap.Error(err))
	}
}

// GetAnalysis 根据 ID 查询分析任务详情。
func (uc *AnalysisUseCase) GetAnalysis(ctx context.Context, id uuid.UUID) (*AnalysisTask, error) {
	return uc.repo.GetByID(ctx, id)
}

// ListAnalyses 根据过滤条件分页查询分析任务列表。
func (uc *AnalysisUseCase) ListAnalyses(ctx context.Context, filter AnalysisFilter) ([]*AnalysisTask, string, error) {
	return uc.repo.List(ctx, filter)
}

// SubmitFeedback 记录用户对分析结果的反馈，用于后续模型效果评估和 prompt 优化。
func (uc *AnalysisUseCase) SubmitFeedback(ctx context.Context, analysisID uuid.UUID, req FeedbackRequest) error {
	uc.logger.Info("feedback received",
		zap.String("analysis_id", analysisID.String()),
		zap.Int("rating", req.Rating),
		zap.Bool("helpful", req.Helpful),
	)
	return uc.repo.SaveFeedback(ctx, analysisID, req)
}

// FeedbackRequest 表示用户反馈请求，包含评分、是否有用和可选的正确根因修正。
type FeedbackRequest struct {
	Rating         int    `json:"rating"`
	Helpful        bool   `json:"helpful"`
	Comment        string `json:"comment,omitempty"`
	CorrectRootCause string `json:"correct_root_cause,omitempty"`
}

// HandleAlertFired 处理 Kafka alert.fired 事件，自动触发根因分析任务。
func (uc *AnalysisUseCase) HandleAlertFired(ctx context.Context, event AlertFiredEvent) error {
	alertID, err := uuid.Parse(event.Data.AlertID)
	if err != nil {
		return fmt.Errorf("invalid alert_id: %w", err)
	}

	rawContext, _ := json.Marshal(map[string]interface{}{
		"alert_info": fmt.Sprintf("AlertID: %s\nTitle: %s\nSeverity: %s\nFiredAt: %s\nHost: %s\nService: %s",
			event.Data.AlertID, event.Data.Title, event.Data.Severity,
			event.Data.FiredAt, event.Data.HostID, event.Data.ServiceName),
	})

	req := AnalysisCreateRequest{
		Type:     AnalysisRootCause,
		AlertIDs: []uuid.UUID{alertID},
		Context:  rawContext,
	}

	_, err = uc.CreateAnalysis(ctx, req)
	return err
}

// HandleIncidentCreated 处理 Kafka incident.created 事件，自动触发事件级根因分析。
func (uc *AnalysisUseCase) HandleIncidentCreated(ctx context.Context, event IncidentCreatedEvent) error {
	incidentID, err := uuid.Parse(event.Data.IncidentID)
	if err != nil {
		return fmt.Errorf("invalid incident_id: %w", err)
	}

	rawContext, _ := json.Marshal(map[string]interface{}{
		"alert_info": fmt.Sprintf("IncidentID: %s\nTitle: %s\nSeverity: %s\nStatus: %s",
			event.Data.IncidentID, event.Data.Title, event.Data.Severity, event.Data.Status),
	})

	req := AnalysisCreateRequest{
		Type:       AnalysisRootCause,
		IncidentID: &incidentID,
		Context:    rawContext,
	}

	_, err = uc.CreateAnalysis(ctx, req)
	return err
}

// AlertFiredEvent 表示 Kafka alert.fired 事件的消息结构。
type AlertFiredEvent struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Source string         `json:"source"`
	Time   string         `json:"time"`
	Data   AlertFiredData `json:"data"`
}

// AlertFiredData 包含告警触发事件的详细数据。
type AlertFiredData struct {
	AlertID       string `json:"alert_id"`
	RuleID        string `json:"rule_id"`
	Severity      string `json:"severity"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	FiredAt       string `json:"fired_at"`
	HostID        string `json:"host_id"`
	ServiceName   string `json:"service_name"`
}

// IncidentCreatedEvent 表示 Kafka incident.created 事件的消息结构。
type IncidentCreatedEvent struct {
	ID     string              `json:"id"`
	Type   string              `json:"type"`
	Source string              `json:"source"`
	Time   string              `json:"time"`
	Data   IncidentCreatedData `json:"data"`
}

// IncidentCreatedData 包含事件创建事件的详细数据。
type IncidentCreatedData struct {
	IncidentID string `json:"incident_id"`
	Title      string `json:"title"`
	Severity   string `json:"severity"`
	Status     string `json:"status"`
}
