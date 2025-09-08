package common

import (
	"net/http"
	"strconv"
)

// PaginationParams represents pagination parameters
type PaginationParams struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Sort     string `json:"sort,omitempty"`
	Order    string `json:"order,omitempty"`
}

// DefaultPaginationParams returns default pagination parameters
func DefaultPaginationParams() PaginationParams {
	return PaginationParams{
		Page:     1,
		PageSize: 20,
		Order:    "desc",
	}
}

// ExtractPaginationParams extracts pagination parameters from request
func ExtractPaginationParams(r *http.Request) PaginationParams {
	params := DefaultPaginationParams()
	
	// Extract page
	if page := r.URL.Query().Get("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 0 {
			params.Page = p
		}
	}
	
	// Extract page size
	if pageSize := r.URL.Query().Get("page_size"); pageSize != "" {
		if ps, err := strconv.Atoi(pageSize); err == nil && ps > 0 {
			if ps > 100 {
				ps = 100 // Max page size
			}
			params.PageSize = ps
		}
	}
	
	// Extract sort field
	if sort := r.URL.Query().Get("sort"); sort != "" {
		params.Sort = sort
	}
	
	// Extract order
	if order := r.URL.Query().Get("order"); order != "" {
		if order == "asc" || order == "desc" {
			params.Order = order
		}
	}
	
	return params
}

// CalculateOffset calculates the offset for database queries
func (p PaginationParams) CalculateOffset() int {
	return (p.Page - 1) * p.PageSize
}

// CalculateTotalPages calculates total number of pages
func CalculateTotalPages(total, pageSize int) int {
	if pageSize <= 0 {
		return 0
	}
	pages := total / pageSize
	if total%pageSize > 0 {
		pages++
	}
	return pages
}

// BuildPaginationMeta builds pagination metadata
func BuildPaginationMeta(page, pageSize, total int) *PaginationInfo {
	totalPages := CalculateTotalPages(total, pageSize)
	
	return &PaginationInfo{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// PaginatedResult represents a paginated result
type PaginatedResult struct {
	Items      interface{}     `json:"items"`
	Pagination *PaginationInfo `json:"pagination"`
}

// NewPaginatedResult creates a new paginated result
func NewPaginatedResult(items interface{}, page, pageSize, total int) *PaginatedResult {
	return &PaginatedResult{
		Items:      items,
		Pagination: BuildPaginationMeta(page, pageSize, total),
	}
}