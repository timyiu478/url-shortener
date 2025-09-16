package handlers

import (
    "context"
    "encoding/json"
		"database/sql"
    "log"
		"errors"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-sql-driver/mysql"
    "url-shortener/internal/repositories"
    "url-shortener/internal/services"
)

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

    if err := h.repo.PingDB(ctx); err != nil {
        log.Printf("Readiness check failed: database error: %v", err)
        http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
        return
    }

    if err := h.repo.PingCache(ctx); err != nil {
        log.Printf("Readiness check failed: cache error: %v", err)
        http.Error(w, "Cache unhealthy", http.StatusServiceUnavailable)
        return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

// CreateShortURL handles POST /newurl
func (h *Handler) CreateShortURL(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Domain string `json:"domain"`
        URL    string `json:"url"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    shortKey, err := h.svc.CreateShortURL(r.Context(), req.URL)
    if err != nil {
        if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
            http.Error(w, "Short key conflict", http.StatusConflict)
            return
        }
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }

    resp := map[string]string{
        "url":        req.URL,
        "shortenUrl": "https://" + req.Domain + "/" + shortKey,
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(resp)
}

// GetOriginalURL handles GET /{shortKey}
func (h *Handler) GetOriginalURL(w http.ResponseWriter, r *http.Request) {
    shortKey := chi.URLParam(r, "shortKey")
    originalURL, err := h.svc.GetOriginalURL(r.Context(), shortKey)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            http.Error(w, "URL not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }

    http.Redirect(w, r, originalURL, http.StatusFound)
}
