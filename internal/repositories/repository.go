package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"urlshortener/internal/config"
  "github.com/go-sql-driver/mysql"
)

// URLRepository defines the interface for URL data access
type URLRepository interface {
	StoreURL(ctx context.Context, shortKey, originalURL string) error
	GetURL(ctx context.Context, shortKey string) (string, error)
	Close() error
	SetupDatabase() error
}

// urlRepository implements URLRepository
type urlRepository struct {
	db    *sql.DB
	cache *redis.Client
}

// NewURLRepository creates a new URL repository
func NewURLRepository(cfg *config.Config) (URLRepository, error) {
	// Initialize database connection (Aurora MySQL)
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true", cfg.DBUser, cfg.DBPassword, cfg.DBEndpoint, cfg.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}
	db.SetConnMaxLifetime(time.Minute * 5)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	// Initialize Redis connection (ElastiCache)
	cache := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisEndpoint,
		Password: "", // Assuming no password for simplicity
		DB:       0,
	})

	// Verify connections
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}
	if _, err := cache.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %v", err)
	}

	repo := &urlRepository{db: db, cache: cache}

	// Setup database schema
	if err := repo.SetupDatabase(); err != nil {
		return nil, fmt.Errorf("failed to setup database: %v", err)
	}

	return repo, nil
}

// StoreURL stores the URL mapping in DB and cache
func (r *urlRepository) StoreURL(ctx context.Context, shortKey, originalURL string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO urls (short_key, original_url, created_at) VALUES (?, ?, ?)",
		shortKey, originalURL, time.Now())
	if err != nil {
		return fmt.Errorf("failed to store URL: %v", err)
	}

	// Cache the mapping
	err = r.cache.Set(ctx, shortKey, originalURL, 24*time.Hour).Err()
	if err != nil {
		log.Printf("Failed to cache URL: %v", err)
		// Continue despite cache failure
	}

	return nil
}

// GetURL retrieves the original URL, checking cache first
func (r *urlRepository) GetURL(ctx context.Context, shortKey string) (string, error) {
	// Check cache
	val, err := r.cache.Get(ctx, shortKey).Result()
	if err == nil {
		return val, nil
	}
	if err != redis.Nil {
		log.Printf("Cache error: %v", err)
	}

	// Check database
	var url string
	err = r.db.QueryRowContext(ctx, "SELECT original_url FROM urls WHERE short_key = ?", shortKey).Scan(&url)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("short URL not found")
	}
	if err != nil {
		return "", fmt.Errorf("database error: %v", err)
	}

	// Update cache
	err = r.cache.Set(ctx, shortKey, url, 24*time.Hour).Err()
	if err != nil {
		log.Printf("Failed to update cache: %v", err)
	}

	return url, nil
}

// Close closes the connections
func (r *urlRepository) Close() error {
	if err := r.cache.Close(); err != nil {
		return err
	}
	return r.db.Close()
}

// SetupDatabase initializes the database schema
func (r *urlRepository) SetupDatabase() error {
	ctx := context.Background()
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS urls (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			short_key VARCHAR(10) NOT NULL UNIQUE,
			original_url TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			INDEX idx_short_key (short_key)
		)`)
	return err
}
