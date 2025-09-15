package repositories_test

import (
	"context"
	"testing"
	"time"

	"url-shortener/internal/config"
	"urlshortener/internal/repositories"
)

func TestURLRepository(t *testing.T) {
	// For unit tests, use mocks or in-memory implementations.
	// Here, assuming integration test with real DB/cache, but in practice, mock them.

	// Example: Skip if no test DB
	t.Skip("Integration test skipped")

	cfg := &config.Config{
		// Fill with test config
	}

	repo, err := repositories.NewURLRepository(cfg)
	if err != nil {
		t.Fatalf("Failed to create repo: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	shortKey := "testkey"
	originalURL := "https://example.com"

	err = repo.StoreURL(ctx, shortKey, originalURL)
	if err != nil {
		t.Errorf("StoreURL failed: %v", err)
	}

	got, err := repo.GetURL(ctx, shortKey)
	if err != nil {
		t.Errorf("GetURL failed: %v", err)
	}
	if got != originalURL {
		t.Errorf("Expected %s, got %s", originalURL, got)
	}
}
