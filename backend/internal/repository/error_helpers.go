package repository

import (
	"brain2-backend/internal/errors"
)

// Helper functions to create standardized repository errors
// These replace the old sentinel errors with unified error system

// ErrNodeNotFound creates a standardized node not found error
func ErrNodeNotFound(userID, nodeID string) error {
	return errors.NotFound(errors.CodeNodeNotFound.String(), "node not found").
		WithDetails("Node does not exist or access is not allowed").
		WithUserID(userID).
		WithResource("node").
		Build()
}

// ErrEdgeNotFound creates a standardized edge not found error
func ErrEdgeNotFound(userID, edgeID string) error {
	return errors.NotFound(errors.CodeEdgeNotFound.String(), "edge not found").
		WithDetails("Edge does not exist or access is not allowed").
		WithUserID(userID).
		WithResource("edge").
		Build()
}

// NewOptimisticLockError creates a standardized optimistic lock error
func NewOptimisticLockError(resourceID string, expectedVersion, actualVersion int) error {
	return errors.Conflict(errors.CodeOptimisticLock.String(), "optimistic lock error").
		WithDetails("Resource was modified by another process").
		WithResource("resource").
		Build()
}

// ErrCategoryNotFound creates a standardized category not found error
func ErrCategoryNotFound(userID, categoryID string) error {
	return errors.NotFound(errors.CodeCategoryNotFound.String(), "category not found").
		WithDetails("Category does not exist or access is not allowed").
		WithUserID(userID).
		WithResource("category").
		Build()
}

// Helper functions to check error types
// These replace the old IsXXX helper functions

// IsNotFound checks if an error represents a not found condition
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if ue, ok := err.(*errors.UnifiedError); ok {
		return ue.Type == errors.ErrorTypeNotFound ||
			ue.Code == errors.CodeNodeNotFound.String() ||
			ue.Code == errors.CodeEdgeNotFound.String() ||
			ue.Code == errors.CodeCategoryNotFound.String() ||
			ue.Code == errors.CodeUserNotFound.String()
	}
	return false
}

// IsConflict checks if an error represents a conflict condition
func IsConflict(err error) bool {
	if err == nil {
		return false
	}
	if ue, ok := err.(*errors.UnifiedError); ok {
		return ue.Type == errors.ErrorTypeConflict ||
			ue.Code == errors.CodeNodeAlreadyExists.String() ||
			ue.Code == errors.CodeEdgeAlreadyExists.String() ||
			ue.Code == errors.CodeCategoryAlreadyExists.String() ||
			ue.Code == errors.CodeOptimisticLock.String() ||
			ue.Code == errors.CodeIdempotencyConflict.String()
	}
	return false
}

// IsInvalidQuery checks if an error represents an invalid query condition
func IsInvalidQuery(err error) bool {
	if err == nil {
		return false
	}
	if ue, ok := err.(*errors.UnifiedError); ok {
		return ue.Type == errors.ErrorTypeValidation ||
			ue.Code == errors.CodeValidationFailed.String() ||
			ue.Code == errors.CodeInvalidInput.String() ||
			ue.Code == errors.CodeMissingField.String() ||
			ue.Code == errors.CodeInvalidFormat.String()
	}
	return false
}