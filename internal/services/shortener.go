package services

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
    "github.com/your-org/url-shortener/internal/repositories"
)

var tracer = otel.Tracer("url-shortener-services")

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
    ctx, span := tracer.Start(ctx, "CreateShortURL", trace.WithAttributes(
        attribute.String("original_url", originalURL),
    ))
    defer span.End()

    const maxAttempts = 3
    for attempt := 0; attempt < maxAttempts; attempt++ {
        shortKey, err := generateShortKey()
        if err != nil {
            span.RecordError(err)
            return "", err
        }
        err = s.repo.StoreURL(ctx, shortKey, originalURL)
        if err == nil {
            span.SetAttributes(attribute.String("short_key", shortKey))
            return shortKey, nil
        }
        if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
            span.AddEvent("Duplicate key retry", trace.WithAttributes(
                attribute.Int("attempt", attempt+1),
            ))
            continue
        }
        span.RecordError(err)
        return "", err
    }
    err := errors.New("max attempts reached for unique short key")
    span.RecordError(err)
    return "", err
}

func (s *shortenerService) GetOriginalURL(ctx context.Context, shortKey string) (string, error) {
    ctx, span := tracer.Start(ctx, "GetOriginalURL", trace.WithAttributes(
        attribute.String("short_key", shortKey),
    ))
    defer span.End()

    originalURL, err := s.repo.GetURL(ctx, shortKey)
    if err != nil {
        span.RecordError(err)
        return "", err
    }
    span.SetAttributes(attribute.String("original_url", originalURL))
    return originalURL, nil
}

func generateShortKey() (string, error) {
    b := make([]byte, 7) // ~9 chars in Base62
    _, err := rand.Read(b)
    if err != nil {
        return "", err
    }
    return base64.RawURLEncoding.EncodeToString(b)[:9], nil
}
