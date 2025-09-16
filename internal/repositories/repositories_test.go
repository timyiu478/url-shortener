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
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
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

    redisClient := redis.NewClient(&redis.Options{})
    defer redisClient.Close()

    repo := &repositories.urlRepository{
        db:    db,
        cache: redisClient,
    }

    ctx := context.Background()
    shortKey := "g20hi3k9Z"
    originalURL := "https://google.com"

    // Test success case
    t.Run("Success", func(t *testing.T) {
        recorder.Reset()
        mock.ExpectExec("INSERT INTO urls").
            WithArgs(shortKey, originalURL, sqlmock.AnyArg()).
            WillReturnResult(sqlmock.NewResult(1, 1))
        redisClient.FlushAll(ctx)

        err := repo.StoreURL(ctx, shortKey, originalURL)
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }

        // Verify cache
        val, err := redisClient.Get(ctx, shortKey).Result()
        if err != nil || val != originalURL {
            t.Errorf("Expected cache value %s, got %s, err %v", originalURL, val, err)
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
        recorder.Reset()
        mock.ExpectExec("INSERT INTO urls").
            WithArgs(shortKey, originalURL, sqlmock.AnyArg()).
            WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
        redisClient.FlushAll(ctx)

        err := repo.StoreURL(ctx, shortKey, originalURL)
        if err == nil || !errors.As(err, &mysql.MySQLError{}) {
            t.Errorf("Expected duplicate key error, got %v", err)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "StoreURL" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in StoreURL span")
        }
    })

    // Test cache set error
    t.Run("CacheSetError", func(t *testing.T) {
        recorder.Reset()
        mock.ExpectExec("INSERT INTO urls").
            WithArgs(shortKey, originalURL, sqlmock.AnyArg()).
            WillReturnResult(sqlmock.NewResult(1, 1))
        redisClient.Close() // Simulate cache failure

        err := repo.StoreURL(ctx, shortKey, originalURL)
        if err == nil || !strings.Contains(err.Error(), "failed to set cache") {
            t.Errorf("Expected cache set error, got %v", err)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "CacheSet" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in CacheSet span")
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

    redisClient := redis.NewClient(&redis.Options{})
    defer redisClient.Close()

    repo := &repositories.urlRepository{
        db:    db,
        cache: redisClient,
    }

    ctx := context.Background()
    shortKey := "g20hi3k9Z"
    originalURL := "https://google.com"

    // Test cache hit
    t.Run("CacheHit", func(t *testing.T) {
        recorder.Reset()
        redisClient.FlushAll(ctx)
        redisClient.Set(ctx, shortKey, originalURL, 24*time.Hour)

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
        for _, span := range spans {
            if span.Name() == "GetURL" {
                attrs := span.Attributes()
                if !hasAttribute(attrs, "source", "cache") {
                    t.Errorf("Expected source=cache attribute")
                }
            }
        }
    })

    // Test cache miss, DB hit
    t.Run("CacheMissDBHit", func(t *testing.T) {
        recorder.Reset()
        redisClient.FlushAll(ctx)
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

        // Verify cache
        val, err := redisClient.Get(ctx, shortKey).Result()
        if err != nil || val != originalURL {
            t.Errorf("Expected cache value %s, got %s, err %v", originalURL, val, err)
        }

        spans := recorder.Ended()
        if len(spans) != 3 || spans[0].Name() != "CacheGet" || spans[1].Name() != "DBQuery" || spans[2].Name() != "CacheSet" {
            t.Errorf("Expected CacheGet, DBQuery, CacheSet spans, got %v", spans)
        }
        for _, span := range spans {
            if span.Name() == "GetURL" {
                attrs := span.Attributes()
                if !hasAttribute(attrs, "source", "database") || !hasAttribute(attrs, "original_url", originalURL) {
                    t.Errorf("Expected source=database and original_url attributes")
                }
            }
        }
    })

    // Test cache miss, DB miss
    t.Run("CacheMissDBMiss", func(t *testing.T) {
        recorder.Reset()
        redisClient.FlushAll(ctx)
        mock.ExpectQuery("SELECT original_url FROM urls").
            WithArgs(shortKey).
            WillReturnError(sql.ErrNoRows)

        _, err := repo.GetURL(ctx, shortKey)
        if !errors.Is(err, sql.ErrNoRows) {
            t.Errorf("Expected sql.ErrNoRows, got %v", err)
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "DBQuery" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in DBQuery span")
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

    // Success case
    t.Run("Success", func(t *testing.T) {
        recorder.Reset()
        mock.ExpectPing()

        err := repo.PingDB(context.Background())
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }

        spans := recorder.Ended()
        if len(spans) != 1 || spans[0].Name() != "PingDB" {
            t.Errorf("Expected PingDB span, got %v", spans)
        }
    })

    // Failure case
    t.Run("Failure", func(t *testing.T) {
        recorder.Reset()
        mock.ExpectPing().WillReturnError(errors.New("db connection failed"))

        err := repo.PingDB(context.Background())
        if err == nil {
            t.Errorf("Expected error, got nil")
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "PingDB" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in PingDB span")
        }
    })
}

func TestURLRepository_PingCache(t *testing.T) {
    recorder := tracetest.NewSpanRecorder()
    provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
    defer provider.Shutdown(context.Background())

    redisClient := redis.NewClient(&redis.Options{})
    defer redisClient.Close()

    repo := &repositories.urlRepository{cache: redisClient}

    // Success case
    t.Run("Success", func(t *testing.T) {
        recorder.Reset()
        err := repo.PingCache(context.Background())
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }

        spans := recorder.Ended()
        if len(spans) != 1 || spans[0].Name() != "PingCache" {
            t.Errorf("Expected PingCache span, got %v", spans)
        }
    })

    // Failure case
    t.Run("Failure", func(t *testing.T) {
        recorder.Reset()
        redisClient.Close() // Simulate failure
        err := repo.PingCache(context.Background())
        if err == nil {
            t.Errorf("Expected error, got nil")
        }

        spans := recorder.Ended()
        found := false
        for _, span := range spans {
            if span.Name() == "PingCache" {
                for _, event := range span.Events() {
                    if event.Name == "exception" {
                        found = true
                        break
                    }
                }
            }
        }
        if !found {
            t.Errorf("Expected error event in PingCache span")
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
