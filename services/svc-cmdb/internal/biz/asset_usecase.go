package biz

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/opsnexus/opsnexus/pkg/event"
	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
)

// AssetRepo 是资产持久化的接口，定义资产的增删改查和去重查找操作。
type AssetRepo interface {
	Create(ctx context.Context, a *Asset) error
	GetByID(ctx context.Context, id string) (*Asset, error)
	Update(ctx context.Context, a *Asset) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, f AssetListFilter) ([]*Asset, int64, error)
	FindByHostnameOrIP(ctx context.Context, hostname, ip string) (*Asset, error)
}

// RelationRepo 是资产关系持久化的接口，定义拓扑关系的增删查及级联查询操作。
type RelationRepo interface {
	Create(ctx context.Context, rel *AssetRelation) error
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (*AssetRelation, error)
	ListByAsset(ctx context.Context, assetID, direction string) ([]*AssetRelation, error)
	GetTopology(ctx context.Context, rootID string, depth int, direction string) (*TopologyGraph, error)
	GetCascadeAssets(ctx context.Context, assetIDs []string) ([]string, error)
}

// GroupRepo 是资产分组持久化的接口，定义分组管理和动态成员评估操作。
type GroupRepo interface {
	Create(ctx context.Context, g *AssetGroup) error
	GetByID(ctx context.Context, id string) (*AssetGroup, error)
	List(ctx context.Context) ([]*AssetGroup, error)
	Update(ctx context.Context, g *AssetGroup) error
	Delete(ctx context.Context, id string) error
	EvalDynamicMembers(ctx context.Context, g *AssetGroup) ([]string, error)
}

// DimensionRepo 是自定义维度持久化的接口，定义维度的增删改查和名称唯一性查询。
type DimensionRepo interface {
	Create(ctx context.Context, d *CustomDimension) error
	GetByID(ctx context.Context, id string) (*CustomDimension, error)
	GetByName(ctx context.Context, name string) (*CustomDimension, error)
	List(ctx context.Context) ([]*CustomDimension, error)
	Update(ctx context.Context, d *CustomDimension) error
	Delete(ctx context.Context, id string) error
}

// DiscoveryRepo 是自动发现记录持久化的接口，定义发现记录的增删查和状态变更操作。
type DiscoveryRepo interface {
	Create(ctx context.Context, d *DiscoveryRecord) error
	GetByID(ctx context.Context, id string) (*DiscoveryRecord, error)
	List(ctx context.Context, status string, page, pageSize int) ([]*DiscoveryRecord, int64, error)
	UpdateStatus(ctx context.Context, id, status string, matchedAssetID *string) error
}

// AssetUsecase 编排 CMDB 领域的所有业务操作，协调资产、关系、分组、维度和发现等子模块。
type AssetUsecase struct {
	assets     AssetRepo
	relations  RelationRepo
	groups     GroupRepo
	dimensions DimensionRepo
	discovery  DiscoveryRepo
	producer   *event.Producer
	logger     *zap.Logger
}

// NewAssetUsecase 创建一个新的 AssetUsecase 实例，注入所有依赖的仓储接口、事件生产者和日志器。
func NewAssetUsecase(
	assets AssetRepo,
	relations RelationRepo,
	groups GroupRepo,
	dimensions DimensionRepo,
	discovery DiscoveryRepo,
	producer *event.Producer,
	logger *zap.Logger,
) *AssetUsecase {
	return &AssetUsecase{
		assets:     assets,
		relations:  relations,
		groups:     groups,
		dimensions: dimensions,
		discovery:  discovery,
		producer:   producer,
		logger:     logger,
	}
}

// --- 资产 CRUD ---

