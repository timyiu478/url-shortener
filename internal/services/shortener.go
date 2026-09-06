package services

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"time"

	"crypto/rand"

	"url-shortener/internal/middleware"
	"url-shortener/internal/repositories"

	"go.opentelemetry.io/otel"
)

var (
	ErrCreateShortURLFailed    = fmt.Errorf("failed to create short URL")
	ErrInvalidURLFormat       = fmt.Errorf("invalid URL format")
	ErrOriginalURLEmpty       = fmt.Errorf("original URL is empty")
	ErrShortKeyNotFound       = fmt.Errorf("short key is not found")
	ErrInvalidShortKeyFormat  = fmt.Errorf("invalid short key format")
)

type Shortener interface {
	CreateShortURL(ctx context.Context, originalURL string) (string, error)
	GetOriginalURL(ctx context.Context, shortKey string) (string, error)
}

type shortenerService struct {
	repo    repositories.URLRepository
	shardId int
}

func NewShortenerService(repo repositories.URLRepository, shardId int) Shortener {
	return &shortenerService{repo: repo, shardId: shardId}
}

func (s *shortenerService) CreateShortURL(ctx context.Context, originalURL string) (string, error) {
	tr := otel.Tracer("shortener")
	ctx, span := tr.Start(ctx, "CreateShortURL")
	defer span.End()

	logger := middleware.FromContext(ctx)

	// Validate URL
	if originalURL == "" {
		logger.Error("Original URL is empty")
		return "", ErrOriginalURLEmpty
	}
	if _, err := url.ParseRequestURI(originalURL); err != nil {
		logger.Error("Invalid URL format", "error", err)
		return "", ErrInvalidURLFormat
	}

	const maxAttempts = 3

	for attempt := 0; attempt < maxAttempts; attempt++ {
		shortKey, err := generateShortKey(s.shardId)
		if err != nil {
			return "", err
		}

		start := time.Now()
		err = s.repo.StoreURL(ctx, shortKey, originalURL)
		middleware.ObserveDBQuery(time.Since(start).Seconds())

		if err == nil {
			return shortKey, nil
		}
		if err.Error() == repositories.ErrDuplicateKey.Error() {
			continue
		}
		return "", err
	}
	logger.Error("Failed to create short URL")
	return "", ErrCreateShortURLFailed
}

func (s *shortenerService) GetOriginalURL(ctx context.Context, shortKey string) (string, error) {
	tr := otel.Tracer("shortener")
	ctx, span := tr.Start(ctx, "GetOriginalURL")
	defer span.End()

	// Validate key format
	match, err := regexp.MatchString("^[a-zA-Z0-9]{9}$", shortKey)
	if err != nil || !match {
		return "", ErrInvalidShortKeyFormat
	}

	start := time.Now()
	originalURL, err := s.repo.GetURL(ctx, shortKey)
	middleware.ObserveDBQuery(time.Since(start).Seconds())

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
