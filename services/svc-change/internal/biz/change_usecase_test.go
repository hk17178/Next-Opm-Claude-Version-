package biz

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// --- Mock 仓储实现 ---

// mockChangeRepo 变更单的内存 mock 仓储，用于单元测试。
type mockChangeRepo struct {
	tickets   map[string]*ChangeTicket
	nextIDSeq int
}

func newMockChangeRepo() *mockChangeRepo {
	return &mockChangeRepo{tickets: make(map[string]*ChangeTicket)}
}

func (m *mockChangeRepo) NextID(_ context.Context) (string, error) {
	m.nextIDSeq++
	today := time.Now().Format("20060102")
	return fmt.Sprintf("CHG-%s-%03d", today, m.nextIDSeq), nil
}

func (m *mockChangeRepo) Create(_ context.Context, ticket *ChangeTicket) error {
	m.tickets[ticket.ID] = ticket
	return nil
}

func (m *mockChangeRepo) GetByID(_ context.Context, id string) (*ChangeTicket, error) {
	ticket, ok := m.tickets[id]
	if !ok {
		return nil, nil
	}
	return ticket, nil
}

func (m *mockChangeRepo) Update(_ context.Context, ticket *ChangeTicket) error {
	m.tickets[ticket.ID] = ticket
	return nil
}

func (m *mockChangeRepo) List(_ context.Context, f ListFilter) ([]*ChangeTicket, int64, error) {
	var result []*ChangeTicket
	for _, t := range m.tickets {
		if f.Status != nil && t.Status != *f.Status {
			continue
		}
		if f.Type != nil && t.Type != *f.Type {
			continue
		}
		result = append(result, t)
	}
	return result, int64(len(result)), nil
}

// FindConflicts 查找与指定时间段和资产列表存在冲突的变更单。
// 冲突条件：时间段重叠 AND 资产有交集，排除终态和自身。
func (m *mockChangeRepo) FindConflicts(_ context.Context, start, end time.Time, assets []string, excludeID string) ([]*ChangeTicket, error) {
	var conflicts []*ChangeTicket
	for _, t := range m.tickets {
		// 排除自身
		if t.ID == excludeID {
			continue
		}
		// 排除终态
		if t.Status == StatusCompleted || t.Status == StatusCancelled || t.Status == StatusRejected {
			continue
		}
		// 检查时间重叠：A.start < B.end AND A.end > B.start
		if t.ScheduledStart.Before(end) && t.ScheduledEnd.After(start) {
			// 检查资产交集
			if hasAssetOverlap(t.AffectedAssets, assets) {
				conflicts = append(conflicts, t)
			}
		}
	}
	return conflicts, nil
}

// hasAssetOverlap 检查两个资产列表是否有交集。
func hasAssetOverlap(a, b []string) bool {
	set := make(map[string]bool)
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		if set[v] {
			return true
		}
	}
	return false
}

func (m *mockChangeRepo) ListByTimeRange(_ context.Context, start, end time.Time) ([]*ChangeTicket, error) {
	var result []*ChangeTicket
	for _, t := range m.tickets {
		if t.Status == StatusCancelled || t.Status == StatusRejected {
			continue
		}
		if t.ScheduledStart.Before(end) && t.ScheduledEnd.After(start) {
			result = append(result, t)
		}
	}
	return result, nil
}

// mockApprovalRepo 审批记录的内存 mock 仓储。
type mockApprovalRepo struct {
	records []*ApprovalRecord
}

func newMockApprovalRepo() *mockApprovalRepo {
	return &mockApprovalRepo{}
}

func (m *mockApprovalRepo) Create(_ context.Context, record *ApprovalRecord) error {
	m.records = append(m.records, record)
	return nil
}

func (m *mockApprovalRepo) ListByChange(_ context.Context, changeID string) ([]*ApprovalRecord, error) {
	var result []*ApprovalRecord
	for _, r := range m.records {
		if r.ChangeID == changeID {
			result = append(result, r)
		}
	}
	return result, nil
}

// --- 测试辅助函数 ---

// newTestChangeUsecase 创建用于测试的变更用例实例。
func newTestChangeUsecase() (*ChangeUsecase, *mockChangeRepo, *mockApprovalRepo) {
	repo := newMockChangeRepo()
	approvalRepo := newMockApprovalRepo()
	log := zap.NewNop()
	uc := NewChangeUsecase(repo, approvalRepo, log)
	return uc, repo, approvalRepo
}

