package persistence

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.uber.org/zap"
)

// StoreType represents the type of store implementation.
type StoreType string

const (
	StoreTypeDynamoDB    StoreType = "dynamodb"
	StoreTypePostgreSQL  StoreType = "postgresql"
	StoreTypeMongoDB     StoreType = "mongodb"
	StoreTypeRedis       StoreType = "redis"
)

// DefaultStoreFactory implements StoreFactory for creating store instances.
type DefaultStoreFactory struct {
	logger *zap.Logger
}

// NewStoreFactory creates a new store factory.
func NewStoreFactory(logger *zap.Logger) StoreFactory {
	return &DefaultStoreFactory{
		logger: logger,
	}
}

// CreateStore creates a store instance based on the provided configuration.
func (f *DefaultStoreFactory) CreateStore(config StoreConfig) (Store, error) {
	storeTypeInterface, ok := config.Attributes["type"]
	if !ok {
		return nil, fmt.Errorf("store type not specified in configuration")
	}
	
	storeTypeStr, ok := storeTypeInterface.(string)
	if !ok {
		return nil, fmt.Errorf("store type must be a string")
	}
	
	storeType := StoreType(storeTypeStr)
	
	switch storeType {
	case StoreTypeDynamoDB:
		return f.createDynamoDBStore(config)
	case StoreTypePostgreSQL:
		return f.createPostgreSQLStore(config)
	case StoreTypeMongoDB:
		return f.createMongoDBStore(config)
	case StoreTypeRedis:
		return f.createRedisStore(config)
	default:
		return nil, fmt.Errorf("unsupported store type: %s", storeType)
	}
}

// GetSupportedTypes returns the list of supported store types.
func (f *DefaultStoreFactory) GetSupportedTypes() []string {
	return []string{
		string(StoreTypeDynamoDB),
		string(StoreTypePostgreSQL),
		string(StoreTypeMongoDB),
		string(StoreTypeRedis),
	}
}

// createDynamoDBStore creates a DynamoDB store instance.
func (f *DefaultStoreFactory) createDynamoDBStore(config StoreConfig) (Store, error) {
	// In a real implementation, you would create the DynamoDB client here
	// For now, we expect the client to be passed via the config attributes
	clientInterface, ok := config.Attributes["client"]
	if !ok {
		return nil, fmt.Errorf("DynamoDB client not provided in config")
	}
	
	client, ok := clientInterface.(*dynamodb.Client)
	if !ok {
		return nil, fmt.Errorf("invalid DynamoDB client type")
	}
	
	return NewDynamoDBStore(client, config, f.logger), nil
}

// createPostgreSQLStore creates a PostgreSQL store instance (placeholder).
func (f *DefaultStoreFactory) createPostgreSQLStore(config StoreConfig) (Store, error) {
	// Placeholder for future PostgreSQL implementation
	return nil, fmt.Errorf("PostgreSQL store not yet implemented")
}

// createMongoDBStore creates a MongoDB store instance (placeholder).
func (f *DefaultStoreFactory) createMongoDBStore(config StoreConfig) (Store, error) {
	// Placeholder for future MongoDB implementation
	return nil, fmt.Errorf("MongoDB store not yet implemented")
}

// createRedisStore creates a Redis store instance (placeholder).
func (f *DefaultStoreFactory) createRedisStore(config StoreConfig) (Store, error) {
	// Placeholder for future Redis implementation
	return nil, fmt.Errorf("Redis store not yet implemented")
}

// CreateDynamoDBConfig creates a standard DynamoDB store configuration.
func CreateDynamoDBConfig(tableName string, client *dynamodb.Client, logger *zap.Logger) StoreConfig {
	return StoreConfig{
		TableName:      tableName,
		IndexNames:     map[string]string{
			"GSI1": "GSI1",
			"GSI2": "GSI2",
		},
		TimeoutMs:      30000, // 30 second timeout
		RetryAttempts:  3,
		ConsistentRead: false, // Eventually consistent by default
		Attributes: map[string]interface{}{
			"type":   string(StoreTypeDynamoDB),
			"client": client,
		},
	}
}

// CreatePostgreSQLConfig creates a standard PostgreSQL store configuration (placeholder).
func CreatePostgreSQLConfig(connectionString string) StoreConfig {
	return StoreConfig{
		TableName:     "brain2_data",
		TimeoutMs:     30000,
		RetryAttempts: 3,
		Attributes: map[string]interface{}{
			"type":              string(StoreTypePostgreSQL),
			"connection_string": connectionString,
		},
	}
}

// StoreConfigBuilder provides a fluent interface for building store configurations.
type StoreConfigBuilder struct {
	config StoreConfig
}

// NewStoreConfigBuilder creates a new store configuration builder.
func NewStoreConfigBuilder(storeType StoreType) *StoreConfigBuilder {
	return &StoreConfigBuilder{
		config: StoreConfig{
			IndexNames: make(map[string]string),
			Attributes: map[string]interface{}{
				"type": string(storeType),
			},
			TimeoutMs:      30000,
			RetryAttempts:  3,
			ConsistentRead: false,
		},
	}
}

// WithTableName sets the table name.
func (b *StoreConfigBuilder) WithTableName(tableName string) *StoreConfigBuilder {
	b.config.TableName = tableName
	return b
}

// WithIndex adds an index configuration.
func (b *StoreConfigBuilder) WithIndex(name, indexName string) *StoreConfigBuilder {
	b.config.IndexNames[name] = indexName
	return b
}

// WithTimeout sets the operation timeout.
func (b *StoreConfigBuilder) WithTimeout(timeoutMs int32) *StoreConfigBuilder {
	b.config.TimeoutMs = timeoutMs
	return b
}

// WithRetryAttempts sets the number of retry attempts.
func (b *StoreConfigBuilder) WithRetryAttempts(attempts int) *StoreConfigBuilder {
	b.config.RetryAttempts = attempts
	return b
}

// WithConsistentRead enables/disables consistent reads.
func (b *StoreConfigBuilder) WithConsistentRead(consistent bool) *StoreConfigBuilder {
	b.config.ConsistentRead = consistent
	return b
}

// WithAttribute adds a store-specific attribute.
func (b *StoreConfigBuilder) WithAttribute(key string, value interface{}) *StoreConfigBuilder {
	b.config.Attributes[key] = value
	return b
}

// WithClient adds a client interface (for DynamoDB, PostgreSQL, etc.).
func (b *StoreConfigBuilder) WithClient(client interface{}) *StoreConfigBuilder {
	// Store the client in attributes as an interface{}
	if b.config.Attributes == nil {
		b.config.Attributes = make(map[string]interface{})
	}
	b.config.Attributes["client"] = client
	return b
}

// Build creates the final store configuration.
func (b *StoreConfigBuilder) Build() StoreConfig {
	return b.config
}