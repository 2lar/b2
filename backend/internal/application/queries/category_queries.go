// Package queries contains query objects for category read operations.
package queries

import (
	"errors"
	"strings"
	"time"
)

// GetCategoryQuery represents a request to retrieve a single category.
type GetCategoryQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	CategoryID  string    `json:"category_id" validate:"required"`
	RequestedAt time.Time `json:"requested_at"`
	
	// Optional flags for controlling what data to include
	IncludeNodes bool `json:"include_nodes"`
	IncludeStats bool `json:"include_stats"`
}

// NewGetCategoryQuery creates a new GetCategoryQuery with validation.
func NewGetCategoryQuery(userID, categoryID string) (*GetCategoryQuery, error) {
	query := &GetCategoryQuery{
		UserID:       userID,
		CategoryID:   categoryID,
		RequestedAt:  time.Now(),
		IncludeNodes: false,
		IncludeStats: false,
	}
	
	if err := query.Validate(); err != nil {
		return nil, err
	}
	
	return query, nil
}

// WithNodes includes nodes in the category in the query result.
func (q *GetCategoryQuery) WithNodes() *GetCategoryQuery {
	q.IncludeNodes = true
	return q
}

// WithStats includes statistics in the query result.
func (q *GetCategoryQuery) WithStats() *GetCategoryQuery {
	q.IncludeStats = true
	return q
}

// Validate performs validation on the query parameters.
func (q *GetCategoryQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(q.CategoryID) == "" {
		return errors.New("category_id is required")
	}
	
	return nil
}

// ListCategoriesQuery represents a request to retrieve a list of categories.
type ListCategoriesQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	RequestedAt time.Time `json:"requested_at"`
	
	// Pagination parameters
	Limit     int    `json:"limit" validate:"min=1,max=100"`
	NextToken string `json:"next_token,omitempty"`
	
	// Filtering options
	SearchQuery string `json:"search_query,omitempty"`
	
	// Sorting options
	SortBy        string `json:"sort_by" validate:"omitempty,oneof=title created_at updated_at node_count"`
	SortDirection string `json:"sort_direction" validate:"omitempty,oneof=asc desc"`
	
	// Include options
	IncludeNodeCounts bool `json:"include_node_counts"`
}

// NewListCategoriesQuery creates a new ListCategoriesQuery with validation and defaults.
func NewListCategoriesQuery(userID string) (*ListCategoriesQuery, error) {
	query := &ListCategoriesQuery{
		UserID:            userID,
		RequestedAt:       time.Now(),
		Limit:             20, // Default limit
		SortBy:            "title",
		SortDirection:     "asc",
		IncludeNodeCounts: false,
	}
	
	if err := query.Validate(); err != nil {
		return nil, err
	}
	
	return query, nil
}

// WithPagination sets pagination parameters.
func (q *ListCategoriesQuery) WithPagination(limit int, nextToken string) *ListCategoriesQuery {
	q.Limit = limit
	q.NextToken = nextToken
	return q
}

// WithSearch adds a search query to filter results.
func (q *ListCategoriesQuery) WithSearch(searchQuery string) *ListCategoriesQuery {
	q.SearchQuery = searchQuery
	return q
}

// WithSort sets the sorting parameters.
func (q *ListCategoriesQuery) WithSort(sortBy, direction string) *ListCategoriesQuery {
	q.SortBy = sortBy
	q.SortDirection = direction
	return q
}

// WithNodeCounts includes node counts in the results.
func (q *ListCategoriesQuery) WithNodeCounts() *ListCategoriesQuery {
	q.IncludeNodeCounts = true
	return q
}

// Validate performs validation on the query parameters.
func (q *ListCategoriesQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if q.Limit < 1 || q.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	
	if q.SortBy != "" {
		validSortFields := map[string]bool{
			"title":      true,
			"created_at": true,
			"updated_at": true,
			"node_count": true,
		}
		if !validSortFields[q.SortBy] {
			return errors.New("invalid sort_by field")
		}
	}
	
	if q.SortDirection != "" && q.SortDirection != "asc" && q.SortDirection != "desc" {
		return errors.New("sort_direction must be 'asc' or 'desc'")
	}
	
	return nil
}

// GetNodesInCategoryQuery represents a request to retrieve nodes in a specific category.
type GetNodesInCategoryQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	CategoryID  string    `json:"category_id" validate:"required"`
	RequestedAt time.Time `json:"requested_at"`
	
	// Pagination parameters
	Limit     int    `json:"limit" validate:"min=1,max=100"`
	NextToken string `json:"next_token,omitempty"`
	
	// Sorting options
	SortBy        string `json:"sort_by" validate:"omitempty,oneof=created_at updated_at content"`
	SortDirection string `json:"sort_direction" validate:"omitempty,oneof=asc desc"`
}

