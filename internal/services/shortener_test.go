package services_test

import (
    "context"
    "database/sql"
    "regexp"
    "testing"
    "time"

    "github.com/DATA-DOG/go-sqlmock"
    "github.com/go-sql-driver/mysql"
    "go.opentelemetry.io/otel/attribute"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/trace/tracetest"
    "url-shortener/internal/repositories"
    "url-shortener/internal/services"
)

type mockRepository struct {
    storeErr  error
    getResult string
    getErr    error
}

func (m *mockRepository) StoreURL(ctx context.Context, shortKey, originalURL string) error {
    return m.storeErr
}

func (m *mockRepository) GetURL(ctx context.Context, shortKey string) (string, error) {
    return m.getResult, m.getErr
}

func (m *mockRepository) PingDB(ctx context.Context) error {
    return nil
}

func (m *mockRepository) PingCache(ctx context.Context) error {
    return nil
}

func (m *mockRepository) Close() error {
    return nil
}

func TestShortenerService_CreateShortURL(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    repo := &mockRepository{}
    svc := services.NewShortenerService(repo)

    ctx := context.Background()
    originalURL := "https://google.com"

    // Test success case
    t.Run("Success", func(t *testing.T) {
        recorder.Reset()
        repo.storeErr = nil

        shortKey, err := svc.CreateShortURL(ctx, originalURL)
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }
        if len(shortKey) != 9 {
            t.Errorf("Expected short key length 9, got %d", len(shortKey))
        }
        if matched, _ := regexp.MatchString("^[0-9a-zA-Z]{9}$", shortKey); !matched {
            t.Errorf("Short key %s is not Base62", shortKey)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "CreateShortURL" {
                attrs := span.Attributes()
                if hasAttribute(attrs, "original_url", originalURL) && hasAttribute(attrs, "short_key", shortKey) {
                    found = true
                }
            }
        }
        if !found {
            t.Errorf("Expected CreateShortURL span with correct attributes")
        }
    })

    // Test invalid URL
    t.Run("InvalidURL", func(t *testing.T) {
        recorder.Reset()
        _, err := svc.CreateShortURL(ctx, "invalid")
        if err == nil || err.Error() != "invalid URL format: parse \"invalid\": invalid URI for request" {
            t.Errorf("Expected invalid URL error, got %v", err)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "CreateShortURL" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in CreateShortURL span")
        }
    })

    // Test duplicate key
    t.Run("DuplicateKey", func(t *testing.T) {
        recorder.Reset()
        repo.storeErr = &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}

        _, err := svc.CreateShortURL(ctx, originalURL)
        if err == nil || err.Error() != "max attempts reached for unique short key" {
            t.Errorf("Expected max attempts error, got %v", err)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "CreateShortURL" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in CreateShortURL span")
        }
    })
}

func TestShortenerService_GetOriginalURL(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    repo := &mockRepository{}
    svc := services.NewShortenerService(repo)

    ctx := context.Background()
    shortKey := "g20hi3k9Z"
    originalURL := "https://google.com"

    // Test success case
    t.Run("Success", func(t *testing.T) {
        recorder.Reset()
        repo.getResult = originalURL
        repo.getErr = nil

        result, err := svc.GetOriginalURL(ctx, shortKey)
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }
        if result != originalURL {
            t.Errorf("Expected %s, got %s", originalURL, result)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "GetOriginalURL" {
                attrs := span.Attributes()
                if hasAttribute(attrs, "short_key", shortKey) && hasAttribute(attrs, "original_url", originalURL) {
                    found = true
                }
            }
        }
        if !found {
            t.Errorf("Expected GetOriginalURL span with correct attributes")
        }
    })

    // Test not found
    t.Run("NotFound", func(t *testing.T) {
        recorder.Reset()
        repo.getErr = sql.ErrNoRows

        _, err := svc.GetOriginalURL(ctx, shortKey)
        if !errors.Is(err, sql.ErrNoRows) {
            t.Errorf("Expected sql.ErrNoRows, got %v", err)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "GetOriginalURL" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in GetOriginalURL span")
        }
    })

    // Test empty short key
    t.Run("EmptyShortKey", func(t *testing.T) {
        recorder.Reset()
        _, err := svc.GetOriginalURL(ctx, "")
        if err == nil || err.Error() != "short key is empty" {
            t.Errorf("Expected empty short key error, got %v", err)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "GetOriginalURL" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in GetOriginalURL span")
        }
    })
}

func hasAttribute(attrs []attribute.KeyValue, key, value string) bool {
    for _, attr := range attrs {
        if attr.Key == attribute.Key(key) && attr.Value.AsString() == value {
            return true
        }
    }
    return false
}
