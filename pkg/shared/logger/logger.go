// Package logger provides a structured logging setup for OpsNexus services.
// All services must use this package instead of fmt.Println or log.Println.
package logger

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const (
	traceIDKey contextKey = "trace_id"
	spanIDKey  contextKey = "span_id"
	serviceKey contextKey = "service"
)

// New creates a new structured logger for a service.
func New(serviceName string, level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)

	return slog.New(handler).With(
		slog.String("service", serviceName),
	)
}

// WithTraceContext returns a logger enriched with trace context from the context.
func WithTraceContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	l := logger
	if traceID, ok := ctx.Value(traceIDKey).(string); ok && traceID != "" {
		l = l.With(slog.String("trace_id", traceID))
	}
	if spanID, ok := ctx.Value(spanIDKey).(string); ok && spanID != "" {
		l = l.With(slog.String("span_id", spanID))
	}
	return l
}

// ContextWithTrace adds trace information to a context.
func ContextWithTrace(ctx context.Context, traceID, spanID string) context.Context {
	ctx = context.WithValue(ctx, traceIDKey, traceID)
	ctx = context.WithValue(ctx, spanIDKey, spanID)
	return ctx
}

// ParseLevel converts a string level to slog.Level.
func ParseLevel(level string) slog.Level {
	switch level {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "info", "INFO":
		return slog.LevelInfo
	case "warn", "WARN", "warning", "WARNING":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
