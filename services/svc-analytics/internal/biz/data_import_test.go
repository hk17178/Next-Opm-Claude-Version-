package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// --- Mock 导入任务仓储 ---

// mockImportTaskRepo 是 DataImportRepo 的内存模拟实现。
type mockImportTaskRepo struct {
	tasks  map[string]*ImportTask
	nextID int
}

func newMockImportTaskRepo() *mockImportTaskRepo {
	return &mockImportTaskRepo{
		tasks: make(map[string]*ImportTask),
	}
}

func (m *mockImportTaskRepo) CreateTask(_ context.Context, task *ImportTask) error {
	m.nextID++
	task.ID = fmt.Sprintf("import-%d", m.nextID)
	m.tasks[task.ID] = task
	return nil
}

func (m *mockImportTaskRepo) GetTask(_ context.Context, id string) (*ImportTask, error) {
	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return task, nil
}

func (m *mockImportTaskRepo) UpdateTask(_ context.Context, task *ImportTask) error {
	m.tasks[task.ID] = task
	return nil
}

// --- 测试用例 ---

// TestCreateImportTask_Success 验证正常创建导入任务。
func TestCreateImportTask_Success(t *testing.T) {
	repo := newMockImportTaskRepo()
	uc := NewDataImportUsecase(repo, nil, nil)

	// 构造有效的 JSON 数据作为 fileURL（测试简化）
	data := []map[string]string{
		{"name": "alert-1", "severity": "critical", "source": "prometheus"},
	}
	fileContent, _ := json.Marshal(data)

	task, err := uc.CreateImportTask(context.Background(), "alert", "json", string(fileContent))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if task.ID == "" {
		t.Error("expected non-empty task ID")
	}
	if task.DataType != "alert" {
		t.Errorf("expected data_type=alert, got %s", task.DataType)
	}
	if task.Format != "json" {
		t.Errorf("expected format=json, got %s", task.Format)
	}
}

// TestCreateImportTask_ValidationErrors 验证参数校验（非法类型/格式）。
func TestCreateImportTask_ValidationErrors(t *testing.T) {
	repo := newMockImportTaskRepo()
	uc := NewDataImportUsecase(repo, nil, nil)

	tests := []struct {
		name     string
		dataType string
		format   string
		fileURL  string
		wantErr  string
	}{
		{
			"非法数据类型",
			"invalid_type", "json", "http://example.com/file.json",
			"invalid data_type: invalid_type, supported: alert, incident, asset, knowledge",
		},
		{
			"非法格式",
			"alert", "xml", "http://example.com/file.xml",
			"invalid format: xml, supported: json, csv",
		},
		{
			"空文件 URL",
			"alert", "json", "",
			"file_url is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.CreateImportTask(context.Background(), tt.dataType, tt.format, tt.fileURL)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error=%q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

// TestGetImportTask 验证查询导入任务状态。
func TestGetImportTask(t *testing.T) {
	repo := newMockImportTaskRepo()
	uc := NewDataImportUsecase(repo, nil, nil)
	ctx := context.Background()

	// 构造 JSON 数据
	data := []map[string]string{
		{"name": "alert-1", "severity": "critical", "source": "prometheus"},
	}
	fileContent, _ := json.Marshal(data)

	created, _ := uc.CreateImportTask(ctx, "alert", "json", string(fileContent))

	task, err := uc.GetImportTask(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if task.ID != created.ID {
		t.Errorf("expected ID=%s, got %s", created.ID, task.ID)
	}

	// 不存在的任务
	_, err = uc.GetImportTask(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task, got nil")
	}
}

// TestProcessImportTask_RowValidation 验证行校验逻辑。
func TestProcessImportTask_RowValidation(t *testing.T) {
	repo := newMockImportTaskRepo()
	uc := NewDataImportUsecase(repo, nil, nil)
	ctx := context.Background()

	// 构造包含有效和无效行的 JSON 数据
	data := []map[string]string{
		{"name": "alert-1", "severity": "critical", "source": "prometheus"}, // 有效
		{"name": "", "severity": "warning", "source": "grafana"},            // 无效：name 为空
		{"name": "alert-3", "severity": "", "source": "zabbix"},             // 无效：severity 为空
	}
	fileContent, _ := json.Marshal(data)

	// 直接创建任务（不通过 CreateImportTask 以避免异步处理干扰）
	task := &ImportTask{
		DataType: "alert",
		Format:   "json",
		FileURL:  string(fileContent),
		Status:   ImportStatusPending,
		Errors:   []string{},
	}
	repo.CreateTask(ctx, task)

	// 同步处理
	err := uc.ProcessImportTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ProcessImportTask error: %v", err)
	}

	// 重新获取任务检查结果
	processed, _ := repo.GetTask(ctx, task.ID)
	if processed.TotalRows != 3 {
		t.Errorf("expected total_rows=3, got %d", processed.TotalRows)
	}
	if processed.ImportedRows != 1 {
		t.Errorf("expected imported_rows=1, got %d", processed.ImportedRows)
	}
	if processed.FailedRows != 2 {
		t.Errorf("expected failed_rows=2, got %d", processed.FailedRows)
	}
	if processed.Status != ImportStatusCompleted {
		t.Errorf("expected status=completed, got %s", processed.Status)
	}
}

// TestValidateRows 验证各类型的行校验函数。
func TestValidateRows(t *testing.T) {
	// 告警行校验
	if err := validateAlertRow(map[string]string{"name": "a", "severity": "high", "source": "prom"}); err != nil {
		t.Errorf("valid alert row should pass, got %v", err)
	}
	if err := validateAlertRow(map[string]string{"severity": "high", "source": "prom"}); err == nil {
		t.Error("alert row without name should fail")
	}

	// 事件行校验
	if err := validateIncidentRow(map[string]string{"title": "inc", "severity": "high", "status": "open"}); err != nil {
		t.Errorf("valid incident row should pass, got %v", err)
	}
	if err := validateIncidentRow(map[string]string{"title": "", "severity": "high", "status": "open"}); err == nil {
		t.Error("incident row without title should fail")
	}

	// 资产行校验
	if err := validateAssetRow(map[string]string{"name": "srv", "type": "server", "ip_address": "10.0.0.1"}); err != nil {
		t.Errorf("valid asset row should pass, got %v", err)
	}
	if err := validateAssetRow(map[string]string{"name": "srv", "type": "", "ip_address": "10.0.0.1"}); err == nil {
		t.Error("asset row without type should fail")
	}

	// 知识库行校验
	if err := validateKnowledgeRow(map[string]string{"title": "kb", "content": "text", "type": "faq"}); err != nil {
		t.Errorf("valid knowledge row should pass, got %v", err)
	}
	if err := validateKnowledgeRow(map[string]string{"title": "kb", "content": "", "type": "faq"}); err == nil {
		t.Error("knowledge row without content should fail")
	}
}
