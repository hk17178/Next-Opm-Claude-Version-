package biz

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/opsnexus/opsnexus/pkg/event"
	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
)

// IncidentRepo 定义事件持久化接口，由数据层实现。
type IncidentRepo interface {
	NextID(ctx context.Context) (string, error)
	Create(ctx context.Context, inc *Incident) error
	GetByID(ctx context.Context, id string) (*Incident, error)
	Update(ctx context.Context, inc *Incident) error
	List(ctx context.Context, f ListFilter) ([]*Incident, int64, error)
	Delete(ctx context.Context, id string) error
	// SetMTTR 在事件关闭时将计算好的 MTTR 秒数持久化到 incidents 表的 mttr_seconds 列。
	// 单独提供此方法是为了避免 Update 方法中需要携带 mttr_seconds 字段，保持向后兼容。
	SetMTTR(ctx context.Context, incidentID string, mttrSeconds int64) error
}

// IncidentChangeRepo 定义事件变更工单关联的持久化接口，由数据层实现。
type IncidentChangeRepo interface {
	// Add 将一条变更关联记录写入 incident_changes 表。
	Add(ctx context.Context, change *IncidentChange) error
	// ListByIncident 查询指定事件的所有变更工单关联记录，按创建时间升序返回。
	ListByIncident(ctx context.Context, incidentID string) ([]*IncidentChange, error)
}

// TimelineRepo 定义事件时间线持久化接口。
type TimelineRepo interface {
	Add(ctx context.Context, entry *TimelineEntry) error
	ListByIncident(ctx context.Context, incidentID string) ([]*TimelineEntry, error)
}

// ScheduleRepo 定义值班排班持久化接口，用于自动分配时查询匹配的值班人员。
type ScheduleRepo interface {
	FindByScope(ctx context.Context, businessUnit string) ([]*OncallSchedule, error)
}

// IncidentUsecase 编排事件全生命周期操作，包括创建、状态流转、分配、升级、复盘和指标计算。
type IncidentUsecase struct {
	repo     IncidentRepo
	timeline TimelineRepo
	schedule ScheduleRepo
	changes  IncidentChangeRepo // 变更工单关联仓储，可选注入
	producer *event.Producer
	logger   *zap.Logger
}

// NewIncidentUsecase 创建一个新的事件用例实例，注入所需的仓储和事件发布器依赖。
func NewIncidentUsecase(
	repo IncidentRepo,
	timeline TimelineRepo,
	schedule ScheduleRepo,
	producer *event.Producer,
	logger *zap.Logger,
) *IncidentUsecase {
	return &IncidentUsecase{
		repo:     repo,
		timeline: timeline,
		schedule: schedule,
		producer: producer,
		logger:   logger,
	}
}

// SetIncidentChangeRepo 注入变更工单关联仓储，支持可选注入以保持向后兼容。
// 若未注入，AddIncidentChange 将返回服务未配置的错误。
func (uc *IncidentUsecase) SetIncidentChangeRepo(repo IncidentChangeRepo) {
	uc.changes = repo
}

// CreateIncident 创建新事件，自动生成唯一 ID，并尝试通过值班排班自动分配处理人。
func (uc *IncidentUsecase) CreateIncident(ctx context.Context, req CreateIncidentRequest) (*Incident, error) {
	id, err := uc.repo.NextID(ctx)
	if err != nil {
		return nil, apperrors.Internal("INCIDENT_ID_GENERATE", fmt.Sprintf("failed to generate incident ID: %v", err))
	}

	now := time.Now()
	detectedAt := now
	if req.DetectedAt != nil {
		detectedAt = *req.DetectedAt
	}

	inc := &Incident{
		IncidentID:     id,
		Title:          req.Title,
		Severity:       req.Severity,
		Status:         StatusCreated,
		DetectedAt:     detectedAt,
		SourceAlerts:   req.SourceAlerts,
		AffectedAssets: req.AffectedAssets,
		BusinessUnit:   req.BusinessUnit,
		Tags:           req.Tags,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if inc.SourceAlerts == nil {
		inc.SourceAlerts = []string{}
	}
	if inc.AffectedAssets == nil {
		inc.AffectedAssets = []string{}
	}
	if inc.Tags == nil {
		inc.Tags = map[string]string{}
	}
	if inc.ImprovementItems == nil {
		inc.ImprovementItems = []ImprovementItem{}
	}

	if err := uc.repo.Create(ctx, inc); err != nil {
		return nil, apperrors.Internal("INCIDENT_CREATE", fmt.Sprintf("failed to create incident: %v", err))
	}

	// 记录事件创建的时间线条目
	uc.addTimelineEntry(ctx, inc.IncidentID, "status_change", "system", map[string]any{
		"new_status": string(StatusCreated),
		"message":    "Incident created",
	})

	// 根据业务单元匹配值班排班，尝试自动分配处理人
	uc.tryAutoAssign(ctx, inc)

	// 发布 incident.created 事件，通知下游服务（如通知服务）
	uc.publishCreated(ctx, inc)

	uc.logger.Info("incident created",
		zap.String("incident_id", inc.IncidentID),
		zap.String("severity", inc.Severity),
		zap.String("business_unit", inc.BusinessUnit),
	)

	return inc, nil
}

// GetIncident 根据事件 ID 查询单条事件记录，不存在时返回 NotFound 错误。
func (uc *IncidentUsecase) GetIncident(ctx context.Context, id string) (*Incident, error) {
	inc, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperrors.Internal("INCIDENT_GET", fmt.Sprintf("failed to get incident: %v", err))
	}
	if inc == nil {
		return nil, apperrors.NotFound(apperrors.ErrIncidentNotFound, fmt.Sprintf("incident %s not found", id))
	}
	return inc, nil
}

