package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"url-shortener/internal/config"
	"url-shortener/internal/handlers"
	"url-shortener/internal/middleware"
	"url-shortener/internal/observability"
	"url-shortener/internal/repositories"
	"url-shortener/internal/services"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log := slog.Default()
		log.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize observability (logger, metrics, tracer)
	shutdownObs, err := observability.Init(observability.Config{ServiceName: "url-shortener", OTLPEndpoint: os.Getenv("OTEL_COLLECTOR_ENDPOINT")})
	if err != nil {
		log := slog.Default()
		log.Error("Failed to initialize observability", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownObs(ctx); err != nil {
			slog.Error("failed to shutdown observability", "error", err)
		}
	}()

	// Initialize repository
	repo, err := repositories.NewURLRepository(cfg)
	if err != nil {
		slog.Error("Failed to initialize repository", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := repo.Close(); err != nil {
			slog.Warn("Error closing repository", "error", err)
		}
	}()

	// Initialize service and handler
	svc := services.NewShortenerService(repo, cfg.ShardId)
	h := handlers.NewHandler(svc, repo)

	// Set up HTTP router
	r := chi.NewRouter()

	// Observability middleware
	r.Use(middleware.TraceLoggerMiddleware)
	r.Use(middleware.PrometheusMetricsMiddleware)

	r.Post("/newurl", h.CreateShortURL)
	r.Get("/{shortKey:[a-zA-Z0-9]{9}}", h.GetOriginalURL)
	r.Get("/healthz", h.HealthCheck)
	r.Get("/readyz", h.ReadinessCheck)

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

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
		slog.Info("Starting server", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("Shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("Server stopped")
}
