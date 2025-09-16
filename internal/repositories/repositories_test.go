package repositories_test

import (
    "context"
    "database/sql"
    "errors"
    "testing"
    "time"

    "github.com/DATA-DOG/go-sqlmock"
    "github.com/go-sql-driver/mysql"
    "github.com/redis/go-redis/v9"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/trace/tracetest"
    "github.com/your-org/url-shortener/internal/config"
    "github.com/your-org/url-shortener/internal/repositories"
)

func TestURLRepository_StoreURL(t *testing.T) {
    // Setup OpenTelemetry test recorder
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    // Setup mocks
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Failed to create sqlmock: %v", err)
    }
    defer db.Close()

    mockRedis := redis.NewClient(&redis.Options{})
    mockRedisCmd := redis.NewStringCmd(context.Background())

    repo := &repositories.urlRepository{
        db:    db,
        cache: mockRedis,
    }

    ctx := context.Background()
    shortKey := "g20hi3k9Z"
    originalURL := "https://google.com"

    // Test success case
    t.Run("Success", func(t *testing.T) {
        mock.ExpectExec("INSERT INTO urls").
            WithArgs(shortKey, originalURL, sqlmock.AnyArg()).
            WillReturnResult(sqlmock.NewResult(1, 1))
        mockRedis.FlushDB(ctx)
        mockRedisCmd.SetVal("OK")

        err := repo.StoreURL(ctx, shortKey, originalURL)
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }

        // Verify spans
        spans := recorder.Ended()
        if len(spans) != 2 {
            t.Errorf("Expected 2 spans, got %d", len(spans))
        }
        for _, span := range spans {
            if span.Name() == "StoreURL" {
                attrs := span.Attributes()
                if !hasAttribute(attrs, "short_key", shortKey) || !hasAttribute(attrs, "original_url", originalURL) {
                    t.Errorf("Missing expected attributes in StoreURL span")
                }
            }
        }
    })

    // Test duplicate key error
    t.Run("DuplicateKey", func(t *testing.T) {
        mock.ExpectExec("INSERT INTO urls").
            WithArgs(shortKey, originalURL, sqlmock.AnyArg()).
            WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})

        err := repo.StoreURL(ctx, shortKey, originalURL)
        if err == nil || !errors.As(err, &mysql.MySQLError{}) {
            t.Errorf("Expected duplicate key error, got %v", err)
        }

        spans := recorder.Ended()
        for _, span := range spans {
            if span.Name() == "StoreURL" && len(span.Events()) == 0 {
                t.Errorf("Expected error event in StoreURL span")
            }
        }
    })
}

func TestURLRepository_GetURL(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Failed to create sqlmock: %v", err)
    }
    defer db.Close()

    mockRedis := redis.NewClient(&redis.Options{})
    mockRedisCmd := redis.NewStringCmd(context.Background())

    repo := &repositories.urlRepository{
        db:    db,
        cache: mockRedis,
    }

    ctx := context.Background()
    shortKey := "g20hi3k9Z"
    originalURL := "https://google.com"

    // Test cache hit
    t.Run("CacheHit", func(t *testing.T) {
        mockRedis.FlushDB(ctx)
        mockRedis.Set(ctx, shortKey, originalURL, 24*time.Hour)
        mockRedisCmd.SetVal(originalURL)

        result, err := repo.GetURL(ctx, shortKey)
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }
        if result != originalURL {
            t.Errorf("Expected %s, got %s", originalURL, result)
        }

        spans := recorder.Ended()
        if len(spans) != 1 || spans[0].Name() != "CacheGet" {
            t.Errorf("Expected CacheGet span, got %v", spans)
        }
    })

    // Test cache miss, DB hit
    t.Run("CacheMissDBHit", func(t *testing.T) {
        mockRedis.FlushDB(ctx)
        mockRedisCmd.SetErr(redis.Nil)
        mock.ExpectQuery("SELECT original_url FROM urls").
            WithArgs(shortKey).
            WillReturnRows(sqlmock.NewRows([]string{"original_url"}).AddRow(originalURL))

        result, err := repo.GetURL(ctx, shortKey)
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }
        if result != originalURL {
            t.Errorf("Expected %s, got %s", originalURL, result)
        }

        spans := recorder.Ended()
        if len(spans) != 3 || spans[0].Name() != "CacheGet" || spans[1].Name() != "DBQuery" || spans[2].Name() != "CacheSet" {
            t.Errorf("Expected CacheGet, DBQuery, CacheSet spans, got %v", spans)
        }
    })
}

func TestURLRepository_PingDB(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Failed to create sqlmock: %v", err)
    }
    defer db.Close()

    repo := &repositories.urlRepository{db: db}

    mock.ExpectPing()

    err = repo.PingDB(context.Background())
    if err != nil {
        t.Errorf("Expected no error, got %v", err)
    }

    spans := recorder.Ended()
    if len(spans) != 1 || spans[0].Name() != "PingDB" {
        t.Errorf("Expected PingDB span, got %v", spans)
    }
}

func TestURLRepository_PingCache(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    mockRedis := redis.NewClient(&redis.Options{})
    mockRedisCmd := redis.NewStringCmd(context.Background())
    mockRedisCmd.SetVal("PONG")

    repo := &repositories.urlRepository{cache: mockRedis}

    err := repo.PingCache(context.Background())
    if err != nil {
        t.Errorf("Expected no error, got %v", err)
    }

    spans := recorder.Ended()
    if len(spans) != 1 || spans[0].Name() != "PingCache" {
        t.Errorf("Expected PingCache span, got %v", spans)
    }
}

func hasAttribute(attrs []attribute.KeyValue, key, value string) bool {
    for _, attr := range attrs {
        if attr.Key == attribute.Key(key) && attr.Value.AsString() == value {
            return true
        }
    }
    return false
}