// ListIncidents 返回按条件过滤和分页的事件列表。
func (uc *IncidentUsecase) ListIncidents(ctx context.Context, f ListFilter) ([]*Incident, int64, error) {
	incidents, total, err := uc.repo.List(ctx, f)
	if err != nil {
		return nil, 0, apperrors.Internal("INCIDENT_LIST", fmt.Sprintf("failed to list incidents: %v", err))
	}
	return incidents, total, nil
}

// UpdateStatus 执行事件状态流转，校验状态机合法性并自动记录关键时间戳。
func (uc *IncidentUsecase) UpdateStatus(ctx context.Context, id string, newStatus Status, note string) (*Incident, error) {
	inc, err := uc.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}

	if !CanTransition(inc.Status, newStatus) {
		return nil, apperrors.BadRequest(apperrors.ErrIncidentInvalidTransition,
			fmt.Sprintf("cannot transition from %s to %s", inc.Status, newStatus))
	}

	// P0/P1 级别事件必须完成复盘后才能关闭
	if newStatus == StatusClosed && RequiresPostmortem(inc.Severity) && inc.Postmortem == nil {
		return nil, apperrors.BadRequest(apperrors.ErrPostmortemRequired,
			"P0/P1 incidents require a completed postmortem before closing")
	}

	oldStatus := inc.Status
	inc.Status = newStatus
	now := time.Now()

	switch newStatus {
	case StatusAssigned:
		if inc.AcknowledgedAt == nil {
			inc.AcknowledgedAt = &now
		}
	case StatusResolved:
		inc.ResolvedAt = &now
	case StatusClosed:
		inc.ClosedAt = &now
	}

	if err := uc.repo.Update(ctx, inc); err != nil {
		return nil, apperrors.Internal("INCIDENT_UPDATE", fmt.Sprintf("failed to update incident: %v", err))
	}

	// 关闭事件时计算并持久化 MTTR（平均修复时间）到 incidents 表。
	// MTTR 定义：resolved_at - created_at（秒）。
	// 若事件尚未解决（resolved_at 为空），则跳过写入（MTTR 保持 NULL）。
	if newStatus == StatusClosed && inc.ResolvedAt != nil {
		mttrSeconds := int64(inc.ResolvedAt.Sub(inc.DetectedAt).Seconds())
		// SetMTTR 失败不影响事件关闭的主流程，仅记录警告日志
		if err := uc.repo.SetMTTR(ctx, inc.IncidentID, mttrSeconds); err != nil {
			uc.logger.Warn("failed to persist mttr_seconds on close",
				zap.String("incident_id", inc.IncidentID),
				zap.Int64("mttr_seconds", mttrSeconds),
				zap.Error(err),
			)
		} else {
			uc.logger.Info("mttr persisted on incident close",
				zap.String("incident_id", inc.IncidentID),
				zap.Int64("mttr_seconds", mttrSeconds),
			)
		}
	}

	// 记录状态变更的时间线条目
	uc.addTimelineEntry(ctx, inc.IncidentID, "status_change", "system", map[string]any{
		"old_status": string(oldStatus),
		"new_status": string(newStatus),
		"note":       note,
	})

	// 发布状态变更事件，已解决时额外发布 incident.resolved 事件，关闭时发布 incident.closed 事件
	uc.publishUpdated(ctx, inc, oldStatus)
	if newStatus == StatusResolved {
		uc.publishResolved(ctx, inc)
	}
	if newStatus == StatusClosed {
		uc.publishClosed(ctx, inc)
	}

	uc.logger.Info("incident status updated",
		zap.String("incident_id", id),
		zap.String("old_status", string(oldStatus)),
		zap.String("new_status", string(newStatus)),
	)

	return inc, nil
}

