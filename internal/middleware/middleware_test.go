package middleware_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"

	"url-shortener/internal/middleware"
)

func TestTraceLoggerMiddleware_IncludesTraceIDInLogs(t *testing.T) {
	// Capture logs to buffer
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{})))

	// Create a fake span context with a known trace id
	var tid trace.TraceID
	copy(tid[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, TraceFlags: trace.FlagsSampled})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log using the logger from context
		logger := middleware.FromContext(r.Context())
		logger.Info("handler log test")
		w.WriteHeader(http.StatusNoContent)
	})

	wrapped := middleware.TraceLoggerMiddleware(h)
	wrapped.ServeHTTP(rr, req)

	// Ensure logged JSON contains the trace id
	out, _ := io.ReadAll(&buf)
	if !strings.Contains(string(out), sc.TraceID().String()) {
		t.Fatalf("expected log to contain trace_id %s, got: %s", sc.TraceID().String(), string(out))
	}
}

func TestPrometheusMetricsMiddleware_RecordsMetrics(t *testing.T) {
	// Handler that returns 204
	svc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux := http.NewServeMux()
	mux.Handle("/test", middleware.PrometheusMetricsMiddleware(svc))
	mux.Handle("/metrics", promhttp.Handler())

	server := httptest.NewServer(mux)
	defer server.Close()

	// Send a request to /test
	resp, err := http.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Fetch metrics
	mresp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("metrics request failed: %v", err)
	}
	defer mresp.Body.Close()
	body, _ := io.ReadAll(mresp.Body)
	metrics := string(body)

	if !strings.Contains(metrics, "http_requests_total") {
		t.Fatalf("expected metrics to contain http_requests_total, got: %s", metrics)
	}

	// Test url redirects counter
	middleware.UrlRedirectsInc()
	mresp2, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("metrics request failed: %v", err)
	}
	defer mresp2.Body.Close()
	body2, _ := io.ReadAll(mresp2.Body)
	metrics2 := string(body2)

	if !strings.Contains(metrics2, "url_redirects_total") {
		t.Fatalf("expected metrics to contain url_redirects_total, got: %s", metrics2)
	}
}
