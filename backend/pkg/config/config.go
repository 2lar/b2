package config

import (
	"os"
)

// Config holds all configuration values
type Config struct {
	TableName        string
	KeywordIndexName string
	Region           string
	LogLevel         string
}

// New creates a new configuration from environment variables
func New() *Config {
	return &Config{
		TableName:        getEnv("TABLE_NAME", "brain2"),
		KeywordIndexName: getEnv("KEYWORD_INDEX_NAME", "KeywordIndex"),
		Region:           getEnv("AWS_REGION", "us-east-1"),
		LogLevel:         getEnv("LOG_LEVEL", "INFO"),
	}
}

// getEnv gets an environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
