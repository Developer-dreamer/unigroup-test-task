package internal

import (
	"context"
	"errors"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
)

var ErrNilLogger = errors.New("logger is nil")

type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
	DebugContext(ctx context.Context, msg string, args ...any)
}

type contextKey string

const messageIDKey contextKey = "message_id"

func WithMessageID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, messageIDKey, id)
}

func GetMessageID(ctx context.Context) string {
	if v, ok := ctx.Value(messageIDKey).(string); ok {
		return v
	}
	return ""
}

// TraceHandler is a slog.Handler wrapper that injects OpenTelemetry trace IDs
// from the context into log records before delegating to the underlying handler.
type TraceHandler struct {
	slog.Handler
}

// Handle enriches the provided slog.Record with the current trace ID, if a valid
// OpenTelemetry span is found in the context, and then forwards the record to
// the wrapped slog.Handler as part of the logging pipeline.
func (h TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		r.AddAttrs(slog.String("trace_id", span.SpanContext().TraceID().String()))
	}

	if msgID := GetMessageID(ctx); msgID != "" {
		r.AddAttrs(slog.String("message_id", msgID))
	}

	return h.Handler.Handle(ctx, r)
}