// newTestApprovalUsecase 创建用于测试的审批用例实例。
func newTestApprovalUsecase() (*ApprovalUsecase, *mockChangeRepo, *mockApprovalRepo) {
	repo := newMockChangeRepo()
	approvalRepo := newMockApprovalRepo()
	log := zap.NewNop()
	uc := NewApprovalUsecase(repo, approvalRepo, log)
	return uc, repo, approvalRepo
}

// createTestTicket 创建一个测试用的变更单。
func createTestTicket(uc *ChangeUsecase) *ChangeTicket {
	ctx := context.Background()
	ticket, _ := uc.Create(ctx, &CreateChangeReq{
		Title:          "升级数据库到 PostgreSQL 16",
		Type:           ChangeTypeNormal,
		RiskLevel:      RiskMedium,
		Requester:      "user-001",
		ExecutorID:     "user-002",
		AffectedAssets: []string{"asset-db-001", "asset-db-002"},
		RollbackPlan:   "回滚到 PostgreSQL 15",
		ScheduledStart: time.Date(2026, 3, 25, 2, 0, 0, 0, time.UTC),
		ScheduledEnd:   time.Date(2026, 3, 25, 4, 0, 0, 0, time.UTC),
		Description:    "将生产数据库从 PostgreSQL 15 升级到 16",
	})
	return ticket
}

// --- 测试用例 ---

// TestCreate_IDFormat 测试创建变更单后 ID 格式是否符合 CHG-YYYYMMDD-NNN。
func TestCreate_IDFormat(t *testing.T) {
	uc, _, _ := newTestChangeUsecase()
	ticket := createTestTicket(uc)

	if ticket == nil {
		t.Fatal("expected ticket to be created")
	}
	if !strings.HasPrefix(ticket.ID, "CHG-") {
		t.Errorf("ticket ID = %s, want prefix CHG-", ticket.ID)
	}
	// 验证格式 CHG-YYYYMMDD-NNN
	parts := strings.Split(ticket.ID, "-")
	if len(parts) != 3 {
		t.Errorf("ticket ID = %s, want format CHG-YYYYMMDD-NNN", ticket.ID)
	}
	if ticket.Status != StatusDraft {
		t.Errorf("status = %s, want draft", ticket.Status)
	}
}

// TestSubmit_StatusTransition 测试提交审批后状态是否流转到 pending_approval。
func TestSubmit_StatusTransition(t *testing.T) {
	uc, repo, _ := newTestChangeUsecase()
	ticket := createTestTicket(uc)
	ctx := context.Background()

	// 提交审批
	err := uc.Submit(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}

	// 验证状态变更
	updated := repo.tickets[ticket.ID]
	if updated.Status != StatusPendingApproval {
		t.Errorf("status = %s, want pending_approval", updated.Status)
	}
}

// TestConflictDetection 测试时间重叠+资产重叠的冲突检测。
func TestConflictDetection(t *testing.T) {
	uc, _, _ := newTestChangeUsecase()
	ctx := context.Background()

	// 创建第一个变更单
	ticket1, _ := uc.Create(ctx, &CreateChangeReq{
		Title:          "变更 A",
		Type:           ChangeTypeNormal,
		RiskLevel:      RiskLow,
		AffectedAssets: []string{"asset-001", "asset-002"},
		ScheduledStart: time.Date(2026, 3, 25, 2, 0, 0, 0, time.UTC),
		ScheduledEnd:   time.Date(2026, 3, 25, 4, 0, 0, 0, time.UTC),
	})

	// 检测冲突：时间重叠 + 资产重叠
	conflicts, err := uc.CheckConflicts(ctx,
		time.Date(2026, 3, 25, 3, 0, 0, 0, time.UTC), // 与 ticket1 时间重叠
		time.Date(2026, 3, 25, 5, 0, 0, 0, time.UTC),
		[]string{"asset-002", "asset-003"}, // 与 ticket1 资产重叠（asset-002）
		"",                                  // 不排除任何变更单
	)
	if err != nil {
		t.Fatalf("CheckConflicts error: %v", err)
	}

	if len(conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(conflicts))
	}
	if len(conflicts) > 0 && conflicts[0].ID != ticket1.ID {
		t.Errorf("conflict ID = %s, want %s", conflicts[0].ID, ticket1.ID)
	}

	// 检测无冲突：时间不重叠
	noConflicts, err := uc.CheckConflicts(ctx,
		time.Date(2026, 3, 26, 2, 0, 0, 0, time.UTC), // 不与 ticket1 重叠
		time.Date(2026, 3, 26, 4, 0, 0, 0, time.UTC),
		[]string{"asset-001"},
		"",
	)
	if err != nil {
		t.Fatalf("CheckConflicts error: %v", err)
	}
	if len(noConflicts) != 0 {
		t.Errorf("expected 0 conflicts for non-overlapping time, got %d", len(noConflicts))
	}

	// 检测无冲突：资产不重叠
	noAssetConflicts, err := uc.CheckConflicts(ctx,
		time.Date(2026, 3, 25, 3, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 25, 5, 0, 0, 0, time.UTC),
		[]string{"asset-999"}, // 无交集
		"",
	)
	if err != nil {
		t.Fatalf("CheckConflicts error: %v", err)
	}
	if len(noAssetConflicts) != 0 {
		t.Errorf("expected 0 conflicts for non-overlapping assets, got %d", len(noAssetConflicts))
	}
}

