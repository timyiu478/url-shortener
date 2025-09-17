package main

import (
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5"
    "url-shortener/internal/config"
    "url-shortener/internal/handlers"
    "url-shortener/internal/repositories"
    "url-shortener/internal/services"
)

func main() {
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Initialize repository
    repo, err := repositories.NewURLRepository(cfg)
    if err != nil {
        log.Fatalf("Failed to initialize repository: %v", err)
    }
    defer func() {
        if err := repo.Close(); err != nil {
            log.Printf("Error closing repository: %v", err)
        }
    }()

    // Initialize service and handler
		shardMap := make(map[string]int)
		shardMap[cfg.AWSRegion] = 0
    svc := services.NewShortenerService(repo, shardMap[cfg.AWSRegion])
    h := handlers.NewHandler(svc, repo)

    // Set up HTTP router
    r := chi.NewRouter()
    r.Post("/newurl", h.CreateShortURL)
    r.Get("/{shortKey}", h.GetOriginalURL)
    r.Get("/healthz", h.HealthCheck)
    r.Get("/readyz", h.ReadinessCheck)

    // Start server with timeouts
    srv := &http.Server{
        Addr:         ":" + cfg.Port,
        Handler:      r,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  15 * time.Second,
    }

    // Handle graceful shutdown
    stop := make(chan os.Signal, 5)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

    go func() {
        log.Printf("Starting server on port %s", cfg.Port)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server error: %v", err)
        }
    }()

    <-stop
    log.Println("Shutting down server...")
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("Server shutdown error: %v", err)
    }
    log.Println("Server stopped")
}
