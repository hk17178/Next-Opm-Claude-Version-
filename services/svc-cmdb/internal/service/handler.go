// Package service 提供 CMDB 领域的 HTTP 和 gRPC 接口层，包括资产、关系、分组、维度和自动发现等 API 处理器。
package service

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
	"github.com/opsnexus/opsnexus/pkg/httputil"
	"github.com/opsnexus/svc-cmdb/internal/biz"
)

// Handler 实现 CMDB 领域的 REST API，处理资产、关系、分组、维度、发现和维护窗口等 HTTP 请求。
type Handler struct {
	uc     *biz.AssetUsecase
	logger *zap.Logger
}

// NewHandler 创建一个新的 CMDB HTTP 处理器实例。
func NewHandler(uc *biz.AssetUsecase, logger *zap.Logger) *Handler {
	return &Handler{uc: uc, logger: logger}
}

// RegisterRoutes 在给定的 chi 路由器上注册 CMDB 所有 API 路由。
// 路由路径遵循 docs/api/svc-cmdb.yaml 中的 OpenAPI 契约。
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/cmdb", func(r chi.Router) {
		// 配置项（CI）— OpenAPI /ci 路径
		r.Get("/ci", h.listAssets)
		r.Post("/ci", h.createAsset)
		r.Get("/ci/{ci_id}", h.getAsset)
		r.Put("/ci/{ci_id}", h.updateAsset)
		r.Delete("/ci/{ci_id}", h.deleteAsset)

		// CI 关系 — OpenAPI /ci/{ci_id}/relationships
		r.Get("/ci/{ci_id}/relationships", h.getAssetRelations)
		r.Post("/relationships", h.createRelation)
		r.Delete("/relationships/{relationship_id}", h.deleteRelation)

		// 拓扑图 — OpenAPI /topology
		r.Post("/topology", h.queryTopology)

		// CI 类型 — OpenAPI /ci-types
		r.Get("/ci-types", h.listCITypes)

		// 资产分组（扩展 API）
		r.Get("/groups", h.listGroups)
		r.Post("/groups", h.createGroup)
		r.Get("/groups/{group_id}", h.getGroup)
		r.Get("/groups/{group_id}/members", h.getGroupMembers)
		r.Delete("/groups/{group_id}", h.deleteGroup)

		// 自定义维度（扩展 API）
		r.Get("/dimensions", h.listDimensions)
		r.Post("/dimensions", h.createDimension)
		r.Delete("/dimensions/{dim_id}", h.deleteDimension)

		// 自动发现（扩展 API）
		r.Get("/discovery", h.listDiscoveryRecords)
		r.Post("/discovery/{record_id}/approve", h.approveDiscovery)
		r.Post("/discovery/{record_id}/ignore", h.ignoreDiscovery)
		r.Post("/discovery/{record_id}/blacklist", h.blacklistDiscovery)

		// 维护窗口（扩展 API）
		r.Post("/maintenance", h.createMaintenanceWindow)
	})
}

// --- 资产 ---

// listAssets 处理 GET /ci 请求，返回分页过滤的资产列表。
func (h *Handler) listAssets(w http.ResponseWriter, r *http.Request) {
	p := httputil.ParsePagination(r)
	f := biz.AssetListFilter{
		Page:     p.Page,
		PageSize: p.PageSize,
	}

	if v := r.URL.Query().Get("type"); v != "" {
		f.AssetType = &v
	}
	if v := r.URL.Query().Get("status"); v != "" {
		f.Status = &v
	}
	if v := r.URL.Query().Get("grade"); v != "" {
		f.Grade = &v
	}
	if v := r.URL.Query().Get("environment"); v != "" {
		f.Environment = &v
	}
	if v := r.URL.Query().Get("business_unit"); v != "" {
		f.BusinessUnit = &v
	}
	if v := r.URL.Query().Get("search"); v != "" {
		f.Search = &v
	}
	sortBy, sortOrder := httputil.ParseSortParams(r, map[string]bool{
		"created_at": true, "hostname": true, "ip": true,
		"asset_type": true, "grade": true, "status": true, "updated_at": true,
	}, "created_at")
	f.SortBy = sortBy
	f.SortOrder = sortOrder

	assets, total, err := h.uc.ListAssets(r.Context(), f)
	if err != nil {
		h.handleError(w, err)
		return
	}

	httputil.PagedJSON(w, assets, p.Page, p.PageSize, total)
}

