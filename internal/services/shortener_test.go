package services_test

import (
	"context"
	"testing"

	"github.com/yourusername/urlshortener/internal/repositories"
	"github.com/yourusername/urlshortener/internal/services"
)

// mockURLRepository mocks the URLRepository
type mockURLRepository struct {
	store map[string]string
}

// StoreURL stores the URL mapping in DB and cache
func (m *mockURLRepository) StoreURL(ctx context.Context, shortKey, originalURL string) error {
	m.store[shortKey] = originalURL
	return nil
}

// GetURL retrieves the original URL, checking cache first
func (m *mockURLRepository) GetURL(ctx context.Context, shortKey string) (string, error) {
	url, ok := m.store[shortKey]
	if !ok {
		return "", fmt.Errorf("short URL not found")
	}
	return url, nil
}

// Close closes the connections
func (m *mockURLRepository) Close() error {
	return nil
}

// SetupDatabase initializes the database schema
func (m *mockURLRepository) SetupDatabase() error {
	return nil
}

func TestURLShortenerService_CreateShortURL(t *testing.T) {
	mockRepo := &mockURLRepository{store: make(map[string]string)}
	s := services.NewURLShortenerService(mockRepo, "shortenurl.org")

	ctx := context.Background()
	shortURL, err := s.CreateShortURL(ctx, "https://google.com")
	if err != nil {
		t.Errorf("CreateShortURL failed: %v", err)
	}
	if !strings.HasPrefix(shortURL, "https://shortenurl.org/") {
		t.Errorf("Expected prefix https://shortenurl.org/, got %s", shortURL)
	}
	key := strings.TrimPrefix(shortURL, "https://shortenurl.org/")
	if len(key) != 9 {
		t.Errorf("Expected 9-character key, got %d", len(key))
	}
}

func TestURLShortenerService_GetOriginalURL(t *testing.T) {
	mockRepo := &mockURLRepository{store: make(map[string]string)}
	mockRepo.store["g20hi3k9Z"] = "https://google.com"
	s := services.NewURLShortenerService(mockRepo, "shortenurl.org")

	ctx := context.Background()
	url, err := s.GetOriginalURL(ctx, "g20hi3k9Z")
	if err != nil {
		t.Errorf("GetOriginalURL failed: %v", err)
	}
	if url != "https://google.com" {
		t.Errorf("Expected https://google.com, got %s", url)
	}
}

func TestURLShortenerService_CreateShortURL_RetryOnCollision(t *testing.T) {
	mockRepo := &mockURLRepository{store: make(map[string]string)}
	mockRepo.store["g20hi3k9Z"] = "https://existing.com" // Simulate existing key
	s := services.NewURLShortenerService(mockRepo, "shortenurl.org")

	ctx := context.Background()
	shortURL, err := s.CreateShortURL(ctx, "https://google.com")
	if err != nil {
		t.Errorf("CreateShortURL failed: %v", err)
	}
	if !strings.HasPrefix(shortURL, "https://shortenurl.org/") {
		t.Errorf("Expected prefix https://shortenurl.org/, got %s", shortURL)
	}
	key := strings.TrimPrefix(shortURL, "https://shortenurl.org/")
	if len(key) != 9 {
		t.Errorf("Expected 9-character key, got %d", len(key))
	}
	if key == "g20hi3k9Z" {
		t.Errorf("Expected retry to generate new key, got existing key")
	}
}