// CreateAsset 创建新资产，设置默认值后持久化并发布 asset.changed 事件。
func (uc *AssetUsecase) CreateAsset(ctx context.Context, a *Asset) error {
	if a.Status == "" {
		a.Status = StatusActive
	}
	if a.Tags == nil {
		a.Tags = map[string]string{}
	}
	if a.CustomDimensions == nil {
		a.CustomDimensions = map[string]any{}
	}
	if a.BusinessUnits == nil {
		a.BusinessUnits = []string{}
	}
	if a.DiscoveredBy == nil {
		method := DiscoveryManual
		a.DiscoveredBy = &method
	}

	if err := uc.assets.Create(ctx, a); err != nil {
		return apperrors.Internal("ASSET_CREATE", fmt.Sprintf("failed to create asset: %v", err))
	}

	uc.publishAssetChanged(ctx, a, "created", nil, nil)

	uc.logger.Info("asset created",
		zap.String("asset_id", a.AssetID),
		zap.String("asset_type", a.AssetType),
	)
	return nil
}

// GetAsset 根据资产 ID 查询资产，不存在时返回 NotFound 错误。
func (uc *AssetUsecase) GetAsset(ctx context.Context, id string) (*Asset, error) {
	a, err := uc.assets.GetByID(ctx, id)
	if err != nil {
		return nil, apperrors.Internal("ASSET_GET", fmt.Sprintf("failed to get asset: %v", err))
	}
	if a == nil {
		return nil, apperrors.NotFound(apperrors.ErrAssetNotFound, fmt.Sprintf("asset %s not found", id))
	}
	return a, nil
}

// UpdateAsset 更新已有资产的指定字段，记录变更字段和旧值，并发布对应的变更事件。
func (uc *AssetUsecase) UpdateAsset(ctx context.Context, id string, updates map[string]any) (*Asset, error) {
	a, err := uc.GetAsset(ctx, id)
	if err != nil {
		return nil, err
	}

	changedFields := []string{}
	previousValues := map[string]any{}

	// 逐字段应用更新，同时记录变更前的值用于事件发布
	if v, ok := updates["hostname"]; ok {
		if s, ok := v.(string); ok {
			previousValues["hostname"] = a.Hostname
			a.Hostname = &s
			changedFields = append(changedFields, "hostname")
		}
	}
	if v, ok := updates["ip"]; ok {
		if s, ok := v.(string); ok {
			previousValues["ip"] = a.IP
			a.IP = &s
			changedFields = append(changedFields, "ip")
		}
	}
	if v, ok := updates["asset_type"]; ok {
		if s, ok := v.(string); ok {
			previousValues["asset_type"] = a.AssetType
			a.AssetType = s
			changedFields = append(changedFields, "asset_type")
		}
	}
	if v, ok := updates["grade"]; ok {
		if s, ok := v.(string); ok {
			previousValues["grade"] = a.Grade
			a.Grade = &s
			changedFields = append(changedFields, "grade")
		}
	}
	if v, ok := updates["status"]; ok {
		if s, ok := v.(string); ok {
			previousValues["status"] = a.Status
			a.Status = s
			changedFields = append(changedFields, "status")
		}
	}
	if v, ok := updates["environment"]; ok {
		if s, ok := v.(string); ok {
			previousValues["environment"] = a.Environment
			a.Environment = &s
			changedFields = append(changedFields, "environment")
		}
	}
	if v, ok := updates["region"]; ok {
		if s, ok := v.(string); ok {
			previousValues["region"] = a.Region
			a.Region = &s
			changedFields = append(changedFields, "region")
		}
	}
	if v, ok := updates["tags"]; ok {
		if m, ok := v.(map[string]any); ok {
			tags := map[string]string{}
			for k, val := range m {
				tags[k] = fmt.Sprint(val)
			}
			previousValues["tags"] = a.Tags
			a.Tags = tags
			changedFields = append(changedFields, "tags")
		}
	}
	if v, ok := updates["business_units"]; ok {
		if arr, ok := v.([]any); ok {
			bus := make([]string, len(arr))
			for i, item := range arr {
				bus[i] = fmt.Sprint(item)
			}
			previousValues["business_units"] = a.BusinessUnits
			a.BusinessUnits = bus
			changedFields = append(changedFields, "business_units")
		}
	}
	if v, ok := updates["custom_dimensions"]; ok {
		if m, ok := v.(map[string]any); ok {
			previousValues["custom_dimensions"] = a.CustomDimensions
			a.CustomDimensions = m
			changedFields = append(changedFields, "custom_dimensions")
		}
	}

	if err := uc.assets.Update(ctx, a); err != nil {
		return nil, apperrors.Internal("ASSET_UPDATE", fmt.Sprintf("failed to update asset: %v", err))
	}

	// 等级或状态变更使用更具体的事件类型，便于下游精确订阅
	changeType := "updated"
	for _, f := range changedFields {
		if f == "grade" {
			changeType = "grade_changed"
			break
		}
		if f == "status" {
			changeType = "status_changed"
			break
		}
	}

	uc.publishAssetChanged(ctx, a, changeType, changedFields, previousValues)

	return a, nil
}

