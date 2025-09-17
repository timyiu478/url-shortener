package services

import (
	  "log/slog"
    "context"
		"regexp"
    "crypto/rand"
		"math/big"
    "fmt"
    "net/url"

    "url-shortener/internal/repositories"
)

var (
	ErrCreateShortURLFailed = fmt.Errorf("failed to create short URL")
	ErrInvalidURLFormat = fmt.Errorf("invalid URL format")
	ErrOriginalURLEmpty = fmt.Errorf("original URL is empty")
	ErrShortKeyNotFound = fmt.Errorf("short key is not found")
	ErrInvalidShortKeyFormat = fmt.Errorf("invalid short key format")
)

type Shortener interface {
    CreateShortURL(ctx context.Context, originalURL string) (string, error)
    GetOriginalURL(ctx context.Context, shortKey string) (string, error)
}

type shortenerService struct {
    repo repositories.URLRepository
	  shardId int
}

func NewShortenerService(repo repositories.URLRepository, shardId int) Shortener {
	return &shortenerService{repo: repo, shardId: shardId}
}

func (s *shortenerService) CreateShortURL(ctx context.Context, originalURL string) (string, error) {
    // Validate URL
    if originalURL == "" {
			  slog.Error("Original URL is empty")
        return "", ErrOriginalURLEmpty
    }
    if _, err := url.ParseRequestURI(originalURL); err != nil {
			  slog.Error("Invalid URL format")
        return "", ErrInvalidURLFormat
    }

    const maxAttempts = 3

    for attempt := 0; attempt < maxAttempts; attempt++ {
        shortKey, err := generateShortKey(s.shardId)
        if err != nil {
            return "", err
        }
        err = s.repo.StoreURL(ctx, shortKey, originalURL)
        if err == nil {
            return shortKey, nil
        }
        if err.Error() == repositories.ErrDuplicateKey.Error() {
            continue
        }
        return "", err
    }
		slog.Error("Failed to create short URL")
    return "", ErrCreateShortURLFailed
}

func (s *shortenerService) GetOriginalURL(ctx context.Context, shortKey string) (string, error) {
    match, err := regexp.MatchString( "^[a-zA-Z0-9]{9}$", shortKey)
		if err != nil || !match {
        return "", ErrInvalidShortKeyFormat
		}

    originalURL, err := s.repo.GetURL(ctx, shortKey)
    if err != nil {
			  if err.Error() == repositories.ErrURLNotFound.Error() {
					return "", ErrShortKeyNotFound
				}
        return "", err

    }
    return originalURL, nil
}

func generateShortKey(shardId int) (string, error) {
		const keyLength = 9
	  const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

		result := make([]byte, keyLength) 

		result[0] = base62Chars[shardId]

		for i := 1; i < keyLength; i++ {
				num, err := rand.Int(rand.Reader, big.NewInt(62))
				if err != nil {
						return "", err
				}
				result[i] = base62Chars[num.Int64()]
		}

    return string(result), nil
}
