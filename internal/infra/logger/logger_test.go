package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/ppa/itprj-0007-sso/internal/infra/tracing"
)

func TestFromContextAddsRequestFields(t *testing.T) {
	var buf bytes.Buffer
	base := NewWithWriter(&buf, "debug")

	tc, ok := tracing.ParseTraceparent("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	if !ok {
		t.Fatal("test traceparent did not parse")
	}
	ctx := WithLogger(context.Background(), base)
	ctx = WithRequestID(ctx, "req-123")
	ctx = tracing.WithContext(ctx, tc)
	ctx = WithUserID(ctx, "user-456")

	FromContext(ctx).Info("hello")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log line is not JSON: %v (%s)", err, buf.String())
	}
	for key, want := range map[string]string{
		"request_id": "req-123",
		"trace_id":   "4bf92f3577b34da6a3ce929d0e0e4736",
		"span_id":    "00f067aa0ba902b7",
		"user_id":    "user-456",
	} {
		if got, _ := line[key].(string); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestFromContextOmitsMissingFields(t *testing.T) {
	var buf bytes.Buffer
	ctx := WithLogger(context.Background(), NewWithWriter(&buf, "info"))

	FromContext(ctx).Info("hello")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	for _, key := range []string{"request_id", "trace_id", "span_id", "user_id"} {
		if _, ok := line[key]; ok {
			t.Errorf("unexpected field %q in %s", key, buf.String())
		}
	}
}

func TestRedaction(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter(&buf, "debug")

	log.Info("login",
		slog.String("email", "admin@example.com"),
		slog.String("password", "hunter2"),
		slog.String("access_token", "tok_123"),
		slog.String("authorization", "Bearer abc"),
		slog.String("cookie", "sso_session=xyz"),
		slog.String("secret", "s3cr3t"),
		slog.String("request_id", "req-1"),
		slog.Group("headers", slog.String("X-Refresh-Token", "refresh-1")),
	)

	out := buf.String()
	for _, raw := range []string{
		"admin@example.com", "hunter2", "tok_123", "Bearer abc",
		"sso_session=xyz", "s3cr3t", "refresh-1",
	} {
		if strings.Contains(out, raw) {
			t.Errorf("log leaked %q: %s", raw, out)
		}
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("expected redaction marker: %s", out)
	}
	if !strings.Contains(out, `"request_id":"req-1"`) {
		t.Errorf("request_id was dropped: %s", out)
	}
}

func TestWithAttrsRedactsBeforeSink(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter(&buf, "info").With(slog.String("client_secret", "shh"))

	log.Info("x", slog.String("note", "ok"))

	out := buf.String()
	if strings.Contains(out, "shh") {
		t.Fatalf("WithAttrs leaked secret: %s", out)
	}
	if !strings.Contains(out, `"note":"ok"`) {
		t.Fatalf("non-sensitive attr missing: %s", out)
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter(&buf, "warn")
	log.Info("dropped")
	log.Warn("kept")

	if strings.Contains(buf.String(), "dropped") {
		t.Errorf("info log was not filtered: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "kept") {
		t.Errorf("warn log was filtered: %s", buf.String())
	}
}