// DeleteAsset 删除指定资产并发布 asset.changed(deleted) 事件。
func (uc *AssetUsecase) DeleteAsset(ctx context.Context, id string) error {
	a, err := uc.GetAsset(ctx, id)
	if err != nil {
		return err
	}
	if err := uc.assets.Delete(ctx, id); err != nil {
		return apperrors.Internal("ASSET_DELETE", fmt.Sprintf("failed to delete asset: %v", err))
	}
	uc.publishAssetChanged(ctx, a, "deleted", nil, nil)
	return nil
}

// ListAssets 根据过滤条件返回分页的资产列表。
func (uc *AssetUsecase) ListAssets(ctx context.Context, f AssetListFilter) ([]*Asset, int64, error) {
	return uc.assets.List(ctx, f)
}

// --- 拓扑关系 ---

// CreateRelation 创建资产关系，创建前会校验源和目标资产是否存在。
func (uc *AssetUsecase) CreateRelation(ctx context.Context, rel *AssetRelation) error {
	// 校验源和目标资产均存在
	if _, err := uc.GetAsset(ctx, rel.SourceAssetID); err != nil {
		return err
	}
	if _, err := uc.GetAsset(ctx, rel.TargetAssetID); err != nil {
		return err
	}
	if err := uc.relations.Create(ctx, rel); err != nil {
		return apperrors.Internal("RELATION_CREATE", fmt.Sprintf("failed to create relation: %v", err))
	}
	return nil
}

// DeleteRelation 根据关系 ID 删除一条资产关系。
func (uc *AssetUsecase) DeleteRelation(ctx context.Context, id string) error {
	return uc.relations.Delete(ctx, id)
}

// GetRelationsByAsset 根据资产 ID 和方向（upstream/downstream/both）返回关联的拓扑关系。
func (uc *AssetUsecase) GetRelationsByAsset(ctx context.Context, assetID, direction string) ([]*AssetRelation, error) {
	return uc.relations.ListByAsset(ctx, assetID, direction)
}

// GetTopology 从根资产出发，按指定深度和方向查询拓扑关系图。
func (uc *AssetUsecase) GetTopology(ctx context.Context, rootID string, depth int, direction string) (*TopologyGraph, error) {
	if _, err := uc.GetAsset(ctx, rootID); err != nil {
		return nil, err
	}
	return uc.relations.GetTopology(ctx, rootID, depth, direction)
}

// --- 资产分组 ---

// CreateGroup 创建新的资产分组，创建后立即评估动态成员并缓存成员数。
func (uc *AssetUsecase) CreateGroup(ctx context.Context, g *AssetGroup) error {
	if g.StaticMembers == nil {
		g.StaticMembers = []string{}
	}
	if err := uc.groups.Create(ctx, g); err != nil {
		return apperrors.Internal("GROUP_CREATE", fmt.Sprintf("failed to create group: %v", err))
	}
	// 评估动态成员并缓存成员数量
	members, _ := uc.groups.EvalDynamicMembers(ctx, g)
	g.MemberCount = len(members)
	uc.groups.Update(ctx, g)
	return nil
}

// GetGroup 根据分组 ID 查询资产分组，不存在时返回 NotFound 错误。
func (uc *AssetUsecase) GetGroup(ctx context.Context, id string) (*AssetGroup, error) {
	g, err := uc.groups.GetByID(ctx, id)
	if err != nil {
		return nil, apperrors.Internal("GROUP_GET", fmt.Sprintf("failed to get group: %v", err))
	}
	if g == nil {
		return nil, apperrors.NotFound(apperrors.ErrAssetGroupNotFound, fmt.Sprintf("asset group %s not found", id))
	}
	return g, nil
}

