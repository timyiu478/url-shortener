package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEndToEnd(t *testing.T) {
	if os.Getenv("RUN_E2E") != "1" {
		t.Skip("skipping e2e tests; set RUN_E2E=1 to run")
	}

	// Assumes service running at localhost:8080, with Aurora, ElastiCache, and OTLP collector
	baseURL := "http://localhost:8080"

	// Test health check
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/healthz")
		if err != nil {
			t.Fatalf("healthz request failed: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "OK", string(body))
	})

	// Test readiness check
	t.Run("ReadinessCheck", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/readyz")
		if err != nil {
			t.Fatalf("readyz request failed: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "OK", string(body))
	})

	// Test create and get URL
	t.Run("CreateAndGetURL", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{
			"domain": "shortenurl.org",
			"url":    "https://google.com",
		})
		resp, err := http.Post(baseURL+"/newurl", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("post newurl failed: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var respBody map[string]string
		json.NewDecoder(resp.Body).Decode(&respBody)
		shortenURL := respBody["shortenUrl"]
		assert.Contains(t, shortenURL, "https://shortenurl.org/")
		shortKey := shortenURL[len("https://shortenurl.org/"):]
		assert.True(t, regexp.MustCompile("^[0-9a-zA-Z]{9}$").MatchString(shortKey), "Short key %s is not Base62", shortKey)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err = client.Get(baseURL + "/" + shortKey)
		if err != nil {
			t.Fatalf("get short url failed: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotModified, resp.StatusCode)
		assert.Equal(t, "https://google.com", resp.Header.Get("Location"))
	})

	// Test invalid URL
	t.Run("InvalidURL", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{
			"domain": "shortenurl.org",
			"url":    "invalid",
		})
		resp, err := http.Post(baseURL+"/newurl", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("post invalid newurl failed: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	// Test not found
	t.Run("NotFound", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(baseURL + "/nonexist29")
		if err != nil {
			t.Fatalf("get nonexist failed: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

}
