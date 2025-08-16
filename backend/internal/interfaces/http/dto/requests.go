// Package dto contains Data Transfer Objects for HTTP request/response handling.
// This package demonstrates best practices for API contract definition, validation,
// and separation of concerns between the HTTP layer and business logic.
//
// Key Concepts Illustrated:
//   - Request DTOs: Structured input validation at the HTTP boundary
//   - Validation Tags: Declarative validation using struct tags
//   - Custom Validation: Business rule validation beyond struct tags
//   - Partial Updates: Optional fields for PATCH operations
//   - Command Pattern: Converting DTOs to application commands
//   - Security: Input sanitization and injection prevention
//
// Design Principles:
//   - DTOs are immutable after validation
//   - Validation happens at the edge (fail fast)
//   - DTOs don't contain business logic
//   - Clear error messages for better UX
//   - Security by default (whitelist approach)
package dto

import (
	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/queries"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Validation constants define limits and patterns for input validation
const (
	MaxContentLength = 10000
	MinContentLength = 1
	MaxTagCount      = 20
	MaxTagLength     = 50
	MinTagLength     = 1
	MaxTitleLength   = 200
	MinTitleLength   = 1
	MaxDescLength    = 1000
	MaxBulkItems     = 100
)

var (
	// Validation patterns
	validTagPattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\s_-]*[a-zA-Z0-9]$`)
	validIDPattern    = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	sqlInjectionCheck = regexp.MustCompile(`(?i)(union|select|insert|update|delete|drop|script|javascript)`)
)

// ValidationError represents a validation error with field-level details
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// ValidationErrors collects multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (v ValidationErrors) Error() string {
	if len(v.Errors) == 0 {
		return "validation failed"
	}
	var messages []string
	for _, err := range v.Errors {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return strings.Join(messages, "; ")
}

// CreateNodeRequest represents the HTTP request for creating a node.
// This DTO validates and sanitizes input before it enters the business layer.
type CreateNodeRequest struct {
	Content string   `json:"content" validate:"required,min=1,max=10000"`
	Tags    []string `json:"tags,omitempty" validate:"max=20,dive,min=1,max=50"`
}

// Validate performs custom validation beyond struct tags
func (r *CreateNodeRequest) Validate() error {
	var errors ValidationErrors

	// Validate content
	if err := validateContent(r.Content); err != nil {
		errors.Errors = append(errors.Errors, ValidationError{
			Field:   "content",
			Message: err.Error(),
			Code:    "INVALID_CONTENT",
		})
	}

	// Validate tags
	if len(r.Tags) > 0 {
		if err := validateTags(r.Tags); err != nil {
			errors.Errors = append(errors.Errors, ValidationError{
				Field:   "tags",
				Message: err.Error(),
				Code:    "INVALID_TAGS",
			})
		}
	}

	if len(errors.Errors) > 0 {
		return errors
	}
	return nil
}

// Sanitize cleans the input data
func (r *CreateNodeRequest) Sanitize() {
	r.Content = sanitizeContent(r.Content)
	r.Tags = sanitizeTags(r.Tags)
}

// ToCommand converts the request to an application command
func (r *CreateNodeRequest) ToCommand(userID string) (*commands.CreateNodeCommand, error) {
	return &commands.CreateNodeCommand{
		UserID:  userID,
		Content: r.Content,
		Tags:    r.Tags,
	}, nil
}

// UpdateNodeRequest supports partial updates with optional fields.
// nil fields are not updated, allowing PATCH semantics.
type UpdateNodeRequest struct {
	Content *string  `json:"content,omitempty" validate:"omitempty,min=1,max=10000"`
	Tags    []string `json:"tags,omitempty" validate:"omitempty,max=20,dive,min=1,max=50"`
	Version *int     `json:"version,omitempty" validate:"omitempty,min=0"` // For optimistic locking
}

// Validate performs custom validation
func (r *UpdateNodeRequest) Validate() error {
	var errors ValidationErrors

	// Only validate content if provided
	if r.Content != nil {
		if err := validateContent(*r.Content); err != nil {
			errors.Errors = append(errors.Errors, ValidationError{
				Field:   "content",
				Message: err.Error(),
				Code:    "INVALID_CONTENT",
			})
		}
	}

	// Only validate tags if provided
	if len(r.Tags) > 0 {
		if err := validateTags(r.Tags); err != nil {
			errors.Errors = append(errors.Errors, ValidationError{
				Field:   "tags",
				Message: err.Error(),
				Code:    "INVALID_TAGS",
			})
		}
	}

	if len(errors.Errors) > 0 {
		return errors
	}
	return nil
}

// HasChanges checks if the request contains any updates
func (r *UpdateNodeRequest) HasChanges() bool {
	return r.Content != nil || len(r.Tags) > 0
}

// Sanitize cleans the input data
func (r *UpdateNodeRequest) Sanitize() {
	if r.Content != nil {
		sanitized := sanitizeContent(*r.Content)
		r.Content = &sanitized
	}
	if len(r.Tags) > 0 {
		r.Tags = sanitizeTags(r.Tags)
	}
}

// ToCommand converts the request to an application command
func (r *UpdateNodeRequest) ToCommand(userID, nodeID string) (*commands.UpdateNodeCommand, error) {
	cmd := &commands.UpdateNodeCommand{
		NodeID: nodeID,
		UserID: userID,
		Tags:   r.Tags,
	}

	if r.Content != nil {
		cmd.Content = *r.Content
	}

	if r.Version != nil {
		cmd.Version = *r.Version
	}

	return cmd, nil
}

// BulkDeleteNodesRequest handles bulk deletion with safety limits
type BulkDeleteNodesRequest struct {
	NodeIDs []string `json:"node_ids" validate:"required,min=1,max=100,dive,required"`
}

// Validate performs custom validation
func (r *BulkDeleteNodesRequest) Validate() error {
	var errors ValidationErrors

	if len(r.NodeIDs) == 0 {
		errors.Errors = append(errors.Errors, ValidationError{
			Field:   "node_ids",
			Message: "at least one node ID is required",
			Code:    "EMPTY_LIST",
		})
	}

	if len(r.NodeIDs) > MaxBulkItems {
		errors.Errors = append(errors.Errors, ValidationError{
			Field:   "node_ids",
			Message: fmt.Sprintf("cannot delete more than %d nodes at once", MaxBulkItems),
			Code:    "TOO_MANY_ITEMS",
		})
	}

	// Validate each ID
	seen := make(map[string]bool)
	for i, id := range r.NodeIDs {
		if !validIDPattern.MatchString(id) {
			errors.Errors = append(errors.Errors, ValidationError{
				Field:   fmt.Sprintf("node_ids[%d]", i),
				Message: "invalid node ID format",
				Code:    "INVALID_ID",
			})
		}

		// Check for duplicates
		if seen[id] {
			errors.Errors = append(errors.Errors, ValidationError{
				Field:   fmt.Sprintf("node_ids[%d]", i),
				Message: "duplicate node ID",
				Code:    "DUPLICATE_ID",
			})
		}
		seen[id] = true
	}

	if len(errors.Errors) > 0 {
		return errors
	}
	return nil
}

// ToCommand converts the request to an application command
func (r *BulkDeleteNodesRequest) ToCommand(userID string) (*commands.BulkDeleteNodesCommand, error) {
	return &commands.BulkDeleteNodesCommand{
		UserID:  userID,
		NodeIDs: r.NodeIDs,
	}, nil
}

// ListNodesRequest handles pagination and filtering for node queries
type ListNodesRequest struct {
	Limit     int      `json:"limit,omitempty" validate:"omitempty,min=1,max=100"`
	NextToken string   `json:"next_token,omitempty"`
	Tags      []string `json:"tags,omitempty" validate:"omitempty,max=10,dive,min=1,max=50"`
	SortBy    string   `json:"sort_by,omitempty" validate:"omitempty,oneof=created_at updated_at content"`
	Order     string   `json:"order,omitempty" validate:"omitempty,oneof=asc desc"`
}

// SetDefaults applies default values for optional fields
func (r *ListNodesRequest) SetDefaults() {
	if r.Limit == 0 {
		r.Limit = 20
	}
	if r.SortBy == "" {
		r.SortBy = "created_at"
	}
	if r.Order == "" {
		r.Order = "desc"
	}
}

// ToQuery converts the request to an application query
func (r *ListNodesRequest) ToQuery(userID string) (*queries.ListNodesQuery, error) {
	r.SetDefaults()
	
	query, err := queries.NewListNodesQuery(userID)
	if err != nil {
		return nil, err
	}
	
	query.WithPagination(r.Limit, r.NextToken)
	
	if len(r.Tags) > 0 {
		query.WithTagFilter(r.Tags)
	}
	
	if r.SortBy != "" && r.Order != "" {
		query.WithSort(r.SortBy, r.Order)
	}
	
	return query, nil
}

// CreateCategoryRequest represents the request to create a category
type CreateCategoryRequest struct {
	Title       string  `json:"title" validate:"required,min=1,max=200"`
	Description string  `json:"description,omitempty" validate:"max=1000"`
	Color       *string `json:"color,omitempty" validate:"omitempty,hexcolor"`
	Icon        *string `json:"icon,omitempty" validate:"omitempty,max=50"`
	ParentID    *string `json:"parent_id,omitempty"`
}

// Validate performs custom validation
func (r *CreateCategoryRequest) Validate() error {
	var errors ValidationErrors

	// Validate title
	if err := validateTitle(r.Title); err != nil {
		errors.Errors = append(errors.Errors, ValidationError{
			Field:   "title",
			Message: err.Error(),
			Code:    "INVALID_TITLE",
		})
	}

	// Validate description if provided
	if r.Description != "" {
		if err := validateDescription(r.Description); err != nil {
			errors.Errors = append(errors.Errors, ValidationError{
				Field:   "description",
				Message: err.Error(),
				Code:    "INVALID_DESCRIPTION",
			})
		}
	}

	// Validate parent ID if provided
	if r.ParentID != nil && !validIDPattern.MatchString(*r.ParentID) {
		errors.Errors = append(errors.Errors, ValidationError{
			Field:   "parent_id",
			Message: "invalid parent category ID format",
			Code:    "INVALID_PARENT_ID",
		})
	}

	if len(errors.Errors) > 0 {
		return errors
	}
	return nil
}

// Sanitize cleans the input data
func (r *CreateCategoryRequest) Sanitize() {
	r.Title = sanitizeTitle(r.Title)
	r.Description = sanitizeDescription(r.Description)
}

// ToCommand converts the request to an application command
func (r *CreateCategoryRequest) ToCommand(userID string) (*commands.CreateCategoryCommand, error) {
	cmd, err := commands.NewCreateCategoryCommand(userID, r.Title, r.Description)
	if err != nil {
		return nil, err
	}
	
	if r.Color != nil {
		cmd.WithColor(*r.Color)
	}
	
	if r.Icon != nil {
		cmd.WithIcon(*r.Icon)
	}
	
	if r.ParentID != nil {
		cmd.WithParentID(*r.ParentID)
	}
	
	return cmd, nil
}

// UpdateCategoryRequest supports partial updates for categories
type UpdateCategoryRequest struct {
	Title       *string `json:"title,omitempty" validate:"omitempty,min=1,max=200"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=1000"`
	Color       *string `json:"color,omitempty" validate:"omitempty,hexcolor"`
	Icon        *string `json:"icon,omitempty" validate:"omitempty,max=50"`
}