// TestApproveAndStartExecution 测试审批通过后可以开始执行。
func TestApproveAndStartExecution(t *testing.T) {
	changeRepo := newMockChangeRepo()
	approvalRepo := newMockApprovalRepo()
	log := zap.NewNop()

	changeUC := NewChangeUsecase(changeRepo, approvalRepo, log)
	approvalUC := NewApprovalUsecase(changeRepo, approvalRepo, log)

	ctx := context.Background()

	// 创建并提交变更单
	ticket, _ := changeUC.Create(ctx, &CreateChangeReq{
		Title:          "部署新版本",
		Type:           ChangeTypeNormal,
		RiskLevel:      RiskMedium,
		AffectedAssets: []string{"asset-app-001"},
		ScheduledStart: time.Date(2026, 3, 25, 2, 0, 0, 0, time.UTC),
		ScheduledEnd:   time.Date(2026, 3, 25, 4, 0, 0, 0, time.UTC),
	})
	changeUC.Submit(ctx, ticket.ID)

	// 审批通过
	err := approvalUC.Approve(ctx, ticket.ID, "approver-001", "同意执行")
	if err != nil {
		t.Fatalf("Approve error: %v", err)
	}

	// 验证状态
	updated := changeRepo.tickets[ticket.ID]
	if updated.Status != StatusApproved {
		t.Errorf("status = %s, want approved", updated.Status)
	}

	// 开始执行
	err = changeUC.StartExecution(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("StartExecution error: %v", err)
	}

	updated = changeRepo.tickets[ticket.ID]
	if updated.Status != StatusInProgress {
		t.Errorf("status = %s, want in_progress", updated.Status)
	}
	if updated.ActualStart == nil {
		t.Error("actual_start should be set after starting execution")
	}
}

// TestCancelledCannotApprove 测试取消后不能再审批。
func TestCancelledCannotApprove(t *testing.T) {
	changeRepo := newMockChangeRepo()
	approvalRepo := newMockApprovalRepo()
	log := zap.NewNop()

	changeUC := NewChangeUsecase(changeRepo, approvalRepo, log)
	approvalUC := NewApprovalUsecase(changeRepo, approvalRepo, log)

	ctx := context.Background()

	// 创建并提交变更单
	ticket, _ := changeUC.Create(ctx, &CreateChangeReq{
		Title:          "升级中间件",
		Type:           ChangeTypeNormal,
		RiskLevel:      RiskHigh,
		AffectedAssets: []string{"asset-mw-001"},
		ScheduledStart: time.Date(2026, 3, 25, 2, 0, 0, 0, time.UTC),
		ScheduledEnd:   time.Date(2026, 3, 25, 4, 0, 0, 0, time.UTC),
	})
	changeUC.Submit(ctx, ticket.ID)

	// 取消变更
	err := changeUC.Cancel(ctx, ticket.ID, "需求变更，暂停执行")
	if err != nil {
		t.Fatalf("Cancel error: %v", err)
	}

	// 验证已取消
	updated := changeRepo.tickets[ticket.ID]
	if updated.Status != StatusCancelled {
		t.Errorf("status = %s, want cancelled", updated.Status)
	}

	// 尝试审批已取消的变更单，应返回错误
	err = approvalUC.Approve(ctx, ticket.ID, "approver-001", "同意")
	if err == nil {
		t.Fatal("expected error: cannot approve a cancelled change ticket")
	}
}

