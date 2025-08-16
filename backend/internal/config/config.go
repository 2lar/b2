package config

import (
	"os"
)

type Config struct {
	TableName string
	IndexName string
	Features  Features
}

// Features contains feature flags for the application
type Features struct {
	EnableCaching      bool
	EnableAutoConnect  bool
	EnableAIProcessing bool
	EnableMetrics      bool
}

func LoadConfig() Config {
	tableName := os.Getenv("TABLE_NAME")
	if tableName == "" {
		tableName = "brain2-dev" // Default for local development
	}
	indexName := os.Getenv("INDEX_NAME")
	if indexName == "" {
		indexName = "KeywordIndex" // Default for local development
	}
	// Load feature flags from environment
	features := Features{
		EnableCaching:      os.Getenv("ENABLE_CACHING") == "true",
		EnableAutoConnect:  os.Getenv("ENABLE_AUTO_CONNECT") != "false", // Default true
		EnableAIProcessing: os.Getenv("ENABLE_AI_PROCESSING") == "true",
		EnableMetrics:      os.Getenv("ENABLE_METRICS") == "true",
	}
	
	return Config{
		TableName: tableName,
		IndexName: indexName,
		Features:  features,
	}
}