// NewGetNodesInCategoryQuery creates a new GetNodesInCategoryQuery with validation.
func NewGetNodesInCategoryQuery(userID, categoryID string) (*GetNodesInCategoryQuery, error) {
	query := &GetNodesInCategoryQuery{
		UserID:        userID,
		CategoryID:    categoryID,
		RequestedAt:   time.Now(),
		Limit:         20, // Default limit
		SortBy:        "updated_at",
		SortDirection: "desc",
	}
	
	if err := query.Validate(); err != nil {
		return nil, err
	}
	
	return query, nil
}

// WithPagination sets pagination parameters.
func (q *GetNodesInCategoryQuery) WithPagination(limit int, nextToken string) *GetNodesInCategoryQuery {
	q.Limit = limit
	q.NextToken = nextToken
	return q
}

// WithSort sets the sorting parameters.
func (q *GetNodesInCategoryQuery) WithSort(sortBy, direction string) *GetNodesInCategoryQuery {
	q.SortBy = sortBy
	q.SortDirection = direction
	return q
}

// Validate performs validation on the query parameters.
func (q *GetNodesInCategoryQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(q.CategoryID) == "" {
		return errors.New("category_id is required")
	}
	
	if q.Limit < 1 || q.Limit > 100 {
		return errors.New("limit must be between 1 and 100")
	}
	
	if q.SortBy != "" {
		validSortFields := map[string]bool{
			"created_at": true,
			"updated_at": true,
			"content":    true,
		}
		if !validSortFields[q.SortBy] {
			return errors.New("invalid sort_by field")
		}
	}
	
	if q.SortDirection != "" && q.SortDirection != "asc" && q.SortDirection != "desc" {
		return errors.New("sort_direction must be 'asc' or 'desc'")
	}
	
	return nil
}

// GetCategoriesForNodeQuery represents a request to retrieve categories for a specific node.
type GetCategoriesForNodeQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	NodeID      string    `json:"node_id" validate:"required"`
	RequestedAt time.Time `json:"requested_at"`
}

// NewGetCategoriesForNodeQuery creates a new GetCategoriesForNodeQuery with validation.
func NewGetCategoriesForNodeQuery(userID, nodeID string) (*GetCategoriesForNodeQuery, error) {
	query := &GetCategoriesForNodeQuery{
		UserID:      userID,
		NodeID:      nodeID,
		RequestedAt: time.Now(),
	}
	
	if err := query.Validate(); err != nil {
		return nil, err
	}
	
	return query, nil
}

// Validate performs validation on the query parameters.
func (q *GetCategoriesForNodeQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(q.NodeID) == "" {
		return errors.New("node_id is required")
	}
	
	return nil
}

// SuggestCategoriesQuery represents a request to get AI-powered category suggestions.
type SuggestCategoriesQuery struct {
	UserID      string    `json:"user_id" validate:"required"`
	Content     string    `json:"content" validate:"required,min=1,max=10000"`
	RequestedAt time.Time `json:"requested_at"`
	
	// Suggestion parameters
	MaxSuggestions int     `json:"max_suggestions" validate:"min=1,max=10"`
	MinConfidence  float64 `json:"min_confidence" validate:"min=0,max=1"`
	
	// Context for better suggestions
	ExistingCategories []string `json:"existing_categories,omitempty"`
}

// NewSuggestCategoriesQuery creates a new SuggestCategoriesQuery with validation.
func NewSuggestCategoriesQuery(userID, content string) (*SuggestCategoriesQuery, error) {
	query := &SuggestCategoriesQuery{
		UserID:         userID,
		Content:        content,
		RequestedAt:    time.Now(),
		MaxSuggestions: 5,     // Default
		MinConfidence:  0.7,   // Default minimum confidence
	}
	
	if err := query.Validate(); err != nil {
		return nil, err
	}
	
	return query, nil
}

// WithSuggestionParams sets the suggestion parameters.
func (q *SuggestCategoriesQuery) WithSuggestionParams(maxSuggestions int, minConfidence float64) *SuggestCategoriesQuery {
	q.MaxSuggestions = maxSuggestions
	q.MinConfidence = minConfidence
	return q
}

// WithExistingCategories provides context of existing categories for better suggestions.
func (q *SuggestCategoriesQuery) WithExistingCategories(categories []string) *SuggestCategoriesQuery {
	q.ExistingCategories = categories
	return q
}

// Validate performs validation on the query parameters.
func (q *SuggestCategoriesQuery) Validate() error {
	if strings.TrimSpace(q.UserID) == "" {
		return errors.New("user_id is required")
	}
	
	if strings.TrimSpace(q.Content) == "" {
		return errors.New("content is required")
	}
	
	if len(q.Content) > 10000 {
		return errors.New("content exceeds maximum length of 10,000 characters")
	}
	
	if q.MaxSuggestions < 1 || q.MaxSuggestions > 10 {
		return errors.New("max_suggestions must be between 1 and 10")
	}
	
	if q.MinConfidence < 0 || q.MinConfidence > 1 {
		return errors.New("min_confidence must be between 0 and 1")
	}
	
	return nil
}