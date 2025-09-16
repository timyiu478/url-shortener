package handlers_test

import (
    "bytes"
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "net/http"
    "net/http/httptest"
    "regexp"
    "strings"
    "testing"

    "github.com/go-chi/chi/v5"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
    sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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

    // Success case
    t.Run("Success", func(t *testing.T) {
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
        shortenURL := resp["shortenUrl"]
        if !strings.HasPrefix(shortenURL, "https://shortenurl.org/") {
            t.Errorf("Expected shortenUrl to start with https://shortenurl.org/, got %s", shortenURL)
        }
        shortKey := shortenURL[len("https://shortenurl.org/"):]
        if matched, _ := regexp.MatchString("^[0-9a-zA-Z]{9}$", shortKey); !matched {
            t.Errorf("Short key %s is not Base62", shortKey)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "POST /newurl" {
                for _, attr := range span.Attributes() {
                    if attr.Key == "http.status_code" && attr.Value.AsInt64() == 200 {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected POST /newurl span with status 200")
        }
    })

    // Error case: duplicate key
    t.Run("DuplicateKey", func(t *testing.T) {
        recorder.Reset()
        mockSvc := &mockShortener{createErr: &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}}
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

        if rr.Code != http.StatusConflict {
            t.Errorf("Expected status 409, got %d", rr.Code)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "POST /newurl" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in POST /newurl span")
        }
    })

    // Error case: invalid URL
    t.Run("InvalidURL", func(t *testing.T) {
        recorder.Reset()
        mockSvc := &mockShortener{createErr: errors.New("invalid URL format: parse \"invalid\": invalid URI for request")}
        mockRepo := &mockRepository{}
        h := handlers.NewHandler(mockSvc, mockRepo)

        r := chi.NewRouter()
        r.Use(h.TracingMiddleware())
        r.Use(h.MetricsMiddleware(latency, errors))
        r.Post("/newurl", h.CreateShortURL)

        reqBody, _ := json.Marshal(map[string]string{"domain": "shortenurl.org", "url": "invalid"})
        req := httptest.NewRequest("POST", "/newurl", bytes.NewReader(reqBody))
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusBadRequest {
            t.Errorf("Expected status 400, got %d", rr.Code)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "POST /newurl" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in POST /newurl span")
        }
    })
}

func TestHandler_GetOriginalURL(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    meterProvider := sdkmetric.NewMeterProvider()
    meter := meterProvider.Meter("url-shortener")
    latency, _ := meter.Float64Histogram("http_request_latency_ms")
    errors, _ := meter.Int64Counter("http_request_errors_total")

    // Success case
    t.Run("Success", func(t *testing.T) {
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

        if rr.Code != http.StatusFound {
            t.Errorf("Expected status 302, got %d", rr.Code)
        }
        if rr.Header().Get("Location") != "https://google.com" {
            t.Errorf("Expected redirect to https://google.com, got %s", rr.Header().Get("Location"))
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "GET /{shortKey}" {
                for _, attr := range span.Attributes() {
                    if attr.Key == "http.status_code" && attr.Value.AsInt64() == 302 {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected GET /{shortKey} span with status 302")
        }
    })

    // Error case: not found
    t.Run("NotFound", func(t *testing.T) {
        recorder.Reset()
        mockSvc := &mockShortener{getErr: sql.ErrNoRows}
        mockRepo := &mockRepository{}
        h := handlers.NewHandler(mockSvc, mockRepo)

        r := chi.NewRouter()
        r.Use(h.TracingMiddleware())
        r.Use(h.MetricsMiddleware(latency, errors))
        r.Get("/{shortKey}", h.GetOriginalURL)

        req := httptest.NewRequest("GET", "/g20hi3k9Z", nil)
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusNotFound {
            t.Errorf("Expected status 404, got %d", rr.Code)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "GET /{shortKey}" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in GET /{shortKey} span")
        }
    })
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

    // Success case
    t.Run("Success", func(t *testing.T) {
        mockRepo := &mockRepository{pingDBErr: nil, pingCacheErr: nil}
        h := handlers.NewHandler(nil, mockRepo)

        r := chi.NewRouter()
        r.Use(h.TracingMiddleware())
        r.Use(h.MetricsMiddleware(latency, errors))
        r.Get("/readyz", h.ReadinessCheck)

        req := httptest.NewRequest("GET", "/readyz", nil)
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusOK {
            t.Errorf("Expected status 200, got %d", rr.Code)
        }
        if rr.Body.String() != "OK" {
            t.Errorf("Expected body OK, got %s", rr.Body.String())
        }

        spans := recorder.Ended()
        if len(spans) != 1 || spans[0].Name() != "GET /readyz" {
            t.Errorf("Expected GET /readyz span, got %v", spans)
        }
    })

    // Failure case: DB down
    t.Run("DBFailure", func(t *testing.T) {
        recorder.Reset()
        mockRepo := &mockRepository{pingDBErr: errors.New("db down"), pingCacheErr: nil}
        h := handlers.NewHandler(nil, mockRepo)

        r := chi.NewRouter()
        r.Use(h.TracingMiddleware())
        r.Use(h.MetricsMiddleware(latency, errors))
        r.Get("/readyz", h.ReadinessCheck)

        req := httptest.NewRequest("GET", "/readyz", nil)
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusServiceUnavailable {
            t.Errorf("Expected status 503, got %d", rr.Code)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "GET /readyz" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in GET /readyz span")
        }
    })

    // Failure case: Cache down
    t.Run("CacheFailure", func(t *testing.T) {
        recorder.Reset()
        mockRepo := &mockRepository{pingDBErr: nil, pingCacheErr: errors.New("cache down")}
        h := handlers.NewHandler(nil, mockRepo)

        r := chi.NewRouter()
        r.Use(h.TracingMiddleware())
        r.Use(h.MetricsMiddleware(latency, errors))
        r.Get("/readyz", h.ReadinessCheck)

        req := httptest.NewRequest("GET", "/readyz", nil)
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusServiceUnavailable {
            t.Errorf("Expected status 503, got %d", rr.Code)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "GET /readyz" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in GET /readyz span")
        }
    })
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
