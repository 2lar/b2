// Package errors provides enhanced error handling with context.
package errors

import (
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