// Validate performs custom validation
func (r *UpdateCategoryRequest) Validate() error {
	var errors ValidationErrors

	if r.Title != nil {
		if err := validateTitle(*r.Title); err != nil {
			errors.Errors = append(errors.Errors, ValidationError{
				Field:   "title",
				Message: err.Error(),
				Code:    "INVALID_TITLE",
			})
		}
	}

	if r.Description != nil {
		if err := validateDescription(*r.Description); err != nil {
			errors.Errors = append(errors.Errors, ValidationError{
				Field:   "description",
				Message: err.Error(),
				Code:    "INVALID_DESCRIPTION",
			})
		}
	}

	if len(errors.Errors) > 0 {
		return errors
	}
	return nil
}

// HasChanges checks if the request contains any updates
func (r *UpdateCategoryRequest) HasChanges() bool {
	return r.Title != nil || r.Description != nil || r.Color != nil || r.Icon != nil
}

// Sanitize cleans the input data
func (r *UpdateCategoryRequest) Sanitize() {
	if r.Title != nil {
		sanitized := sanitizeTitle(*r.Title)
		r.Title = &sanitized
	}
	if r.Description != nil {
		sanitized := sanitizeDescription(*r.Description)
		r.Description = &sanitized
	}
}

// ToCommand converts the request to an application command
func (r *UpdateCategoryRequest) ToCommand(userID, categoryID string) (*commands.UpdateCategoryCommand, error) {
	cmd, err := commands.NewUpdateCategoryCommand(userID, categoryID)
	if err != nil {
		return nil, err
	}

	if r.Title != nil {
		cmd.WithTitle(*r.Title)
	}

	if r.Description != nil {
		cmd.WithDescription(*r.Description)
	}

	if r.Color != nil {
		cmd.WithColor(*r.Color)
	}

	if r.Icon != nil {
		cmd.WithIcon(*r.Icon)
	}

	return cmd, nil
}

