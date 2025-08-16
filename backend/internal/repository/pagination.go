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
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
	Cursor        string `json:"cursor,omitempty"`
	SortBy        string `json:"sort_by,omitempty"`
	SortOrder     string `json:"sort_order,omitempty"` // "asc" or "desc"
	SortDirection string `json:"sort_direction,omitempty"` // Alias for SortOrder for compatibility
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
	Items      []T      `json:"items"`
	TotalCount int      `json:"total_count,omitempty"`
	HasMore    bool     `json:"has_more"`
	NextCursor string   `json:"next_cursor,omitempty"`
	PageInfo   PageInfo `json:"page_info"`
}

// PageInfo contains pagination metadata
type PageInfo struct {
	CurrentPage int `json:"current_page"`
	PageSize    int `json:"page_size"`
	TotalPages  int `json:"total_pages,omitempty"`
	ItemsInPage int `json:"items_in_page"`
}

// NodePage represents a paginated list of nodes
type NodePage = PaginatedResult[*domain.Node]

// EdgePage represents a paginated list of edges
type EdgePage = PaginatedResult[*domain.Edge]

// CategoryPage represents a paginated list of categories
type CategoryPage = PaginatedResult[domain.Category]

// CursorData represents the data stored in a pagination cursor
type CursorData struct {
	LastEvaluatedKey map[string]types.AttributeValue `json:"last_evaluated_key"`
	Timestamp        int64                           `json:"timestamp"`
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

// PageRequest represents pagination parameters for queries - Enhanced for optimization
type PageRequest struct {
	Limit     int    `json:"limit"`
	NextToken string `json:"nextToken,omitempty"`
	SortBy    string `json:"sortBy,omitempty"`
	SortOrder string `json:"sortOrder,omitempty"` // "asc" or "desc"
}

// PageResponse represents a paginated response - Enhanced for optimization
type PageResponse struct {
	Items     interface{} `json:"items"`
	NextToken string      `json:"nextToken,omitempty"`
	HasMore   bool        `json:"hasMore"`
	Total     int         `json:"total,omitempty"`
}

// LastEvaluatedKey represents DynamoDB's last evaluated key for pagination with GSI support
type LastEvaluatedKey struct {
	PK      string `json:"pk"`
	SK      string `json:"sk"`
	GSI1PK  string `json:"gsi1pk,omitempty"`
	GSI1SK  string `json:"gsi1sk,omitempty"`
	GSI2PK  string `json:"gsi2pk,omitempty"`
	GSI2SK  string `json:"gsi2sk,omitempty"`
}

// Constants for pagination
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// NewPageRequest creates a new PageRequest with default values
func NewPageRequest(limit int, nextToken string) PageRequest {
	if limit <= 0 || limit > MaxPageSize {
		limit = DefaultPageSize
	}
	return PageRequest{
		Limit:     limit,
		NextToken: nextToken,
	}
}

// GetEffectiveLimit returns the effective limit for PageRequest, ensuring it's within bounds
func (pr PageRequest) GetEffectiveLimit() int {
	if pr.Limit <= 0 || pr.Limit > MaxPageSize {
		return DefaultPageSize
	}
	return pr.Limit
}

// HasNextToken returns true if the request has a pagination token
func (pr PageRequest) HasNextToken() bool {
	return pr.NextToken != ""
}

// EncodeNextToken encodes a LastEvaluatedKey as a base64 token
func EncodeNextToken(key LastEvaluatedKey) string {
	data, err := json.Marshal(key)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeNextToken decodes a base64 token back to LastEvaluatedKey
func DecodeNextToken(token string) (*LastEvaluatedKey, error) {
	if token == "" {
		return nil, nil
	}

	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}

	var key LastEvaluatedKey
	err = json.Unmarshal(data, &key)
	if err != nil {
		return nil, err
	}

	return &key, nil
}

// ToDynamoDBKey converts LastEvaluatedKey to DynamoDB exclusive start key format
func (lek LastEvaluatedKey) ToDynamoDBKey() map[string]types.AttributeValue {
	key := make(map[string]types.AttributeValue)
	
	if lek.PK != "" {
		key["PK"] = &types.AttributeValueMemberS{Value: lek.PK}
	}
	if lek.SK != "" {
		key["SK"] = &types.AttributeValueMemberS{Value: lek.SK}
	}
	if lek.GSI1PK != "" {
		key["GSI1PK"] = &types.AttributeValueMemberS{Value: lek.GSI1PK}
	}
	if lek.GSI1SK != "" {
		key["GSI1SK"] = &types.AttributeValueMemberS{Value: lek.GSI1SK}
	}
	if lek.GSI2PK != "" {
		key["GSI2PK"] = &types.AttributeValueMemberS{Value: lek.GSI2PK}
	}
	if lek.GSI2SK != "" {
		key["GSI2SK"] = &types.AttributeValueMemberS{Value: lek.GSI2SK}
	}

	return key
}

// FromDynamoDBKey creates LastEvaluatedKey from DynamoDB last evaluated key
func FromDynamoDBKey(key map[string]types.AttributeValue) LastEvaluatedKey {
	lek := LastEvaluatedKey{}
	
	if pk, ok := key["PK"].(*types.AttributeValueMemberS); ok {
		lek.PK = pk.Value
	}
	if sk, ok := key["SK"].(*types.AttributeValueMemberS); ok {
		lek.SK = sk.Value
	}
	if gsi1pk, ok := key["GSI1PK"].(*types.AttributeValueMemberS); ok {
		lek.GSI1PK = gsi1pk.Value
	}
	if gsi1sk, ok := key["GSI1SK"].(*types.AttributeValueMemberS); ok {
		lek.GSI1SK = gsi1sk.Value
	}
	if gsi2pk, ok := key["GSI2PK"].(*types.AttributeValueMemberS); ok {
		lek.GSI2PK = gsi2pk.Value
	}
	if gsi2sk, ok := key["GSI2SK"].(*types.AttributeValueMemberS); ok {
		lek.GSI2SK = gsi2sk.Value
	}

	return lek
}

// CreatePageResponse creates a paginated response with the given items and pagination info
func CreatePageResponse(items interface{}, lastKey map[string]types.AttributeValue, hasMore bool) *PageResponse {
	response := &PageResponse{
		Items:   items,
		HasMore: hasMore,
	}

	// Encode next token if there are more results
	if hasMore && lastKey != nil {
		lek := FromDynamoDBKey(lastKey)
		response.NextToken = EncodeNextToken(lek)
	}

	return response
}