// TestCreate_ValidationErrors 测试创建变更单时的参数校验。
func TestCreate_ValidationErrors(t *testing.T) {
	uc, _, _ := newTestChangeUsecase()
	ctx := context.Background()

	// 缺少标题
	_, err := uc.Create(ctx, &CreateChangeReq{
		Type:      ChangeTypeNormal,
		RiskLevel: RiskLow,
	})
	if err == nil {
		t.Fatal("expected error for missing title")
	}

	// 缺少类型
	_, err = uc.Create(ctx, &CreateChangeReq{
		Title:     "Test",
		RiskLevel: RiskLow,
	})
	if err == nil {
		t.Fatal("expected error for missing type")
	}

	// 缺少风险级别
	_, err = uc.Create(ctx, &CreateChangeReq{
		Title: "Test",
		Type:  ChangeTypeNormal,
	})
	if err == nil {
		t.Fatal("expected error for missing risk_level")
	}
}

// TestUpdate_OnlyDraft 测试仅草稿状态可编辑。
func TestUpdate_OnlyDraft(t *testing.T) {
	uc, _, _ := newTestChangeUsecase()
	ctx := context.Background()

	ticket := createTestTicket(uc)

	// 草稿状态可编辑
	newTitle := "更新后的标题"
	updated, err := uc.Update(ctx, ticket.ID, &UpdateChangeReq{Title: &newTitle})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if updated.Title != newTitle {
		t.Errorf("title = %s, want %s", updated.Title, newTitle)
	}

	// 提交后不可编辑
	uc.Submit(ctx, ticket.ID)
	_, err = uc.Update(ctx, ticket.ID, &UpdateChangeReq{Title: &newTitle})
	if err == nil {
		t.Fatal("expected error: cannot edit after submission")
	}
}

// TestComplete_SetsActualEnd 测试完成变更后设置实际结束时间。
func TestComplete_SetsActualEnd(t *testing.T) {
	changeRepo := newMockChangeRepo()
	approvalRepo := newMockApprovalRepo()
	log := zap.NewNop()

	changeUC := NewChangeUsecase(changeRepo, approvalRepo, log)
	approvalUC := NewApprovalUsecase(changeRepo, approvalRepo, log)

	ctx := context.Background()

	ticket, _ := changeUC.Create(ctx, &CreateChangeReq{
		Title:          "部署",
		Type:           ChangeTypeNormal,
		RiskLevel:      RiskLow,
		AffectedAssets: []string{"asset-001"},
		ScheduledStart: time.Date(2026, 3, 25, 2, 0, 0, 0, time.UTC),
		ScheduledEnd:   time.Date(2026, 3, 25, 4, 0, 0, 0, time.UTC),
	})

	changeUC.Submit(ctx, ticket.ID)
	approvalUC.Approve(ctx, ticket.ID, "approver-001", "")
	changeUC.StartExecution(ctx, ticket.ID)

	err := changeUC.Complete(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}

	updated := changeRepo.tickets[ticket.ID]
	if updated.Status != StatusCompleted {
		t.Errorf("status = %s, want completed", updated.Status)
	}
	if updated.ActualEnd == nil {
		t.Error("actual_end should be set after completion")
	}
}

// TestAutoRoute_RiskBased 测试基于风险级别的自动审批路由。
func TestAutoRoute_RiskBased(t *testing.T) {
	approvalUC, _, _ := newTestApprovalUsecase()
	ctx := context.Background()

	tests := []struct {
		riskLevel RiskLevel
		wantLen   int
		wantRole  string
	}{
		{RiskLow, 0, ""},                  // 低风险自动通过
		{RiskMedium, 1, "role:supervisor"}, // 中风险需主管
		{RiskHigh, 1, "role:director"},     // 高风险需总监
		{RiskCritical, 1, "role:vp"},       // 极高风险需 VP
	}

	for _, tt := range tests {
		ticket := &ChangeTicket{RiskLevel: tt.riskLevel}
		approvers, err := approvalUC.AutoRoute(ctx, ticket)
		if err != nil {
			t.Fatalf("AutoRoute(%s) error: %v", tt.riskLevel, err)
		}
		if len(approvers) != tt.wantLen {
			t.Errorf("AutoRoute(%s) returned %d approvers, want %d", tt.riskLevel, len(approvers), tt.wantLen)
		}
		if tt.wantLen > 0 && approvers[0] != tt.wantRole {
			t.Errorf("AutoRoute(%s) approver = %s, want %s", tt.riskLevel, approvers[0], tt.wantRole)
		}
	}
}

