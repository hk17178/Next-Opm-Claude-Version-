package biz

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// --- Mock 导出任务仓储 ---

// mockExportTaskRepo 是 ExportTaskRepo 的内存模拟实现。
type mockExportTaskRepo struct {
	tasks  map[string]*ExportTask
	nextID int
}

func newMockExportTaskRepo() *mockExportTaskRepo {
	return &mockExportTaskRepo{
		tasks: make(map[string]*ExportTask),
	}
}

func (m *mockExportTaskRepo) Create(_ context.Context, task *ExportTask) error {
	m.nextID++
	task.ID = fmt.Sprintf("export-%d", m.nextID)
	m.tasks[task.ID] = task
	return nil
}

func (m *mockExportTaskRepo) Get(_ context.Context, id string) (*ExportTask, error) {
	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return task, nil
}

func (m *mockExportTaskRepo) UpdateStatus(_ context.Context, id, status, fileURL, errMsg string) error {
	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("not found: %s", id)
	}
	task.Status = status
	task.FileURL = fileURL
	task.Error = errMsg
	return nil
}

// --- Mock 对象存储 ---

// mockObjectStorage 是 ObjectStorage 的模拟实现。
type mockObjectStorage struct {
	files map[string][]byte
}

func newMockObjectStorage() *mockObjectStorage {
	return &mockObjectStorage{
		files: make(map[string][]byte),
	}
}

func (m *mockObjectStorage) Upload(_ context.Context, bucket, key string, data []byte) error {
	m.files[bucket+"/"+key] = data
	return nil
}

func (m *mockObjectStorage) GetPresignedURL(_ context.Context, bucket, key string, _ time.Duration) (string, error) {
	fullKey := bucket + "/" + key
	if _, ok := m.files[fullKey]; !ok {
		return "", fmt.Errorf("object not found: %s", fullKey)
	}
	return fmt.Sprintf("https://minio.example.com/%s?signed=true", fullKey), nil
}

// --- Mock 指标仓储（用于导出测试） ---

type mockExportMetricsRepo struct{}

func (m *mockExportMetricsRepo) InsertBusinessMetrics(_ context.Context, _ []BusinessMetric) error {
	return nil
}
func (m *mockExportMetricsRepo) InsertResourceMetrics(_ context.Context, _ []ResourceMetric) error {
	return nil
}
func (m *mockExportMetricsRepo) QueryBusinessMetrics(_ context.Context, _ MetricQueryRequest) (*MetricQueryResponse, error) {
	return &MetricQueryResponse{}, nil
}
func (m *mockExportMetricsRepo) QueryResourceMetrics(_ context.Context, _ MetricQueryRequest) (*MetricQueryResponse, error) {
	return &MetricQueryResponse{}, nil
}
func (m *mockExportMetricsRepo) CorrelateMetrics(_ context.Context, _, _, _ string, _, _ time.Time) (*ResourceCorrelation, error) {
	return &ResourceCorrelation{}, nil
}
func (m *mockExportMetricsRepo) ExecuteQuery(_ context.Context, _ string, _ *TimeRange, _ int) (*QueryResponse, error) {
	return &QueryResponse{
		Columns:   []QueryColumn{{Name: "id", Type: "String"}, {Name: "name", Type: "String"}},
		Rows:      [][]any{{"1", "test-alert"}, {"2", "test-incident"}},
		TotalRows: 2,
	}, nil
}

// --- 测试用例 ---

// TestCreateExportTask_Success 验证正常创建导出任务。
func TestCreateExportTask_Success(t *testing.T) {
	repo := newMockExportTaskRepo()
	uc := NewDataExportUsecase(repo, nil, nil, nil)

	task, err := uc.CreateTask(context.Background(), []string{"alerts", "incidents"}, "json", "user-001")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if task.ID == "" {
		t.Error("expected non-empty task ID")
	}
	if task.Status != ExportStatusPending {
		t.Errorf("expected status=pending, got %s", task.Status)
	}
	if task.Format != "json" {
		t.Errorf("expected format=json, got %s", task.Format)
	}
	if len(task.Scope) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(task.Scope))
	}
	if task.ExpiresAt.Before(task.CreatedAt) {
		t.Error("expected expires_at after created_at")
	}
}

