// Package ports defines service interfaces that external systems must implement
package ports

import (
	"context"
	"time"
	
	"brain2-backend/internal/core/domain/events"
)

// EventBus is the port for publishing and subscribing to domain events
type EventBus interface {
	// Publish sends an event to all subscribers
	Publish(ctx context.Context, event events.DomainEvent) error
	
	// PublishBatch sends multiple events
	PublishBatch(ctx context.Context, events []events.DomainEvent) error
	
	// Subscribe registers a handler for specific event types
	Subscribe(eventType string, handler EventHandler) error
	
	// SubscribeAll registers a handler for all events
	SubscribeAll(handler EventHandler) error
	
	// Unsubscribe removes a handler
	Unsubscribe(eventType string, handler EventHandler) error
	
	// Start begins processing events
	Start(ctx context.Context) error
	
	// Stop stops processing events
	Stop(ctx context.Context) error
}

// EventHandler processes domain events
type EventHandler func(ctx context.Context, event events.DomainEvent) error

// MessageQueue is the port for async messaging
type MessageQueue interface {
	// Send sends a message to a queue
	Send(ctx context.Context, queue string, message Message) error
	
	// SendDelayed sends a message with a delay
	SendDelayed(ctx context.Context, queue string, message Message, delay time.Duration) error
	
	// Receive receives messages from a queue
	Receive(ctx context.Context, queue string, handler MessageHandler) error
	
	// CreateQueue creates a new queue
	CreateQueue(ctx context.Context, name string, options QueueOptions) error
	
	// DeleteQueue deletes a queue
	DeleteQueue(ctx context.Context, name string) error
}

// Message represents a queue message
type Message struct {
	ID            string
	Body          []byte
	Attributes    map[string]string
	CorrelationID string
}

// MessageHandler processes queue messages
type MessageHandler func(ctx context.Context, message Message) error

// QueueOptions contains queue configuration
type QueueOptions struct {
	MaxRetries      int
	VisibilityTimeout time.Duration
	MessageRetention  time.Duration
	DeadLetterQueue   string
}

// Cache is the port for caching operations
type Cache interface {
	// Get retrieves a value from cache
	Get(ctx context.Context, key string) ([]byte, error)
	
	// Set stores a value in cache
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	
	// Delete removes a value from cache
	Delete(ctx context.Context, key string) error
	
	// Exists checks if a key exists
	Exists(ctx context.Context, key string) (bool, error)
	
	// Clear clears all cache entries
	Clear(ctx context.Context) error
	
	// GetMulti retrieves multiple values
	GetMulti(ctx context.Context, keys []string) (map[string][]byte, error)
	
	// SetMulti stores multiple values
	SetMulti(ctx context.Context, items map[string][]byte, ttl time.Duration) error
}

// Logger is the port for logging operations
type Logger interface {
	// Debug logs a debug message
	Debug(msg string, fields ...Field)
	
	// Info logs an info message
	Info(msg string, fields ...Field)
	
	// Warn logs a warning message
	Warn(msg string, fields ...Field)
	
	// Error logs an error message
	Error(msg string, err error, fields ...Field)
	
	// Fatal logs a fatal message and exits
	Fatal(msg string, err error, fields ...Field)
	
	// WithFields returns a logger with additional fields
	WithFields(fields ...Field) Logger
	
	// WithContext returns a logger with context
	WithContext(ctx context.Context) Logger
}

// Field represents a log field
type Field struct {
	Key   string
	Value interface{}
}

// Metrics is the port for metrics collection
type Metrics interface {
	// IncrementCounter increments a counter metric
	IncrementCounter(name string, tags ...Tag)
	
	// RecordGauge records a gauge value
	RecordGauge(name string, value float64, tags ...Tag)
	
	// RecordHistogram records a histogram value
	RecordHistogram(name string, value float64, tags ...Tag)
	
	// RecordDuration records a duration
	RecordDuration(name string, duration time.Duration, tags ...Tag)
	
	// StartTimer starts a timing operation
	StartTimer(name string, tags ...Tag) Timer
}

// Timer measures elapsed time
type Timer interface {
	// Stop stops the timer and records the duration
	Stop()
}

// Tag represents a metric tag
type Tag struct {
	Key   string
	Value string
}

// KeywordExtractor extracts keywords from text content
type KeywordExtractor interface {
	// Extract extracts keywords from text
	Extract(ctx context.Context, text string) ([]string, error)
	
	// ExtractWithOptions extracts keywords with custom options
	ExtractWithOptions(ctx context.Context, text string, options KeywordOptions) ([]string, error)
}

// KeywordOptions configures keyword extraction
type KeywordOptions struct {
	MaxKeywords  int
	MinLength    int
	Language     string
	ExcludeWords []string
}

// ConnectionAnalyzer analyzes and suggests connections between nodes
type ConnectionAnalyzer interface {
	// FindSimilarNodes finds nodes similar to the given content
	FindSimilarNodes(ctx context.Context, userID, content string, keywords []string, limit int) ([]string, error)
	
	// CalculateSimilarity calculates similarity between two nodes
	CalculateSimilarity(ctx context.Context, node1ID, node2ID string) (float64, error)
	
	// SuggestConnections suggests connections for a node
	SuggestConnections(ctx context.Context, nodeID string, limit int) ([]ConnectionSuggestion, error)
}

