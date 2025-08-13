package repository

import (
	"brain2-backend/internal/domain"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// Limits for validation
	MaxContentLength = 10000 // Maximum content length in characters
	MaxKeywordCount  = 50    // Maximum number of keywords per node
	MaxKeywordLength = 100   // Maximum length of a single keyword
	MaxNodeIDLength  = 100   // Maximum length of node ID
	MaxUserIDLength  = 100   // Maximum length of user ID
	MinContentLength = 1     // Minimum content length
	MinKeywordLength = 1     // Minimum keyword length
)

var (
	// Regex patterns for validation
	validIDPattern      = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	validKeywordPattern = regexp.MustCompile(`^[a-zA-Z0-9\s_-]+$`)
	sqlInjectionPattern = regexp.MustCompile(`(?i)(union|select|insert|update|delete|drop|create|alter|exec|execute|script|javascript|vbscript|onload|onerror)`)
)

// ValidationError represents validation-specific errors
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	_, ok := err.(ValidationError)
	return ok
}

// ValidateNode validates a domain node for repository operations
func ValidateNode(node *domain.Node) error {
	// Validate node ID
	if err := validateID(node.ID().String(), "NodeID"); err != nil {
		return err
	}

	// Validate user ID
	if err := validateID(node.UserID().String(), "UserID"); err != nil {
		return err
	}

	// Validate content
	if err := validateContent(node.Content().String()); err != nil {
		return err
	}

	// Validate keywords
	if err := validateKeywords(node.Keywords().ToSlice()); err != nil {
		return err
	}

	// Validate version
	if node.Version().Int() < 0 {
		return ValidationError{
			Field:   "Version",
			Value:   fmt.Sprintf("%d", node.Version().Int()),
			Message: "version cannot be negative",
		}
	}

	return nil
}

// ValidateNodeIDs validates a slice of node IDs
func ValidateNodeIDs(nodeIDs []string) error {
	if len(nodeIDs) == 0 {
		return nil
	}

	for i, nodeID := range nodeIDs {
		if err := validateID(nodeID, fmt.Sprintf("NodeID[%d]", i)); err != nil {
			return err
		}
	}

	return nil
}

// ValidateUserID validates a user ID
func ValidateUserID(userID string) error {
	return validateID(userID, "UserID")
}

// validateID validates ID format and length
func validateID(id, fieldName string) error {
	if id == "" {
		return ValidationError{
			Field:   fieldName,
			Value:   id,
			Message: "cannot be empty",
		}
	}

	if len(id) > MaxNodeIDLength {
		return ValidationError{
			Field:   fieldName,
			Value:   id,
			Message: fmt.Sprintf("length exceeds maximum of %d characters", MaxNodeIDLength),
		}
	}

	if !validIDPattern.MatchString(id) {
		return ValidationError{
			Field:   fieldName,
			Value:   id,
			Message: "contains invalid characters (only alphanumeric, underscore, and hyphen allowed)",
		}
	}

	// Check for potential injection attempts
	if sqlInjectionPattern.MatchString(strings.ToLower(id)) {
		return ValidationError{
			Field:   fieldName,
			Value:   id,
			Message: "contains potentially malicious content",
		}
	}

	return nil
}

// validateContent validates node content
func validateContent(content string) error {
	if content == "" {
		return ValidationError{
			Field:   "Content",
			Value:   content,
			Message: "cannot be empty",
		}
	}

	if !utf8.ValidString(content) {
		return ValidationError{
			Field:   "Content",
			Value:   content,
			Message: "contains invalid UTF-8 characters",
		}
	}

	if len(content) > MaxContentLength {
		return ValidationError{
			Field:   "Content",
			Value:   fmt.Sprintf("(length: %d)", len(content)),
			Message: fmt.Sprintf("length exceeds maximum of %d characters", MaxContentLength),
		}
	}

	if len(content) < MinContentLength {
		return ValidationError{
			Field:   "Content",
			Value:   content,
			Message: fmt.Sprintf("length is below minimum of %d characters", MinContentLength),
		}
	}

	return nil
}

// validateKeywords validates keyword list
func validateKeywords(keywords []string) error {
	if len(keywords) > MaxKeywordCount {
		return ValidationError{
			Field:   "Keywords",
			Value:   fmt.Sprintf("(count: %d)", len(keywords)),
			Message: fmt.Sprintf("count exceeds maximum of %d", MaxKeywordCount),
		}
	}

	seen := make(map[string]bool)
	for i, keyword := range keywords {
		fieldName := fmt.Sprintf("Keywords[%d]", i)

		// Check for empty keywords
		if keyword == "" {
			return ValidationError{
				Field:   fieldName,
				Value:   keyword,
				Message: "cannot be empty",
			}
		}

		// Check keyword length
		if len(keyword) > MaxKeywordLength {
			return ValidationError{
				Field:   fieldName,
				Value:   keyword,
				Message: fmt.Sprintf("length exceeds maximum of %d characters", MaxKeywordLength),
			}
		}

		if len(keyword) < MinKeywordLength {
			return ValidationError{
				Field:   fieldName,
				Value:   keyword,
				Message: fmt.Sprintf("length is below minimum of %d characters", MinKeywordLength),
			}
		}

		// Check for valid characters
		if !validKeywordPattern.MatchString(keyword) {
			return ValidationError{
				Field:   fieldName,
				Value:   keyword,
				Message: "contains invalid characters",
			}
		}

		// Check for duplicates
		normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
		if seen[normalizedKeyword] {
			return ValidationError{
				Field:   fieldName,
				Value:   keyword,
				Message: "duplicate keyword detected",
			}
		}
		seen[normalizedKeyword] = true

		// Check for potential injection attempts
		if sqlInjectionPattern.MatchString(strings.ToLower(keyword)) {
			return ValidationError{
				Field:   fieldName,
				Value:   keyword,
				Message: "contains potentially malicious content",
			}
		}
	}

	return nil
}

// SanitizeKeywords removes duplicates and normalizes keywords
func SanitizeKeywords(keywords []string) []string {
	seen := make(map[string]bool)
	var sanitized []string

	for _, keyword := range keywords {
		// Normalize keyword
		normalized := strings.ToLower(strings.TrimSpace(keyword))

		// Skip empty or duplicate keywords
		if normalized == "" || seen[normalized] {
			continue
		}

		seen[normalized] = true
		sanitized = append(sanitized, normalized)
	}

	return sanitized
}

// SanitizeContent removes potentially harmful content and normalizes
func SanitizeContent(content string) string {
	// Remove null bytes and control characters except newlines and tabs
	sanitized := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return r
		}
		if r < 32 || r == 127 {
			return -1 // Remove control characters
		}
		return r
	}, content)

	// Trim excessive whitespace
	sanitized = strings.TrimSpace(sanitized)

	return sanitized
}
