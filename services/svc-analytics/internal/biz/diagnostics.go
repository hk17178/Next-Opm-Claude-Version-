// diagnostics.go 实现诊断包收集业务逻辑（FR-29-004）。
// 收集系统信息、服务健康状态、最近错误日志和配置摘要，
// 压缩后存入 MinIO 对象存储，提供限时下载链接。
package biz

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"go.uber.org/zap"
)

// DiagnosticBundle 诊断包实体，包含多个诊断段落。
type DiagnosticBundle struct {
	ID          string        `json:"id"`           // 诊断包 UUID
	CollectedAt time.Time     `json:"collected_at"` // 收集时间
	Version     string        `json:"version"`      // 平台版本号
	Summary     string        `json:"summary"`      // 问题描述（由调用方填写）
	Sections    []DiagSection `json:"sections"`      // 诊断段落列表
}

// DiagSection 诊断包中的一个段落。
type DiagSection struct {
	Name        string    `json:"name"`         // 段落名称：system_info/service_health/recent_errors/config_summary
	Content     string    `json:"content"`      // 段落内容（JSON 或文本）
	CollectedAt time.Time `json:"collected_at"` // 段落收集时间
}

// DiagnosticsRepo 诊断包数据访问接口。
type DiagnosticsRepo interface {
	// Save 保存诊断包记录
	Save(ctx context.Context, bundle *DiagnosticBundle) error
	// Get 根据 ID 获取诊断包记录
	Get(ctx context.Context, id string) (*DiagnosticBundle, error)
}

// ServiceEndpoint 描述一个需要健康检查的服务端点。
type ServiceEndpoint struct {
	Name string // 服务名称
	URL  string // 健康检查 URL
}

// DiagnosticsUsecase 诊断包收集业务逻辑用例。
type DiagnosticsUsecase struct {
	storage    ObjectStorage     // 对象存储（MinIO）
	logger     *zap.Logger       // 日志记录器
	endpoints  []ServiceEndpoint // 需要健康检查的服务端点列表
	httpClient *http.Client      // HTTP 客户端（健康检查用）
	startTime  time.Time         // 进程启动时间
	repo       DiagnosticsRepo   // 诊断包数据访问层（可为 nil，nil 时不持久化）
}

// NewDiagnosticsUsecase 创建诊断包收集业务用例实例。
func NewDiagnosticsUsecase(storage ObjectStorage, logger *zap.Logger, endpoints []ServiceEndpoint) *DiagnosticsUsecase {
	return &DiagnosticsUsecase{
		storage:   storage,
		logger:    logger,
		endpoints: endpoints,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		startTime: time.Now(),
	}
}

// SetRepo 设置诊断包数据访问层（可选，用于持久化记录）。
func (uc *DiagnosticsUsecase) SetRepo(repo DiagnosticsRepo) {
	uc.repo = repo
}

// CollectDiagnostics 收集诊断包（FR-29-004）。
// 收集系统信息、服务健康状态、最近错误日志和配置摘要，
// 序列化为 JSON 并 gzip 压缩后存入 MinIO。
func (uc *DiagnosticsUsecase) CollectDiagnostics(ctx context.Context, summary string) (*DiagnosticBundle, error) {
	bundleID := fmt.Sprintf("diag-%d", time.Now().UnixNano())

	bundle := &DiagnosticBundle{
		ID:          bundleID,
		CollectedAt: time.Now(),
		Version:     "1.0.0", // 硬编码平台版本号
		Summary:     summary,
		Sections:    make([]DiagSection, 0, 4),
	}

	// 收集 system_info：平台版本、Go 版本、启动时间
	bundle.Sections = append(bundle.Sections, uc.collectSystemInfo())

	// 收集 service_health：各服务健康状态
	bundle.Sections = append(bundle.Sections, uc.collectServiceHealth(ctx))

	// 收集 recent_errors：最近错误日志摘要
	bundle.Sections = append(bundle.Sections, uc.collectRecentErrors())

	// 收集 config_summary：配置摘要（不含密码）
	bundle.Sections = append(bundle.Sections, uc.collectConfigSummary())

	// 序列化为 JSON
	bundleJSON, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化诊断包失败: %w", err)
	}

	// Gzip 压缩
	var compressed bytes.Buffer
	gzWriter := gzip.NewWriter(&compressed)
	if _, err := gzWriter.Write(bundleJSON); err != nil {
		return nil, fmt.Errorf("压缩诊断包失败: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return nil, fmt.Errorf("关闭 gzip 写入器失败: %w", err)
	}

	// 上传到 MinIO
	if uc.storage != nil {
		key := fmt.Sprintf("diagnostics/%s.json.gz", bundleID)
		if err := uc.storage.Upload(ctx, "diagnostics", key, compressed.Bytes()); err != nil {
			return nil, fmt.Errorf("上传诊断包到 MinIO 失败: %w", err)
		}
	}

	// 持久化诊断包记录（如果有 repo）
	if uc.repo != nil {
		if err := uc.repo.Save(ctx, bundle); err != nil {
			if uc.logger != nil {
				uc.logger.Warn("保存诊断包记录失败", zap.Error(err))
			}
		}
	}

	if uc.logger != nil {
		uc.logger.Info("诊断包收集完成",
			zap.String("bundle_id", bundleID),
			zap.Int("sections", len(bundle.Sections)),
		)
	}

	return bundle, nil
}

