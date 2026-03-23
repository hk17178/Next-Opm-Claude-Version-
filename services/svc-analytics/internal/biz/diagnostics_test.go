package biz

import (
	"context"
	"testing"
)

// --- 测试用例 ---

// TestCollectDiagnostics_Success 验证诊断包收集成功（模拟 ObjectStorage）。
func TestCollectDiagnostics_Success(t *testing.T) {
	storage := newMockObjectStorage()
	uc := NewDiagnosticsUsecase(storage, nil, []ServiceEndpoint{})

	bundle, err := uc.CollectDiagnostics(context.Background(), "系统运行缓慢需要排查")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if bundle == nil {
		t.Fatal("expected non-nil bundle")
	}
	if bundle.ID == "" {
		t.Error("expected non-empty bundle ID")
	}
	if bundle.Version != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %s", bundle.Version)
	}
	if bundle.Summary != "系统运行缓慢需要排查" {
		t.Errorf("expected summary=系统运行缓慢需要排查, got %s", bundle.Summary)
	}

	// 验证 gzip 文件已上传到 MinIO
	key := "diagnostics/diagnostics/" + bundle.ID + ".json.gz"
	if _, ok := storage.files[key]; !ok {
		t.Error("expected diagnostic bundle to be uploaded to MinIO")
	}
}

// TestCollectDiagnostics_SectionCount 验证诊断包包含 4 个段落。
func TestCollectDiagnostics_SectionCount(t *testing.T) {
	uc := NewDiagnosticsUsecase(nil, nil, []ServiceEndpoint{
		{Name: "svc-alert", URL: "http://localhost:9999/health"},
	})

	bundle, err := uc.CollectDiagnostics(context.Background(), "测试段落数量")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 应包含 4 个段落：system_info, service_health, recent_errors, config_summary
	if len(bundle.Sections) != 4 {
		t.Errorf("expected 4 sections, got %d", len(bundle.Sections))
	}

	// 验证段落名称
	expectedNames := map[string]bool{
		"system_info":    true,
		"service_health": true,
		"recent_errors":  true,
		"config_summary": true,
	}
	for _, s := range bundle.Sections {
		if !expectedNames[s.Name] {
			t.Errorf("unexpected section name: %s", s.Name)
		}
		if s.Content == "" {
			t.Errorf("section %s has empty content", s.Name)
		}
	}
}

// TestGetDiagnosticsDownloadURL 验证下载 URL 生成。
func TestGetDiagnosticsDownloadURL(t *testing.T) {
	storage := newMockObjectStorage()
	uc := NewDiagnosticsUsecase(storage, nil, []ServiceEndpoint{})

	ctx := context.Background()

	// 先收集一个诊断包
	bundle, err := uc.CollectDiagnostics(ctx, "测试下载链接")
	if err != nil {
		t.Fatalf("CollectDiagnostics error: %v", err)
	}

	// 生成下载 URL
	url, err := uc.GetDiagnosticsDownloadURL(ctx, bundle.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if url == "" {
		t.Error("expected non-empty download URL")
	}

	// 空 ID 应返回错误
	_, err = uc.GetDiagnosticsDownloadURL(ctx, "")
	if err == nil {
		t.Error("expected error for empty id, got nil")
	}
}