// TestReject_StatusTransition 测试拒绝后状态流转。
func TestReject_StatusTransition(t *testing.T) {
	changeRepo := newMockChangeRepo()
	approvalRepo := newMockApprovalRepo()
	log := zap.NewNop()

	changeUC := NewChangeUsecase(changeRepo, approvalRepo, log)
	approvalUC := NewApprovalUsecase(changeRepo, approvalRepo, log)

	ctx := context.Background()

	ticket, _ := changeUC.Create(ctx, &CreateChangeReq{
		Title:          "变更测试",
		Type:           ChangeTypeNormal,
		RiskLevel:      RiskHigh,
		AffectedAssets: []string{"asset-001"},
		ScheduledStart: time.Date(2026, 3, 25, 2, 0, 0, 0, time.UTC),
		ScheduledEnd:   time.Date(2026, 3, 25, 4, 0, 0, 0, time.UTC),
	})
	changeUC.Submit(ctx, ticket.ID)

	err := approvalUC.Reject(ctx, ticket.ID, "approver-001", "风险太高")
	if err != nil {
		t.Fatalf("Reject error: %v", err)
	}

	updated := changeRepo.tickets[ticket.ID]
	if updated.Status != StatusRejected {
		t.Errorf("status = %s, want rejected", updated.Status)
	}
}

// TestStartExecution_RequiresApproval 测试未审批不能开始执行。
func TestStartExecution_RequiresApproval(t *testing.T) {
	uc, _, _ := newTestChangeUsecase()
	ctx := context.Background()

	ticket := createTestTicket(uc)

	// 草稿状态不能直接开始执行
	err := uc.StartExecution(ctx, ticket.ID)
	if err == nil {
		t.Fatal("expected error: cannot start execution in draft status")
	}

	// 提交后仍不能开始执行（需要审批通过）
	uc.Submit(ctx, ticket.ID)
	err = uc.StartExecution(ctx, ticket.ID)
	if err == nil {
		t.Fatal("expected error: cannot start execution in pending_approval status")
	}
}

// --- 跨服务集成测试 ---

// mockAuditLogger 记录审计日志调用的 mock 实现。
type mockAuditLogger struct {
	logs []auditLogEntry
}

// auditLogEntry 审计日志记录条目。
type auditLogEntry struct {
	Action     string
	Operator   string
	Resource   string
	ResourceID string
	Detail     string
}

func (m *mockAuditLogger) Log(_ context.Context, action, operator, resource, resourceID, detail string) error {
	m.logs = append(m.logs, auditLogEntry{
		Action:     action,
		Operator:   operator,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     detail,
	})
	return nil
}

// mockMaintenanceClient 记录维护模式调用的 mock 实现。
type mockMaintenanceClient struct {
	enableCalls  []maintenanceEnableCall
	disableCalls [][]string
}

// maintenanceEnableCall 记录 EnableMaintenance 调用参数。
type maintenanceEnableCall struct {
	ResourceIDs []string
	Duration    time.Duration
	Reason      string
}

func (m *mockMaintenanceClient) EnableMaintenance(_ context.Context, resourceIDs []string, duration time.Duration, reason string) error {
	m.enableCalls = append(m.enableCalls, maintenanceEnableCall{
		ResourceIDs: resourceIDs,
		Duration:    duration,
		Reason:      reason,
	})
	return nil
}

func (m *mockMaintenanceClient) DisableMaintenance(_ context.Context, resourceIDs []string) error {
	m.disableCalls = append(m.disableCalls, resourceIDs)
	return nil
}