// ConnectionSuggestion represents a suggested connection
type ConnectionSuggestion struct {
	NodeID     string
	Score      float64
	Reason     string
	CommonTags []string
}

// SearchService provides full-text search capabilities
type SearchService interface {
	// Index adds or updates a document in the search index
	Index(ctx context.Context, doc SearchDocument) error
	
	// Delete removes a document from the search index
	Delete(ctx context.Context, id string) error
	
	// Search performs a search query
	Search(ctx context.Context, query SearchQuery) (*SearchResult, error)
	
	// BatchIndex indexes multiple documents
	BatchIndex(ctx context.Context, docs []SearchDocument) error
}

// SearchDocument represents a searchable document
type SearchDocument struct {
	ID        string
	UserID    string
	Content   string
	Title     string
	Tags      []string
	Keywords  []string
	Type      string
	UpdatedAt time.Time
	Metadata  map[string]interface{}
}

// SearchQuery configures a search operation
type SearchQuery struct {
	Query   string
	UserID  string
	Filters map[string]interface{}
	Limit   int
	Offset  int
	Sort    string
}

// SearchResult contains search results
type SearchResult struct {
	Items      []SearchDocument
	Hits       []SearchHit
	TotalCount int
	Facets     map[string][]FacetValue
	Duration   time.Duration
}

// FacetValue represents a facet value in search results
type FacetValue struct {
	Value string
	Count int
}

// GraphAnalyzer provides graph analysis capabilities
type GraphAnalyzer interface {
	// WouldCreateCycle checks if adding an edge would create a cycle
	WouldCreateCycle(ctx context.Context, sourceID, targetID string) (bool, error)
	
	// UpdateCentrality updates centrality scores for nodes
	UpdateCentrality(ctx context.Context, userID string, nodeIDs []string) error
	
	// UpdateClustering updates clustering coefficients
	UpdateClustering(ctx context.Context, userID string, nodeID string) error
	
	// FindShortestPath finds the shortest path between two nodes
	FindShortestPath(ctx context.Context, sourceID, targetID string) ([]string, error)
	
	// GetConnectedComponents finds connected components in the graph
	GetConnectedComponents(ctx context.Context, userID string) ([][]string, error)
}

// Note: Edge type is defined in repositories.go

// Tracer is the port for distributed tracing
type Tracer interface {
	// StartSpan starts a new span
	StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
	
	// Extract extracts span context from carrier
	Extract(ctx context.Context, carrier interface{}) context.Context
	
	// Inject injects span context into carrier
	Inject(ctx context.Context, carrier interface{}) error
}

// Span represents a trace span
type Span interface {
	// End ends the span
	End()
	
	// SetTag sets a tag on the span
	SetTag(key string, value interface{})
	
	// SetError marks the span as errored
	SetError(err error)
	
	// AddEvent adds an event to the span
	AddEvent(name string, attributes ...Attribute)
}

// SpanOption configures a span
type SpanOption func(*SpanConfig)

// SpanConfig contains span configuration
type SpanConfig struct {
	Kind       SpanKind
	Attributes []Attribute
}

// SpanKind represents the kind of span
type SpanKind int

const (
	SpanKindInternal SpanKind = iota
	SpanKindServer
	SpanKindClient
	SpanKindProducer
	SpanKindConsumer
)

// Attribute represents a span attribute
type Attribute struct {
	Key   string
	Value interface{}
}

// EmailService is the port for sending emails
type EmailService interface {
	// Send sends an email
	Send(ctx context.Context, email Email) error
	
	// SendBatch sends multiple emails
	SendBatch(ctx context.Context, emails []Email) error
	
	// ValidateAddress validates an email address
	ValidateAddress(address string) error
}

// Email represents an email message
type Email struct {
	From        string
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        string
	HTML        string
	Attachments []Attachment
}

// Attachment represents an email attachment
type Attachment struct {
	Name        string
	ContentType string
	Data        []byte
}

// SearchHit represents a single search result
type SearchHit struct {
	ID         string
	Score      float64
	Document   SearchDocument
	Highlights map[string][]string
}

// AuthService is the port for authentication operations
type AuthService interface {
	// Authenticate validates credentials
	Authenticate(ctx context.Context, credentials Credentials) (*AuthResult, error)
	
	// ValidateToken validates an auth token
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
	
	// RefreshToken refreshes an auth token
	RefreshToken(ctx context.Context, refreshToken string) (*AuthResult, error)
	
	// Revoke revokes a token
	Revoke(ctx context.Context, token string) error
}

// Credentials represents authentication credentials
type Credentials struct {
	Username string
	Password string
	MFA      string
}

// AuthResult contains authentication results
type AuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	UserID       string
}

// TokenClaims contains token claims
type TokenClaims struct {
	UserID    string
	Email     string
	Roles     []string
	ExpiresAt int64
}

// RateLimiter is the port for rate limiting
type RateLimiter interface {
	// Allow checks if a request is allowed
	Allow(ctx context.Context, key string) (bool, error)
	
	// AllowN checks if N requests are allowed
	AllowN(ctx context.Context, key string, n int) (bool, error)
	
	// Reset resets the rate limit for a key
	Reset(ctx context.Context, key string) error
	
	// GetLimit returns the current limit info
	GetLimit(ctx context.Context, key string) (*LimitInfo, error)
}

// LimitInfo contains rate limit information
type LimitInfo struct {
	Limit     int
	Remaining int
	ResetAt   time.Time
}