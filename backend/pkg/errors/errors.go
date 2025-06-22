package errors

import (
	"fmt"
)

// ErrorType defines different categories of errors
type ErrorType string

const (
	ErrorTypeValidation ErrorType = "VALIDATION"
	ErrorTypeNotFound   ErrorType = "NOT_FOUND"
	ErrorTypeInternal   ErrorType = "INTERNAL"
)

// AppError is the custom error type for the application
type AppError struct {
	Type    ErrorType
	Message string
	Err     error
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap allows errors.Is and errors.As to work
func (e *AppError) Unwrap() error {
	return e.Err
}

// Constructor functions for different error types

// NewValidation creates a validation error
func NewValidation(message string) error {
	return &AppError{
		Type:    ErrorTypeValidation,
		Message: message,
	}
}

// NewNotFound creates a not found error
func NewNotFound(message string) error {
	return &AppError{
		Type:    ErrorTypeNotFound,
		Message: message,
	}
}

// NewInternal creates an internal error
func NewInternal(message string, err error) error {
	return &AppError{
		Type:    ErrorTypeInternal,
		Message: message,
		Err:     err,
	}
}

// Wrap wraps an error with additional context
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}

	// If it's already an AppError, preserve the type
	if appErr, ok := err.(*AppError); ok {
		return &AppError{
			Type:    appErr.Type,
			Message: fmt.Sprintf("%s: %s", message, appErr.Message),
			Err:     appErr.Err,
		}
	}

	// Otherwise, create an internal error
	return &AppError{
		Type:    ErrorTypeInternal,
		Message: message,
		Err:     err,
	}
}

// Type checking functions

// IsValidation checks if an error is a validation error
func IsValidation(err error) bool {
	appErr, ok := err.(*AppError)
	return ok && appErr.Type == ErrorTypeValidation
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	appErr, ok := err.(*AppError)
	return ok && appErr.Type == ErrorTypeNotFound
}

// IsInternal checks if an error is an internal error
func IsInternal(err error) bool {
	appErr, ok := err.(*AppError)
	return ok && appErr.Type == ErrorTypeInternal
}
