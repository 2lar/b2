// Package shared provides domain error definitions using the unified error system.
package shared

import (
	"brain2-backend/internal/errors"
)

// Domain error definitions using unified error system
var (
	// Node errors
	ErrInvalidNodeID = errors.Validation(errors.CodeInvalidUUID.String(), "invalid node ID: must be a valid UUID").
		WithResource("node").
		Build()
	ErrNodeNotFound = errors.NotFound(errors.CodeNodeNotFound.String(), "node not found").
		WithResource("node").
		Build()
	ErrNodeAlreadyExists = errors.Conflict(errors.CodeNodeAlreadyExists.String(), "node already exists").
		WithResource("node").
		Build()
	ErrCannotUpdateArchivedNode = errors.NewError(errors.ErrorTypeDomain, errors.CodeNodeArchived.String(), "cannot update archived node").
		WithResource("node").
		WithSeverity(errors.SeverityMedium).
		Build()
	ErrCannotConnectToSelf = errors.NewError(errors.ErrorTypeDomain, errors.CodeNodeSelfConnection.String(), "cannot connect node to itself").
		WithResource("edge").
		WithSeverity(errors.SeverityLow).
		Build()
	ErrCrossUserConnection = errors.NewError(errors.ErrorTypeDomain, errors.CodeNodeCrossUser.String(), "cannot connect nodes from different users").
		WithResource("edge").
		WithSeverity(errors.SeverityMedium).
		Build()
	ErrCannotConnectArchivedNodes = errors.NewError(errors.ErrorTypeDomain, errors.CodeCannotConnectArchived.String(), "cannot connect archived nodes").
		WithResource("edge").
		WithSeverity(errors.SeverityMedium).
		Build()

	// Edge errors
	ErrEdgeNotFound = errors.NotFound(errors.CodeEdgeNotFound.String(), "edge not found").
		WithResource("edge").
		Build()
	ErrEdgeAlreadyExists = errors.Conflict(errors.CodeEdgeAlreadyExists.String(), "edge already exists").
		WithResource("edge").
		Build()
	ErrInvalidEdge = errors.Validation(errors.CodeEdgeValidationFailed.String(), "invalid edge: source and target must be different").
		WithResource("edge").
		Build()

	// Category errors
	ErrCategoryNotFound = errors.NotFound(errors.CodeCategoryNotFound.String(), "category not found").
		WithResource("category").
		Build()
	ErrCategoryAlreadyExists = errors.Conflict(errors.CodeCategoryAlreadyExists.String(), "category already exists").
		WithResource("category").
		Build()
	ErrCircularReference = errors.NewError(errors.ErrorTypeDomain, errors.CodeCategoryCircularRef.String(), "circular reference detected in category hierarchy").
		WithResource("category").
		WithSeverity(errors.SeverityHigh).
		Build()
	ErrInvalidCategoryLevel = errors.Validation(errors.CodeCategoryInvalidLevel.String(), "invalid category level").
		WithResource("category").
		Build()
	ErrInvalidCategoryID = errors.Validation(errors.CodeInvalidUUID.String(), "invalid category ID").
		WithResource("category").
		Build()

	// Content errors
	ErrEmptyContent = errors.Validation(errors.CodeContentEmpty.String(), "content cannot be empty").
		WithResource("content").
		Build()
	ErrContentTooLong = errors.Validation(errors.CodeContentTooLong.String(), "content exceeds maximum length").
		WithResource("content").
		Build()
	ErrInappropriateContent = errors.Validation(errors.CodeInappropriateContent.String(), "content contains inappropriate material").
		WithResource("content").
		WithSeverity(errors.SeverityHigh).
		Build()
	ErrTitleTooLong = errors.Validation(errors.CodeNodeTitleTooLong.String(), "title exceeds maximum length").
		WithResource("node").
		Build()

	// User errors
	ErrEmptyUserID = errors.Validation(errors.CodeUserIDEmpty.String(), "user ID cannot be empty").
		WithResource("user").
		Build()
	ErrUserIDTooLong = errors.Validation(errors.CodeUserIDTooLong.String(), "user ID exceeds maximum length").
		WithResource("user").
		Build()
	ErrUnauthorized = errors.Unauthorized(errors.CodeUserUnauthorized.String(), "unauthorized operation").
		WithResource("user").
		Build()

	// General errors
	ErrValidation = errors.Validation(errors.CodeValidationFailed.String(), "validation failed").
		Build()
	ErrConflict = errors.Conflict(errors.CodeOptimisticLock.String(), "conflict detected").
		Build()
	ErrNotFound = errors.NotFound("RESOURCE_NOT_FOUND", "resource not found").
		Build()
	ErrInternalError = errors.Internal(errors.CodeInternalError.String(), "internal error").
		Build()
	ErrInvalidOperation = errors.NewError(errors.ErrorTypeDomain, errors.CodeInvalidInput.String(), "invalid operation").
		WithSeverity(errors.SeverityMedium).
		Build()
)

// Error type checking helpers

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	return errors.IsValidation(err)
}

// IsBusinessRuleError checks if an error is a business rule error
func IsBusinessRuleError(err error) bool {
	// Check for domain-specific business rule violations
	return errors.IsType(err, errors.ErrorTypeDomain)
}

// IsConflictError checks if an error is a conflict error
func IsConflictError(err error) bool {
	return errors.IsConflict(err)
}

// IsNotFoundError checks if an error is a not found error
func IsNotFoundError(err error) bool {
	return errors.IsNotFound(err)
}

// IsUnauthorizedError checks if an error is an unauthorized error
func IsUnauthorizedError(err error) bool {
	return errors.IsUnauthorized(err)
}