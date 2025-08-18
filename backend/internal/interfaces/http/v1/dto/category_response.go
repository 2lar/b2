// Package dto provides data transfer objects for HTTP API responses.
package dto

import (
	"time"
)

// Placeholder view types until proper CQRS query views are implemented
type CategoryView struct {
	ID          string
	Title       string
	Description string
	Color       string
	Level       int
	NoteCount   int
	ParentID    *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CategoryResponse represents a category in HTTP API responses.
// This DTO is designed to be the single source of truth for category representation
// across all API endpoints, eliminating duplication and ensuring consistency.
type CategoryResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Level       int     `json:"level"`
	ParentID    *string `json:"parentId"`
	Color       *string `json:"color"`
	Icon        *string `json:"icon"`
	AIGenerated bool    `json:"aiGenerated"`
	NoteCount   int     `json:"noteCount"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

// CategoryListResponse represents the response for list categories endpoint.
type CategoryListResponse struct {
	Categories []CategoryResponse `json:"categories"`
}

// CategoryConverter handles conversion between domain/query models and DTOs.
// This converter implements the Single Responsibility Principle by focusing
// solely on data transformation logic.
type CategoryConverter struct{}

// NewCategoryConverter creates a new category converter.
func NewCategoryConverter() *CategoryConverter {
	return &CategoryConverter{}
}

// FromCategoryView converts a CategoryView to CategoryResponse.
// This method handles the mapping between CQRS query results and HTTP DTOs.
func (c *CategoryConverter) FromCategoryView(view CategoryView) CategoryResponse {
	var color *string
	if view.Color != "" {
		color = &view.Color
	}
	
	return CategoryResponse{
		ID:          view.ID,
		Title:       view.Title,
		Description: view.Description,
		Level:       0,         // CategoryView doesn't have Level field
		ParentID:    nil,       // CategoryView doesn't have ParentID field  
		Color:       color,
		Icon:        nil,       // CategoryView doesn't have Icon field
		AIGenerated: false,     // CategoryView doesn't have AIGenerated field
		NoteCount:   view.NoteCount,
		CreatedAt:   view.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   view.UpdatedAt.Format(time.RFC3339),
	}
}

// FromCategoryViews converts a slice of CategoryView to CategoryListResponse.
func (c *CategoryConverter) FromCategoryViews(views []CategoryView) CategoryListResponse {
	categories := make([]CategoryResponse, 0, len(views))
	for _, view := range views {
		categories = append(categories, c.FromCategoryView(view))
	}
	
	return CategoryListResponse{
		Categories: categories,
	}
}