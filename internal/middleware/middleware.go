package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"log/slog"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	"github.com/prometheus/client_golang/prometheus"
)

type ctxKey string

const loggerKey ctxKey = "logger_with_trace"

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "endpoint", "status_code"},
	)

	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP request durations in seconds", Buckets: prometheus.ExponentialBuckets(0.001, 2, 15)},
		[]string{"method", "endpoint"},
	)

	urlRedirectsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "url_redirects_total", Help: "Total successful URL redirects (business metric)"},
	)

	dbQueryDurationSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{Name: "db_query_duration_seconds", Help: "Database query durations in seconds", Buckets: prometheus.ExponentialBuckets(0.0005, 2, 16)},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDurationSeconds, urlRedirectsTotal, dbQueryDurationSeconds)
}

// OtelMiddleware instruments incoming requests with OpenTelemetry.
func OtelMiddleware(next http.Handler) http.Handler {
	return otelhttp.NewHandler(next, "http.request")
}

// TraceLoggerMiddleware extracts the trace id from the context and attaches
// a logger (slog.Logger) with trace_id into the request context so handlers
// and services can pull it.
func TraceLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		sc := trace.SpanContextFromContext(ctx)
		logger := slog.Default()
		// If the span context contains a valid trace ID, attach it to a request logger.
		if sc.TraceID().IsValid() {
			logger = logger.With("trace_id", sc.TraceID().String())
		}
		ctx = context.WithValue(ctx, loggerKey, logger)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext returns the logger attached in context by TraceLoggerMiddleware,
// or the global default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if v := ctx.Value(loggerKey); v != nil {
		switch l := v.(type) {
		case *slog.Logger:
			return l
		case slog.Logger:
			// stored as a value; return pointer to a copy
			return &l
		}
	}
	return slog.Default()
}

// PrometheusMetricsMiddleware collects RED metrics: rate, errors, duration.
func PrometheusMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Capture response status code by using a ResponseWriter wrapper
		rr := &statusRecordingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rr, r)
		duration := time.Since(start).Seconds()

		endpoint := r.URL.Path
		method := r.Method
		status := strconv.Itoa(rr.statusCode)

		httpRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
		httpRequestDurationSeconds.WithLabelValues(method, endpoint).Observe(duration)

		// Note: redirect business metric should be incremented by handler when redirect occurs
	})
}

type statusRecordingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusRecordingResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// ObserveDBQuery records DB latency.
func ObserveDBQuery(durationSeconds float64) {
	dbQueryDurationSeconds.Observe(durationSeconds)
}

// UrlRedirectsInc increments business redirect counter.
func UrlRedirectsInc() {
	urlRedirectsTotal.Inc()
}
