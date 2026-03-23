package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// PrometheusDiscoveryConfig 保存 Prometheus 自动发现的配置信息。
type PrometheusDiscoveryConfig struct {
	PrometheusURL string        // Prometheus 服务地址，如 http://prometheus:9090
	Interval      time.Duration // 拉取间隔
	Enabled       bool
}

// PrometheusDiscoverer 从 Prometheus targets API 自动发现主机并同步到 CMDB。
type PrometheusDiscoverer struct {
	config    PrometheusDiscoveryConfig
	assets    AssetRepo
	discovery DiscoveryRepo
	client    *http.Client
	logger    *zap.Logger
}

// NewPrometheusDiscoverer 创建 Prometheus 自动发现实例。
func NewPrometheusDiscoverer(
	config PrometheusDiscoveryConfig,
	assets AssetRepo,
	discovery DiscoveryRepo,
	logger *zap.Logger,
) *PrometheusDiscoverer {
	return &PrometheusDiscoverer{
		config:    config,
		assets:    assets,
		discovery: discovery,
		client:    &http.Client{Timeout: 30 * time.Second},
		logger:    logger.Named("prometheus-discovery"),
	}
}

// prometheusTargetsResponse 对应 Prometheus /api/v1/targets 的响应格式。
type prometheusTargetsResponse struct {
	Status string `json:"status"`
	Data   struct {
		ActiveTargets []prometheusTarget `json:"activeTargets"`
	} `json:"data"`
}

// prometheusTarget 表示 Prometheus 中一个活跃的监控目标。
type prometheusTarget struct {
	Labels         map[string]string `json:"labels"`
	DiscoveredLabels map[string]string `json:"discoveredLabels"`
	ScrapeURL      string            `json:"scrapeUrl"`
	Health         string            `json:"health"` // up/down/unknown
}

// Start 启动后台定时拉取任务，应以 go discoverer.Start(ctx) 方式调用。
func (d *PrometheusDiscoverer) Start(ctx context.Context) {
	if !d.config.Enabled {
		d.logger.Info("prometheus discovery is disabled")
		return
	}

	interval := d.config.Interval
	if interval == 0 {
		interval = 5 * time.Minute
	}

	d.logger.Info("prometheus discovery started",
		zap.String("prometheus_url", d.config.PrometheusURL),
		zap.Duration("interval", interval),
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 启动时立即执行一次
	d.syncTargets(ctx)

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("prometheus discovery stopped")
			return
		case <-ticker.C:
			d.syncTargets(ctx)
		}
	}
}

// syncTargets 从 Prometheus targets API 拉取当前所有活跃目标，并与 CMDB 同步。
func (d *PrometheusDiscoverer) syncTargets(ctx context.Context) {
	targets, err := d.fetchTargets(ctx)
	if err != nil {
		d.logger.Error("failed to fetch prometheus targets", zap.Error(err))
		return
	}

	d.logger.Info("fetched prometheus targets", zap.Int("count", len(targets)))

	for _, target := range targets {
		if err := d.processTarget(ctx, target); err != nil {
			d.logger.Error("failed to process target",
				zap.String("scrape_url", target.ScrapeURL),
				zap.Error(err),
			)
		}
	}
}

// fetchTargets 调用 Prometheus /api/v1/targets 接口获取活跃目标列表。
func (d *PrometheusDiscoverer) fetchTargets(ctx context.Context) ([]prometheusTarget, error) {
	url := d.config.PrometheusURL + "/api/v1/targets"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch targets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result prometheusTargetsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus returned status: %s", result.Status)
	}

	return result.Data.ActiveTargets, nil
}

