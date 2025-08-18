// Package cloud provides abstractions for cloud service dependencies.
// This package implements the Dependency Inversion Principle by creating
// interfaces that abstract AWS SDK types and allow for easier testing and
// potential cloud provider switching.
package cloud

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ============================================================================
// DATABASE ABSTRACTIONS
// ============================================================================

// DatabaseClient provides an abstraction over DynamoDB operations.
// This interface follows the Dependency Inversion Principle by allowing
// the domain and application layers to depend on abstractions rather than
// concrete AWS SDK implementations.
type DatabaseClient interface {
	// Item operations
	GetItem(ctx context.Context, request GetItemRequest) (*GetItemResponse, error)
	PutItem(ctx context.Context, request PutItemRequest) (*PutItemResponse, error)
	UpdateItem(ctx context.Context, request UpdateItemRequest) (*UpdateItemResponse, error)
	DeleteItem(ctx context.Context, request DeleteItemRequest) (*DeleteItemResponse, error)
	
	// Query operations
	Query(ctx context.Context, request QueryRequest) (*QueryResponse, error)
	Scan(ctx context.Context, request ScanRequest) (*ScanResponse, error)
	
	// Batch operations
	BatchGetItem(ctx context.Context, request BatchGetItemRequest) (*BatchGetItemResponse, error)
	BatchWriteItem(ctx context.Context, request BatchWriteItemRequest) (*BatchWriteItemResponse, error)
	
	// Transaction operations
	TransactWriteItems(ctx context.Context, request TransactWriteItemsRequest) (*TransactWriteItemsResponse, error)
	TransactGetItems(ctx context.Context, request TransactGetItemsRequest) (*TransactGetItemsResponse, error)
}

// DatabaseTransaction provides an abstraction for database transactions.
type DatabaseTransaction interface {
	// Add operations to the transaction
	Put(tableName string, item map[string]interface{}) error
	Update(tableName string, key map[string]interface{}, updateExpression string, values map[string]interface{}) error
	Delete(tableName string, key map[string]interface{}) error
	
	// Execute the transaction
	Execute(ctx context.Context) error
	
	// Rollback the transaction (if supported)
	Rollback(ctx context.Context) error
}

// ============================================================================
// EVENT BUS ABSTRACTIONS
// ============================================================================

// EventPublisher provides an abstraction over event publishing services.
// This interface abstracts EventBridge operations and allows for different
// event publishing implementations.
type EventPublisher interface {
	// Publish a single event
	PublishEvent(ctx context.Context, request PublishEventRequest) (*PublishEventResponse, error)
	
	// Publish multiple events in a batch
	PublishEvents(ctx context.Context, request PublishEventsRequest) (*PublishEventsResponse, error)
	
	// Create custom event bus (if supported)
	CreateEventBus(ctx context.Context, request CreateEventBusRequest) (*CreateEventBusResponse, error)
	
	// List available event buses
	ListEventBuses(ctx context.Context) (*ListEventBusesResponse, error)
}

// ============================================================================
// REQUEST/RESPONSE TYPES
// ============================================================================

// Database request/response types
type GetItemRequest struct {
	TableName         string
	Key               map[string]interface{}
	ConsistentRead    bool
	ProjectionExpr    string
	AttributeNames    map[string]string
	AttributeValues   map[string]interface{}
}

type GetItemResponse struct {
	Item   map[string]interface{}
	Found  bool
}

type PutItemRequest struct {
	TableName           string
	Item                map[string]interface{}
	ConditionExpression string
	AttributeNames      map[string]string
	AttributeValues     map[string]interface{}
}

type PutItemResponse struct {
	Success           bool
	ConsumedCapacity  *ConsumedCapacity
}

type UpdateItemRequest struct {
	TableName           string
	Key                 map[string]interface{}
	UpdateExpression    string
	ConditionExpression string
	AttributeNames      map[string]string
	AttributeValues     map[string]interface{}
	ReturnValues        string
}

type UpdateItemResponse struct {
	Attributes       map[string]interface{}
	ConsumedCapacity *ConsumedCapacity
}

