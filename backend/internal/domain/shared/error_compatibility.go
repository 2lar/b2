// Package shared provides compatibility functions for the domain layer
// to use the unified error system without import cycles.
package shared

import (
	"fmt"
	"brain2-backend/internal/errors"
)

// DomainError represents a domain-specific error (for backward compatibility)
type DomainError struct {
	Type    string
	Message string
	Cause   error
	Context map[string]interface{}
}

func (e DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s - %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// NewDomainError creates a domain-specific error using the unified error system
func NewDomainError(errorType, message string, cause error) error {
	return errors.NewError(errors.ErrorTypeDomain, errorType, message).
		WithCause(cause).
		WithSeverity(errors.SeverityMedium).
		Build()
}

// NewValidationError creates a validation error using the unified error system
func NewValidationError(field, message string, value interface{}) error {
	return errors.Validation(errors.CodeValidationFailed.String(), 
		fmt.Sprintf("Field '%s': %s", field, message)).
		WithDetails(fmt.Sprintf("Value: %v", value)).
		Build()
}

// NewBusinessRuleError creates a business rule error using the unified error system
func NewBusinessRuleError(rule, entity, message string) *errors.UnifiedError {
	return errors.NewError(errors.ErrorTypeDomain, "BUSINESS_RULE_VIOLATION",
		fmt.Sprintf("Rule '%s' violated for %s: %s", rule, entity, message)).
		WithSeverity(errors.SeverityMedium).
		Build()
}

// NewConflictError creates a conflict error using the unified error system
func NewConflictError(resource, identifier, message string, expectedVersion, actualVersion int) error {
	return errors.Conflict(errors.CodeOptimisticLock.String(),
		fmt.Sprintf("Conflict in %s '%s': %s", resource, identifier, message)).
		WithRecoveryMetadata(map[string]interface{}{
			"expected_version": expectedVersion,
			"actual_version":   actualVersion,
		}).
		Build()
}