// AssignIncident 将事件分配给指定处理人，若事件处于早期阶段则自动流转到 assigned 状态。
func (uc *IncidentUsecase) AssignIncident(ctx context.Context, id string, assigneeID, assigneeName string) (*Incident, error) {
	inc, err := uc.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}

	if IsTerminal(inc.Status) {
		return nil, apperrors.BadRequest(apperrors.ErrIncidentAlreadyClosed, "cannot assign a closed incident")
	}

	inc.AssigneeID = &assigneeID
	inc.AssigneeName = &assigneeName

	// 如果事件处于创建/分诊/分析等早期阶段，自动流转到已分配状态
	if inc.Status == StatusCreated || inc.Status == StatusTriaging || inc.Status == StatusAnalyzing {
		now := time.Now()
		if inc.AcknowledgedAt == nil {
			inc.AcknowledgedAt = &now
		}
		inc.Status = StatusAssigned
	}

	if err := uc.repo.Update(ctx, inc); err != nil {
		return nil, apperrors.Internal("INCIDENT_ASSIGN", fmt.Sprintf("failed to assign incident: %v", err))
	}

	uc.addTimelineEntry(ctx, inc.IncidentID, "assignment", "system", map[string]any{
		"assignee_id":   assigneeID,
		"assignee_name": assigneeName,
	})

	uc.publishUpdated(ctx, inc, inc.Status)

	return inc, nil
}

// EscalateIncident 升级事件严重等级，记录升级原因到时间线并发布变更事件。
func (uc *IncidentUsecase) EscalateIncident(ctx context.Context, id string, newSeverity, reason string) (*Incident, error) {
	inc, err := uc.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}

	oldSeverity := inc.Severity
	inc.Severity = newSeverity

	if err := uc.repo.Update(ctx, inc); err != nil {
		return nil, apperrors.Internal("INCIDENT_ESCALATE", fmt.Sprintf("failed to escalate incident: %v", err))
	}

	uc.addTimelineEntry(ctx, inc.IncidentID, "escalation", "system", map[string]any{
		"old_severity": oldSeverity,
		"new_severity": newSeverity,
		"reason":       reason,
	})

	uc.publishUpdated(ctx, inc, inc.Status)

	uc.logger.Info("incident escalated",
		zap.String("incident_id", id),
		zap.String("old_severity", oldSeverity),
		zap.String("new_severity", newSeverity),
	)

	return inc, nil
}

// AddPostmortem 为事件添加复盘报告（P0/P1 级别事件关闭前必须提交），已解决事件自动流转到复盘状态。
func (uc *IncidentUsecase) AddPostmortem(ctx context.Context, id string, pm *Postmortem) (*Incident, error) {
	inc, err := uc.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}

	inc.Postmortem = pm

	// 如果事件已解决，自动流转到复盘状态
	if inc.Status == StatusResolved {
		inc.Status = StatusPostmortem
	}

	if err := uc.repo.Update(ctx, inc); err != nil {
		return nil, apperrors.Internal("INCIDENT_POSTMORTEM", fmt.Sprintf("failed to add postmortem: %v", err))
	}

	uc.addTimelineEntry(ctx, inc.IncidentID, "note", "human", map[string]any{
		"type":       "postmortem",
		"root_cause": pm.RootCause,
		"author_id":  pm.AuthorID,
	})

	return inc, nil
}

// GetTimeline 返回指定事件的完整时间线记录。
func (uc *IncidentUsecase) GetTimeline(ctx context.Context, incidentID string) ([]*TimelineEntry, error) {
	// 先验证事件是否存在
	if _, err := uc.GetIncident(ctx, incidentID); err != nil {
		return nil, err
	}
	entries, err := uc.timeline.ListByIncident(ctx, incidentID)
	if err != nil {
		return nil, apperrors.Internal("TIMELINE_LIST", fmt.Sprintf("failed to list timeline: %v", err))
	}
	return entries, nil
}