type DeleteItemRequest struct {
	TableName           string
	Key                 map[string]interface{}
	ConditionExpression string
	AttributeNames      map[string]string
	AttributeValues     map[string]interface{}
	ReturnValues        string
}

type DeleteItemResponse struct {
	Attributes       map[string]interface{}
	ConsumedCapacity *ConsumedCapacity
}

type QueryRequest struct {
	TableName                 string
	IndexName                 string
	KeyConditionExpression    string
	FilterExpression          string
	ProjectionExpression      string
	AttributeNames            map[string]string
	AttributeValues           map[string]interface{}
	ScanIndexForward          bool
	Limit                     int32
	ExclusiveStartKey         map[string]interface{}
	ConsistentRead            bool
}

type QueryResponse struct {
	Items            []map[string]interface{}
	Count            int32
	ScannedCount     int32
	LastEvaluatedKey map[string]interface{}
	ConsumedCapacity *ConsumedCapacity
}

type ScanRequest struct {
	TableName            string
	IndexName            string
	FilterExpression     string
	ProjectionExpression string
	AttributeNames       map[string]string
	AttributeValues      map[string]interface{}
	Limit                int32
	ExclusiveStartKey    map[string]interface{}
	ConsistentRead       bool
	Segment              int32
	TotalSegments        int32
}

type ScanResponse struct {
	Items            []map[string]interface{}
	Count            int32
	ScannedCount     int32
	LastEvaluatedKey map[string]interface{}
	ConsumedCapacity *ConsumedCapacity
}

type BatchGetItemRequest struct {
	RequestItems map[string]BatchGetItemTableRequest
}

type BatchGetItemTableRequest struct {
	Keys               []map[string]interface{}
	ConsistentRead     bool
	ProjectionExpr     string
	AttributeNames     map[string]string
}

type BatchGetItemResponse struct {
	Responses       map[string][]map[string]interface{}
	UnprocessedKeys map[string]BatchGetItemTableRequest
	ConsumedCapacity []*ConsumedCapacity
}

type BatchWriteItemRequest struct {
	RequestItems map[string][]WriteRequest
}

type WriteRequest struct {
	PutRequest    *PutRequest
	DeleteRequest *DeleteRequest
}

type PutRequest struct {
	Item map[string]interface{}
}

type DeleteRequest struct {
	Key map[string]interface{}
}

type BatchWriteItemResponse struct {
	UnprocessedItems map[string][]WriteRequest
	ConsumedCapacity []*ConsumedCapacity
}

type TransactWriteItemsRequest struct {
	TransactItems []TransactWriteItem
}

type TransactWriteItem struct {
	Put          *TransactPutItem
	Update       *TransactUpdateItem
	Delete       *TransactDeleteItem
	ConditionCheck *TransactConditionCheck
}

type TransactPutItem struct {
	TableName           string
	Item                map[string]interface{}
	ConditionExpression string
	AttributeNames      map[string]string
	AttributeValues     map[string]interface{}
}

type TransactUpdateItem struct {
	TableName           string
	Key                 map[string]interface{}
	UpdateExpression    string
	ConditionExpression string
	AttributeNames      map[string]string
	AttributeValues     map[string]interface{}
}

type TransactDeleteItem struct {
	TableName           string
	Key                 map[string]interface{}
	ConditionExpression string
	AttributeNames      map[string]string
	AttributeValues     map[string]interface{}
}

type TransactConditionCheck struct {
	TableName           string
	Key                 map[string]interface{}
	ConditionExpression string
	AttributeNames      map[string]string
	AttributeValues     map[string]interface{}
}

type TransactWriteItemsResponse struct {
	ConsumedCapacity []*ConsumedCapacity
}

type TransactGetItemsRequest struct {
	TransactItems []TransactGetItem
}

type TransactGetItem struct {
	TableName        string
	Key              map[string]interface{}
	ProjectionExpr   string
	AttributeNames   map[string]string
}

type TransactGetItemsResponse struct {
	Responses        []map[string]interface{}
	ConsumedCapacity []*ConsumedCapacity
}

// Event publisher request/response types
type PublishEventRequest struct {
	EventBusName string
	Source       string
	DetailType   string
	Detail       map[string]interface{}
	Resources    []string
	Time         *time.Time
}

