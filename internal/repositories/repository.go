package repositories

import (
    "context"
    "database/sql"
    "sync"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
    "github.com/go-sql-driver/mysql"
    "github.com/redis/go-redis/v9"
    "url-shortener/internal/config"
)

var tracer = otel.Tracer("url-shortener-repositories")

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
    ctx, span := tracer.Start(ctx, "StoreURL", trace.WithAttributes(
        attribute.String("short_key", shortKey),
        attribute.String("original_url", originalURL),
    ))
    defer span.End()

    // Add timeout for DB operation
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    query := "INSERT INTO urls (short_key, original_url, created_at) VALUES (?, ?, ?)"
    _, err := r.db.ExecContext(ctx, query, shortKey, originalURL, time.Now())
    if err != nil {
        span.RecordError(err)
        return err
    }

    ctx, cacheSpan := tracer.Start(ctx, "CacheSet")
    defer cacheSpan.End()
    err = r.cache.SetEx(ctx, shortKey, originalURL, 24*time.Hour).Err()
    if err != nil {
        cacheSpan.RecordError(err)
        return fmt.Errorf("failed to set cache: %w", err)
    }
    return nil
}

func (r *urlRepository) GetURL(ctx context.Context, shortKey string) (string, error) {
    ctx, span := tracer.Start(ctx, "GetURL", trace.WithAttributes(
        attribute.String("short_key", shortKey),
    ))
    defer span.End()

    ctx, cacheSpan := tracer.Start(ctx, "CacheGet")
    val, err := r.cache.Get(ctx, shortKey).Result()
    cacheSpan.End()
    if err == nil {
        span.SetAttributes(attribute.String("source", "cache"))
        return val, nil
    }
    if err != redis.Nil {
        cacheSpan.RecordError(err)
    }

    ctx, dbSpan := tracer.Start(ctx, "DBQuery")
    defer dbSpan.End()
    // Add timeout for DB operation
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    var originalURL string
    err = r.db.QueryRowContext(ctx, "SELECT original_url FROM urls WHERE short_key = ?", shortKey).Scan(&originalURL)
    if err != nil {
        dbSpan.RecordError(err)
        return "", err
    }

    ctx, cacheSetSpan := tracer.Start(ctx, "CacheSet")
    defer cacheSetSpan.End()
    if err := r.cache.SetEx(ctx, shortKey, originalURL, 24*time.Hour).Err(); err != nil {
        cacheSetSpan.RecordError(err)
        return originalURL, nil // Return URL despite cache error to maintain functionality
    }

    span.SetAttributes(attribute.String("original_url", originalURL), attribute.String("source", "database"))
    return originalURL, nil
}

func (r *urlRepository) PingDB(ctx context.Context) error {
    ctx, span := tracer.Start(ctx, "PingDB")
    defer span.End()
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    err := r.db.PingContext(ctx)
    if err != nil {
        span.RecordError(err)
    }
    return err
}

func (r *urlRepository) PingCache(ctx context.Context) error {
    ctx, span := tracer.Start(ctx, "PingCache")
    defer span.End()
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    _, err := r.cache.Ping(ctx).Result()
    if err != nil {
        span.RecordError(err)
    }
    return err
}
