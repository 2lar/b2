// Package validation provides a centralized validation service for HTTP requests.
// This package demonstrates best practices for input validation including
// struct tag validation, custom validators, and business rule enforcement.
//
// Key Concepts Illustrated:
//   - Centralized Validation: Single source of truth for validation logic
//   - Declarative Validation: Using struct tags for common rules
//   - Custom Validators: Extending validation for business rules
//   - Error Aggregation: Collecting all validation errors
//   - Security: Input sanitization and injection prevention
//
// Design Principles:
//   - Fail fast: Validate at the HTTP boundary
//   - Clear errors: Provide actionable error messages
//   - Extensible: Easy to add new validation rules
//   - Testable: Validation logic is pure functions
//   - Performance: Cache validator instances
package validation

import (
	"brain2-backend/internal/interfaces/http/dto"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// Validator provides comprehensive request validation
type Validator struct {
	validate    *validator.Validate
	mu          sync.RWMutex
	customRules map[string]validator.Func
}

// singleton instance for performance
var (
	instance *Validator
	once     sync.Once
)

// GetValidator returns the singleton validator instance
func GetValidator() *Validator {
	once.Do(func() {
		instance = NewValidator()
	})
	return instance
}

// NewValidator creates a new validator with custom rules
func NewValidator() *Validator {
	v := &Validator{
		validate:    validator.New(),
		customRules: make(map[string]validator.Func),
	}

	// Configure validator
	v.configure()

	// Register custom validators
	v.registerCustomValidators()

	return v
}

// configure sets up the validator configuration
func (v *Validator) configure() {
	// Use JSON tag names in error messages
	v.validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Set custom error messages
	v.validate.RegisterValidation("required", v.requiredValidator)
}

// registerCustomValidators registers all custom validation rules
func (v *Validator) registerCustomValidators() {
	// Hex color validation
	v.RegisterCustom("hexcolor", v.hexColorValidator)

	// Node ID format validation
	v.RegisterCustom("nodeid", v.nodeIDValidator)

	// Safe string validation (no SQL injection)
	v.RegisterCustom("safestring", v.safeStringValidator)

	// Tag format validation
	v.RegisterCustom("tagformat", v.tagFormatValidator)

	// URL validation
	v.RegisterCustom("url", v.urlValidator)

	// Email validation
	v.RegisterCustom("email", v.emailValidator)
}

// Validate performs validation on any struct
func (v *Validator) Validate(i interface{}) error {
	// First, check if the struct implements its own Validate method
	if validator, ok := i.(SelfValidator); ok {
		if err := validator.Validate(); err != nil {
			return err
		}
	}

	// Then perform struct tag validation
	if err := v.validate.Struct(i); err != nil {
		return v.formatValidationError(err)
	}

	// Finally, check if it needs sanitization
	if sanitizer, ok := i.(Sanitizer); ok {
		sanitizer.Sanitize()
	}

	return nil
}

// ValidateVar validates a single variable
func (v *Validator) ValidateVar(field interface{}, tag string) error {
	return v.validate.Var(field, tag)
}

// RegisterCustom registers a custom validation function
func (v *Validator) RegisterCustom(tag string, fn validator.Func) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.customRules[tag] = fn
	return v.validate.RegisterValidation(tag, fn)
}

// formatValidationError converts validator errors to our error format
func (v *Validator) formatValidationError(err error) error {
	var errors dto.ValidationErrors

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			field := e.Field()
			tag := e.Tag()
			param := e.Param()

			message := v.getErrorMessage(field, tag, param, e.Value())

			errors.Errors = append(errors.Errors, dto.ValidationError{
				Field:   field,
				Message: message,
				Code:    strings.ToUpper(tag),
			})
		}
	}

	if len(errors.Errors) > 0 {
		return errors
	}

	return err
}

// getErrorMessage returns a human-readable error message for a validation error
func (v *Validator) getErrorMessage(field, tag, param string, value interface{}) string {
	switch tag {
	case "required":
		return "This field is required"
	case "min":
		return fmt.Sprintf("Must be at least %s characters", param)
	case "max":
		return fmt.Sprintf("Must be at most %s characters", param)
	case "email":
		return "Must be a valid email address"
	case "url":
		return "Must be a valid URL"
	case "hexcolor":
		return "Must be a valid hex color (e.g., #FF5733)"
	case "oneof":
		return fmt.Sprintf("Must be one of: %s", strings.ReplaceAll(param, " ", ", "))
	case "nodeid":
		return "Must be a valid node ID format"
	case "safestring":
		return "Contains invalid or potentially unsafe characters"
	case "tagformat":
		return "Must contain only letters, numbers, spaces, hyphens, and underscores"
	case "gte":
		return fmt.Sprintf("Must be greater than or equal to %s", param)
	case "lte":
		return fmt.Sprintf("Must be less than or equal to %s", param)
	case "len":
		return fmt.Sprintf("Must be exactly %s characters", param)
	case "dive":
		return "Invalid item in collection"
	default:
		return fmt.Sprintf("Failed %s validation", tag)
	}
}

// Custom validator implementations

func (v *Validator) requiredValidator(fl validator.FieldLevel) bool {
	field := fl.Field()

	switch field.Kind() {
	case reflect.String:
		return strings.TrimSpace(field.String()) != ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return field.Len() > 0
	default:
		return !field.IsZero()
	}
}