// TestAuditLogger_CalledOnActions 测试审计日志在各操作后被正确调用。
func TestAuditLogger_CalledOnActions(t *testing.T) {
	repo := newMockChangeRepo()
	approvalRepo := newMockApprovalRepo()
	log := zap.NewNop()
	auditLog := &mockAuditLogger{}

	changeUC := NewChangeUsecase(repo, approvalRepo, log,
		WithAuditLogger(auditLog),
	)
	approvalUC := NewApprovalUsecase(repo, approvalRepo, log,
		WithApprovalAuditLogger(auditLog),
	)

	ctx := context.Background()

	// 创建变更单 → 应记录"创建变更单"
	ticket, err := changeUC.Create(ctx, &CreateChangeReq{
		Title:          "审计测试变更",
		Type:           ChangeTypeNormal,
		RiskLevel:      RiskLow,
		Requester:      "user-audit",
		AffectedAssets: []string{"asset-001"},
		ScheduledStart: time.Date(2026, 3, 25, 2, 0, 0, 0, time.UTC),
		ScheduledEnd:   time.Date(2026, 3, 25, 4, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if len(auditLog.logs) != 1 || auditLog.logs[0].Action != "创建变更单" {
		t.Errorf("expected 1 audit log with action=创建变更单, got %d logs", len(auditLog.logs))
	}

	// 提交审批 → 应记录"提交审批"
	changeUC.Submit(ctx, ticket.ID)
	if len(auditLog.logs) != 2 || auditLog.logs[1].Action != "提交审批" {
		t.Errorf("expected 2 audit logs with action=提交审批, got %d logs", len(auditLog.logs))
	}

	// 审批通过 → 应记录"审批通过"
	approvalUC.Approve(ctx, ticket.ID, "approver-001", "同意")
	if len(auditLog.logs) != 3 || auditLog.logs[2].Action != "审批通过" {
		t.Errorf("expected 3 audit logs with action=审批通过, got %d logs", len(auditLog.logs))
	}

	// 开始执行 → 应记录"开始执行变更"
	changeUC.StartExecution(ctx, ticket.ID)
	if len(auditLog.logs) != 4 || auditLog.logs[3].Action != "开始执行变更" {
		t.Errorf("expected 4 audit logs with action=开始执行变更, got %d logs", len(auditLog.logs))
	}

	// 完成变更 → 应记录"完成变更"
	changeUC.Complete(ctx, ticket.ID)
	if len(auditLog.logs) != 5 || auditLog.logs[4].Action != "完成变更" {
		t.Errorf("expected 5 audit logs with action=完成变更, got %d logs", len(auditLog.logs))
	}
}

// TestMaintenanceMode_TriggeredOnStartExecution 测试开始执行变更时维护模式被正确触发。
func TestMaintenanceMode_TriggeredOnStartExecution(t *testing.T) {
	repo := newMockChangeRepo()
	approvalRepo := newMockApprovalRepo()
	log := zap.NewNop()
	mc := &mockMaintenanceClient{}

	changeUC := NewChangeUsecase(repo, approvalRepo, log,
		WithMaintenanceClient(mc),
	)
	approvalUC := NewApprovalUsecase(repo, approvalRepo, log)

	ctx := context.Background()

	// 创建带有影响资产的变更单
	ticket, _ := changeUC.Create(ctx, &CreateChangeReq{
		Title:          "维护模式测试",
		Type:           ChangeTypeNormal,
		RiskLevel:      RiskLow,
		AffectedAssets: []string{"asset-db-001", "asset-db-002"},
		ScheduledStart: time.Date(2026, 3, 25, 2, 0, 0, 0, time.UTC),
		ScheduledEnd:   time.Date(2026, 3, 25, 4, 0, 0, 0, time.UTC),
	})

	changeUC.Submit(ctx, ticket.ID)
	approvalUC.Approve(ctx, ticket.ID, "approver-001", "")

	// 开始执行 → 应触发 EnableMaintenance
	err := changeUC.StartExecution(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("StartExecution error: %v", err)
	}

	if len(mc.enableCalls) != 1 {
		t.Fatalf("expected 1 EnableMaintenance call, got %d", len(mc.enableCalls))
	}
	if len(mc.enableCalls[0].ResourceIDs) != 2 {
		t.Errorf("expected 2 resource IDs, got %d", len(mc.enableCalls[0].ResourceIDs))
	}

	// 验证维护时长 = 计划结束 - 计划开始 + 30 分钟 = 2h + 30m = 2h30m
	expectedDuration := 2*time.Hour + 30*time.Minute
	if mc.enableCalls[0].Duration != expectedDuration {
		t.Errorf("expected duration=%v, got %v", expectedDuration, mc.enableCalls[0].Duration)
	}

	// 完成变更 → 应触发 DisableMaintenance
	err = changeUC.Complete(ctx, ticket.ID)
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}

	if len(mc.disableCalls) != 1 {
		t.Fatalf("expected 1 DisableMaintenance call, got %d", len(mc.disableCalls))
	}
	if len(mc.disableCalls[0]) != 2 {
		t.Errorf("expected 2 resource IDs in disable call, got %d", len(mc.disableCalls[0]))
	}
}
