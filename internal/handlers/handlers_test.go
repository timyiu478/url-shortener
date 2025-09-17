package handlers_test

import (
    "bytes"
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "net/http"
    "net/http/httptest"
    "regexp"
    "strings"
    "testing"

    "github.com/go-chi/chi/v5"
    "github.com/go-sql-driver/mysql"
    "url-shortener/internal/handlers"
)

type mockShortener struct {
    createResult string
    createErr    error
    getResult    string
    getErr       error
}

func (m *mockShortener) CreateShortURL(ctx context.Context, originalURL string) (string, error) {
    return m.createResult, m.createErr
}

func (m *mockShortener) GetOriginalURL(ctx context.Context, shortKey string) (string, error) {
    return m.getResult, m.getErr
}

type mockRepository struct {
    pingDBErr    error
    pingCacheErr error
}

func (m *mockRepository) StoreURL(ctx context.Context, shortKey, originalURL string) error {
    return nil
}

func (m *mockRepository) GetURL(ctx context.Context, shortKey string) (string, error) {
    return "", nil
}

func (m *mockRepository) PingDB(ctx context.Context) error {
    return m.pingDBErr
}

func (m *mockRepository) PingCache(ctx context.Context) error {
    return m.pingCacheErr
}

func (m *mockRepository) Close() error {
    return nil
}

func TestHandler_CreateShortURL(t *testing.T) {
    // Success case
    t.Run("Success", func(t *testing.T) {
        mockSvc := &mockShortener{createResult: "g20hi3k9Z", createErr: nil}
        mockRepo := &mockRepository{}
        h := handlers.NewHandler(mockSvc, mockRepo)

        r := chi.NewRouter()
        r.Post("/newurl", h.CreateShortURL)

        reqBody, _ := json.Marshal(map[string]string{"domain": "shortenurl.org", "url": "https://google.com"})
        req := httptest.NewRequest("POST", "/newurl", bytes.NewReader(reqBody))
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusOK {
            t.Errorf("Expected status 200, got %d", rr.Code)
        }

        var resp map[string]string
        json.Unmarshal(rr.Body.Bytes(), &resp)
        shortenURL := resp["shortenUrl"]
        if !strings.HasPrefix(shortenURL, "https://shortenurl.org/") {
            t.Errorf("Expected shortenUrl to start with https://shortenurl.org/, got %s", shortenURL)
        }
        shortKey := shortenURL[len("https://shortenurl.org/"):]
        if matched, _ := regexp.MatchString("^[0-9a-zA-Z]{9}$", shortKey); !matched {
            t.Errorf("Short key %s is not Base62", shortKey)
        }
    })

    // Error case: duplicate key
    t.Run("DuplicateKey", func(t *testing.T) {
        mockSvc := &mockShortener{createErr: &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}}
        mockRepo := &mockRepository{}
        h := handlers.NewHandler(mockSvc, mockRepo)

        r := chi.NewRouter()
        r.Post("/newurl", h.CreateShortURL)

        reqBody, _ := json.Marshal(map[string]string{"domain": "shortenurl.org", "url": "https://google.com"})
        req := httptest.NewRequest("POST", "/newurl", bytes.NewReader(reqBody))
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusConflict {
            t.Errorf("Expected status 409, got %d", rr.Code)
        }
    })

    // Error case: invalid URL
    t.Run("InvalidURL", func(t *testing.T) {
        mockSvc := &mockShortener{createErr: errors.New("invalid URL format")}
        mockRepo := &mockRepository{}
        h := handlers.NewHandler(mockSvc, mockRepo)

        r := chi.NewRouter()
        r.Post("/newurl", h.CreateShortURL)

        reqBody, _ := json.Marshal(map[string]string{"domain": "shortenurl.org", "url": "!invalid"})
        req := httptest.NewRequest("POST", "/newurl", bytes.NewReader(reqBody))
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusBadRequest {
            t.Errorf("Expected status 400, got %d", rr.Code)
        }
    })
}

func TestHandler_GetOriginalURL(t *testing.T) {
    // Success case
    t.Run("Success", func(t *testing.T) {
        mockSvc := &mockShortener{getResult: "https://google.com", getErr: nil}
        mockRepo := &mockRepository{}
        h := handlers.NewHandler(mockSvc, mockRepo)

        r := chi.NewRouter()
        r.Get("/{shortKey}", h.GetOriginalURL)

        req := httptest.NewRequest("GET", "/g20hi3k9Z", nil)
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusFound {
            t.Errorf("Expected status 302, got %d", rr.Code)
        }
        if rr.Header().Get("Location") != "https://google.com" {
            t.Errorf("Expected redirect to https://google.com, got %s", rr.Header().Get("Location"))
        }
    })

    // Error case: not found
    t.Run("NotFound", func(t *testing.T) {
        mockSvc := &mockShortener{getErr: sql.ErrNoRows}
        mockRepo := &mockRepository{}
        h := handlers.NewHandler(mockSvc, mockRepo)

        r := chi.NewRouter()
        r.Get("/{shortKey}", h.GetOriginalURL)

        req := httptest.NewRequest("GET", "/g20hi3k9Z", nil)
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusNotFound {
            t.Errorf("Expected status 404, got %d", rr.Code)
        }
    })
}

func TestHandler_HealthCheck(t *testing.T) {
    h := handlers.NewHandler(nil, nil)

    r := chi.NewRouter()
    r.Get("/healthz", h.HealthCheck)

    req := httptest.NewRequest("GET", "/healthz", nil)
    rr := httptest.NewRecorder()

    r.ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rr.Code)
    }
    if rr.Body.String() != "OK" {
        t.Errorf("Expected body OK, got %s", rr.Body.String())
    }
}

func TestHandler_ReadinessCheck(t *testing.T) {
    // Success case
    t.Run("Success", func(t *testing.T) {
        mockRepo := &mockRepository{pingDBErr: nil, pingCacheErr: nil}
        h := handlers.NewHandler(nil, mockRepo)

        r := chi.NewRouter()
        r.Get("/readyz", h.ReadinessCheck)

        req := httptest.NewRequest("GET", "/readyz", nil)
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusOK {
            t.Errorf("Expected status 200, got %d", rr.Code)
        }
        if rr.Body.String() != "OK" {
            t.Errorf("Expected body OK, got %s", rr.Body.String())
        }
    })

    // Failure case: DB down
    t.Run("DBFailure", func(t *testing.T) {
        mockRepo := &mockRepository{pingDBErr: errors.New("db down"), pingCacheErr: nil}
        h := handlers.NewHandler(nil, mockRepo)

        r := chi.NewRouter()
        r.Get("/readyz", h.ReadinessCheck)

        req := httptest.NewRequest("GET", "/readyz", nil)
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusServiceUnavailable {
            t.Errorf("Expected status 503, got %d", rr.Code)
        }
    })

    // Failure case: Cache down
    t.Run("CacheFailure", func(t *testing.T) {
        mockRepo := &mockRepository{pingDBErr: nil, pingCacheErr: errors.New("cache down")}
        h := handlers.NewHandler(nil, mockRepo)

        r := chi.NewRouter()
        r.Get("/readyz", h.ReadinessCheck)

        req := httptest.NewRequest("GET", "/readyz", nil)
        rr := httptest.NewRecorder()

        r.ServeHTTP(rr, req)

        if rr.Code != http.StatusServiceUnavailable {
            t.Errorf("Expected status 503, got %d", rr.Code)
        }
    })
}