// AddTimelineEntry 向事件时间线添加一条手动记录（如人工备注）。
func (uc *IncidentUsecase) AddTimelineEntry(ctx context.Context, incidentID string, entry *TimelineEntry) error {
	if _, err := uc.GetIncident(ctx, incidentID); err != nil {
		return err
	}
	entry.IncidentID = incidentID
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	return uc.timeline.Add(ctx, entry)
}

// GetMetrics 计算指定事件的 MTTA/MTTI/MTTR 时效指标。
func (uc *IncidentUsecase) GetMetrics(ctx context.Context, id string) (*IncidentMetrics, error) {
	inc, err := uc.GetIncident(ctx, id)
	if err != nil {
		return nil, err
	}
	m := inc.CalculateMetrics()
	return &m, nil
}

// AddIncidentChange 将变更工单与指定事件关联，记录操作人信息和关联描述。
//
// 使用场景：
//   - 排查事件时发现某次变更是触发根因，操作人通过此接口将变更工单关联到事件。
//   - 关闭事件时引用对应的回滚变更工单作为修复证据。
//
// 参数：
//   - ctx: 请求上下文。
//   - incidentID: 目标事件 ID（必须存在）。
//   - req: 变更关联请求，包含变更工单 ID、描述和操作人信息。
func (uc *IncidentUsecase) AddIncidentChange(ctx context.Context, incidentID string, req AddIncidentChangeRequest) (*IncidentChange, error) {
	// 校验变更工单关联功能是否已配置
	if uc.changes == nil {
		return nil, apperrors.Internal("CHANGE_REPO_NIL", "incident change repository is not configured")
	}

	// 校验事件存在
	if _, err := uc.GetIncident(ctx, incidentID); err != nil {
		return nil, err
	}

	// 校验变更工单 ID 不能为空
	if req.ChangeOrderID == "" {
		return nil, apperrors.BadRequest("VALIDATION", "change_order_id is required")
	}

	change := &IncidentChange{
		IncidentID:    incidentID,
		ChangeOrderID: req.ChangeOrderID,
		Description:   req.Description,
		OperatorID:    req.OperatorID,
		OperatorName:  req.OperatorName,
	}

	if err := uc.changes.Add(ctx, change); err != nil {
		return nil, apperrors.Internal("INCIDENT_CHANGE_ADD", fmt.Sprintf("failed to add incident change: %v", err))
	}

	// 将变更关联操作记录到事件时间线，便于事后追溯
	uc.addTimelineEntry(ctx, incidentID, "note", "human", map[string]any{
		"type":            "change_linked",
		"change_order_id": req.ChangeOrderID,
		"description":     req.Description,
		"operator_id":     req.OperatorID,
		"operator_name":   req.OperatorName,
	})

	uc.logger.Info("incident change linked",
		zap.String("incident_id", incidentID),
		zap.String("change_order_id", req.ChangeOrderID),
		zap.String("operator_id", req.OperatorID),
	)

	return change, nil
}

// ListIncidentChanges 查询指定事件关联的所有变更工单记录，按创建时间升序返回。
//
// 参数：
//   - ctx: 请求上下文。
//   - incidentID: 事件 ID。
func (uc *IncidentUsecase) ListIncidentChanges(ctx context.Context, incidentID string) ([]*IncidentChange, error) {
	if uc.changes == nil {
		return []*IncidentChange{}, nil
	}
	// 校验事件存在
	if _, err := uc.GetIncident(ctx, incidentID); err != nil {
		return nil, err
	}
	changes, err := uc.changes.ListByIncident(ctx, incidentID)
	if err != nil {
		return nil, apperrors.Internal("INCIDENT_CHANGE_LIST", fmt.Sprintf("failed to list incident changes: %v", err))
	}
	return changes, nil
}

// AddIncidentChangeRequest 封装关联变更工单的请求参数。
type AddIncidentChangeRequest struct {
	ChangeOrderID string `json:"change_order_id"` // 变更工单 ID（必填）
	Description   string `json:"description"`     // 变更关联描述（可选）
	OperatorID    string `json:"operator_id"`     // 操作人用户 ID（可选）
	OperatorName  string `json:"operator_name"`   // 操作人姓名（可选，冗余存储）
}

