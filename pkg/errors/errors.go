// Package errors 提供 OpsNexus 全平台统一的错误码体系和结构化错误类型。
// 错误分为 NotFound（404）、BadRequest（400）、Internal（500）、Conflict（409）、
// Forbidden（403）、Unauthorized（401）、TooManyRequests（429）等类别。
// 各业务域（日志、事件、CMDB、告警、AI、通知等）在此定义各自的错误码常量。
package errors

import (
	"fmt"
	"net/http"
)

// AppError 表示结构化的应用错误，包含支持国际化的错误码、错误消息和对应的 HTTP 状态码。
type AppError struct {
	Code       string `json:"code"`    // 错误码，格式为 "{域}.{资源}.{错误类型}"
	Message    string `json:"message"` // 人类可读的错误描述
	HTTPStatus int    `json:"-"`       // 对应的 HTTP 状态码，不序列化到 JSON
}

// Error 实现 error 接口，返回格式化的错误字符串。
func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// New 创建一个新的 AppError 实例。
func New(code, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// 以下为各 HTTP 状态码对应的错误构造函数。

// NotFound 创建 404 资源未找到错误。
func NotFound(code, message string) *AppError {
	return New(code, message, http.StatusNotFound)
}

// BadRequest 创建 400 请求参数错误。
func BadRequest(code, message string) *AppError {
	return New(code, message, http.StatusBadRequest)
}

// Internal 创建 500 服务器内部错误。
func Internal(code, message string) *AppError {
	return New(code, message, http.StatusInternalServerError)
}

// Conflict 创建 409 资源冲突错误。
func Conflict(code, message string) *AppError {
	return New(code, message, http.StatusConflict)
}

// Forbidden 创建 403 禁止访问错误。
func Forbidden(code, message string) *AppError {
	return New(code, message, http.StatusForbidden)
}

// Unauthorized 创建 401 未认证错误。
func Unauthorized(code, message string) *AppError {
	return New(code, message, http.StatusUnauthorized)
}

// TooManyRequests 创建 429 请求过于频繁错误。
func TooManyRequests(code, message string) *AppError {
	return New(code, message, http.StatusTooManyRequests)
}

// 日志域错误码
const (
	ErrLogSourceNotFound    = "log.source.not_found"
	ErrLogSourceExists      = "log.source.already_exists"
	ErrParseRuleNotFound    = "log.parse_rule.not_found"
	ErrMaskingRuleNotFound  = "log.masking_rule.not_found"
	ErrRetentionNotFound    = "log.retention.not_found"
	ErrLogSearchFailed      = "log.search.failed"
	ErrLogIngestFailed      = "log.ingest.failed"
	ErrLogExportFailed      = "log.export.failed"
	ErrInvalidLogFormat     = "log.format.invalid"
	ErrInvalidSearchQuery   = "log.search.invalid_query"
)

// 事件域错误码
const (
	ErrIncidentNotFound        = "incident.not_found"
	ErrIncidentAlreadyResolved = "incident.already_resolved"
	ErrIncidentAlreadyClosed   = "incident.already_closed"
	ErrIncidentInvalidTransition = "incident.invalid_status_transition"
	ErrPostmortemRequired      = "incident.postmortem_required"
	ErrChangeOrderNotFound     = "incident.change_order.not_found"
	ErrScheduleNotFound        = "incident.schedule.not_found"
	ErrTimelineEntryNotFound   = "incident.timeline.not_found"
)

// 变更管理域错误码
const (
	ErrChangeNotFound          = "change.not_found"
	ErrChangeInvalidTransition = "change.invalid_status_transition"
	ErrChangeNotEditable       = "change.not_editable"
	ErrChangeConflict          = "change.conflict"
	ErrApprovalInvalidStatus   = "change.approval.invalid_status"
)

// CMDB 配置管理域错误码
const (
	ErrAssetNotFound          = "cmdb.asset.not_found"
	ErrAssetAlreadyExists     = "cmdb.asset.already_exists"
	ErrAssetGroupNotFound     = "cmdb.asset_group.not_found"
	ErrRelationNotFound       = "cmdb.relation.not_found"
	ErrRelationCycle          = "cmdb.relation.cycle_detected"
	ErrDimensionNotFound      = "cmdb.dimension.not_found"
	ErrDimensionAlreadyExists = "cmdb.dimension.already_exists"
	ErrDiscoveryRecordNotFound = "cmdb.discovery.not_found"
	ErrImportFailed           = "cmdb.import.failed"
	ErrTopologyQueryFailed    = "cmdb.topology.query_failed"
)

// 分析域错误码
const (
	ErrSLANotFound         = "analytics.sla.not_found"
	ErrSLACalculationFailed = "analytics.sla.calculation_failed"
	ErrReportNotFound      = "analytics.report.not_found"
	ErrReportGenerateFailed = "analytics.report.generate_failed"
	ErrCorrelationFailed   = "analytics.correlation.failed"
	ErrKnowledgeNotFound   = "analytics.knowledge.not_found"
)

// 告警域错误码
const (
	ErrAlertRuleNotFound    = "alert.rule.not_found"
	ErrAlertRuleExists      = "alert.rule.already_exists"
	ErrAlertNotFound        = "alert.not_found"
	ErrAlertAlreadyResolved = "alert.already_resolved"
	ErrSilenceNotFound      = "alert.silence.not_found"
	ErrInvalidRuleExpr      = "alert.rule.invalid_expression"
)

// AI 智能分析域错误码
const (
	ErrAnalysisNotFound    = "ai.analysis.not_found"
	ErrAnalysisFailed      = "ai.analysis.failed"
	ErrModelNotFound       = "ai.model.not_found"
	ErrModelUnavailable    = "ai.model.unavailable"
	ErrKnowledgeBaseEmpty  = "ai.knowledge.empty"
)

// 通知域错误码
const (
	ErrChannelNotFound      = "notify.channel.not_found"
	ErrChannelDisabled      = "notify.channel.disabled"
	ErrTemplateNotFound     = "notify.template.not_found"
	ErrDeliveryFailed       = "notify.delivery.failed"
	ErrContactNotFound      = "notify.contact.not_found"
	ErrRateLimitExceeded    = "notify.rate_limit.exceeded"
)

// 认证域错误码
const (
	ErrAuthUnauthenticated  = "auth.unauthenticated"
	ErrAuthPermissionDenied = "auth.permission_denied"
	ErrAuthTokenExpired     = "auth.token_expired"
	ErrAuthTokenInvalid     = "auth.token_invalid"
)

// 安全域错误码
const (
	ErrSecurityIPBlocked      = "security.ip.blocked"       // IP 不在白名单中
	ErrSecurityAccountLocked  = "security.account.locked"   // 账户因多次失败被锁定
	ErrSecurityLoginFailed    = "security.login.failed"     // 登录失败
	ErrSecurityAnomalyLogin   = "security.login.anomaly"    // 异常登录（异地/异设备）
	ErrSecuritySessionExpired = "security.session.expired"  // 会话过期
	ErrSecuritySessionRevoked = "security.session.revoked"  // 会话已被撤销
	ErrSecurityMFARequired    = "security.mfa.required"     // 需要多因素认证
	ErrSecurityMFAInvalid     = "security.mfa.invalid_code" // MFA 验证码无效
	ErrSecurityMFASetupFailed = "security.mfa.setup_failed" // MFA 设置失败
)

// Unauthenticated 创建 401 未认证错误，用于缺少或无效的认证凭据。
func Unauthenticated(message string) *AppError {
	return New(ErrAuthUnauthenticated, message, http.StatusUnauthorized)
}

// PermissionDenied 创建 403 权限不足错误，支持格式化消息。
func PermissionDenied(format string, args ...interface{}) *AppError {
	return New(ErrAuthPermissionDenied, fmt.Sprintf(format, args...), http.StatusForbidden)
}

// Wrap 将已有错误包装为 AppError，默认 HTTP 状态码为 500。
func Wrap(code string, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    err.Error(),
		HTTPStatus: http.StatusInternalServerError,
	}
}

// Is 检查错误是否为指定错误码的 AppError。
func Is(err error, code string) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == code
	}
	return false
}
