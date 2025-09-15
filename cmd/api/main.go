package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/yourusername/urlshortener/internal/config"
	"github.com/yourusername/urlshortener/internal/handlers"
	"github.com/yourusername/urlshortener/internal/repositories"
	"github.com/yourusername/urlshortener/internal/services"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize repository
	repo, err := repositories.NewURLRepository(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}
	defer repo.Close()

	// Initialize service
	svc := services.NewURLShortenerService(repo, cfg.Domain)

	// Initialize handlers
	h := handlers.NewHandlers(svc)

	// Setup router
	r := mux.NewRouter()
	r.HandleFunc("/newurl", h.CreateURL).Methods("POST")
	r.HandleFunc("/{shortKey}", h.GetURL).Methods("GET")

	// Start server
	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
