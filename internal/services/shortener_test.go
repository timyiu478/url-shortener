package services_test

import (
    "context"
    "database/sql"
    "regexp"
    "testing"
		"errors"

    "github.com/go-sql-driver/mysql"
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
    repo := &mockRepository{}
    svc := services.NewShortenerService(repo)

    ctx := context.Background()
    originalURL := "https://google.com"

    // Test success case
    t.Run("Success", func(t *testing.T) {
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
    })

    // Test invalid URL
    t.Run("InvalidURL", func(t *testing.T) {
        _, err := svc.CreateShortURL(ctx, "!invalid")
        if err == nil || err.Error() != "invalid URL format" {
            t.Errorf("Expected invalid URL error, got %v", err)
        }
    })

    // Test duplicate key
    t.Run("DuplicateKey", func(t *testing.T) {
        repo.storeErr = &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}

        _, err := svc.CreateShortURL(ctx, originalURL)
        if err == nil || err.Error() != "max attempts reached for unique short key" {
            t.Errorf("Expected max attempts error, got %v", err)
        }
    })
}

func TestShortenerService_GetOriginalURL(t *testing.T) {
    repo := &mockRepository{}
    svc := services.NewShortenerService(repo)

    ctx := context.Background()
    shortKey := "g20hi3k9Z"
    originalURL := "https://google.com"

    // Test success case
    t.Run("Success", func(t *testing.T) {
        repo.getResult = originalURL
        repo.getErr = nil

        result, err := svc.GetOriginalURL(ctx, shortKey)
        if err != nil {
            t.Errorf("Expected no error, got %v", err)
        }
        if result != originalURL {
            t.Errorf("Expected %s, got %s", originalURL, result)
        }
    })

    // Test not found
    t.Run("NotFound", func(t *testing.T) {
        repo.getErr = sql.ErrNoRows

        _, err := svc.GetOriginalURL(ctx, shortKey)
        if !errors.Is(err, sql.ErrNoRows) {
            t.Errorf("Expected sql.ErrNoRows, got %v", err)
        }
    })

    // Test empty short key
    t.Run("EmptyShortKey", func(t *testing.T) {
        _, err := svc.GetOriginalURL(ctx, "")
        if err == nil || err.Error() != "short key is empty" {
            t.Errorf("Expected empty short key error, got %v", err)
        }
    })
}