// AssignNodeToCategoryRequest represents assigning a node to a category
type AssignNodeToCategoryRequest struct {
	NodeID string `json:"node_id" validate:"required"`
}

// Validate performs custom validation
func (r *AssignNodeToCategoryRequest) Validate() error {
	if !validIDPattern.MatchString(r.NodeID) {
		return ValidationErrors{
			Errors: []ValidationError{{
				Field:   "node_id",
				Message: "invalid node ID format",
				Code:    "INVALID_ID",
			}},
		}
	}
	return nil
}

// ToCommand converts the request to an application command
func (r *AssignNodeToCategoryRequest) ToCommand(userID, categoryID string) (*commands.AssignNodeToCategoryCommand, error) {
	return commands.NewAssignNodeToCategoryCommand(userID, categoryID, r.NodeID)
}

// ConnectNodesRequest represents creating a connection between nodes
type ConnectNodesRequest struct {
	SourceNodeID string  `json:"source_node_id" validate:"required"`
	TargetNodeID string  `json:"target_node_id" validate:"required"`
	Weight       float64 `json:"weight,omitempty" validate:"omitempty,min=0,max=1"`
}

// Validate performs custom validation
func (r *ConnectNodesRequest) Validate() error {
	var errors ValidationErrors

	if !validIDPattern.MatchString(r.SourceNodeID) {
		errors.Errors = append(errors.Errors, ValidationError{
			Field:   "source_node_id",
			Message: "invalid source node ID format",
			Code:    "INVALID_SOURCE_ID",
		})
	}

	if !validIDPattern.MatchString(r.TargetNodeID) {
		errors.Errors = append(errors.Errors, ValidationError{
			Field:   "target_node_id",
			Message: "invalid target node ID format",
			Code:    "INVALID_TARGET_ID",
		})
	}

	if r.SourceNodeID == r.TargetNodeID {
		errors.Errors = append(errors.Errors, ValidationError{
			Field:   "target_node_id",
			Message: "cannot connect a node to itself",
			Code:    "SELF_CONNECTION",
		})
	}

	if len(errors.Errors) > 0 {
		return errors
	}
	return nil
}

