package tracing

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseTraceparentValid(t *testing.T) {
	header := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	tc, ok := ParseTraceparent(header)
	if !ok {
		t.Fatal("ParseTraceparent() rejected a valid header")
	}
	if got := tc.TraceID.String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("TraceID = %s", got)
	}
	if got := tc.SpanID.String(); got != "00f067aa0ba902b7" {
		t.Errorf("SpanID = %s", got)
	}
	if tc.Flags != 1 {
		t.Errorf("Flags = %d", tc.Flags)
	}
	if got := tc.Header(); got != header {
		t.Errorf("Header() = %s, want %s", got, header)
	}
}

func TestParseTraceparentMalformed(t *testing.T) {
	tests := []string{
		"",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7",
		"ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
		"00-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-0z",
		"01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-extra",
	}
	for _, header := range tests {
		if _, ok := ParseTraceparent(header); ok {
			t.Errorf("ParseTraceparent(%q) accepted a malformed header", header)
		}
	}
}

func TestMiddlewareAdoptsValidHeader(t *testing.T) {
	header := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	var seen TraceContext
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := FromContext(r.Context())
		if !ok {
			t.Fatal("trace context missing from request")
		}
		seen = tc
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("traceparent", header)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if seen.TraceID.String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("TraceID = %s", seen.TraceID)
	}
	if got := rec.Header().Get("traceparent"); got != header {
		t.Errorf("response traceparent = %q, want %q", got, header)
	}
}

func TestMiddlewareGeneratesWhenAbsentOrMalformed(t *testing.T) {
	for name, header := range map[string]string{
		"absent":    "",
		"malformed": "not-a-traceparent",
	} {
		t.Run(name, func(t *testing.T) {
			var seen TraceContext
			handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen, _ = FromContext(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if header != "" {
				req.Header.Set("traceparent", header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if isZero(seen.TraceID[:]) || isZero(seen.SpanID[:]) {
				t.Fatal("generated zero identifiers")
			}
			if _, ok := ParseTraceparent(rec.Header().Get("traceparent")); !ok {
				t.Fatalf("response traceparent %q is not parseable", rec.Header().Get("traceparent"))
			}
		})
	}
}