// createAsset 处理 POST /ci 请求，创建新资产。
func (h *Handler) createAsset(w http.ResponseWriter, r *http.Request) {
	var a biz.Asset
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if a.AssetType == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "asset_type is required"))
		return
	}

	if err := h.uc.CreateAsset(r.Context(), &a); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, a)
}

// getAsset 处理 GET /ci/{ci_id} 请求，查询单个资产。
func (h *Handler) getAsset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "ci_id")
	a, err := h.uc.GetAsset(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, a)
}

// updateAsset 处理 PUT /ci/{ci_id} 请求，更新资产字段。
func (h *Handler) updateAsset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "ci_id")
	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}

	a, err := h.uc.UpdateAsset(r.Context(), id, updates)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, a)
}

// deleteAsset 处理 DELETE /ci/{ci_id} 请求，删除资产。
func (h *Handler) deleteAsset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "ci_id")
	if err := h.uc.DeleteAsset(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Relations ---

func (h *Handler) getAssetRelations(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "ci_id")
	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "both"
	}

	relations, err := h.uc.GetRelationsByAsset(r.Context(), id, direction)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"relationships": relations})
}

func (h *Handler) createRelation(w http.ResponseWriter, r *http.Request) {
	var rel biz.AssetRelation
	if err := json.NewDecoder(r.Body).Decode(&rel); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if rel.SourceAssetID == "" || rel.TargetAssetID == "" || rel.RelationType == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "source_asset_id, target_asset_id, and relation_type are required"))
		return
	}

	if err := h.uc.CreateRelation(r.Context(), &rel); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, rel)
}

