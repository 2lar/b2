package utils

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

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