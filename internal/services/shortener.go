package services

import (
    "context"
    "crypto/rand"
    "errors"
    "fmt"
    "net/url"

    "github.com/go-sql-driver/mysql"
    "url-shortener/internal/repositories"
)

// Base62 characters
const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type Shortener interface {
    CreateShortURL(ctx context.Context, originalURL string) (string, error)
    GetOriginalURL(ctx context.Context, shortKey string) (string, error)
}

type shortenerService struct {
    repo repositories.URLRepository
}

func NewShortenerService(repo repositories.URLRepository) Shortener {
    return &shortenerService{repo: repo}
}

func (s *shortenerService) CreateShortURL(ctx context.Context, originalURL string) (string, error) {
    // Validate URL
    if originalURL == "" {
        err := errors.New("original URL is empty")
        return "", err
    }
    if _, err := url.ParseRequestURI(originalURL); err != nil {
        err = fmt.Errorf("invalid URL format: %w", err)
        return "", err
    }

    const maxAttempts = 3
    for attempt := 0; attempt < maxAttempts; attempt++ {
        shortKey, err := generateShortKey()
        if err != nil {
            return "", err
        }
        err = s.repo.StoreURL(ctx, shortKey, originalURL)
        if err == nil {
            return shortKey, nil
        }
        if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
            continue
        }
        return "", err
    }
    err := errors.New("max attempts reached for unique short key")
    return "", err
}

func (s *shortenerService) GetOriginalURL(ctx context.Context, shortKey string) (string, error) {
    if shortKey == "" {
        err := errors.New("short key is empty")
        return "", err
    }

    originalURL, err := s.repo.GetURL(ctx, shortKey)
    if err != nil {
        return "", err
    }
    return originalURL, nil
}

func generateShortKey() (string, error) {
    b := make([]byte, 7) // ~9 chars in Base62
    _, err := rand.Read(b)
    if err != nil {
        return "", fmt.Errorf("failed to generate random bytes: %w", err)
    }
    // Convert to Base62
    var num uint64
    for i, v := range b {
        num |= uint64(v) << (8 * i)
    }
    result := make([]byte, 0, 9)
    for num > 0 && len(result) < 9 {
        result = append(result, base62Chars[num%62])
        num /= 62
    }
    // Pad with '0' if necessary to ensure 9 characters
    for len(result) < 9 {
        result = append(result, '0')
    }
    // Reverse the result to get correct order
    for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
        result[i], result[j] = result[j], result[i]
    }
    return string(result), nil
}
