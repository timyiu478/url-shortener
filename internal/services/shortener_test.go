package services_test

import (
    "context"
    "regexp"
    "testing"

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
    svc := services.NewShortenerService(repo, 0)

    ctx := context.Background()
    originalURL := "https://google.com"

    // Test success case
    t.Run("Success", func(t *testing.T) {
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
        if err == nil || err.Error() != services.ErrInvalidURLFormat.Error() {
            t.Errorf("Expected invalid URL error, got %v", err)
        }
    })
}

func TestShortenerService_GetOriginalURL(t *testing.T) {
    repo := &mockRepository{}
    svc := services.NewShortenerService(repo, 0)

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
        _, err := svc.GetOriginalURL(ctx, "notfound")
        if err == nil {
            t.Errorf("Expected error, got %v", err)
        }
    })

    // Test empty short key
    t.Run("EmptyShortKey", func(t *testing.T) {
        _, err := svc.GetOriginalURL(ctx, "")
        if err == nil || err.Error() != services.ErrInvalidShortKeyFormat.Error() {
            t.Errorf("Expected empty short key error, got %v", err)
        }
    })
}
