// Package validation provides unified validation logic to eliminate duplication
// across handlers and reduce repeated validation code.
package validation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ============================================================================
// COMMON VALIDATION FUNCTIONS
// ============================================================================

// CommonValidator provides reusable validation functions.
type CommonValidator struct {
	// Regular expressions for common validations
	uuidRegex  *regexp.Regexp
	emailRegex *regexp.Regexp
	slugRegex  *regexp.Regexp
}

// NewCommonValidator creates a new common validator with compiled regexes.
func NewCommonValidator() *CommonValidator {
	return &CommonValidator{
		uuidRegex:  regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`),
		emailRegex: regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
		slugRegex:  regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`),
	}
}

// ============================================================================
// STRING VALIDATIONS
// ============================================================================

// ValidateRequired checks if a string is non-empty after trimming.
func (v *CommonValidator) ValidateRequired(value, fieldName string) *ValidationError {
	if strings.TrimSpace(value) == "" {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s cannot be empty", fieldName),
		}
	}
	return nil
}

// ValidateLength checks if a string length is within specified bounds.
func (v *CommonValidator) ValidateLength(value, fieldName string, min, max int) *ValidationError {
	length := utf8.RuneCountInString(value)
	
	if min > 0 && length < min {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s must be at least %d characters long", fieldName, min),
		}
	}
	
	if max > 0 && length > max {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s cannot exceed %d characters", fieldName, max),
		}
	}
	
	return nil
}

// ValidateUUID checks if a string is a valid UUID.
func (v *CommonValidator) ValidateUUID(value, fieldName string) *ValidationError {
	if value == "" {
		return nil // Allow empty UUIDs if not required
	}
	
	if !v.uuidRegex.MatchString(strings.ToLower(value)) {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s must be a valid UUID", fieldName),
		}
	}
	
	return nil
}

// ValidateEmail checks if a string is a valid email address.
func (v *CommonValidator) ValidateEmail(value, fieldName string) *ValidationError {
	if value == "" {
		return nil // Allow empty emails if not required
	}
	
	if !v.emailRegex.MatchString(value) {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s must be a valid email address", fieldName),
		}
	}
	
	return nil
}

// ValidateSlug checks if a string is a valid URL slug.
func (v *CommonValidator) ValidateSlug(value, fieldName string) *ValidationError {
	if value == "" {
		return nil
	}
	
	if !v.slugRegex.MatchString(value) {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s must be a valid slug (lowercase letters, numbers, and hyphens only)", fieldName),
		}
	}
	
	return nil
}

// ValidateEnum checks if a value is in a list of allowed values.
func (v *CommonValidator) ValidateEnum(value, fieldName string, allowedValues []string) *ValidationError {
	if value == "" {
		return nil
	}
	
	for _, allowed := range allowedValues {
		if value == allowed {
			return nil
		}
	}
	
	return &ValidationError{
		Field:   fieldName,
		Message: fmt.Sprintf("%s must be one of: %s", fieldName, strings.Join(allowedValues, ", ")),
	}
}

// ValidatePattern checks if a string matches a custom pattern.
func (v *CommonValidator) ValidatePattern(value, fieldName, pattern string) *ValidationError {
	if value == "" {
		return nil
	}
	
	matched, err := regexp.MatchString(pattern, value)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s validation pattern is invalid", fieldName),
		}
	}
	
	if !matched {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s format is invalid", fieldName),
		}
	}
	
	return nil
}

// ============================================================================
// NUMERIC VALIDATIONS
// ============================================================================

// ValidateRange checks if a numeric value is within specified bounds.
func (v *CommonValidator) ValidateRange(value int, fieldName string, min, max int) *ValidationError {
	if min != 0 && value < min {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s must be at least %d", fieldName, min),
		}
	}
	
	if max != 0 && value > max {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s cannot exceed %d", fieldName, max),
		}
	}
	
	return nil
}

// ValidatePositive checks if a numeric value is positive.
func (v *CommonValidator) ValidatePositive(value int, fieldName string) *ValidationError {
	if value <= 0 {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s must be a positive number", fieldName),
		}
	}
	
	return nil
}

// ============================================================================
// COLLECTION VALIDATIONS
// ============================================================================

// ValidateArrayLength checks if an array length is within specified bounds.
func (v *CommonValidator) ValidateArrayLength(array []interface{}, fieldName string, min, max int) *ValidationError {
	length := len(array)
	
	if min > 0 && length < min {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s must contain at least %d items", fieldName, min),
		}
	}
	
	if max > 0 && length > max {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("%s cannot contain more than %d items", fieldName, max),
		}
	}
	
	return nil
}

// ValidateStringArray checks if all items in a string array meet certain criteria.
func (v *CommonValidator) ValidateStringArray(array []string, fieldName string, itemValidator func(string) *ValidationError) []ValidationError {
	var errors []ValidationError
	
	for i, item := range array {
		if err := itemValidator(item); err != nil {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("%s[%d]", fieldName, i),
				Message: err.Message,
			})
		}
	}
	
	return errors
}

