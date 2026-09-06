package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"

	"url-shortener/internal/middleware"
	"url-shortener/internal/repositories"
	"url-shortener/internal/services"
)

// Handler handles HTTP requests for the shortener service.
type Handler struct {
	svc  services.Shortener
	repo repositories.URLRepository
}

func NewHandler(svc services.Shortener, repo repositories.URLRepository) *Handler {
	return &Handler{svc: svc, repo: repo}
}

// HealthCheck handles liveness probe (HTTP server is running)
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// ReadinessCheck handles readiness probe (dependencies are healthy)
func (h *Handler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	logger := middleware.FromContext(ctx)
	if err := h.repo.PingDB(ctx); err != nil {
		logger.Error("Readiness check failed: database error", "error", err)
		http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// CreateShortURL handles POST /newurl
func (h *Handler) CreateShortURL(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := middleware.FromContext(ctx)
	tr := otel.Tracer("handlers")
	ctx, span := tr.Start(ctx, "CreateShortURL")
	defer span.End()

	var req struct {
		Domain string `json:"domain"`
		URL    string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" || req.Domain == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	shortKey, err := h.svc.CreateShortURL(ctx, req.URL)
	if err != nil {
		if err.Error() == services.ErrOriginalURLEmpty.Error() || err.Error() == services.ErrInvalidURLFormat.Error() {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		logger.Error("CreateShortURL error", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Assumed req.Domain is a valid that can be resolved to the our owned IP(s)
	resp := map[string]string{
		"url":        req.URL,
		"shortenUrl": "https://" + req.Domain + "/" + shortKey,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetOriginalURL handles GET /{shortKey}
func (h *Handler) GetOriginalURL(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := middleware.FromContext(ctx)
	tr := otel.Tracer("handlers")
	ctx, span := tr.Start(ctx, "GetOriginalURL")
	defer span.End()

	shortKey := chi.URLParam(r, "shortKey")

	originalURL, err := h.svc.GetOriginalURL(ctx, shortKey)

	if err != nil {
		if err.Error() == services.ErrInvalidShortKeyFormat.Error() || err.Error() == services.ErrShortKeyNotFound.Error() {
			http.Error(w, "URL not found", http.StatusNotFound)
			return
		}
		logger.Error("GetOriginalURL error", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Business metric: increment redirects total and record redirect event
	middleware.UrlRedirectsInc()

	http.Redirect(w, r, originalURL, http.StatusNotModified)
}