// GetDiagnosticsDownloadURL 生成诊断包的预签名下载 URL（30 分钟有效）。
func (uc *DiagnosticsUsecase) GetDiagnosticsDownloadURL(ctx context.Context, id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("diagnostics id is required")
	}
	if uc.storage == nil {
		return "", fmt.Errorf("object storage not configured")
	}

	key := fmt.Sprintf("diagnostics/%s.json.gz", id)
	url, err := uc.storage.GetPresignedURL(ctx, "diagnostics", key, 30*time.Minute)
	if err != nil {
		return "", fmt.Errorf("生成下载链接失败: %w", err)
	}

	return url, nil
}

// collectSystemInfo 收集系统信息段落。
func (uc *DiagnosticsUsecase) collectSystemInfo() DiagSection {
	info := map[string]string{
		"platform_version": "1.0.0",
		"go_version":       runtime.Version(),
		"os":               runtime.GOOS,
		"arch":             runtime.GOARCH,
		"uptime":           time.Since(uc.startTime).String(),
		"start_time":       uc.startTime.Format(time.RFC3339),
	}
	content, _ := json.Marshal(info)
	return DiagSection{
		Name:        "system_info",
		Content:     string(content),
		CollectedAt: time.Now(),
	}
}

// collectServiceHealth 收集各服务健康状态段落。
// 对每个注册的服务端点执行 HTTP GET /health 检查，超时 2 秒，失败不阻塞。
func (uc *DiagnosticsUsecase) collectServiceHealth(ctx context.Context) DiagSection {
	type healthStatus struct {
		Service string `json:"service"`
		Status  string `json:"status"` // healthy/unhealthy/unreachable
		Latency string `json:"latency,omitempty"`
	}

	var results []healthStatus

	for _, ep := range uc.endpoints {
		start := time.Now()
		status := "healthy"

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.URL, nil)
		if err != nil {
			results = append(results, healthStatus{
				Service: ep.Name,
				Status:  "unreachable",
			})
			continue
		}

		resp, err := uc.httpClient.Do(req)
		latency := time.Since(start)

		if err != nil {
			status = "unreachable"
		} else {
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				status = "unhealthy"
			}
		}

		results = append(results, healthStatus{
			Service: ep.Name,
			Status:  status,
			Latency: latency.String(),
		})
	}

	content, _ := json.Marshal(results)
	return DiagSection{
		Name:        "service_health",
		Content:     string(content),
		CollectedAt: time.Now(),
	}
}

// collectRecentErrors 收集最近错误日志摘要段落。
// 如果有 error buffer 则使用，否则返回占位信息。
func (uc *DiagnosticsUsecase) collectRecentErrors() DiagSection {
	// 当前实现返回占位信息，实际生产中应从 zap 的 error buffer 中获取
	placeholder := map[string]string{
		"message": "错误日志 buffer 未配置，无法获取最近错误",
		"count":   "0",
	}
	content, _ := json.Marshal(placeholder)
	return DiagSection{
		Name:        "recent_errors",
		Content:     string(content),
		CollectedAt: time.Now(),
	}
}

// collectConfigSummary 收集配置摘要段落（不含密码等敏感信息）。
func (uc *DiagnosticsUsecase) collectConfigSummary() DiagSection {
	summary := map[string]string{
		"database":        "configured",
		"redis":           "configured",
		"object_storage":  "configured",
		"service_count":   fmt.Sprintf("%d", len(uc.endpoints)),
	}
	if uc.storage == nil {
		summary["object_storage"] = "not_configured"
	}
	content, _ := json.Marshal(summary)
	return DiagSection{
		Name:        "config_summary",
		Content:     string(content),
		CollectedAt: time.Now(),
	}
}
