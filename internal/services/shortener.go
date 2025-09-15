package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/yourusername/urlshortener/internal/repositories"
)

// base62Chars defines the Base62 character set for short keys
const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// URLShortenerService handles business logic for URL shortening
type URLShortenerService struct {
	repo   repositories.URLRepository
	domain string
}

// NewURLShortenerService creates a new service
func NewURLShortenerService(repo repositories.URLRepository, domain string) *URLShortenerService {
	return &URLShortenerService{repo: repo, domain: domain}
}

// CreateShortURL generates and stores a short URL
func (s *URLShortenerService) CreateShortURL(ctx context.Context, originalURL string) (string, error) {
	const maxAttempts = 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		shortKey, err := s.generateShortKey()
		if err != nil {
			return "", err
		}

		err = s.repo.StoreURL(ctx, shortKey, originalURL)
		if err == nil {
			return fmt.Sprintf("https://%s/%s", s.domain, shortKey), nil
		}
		// Check for MySQL duplicate key error (code 1062)
		if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
			if attempt < maxAttempts {
				continue // Retry with a new key
			}
			return "", fmt.Errorf("failed to generate unique short key after %d attempts", maxAttempts)
		}
		return "", fmt.Errorf("failed to store URL: %v", err)
	}
	return "", fmt.Errorf("failed to generate unique short key after %d attempts", maxAttempts)
}

// GetOriginalURL retrieves the original URL from short key
func (s *URLShortenerService) GetOriginalURL(ctx context.Context, shortKey string) (string, error) {
	return s.repo.GetURL(ctx, shortKey)
}

// generateShortKey creates a random 9-character Base62 short key
func (s *URLShortenerService) generateShortKey() (string, error) {
	const keyLength = 9
	b := make([]byte, keyLength)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	// Convert random bytes to Base62
	result := make([]byte, keyLength)
	for i := 0; i < keyLength; i++ {
		result[i] = base62Chars[int(b[i])%62]
	}
	return string(result), nil
}
