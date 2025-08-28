package errors

import (
	"errors"
	"fmt"
	"testing"
	"time"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnifiedError_Creation(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *UnifiedError
		expected *UnifiedError
	}{
		{
			name: "validation error",
			builder: func() *UnifiedError {
				return Validation("INVALID_INPUT", "Input validation failed").
					WithDetails("Field 'email' is required").
					Build()
			},
			expected: &UnifiedError{
				Type:      ErrorTypeValidation,
				Code:      "INVALID_INPUT",
				Message:   "Input validation failed",
				Details:   "Field 'email' is required",
				Severity:  SeverityLow,
				Retryable: false,
			},
		},
		{
			name: "not found error",
			builder: func() *UnifiedError {
				return NotFound("RESOURCE_NOT_FOUND", "Resource not found").
					WithResource("node").
					Build()
			},
			expected: &UnifiedError{
				Type:      ErrorTypeNotFound,
				Code:      "RESOURCE_NOT_FOUND",
				Message:   "Resource not found",
				Resource:  "node",
				Severity:  SeverityLow,
				Retryable: false,
			},
		},
		{
			name: "retryable error",
			builder: func() *UnifiedError {
				return Timeout("OPERATION_TIMEOUT", "Operation timed out").
					WithRetryAfter(5 * time.Second).
					Build()
			},
			expected: &UnifiedError{
				Type:       ErrorTypeTimeout,
				Code:       "OPERATION_TIMEOUT",
				Message:    "Operation timed out",
				Severity:   SeverityMedium,
				Retryable:  true,
				RetryAfter: 5 * time.Second,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.builder()
			
			assert.Equal(t, tt.expected.Type, err.Type)
			assert.Equal(t, tt.expected.Code, err.Code)
			assert.Equal(t, tt.expected.Message, err.Message)
			assert.Equal(t, tt.expected.Details, err.Details)
			assert.Equal(t, tt.expected.Resource, err.Resource)
			assert.Equal(t, tt.expected.Severity, err.Severity)
			assert.Equal(t, tt.expected.Retryable, err.Retryable)
			assert.Equal(t, tt.expected.RetryAfter, err.RetryAfter)
		})
	}
}

func TestUnifiedError_ErrorInterface(t *testing.T) {
	err := Validation("TEST_CODE", "Test message").
		WithDetails("Additional details").
		Build()
	
	// Test Error() method
	expected := "[VALIDATION:TEST_CODE] Test message: Additional details"
	assert.Equal(t, expected, err.Error())
	
	// Test without details
	err2 := NotFound("NOT_FOUND", "Item not found").Build()
	expected2 := "[NOT_FOUND:NOT_FOUND] Item not found"
	assert.Equal(t, expected2, err2.Error())
}

func TestUnifiedError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	err := Internal("INTERNAL", "Wrapped error").
		WithCause(originalErr).
		Build()
	
	// Test Unwrap
	assert.Equal(t, originalErr, err.Unwrap())
	
	// Test errors.Is
	assert.True(t, errors.Is(err, originalErr))
}

func TestErrorType_Checking(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		checkFn  func(error) bool
		expected bool
	}{
		{
			name:     "IsValidation - true",
			err:      Validation("CODE", "msg").Build(),
			checkFn:  IsValidation,
			expected: true,
		},
		{
			name:     "IsValidation - false",
			err:      NotFound("CODE", "msg").Build(),
			checkFn:  IsValidation,
			expected: false,
		},
		{
			name:     "IsNotFound - true",
			err:      NotFound("CODE", "msg").Build(),
			checkFn:  IsNotFound,
			expected: true,
		},
		{
			name:     "IsTimeout - true",
			err:      Timeout("CODE", "msg").Build(),
			checkFn:  IsTimeout,
			expected: true,
		},
		{
			name:     "IsRetryable - true",
			err:      Timeout("CODE", "msg").Build(),
			checkFn:  IsRetryable,
			expected: true,
		},
		{
			name:     "IsRetryable - false",
			err:      Validation("CODE", "msg").Build(),
			checkFn:  IsRetryable,
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.checkFn(tt.err))
		})
	}
}

func TestErrorSeverity(t *testing.T) {
	tests := []struct {
		name     string
		err      *UnifiedError
		expected ErrorSeverity
	}{
		{
			name:     "validation error - low severity",
			err:      Validation("CODE", "msg").Build(),
			expected: SeverityLow,
		},
		{
			name:     "internal error - high severity",
			err:      Internal("CODE", "msg").Build(),
			expected: SeverityHigh,
		},
		{
			name:     "timeout error - medium severity",
			err:      Timeout("CODE", "msg").Build(),
			expected: SeverityMedium,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetSeverity(tt.err))
		})
	}
}