type PublishEventResponse struct {
	EventID   string
	Success   bool
	ErrorCode string
	ErrorMsg  string
}

type PublishEventsRequest struct {
	Events []PublishEventRequest
}

type PublishEventsResponse struct {
	Results     []PublishEventResponse
	FailedCount int
}

type CreateEventBusRequest struct {
	Name        string
	Description string
	Tags        map[string]string
}

type CreateEventBusResponse struct {
	EventBusArn string
	Success     bool
}

type ListEventBusesResponse struct {
	EventBuses []EventBusInfo
}

type EventBusInfo struct {
	Name        string
	Arn         string
	Description string
	State       string
}

// ============================================================================
// COMMON TYPES
// ============================================================================

type ConsumedCapacity struct {
	TableName      string
	CapacityUnits  float64
	ReadCapacityUnits  float64
	WriteCapacityUnits float64
}

// ============================================================================
// FACTORY INTERFACES
// ============================================================================

// CloudClientFactory provides a factory for creating cloud service clients.
// This factory follows the Abstract Factory pattern and allows for
// dependency injection of different cloud implementations.
type CloudClientFactory interface {
	// Create database client
	CreateDatabaseClient(config DatabaseConfig) (DatabaseClient, error)
	
	// Create event publisher
	CreateEventPublisher(config EventPublisherConfig) (EventPublisher, error)
	
	// Create transaction
	CreateTransaction() DatabaseTransaction
}

// Configuration types
type DatabaseConfig struct {
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Timeout         time.Duration
	RetryAttempts   int
	RetryDelay      time.Duration
}

type EventPublisherConfig struct {
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Timeout         time.Duration
	RetryAttempts   int
	RetryDelay      time.Duration
}

// ============================================================================
// CLOUD PROVIDER ENUM
// ============================================================================

// CloudProvider represents different cloud providers.
type CloudProvider string

const (
	CloudProviderAWS   CloudProvider = "AWS"
	CloudProviderGCP   CloudProvider = "GCP"
	CloudProviderAzure CloudProvider = "AZURE"
	CloudProviderLocal CloudProvider = "LOCAL"
)

// CloudClientConfig provides configuration for cloud clients.
type CloudClientConfig struct {
	Provider         CloudProvider
	DatabaseConfig   DatabaseConfig
	EventConfig      EventPublisherConfig
	EnableMetrics    bool
	EnableTracing    bool
	MetricsNamespace string
}

// ============================================================================
// CLOUD CLIENT REGISTRY
// ============================================================================

// CloudClientRegistry provides a registry for different cloud client implementations.
// This registry follows the Registry pattern and allows for runtime selection
// of cloud providers.
type CloudClientRegistry interface {
	// Register a factory for a specific cloud provider
	RegisterFactory(provider CloudProvider, factory CloudClientFactory) error
	
	// Get a factory for a specific cloud provider
	GetFactory(provider CloudProvider) (CloudClientFactory, error)
	
	// List available providers
	ListProviders() []CloudProvider
	
	// Create clients for a specific provider
	CreateClients(provider CloudProvider, config CloudClientConfig) (*CloudClients, error)
}

// CloudClients contains all cloud service clients.
type CloudClients struct {
	Database DatabaseClient
	Events   EventPublisher
}

// ============================================================================
// ERROR TYPES
// ============================================================================

// CloudError represents errors from cloud operations.
type CloudError struct {
	Provider    CloudProvider
	Service     string
	Operation   string
	Code        string
	Message     string
	Retryable   bool
	Cause       error
}

func (e *CloudError) Error() string {
	return fmt.Sprintf("[%s:%s:%s] %s: %s", e.Provider, e.Service, e.Operation, e.Code, e.Message)
}

func (e *CloudError) Unwrap() error {
	return e.Cause
}

// IsCloudError checks if an error is a CloudError.
func IsCloudError(err error) bool {
	var cloudErr *CloudError
	return errors.As(err, &cloudErr)
}

// GetCloudProvider returns the cloud provider from a CloudError.
func GetCloudProvider(err error) CloudProvider {
	var cloudErr *CloudError
	if errors.As(err, &cloudErr) {
		return cloudErr.Provider
	}
	return ""
}