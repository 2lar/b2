package repository

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"brain2-backend/internal/domain"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Pagination represents pagination parameters with cursor-based pagination
type Pagination struct {
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
	Cursor     string `json:"cursor,omitempty"`
	SortBy     string `json:"sort_by,omitempty"`
	SortOrder  string `json:"sort_order,omitempty"` // "asc" or "desc"
}

// Validate checks if pagination parameters are valid
func (p Pagination) Validate() error {
	if p.Limit < 0 {
		return NewInvalidQuery("Limit", "cannot be negative")
	}
	if p.Offset < 0 {
		return NewInvalidQuery("Offset", "cannot be negative")
	}
	if p.Limit > 1000 {
		return NewInvalidQuery("Limit", "cannot exceed 1000")
	}
	return nil
}

// GetEffectiveLimit returns the limit to use, with a default if not specified
func (p Pagination) GetEffectiveLimit() int {
	if p.Limit <= 0 {
		return 50 // Default page size
	}
	if p.Limit > 1000 {
		return 1000 // Maximum page size
	}
	return p.Limit
}

// HasCursor returns true if cursor-based pagination is being used
func (p Pagination) HasCursor() bool {
	return p.Cursor != ""
}

// PaginatedResult represents a paginated response with metadata
type PaginatedResult[T any] struct {
	Items      []T    `json:"items"`
	TotalCount int    `json:"total_count,omitempty"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
	PageInfo   PageInfo `json:"page_info"`
}

// PageInfo contains pagination metadata
type PageInfo struct {
	CurrentPage  int `json:"current_page"`
	PageSize     int `json:"page_size"`
	TotalPages   int `json:"total_pages,omitempty"`
	ItemsInPage  int `json:"items_in_page"`
}

// NodePage represents a paginated list of nodes
type NodePage = PaginatedResult[domain.Node]

// EdgePage represents a paginated list of edges  
type EdgePage = PaginatedResult[domain.Edge]

// CategoryPage represents a paginated list of categories
type CategoryPage = PaginatedResult[domain.Category]

// CursorData represents the data stored in a pagination cursor
type CursorData struct {
	LastEvaluatedKey map[string]types.AttributeValue `json:"last_evaluated_key"`
	Timestamp        int64                            `json:"timestamp"`
}

// EncodeCursor creates a base64 encoded cursor from DynamoDB's LastEvaluatedKey
func EncodeCursor(lastEvaluatedKey map[string]types.AttributeValue) string {
	if lastEvaluatedKey == nil {
		return ""
	}

	cursorData := CursorData{
		LastEvaluatedKey: lastEvaluatedKey,
		Timestamp:        time.Now().Unix(),
	}

	jsonData, err := json.Marshal(cursorData)
	if err != nil {
		return ""
	}

	return base64.URLEncoding.EncodeToString(jsonData)
}

// DecodeCursor decodes a base64 cursor back to DynamoDB's LastEvaluatedKey format
func DecodeCursor(cursor string) (map[string]types.AttributeValue, error) {
	if cursor == "" {
		return nil, nil
	}

	jsonData, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	var cursorData CursorData
	if err := json.Unmarshal(jsonData, &cursorData); err != nil {
		return nil, fmt.Errorf("invalid cursor data: %w", err)
	}

	return cursorData.LastEvaluatedKey, nil
}

// CreatePageInfo creates pagination metadata for a result set
func CreatePageInfo(pagination Pagination, itemCount int, hasMore bool) PageInfo {
	pageSize := pagination.GetEffectiveLimit()
	currentPage := (pagination.Offset / pageSize) + 1
	
	return PageInfo{
		CurrentPage: currentPage,
		PageSize:    pageSize,
		ItemsInPage: itemCount,
	}
}