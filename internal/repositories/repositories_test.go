package repositories_test

import (
    "context"
    "database/sql"
    "errors"
    "strings"
    "testing"
    "time"

    "github.com/DATA-DOG/go-sqlmock"
    "github.com/go-sql-driver/mysql"
    "github.com/redis/go-redis/v9"
    "url-shortener/internal/repositories"
)

func TestURLRepository_StoreURL(t *testing.T) {
    // Setup mocks
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Failed to create sqlmock: %v", err)
    }
    defer db.Close()

    redisClient := redis.NewClient(&redis.Options{})
    defer redisClient.Close()

    repo := repositories.urlRepository{
        db:    db,
        cache: redisClient,
    }

    ctx := context.Background()
    shortKey := "g20hi3k9Z"
    originalURL := "https://google.com"

    // Test success case
    t.Run("Success", func(t *testing.T) {
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
    })

    // Test duplicate key error
    t.Run("DuplicateKey", func(t *testing.T) {
        mock.ExpectExec("INSERT INTO urls").
            WithArgs(shortKey, originalURL, sqlmock.AnyArg()).
            WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
        redisClient.FlushAll(ctx)

        err := repo.StoreURL(ctx, shortKey, originalURL)
        if err == nil || !errors.As(err, &mysql.MySQLError{}) {
            t.Errorf("Expected duplicate key error, got %v", err)
        }
    })

    // Test cache set error
    t.Run("CacheSetError", func(t *testing.T) {
        mock.ExpectExec("INSERT INTO urls").
            WithArgs(shortKey, originalURL, sqlmock.AnyArg()).
            WillReturnResult(sqlmock.NewResult(1, 1))
        redisClient.Close() // Simulate cache failure

        err := repo.StoreURL(ctx, shortKey, originalURL)
        if err == nil || !strings.Contains(err.Error(), "failed to set cache") {
            t.Errorf("Expected cache set error, got %v", err)
        }
    })
}

func TestURLRepository_GetURL(t *testing.T) {
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
        redisClient.FlushAll(ctx)
        redisClient.Set(ctx, shortKey, originalURL, 24*time.Hour)

        result, err := repo.GetURL(ctx, shortKey)
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }
        if result != originalURL {
            t.Errorf("Expected %s, got %s", originalURL, result)
        }
    })

    // Test cache miss, DB hit
    t.Run("CacheMissDBHit", func(t *testing.T) {
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
    })

    // Test cache miss, DB miss
    t.Run("CacheMissDBMiss", func(t *testing.T) {
        redisClient.FlushAll(ctx)
        mock.ExpectQuery("SELECT original_url FROM urls").
            WithArgs(shortKey).
            WillReturnError(sql.ErrNoRows)

        _, err := repo.GetURL(ctx, shortKey)
        if !errors.Is(err, sql.ErrNoRows) {
            t.Errorf("Expected sql.ErrNoRows, got %v", err)
        }
    })
}

func TestURLRepository_PingDB(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Failed to create sqlmock: %v", err)
    }
    defer db.Close()

    repo := &repositories.urlRepository{db: db}

    // Success case
    t.Run("Success", func(t *testing.T) {
        mock.ExpectPing()

        err := repo.PingDB(context.Background())
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }
    })

    // Failure case
    t.Run("Failure", func(t *testing.T) {
        mock.ExpectPing().WillReturnError(errors.New("db connection failed"))

        err := repo.PingDB(context.Background())
        if err == nil {
            t.Errorf("Expected error, got nil")
        }
    })
}

func TestURLRepository_PingCache(t *testing.T) {
    redisClient := redis.NewClient(&redis.Options{})
    defer redisClient.Close()

    repo := &repositories.urlRepository{cache: redisClient}

    // Success case
    t.Run("Success", func(t *testing.T) {
        err := repo.PingCache(context.Background())
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }
    })

    // Failure case
    t.Run("Failure", func(t *testing.T) {
        redisClient.Close() // Simulate failure
        err := repo.PingCache(context.Background())
        if err == nil {
            t.Errorf("Expected error, got nil")
        }
    })
}
