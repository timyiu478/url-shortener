package config

import (
    "errors"
    "fmt"

    "github.com/spf13/viper"
)

type Config struct {
    Port              string
		DynamoDBTable     string
		AWSRegion         string
		ShardId           int
}

func Load() (*Config, error) {
    viper.SetConfigName("config")
    viper.AddConfigPath("./configs")
    viper.AutomaticEnv()

    viper.SetDefault("PORT", "8080")
    viper.SetDefault("DYNAMODB_TABLE", "urls")
    viper.SetDefault("AWS_REGION", "us-east-1")
    viper.SetDefault("ShardId", "0")

    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, fmt.Errorf("failed to read config: %w", err)
        }
    }

    cfg := &Config{
        Port:              viper.GetString("PORT"),
        DynamoDBTable:     viper.GetString("DynamoDBTable"),
        AWSRegion:         viper.GetString("AWSRegion"),
        ShardId:           viper.GetInt("SHARDID")
    }

    // Validate config
    if cfg.Port == "" {
        return nil, errors.New("PORT is required")
    }
    if cfg.DynamoDBTable == "" {
        return nil, errors.New("DYNAMODB_TABLe is required")
    }
    if cfg.AWSRegion == "" {
        return nil, errors.New("AWS_REGION is required")
    }
    if cfg.ShardId < 0 || cfg.ShardId > 61 {
        return nil, errors.New("SHARDID is required")
    }

    return cfg, nil
}
