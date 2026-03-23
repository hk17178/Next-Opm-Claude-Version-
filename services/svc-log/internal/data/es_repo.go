// Package data 实现日志服务的数据访问层，提供 PostgreSQL 和 Elasticsearch 的具体存储操作。
package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/opsnexus/svc-log/internal/biz"
)

// ESRepo 使用 Elasticsearch 实现 biz.ESRepository 接口，负责日志的批量写入和搜索查询。
type ESRepo struct {
	client      *elasticsearch.Client
	indexPrefix string
}

// NewESClient 创建一个新的 Elasticsearch 客户端连接。
func NewESClient(addresses []string) (*elasticsearch.Client, error) {
	cfg := elasticsearch.Config{
		Addresses: addresses,
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create es client: %w", err)
	}
	return client, nil
}

// NewESRepo 创建一个新的 ESRepo 实例，indexPrefix 用于生成按日期分片的索引名。
func NewESRepo(client *elasticsearch.Client, indexPrefix string) *ESRepo {
	return &ESRepo{
		client:      client,
		indexPrefix: indexPrefix,
	}
}

// BulkIndex 使用 Elasticsearch Bulk API 批量写入日志条目，索引按日期自动分片。
func (r *ESRepo) BulkIndex(ctx context.Context, entries []biz.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, entry := range entries {
		index := fmt.Sprintf("%s-%s", r.indexPrefix, entry.Timestamp.Format("2006.01.02"))

		meta := map[string]any{
			"index": map[string]any{
				"_index": index,
				"_id":    entry.ID,
			},
		}
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("marshal bulk meta: %w", err)
		}
		buf.Write(metaJSON)
		buf.WriteByte('\n')

		docJSON, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal bulk doc: %w", err)
		}
		buf.Write(docJSON)
		buf.WriteByte('\n')
	}

	res, err := r.client.Bulk(bytes.NewReader(buf.Bytes()), r.client.Bulk.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("bulk request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk response error: %s", res.String())
	}

	var bulkResp struct {
		Errors bool `json:"errors"`
		Items  []struct {
			Index struct {
				Error any `json:"error"`
			} `json:"index"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&bulkResp); err != nil {
		return fmt.Errorf("decode bulk response: %w", err)
	}
	if bulkResp.Errors {
		errCount := 0
		for _, item := range bulkResp.Items {
			if item.Index.Error != nil {
				errCount++
			}
		}
		return fmt.Errorf("bulk index: %d of %d documents failed", errCount, len(entries))
	}

	return nil
}

// Search 基于 OpenAPI LogSearchRequest 结构查询 Elasticsearch，支持全文检索、时间范围过滤和 search_after 分页。
func (r *ESRepo) Search(ctx context.Context, req biz.LogSearchRequest) (*biz.LogSearchResponse, error) {
	esQuery := buildSearchQuery(req)

	queryJSON, err := json.Marshal(esQuery)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	indexPattern := fmt.Sprintf("%s-*", r.indexPrefix)

	res, err := r.client.Search(
		r.client.Search.WithContext(ctx),
		r.client.Search.WithIndex(indexPattern),
		r.client.Search.WithBody(bytes.NewReader(queryJSON)),
	)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.String())
	}

	var searchResp struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source biz.LogEntry `json:"_source"`
				Sort   []any       `json:"sort"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	result := &biz.LogSearchResponse{
		Total: searchResp.Hits.Total.Value,
	}
	for _, hit := range searchResp.Hits.Hits {
		result.Entries = append(result.Entries, hit.Source)
	}

	// 从最后一条命中结果的排序值构建 next_page_token，用于 search_after 分页
	if len(searchResp.Hits.Hits) > 0 {
		lastSort := searchResp.Hits.Hits[len(searchResp.Hits.Hits)-1].Sort
		if len(lastSort) > 0 {
			sortJSON, _ := json.Marshal(lastSort)
			result.NextPageToken = string(sortJSON)
		}
	}

	return result, nil
}

// Aggregate 执行 Elasticsearch 聚合查询，支持 terms 和 date_histogram 两种聚合方式。
func (r *ESRepo) Aggregate(ctx context.Context, req biz.LogStatsRequest) (*biz.LogStatsResponse, error) {
	esQuery := buildAggregationQuery(req)

	queryJSON, err := json.Marshal(esQuery)
	if err != nil {
		return nil, fmt.Errorf("marshal agg query: %w", err)
	}

	indexPattern := fmt.Sprintf("%s-*", r.indexPrefix)

	res, err := r.client.Search(
		r.client.Search.WithContext(ctx),
		r.client.Search.WithIndex(indexPattern),
		r.client.Search.WithBody(bytes.NewReader(queryJSON)),
		r.client.Search.WithSize(0),
	)
	if err != nil {
		return nil, fmt.Errorf("agg request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("agg error: %s", res.String())
	}

	var aggResp struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
		} `json:"hits"`
		Aggregations struct {
			GroupBy struct {
				Buckets []struct {
					Key      any    `json:"key"`
					KeyStr   string `json:"key_as_string"`
					DocCount int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"group_by"`
		} `json:"aggregations"`
	}
	if err := json.NewDecoder(res.Body).Decode(&aggResp); err != nil {
		return nil, fmt.Errorf("decode agg response: %w", err)
	}

	result := &biz.LogStatsResponse{
		Total: aggResp.Hits.Total.Value,
	}
	for _, b := range aggResp.Aggregations.GroupBy.Buckets {
		key := b.KeyStr
		if key == "" {
			key = fmt.Sprintf("%v", b.Key)
		}
		result.Buckets = append(result.Buckets, biz.StatsBucket{
			Key:      key,
			DocCount: b.DocCount,
		})
	}

	return result, nil
}

// ListIndices 列出匹配指定模式的 Elasticsearch 索引，返回索引名和创建时间。
func (r *ESRepo) ListIndices(ctx context.Context, pattern string) ([]biz.IndexInfo, error) {
	res, err := r.client.Cat.Indices(
		r.client.Cat.Indices.WithIndex(pattern),
		r.client.Cat.Indices.WithFormat("json"),
		r.client.Cat.Indices.WithH("index", "creation.date.string"),
		r.client.Cat.Indices.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("cat indices: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("cat indices error: %s", res.String())
	}

	var catResp []struct {
		Index        string `json:"index"`
		CreationDate string `json:"creation.date.string"`
	}
	if err := json.NewDecoder(res.Body).Decode(&catResp); err != nil {
		return nil, fmt.Errorf("decode cat indices: %w", err)
	}

	var indices []biz.IndexInfo
	for _, item := range catResp {
		info := biz.IndexInfo{Name: item.Index}
		// 尝试从索引名中解析日期（格式: prefix-YYYY.MM.DD）
		parts := strings.Split(item.Index, "-")
		if len(parts) > 0 {
			datePart := parts[len(parts)-1]
			if t, err := time.Parse("2006.01.02", datePart); err == nil {
				info.CreatedAt = t
			}
		}
		indices = append(indices, info)
	}
	return indices, nil
}

// DeleteIndex 删除指定的 Elasticsearch 索引。
func (r *ESRepo) DeleteIndex(ctx context.Context, indexName string) error {
	res, err := r.client.Indices.Delete(
		[]string{indexName},
		r.client.Indices.Delete.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("delete index %s: %w", indexName, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("delete index %s error: %s", indexName, res.String())
	}
	return nil
}

// buildSearchQuery 将业务层搜索请求转换为 Elasticsearch 查询 DSL。
func buildSearchQuery(q biz.LogSearchRequest) map[string]any {
	must := []map[string]any{}

	// Main query string (Lucene syntax)
	if q.Query != "" && q.Query != "*" {
		must = append(must, map[string]any{
			"query_string": map[string]any{
				"query":            q.Query,
				"default_field":    "message",
				"analyze_wildcard": true,
			},
		})
	}

	// Time range filter
	if q.TimeRange != nil {
		rangeFilter := map[string]any{}
		if q.TimeRange.Start != nil {
			rangeFilter["gte"] = q.TimeRange.Start.Format("2006-01-02T15:04:05Z")
		}
		if q.TimeRange.End != nil {
			rangeFilter["lte"] = q.TimeRange.End.Format("2006-01-02T15:04:05Z")
		}
		if len(rangeFilter) > 0 {
			must = append(must, map[string]any{
				"range": map[string]any{
					"timestamp": rangeFilter,
				},
			})
		}
	}

	// Generic filters (key=ES field, value=term value)
	for field, value := range q.Filters {
		must = append(must, map[string]any{
			"term": map[string]any{
				field: value,
			},
		})
	}

	sortOrder := "desc"
	if q.Sort != "" {
		sortOrder = strings.ToLower(q.Sort)
	}

	esQuery := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": must,
			},
		},
		"size": q.PageSize,
		"sort": []map[string]any{
			{"timestamp": map[string]any{"order": sortOrder}},
		},
	}

	// search_after pagination via page_token
	if q.PageToken != "" {
		var searchAfter []any
		if err := json.Unmarshal([]byte(q.PageToken), &searchAfter); err == nil {
			esQuery["search_after"] = searchAfter
		}
	}

	return esQuery
}

// buildAggregationQuery 将业务层统计请求转换为 Elasticsearch 聚合查询 DSL。
func buildAggregationQuery(req biz.LogStatsRequest) map[string]any {
	must := []map[string]any{}

	if req.TimeRange != nil {
		rangeFilter := map[string]any{}
		if req.TimeRange.Start != nil {
			rangeFilter["gte"] = req.TimeRange.Start.Format("2006-01-02T15:04:05Z")
		}
		if req.TimeRange.End != nil {
			rangeFilter["lte"] = req.TimeRange.End.Format("2006-01-02T15:04:05Z")
		}
		if len(rangeFilter) > 0 {
			must = append(must, map[string]any{
				"range": map[string]any{
					"timestamp": rangeFilter,
				},
			})
		}
	}

	aggs := map[string]any{}
	if req.Interval != "" {
		aggs["group_by"] = map[string]any{
			"date_histogram": map[string]any{
				"field":          req.GroupBy,
				"fixed_interval": req.Interval,
				"min_doc_count":  0,
			},
		}
	} else {
		aggs["group_by"] = map[string]any{
			"terms": map[string]any{
				"field": req.GroupBy,
				"size":  100,
			},
		}
	}

	return map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": must,
			},
		},
		"aggs": aggs,
		"size": 0,
	}
}
