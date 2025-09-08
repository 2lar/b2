package utils

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"
)

var (
	validate = validator.New()

	// Common validation patterns
	uuidRegex = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
)

// ValidateStruct validates a struct based on its validation tags
func ValidateStruct(s interface{}) error {
	if err := validate.Struct(s); err != nil {
		return formatValidationError(err)
	}
	return nil
}

// formatValidationError formats validation errors into readable messages
func formatValidationError(err error) error {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var errors []string
		for _, e := range validationErrors {
			errors = append(errors, formatFieldError(e))
		}
		return fmt.Errorf(strings.Join(errors, "; "))
	}
	return err
}

// formatFieldError formats a single field validation error
func formatFieldError(e validator.FieldError) string {
	field := strings.ToLower(e.Field())

	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, e.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, e.Param())
	case "dive":
		return fmt.Sprintf("%s contains invalid values", field)
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}

// ValidateUUID checks if a string is a valid UUID
func ValidateUUID(uuid string) bool {
	if uuid == "" || len(uuid) != 36 {
		return false
	}
	return uuidRegex.MatchString(uuid)
}

// ValidateStringLength validates string length with UTF-8 awareness
func ValidateStringLength(s string, minLength, maxLength int) error {
	length := utf8.RuneCountInString(s)
	if length < minLength {
		return fmt.Errorf("string too short: minimum %d characters required, got %d", minLength, length)
	}
	if maxLength > 0 && length > maxLength {
		return fmt.Errorf("string too long: maximum %d characters allowed, got %d", maxLength, length)
	}
	return nil
}

// ValidateRequired checks if a value is not empty
func ValidateRequired(value interface{}, fieldName string) error {
	if value == nil {
		return fmt.Errorf("%s is required", fieldName)
	}

	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s cannot be empty", fieldName)
		}
	case []interface{}:
		if len(v) == 0 {
			return fmt.Errorf("%s cannot be empty", fieldName)
		}
	case map[string]interface{}:
		if len(v) == 0 {
			return fmt.Errorf("%s cannot be empty", fieldName)
		}
	}

	return nil
}

// ValidateEnum checks if a value is in a list of allowed values
func ValidateEnum(value string, allowed []string, fieldName string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of: %v", fieldName, allowed)
}

// ValidateRange checks if a numeric value is within a range
func ValidateRange(value, min, max float64, fieldName string) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %f and %f", fieldName, min, max)
	}
	return nil
}

// SanitizeString removes potentially dangerous characters
func SanitizeString(input string) string {
	// Remove null bytes and control characters
	var result strings.Builder
	for _, r := range input {
		if r >= 32 && r != 127 { // Keep printable characters, skip control chars
			result.WriteRune(r)
		}
	}

	// Trim whitespace
	return strings.TrimSpace(result.String())
}

// NormalizeString normalizes a string for consistent storage
func NormalizeString(input string) string {
	// Trim whitespace
	input = strings.TrimSpace(input)

	// Normalize multiple spaces to single space
	input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")

	return input
}

// ValidationRule represents a reusable validation rule
type ValidationRule func(value interface{}) error

// CombineRules combines multiple validation rules
func CombineRules(rules ...ValidationRule) ValidationRule {
	return func(value interface{}) error {
		for _, rule := range rules {
			if err := rule(value); err != nil {
				return err
			}
		}
		return nil
	}
}

// StandardNodeTitleValidation provides standard validation for node titles
func StandardNodeTitleValidation() ValidationRule {
	return func(value interface{}) error {
		title, ok := value.(string)
		if !ok {
			return fmt.Errorf("title must be a string")
		}
		if err := ValidateRequired(title, "title"); err != nil {
			return err
		}
		return ValidateStringLength(title, 1, 200)
	}
}

// StandardNodeContentValidation provides standard validation for node content
func StandardNodeContentValidation() ValidationRule {
	return func(value interface{}) error {
		content, ok := value.(string)
		if !ok {
			return fmt.Errorf("content must be a string")
		}
		return ValidateStringLength(content, 0, 10000)
	}
}
