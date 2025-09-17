package services

import (
    "context"
    "crypto/rand"
    "fmt"
    "net/url"

    "url-shortener/internal/repositories"
)

var (
	ErrGenUniqShortKeyFailed = fmt.Errorf("max attempts reached for unique short key")
	ErrInvalidURLFormat = fmt.Errorf("invalid URL format")
	ErrOriginalURLEmpty = fmt.Errorf("original URL is empty")
	ErrShortKeyEmpty = fmt.Errorf("short key is empty")
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
        return "", ErrOriginalURLEmpty
    }
    if _, err := url.ParseRequestURI(originalURL); err != nil {
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
    return "", ErrGenUniqShortKeyFailed
}

func (s *shortenerService) GetOriginalURL(ctx context.Context, shortKey string) (string, error) {
    if shortKey == "" {
        return "", ErrShortKeyEmpty
    }

    originalURL, err := s.repo.GetURL(ctx, shortKey)
    if err != nil {
        return "", err
    }
    return originalURL, nil
}

func generateShortKey(shardId int) (string, error) {
		const keyLength = 9
	  const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

		result := make([]byte, keyLength) 

		result[0] = base62Char[shardId]

		for i := 1; i < keyLength; i++ {
				num, err := rand.Int(rand.Reader, big.NewInt(62)))
				if err != nil {
						return "", err
				}
				result[i] = base62Chars[num.Int64()]
		}

    return string(result), nil
}
