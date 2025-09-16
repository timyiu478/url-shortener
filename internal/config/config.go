package config

import (
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
    viper.SetDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318") // Default OTLP endpoint

    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, err
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
    return cfg, nil
}