// ListGroups 返回所有资产分组列表。
func (uc *AssetUsecase) ListGroups(ctx context.Context) ([]*AssetGroup, error) {
	return uc.groups.List(ctx)
}

// GetGroupMembers 实时评估并返回分组的当前成员列表（动态分组每次重新计算）。
func (uc *AssetUsecase) GetGroupMembers(ctx context.Context, id string) ([]string, error) {
	g, err := uc.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	return uc.groups.EvalDynamicMembers(ctx, g)
}

// DeleteGroup 根据分组 ID 删除资产分组。
func (uc *AssetUsecase) DeleteGroup(ctx context.Context, id string) error {
	return uc.groups.Delete(ctx, id)
}

// --- 自定义维度 ---

// CreateDimension 创建新的自定义维度，名称唯一，重复时返回 Conflict 错误。
func (uc *AssetUsecase) CreateDimension(ctx context.Context, d *CustomDimension) error {
	existing, _ := uc.dimensions.GetByName(ctx, d.Name)
	if existing != nil {
		return apperrors.Conflict(apperrors.ErrDimensionAlreadyExists,
			fmt.Sprintf("dimension %s already exists", d.Name))
	}
	if err := uc.dimensions.Create(ctx, d); err != nil {
		return apperrors.Internal("DIMENSION_CREATE", fmt.Sprintf("failed to create dimension: %v", err))
	}
	return nil
}

// ListDimensions 返回所有自定义维度列表。
func (uc *AssetUsecase) ListDimensions(ctx context.Context) ([]*CustomDimension, error) {
	return uc.dimensions.List(ctx)
}

// DeleteDimension 根据维度 ID 删除自定义维度。
func (uc *AssetUsecase) DeleteDimension(ctx context.Context, id string) error {
	return uc.dimensions.Delete(ctx, id)
}

// --- 自动发现 ---

// ListDiscoveryRecords 根据状态过滤返回分页的自动发现记录列表。
func (uc *AssetUsecase) ListDiscoveryRecords(ctx context.Context, status string, page, pageSize int) ([]*DiscoveryRecord, int64, error) {
	return uc.discovery.List(ctx, status, page, pageSize)
}

// ApproveDiscovery 审核通过一条发现记录：如果已有匹配资产则关联，否则创建新资产。
func (uc *AssetUsecase) ApproveDiscovery(ctx context.Context, recordID string) (*Asset, error) {
	rec, err := uc.discovery.GetByID(ctx, recordID)
	if err != nil {
		return nil, apperrors.Internal("DISCOVERY_GET", fmt.Sprintf("failed to get record: %v", err))
	}
	if rec == nil {
		return nil, apperrors.NotFound(apperrors.ErrDiscoveryRecordNotFound, "discovery record not found")
	}

	// 通过主机名或 IP 检查是否已存在匹配的资产
	hostname := ""
	ip := ""
	if rec.Hostname != nil {
		hostname = *rec.Hostname
	}
	if rec.IP != nil {
		ip = *rec.IP
	}

	existing, _ := uc.assets.FindByHostnameOrIP(ctx, hostname, ip)
	if existing != nil {
		// 已有匹配资产，直接关联
		if err := uc.discovery.UpdateStatus(ctx, recordID, DiscoveryStatusApproved, &existing.AssetID); err != nil {
			return nil, apperrors.Internal("DISCOVERY_UPDATE", fmt.Sprintf("failed to update record: %v", err))
		}
		return existing, nil
	}

	// 无匹配资产，根据发现记录创建新资产
	assetType := "server"
	if rec.DetectedType != nil {
		assetType = *rec.DetectedType
	}
	a := &Asset{
		Hostname:         rec.Hostname,
		IP:               rec.IP,
		AssetType:        assetType,
		Grade:            rec.DetectedGrade,
		Status:           StatusActive,
		DiscoveredBy:     &rec.DiscoveryMethod,
		Tags:             map[string]string{},
		BusinessUnits:    []string{},
		CustomDimensions: map[string]any{},
	}
	if err := uc.CreateAsset(ctx, a); err != nil {
		return nil, err
	}

	uc.discovery.UpdateStatus(ctx, recordID, DiscoveryStatusApproved, &a.AssetID)

	return a, nil
}