func (h *Handler) deleteRelation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "relationship_id")
	if err := h.uc.DeleteRelation(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Topology ---

func (h *Handler) queryTopology(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RootAssetID string   `json:"root_ci_id"`
		Depth       int      `json:"depth"`
		Direction   string   `json:"direction"`
		TypeFilter  []string `json:"ci_type_filter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if req.RootAssetID == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "root_ci_id is required"))
		return
	}
	if req.Depth == 0 {
		req.Depth = 3
	}
	if req.Direction == "" {
		req.Direction = "both"
	}

	graph, err := h.uc.GetTopology(r.Context(), req.RootAssetID, req.Depth, req.Direction)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, graph)
}

// --- Groups ---

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.uc.ListGroups(r.Context())
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	var g biz.AssetGroup
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if g.Name == "" || g.GroupType == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "name and group_type are required"))
		return
	}

	if err := h.uc.CreateGroup(r.Context(), &g); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, g)
}

func (h *Handler) getGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "group_id")
	g, err := h.uc.GetGroup(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, g)
}

func (h *Handler) getGroupMembers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "group_id")
	members, err := h.uc.GetGroupMembers(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"members": members, "count": len(members)})
}

func (h *Handler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "group_id")
	if err := h.uc.DeleteGroup(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Dimensions ---

func (h *Handler) listDimensions(w http.ResponseWriter, r *http.Request) {
	dims, err := h.uc.ListDimensions(r.Context())
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"dimensions": dims})
}

func (h *Handler) createDimension(w http.ResponseWriter, r *http.Request) {
	var d biz.CustomDimension
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if d.Name == "" || d.DisplayName == "" || d.DimType == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "name, display_name, and dim_type are required"))
		return
	}

	if err := h.uc.CreateDimension(r.Context(), &d); err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusCreated, d)
}

func (h *Handler) deleteDimension(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "dim_id")
	if err := h.uc.DeleteDimension(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Discovery ---

func (h *Handler) listDiscoveryRecords(w http.ResponseWriter, r *http.Request) {
	p := httputil.ParsePagination(r)
	status := r.URL.Query().Get("status")

	records, total, err := h.uc.ListDiscoveryRecords(r.Context(), status, p.Page, p.PageSize)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.PagedJSON(w, records, p.Page, p.PageSize, total)
}

func (h *Handler) approveDiscovery(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "record_id")
	asset, err := h.uc.ApproveDiscovery(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, asset)
}

func (h *Handler) ignoreDiscovery(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "record_id")
	if err := h.uc.IgnoreDiscovery(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) blacklistDiscovery(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "record_id")
	if err := h.uc.BlacklistDiscovery(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Maintenance Windows ---

func (h *Handler) createMaintenanceWindow(w http.ResponseWriter, r *http.Request) {
	var mw biz.MaintenanceWindow
	if err := json.NewDecoder(r.Body).Decode(&mw); err != nil {
		httputil.Error(w, apperrors.BadRequest("INVALID_JSON", "invalid request body"))
		return
	}
	if mw.Name == "" {
		httputil.Error(w, apperrors.BadRequest("VALIDATION", "name is required"))
		return
	}

	// 发布维护事件用于告警抑制
	h.uc.PublishMaintenanceEvent(r.Context(), &mw, "started")
	httputil.JSON(w, http.StatusCreated, mw)
}

// --- CI Types ---

// ciTypeDefinition 描述一种 CI 类型，用于 /ci-types 端点返回所有支持的类型列表。
type ciTypeDefinition struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

func (h *Handler) listCITypes(w http.ResponseWriter, r *http.Request) {
	types := []ciTypeDefinition{
		{Name: biz.AssetTypeServer, DisplayName: "Server", Description: "Physical or bare-metal server"},
		{Name: biz.AssetTypeVirtualMachine, DisplayName: "Virtual Machine", Description: "VM instance"},
		{Name: biz.AssetTypeContainer, DisplayName: "Container", Description: "Container instance"},
		{Name: biz.AssetTypeNetworkDevice, DisplayName: "Network Device", Description: "Switch, router, or other network equipment"},
		{Name: biz.AssetTypeDatabase, DisplayName: "Database", Description: "Database instance"},
		{Name: biz.AssetTypeMiddleware, DisplayName: "Middleware", Description: "Middleware service"},
		{Name: biz.AssetTypeApplication, DisplayName: "Application", Description: "Application service"},
		{Name: biz.AssetTypeStorage, DisplayName: "Storage", Description: "Storage system"},
		{Name: biz.AssetTypeLoadBalancer, DisplayName: "Load Balancer", Description: "Load balancer"},
		{Name: biz.AssetTypeFirewall, DisplayName: "Firewall", Description: "Firewall device"},
		{Name: biz.AssetTypeDNS, DisplayName: "DNS", Description: "DNS service"},
		{Name: biz.AssetTypeCDN, DisplayName: "CDN", Description: "Content delivery network"},
		{Name: biz.AssetTypeMessageQueue, DisplayName: "Message Queue", Description: "Message queue or event bus"},
		{Name: biz.AssetTypeCache, DisplayName: "Cache", Description: "Cache service (Redis, Memcached)"},
		{Name: biz.AssetTypeMonitoring, DisplayName: "Monitoring", Description: "Monitoring system"},
		{Name: biz.AssetTypeCloudInstance, DisplayName: "Cloud Instance", Description: "Cloud VM or compute instance"},
		{Name: biz.AssetTypeCloudService, DisplayName: "Cloud Service", Description: "Managed cloud service"},
		{Name: biz.AssetTypeK8sCluster, DisplayName: "Kubernetes Cluster", Description: "Kubernetes cluster"},
		{Name: biz.AssetTypeK8sPod, DisplayName: "Kubernetes Pod", Description: "Kubernetes pod"},
		{Name: biz.AssetTypeK8sService, DisplayName: "Kubernetes Service", Description: "Kubernetes service"},
		{Name: biz.AssetTypeAPIGateway, DisplayName: "API Gateway", Description: "API gateway"},
		{Name: biz.AssetTypeOther, DisplayName: "Other", Description: "Other CI type"},
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"types": types})
}

// handleError 将 AppError 映射为对应的 HTTP 错误响应。
func (h *Handler) handleError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apperrors.AppError); ok {
		httputil.Error(w, appErr)
		return
	}
	httputil.Error(w, apperrors.Internal("INTERNAL", err.Error()))
}
