// Package errors provides enhanced error handling with context.
package errors

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
)

// WrapWithContext adds context and call location to errors.
// This helps with debugging by providing the file and line number where the error occurred.
func WrapWithContext(err error, msg string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	
	// Get caller information
	_, file, line, ok := runtime.Caller(1)
	if ok {
		// Extract just the filename, not the full path
		file = filepath.Base(file)
	} else {
		file = "unknown"
		line = 0
	}
	
	// Format the context message
	context := fmt.Sprintf(msg, args...)
	
	// Return wrapped error with location
	return fmt.Errorf("%s:%d: %s: %w", file, line, context, err)
}

// WrapWithStack is like Wrap but includes more stack frames.
// Use this for critical errors where you need the full call chain.
func WrapWithStack(err error, msg string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	
	// Format the context message
	context := fmt.Sprintf(msg, args...)
	
	// Get multiple stack frames
	var frames []string
	for i := 1; i <= 3; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		frames = append(frames, fmt.Sprintf("%s:%d", filepath.Base(file), line))
	}
	
	if len(frames) > 0 {
		return fmt.Errorf("[%s] %s: %w", frames[0], context, err)
	}
	
	return fmt.Errorf("%s: %w", context, err)
}

// WrapWithRequestContext enhances error with context.Context information including request ID and user ID.
// This provides enhanced tracing capabilities for debugging distributed operations.
func WrapWithRequestContext(ctx context.Context, err error, msg string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	
	// Start with basic context wrapping
	wrappedErr := WrapWithContext(err, msg, args...)
	
	// Extract additional context information
	var contextInfo []string
	
	// Try to extract request ID from context
	if requestID := extractRequestID(ctx); requestID != "" {
		contextInfo = append(contextInfo, fmt.Sprintf("req=%s", requestID))
	}
	
	// Try to extract user ID from context  
	if userID := extractUserID(ctx); userID != "" {
		contextInfo = append(contextInfo, fmt.Sprintf("user=%s", userID))
	}
	
	// If we have context info, enhance the error message
	if len(contextInfo) > 0 {
		return fmt.Errorf("[%s] %w", fmt.Sprintf("%v", contextInfo), wrappedErr)
	}
	
	return wrappedErr
}

// Helper functions to safely extract context values
func extractRequestID(ctx context.Context) string {
	if val := ctx.Value(contextKey{"requestID"}); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func extractUserID(ctx context.Context) string {
	if val := ctx.Value(contextKey{"userID"}); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// contextKey matches the type used in internal/context package
type contextKey struct {
	name string
}

// HasNotFoundContext checks if an error represents a "not found" condition.
// This is useful for handling missing resources gracefully.
func HasNotFoundContext(err error) bool {
	if err == nil {
		return false
	}
	// Check for common "not found" error patterns
	// You can extend this based on your specific error types
	return err.Error() == "not found" || err.Error() == "item not found"
}