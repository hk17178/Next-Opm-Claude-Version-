// Package biz 包含 CMDB 领域的核心业务逻辑，包括资产管理、拓扑关系、分组、自定义维度和自动发现等功能。
package biz

import (
	"time"
)

// 资产类型常量，共 22 种子类型，涵盖计算、网络、存储、中间件、云原生等分类。
const (
	AssetTypeServer        = "server"
	AssetTypeVirtualMachine = "virtual_machine"
	AssetTypeContainer     = "container"
	AssetTypeNetworkDevice = "network_device"
	AssetTypeDatabase      = "database"
	AssetTypeMiddleware    = "middleware"
	AssetTypeApplication   = "application"
	AssetTypeStorage       = "storage"
	AssetTypeLoadBalancer  = "loadbalancer"
	AssetTypeFirewall      = "firewall"
	AssetTypeDNS           = "dns"
	AssetTypeCDN           = "cdn"
	AssetTypeMessageQueue  = "message_queue"
	AssetTypeCache         = "cache"
	AssetTypeMonitoring    = "monitoring"
	AssetTypeCloudInstance = "cloud_instance"
	AssetTypeCloudService  = "cloud_service"
	AssetTypeK8sCluster    = "k8s_cluster"
	AssetTypeK8sPod        = "k8s_pod"
	AssetTypeK8sService    = "k8s_service"
	AssetTypeAPIGateway    = "api_gateway"
	AssetTypeOther         = "other"
)

// 资产等级 S-D，用于衡量资产对业务的重要程度。
const (
	GradeS = "S" // 核心生命线
	GradeA = "A" // 核心业务
	GradeB = "B" // 重要支撑
	GradeC = "C" // 一般
	GradeD = "D" // 非关键
)

// 资产状态常量，表示资产在其生命周期中的当前阶段。
const (
	StatusActive      = "active"      // 运行中
	StatusIdle        = "idle"        // 空闲
	StatusMaintenance = "maintenance" // 维护中
	StatusRetired     = "retired"     // 已下线
)

// 拓扑关系类型常量，用于描述资产之间的依赖和部署关系。
const (
	RelationDependsOn   = "depends_on"   // 依赖于
	RelationDeployedOn  = "deployed_on"  // 部署于
	RelationConnectedTo = "connected_to" // 连接到
	RelationChildOf     = "child_of"     // 子级
)

// 资产分组类型常量。
const (
	GroupTypeDynamic = "dynamic" // 动态分组，根据条件自动匹配
	GroupTypeStatic  = "static"  // 静态分组，手动指定成员
	GroupTypeHybrid  = "hybrid"  // 混合分组，同时支持动态和静态成员
)

// 自动发现方式常量。
const (
	DiscoveryScan      = "scan"       // 网络扫描
	DiscoverySNMP      = "snmp"       // SNMP 协议发现
	DiscoveryCloudAPI  = "cloud_api"  // 云厂商 API
	DiscoveryDHCP      = "dhcp"       // DHCP 日志
	DiscoveryLogSource = "log_source" // 日志源解析
	DiscoveryManual    = "manual"     // 手动录入
)

// 发现记录状态常量，表示自动发现记录的审核流转状态。
const (
	DiscoveryStatusPending     = "pending"     // 待审核
	DiscoveryStatusApproved    = "approved"    // 已审核通过
	DiscoveryStatusIgnored     = "ignored"     // 已忽略
	DiscoveryStatusBlacklisted = "blacklisted" // 已加入黑名单
)

// Asset 表示 CMDB 中的一个配置项（CI），是资产管理的核心实体。
type Asset struct {
	AssetID          string            `json:"asset_id" db:"asset_id"`
	Hostname         *string           `json:"hostname,omitempty" db:"hostname"`
	IP               *string           `json:"ip,omitempty" db:"ip"`
	AssetType        string            `json:"asset_type" db:"asset_type"`
	AssetSubtype     *string           `json:"asset_subtype,omitempty" db:"asset_subtype"`
	BusinessUnits    []string          `json:"business_units" db:"business_units"`
	Organization     *string           `json:"organization,omitempty" db:"organization"`
	Environment      *string           `json:"environment,omitempty" db:"environment"`
	Region           *string           `json:"region,omitempty" db:"region"`
	Datacenter       *string           `json:"datacenter,omitempty" db:"datacenter"`
	Grade            *string           `json:"grade,omitempty" db:"grade"`
	GradeScore       any               `json:"grade_score,omitempty" db:"grade_score"`
	Status           string            `json:"status" db:"status"`
	Tags             map[string]string `json:"tags" db:"tags"`
	CustomDimensions map[string]any    `json:"custom_dimensions" db:"custom_dimensions"`
	DiscoveredBy     *string           `json:"discovered_by,omitempty" db:"discovered_by"`
	CreatedAt        time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at" db:"updated_at"`
}

