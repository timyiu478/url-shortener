package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Port              string
	DynamoDBTableName string
	AWS_ENDPOINT      string
	AWSRegion         string
	ShardId           int
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.AddConfigPath("./configs")
	viper.AutomaticEnv()

	viper.SetDefault("PORT", "8080")
	viper.SetDefault("DYNAMODB_TABLE_NAME", "Urls")
	// Default endpoint when running inside compose points to the dynamodb service
	viper.SetDefault("AWS_ENDPOINT", "http://dynamodb:8000")
	viper.SetDefault("AWS_REGION", "us-east-1")
	viper.SetDefault("SHARDID", 0)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	cfg := &Config{
		Port:              viper.GetString("PORT"),
		DynamoDBTableName: viper.GetString("DYNAMODB_TABLE_NAME"),
		AWS_ENDPOINT:      viper.GetString("AWS_ENDPOINT"),
		AWSRegion:         viper.GetString("AWS_REGION"),
		ShardId:           viper.GetInt("SHARDID"),
	}

	// Validate config
	if cfg.Port == "" {
		return nil, errors.New("PORT is required")
	}
	if cfg.DynamoDBTableName == "" {
		return nil, errors.New("DYNAMODB_TABLE_NAME is required")
	}
	if cfg.AWS_ENDPOINT == "" {
		return nil, errors.New("AWS_ENDPOINT is required")
	}
	if cfg.AWSRegion == "" {
		return nil, errors.New("AWS_REGION is required")
	}
	if cfg.ShardId < 0 || cfg.ShardId > 61 {
		return nil, errors.New("SHARDID must be between 0 and 61")
	}

	return cfg, nil
}
