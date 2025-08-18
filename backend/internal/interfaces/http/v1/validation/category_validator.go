// Package validation provides HTTP request validation for category operations.
package validation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// CategoryValidator handles validation for category-related HTTP requests.
// This validator implements the Single Responsibility Principle by focusing
// solely on input validation logic.
type CategoryValidator struct{}

// NewCategoryValidator creates a new category validator.
func NewCategoryValidator() *CategoryValidator {
	return &CategoryValidator{}
}

// CreateCategoryRequest represents the request body for creating a category.
type CreateCategoryRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// UpdateCategoryRequest represents the request body for updating a category.
type UpdateCategoryRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// ValidationError represents a validation error with detailed information.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult contains validation outcome and any errors.
type ValidationResult struct {
	IsValid bool              `json:"isValid"`
	Errors  []ValidationError `json:"errors,omitempty"`
}

// ValidateCreateCategoryRequest validates and parses create category request.
func (v *CategoryValidator) ValidateCreateCategoryRequest(r *http.Request) (*CreateCategoryRequest, *ValidationResult) {
	var req CreateCategoryRequest
	
	// Parse JSON body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, &ValidationResult{
			IsValid: false,
			Errors: []ValidationError{
				{Field: "body", Message: "Invalid JSON format"},
			},
		}
	}
	
	// Validate fields
	var errors []ValidationError
	
	if strings.TrimSpace(req.Title) == "" {
		errors = append(errors, ValidationError{
			Field:   "title",
			Message: "Title cannot be empty",
		})
	} else if len(req.Title) > 255 {
		errors = append(errors, ValidationError{
			Field:   "title",
			Message: "Title cannot exceed 255 characters",
		})
	}
	
	if len(req.Description) > 1000 {
		errors = append(errors, ValidationError{
			Field:   "description",
			Message: "Description cannot exceed 1000 characters",
		})
	}
	
	// Trim whitespace
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	
	if len(errors) > 0 {
		return nil, &ValidationResult{
			IsValid: false,
			Errors:  errors,
		}
	}
	
	return &req, &ValidationResult{IsValid: true}
}

// ValidateUpdateCategoryRequest validates and parses update category request.
func (v *CategoryValidator) ValidateUpdateCategoryRequest(r *http.Request) (*UpdateCategoryRequest, *ValidationResult) {
	var req UpdateCategoryRequest
	
	// Parse JSON body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, &ValidationResult{
			IsValid: false,
			Errors: []ValidationError{
				{Field: "body", Message: "Invalid JSON format"},
			},
		}
	}
	
	// Validate fields
	var errors []ValidationError
	
	if strings.TrimSpace(req.Title) == "" {
		errors = append(errors, ValidationError{
			Field:   "title",
			Message: "Title cannot be empty",
		})
	} else if len(req.Title) > 255 {
		errors = append(errors, ValidationError{
			Field:   "title",
			Message: "Title cannot exceed 255 characters",
		})
	}
	
	if len(req.Description) > 1000 {
		errors = append(errors, ValidationError{
			Field:   "description",
			Message: "Description cannot exceed 1000 characters",
		})
	}
	
	// Trim whitespace
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	
	if len(errors) > 0 {
		return nil, &ValidationResult{
			IsValid: false,
			Errors:  errors,
		}
	}
	
	return &req, &ValidationResult{IsValid: true}
}

// FormatValidationErrors formats validation errors for HTTP response.
func (v *CategoryValidator) FormatValidationErrors(errors []ValidationError) string {
	if len(errors) == 0 {
		return "Validation failed"
	}
	
	var messages []string
	for _, err := range errors {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	
	return strings.Join(messages, "; ")
}