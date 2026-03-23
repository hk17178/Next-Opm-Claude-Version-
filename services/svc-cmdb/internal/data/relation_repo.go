package data

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opsnexus/svc-cmdb/internal/biz"
)

// RelationRepo 实现 biz.RelationRepo 接口，负责资产拓扑关系的数据库持久化操作。
type RelationRepo struct {
	db *pgxpool.Pool
}

// NewRelationRepo 创建一个新的 RelationRepo 实例。
func NewRelationRepo(db *pgxpool.Pool) *RelationRepo {
	return &RelationRepo{db: db}
}

// Create 插入一条新的资产关系记录，回填 relation_id 和 created_at。
func (r *RelationRepo) Create(ctx context.Context, rel *biz.AssetRelation) error {
	metadataJSON, _ := json.Marshal(rel.Metadata)
	err := r.db.QueryRow(ctx,
		`INSERT INTO asset_relations (source_asset_id, target_asset_id, relation_type, metadata)
		 VALUES ($1, $2, $3, $4)
		 RETURNING relation_id, created_at`,
		rel.SourceAssetID, rel.TargetAssetID, rel.RelationType, metadataJSON,
	).Scan(&rel.RelationID, &rel.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert relation: %w", err)
	}
	return nil
}

// Delete 根据关系 ID 删除一条记录。
func (r *RelationRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM asset_relations WHERE relation_id = $1", id)
	return err
}

