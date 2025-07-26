package config

import (
	"os"
	"strconv"
)

// Config holds all configuration values
type Config struct {
	TableName        string
	KeywordIndexName string
	Region           string
	LogLevel         string
	DynamoDBEndpoint string
	// Tagger configuration
	TaggerType       string
	TaggerServiceURL string
	TaggerMaxTags    int
	TaggerFallback   bool
}

// New creates a new configuration from environment variables
func New() *Config {
	maxTags, _ := strconv.Atoi(getEnv("TAGGER_MAX_TAGS", "5"))
	fallback, _ := strconv.ParseBool(getEnv("TAGGER_FALLBACK", "true"))
	
	return &Config{
		TableName:        getEnv("TABLE_NAME", "brain2"),
		KeywordIndexName: getEnv("KEYWORD_INDEX_NAME", "KeywordIndex"),
		Region:           getEnv("AWS_REGION", "us-east-1"),
		LogLevel:         getEnv("LOG_LEVEL", "INFO"),
		DynamoDBEndpoint: getEnv("DYNAMODB_ENDPOINT", ""),
		// Tagger configuration
		TaggerType:       getEnv("TAGGER_TYPE", "local_llm"),
		TaggerServiceURL: getEnv("TAGGER_SERVICE_URL", "http://tagger-service:8000"),
		TaggerMaxTags:    maxTags,
		TaggerFallback:   fallback,
	}
}

// getEnv gets an environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
