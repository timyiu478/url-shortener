package config

import (
	"fmt"
	"os"
)

// Config holds application configuration
type Config struct {
	Port          string
	DBEndpoint    string
	DBUser        string
	DBPassword    string
	DBName        string
	RedisEndpoint string
	Domain        string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		DBEndpoint:    getEnv("DB_ENDPOINT", ""),
		DBUser:        getEnv("DB_USER", ""),
		DBPassword:    getEnv("DB_PASSWORD", ""),
		DBName:        getEnv("DB_NAME", ""),
		RedisEndpoint: getEnv("REDIS_ENDPOINT", ""),
		Domain:        getEnv("DOMAIN", "shortenurl.org"),
	}

	if cfg.DBEndpoint == "" || cfg.DBUser == "" || cfg.DBPassword == "" || cfg.DBName == "" || cfg.RedisEndpoint == "" {
		return nil, fmt.Errorf("missing required configuration")
	}

	return cfg, nil
}

// getEnv retrieves environment variable with fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
