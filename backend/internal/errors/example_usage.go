// Package errors provides examples of using the unified error system.
package errors

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	
	"go.uber.org/zap"
)

// ExampleDomainLayerUsage shows how to use unified errors in the domain layer
func ExampleDomainLayerUsage() {
	// Creating a validation error
	err := Validation(CodeNodeContentEmpty.String(), "Node content cannot be empty").
		WithResource("node").
		WithDetails("Content is required for all nodes").
		Build()
	
	// Creating a business rule error
	err = NewError(ErrorTypeDomain, CodeNodeSelfConnection.String(), 
		"Cannot create self-referencing edge").
		WithResource("edge").
		WithSeverity(SeverityMedium).
		WithRecoveryStrategy("Use different source and target nodes").
		Build()
	
	// Creating a not found error
	err = NotFound(CodeNodeNotFound.String(), "Node not found").
		WithResource("node").
		WithRecoveryMetadata(map[string]interface{}{
			"node_id": "123e4567-e89b-12d3-a456-426614174000",
		}).
		Build()
	
	_ = err
}

// ExampleRepositoryLayerUsage shows how to use unified errors in the repository layer
func ExampleRepositoryLayerUsage(ctx context.Context) error {
	// Simulating a DynamoDB error
	var dbErr error // This would be the actual DynamoDB error
	
	// Wrap repository error with context
	if dbErr != nil {
		return RepositoryError("GetNode", "node", dbErr)
	}
	
	// Creating an optimistic lock error
	return Conflict(CodeOptimisticLock.String(), "Node version mismatch").
		WithOperation("UpdateNode").
		WithResource("node").
		WithRetryable(true).
		WithRecoveryMetadata(map[string]interface{}{
			"expected_version": 5,
			"actual_version":   6,
		}).
		Build()
}

// ExampleApplicationLayerUsage shows how to use unified errors in the application layer
func ExampleApplicationLayerUsage(ctx context.Context) error {
	// Wrap domain error with application context
	domainErr := NotFound(CodeNodeNotFound.String(), "node not found").Build()
	return ApplicationError(ctx, "CreateNodeCommand", domainErr)
}

// ExampleBulkOperationError shows how to handle bulk operation errors
func ExampleBulkOperationError() *UnifiedError {
	bulkErr := NewBulkOperationError("BulkCreateNodes", 100)
	
	// Simulate processing items
	for i := 0; i < 100; i++ {
		if i%10 == 0 { // Simulate 10% failure rate
			err := Validation(CodeNodeContentEmpty.String(), "Content is empty").Build()
			bulkErr.AddError(i, fmt.Sprintf("node-%d", i), err)
		} else {
			bulkErr.AddSuccess()
		}
	}
	
	// Convert to unified error
	return bulkErr.ToUnifiedError()
}

// ExampleHTTPHandlerUsage shows how to use unified errors in HTTP handlers
func ExampleHTTPHandlerUsage(w http.ResponseWriter, r *http.Request, logger *zap.Logger) {
	// Simulate a service error
	err := ServiceNotFoundError("node", "123e4567-e89b-12d3-a456-426614174000")
	
	// Write error response using the standardized system
	WriteHTTPError(w, err, logger)
}

// ExampleMiddlewareUsage shows how to set up error handling middleware
func ExampleMiddlewareUsage(logger *zap.Logger) http.Handler {
	mux := http.NewServeMux()
	
	// Apply error enrichment middleware
	handler := ErrorEnrichmentMiddleware(logger)(mux)
	
	// Apply correlation ID middleware
	structuredLogger, _ := NewStructuredLogger("development")
	handler = CorrelationIDMiddleware(structuredLogger)(handler)
	
	// Apply request logging middleware
	handler = RequestLoggingMiddleware(structuredLogger)(handler)
	
	// Apply error logging middleware
	handler = ErrorLoggingMiddleware(structuredLogger)(handler)
	
	return handler
}

// ExampleErrorChecking shows how to check error types
func ExampleErrorChecking(err error) {
	// Check error type
	if IsValidation(err) {
		// Handle validation error
		fmt.Println("Validation error occurred")
	}
	
	if IsNotFound(err) {
		// Handle not found error
		fmt.Println("Resource not found")
	}
	
	if IsRetryable(err) {
		// Retry the operation
		fmt.Println("Error is retryable")
	}
	
	// Get error severity
	severity := GetSeverity(err)
	switch severity {
	case SeverityCritical:
		// Page on-call engineer
		fmt.Println("Critical error - immediate attention required")
	case SeverityHigh:
		// Alert monitoring team
		fmt.Println("High severity error")
	case SeverityMedium:
		// Log and monitor
		fmt.Println("Medium severity error")
	case SeverityLow:
		// Log for analysis
		fmt.Println("Low severity error")
	}
}

// ExampleLoggingUsage shows how to use structured logging
func ExampleLoggingUsage(ctx context.Context) {
	logger, _ := NewStructuredLogger("production")
	
	// Log with context
	contextLogger := logger.WithContext(ctx)
	
	// Log service operation
	err := LogServiceCall(ctx, logger, "CreateNode", func() error {
		// Service logic here
		return nil
	})
	
	if err != nil {
		contextLogger.LogError(err, "Failed to create node",
			zap.String("operation", "CreateNode"),
		)
	}
	
	// Audit logging
	AuditLog(ctx, logger, "NODE_CREATED", map[string]interface{}{
		"node_id": "123e4567-e89b-12d3-a456-426614174000",
		"user_id": "user-123",
		"action":  "CREATE",
	})
	
	// Metrics logging
	MetricsLog(ctx, logger, "node.creation.duration", 125.5, map[string]string{
		"status": "success",
		"region": "us-east-1",
	})
}

// ExampleErrorRecovery shows how to use error recovery strategies
func ExampleErrorRecovery(err error) {
	var unifiedErr *UnifiedError
	if !errors.As(err, &unifiedErr) {
		return
	}
	
	// Check recovery strategy
	if unifiedErr.RecoveryStrategy != "" {
		fmt.Printf("Recovery strategy: %s\n", unifiedErr.RecoveryStrategy)
	}
	
	// Check if retryable
	if unifiedErr.Retryable {
		if unifiedErr.RetryAfter > 0 {
			fmt.Printf("Retry after %v seconds\n", unifiedErr.RetryAfter.Seconds())
		}
		
		if unifiedErr.MaxRetries > 0 {
			fmt.Printf("Maximum retries: %d\n", unifiedErr.MaxRetries)
		}
	}
	
	// Execute compensation function if available
	if unifiedErr.CompensationFunc != nil {
		if compensationErr := unifiedErr.CompensationFunc(context.Background()); compensationErr != nil {
			fmt.Printf("Compensation failed: %v\n", compensationErr)
		}
	}
}