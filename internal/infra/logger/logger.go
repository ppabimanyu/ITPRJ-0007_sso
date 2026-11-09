// Package logger provides the service JSON logger and request-scoped fields.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/ppa/itprj-0007-sso/internal/infra/tracing"
)

type contextKey int

const (
	requestIDKey contextKey = iota
	userIDKey
	baseLoggerKey
)

// New returns a JSON logger writing to stdout at the given level.
func New(level string) *slog.Logger {
	return NewWithWriter(os.Stdout, level)
}

// NewWithWriter mirrors New for tests and tools that capture output.
func NewWithWriter(w io.Writer, level string) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parseLevel(level)})
	return slog.New(&redactHandler{next: handler})
}

// WithRequestID stores the correlation ID later attached to every log line.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithUserID stores the authenticated user's internal ID. Handlers set it only
// after session validation.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// WithLogger stores a base logger, typically during startup wiring.
func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, baseLoggerKey, log)
}

// FromContext returns a logger carrying request_id, trace_id, span_id, and
// user_id when they are present. It falls back to slog.Default().
func FromContext(ctx context.Context) *slog.Logger {
	log, ok := ctx.Value(baseLoggerKey).(*slog.Logger)
	if !ok || log == nil {
		log = slog.Default()
	}

	attrs := make([]any, 0, 8)
	if id := RequestID(ctx); id != "" {
		attrs = append(attrs, slog.String("request_id", id))
	}
	if tc, ok := tracing.FromContext(ctx); ok {
		attrs = append(attrs,
			slog.String("trace_id", tc.TraceID.String()),
			slog.String("span_id", tc.SpanID.String()),
		)
	}
	if id := UserID(ctx); id != "" {
		attrs = append(attrs, slog.String("user_id", id))
	}
	if len(attrs) == 0 {
		return log
	}
	return log.With(attrs...)
}

// RequestID returns the request correlation ID stored in ctx.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// UserID returns the authenticated user ID stored in ctx.
func UserID(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// redactHandler removes secrets before they reach the sink. It runs at every
// WithAttrs and Handle boundary so nested groups cannot leak a value.
type redactHandler struct {
	next slog.Handler
}

func (h *redactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactHandler) Handle(ctx context.Context, record slog.Record) error {
	cleaned := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		cleaned.AddAttrs(redact(attr))
		return true
	})
	return h.next.Handle(ctx, cleaned)
}

func (h *redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	safe := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		safe[i] = redact(attr)
	}
	return &redactHandler{next: h.next.WithAttrs(safe)}
}

func (h *redactHandler) WithGroup(name string) slog.Handler {
	return &redactHandler{next: h.next.WithGroup(name)}
}

func redact(attr slog.Attr) slog.Attr {
	if attr.Key == "" && attr.Value.Kind() == slog.KindGroup {
		return attr
	}
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		safe := make([]slog.Attr, len(group))
		for i, child := range group {
			safe[i] = redact(child)
		}
		attr.Value = slog.GroupValue(safe...)
		return attr
	}
	if sensitive(attr.Key) {
		return slog.String(attr.Key, "[REDACTED]")
	}
	return attr
}

func sensitive(key string) bool {
	key = strings.ToLower(key)
	switch key {
	case "email", "token", "access_token", "refresh_token", "password",
		"authorization", "cookie", "secret", "id_token", "client_secret":
		return true
	}
	for _, fragment := range []string{"password", "secret", "token", "cookie", "authorization", "email"} {
		if strings.Contains(key, fragment) {
			return true
		}
	}
	return false
}
