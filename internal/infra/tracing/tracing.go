// Package tracing implements W3C trace context without an OpenTelemetry
// exporter. The trace and span identifiers created here are the only ones the
// service emits until a collector exists.
package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// TraceID is a 16-byte W3C trace identifier.
type TraceID [16]byte

// SpanID is an 8-byte W3C span identifier.
type SpanID [8]byte

func (t TraceID) String() string { return hex.EncodeToString(t[:]) }
func (s SpanID) String() string  { return hex.EncodeToString(s[:]) }

// TraceContext is the parsed traceparent state of one request.
type TraceContext struct {
	TraceID TraceID
	SpanID  SpanID
	Flags   byte
}

// Header renders the context as a traceparent value.
func (tc TraceContext) Header() string {
	return fmt.Sprintf("00-%s-%s-%02x", tc.TraceID.String(), tc.SpanID.String(), tc.Flags)
}

// New creates a sampled trace context with random identifiers.
func New() TraceContext {
	return TraceContext{
		TraceID: newTraceID(),
		SpanID:  newSpanID(),
		Flags:   1,
	}
}

// NewSpan keeps the trace ID and starts a new child span.
func (tc TraceContext) NewSpan() TraceContext {
	return TraceContext{TraceID: tc.TraceID, SpanID: newSpanID(), Flags: tc.Flags}
}

// ParseTraceparent validates and parses a W3C traceparent header. The all-zero
// trace and span IDs are rejected, as is the reserved version ff.
func ParseTraceparent(value string) (TraceContext, bool) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "-")
	if len(parts) != 4 {
		return TraceContext{}, false
	}
	if len(parts[0]) != 2 || parts[0] == "ff" {
		return TraceContext{}, false
	}
	if _, err := hex.DecodeString(parts[0]); err != nil {
		return TraceContext{}, false
	}
	if len(parts[1]) != 32 || len(parts[2]) != 16 || len(parts[3]) != 2 {
		return TraceContext{}, false
	}
	traceBytes, err := hex.DecodeString(parts[1])
	if err != nil || isZero(traceBytes) {
		return TraceContext{}, false
	}
	spanBytes, err := hex.DecodeString(parts[2])
	if err != nil || isZero(spanBytes) {
		return TraceContext{}, false
	}
	flagBytes, err := hex.DecodeString(parts[3])
	if err != nil {
		return TraceContext{}, false
	}

	var tc TraceContext
	copy(tc.TraceID[:], traceBytes)
	copy(tc.SpanID[:], spanBytes)
	tc.Flags = flagBytes[0]
	return tc, true
}

type contextKey struct{}

// WithContext stores the request trace context.
func WithContext(ctx context.Context, tc TraceContext) context.Context {
	return context.WithValue(ctx, contextKey{}, tc)
}

// FromContext returns the request trace context when the middleware ran.
func FromContext(ctx context.Context) (TraceContext, bool) {
	tc, ok := ctx.Value(contextKey{}).(TraceContext)
	return tc, ok
}

// Middleware adopts a valid incoming traceparent or creates a new context, then
// echoes the resulting value in the response.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := ParseTraceparent(r.Header.Get("traceparent"))
		if !ok {
			tc = New()
		}
		w.Header().Set("traceparent", tc.Header())
		next.ServeHTTP(w, r.WithContext(WithContext(r.Context(), tc)))
	})
}

func newTraceID() TraceID {
	var id TraceID
	for {
		if _, err := rand.Read(id[:]); err != nil {
			panic(fmt.Sprintf("tracing: read random trace id: %v", err))
		}
		if !isZero(id[:]) {
			return id
		}
	}
}

func newSpanID() SpanID {
	var id SpanID
	for {
		if _, err := rand.Read(id[:]); err != nil {
			panic(fmt.Sprintf("tracing: read random span id: %v", err))
		}
		if !isZero(id[:]) {
			return id
		}
	}
}

func isZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}
