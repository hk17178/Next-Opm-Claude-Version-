// request.go 提供 HTTP 请求参数解析辅助函数，包括分页和排序参数提取。

package httputil

import (
	"net/http"
	"strconv"
)

// Pagination 保存解析后的分页参数。
type Pagination struct {
	Page     int
	PageSize int
}

// 分页参数默认值和限制
const (
	DefaultPage     = 1   // 默认页码
	DefaultPageSize = 20  // 默认每页条数
	MaxPageSize     = 100 // 最大每页条数
)

// ParsePagination 从 URL 查询参数中提取 page 和 page_size，并进行合法性校验。
func ParsePagination(r *http.Request) Pagination {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = DefaultPage
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return Pagination{Page: page, PageSize: pageSize}
}

// Offset 返回当前页对应的 SQL OFFSET 值。
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// ParseSortParams 从查询参数中提取排序字段和排序方向。
// allowedFields 为允许排序的字段白名单，不在白名单中则使用 defaultField。
func ParseSortParams(r *http.Request, allowedFields map[string]bool, defaultField string) (string, string) {
	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")

	if sortBy == "" || !allowedFields[sortBy] {
		sortBy = defaultField
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	return sortBy, sortOrder
}
