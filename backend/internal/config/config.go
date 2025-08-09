package config

import (
	"os"
)

type Config struct {
	TableName string
	IndexName string
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
	return Config{
		TableName: tableName,
		IndexName: indexName,
	}
}