func TestErrorBuilder_FluentInterface(t *testing.T) {
	err := NewError(ErrorTypeValidation, "TEST_CODE", "Test message").
		WithDetails("Details").
		WithOperation("CreateNode").
		WithResource("node").
		WithUserID("user-123").
		WithRequestID("req-456").
		WithSeverity(SeverityHigh).
		WithRetryable(true).
		WithRetryAfter(10 * time.Second).
		WithRetryInfo(2, 5).
		WithRecoveryStrategy("Retry with exponential backoff").
		WithRecoveryMetadata(map[string]interface{}{
			"key": "value",
		}).
		Build()
	
	assert.Equal(t, ErrorTypeValidation, err.Type)
	assert.Equal(t, "TEST_CODE", err.Code)
	assert.Equal(t, "Test message", err.Message)
	assert.Equal(t, "Details", err.Details)
	assert.Equal(t, "CreateNode", err.Operation)
	assert.Equal(t, "node", err.Resource)
	assert.Equal(t, "user-123", err.UserID)
	assert.Equal(t, "req-456", err.RequestID)
	assert.Equal(t, SeverityHigh, err.Severity)
	assert.True(t, err.Retryable)
	assert.Equal(t, 10*time.Second, err.RetryAfter)
	assert.Equal(t, 2, err.RetryCount)
	assert.Equal(t, 5, err.MaxRetries)
	assert.Equal(t, "Retry with exponential backoff", err.RecoveryStrategy)
	assert.Equal(t, "value", err.RecoveryMetadata["key"])
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	
	// Test wrapping non-UnifiedError
	wrapped := Wrap(originalErr, "CreateNode", "Failed to create node")
	
	assert.Equal(t, ErrorTypeInternal, wrapped.Type)
	assert.Equal(t, "WRAP_ERROR", wrapped.Code)
	assert.Equal(t, "Failed to create node", wrapped.Message)
	assert.Equal(t, "original error", wrapped.Details)
	assert.Equal(t, "CreateNode", wrapped.Operation)
	assert.Equal(t, originalErr, wrapped.Cause)
	
	// Test wrapping UnifiedError
	unifiedErr := Validation("INVALID", "Invalid input").
		WithResource("node").
		Build()
	
	wrapped2 := Wrap(unifiedErr, "UpdateNode", "Update failed")
	
	assert.Equal(t, ErrorTypeValidation, wrapped2.Type) // Preserves original type
	assert.Equal(t, "INVALID", wrapped2.Code)           // Preserves original code
	assert.Equal(t, "Update failed", wrapped2.Message)
	assert.Equal(t, "Invalid input", wrapped2.Details) // Original message becomes details
	assert.Equal(t, "UpdateNode", wrapped2.Operation)
	assert.Equal(t, "node", wrapped2.Resource) // Preserves resource
}

func TestErrorCodes_HTTPStatus(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected int
	}{
		{CodeNodeValidationFailed, 400},
		{CodeUserUnauthorized, 401},
		{CodeUserForbidden, 403},
		{CodeNodeNotFound, 404},
		{CodeNodeAlreadyExists, 409},
		{CodeRateLimitExceeded, 429},
		{CodeServiceUnavailable, 503},
		{CodeInternalError, 500},
	}
	
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.HTTPStatusCode())
		})
	}
}

func TestErrorCodes_IsRetryable(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected bool
	}{
		{CodeTimeout, true},
		{CodeConnectionFailed, true},
		{CodeOptimisticLock, true},
		{CodeRateLimitExceeded, true},
		{CodeValidationFailed, false},
		{CodeNodeNotFound, false},
		{CodeUserUnauthorized, false},
	}
	
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.IsRetryable())
		})
	}
}

func TestBulkOperationError(t *testing.T) {
	bulkErr := NewBulkOperationError("BulkCreate", 10)
	
	// Add some successes
	for i := 0; i < 7; i++ {
		bulkErr.AddSuccess()
	}
	
	// Add some errors
	for i := 0; i < 3; i++ {
		err := Validation("INVALID", "Invalid item").Build()
		bulkErr.AddError(i, fmt.Sprintf("item-%d", i), err)
	}
	
	assert.True(t, bulkErr.HasErrors())
	assert.Equal(t, 7, bulkErr.SuccessfulItems)
	assert.Equal(t, 3, bulkErr.FailedItems)
	assert.Equal(t, 3, len(bulkErr.Errors))
	
	// Convert to UnifiedError
	unified := bulkErr.ToUnifiedError()
	require.NotNil(t, unified)
	
	assert.Equal(t, ErrorTypeApplication, unified.Type)
	assert.Contains(t, unified.Message, "3/10 items failed")
	assert.Equal(t, SeverityMedium, unified.Severity) // 30% failure rate
	
	// Check metadata
	metadata := unified.RecoveryMetadata
	assert.Equal(t, 10, metadata["total_items"])
	assert.Equal(t, 7, metadata["successful_items"])
	assert.Equal(t, 3, metadata["failed_items"])
	
	errorDetails := metadata["errors"].([]map[string]interface{})
	assert.Equal(t, 3, len(errorDetails))
}

func TestFromLegacyError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedType ErrorType
		expectedCode string
	}{
		{
			name:         "validation error",
			err:          errors.New("validation failed"),
			expectedType: ErrorTypeValidation,
			expectedCode: "DOMAIN_VALIDATION",
		},
		{
			name:         "not found error",
			err:          errors.New("item not found"),
			expectedType: ErrorTypeNotFound,
			expectedCode: "RESOURCE_NOT_FOUND",
		},
		{
			name:         "already exists error",
			err:          errors.New("resource already exists"),
			expectedType: ErrorTypeConflict,
			expectedCode: "RESOURCE_EXISTS",
		},
		{
			name:         "timeout error",
			err:          errors.New("operation timeout"),
			expectedType: ErrorTypeTimeout,
			expectedCode: "OPERATION_TIMEOUT",
		},
		{
			name:         "unknown error",
			err:          errors.New("something went wrong"),
			expectedType: ErrorTypeInternal,
			expectedCode: "UNKNOWN_ERROR",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified := FromLegacyError(tt.err)
			
			assert.Equal(t, tt.expectedType, unified.Type)
			assert.Equal(t, tt.expectedCode, unified.Code)
			assert.Contains(t, unified.Message, tt.err.Error())
		})
	}
}

func TestStackTrace(t *testing.T) {
	err := Internal("TEST", "Test error").Build()
	
	// Check that stack trace is captured
	assert.NotEmpty(t, err.StackTrace)
	assert.Greater(t, len(err.StackTrace), 0)
	
	// Check that file and line are captured
	assert.NotEmpty(t, err.File)
	assert.Greater(t, err.Line, 0)
}