// CreateIncidentRequest 封装创建事件的请求参数。
type CreateIncidentRequest struct {
	Title          string            `json:"title"`
	Severity       string            `json:"severity"`
	SourceAlerts   []string          `json:"source_alerts"`
	AffectedAssets []string          `json:"affected_assets"`
	BusinessUnit   string            `json:"business_unit"`
	Tags           map[string]string `json:"tags"`
	DetectedAt     *time.Time        `json:"detected_at"`
}

// tryAutoAssign 尝试根据值班排班自动分配事件处理人。
// 通过业务单元匹配排班计划，使用轮转算法找到当前值班人员后自动指派。
func (uc *IncidentUsecase) tryAutoAssign(ctx context.Context, inc *Incident) {
	if inc.BusinessUnit == "" {
		return
	}
	schedules, err := uc.schedule.FindByScope(ctx, inc.BusinessUnit)
	if err != nil || len(schedules) == 0 {
		return
	}

	now := time.Now()
	for _, sched := range schedules {
		if !sched.Enabled {
			continue
		}
		member := GetCurrentOnCallPerson(sched, now)
		if member == nil {
			continue
		}

		inc.AssigneeID = &member.UserID
		inc.AssigneeName = &member.Name
		if inc.AcknowledgedAt == nil {
			inc.AcknowledgedAt = &now
		}
		inc.Status = StatusAssigned

		uc.addTimelineEntry(ctx, inc.IncidentID, "assignment", "system", map[string]any{
			"assignee_id":   member.UserID,
			"assignee_name": member.Name,
			"source":        "oncall_rotation",
			"schedule_id":   sched.ScheduleID,
		})

		uc.logger.Info("auto-assigned incident via on-call rotation",
			zap.String("incident_id", inc.IncidentID),
			zap.String("assignee_id", member.UserID),
			zap.String("schedule_id", sched.ScheduleID),
		)
		return
	}

	uc.logger.Debug("auto-assign: no on-call person found",
		zap.String("incident_id", inc.IncidentID),
		zap.Int("schedule_count", len(schedules)),
	)
}

// addTimelineEntry 内部辅助方法，向事件时间线追加一条系统自动生成的记录。
func (uc *IncidentUsecase) addTimelineEntry(ctx context.Context, incidentID, entryType, source string, content any) {
	entry := &TimelineEntry{
		IncidentID: incidentID,
		Timestamp:  time.Now(),
		EntryType:  entryType,
		Source:     source,
		Content:    content,
	}
	if err := uc.timeline.Add(ctx, entry); err != nil {
		uc.logger.Error("failed to add timeline entry",
			zap.String("incident_id", incidentID),
			zap.Error(err),
		)
	}
}

// publishCreated 发布 incident.created CloudEvent 事件，通知下游服务（如 svc-notify）。
// 若 Producer 未配置（为 nil）则跳过发布，避免空指针 panic。
func (uc *IncidentUsecase) publishCreated(ctx context.Context, inc *Incident) {
	// BUG-001 修复：Producer 未配置时（如单元测试场景），直接返回，防止 nil 指针 panic。
	if uc.producer == nil {
		return
	}
	data := map[string]any{
		"incident_id":     inc.IncidentID,
		"title":           inc.Title,
		"severity":        inc.Severity,
		"status":          string(inc.Status),
		"source_alerts":   inc.SourceAlerts,
		"assignee":        map[string]any{"user_id": inc.AssigneeID, "name": inc.AssigneeName},
		"business_unit":   inc.BusinessUnit,
		"affected_assets": inc.AffectedAssets,
		"detected_at":     inc.DetectedAt.Format(time.RFC3339),
	}
	evt, err := event.NewCloudEvent(event.TypeIncidentCreated, "svc-incident", data)
	if err != nil {
		uc.logger.Error("failed to create incident.created event", zap.Error(err))
		return
	}
	if err := uc.producer.Publish(ctx, event.TopicIncidentCreated, evt); err != nil {
		uc.logger.Error("failed to publish incident.created event", zap.Error(err))
	}
}

