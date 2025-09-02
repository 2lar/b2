// Package helpers provides test helper utilities
package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ContextHelper provides context utilities for tests
type ContextHelper struct {
	t *testing.T
}

// NewContextHelper creates a new context helper
func NewContextHelper(t *testing.T) *ContextHelper {
	return &ContextHelper{t: t}
}

// WithTimeout creates a context with timeout
func (h *ContextHelper) WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// WithTestDeadline creates a context that expires with the test
func (h *ContextHelper) WithTestDeadline() context.Context {
	deadline, ok := h.t.Deadline()
	if !ok {
		// No test deadline, use 30 seconds as default
		ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
		return ctx
	}
	
	ctx, _ := context.WithDeadline(context.Background(), deadline)
	return ctx
}

// WithUserID adds a user ID to the context
func (h *ContextHelper) WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID retrieves user ID from context
func (h *ContextHelper) GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}

// WithCorrelationID adds a correlation ID to the context
func (h *ContextHelper) WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// GetCorrelationID retrieves correlation ID from context
func (h *ContextHelper) GetCorrelationID(ctx context.Context) (string, bool) {
	correlationID, ok := ctx.Value(correlationIDKey).(string)
	return correlationID, ok
}

// WithRequestID adds a request ID to the context
func (h *ContextHelper) WithRequestID(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestIDKey, uuid.New().String())
}

// GetRequestID retrieves request ID from context
func (h *ContextHelper) GetRequestID(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDKey).(string)
	return requestID, ok
}

// WithTestMetadata adds test metadata to context
func (h *ContextHelper) WithTestMetadata(ctx context.Context) context.Context {
	return context.WithValue(ctx, testMetadataKey, TestMetadata{
		TestName:  h.t.Name(),
		StartTime: time.Now(),
		TestID:    uuid.New().String(),
	})
}

// GetTestMetadata retrieves test metadata from context
func (h *ContextHelper) GetTestMetadata(ctx context.Context) (TestMetadata, bool) {
	metadata, ok := ctx.Value(testMetadataKey).(TestMetadata)
	return metadata, ok
}

// TestMetadata contains test-specific metadata
type TestMetadata struct {
	TestName  string
	StartTime time.Time
	TestID    string
}

// Context key types
type contextKey string

const (
	userIDKey        contextKey = "userID"
	correlationIDKey contextKey = "correlationID"
	requestIDKey     contextKey = "requestID"
	testMetadataKey  contextKey = "testMetadata"
)

// CreateTestContext creates a fully configured test context
func CreateTestContext(t *testing.T, userID string) context.Context {
	helper := NewContextHelper(t)
	ctx := helper.WithTestDeadline()
	ctx = helper.WithUserID(ctx, userID)
	ctx = helper.WithCorrelationID(ctx, uuid.New().String())
	ctx = helper.WithRequestID(ctx)
	ctx = helper.WithTestMetadata(ctx)
	return ctx
}

// TimeoutConfig provides standard timeout configurations for tests
type TimeoutConfig struct {
	Short  time.Duration // For fast operations
	Medium time.Duration // For normal operations
	Long   time.Duration // For slow operations
}

// DefaultTimeouts returns default timeout configurations
func DefaultTimeouts() TimeoutConfig {
	return TimeoutConfig{
		Short:  100 * time.Millisecond,
		Medium: 1 * time.Second,
		Long:   5 * time.Second,
	}
}

// LambdaTimeouts returns timeout configurations for Lambda environment
func LambdaTimeouts() TimeoutConfig {
	return TimeoutConfig{
		Short:  50 * time.Millisecond,
		Medium: 500 * time.Millisecond,
		Long:   2 * time.Second,
	}
}