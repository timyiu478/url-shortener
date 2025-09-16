package config

import (
    "errors"
    "fmt"

    "github.com/spf13/viper"
)

type Config struct {
    Port              string
    DBEndpoint        string
    DBUser            string
    DBPassword        string
    DBName            string
    RedisEndpoint     string
    OTLPTraceEndpoint string
}

func Load() (*Config, error) {
    viper.SetConfigName("config")
    viper.AddConfigPath("./configs")
    viper.AutomaticEnv()

    viper.SetDefault("PORT", "8080")
    viper.SetDefault("DB_ENDPOINT", "localhost:3306")
    viper.SetDefault("DB_USER", "user")
    viper.SetDefault("DB_PASSWORD", "password")
    viper.SetDefault("DB_NAME", "urlshortener")
    viper.SetDefault("REDIS_ENDPOINT", "localhost:6379")
    viper.SetDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")

    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, fmt.Errorf("failed to read config: %w", err)
        }
    }

    cfg := &Config{
        Port:              viper.GetString("PORT"),
        DBEndpoint:        viper.GetString("DB_ENDPOINT"),
        DBUser:            viper.GetString("DB_USER"),
        DBPassword:        viper.GetString("DB_PASSWORD"),
        DBName:            viper.GetString("DB_NAME"),
        RedisEndpoint:     viper.GetString("REDIS_ENDPOINT"),
        OTLPTraceEndpoint: viper.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"),
    }

    // Validate config
    if cfg.Port == "" {
        return nil, errors.New("PORT is required")
    }
    if cfg.DBEndpoint == "" {
        return nil, errors.New("DB_ENDPOINT is required")
    }
    if cfg.DBUser == "" {
        return nil, errors.New("DB_USER is required")
    }
    if cfg.DBName == "" {
        return nil, errors.New("DB_NAME is required")
    }
    if cfg.RedisEndpoint == "" {
        return nil, errors.New("REDIS_ENDPOINT is required")
    }
    if cfg.OTLPTraceEndpoint == "" {
        return nil, errors.New("OTEL_EXPORTER_OTLP_ENDPOINT is required")
    }

    return cfg, nil
}
