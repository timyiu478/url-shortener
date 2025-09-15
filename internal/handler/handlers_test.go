package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/yourusername/urlshortener/internal/handlers"
	"github.com/yourusername/urlshortener/internal/services"
)

// mockURLShortenerService mocks the service
type mockURLShortenerService struct{}

func (m *mockURLShortenerService) CreateShortURL(ctx context.Context, originalURL string) (string, error) {
	return "https://example.com/abc123", nil
}

func (m *mockURLShortenerService) GetOriginalURL(ctx context.Context, shortKey string) (string, error) {
	return "https://test.com", nil
}

func TestHandlers_CreateURL(t *testing.T) {
	svc := &mockURLShortenerService{}
	h := handlers.NewHandlers(svc) // Note: domain in svc is "example.com", but mock doesn't use it

	reqBody := handlers.URLRequest{Domain: "example.com", URL: "https://test.com"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/newurl", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.CreateURL(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestHandlers_GetURL(t *testing.T) {
	svc := &mockURLShortenerService{}
	h := handlers.NewHandlers(svc)

	req := httptest.NewRequest("GET", "/abc123", nil)
	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/{shortKey}", h.GetURL).Methods("GET")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Expected 302, got %d", rr.Code)
	}
}
