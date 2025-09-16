package handlers_test

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/go-chi/chi/v5"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
    sdkmetric "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/sdk/metric/metricdata"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/trace/tracetest"
    "github.com/your-org/url-shortener/internal/handlers"
    "github.com/your-org/url-shortener/internal/services"
)

type mockShortener struct {
    createResult string
    createErr    error
    getResult    string
    getErr       error
}

func (m *mockShortener) CreateShortURL(ctx context.Context, originalURL string) (string, error) {
    return m.createResult, m.createErr
}

func (m *mockShortener) GetOriginalURL(ctx context.Context, shortKey string) (string, error) {
    return m.getResult, m.getErr
}

type mockRepository struct {
    pingDBErr    error
    pingCacheErr error
}

func (m *mockRepository) StoreURL(ctx context.Context, shortKey, originalURL string) error {
    return nil
}

func (m *mockRepository) GetURL(ctx context.Context, shortKey string) (string, error) {
    return "", nil
}

func (m *mockRepository) PingDB(ctx context.Context) error {
    return m.pingDBErr
}

func (m *mockRepository) PingCache(ctx context.Context) error {
    return m.pingCacheErr
}

func (m *mockRepository) Close() error {
    return nil
}

func TestHandler_CreateShortURL(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    meterProvider := sdkmetric.NewMeterProvider()
    meter := meterProvider.Meter("url-shortener")
    latency, _ := meter.Float64Histogram("http_request_latency_ms")
    errors, _ := meter.Int64Counter("http_request_errors_total")

    mockSvc := &mockShortener{createResult: "g20hi3k9Z", createErr: nil}
    mockRepo := &mockRepository{}
    h := handlers.NewHandler(mockSvc, mockRepo)

    r := chi.NewRouter()
    r.Use(h.TracingMiddleware())
    r.Use(h.MetricsMiddleware(latency, errors))
    r.Post("/newurl", h.CreateShortURL)

    reqBody, _ := json.Marshal(map[string]string{"domain": "shortenurl.org", "url": "https://google.com"})
    req := httptest.NewRequest("POST", "/newurl", bytes.NewReader(reqBody))
    rr := httptest.NewRecorder()

    r.ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rr.Code)
    }

    var resp map[string]string
    json.Unmarshal(rr.Body.Bytes(), &resp)
    if resp["shortenUrl"] != "https://shortenurl.org/g20hi3k9Z" {
        t.Errorf("Expected shortenUrl https://shortenurl.org/g20hi3k9Z, got %s", resp["shortenUrl"])
    }

    spans := recorder.Ended()
    if len(spans) < 1 || spans[0].Name() != "POST /newurl" {
        t.Errorf("Expected POST /newurl span, got %v", spans)
    }
}

func TestHandler_GetOriginalURL(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    meterProvider := sdkmetric.NewMeterProvider()
    meter := meterProvider.Meter("url-shortener")
    latency, _ := meter.Float64Histogram("http_request_latency_ms")
    errors, _ := meter.Int64Counter("http_request_errors_total")

    mockSvc := &mockShortener{getResult: "https://google.com", getErr: nil}
    mockRepo := &mockRepository{}
    h := handlers.NewHandler(mockSvc, mockRepo)

    r := chi.NewRouter()
    r.Use(h.TracingMiddleware())
    r.Use(h.MetricsMiddleware(latency, errors))
    r.Get("/{shortKey}", h.GetOriginalURL)

    req := httptest.NewRequest("GET", "/g20hi3k9Z", nil)
    rr := httptest.NewRecorder()

    r.ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rr.Code)
    }

    spans := recorder.Ended()
    if len(spans) < 1 || spans[0].Name() != "GET /{shortKey}" {
        t.Errorf("Expected GET /{shortKey} span, got %v", spans)
    }
}

func TestHandler_HealthCheck(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    meterProvider := sdkmetric.NewMeterProvider()
    meter := meterProvider.Meter("url-shortener")
    latency, _ := meter.Float64Histogram("http_request_latency_ms")
    errors, _ := meter.Int64Counter("http_request_errors_total")

    h := handlers.NewHandler(nil, nil)

    r := chi.NewRouter()
    r.Use(h.TracingMiddleware())
    r.Use(h.MetricsMiddleware(latency, errors))
    r.Get("/healthz", h.HealthCheck)

    req := httptest.NewRequest("GET", "/healthz", nil)
    rr := httptest.NewRecorder()

    r.ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rr.Code)
    }
    if rr.Body.String() != "OK" {
        t.Errorf("Expected body OK, got %s", rr.Body.String())
    }

    spans := recorder.Ended()
    if len(spans) != 1 || spans[0].Name() != "GET /healthz" {
        t.Errorf("Expected GET /healthz span, got %v", spans)
    }
}

func TestHandler_ReadinessCheck(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    meterProvider := sdkmetric.NewMeterProvider()
    meter := meterProvider.Meter("url-shortener")
    latency, _ := meter.Float64Histogram("http_request_latency_ms")
    errors, _ := meter.Int64Counter("http_request_errors_total")

    mockRepo := &mockRepository{pingDBErr: nil, pingCacheErr: nil}
    h := handlers.NewHandler(nil, mockRepo)

    r := chi.NewRouter()
    r.Use(h.TracingMiddleware())
    r.Use(h.MetricsMiddleware(latency, errors))
    r.Get("/readyz", h.ReadinessCheck)

    // Test success
    req := httptest.NewRequest("GET", "/readyz", nil)
    rr := httptest.NewRecorder()

    r.ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rr.Code)
    }
    if rr.Body.String() != "OK" {
        t.Errorf("Expected body OK, got %s", rr.Body.String())
    }

    // Test DB failure
    mockRepo.pingDBErr = errors.New("db down")
    rr = httptest.NewRecorder()
    r.ServeHTTP(rr, req)

    if rr.Code != http.StatusServiceUnavailable {
        t.Errorf("Expected status 503, got %d", rr.Code)
    }

    spans := recorder.Ended()
    if len(spans) < 1 || spans[0].Name() != "GET /readyz" {
        t.Errorf("Expected GET /readyz span, got %v", spans)
    }
}

func TestHandler_Metrics(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    meterProvider := sdkmetric.NewMeterProvider()
    meter := meterProvider.Meter("url-shortener")
    latency, _ := meter.Float64Histogram("http_request_latency_ms")
    errors, _ := meter.Int64Counter("http_request_errors_total")

    h := handlers.NewHandler(nil, nil)

    r := chi.NewRouter()
    r.Use(h.TracingMiddleware())
    r.Use(h.MetricsMiddleware(latency, errors))
    r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("metrics"))
    })

    req := httptest.NewRequest("GET", "/metrics", nil)
    rr := httptest.NewRecorder()

    r.ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rr.Code)
    }

    spans := recorder.Ended()
    if len(spans) != 1 || spans[0].Name() != "GET /metrics" {
        t.Errorf("Expected GET /metrics span, got %v", spans)
    }
}