// IgnoreDiscovery 将发现记录标记为已忽略。
func (uc *AssetUsecase) IgnoreDiscovery(ctx context.Context, recordID string) error {
	return uc.discovery.UpdateStatus(ctx, recordID, DiscoveryStatusIgnored, nil)
}

// BlacklistDiscovery 将发现记录加入黑名单，后续相同特征的发现将自动过滤。
func (uc *AssetUsecase) BlacklistDiscovery(ctx context.Context, recordID string) error {
	return uc.discovery.UpdateStatus(ctx, recordID, DiscoveryStatusBlacklisted, nil)
}

// --- 维护窗口 ---

// GetCascadeAssets 返回指定资产进入维护时需要级联抑制告警的所有关联资产。
func (uc *AssetUsecase) GetCascadeAssets(ctx context.Context, assetIDs []string) ([]string, error) {
	return uc.relations.GetCascadeAssets(ctx, assetIDs)
}

// --- 事件发布 ---

// publishAssetChanged 构造并发布 asset.changed 事件，包含变更类型、变更字段和旧值。
// 若 Producer 未配置（为 nil）则跳过发布，避免空指针 panic。
func (uc *AssetUsecase) publishAssetChanged(ctx context.Context, a *Asset, changeType string, changedFields []string, previousValues map[string]any) {
	// BUG-002 修复：Producer 未配置时（如单元测试场景），直接返回，防止 nil 指针 panic。
	if uc.producer == nil {
		return
	}
	data := map[string]any{
		"asset_id":        a.AssetID,
		"change_type":     changeType,
		"hostname":        a.Hostname,
		"ip":              a.IP,
		"asset_type":      a.AssetType,
		"business_units":  a.BusinessUnits,
		"environment":     a.Environment,
		"region":          a.Region,
		"grade":           a.Grade,
		"discovered_by":   a.DiscoveredBy,
		"changed_fields":  changedFields,
		"previous_values": previousValues,
	}

	evt, err := event.NewCloudEvent(event.TypeAssetChanged, "svc-cmdb", data)
	if err != nil {
		uc.logger.Error("failed to create asset.changed event", zap.Error(err))
		return
	}
	if err := uc.producer.Publish(ctx, event.TopicAssetChanged, evt); err != nil {
		uc.logger.Error("failed to publish asset.changed event", zap.Error(err))
	}
}

// PublishMaintenanceEvent 发布 asset.maintenance 事件，通知告警服务进行维护期间的告警抑制。
// 若 Producer 未配置（为 nil）则跳过发布，避免空指针 panic。
func (uc *AssetUsecase) PublishMaintenanceEvent(ctx context.Context, mw *MaintenanceWindow, action string) {
	// BUG-002 修复：Producer 未配置时直接返回，防止 nil 指针 panic。
	if uc.producer == nil {
		return
	}
	cascadeAssets, _ := uc.GetCascadeAssets(ctx, mw.Assets)

	data := map[string]any{
		"maintenance_id": mw.MWID,
		"action":         action,
		"name":           mw.Name,
		"assets":         mw.Assets,
		"asset_groups":   mw.AssetGroups,
		"cascade_assets": cascadeAssets,
		"start_time":     mw.StartTime.Format(time.RFC3339),
		"end_time":       mw.EndTime.Format(time.RFC3339),
		"change_order_id": mw.ChangeOrderID,
		"created_by":     mw.CreatedBy,
	}

	evt, err := event.NewCloudEvent(event.TypeAssetMaintenance, "svc-cmdb", data)
	if err != nil {
		uc.logger.Error("failed to create asset.maintenance event", zap.Error(err))
		return
	}
	if err := uc.producer.Publish(ctx, event.TopicAssetMaintenance, evt); err != nil {
		uc.logger.Error("failed to publish asset.maintenance event", zap.Error(err))
	}
}
