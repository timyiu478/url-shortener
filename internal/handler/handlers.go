package handlers

import (
    "context"
    "database/sql"
    "net/http"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/trace"
    "github.com/your-org/url-shortener/internal/repositories"
    "github.com/your-org/url-shortener/internal/services"
)

var tracer = otel.Tracer("url-shortener-handlers")

type Handler struct {
    svc  services.Shortener
    repo repositories.URLRepository
}

func NewHandler(svc services.Shortener, repo repositories.URLRepository) *Handler {
    return &Handler{svc: svc, repo: repo}
}

// TracingMiddleware adds distributed tracing to HTTP requests
func (h *Handler) TracingMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
            ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path, trace.WithAttributes(
                attribute.String("http.method", r.Method),
                attribute.String("http.url", r.URL.String()),
            ))
            defer span.End()

            r = r.WithContext(ctx)
            next.ServeHTTP(w, r)
        })
    }
}

// HealthCheck handles liveness probe (HTTP server is running)
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

// ReadinessCheck handles readiness probe (dependencies are healthy)
func (h *Handler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()

    if err := h.repo.PingDB(ctx); err != nil {
        http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
        return
    }

    if err := h.repo.PingCache(ctx); err != nil {
        http.Error(w, "Cache unhealthy", http.StatusServiceUnavailable)
        return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

// MetricsMiddleware instruments HTTP requests with latency and error metrics
func (h *Handler) MetricsMiddleware(latency metric.Float64Histogram, errors metric.Int64Counter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

            next.ServeHTTP(rw, r)

            duration := time.Since(start).Milliseconds()
            endpoint := getEndpoint(r)
            status := http.StatusText(rw.statusCode)

            latency.Record(r.Context(), float64(duration),
                metric.WithAttributes(
                    attribute.String("endpoint", endpoint),
                    attribute.String("status", status),
                ))

            if rw.statusCode >= 400 {
                errorType := getErrorType(rw.statusCode, r.Method)
                errors.Add(r.Context(), 1,
                    metric.WithAttributes(
                        attribute.String("endpoint", endpoint),
                        attribute.String("error_type", errorType),
                    ))
            }
        })
    }
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

// getEndpoint extracts the endpoint pattern
func getEndpoint(r *http.Request) string {
    route := chi.RouteContext(r.Context())
    if route != nil {
        return route.RoutePattern()
    }
    return r.URL.Path
}

// getErrorType determines the error type based on status and method
func getErrorType(statusCode int, method string) string {
    if statusCode == http.StatusNotFound && method == "GET" {
        return "not_found"
    }
    if statusCode == http.StatusConflict && method == "POST" {
        return "duplicate_key"
    }
    return "other"
}

func (h *Handler) CreateShortURL(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...
}

func (h *Handler) GetOriginalURL(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...
}