// ToCommand converts the request to an application command
func (r *ConnectNodesRequest) ToCommand(userID string) (*commands.ConnectNodesCommand, error) {
	weight := r.Weight
	if weight == 0 {
		weight = 1.0 // Default weight
	}

	return &commands.ConnectNodesCommand{
		UserID:       userID,
		SourceNodeID: r.SourceNodeID,
		TargetNodeID: r.TargetNodeID,
		Weight:       weight,
	}, nil
}

// Validation helper functions

func validateContent(content string) error {
	if len(content) < MinContentLength {
		return fmt.Errorf("content must be at least %d characters", MinContentLength)
	}
	if len(content) > MaxContentLength {
		return fmt.Errorf("content cannot exceed %d characters", MaxContentLength)
	}
	if !utf8.ValidString(content) {
		return fmt.Errorf("content contains invalid UTF-8 characters")
	}
	if sqlInjectionCheck.MatchString(content) {
		return fmt.Errorf("content contains potentially malicious patterns")
	}
	return nil
}

func validateTags(tags []string) error {
	if len(tags) > MaxTagCount {
		return fmt.Errorf("cannot have more than %d tags", MaxTagCount)
	}

	seen := make(map[string]bool)
	for _, tag := range tags {
		if len(tag) < MinTagLength || len(tag) > MaxTagLength {
			return fmt.Errorf("tag '%s' must be between %d and %d characters", tag, MinTagLength, MaxTagLength)
		}
		if !validTagPattern.MatchString(tag) {
			return fmt.Errorf("tag '%s' contains invalid characters", tag)
		}

		normalized := strings.ToLower(strings.TrimSpace(tag))
		if seen[normalized] {
			return fmt.Errorf("duplicate tag: %s", tag)
		}
		seen[normalized] = true
	}
	return nil
}

func validateTitle(title string) error {
	if len(title) < MinTitleLength {
		return fmt.Errorf("title must be at least %d characters", MinTitleLength)
	}
	if len(title) > MaxTitleLength {
		return fmt.Errorf("title cannot exceed %d characters", MaxTitleLength)
	}
	if !utf8.ValidString(title) {
		return fmt.Errorf("title contains invalid UTF-8 characters")
	}
	return nil
}

func validateDescription(desc string) error {
	if len(desc) > MaxDescLength {
		return fmt.Errorf("description cannot exceed %d characters", MaxDescLength)
	}
	if !utf8.ValidString(desc) {
		return fmt.Errorf("description contains invalid UTF-8 characters")
	}
	return nil
}

// Sanitization helper functions

func sanitizeContent(content string) string {
	// Remove null bytes and control characters
	content = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return r
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, content)

	return strings.TrimSpace(content)
}

func sanitizeTags(tags []string) []string {
	seen := make(map[string]bool)
	var sanitized []string

	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			sanitized = append(sanitized, normalized)
		}
	}

	return sanitized
}

func sanitizeTitle(title string) string {
	return strings.TrimSpace(title)
}

func sanitizeDescription(desc string) string {
	return strings.TrimSpace(desc)
}