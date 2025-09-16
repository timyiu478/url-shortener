package repositories

import (
    "context"
    "database/sql"
    "sync"
    "time"
		"fmt"

    "github.com/go-sql-driver/mysql"
    "github.com/redis/go-redis/v9"
    "url-shortener/internal/config"
)

type URLRepository interface {
    StoreURL(ctx context.Context, shortKey, originalURL string) error
    GetURL(ctx context.Context, shortKey string) (string, error)
    PingDB(ctx context.Context) error
    PingCache(ctx context.Context) error
    Close() error
}

type urlRepository struct {
    db    *sql.DB
    cache *redis.Client
    mu    sync.Mutex // For safe closing
}

func NewURLRepository(cfg *config.Config) (URLRepository, error) {
    dbCfg := mysql.Config{
        User:   cfg.DBUser,
        Passwd: cfg.DBPassword,
        Net:    "tcp",
        Addr:   cfg.DBEndpoint,
        DBName: cfg.DBName,
    }
    db, err := sql.Open("mysql", dbCfg.FormatDSN())
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)

    cache := redis.NewClient(&redis.Options{
        Addr: cfg.RedisEndpoint,
    })

    return &urlRepository{db: db, cache: cache}, nil
}

func (r *urlRepository) Close() error {
    r.mu.Lock()
    defer r.mu.Unlock()

    var errs []error
    if err := r.cache.Close(); err != nil {
        errs = append(errs, fmt.Errorf("failed to close cache: %w", err))
    }
    if err := r.db.Close(); err != nil {
        errs = append(errs, fmt.Errorf("failed to close database: %w", err))
    }
    if len(errs) > 0 {
        return fmt.Errorf("errors closing resources: %v", errs)
    }
    return nil
}

func (r *urlRepository) StoreURL(ctx context.Context, shortKey, originalURL string) error {
    // Add timeout for DB operation
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    query := "INSERT INTO urls (short_key, original_url, created_at) VALUES (?, ?, ?)"
    _, err := r.db.ExecContext(ctx, query, shortKey, originalURL, time.Now())
    if err != nil {
        return err
    }

    err = r.cache.SetEx(ctx, shortKey, originalURL, 24*time.Hour).Err()
    if err != nil {
        return fmt.Errorf("failed to set cache: %w", err)
    }
    return nil
}

func (r *urlRepository) GetURL(ctx context.Context, shortKey string) (string, error) {
    val, err := r.cache.Get(ctx, shortKey).Result()
    if err == nil {
        return val, nil
    }

    // Add timeout for DB operation
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    var originalURL string
    err = r.db.QueryRowContext(ctx, "SELECT original_url FROM urls WHERE short_key = ?", shortKey).Scan(&originalURL)

    if err != nil {
        return "", err
    }


    if err := r.cache.SetEx(ctx, shortKey, originalURL, 24*time.Hour).Err(); err != nil {
        return originalURL, nil // Return URL despite cache error to maintain functionality
    }

    return originalURL, nil
}

func (r *urlRepository) PingDB(ctx context.Context) error {
    err := r.db.PingContext(ctx)
    return err
}

func (r *urlRepository) PingCache(ctx context.Context) error {
    _, err := r.cache.Ping(ctx).Result()
    return err
}