// TestCreateExportTask_ValidationErrors 验证参数校验。
func TestCreateExportTask_ValidationErrors(t *testing.T) {
	repo := newMockExportTaskRepo()
	uc := NewDataExportUsecase(repo, nil, nil, nil)

	tests := []struct {
		name    string
		scope   []string
		format  string
		userID  string
		wantErr string
	}{
		{"空范围", nil, "json", "user-1", "scope is required, supported: alerts, incidents, assets, knowledge, audit_logs"},
		{"无效范围", []string{"invalid"}, "json", "user-1", "invalid scope: invalid"},
		{"无效格式", []string{"alerts"}, "xml", "user-1", "invalid format: xml, supported: json, csv"},
		{"空用户", []string{"alerts"}, "json", "", "user_id is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.CreateTask(context.Background(), tt.scope, tt.format, tt.userID)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error=%q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

// TestCreateExportTask_DefaultFormat 验证默认导出格式为 JSON。
func TestCreateExportTask_DefaultFormat(t *testing.T) {
	repo := newMockExportTaskRepo()
	uc := NewDataExportUsecase(repo, nil, nil, nil)

	task, err := uc.CreateTask(context.Background(), []string{"alerts"}, "", "user-001")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if task.Format != "json" {
		t.Errorf("expected default format=json, got %s", task.Format)
	}
}

// TestGetExportTask 验证获取导出任务。
func TestGetExportTask(t *testing.T) {
	repo := newMockExportTaskRepo()
	uc := NewDataExportUsecase(repo, nil, nil, nil)

	ctx := context.Background()
	created, _ := uc.CreateTask(ctx, []string{"alerts"}, "csv", "user-001")

	task, err := uc.GetTask(ctx, created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if task.ID != created.ID {
		t.Errorf("expected ID=%s, got %s", created.ID, task.ID)
	}

	// 不存在的任务
	_, err = uc.GetTask(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task, got nil")
	}
}

// TestProcessExportTask_Success 验证导出任务处理流程。
func TestProcessExportTask_Success(t *testing.T) {
	repo := newMockExportTaskRepo()
	storage := newMockObjectStorage()
	metrics := &mockExportMetricsRepo{}
	uc := NewDataExportUsecase(repo, storage, metrics, nil)

	ctx := context.Background()
	task, _ := uc.CreateTask(ctx, []string{"alerts"}, "json", "user-001")

	err := uc.ProcessTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 验证任务已更新为完成状态
	processed, _ := uc.GetTask(ctx, task.ID)
	if processed.Status != ExportStatusDone {
		t.Errorf("expected status=done, got %s", processed.Status)
	}
	if processed.FileURL == "" {
		t.Error("expected non-empty file URL after processing")
	}
}

// TestProcessExportTask_NotFound 验证处理不存在的任务。
func TestProcessExportTask_NotFound(t *testing.T) {
	repo := newMockExportTaskRepo()
	uc := NewDataExportUsecase(repo, nil, nil, nil)

	err := uc.ProcessTask(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task, got nil")
	}
}

// TestFormatCSV 验证 CSV 格式化。
func TestFormatCSV(t *testing.T) {
	resp := &QueryResponse{
		Columns: []QueryColumn{{Name: "id"}, {Name: "name"}},
		Rows:    [][]any{{"1", "test"}},
	}

	result := formatCSV(resp)
	if len(result) == 0 {
		t.Error("expected non-empty CSV output")
	}

	// 验证 nil 输入
	if formatCSV(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

// TestFormatJSON 验证 JSON 格式化。
func TestFormatJSON(t *testing.T) {
	resp := &QueryResponse{
		Columns: []QueryColumn{{Name: "id"}, {Name: "name"}},
		Rows:    [][]any{{"1", "test"}},
	}

	result := formatJSON(resp)
	if len(result) == 0 {
		t.Error("expected non-empty JSON output")
	}

	// 验证 nil 输入
	if formatJSON(nil) != nil {
		t.Error("expected nil for nil input")
	}
}