// processTarget 处理单个 Prometheus 目标，实现以下逻辑：
//  1. 从 target labels 中提取 hostname 和 IP（以 __address__ 中的 host 部分为 IP 优先来源）。
//  2. 检查 CMDB 中是否已有该主机（通过 hostname/IP 匹配）：
//     - 已存在：执行 upsert 更新（刷新 discovered_by、asset_type 等标签信息）。
//     - 不存在：创建一条 pending 状态的发现记录，等待人工审核后转为正式资产。
//  3. 以 IP 为唯一键进行去重：当 IP 非空时优先以 IP 为匹配条件。
//
// 参数：
//   - ctx: 请求上下文。
//   - target: 来自 Prometheus /api/v1/targets 的单个活跃 target。
func (d *PrometheusDiscoverer) processTarget(ctx context.Context, target prometheusTarget) error {
	// 提取主机名和 IP
	// instance label 通常是 host:port 格式，如 "192.168.1.1:9100"
	hostname := target.Labels["instance"]
	ip := ""

	// 如果 labels 中有明确的 hostname 标签，优先使用
	if h, ok := target.Labels["hostname"]; ok && h != "" {
		hostname = h
	}

	// 从 __address__ 或 instance 中提取纯 IP（去掉 :port）
	if addr, ok := target.Labels["__address__"]; ok && addr != "" {
		if h := splitHostPort(addr); h != "" {
			ip = h
		}
	}
	// 如果 __address__ 不可用，尝试从 instance 中解析 IP
	if ip == "" && hostname != "" {
		if h := splitHostPort(hostname); h != "" {
			ip = h
		}
	}

	// hostname 和 IP 都为空时无法定位资产，跳过
	if hostname == "" && ip == "" {
		d.logger.Debug("skipping prometheus target: no hostname or ip",
			zap.String("scrape_url", target.ScrapeURL),
		)
		return nil
	}

	// 推断资产类型（根据 job label 判断）
	detectedType := inferAssetType(target.Labels)

	// 查询 CMDB 中是否已存在该资产
	existing, err := d.assets.FindByHostnameOrIP(ctx, hostname, ip)
	if err != nil {
		// 查询失败时记录警告但不阻断流程
		d.logger.Warn("failed to find existing asset, will create discovery record",
			zap.String("hostname", hostname),
			zap.String("ip", ip),
			zap.Error(err),
		)
	}

	if existing != nil {
		// 资产已存在：执行 upsert 更新操作
		// 更新内容：asset_type（如推断结果与当前不同）、discovered_by 标记
		return d.upsertExistingAsset(ctx, existing, detectedType, target.Labels)
	}

	// 资产不存在：创建 pending 状态的发现记录，等待人工审核
	// 不直接创建资产，是为了避免误录入，需人工确认后通过 ApproveDiscovery 转为正式资产
	discoveryMethod := "prometheus"
	record := &DiscoveryRecord{
		DiscoveryMethod: discoveryMethod,
		Hostname:        strPtr(hostname),
		IP:              strPtr(ip),
		DetectedType:    strPtr(detectedType),
		Status:          DiscoveryStatusPending,
		RawData:         target.Labels, // 保存完整的 Prometheus labels 便于审核时查看
	}

	if err := d.discovery.Create(ctx, record); err != nil {
		return fmt.Errorf("create discovery record for %s/%s: %w", hostname, ip, err)
	}

	d.logger.Info("created pending discovery record from prometheus target",
		zap.String("hostname", hostname),
		zap.String("ip", ip),
		zap.String("detected_type", detectedType),
	)

	return nil
}

// upsertExistingAsset 对 CMDB 中已存在的资产执行更新操作。
// 仅在以下字段需要变更时才调用 Update，避免无谓写入：
//   - asset_type：若 Prometheus 推断的类型与当前不同（说明服务角色发生了变化）
//   - discovered_by：将来源刷新为 "prometheus"（标记该资产已被 Prometheus 监控覆盖）
//
// 设计考量：
//   - 不覆盖 hostname、IP、grade、business_units 等由人工维护的字段
//   - 更新后发布 asset.changed 事件由 AssetUsecase.Update 完成；此处直接调用底层
//     AssetRepo.Update 以避免循环依赖，同时保持发现模块的轻量化
//
// 参数：
//   - ctx: 请求上下文。
//   - asset: CMDB 中已存在的资产对象。
//   - detectedType: 本次 Prometheus 推断的资产类型。
//   - labels: 完整的 Prometheus target labels，用于日志记录。
func (d *PrometheusDiscoverer) upsertExistingAsset(ctx context.Context, asset *Asset, detectedType string, labels map[string]string) error {
	needsUpdate := false
	prometheusSource := "prometheus"

	// 条件 1：资产类型与推断结果不一致时更新
	if detectedType != "" && asset.AssetType != detectedType {
		d.logger.Info("updating asset_type via prometheus discovery",
			zap.String("asset_id", asset.AssetID),
			zap.String("old_type", asset.AssetType),
			zap.String("new_type", detectedType),
		)
		asset.AssetType = detectedType
		needsUpdate = true
	}

	// 条件 2：discovered_by 不是 prometheus 时更新（标记该资产已被纳入 Prometheus 监控）
	if asset.DiscoveredBy == nil || *asset.DiscoveredBy != prometheusSource {
		asset.DiscoveredBy = &prometheusSource
		needsUpdate = true
	}

	if !needsUpdate {
		// 无需更新，跳过写操作
		d.logger.Debug("asset already up-to-date, skipping upsert",
			zap.String("asset_id", asset.AssetID),
		)
		return nil
	}

	if err := d.assets.Update(ctx, asset); err != nil {
		return fmt.Errorf("upsert asset %s from prometheus: %w", asset.AssetID, err)
	}

	d.logger.Info("upserted existing asset from prometheus discovery",
		zap.String("asset_id", asset.AssetID),
		zap.String("detected_type", detectedType),
	)

	return nil
}

// splitHostPort 从 host:port 字符串中提取 host 部分。
func splitHostPort(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// inferAssetType 根据 Prometheus labels 推断资产类型。
func inferAssetType(labels map[string]string) string {
	if job, ok := labels["job"]; ok {
		switch {
		case contains(job, "node"):
			return AssetTypeServer
		case contains(job, "mysql"), contains(job, "postgres"), contains(job, "mongo"), contains(job, "redis"):
			return AssetTypeDatabase
		case contains(job, "nginx"), contains(job, "haproxy"):
			return AssetTypeLoadBalancer
		case contains(job, "kubernetes"), contains(job, "kube"):
			return AssetTypeK8sCluster
		case contains(job, "kafka"), contains(job, "rabbitmq"), contains(job, "nats"):
			return AssetTypeMessageQueue
		case contains(job, "memcached"), contains(job, "redis"):
			return AssetTypeCache
		}
	}
	return AssetTypeServer
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if matchLower(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func matchLower(a, b string) bool {
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