// GetByID 根据关系 ID 查询单条记录，不存在时返回 (nil, nil)。
func (r *RelationRepo) GetByID(ctx context.Context, id string) (*biz.AssetRelation, error) {
	rel := &biz.AssetRelation{}
	var metadataJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT relation_id, source_asset_id, target_asset_id, relation_type, metadata, created_at
		 FROM asset_relations WHERE relation_id = $1`, id).Scan(
		&rel.RelationID, &rel.SourceAssetID, &rel.TargetAssetID,
		&rel.RelationType, &metadataJSON, &rel.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get relation: %w", err)
	}
	json.Unmarshal(metadataJSON, &rel.Metadata)
	return rel, nil
}

// ListByAsset 根据资产 ID 和方向（upstream/downstream/both）查询相关的拓扑关系列表。
func (r *RelationRepo) ListByAsset(ctx context.Context, assetID, direction string) ([]*biz.AssetRelation, error) {
	var query string
	switch direction {
	case "upstream":
		query = `SELECT relation_id, source_asset_id, target_asset_id, relation_type, metadata, created_at
				 FROM asset_relations WHERE target_asset_id = $1 ORDER BY created_at`
	case "downstream":
		query = `SELECT relation_id, source_asset_id, target_asset_id, relation_type, metadata, created_at
				 FROM asset_relations WHERE source_asset_id = $1 ORDER BY created_at`
	default: // both
		query = `SELECT relation_id, source_asset_id, target_asset_id, relation_type, metadata, created_at
				 FROM asset_relations WHERE source_asset_id = $1 OR target_asset_id = $1 ORDER BY created_at`
	}

	rows, err := r.db.Query(ctx, query, assetID)
	if err != nil {
		return nil, fmt.Errorf("list relations: %w", err)
	}
	defer rows.Close()

	var relations []*biz.AssetRelation
	for rows.Next() {
		rel := &biz.AssetRelation{}
		var metadataJSON []byte
		if err := rows.Scan(
			&rel.RelationID, &rel.SourceAssetID, &rel.TargetAssetID,
			&rel.RelationType, &metadataJSON, &rel.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		json.Unmarshal(metadataJSON, &rel.Metadata)
		relations = append(relations, rel)
	}
	return relations, nil
}

// GetTopology 从根资产出发，使用 PostgreSQL 递归公用表表达式（WITH RECURSIVE）
// 构建指定深度和方向的多层依赖拓扑图，一次 SQL 请求完成全部层次遍历。
//
// 算法说明（递归 CTE 工作原理）：
//  1. 基础情况（Anchor）：将根资产 rootID 作为深度 0 的起始节点加入集合。
//  2. 递归步骤（Recursive）：对集合中的每个节点，沿 asset_relations 边找到其邻居节点，
//     深度加 1 后加入集合；重复此过程直到达到 depth 上限或无更多邻居。
//  3. UNION（非 UNION ALL）：自动去重，避免在有环拓扑中无限循环。
//  4. DISTINCT：最终结果再次去重，确保同一资产只出现一次。
//
// 性能优势：相比在应用层逐层 BFS（N+1 查询），递归 CTE 仅需一次数据库往返，
// 适合 depth <= 10 的常规拓扑查询。
//
// 参数：
//   - ctx: 请求上下文。
//   - rootID: 起始资产的 UUID。
//   - depth: 最大遍历深度（1-10，超出范围自动裁剪）。
//   - direction: 遍历方向：
//     "downstream" 仅跟随 source→target 方向（查询依赖的下游）；
//     "upstream"   仅跟随 target→source 方向（查询被依赖的上游）；
//     其他值（含空串）    双向遍历（默认）。
//
// 返回值：
//   - *biz.TopologyGraph: 包含所有节点（资产信息）和边（关系信息）的拓扑图对象。
//   - error: 数据库查询失败时返回错误。
func (r *RelationRepo) GetTopology(ctx context.Context, rootID string, depth int, direction string) (*biz.TopologyGraph, error) {
	// 边界检查：深度至少为 1，最大为 10（防止超大拓扑图导致内存/性能问题）
	if depth < 1 {
		depth = 1
	}
	if depth > 10 {
		depth = 10
	}

	// 根据方向参数构建递归 CTE 中的 JOIN 条件
	//   downstream: 跟随 source_asset_id = 当前节点，即沿正向依赖链向下遍历
	//   upstream:   跟随 target_asset_id = 当前节点，即沿反向依赖链向上遍历
	//   both:       两个方向都跟随（双向 BFS）
	var directionFilter string
	switch direction {
	case "downstream":
		// 只查当前资产指向的下游节点（当前节点是 source，目标节点是 target）
		directionFilter = "ar.source_asset_id = t.asset_id"
	case "upstream":
		// 只查依赖当前资产的上游节点（当前节点是 target，上游节点是 source）
		directionFilter = "ar.target_asset_id = t.asset_id"
	default: // "both" 或空值：双向遍历
		directionFilter = "(ar.source_asset_id = t.asset_id OR ar.target_asset_id = t.asset_id)"
	}

	// 构建递归 CTE SQL：
	//   topology CTE 从根节点出发，逐层展开所有可达节点，直到达到 depth 限制。
	//   CASE 表达式用于在双向遍历时，始终取"对端"节点 ID（非当前节点所在端）。
	query := fmt.Sprintf(`
		WITH RECURSIVE topology AS (
			-- 基础情况：将根节点放入集合，深度标记为 0
			SELECT $1::uuid AS asset_id, 0 AS depth
			UNION
			-- 递归情况：对集合中的每个节点，沿关系边找邻居节点，深度 +1
			SELECT CASE
				WHEN ar.source_asset_id = t.asset_id THEN ar.target_asset_id
				ELSE ar.source_asset_id
			END AS asset_id,
			t.depth + 1 AS depth
			FROM topology t
			JOIN asset_relations ar ON %s
			-- depth 守卫：确保递归在达到最大深度时停止（$2 = depth 参数）
			WHERE t.depth < $2
		)
		-- DISTINCT 去重：相同节点可能通过不同路径被多次发现
		SELECT DISTINCT asset_id FROM topology
	`, directionFilter)

	// 第一步：获取所有相关资产 ID
	rows, err := r.db.Query(ctx, query, rootID, depth)
	if err != nil {
		return nil, fmt.Errorf("topology CTE query: %w", err)
	}
	defer rows.Close()

	var assetIDs []string
	assetIDSet := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan topology node: %w", err)
		}
		if !assetIDSet[id] {
			assetIDs = append(assetIDs, id)
			assetIDSet[id] = true
		}
	}

	graph := &biz.TopologyGraph{
		Nodes: []*biz.Asset{},
		Edges: []*biz.AssetRelation{},
	}

	if len(assetIDs) == 0 {
		return graph, nil
	}

	// 第二步：批量获取所有资产信息
	assetRepo := NewAssetRepo(r.db)
	for _, id := range assetIDs {
		asset, err := assetRepo.GetByID(ctx, id)
		if err != nil || asset == nil {
			continue
		}
		graph.Nodes = append(graph.Nodes, asset)
	}

	// 第三步：批量获取所有相关的边（只取两端都在节点集合中的边）
	edgeRows, err := r.db.Query(ctx,
		`SELECT relation_id, source_asset_id, target_asset_id, relation_type, metadata, created_at
		 FROM asset_relations
		 WHERE source_asset_id = ANY($1) OR target_asset_id = ANY($1)`,
		assetIDs)
	if err != nil {
		return nil, fmt.Errorf("topology edges query: %w", err)
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		rel := &biz.AssetRelation{}
		var metadataJSON []byte
		if err := edgeRows.Scan(
			&rel.RelationID, &rel.SourceAssetID, &rel.TargetAssetID,
			&rel.RelationType, &metadataJSON, &rel.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan topology edge: %w", err)
		}
		json.Unmarshal(metadataJSON, &rel.Metadata)

		// 只包含两端都在拓扑图中的边
		if assetIDSet[rel.SourceAssetID] && assetIDSet[rel.TargetAssetID] {
			graph.Edges = append(graph.Edges, rel)
		}
	}

	return graph, nil
}

// GetCascadeAssets 返回通过 deployed_on/child_of 关系传递关联的所有资产 ID。
// 用于维护窗口的级联告警抑制。
func (r *RelationRepo) GetCascadeAssets(ctx context.Context, assetIDs []string) ([]string, error) {
	visited := map[string]bool{}
	queue := make([]string, len(assetIDs))
	copy(queue, assetIDs)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true

		// 查找部署于（deployed_on）或从属于（child_of）当前资产的下级资产
		rows, err := r.db.Query(ctx,
			`SELECT source_asset_id FROM asset_relations
			 WHERE target_asset_id = $1 AND relation_type IN ('deployed_on', 'child_of')`,
			current)
		if err != nil {
			return nil, fmt.Errorf("cascade query: %w", err)
		}

		for rows.Next() {
			var childID string
			rows.Scan(&childID)
			if !visited[childID] {
				queue = append(queue, childID)
			}
		}
		rows.Close()
	}

	result := make([]string, 0, len(visited))
	for id := range visited {
		result = append(result, id)
	}
	return result, nil
}
