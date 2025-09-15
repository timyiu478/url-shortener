package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/yourusername/urlshortener/internal/services"
)

// Handlers holds the HTTP handlers
type Handlers struct {
	svc *services.URLShortenerService
}

// NewHandlers creates new handlers
func NewHandlers(svc *services.URLShortenerService) *Handlers {
	return &Handlers{svc: svc}
}

// URLRequest represents the POST /newurl request payload
type URLRequest struct {
	Domain string `json:"domain"`
	URL    string `json:"url"`
}

// URLResponse represents the POST /newurl response payload
type URLResponse struct {
	URL        string `json:"url"`
	ShortenURL string `json:"shortenUrl"`
}

// CreateURL handles POST /newurl
func (h *Handlers) CreateURL(w http.ResponseWriter, r *http.Request) {
	var req URLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.Domain != h.svc.domain || req.URL == "" {
		http.Error(w, "Invalid domain or URL", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	shortURL, err := h.svc.CreateShortURL(ctx, req.URL)
	if err != nil {
		http.Error(w, "Failed to create short URL", http.StatusInternalServerError)
		return
	}

	resp := URLResponse{URL: req.URL, ShortenURL: shortURL}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetURL handles GET /{shortKey}
func (h *Handlers) GetURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	shortKey := vars["shortKey"]

	ctx := r.Context()
	originalURL, err := h.svc.GetOriginalURL(ctx, shortKey)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Short URL not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	http.Redirect(w, r, originalURL, http.StatusFound)
}