// publishUpdated 发布 incident.updated CloudEvent 事件，包含状态变更前后信息。
// 若 Producer 未配置（为 nil）则跳过发布，避免空指针 panic。
func (uc *IncidentUsecase) publishUpdated(ctx context.Context, inc *Incident, oldStatus Status) {
	// BUG-001 修复：Producer 未配置时直接返回，防止 nil 指针 panic。
	if uc.producer == nil {
		return
	}
	data := map[string]any{
		"incident_id":    inc.IncidentID,
		"title":          inc.Title,
		"severity":       inc.Severity,
		"previous_status": string(oldStatus),
		"new_status":     string(inc.Status),
		"assignee":       map[string]any{"user_id": inc.AssigneeID, "name": inc.AssigneeName},
		"update_type":    "status_change",
		"update_source":  "system",
		"business_unit":  inc.BusinessUnit,
	}
	evt, err := event.NewCloudEvent(event.TypeIncidentUpdated, "svc-incident", data)
	if err != nil {
		uc.logger.Error("failed to create incident.updated event", zap.Error(err))
		return
	}
	if err := uc.producer.Publish(ctx, event.TopicIncidentUpdated, evt); err != nil {
		uc.logger.Error("failed to publish incident.updated event", zap.Error(err))
	}
}

// publishClosed 发布 incident.closed CloudEvent 事件，包含完整的事件生命周期指标。
// 若 Producer 未配置（为 nil）则跳过发布，避免空指针 panic。
func (uc *IncidentUsecase) publishClosed(ctx context.Context, inc *Incident) {
	// BUG-001 修复：Producer 未配置时直接返回，防止 nil 指针 panic。
	if uc.producer == nil {
		return
	}
	m := inc.CalculateMetrics()
	data := map[string]any{
		"incident_id":        inc.IncidentID,
		"title":              inc.Title,
		"severity":           inc.Severity,
		"status":             string(inc.Status),
		"root_cause_category": inc.RootCauseCategory,
		"mtta_seconds":       m.MTTASeconds,
		"mttr_seconds":       m.MTTRSeconds,
		"detected_at":        inc.DetectedAt.Format(time.RFC3339),
		"closed_at":          nil,
		"has_postmortem":     inc.Postmortem != nil,
		"business_unit":      inc.BusinessUnit,
	}
	if inc.ClosedAt != nil {
		data["closed_at"] = inc.ClosedAt.Format(time.RFC3339)
	}
	if inc.ResolvedAt != nil {
		data["resolved_at"] = inc.ResolvedAt.Format(time.RFC3339)
	}
	evt, err := event.NewCloudEvent("incident.closed", "svc-incident", data)
	if err != nil {
		uc.logger.Error("failed to create incident.closed event", zap.Error(err))
		return
	}
	if err := uc.producer.Publish(ctx, "incident.closed", evt); err != nil {
		uc.logger.Error("failed to publish incident.closed event", zap.Error(err))
	}
}

// publishResolved 发布 incident.resolved CloudEvent 事件，包含 MTTA/MTTR 等时效指标。
// 若 Producer 未配置（为 nil）则跳过发布，避免空指针 panic。
func (uc *IncidentUsecase) publishResolved(ctx context.Context, inc *Incident) {
	// BUG-001 修复：Producer 未配置时直接返回，防止 nil 指针 panic。
	if uc.producer == nil {
		return
	}
	m := inc.CalculateMetrics()
	data := map[string]any{
		"incident_id":        inc.IncidentID,
		"title":              inc.Title,
		"severity":           inc.Severity,
		"status":             string(inc.Status),
		"root_cause_category": inc.RootCauseCategory,
		"assignee":           map[string]any{"user_id": inc.AssigneeID, "name": inc.AssigneeName},
		"detected_at":        inc.DetectedAt.Format(time.RFC3339),
		"acknowledged_at":    nil,
		"resolved_at":        nil,
		"mtta_seconds":       m.MTTASeconds,
		"mttr_seconds":       m.MTTRSeconds,
		"affected_assets":    inc.AffectedAssets,
		"business_unit":      inc.BusinessUnit,
	}
	if inc.AcknowledgedAt != nil {
		data["acknowledged_at"] = inc.AcknowledgedAt.Format(time.RFC3339)
	}
	if inc.ResolvedAt != nil {
		data["resolved_at"] = inc.ResolvedAt.Format(time.RFC3339)
	}
	evt, err := event.NewCloudEvent(event.TypeIncidentResolved, "svc-incident", data)
	if err != nil {
		uc.logger.Error("failed to create incident.resolved event", zap.Error(err))
		return
	}
	if err := uc.producer.Publish(ctx, event.TopicIncidentResolved, evt); err != nil {
		uc.logger.Error("failed to publish incident.resolved event", zap.Error(err))
	}
}