// AssetRelation 表示两个资产之间的拓扑依赖关系，用于构建资产关系图。
type AssetRelation struct {
	RelationID    string    `json:"relation_id" db:"relation_id"`
	SourceAssetID string    `json:"source_asset_id" db:"source_asset_id"`
	TargetAssetID string    `json:"target_asset_id" db:"target_asset_id"`
	RelationType  string    `json:"relation_type" db:"relation_type"`
	Metadata      any       `json:"metadata,omitempty" db:"metadata"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// AssetGroup 表示资产分组，支持动态（按条件匹配）、静态（手动指定）和混合三种类型。
type AssetGroup struct {
	GroupID       string    `json:"group_id" db:"group_id"`
	Name          string    `json:"name" db:"name"`
	GroupType     string    `json:"group_type" db:"group_type"`
	Conditions    any       `json:"conditions,omitempty" db:"conditions"`
	StaticMembers []string  `json:"static_members" db:"static_members"`
	MemberCount   int       `json:"member_count" db:"member_count"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// CustomDimension 定义用户自建的资产扩展维度，支持枚举、树形、文本、日期、数值、引用等类型。
type CustomDimension struct {
	DimID       string    `json:"dim_id" db:"dim_id"`
	Name        string    `json:"name" db:"name"`
	DisplayName string    `json:"display_name" db:"display_name"`
	DimType     string    `json:"dim_type" db:"dim_type"` // enum/tree/text/date/numeric/reference
	Config      any       `json:"config,omitempty" db:"config"`
	Required    bool      `json:"required" db:"required"`
	Sortable    bool      `json:"sortable" db:"sortable"`
	Filterable  bool      `json:"filterable" db:"filterable"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// DiscoveryRecord 表示自动发现的设备记录，经过审核后可转为正式资产。
type DiscoveryRecord struct {
	RecordID        string    `json:"record_id" db:"record_id"`
	DiscoveryMethod string    `json:"discovery_method" db:"discovery_method"`
	IP              *string   `json:"ip,omitempty" db:"ip"`
	Hostname        *string   `json:"hostname,omitempty" db:"hostname"`
	DetectedType    *string   `json:"detected_type,omitempty" db:"detected_type"`
	DetectedGrade   *string   `json:"detected_grade,omitempty" db:"detected_grade"`
	Status          string    `json:"status" db:"status"`
	MatchedAssetID  *string   `json:"matched_asset_id,omitempty" db:"matched_asset_id"`
	RawData         any       `json:"raw_data,omitempty" db:"raw_data"`
	DiscoveredAt    time.Time `json:"discovered_at" db:"discovered_at"`
}

// TopologyNode 表示拓扑图响应中的一个节点，包含资产信息及其关联关系。
type TopologyNode struct {
	Asset     *Asset          `json:"asset"`
	Relations []*AssetRelation `json:"relations,omitempty"`
}

// TopologyGraph 表示完整的拓扑查询结果，包含所有节点和边。
type TopologyGraph struct {
	Nodes []*Asset         `json:"nodes"`
	Edges []*AssetRelation `json:"edges"`
}

// AssetListFilter 封装资产列表查询的过滤条件和分页排序参数。
type AssetListFilter struct {
	AssetType    *string
	Status       *string
	Grade        *string
	Environment  *string
	BusinessUnit *string
	Search       *string // hostname/IP search
	TagKey       *string
	TagValue     *string
	Page         int
	PageSize     int
	SortBy       string
	SortOrder    string
}

// MaintenanceWindow 表示计划维护窗口，用于在维护期间抑制相关告警。
type MaintenanceWindow struct {
	MWID          string    `json:"mw_id" db:"mw_id"`
	Name          string    `json:"name" db:"name"`
	Status        string    `json:"status" db:"status"`
	StartTime     time.Time `json:"start_time" db:"start_time"`
	EndTime       time.Time `json:"end_time" db:"end_time"`
	Assets        []string  `json:"assets" db:"assets"`
	AssetGroups   []string  `json:"asset_groups" db:"asset_groups"`
	Cascade       bool      `json:"cascade" db:"cascade"`
	ChangeOrderID *string   `json:"change_order_id,omitempty" db:"change_order_id"`
	CreatedBy     *string   `json:"created_by,omitempty" db:"created_by"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}
