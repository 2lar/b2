package domain

import (
	"context"
	"time"
)

// CategoryID represents the unique identifier for a Category.
type CategoryID string

// Category represents a classification for a piece of memory or a node.
type Category struct {
	ID          CategoryID  `json:"id"`
	UserID      string      `json:"user_id"`
	Name        string      `json:"name"`
	Title       string      `json:"title"`         // Alternative name field for compatibility
	ParentID    *CategoryID `json:"parent_id,omitempty"`
	Level       int         `json:"level"`         // Hierarchy level (0 = root)
	NoteCount   int         `json:"note_count"`    // Number of notes in this category
	AIGenerated bool        `json:"ai_generated"`  // Whether this category was AI-generated
	Color       *string     `json:"color,omitempty"`      // Category color for UI
	Icon        *string     `json:"icon,omitempty"`       // Category icon for UI
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Description string      `json:"description,omitempty"`
}

// CategoryRepository defines the persistence methods required for a Category.
// This interface is part of the domain layer and dictates the contract for
// how category data is accessed, abstracting away the specific database implementation.
type CategoryRepository interface {
	// FindByID retrieves a single category by its unique ID for a given user.
	FindByID(ctx context.Context, userID string, id CategoryID) (*Category, error)

	// ListByParentID retrieves all direct children of a given category for a user.
	ListByParentID(ctx context.Context, userID string, parentID CategoryID) ([]*Category, error)

	// ListRoot retrieves all categories that do not have a parent for a user.
	ListRoot(ctx context.Context, userID string) ([]*Category, error)

	// Save persists a Category. It handles both creation and updates.
	Save(ctx context.Context, category *Category) error

	// Delete removes a category by its ID for a given user.
	Delete(ctx context.Context, userID string, id CategoryID) error
}
