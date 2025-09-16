package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/spf13/viper"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    "go.opentelemetry.io/otel/exporters/prometheus"
    "go.opentelemetry.io/otel/metric"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
    "url-shortener/internal/config"
    "url-shortener/internal/handlers"
    "url-shortener/internal/repositories"
    "url-shortener/internal/services"
)

func main() {
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Initialize OpenTelemetry tracing
    tracerProvider, err := initTracer(cfg.OTLPTraceEndpoint)
    if err != nil {
        log.Fatalf("Failed to initialize tracer: %v", err)
    }
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := tracerProvider.Shutdown(ctx); err != nil {
            log.Printf("Error shutting down tracer provider: %v", err)
        }
    }()
    otel.SetTracerProvider(tracerProvider)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

    // Initialize repository
    repo, err := repositories.NewURLRepository(cfg)
    if err != nil {
        log.Fatalf("Failed to initialize repository: %v", err)
    }
    defer func() {
        if err := repo.Close(); err != nil {
            log.Printf("Error closing repository: %v", err)
        }
    }()

    // Initialize service and handler
    svc := services.NewShortenerService(repo)
    h := handlers.NewHandler(svc, repo)

    // Initialize OpenTelemetry metrics
    exporter, err := prometheus.New()
    if err != nil {
        log.Fatalf("Failed to create Prometheus exporter: %v", err)
    }
    provider := metric.NewMeterProvider(metric.WithReader(exporter))
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := provider.Shutdown(ctx); err != nil {
            log.Printf("Error shutting down metric provider: %v", err)
        }
    }()
    meter := provider.Meter("url-shortener")

    // Define metrics
    requestLatency, err := meter.Float64Histogram(
        "http_request_latency_ms",
        metric.WithDescription("HTTP request latency in milliseconds"),
        metric.WithUnit("ms"),
    )
    if err != nil {
        log.Fatalf("Failed to create latency histogram: %v", err)
    }

    requestErrors, err := meter.Int64Counter(
        "http_request_errors_total",
        metric.WithDescription("Total number of HTTP request errors"),
        metric.WithUnit("1"),
    )
    if err != nil {
        log.Fatalf("Failed to create error counter: %v", err)
    }

    // Set up HTTP router
    r := chi.NewRouter()
    r.Use(h.TracingMiddleware())
    r.Use(h.MetricsMiddleware(requestLatency, requestErrors))
    r.Post("/newurl", h.CreateShortURL)
    r.Get("/{shortKey}", h.GetOriginalURL)
    r.Get("/metrics", exporter.ServeHTTP)
    r.Get("/healthz", h.HealthCheck)
    r.Get("/readyz", h.ReadinessCheck)

    // Start server with timeouts
    srv := &http.Server{
        Addr:         ":" + cfg.Port,
        Handler:      r,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  15 * time.Second,
    }

    // Handle graceful shutdown
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

    go func() {
        log.Printf("Starting server on port %s", cfg.Port)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server error: %v", err)
        }
    }()

    <-stop
    log.Println("Shutting down server...")
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("Server shutdown error: %v", err)
    }
    log.Println("Server stopped")
}

// initTracer sets up OpenTelemetry tracing with OTLP exporter
func initTracer(otlpEndpoint string) (*trace.TracerProvider, error) {
    ctx := context.Background()

    if otlpEndpoint == "" {
        return nil, fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT is not set")
    }

    exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(otlpEndpoint))
    if err != nil {
        return nil, err
    }

    res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceNameKey.String("url-shortener")))
    if err != nil {
        return nil, err
    }

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(res),
        trace.WithSampler(trace.ParentBased(trace.AlwaysSample())),
    )

    return tp, nil
}
