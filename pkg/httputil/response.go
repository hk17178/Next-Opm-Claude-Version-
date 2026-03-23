// Package httputil 提供 OpsNexus 所有服务共享的 HTTP 请求和响应辅助函数，
// 包括统一的 JSON 响应格式、分页响应、错误响应和请求参数解析。
package httputil

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	apperrors "github.com/opsnexus/opsnexus/pkg/errors"
)

// Response 是标准的 API 响应信封，所有接口响应统一使用此结构。
type Response struct {
	Code      string      `json:"code,omitempty"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id"`
	Timestamp string      `json:"timestamp"`
}

// PagedResponse 在标准响应基础上添加分页元数据，用于列表查询接口。
type PagedResponse struct {
	Response
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// JSON 写入成功的 JSON 响应，自动生成 request_id 和时间戳。
func JSON(w http.ResponseWriter, status int, data interface{}) {
	resp := Response{
		Data:      data,
		RequestID: uuid.New().String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// PagedJSON 写入带分页信息的成功 JSON 响应。
func PagedJSON(w http.ResponseWriter, data interface{}, page, pageSize int, total int64) {
	resp := PagedResponse{
		Response: Response{
			Data:      data,
			RequestID: uuid.New().String(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// Error 写入错误 JSON 响应，HTTP 状态码由 AppError 决定。
func Error(w http.ResponseWriter, err *apperrors.AppError) {
	resp := Response{
		Code:      err.Code,
		Message:   err.Message,
		RequestID: uuid.New().String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(err.HTTPStatus)
	json.NewEncoder(w).Encode(resp)
}

// ErrorMsg 根据错误码、消息和状态码写入简单的错误 JSON 响应。
func ErrorMsg(w http.ResponseWriter, code, message string, status int) {
	Error(w, apperrors.New(code, message, status))
}
