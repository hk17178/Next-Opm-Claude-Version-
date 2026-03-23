// Package logger 提供 OpsNexus 所有服务共享的结构化日志工具。
// 所有服务必须使用此包进行日志记录，禁止使用 fmt.Println 或 log.Println。
// 支持链路追踪上下文（trace_id、span_id）和请求ID（request_id）的关联。
package logger

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// contextKey 是用于 context 存取日志相关信息的键类型
type contextKey string

// context 键常量，用于在请求上下文中传递链路追踪和请求标识信息
const (
	traceIDKey   contextKey = "trace_id"
	spanIDKey    contextKey = "span_id"
	requestIDKey contextKey = "request_id"
)

// New 为指定服务创建结构化日志实例。
// 日志级别可通过 LOG_LEVEL 环境变量配置，默认 INFO。输出 JSON 格式到 stdout。
func New(serviceName string) *zap.Logger {
	level := zapcore.InfoLevel
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		_ = level.UnmarshalText([]byte(envLevel))
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)

	return zap.New(core).With(
		zap.String("service", serviceName),
	)
}

// NewWithLevel 创建指定日志级别的日志实例。
func NewWithLevel(serviceName string, level zapcore.Level) *zap.Logger {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)

	return zap.New(core).With(
		zap.String("service", serviceName),
	)
}

// WithTraceContext 从上下文中提取链路追踪信息并注入日志实例，
// 确保同一请求内的所有日志条目都关联到相同的 trace。
func WithTraceContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	l := logger
	if traceID, ok := ctx.Value(traceIDKey).(string); ok && traceID != "" {
		l = l.With(zap.String("trace_id", traceID))
	}
	if spanID, ok := ctx.Value(spanIDKey).(string); ok && spanID != "" {
		l = l.With(zap.String("span_id", spanID))
	}
	if requestID, ok := ctx.Value(requestIDKey).(string); ok && requestID != "" {
		l = l.With(zap.String("request_id", requestID))
	}
	return l
}

// ContextWithTrace 将链路追踪ID注入上下文，用于下游传播。
func ContextWithTrace(ctx context.Context, traceID, spanID string) context.Context {
	ctx = context.WithValue(ctx, traceIDKey, traceID)
	ctx = context.WithValue(ctx, spanIDKey, spanID)
	return ctx
}

// ContextWithRequestID 将请求ID注入上下文。
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFromContext 从上下文中提取请求ID。
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// TraceIDFromContext 从上下文中提取链路追踪ID。
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// ParseLevel 将字符串日志级别转换为 zapcore.Level。无法识别时默认返回 INFO。
func ParseLevel(level string) zapcore.Level {
	var l zapcore.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return zapcore.InfoLevel
	}
	return l
}