func (v *Validator) hexColorValidator(fl validator.FieldLevel) bool {
	color := fl.Field().String()
	if color == "" {
		return true // Optional field
	}

	matched, _ := regexp.MatchString(`^#[0-9A-Fa-f]{6}$`, color)
	return matched
}

func (v *Validator) nodeIDValidator(fl validator.FieldLevel) bool {
	id := fl.Field().String()
	if id == "" {
		return true // Optional field
	}

	// Node IDs should be alphanumeric with hyphens and underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`, id)
	return matched && len(id) <= 100
}

func (v *Validator) safeStringValidator(fl validator.FieldLevel) bool {
	str := fl.Field().String()
	if str == "" {
		return true // Optional field
	}

	// Check for potential SQL injection patterns
	dangerous := []string{
		"union", "select", "insert", "update", "delete", "drop",
		"script", "javascript", "vbscript", "onload", "onerror",
		"<", ">", "&lt;", "&gt;",
	}

	lower := strings.ToLower(str)
	for _, pattern := range dangerous {
		if strings.Contains(lower, pattern) {
			return false
		}
	}

	return true
}

func (v *Validator) tagFormatValidator(fl validator.FieldLevel) bool {
	tag := fl.Field().String()
	if tag == "" {
		return true // Optional field
	}

	// Tags should be alphanumeric with spaces, hyphens, and underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9\s_-]*[a-zA-Z0-9]$`, tag)
	return matched
}

func (v *Validator) urlValidator(fl validator.FieldLevel) bool {
	url := fl.Field().String()
	if url == "" {
		return true // Optional field
	}

	// Basic URL validation
	matched, _ := regexp.MatchString(`^https?://[^\s/$.?#].[^\s]*$`, url)
	return matched
}

func (v *Validator) emailValidator(fl validator.FieldLevel) bool {
	email := fl.Field().String()
	if email == "" {
		return true // Optional field
	}

	// Basic email validation
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, email)
	return matched
}

// Interfaces for self-validating and self-sanitizing types

// SelfValidator is implemented by types that can validate themselves
type SelfValidator interface {
	Validate() error
}

// Sanitizer is implemented by types that can sanitize their input
type Sanitizer interface {
	Sanitize()
}

// ValidateRequest is a helper function that validates and sanitizes a request
func ValidateRequest(req interface{}) error {
	v := GetValidator()
	return v.Validate(req)
}

// ValidationMiddleware creates an HTTP middleware for automatic validation
func ValidationMiddleware(targetType reflect.Type) func(next func(interface{}) error) func(interface{}) error {
	return func(next func(interface{}) error) func(interface{}) error {
		return func(req interface{}) error {
			if err := ValidateRequest(req); err != nil {
				return err
			}
			return next(req)
		}
	}
}

// BatchValidator validates multiple items and aggregates errors
type BatchValidator struct {
	errors []dto.ValidationError
	mu     sync.Mutex
}

// NewBatchValidator creates a new batch validator
func NewBatchValidator() *BatchValidator {
	return &BatchValidator{
		errors: make([]dto.ValidationError, 0),
	}
}

// ValidateItem validates a single item in the batch
func (b *BatchValidator) ValidateItem(index int, item interface{}) {
	if err := ValidateRequest(item); err != nil {
		b.mu.Lock()
		defer b.mu.Unlock()

		if valErrors, ok := err.(dto.ValidationErrors); ok {
			for _, e := range valErrors.Errors {
				// Prefix field with item index
				e.Field = fmt.Sprintf("[%d].%s", index, e.Field)
				b.errors = append(b.errors, e)
			}
		}
	}
}

// GetErrors returns all validation errors
func (b *BatchValidator) GetErrors() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.errors) == 0 {
		return nil
	}

	return dto.ValidationErrors{
		Errors: b.errors,
	}
}

// ValidatePagination validates common pagination parameters
func ValidatePagination(limit, offset int) error {
	var errors dto.ValidationErrors

	if limit < 1 {
		errors.Errors = append(errors.Errors, dto.ValidationError{
			Field:   "limit",
			Message: "Must be at least 1",
			Code:    "MIN",
		})
	}

	if limit > 100 {
		errors.Errors = append(errors.Errors, dto.ValidationError{
			Field:   "limit",
			Message: "Cannot exceed 100",
			Code:    "MAX",
		})
	}

	if offset < 0 {
		errors.Errors = append(errors.Errors, dto.ValidationError{
			Field:   "offset",
			Message: "Cannot be negative",
			Code:    "MIN",
		})
	}

	if len(errors.Errors) > 0 {
		return errors
	}

	return nil
}

// ValidateSort validates sort parameters
func ValidateSort(sortBy string, allowedFields []string) error {
	if sortBy == "" {
		return nil // Optional
	}

	for _, field := range allowedFields {
		if sortBy == field {
			return nil
		}
	}

	return dto.ValidationErrors{
		Errors: []dto.ValidationError{{
			Field:   "sort_by",
			Message: fmt.Sprintf("Must be one of: %s", strings.Join(allowedFields, ", ")),
			Code:    "INVALID_SORT_FIELD",
		}},
	}
}