// ValidateUniqueStrings checks if all strings in an array are unique.
func (v *CommonValidator) ValidateUniqueStrings(array []string, fieldName string) *ValidationError {
	seen := make(map[string]bool)
	
	for _, item := range array {
		if seen[item] {
			return &ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("%s must contain unique values", fieldName),
			}
		}
		seen[item] = true
	}
	
	return nil
}

// ============================================================================
// REQUEST PARSING AND VALIDATION
// ============================================================================

// RequestValidator provides utilities for parsing and validating HTTP requests.
type RequestValidator struct {
	common *CommonValidator
}

// NewRequestValidator creates a new request validator.
func NewRequestValidator() *RequestValidator {
	return &RequestValidator{
		common: NewCommonValidator(),
	}
}

// ParseAndValidateJSON parses JSON from request body and validates it.
func (rv *RequestValidator) ParseAndValidateJSON(r *http.Request, target interface{}, validator func(interface{}) []ValidationError) *ValidationResult {
	// Parse JSON
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return &ValidationResult{
			IsValid: false,
			Errors: []ValidationError{
				{Field: "body", Message: "Invalid JSON format"},
			},
		}
	}
	
	// Validate the parsed object
	if validator != nil {
		if errors := validator(target); len(errors) > 0 {
			return &ValidationResult{
				IsValid: false,
				Errors:  errors,
			}
		}
	}
	
	return &ValidationResult{IsValid: true}
}

// ValidateQueryParams validates query parameters from the request.
func (rv *RequestValidator) ValidateQueryParams(r *http.Request, validations map[string]func(string) *ValidationError) *ValidationResult {
	var errors []ValidationError
	
	for param, validator := range validations {
		value := r.URL.Query().Get(param)
		if err := validator(value); err != nil {
			errors = append(errors, *err)
		}
	}
	
	if len(errors) > 0 {
		return &ValidationResult{
			IsValid: false,
			Errors:  errors,
		}
	}
	
	return &ValidationResult{IsValid: true}
}

// ============================================================================
// COMPOSITE VALIDATORS
// ============================================================================

// NodeValidator provides validation specific to node operations.
type NodeValidator struct {
	common *CommonValidator
}

// NewNodeValidator creates a new node validator.
func NewNodeValidator() *NodeValidator {
	return &NodeValidator{
		common: NewCommonValidator(),
	}
}

// ValidateNodeID validates a node ID.
func (nv *NodeValidator) ValidateNodeID(nodeID string) *ValidationError {
	if err := nv.common.ValidateRequired(nodeID, "nodeId"); err != nil {
		return err
	}
	
	return nv.common.ValidateUUID(nodeID, "nodeId")
}

// ValidateNodeContent validates node content.
func (nv *NodeValidator) ValidateNodeContent(content string) *ValidationError {
	if err := nv.common.ValidateRequired(content, "content"); err != nil {
		return err
	}
	
	return nv.common.ValidateLength(content, "content", 1, 10000)
}

// ValidateNodeTags validates node tags.
func (nv *NodeValidator) ValidateNodeTags(tags []string) []ValidationError {
	var errors []ValidationError
	
	// Check array length
	if err := nv.common.ValidateArrayLength(toInterfaceSlice(tags), "tags", 0, 20); err != nil {
		errors = append(errors, *err)
	}
	
	// Check uniqueness
	if err := nv.common.ValidateUniqueStrings(tags, "tags"); err != nil {
		errors = append(errors, *err)
	}
	
	// Validate each tag
	tagErrors := nv.common.ValidateStringArray(tags, "tags", func(tag string) *ValidationError {
		return nv.common.ValidateLength(tag, "tag", 1, 50)
	})
	
	errors = append(errors, tagErrors...)
	return errors
}

// UserValidator provides validation specific to user operations.
type UserValidator struct {
	common *CommonValidator
}

// NewUserValidator creates a new user validator.
func NewUserValidator() *UserValidator {
	return &UserValidator{
		common: NewCommonValidator(),
	}
}

// ValidateUserID validates a user ID.
func (uv *UserValidator) ValidateUserID(userID string) *ValidationError {
	if err := uv.common.ValidateRequired(userID, "userId"); err != nil {
		return err
	}
	
	return uv.common.ValidateLength(userID, "userId", 1, 255)
}

// ============================================================================
// VALIDATION UTILITIES
// ============================================================================

// CombineValidationResults combines multiple validation results.
func CombineValidationResults(results ...*ValidationResult) *ValidationResult {
	var allErrors []ValidationError
	
	for _, result := range results {
		if result != nil && !result.IsValid {
			allErrors = append(allErrors, result.Errors...)
		}
	}
	
	return &ValidationResult{
		IsValid: len(allErrors) == 0,
		Errors:  allErrors,
	}
}

// FormatValidationErrors formats validation errors for HTTP response.
func FormatValidationErrors(errors []ValidationError) string {
	if len(errors) == 0 {
		return "Validation failed"
	}
	
	var messages []string
	for _, err := range errors {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	
	return strings.Join(messages, "; ")
}

// toInterfaceSlice converts a string slice to interface slice for generic validation.
func toInterfaceSlice(strings []string) []interface{} {
	interfaces := make([]interface{}, len(strings))
	for i, s := range strings {
		interfaces[i] = s
	}
	return interfaces
}