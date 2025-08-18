package category

import (
	"context"
	"time"

	"brain2-backend/internal/domain/shared"
)

// Category represents a classification for a piece of memory or a node.
type Category struct {
	ID          shared.CategoryID  `json:"id"`
	UserID      string      `json:"user_id"`
	Name        string      `json:"name"`
	Title       string      `json:"title"`         // Alternative name field for compatibility
	ParentID    *shared.CategoryID `json:"parent_id,omitempty"`
	Level       int         `json:"level"`         // Hierarchy level (0 = root)
	NoteCount   int         `json:"note_count"`    // Number of notes in this category
	AIGenerated bool        `json:"ai_generated"`  // Whether this category was AI-generated
	Color       *string     `json:"color,omitempty"`      // Category color for UI
	Icon        *string     `json:"icon,omitempty"`       // Category icon for UI
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Description string      `json:"description,omitempty"`
	
	// Domain events
	events []shared.DomainEvent
}

// NewCategory creates a new category with validation.
// This factory method ensures categories are created in a valid state.
func NewCategory(userID shared.UserID, title, description string) (*Category, error) {
	if title == "" {
		return nil, shared.ErrValidation
	}
	
	now := time.Now()
	categoryID := shared.CategoryID(shared.NewNodeID().String()) // Generate unique ID
	
	category := &Category{
		ID:          categoryID,
		UserID:      userID.String(),
		Name:        title,
		Title:       title,
		Description: description,
		Level:       0,
		NoteCount:   0,
		AIGenerated: false,
		CreatedAt:   now,
		UpdatedAt:   now,
		events:      []shared.DomainEvent{},
	}
	
	// Generate domain event for category creation
	createdEvent := shared.NewCategoryCreatedEvent(categoryID, userID, title, description, 0)
	category.addEvent(createdEvent)
	
	return category, nil
}

// UpdateTitle updates the category title
func (c *Category) UpdateTitle(title string) error {
	if title == "" {
		return shared.ErrValidation
	}
	c.Title = title
	c.Name = title
	c.UpdatedAt = time.Now()
	return nil
}

// UpdateDescription updates the category description
func (c *Category) UpdateDescription(description string) error {
	c.Description = description
	c.UpdatedAt = time.Now()
	return nil
}

// SetColor sets the category color
func (c *Category) SetColor(color string) error {
	c.Color = &color
	c.UpdatedAt = time.Now()
	return nil
}

// Domain Events Implementation (EventAggregate interface)

// GetUncommittedEvents returns events that haven't been persisted yet
func (c *Category) GetUncommittedEvents() []shared.DomainEvent {
	return c.events
}

// MarkEventsAsCommitted clears the events after persistence
func (c *Category) MarkEventsAsCommitted() {
	c.events = []shared.DomainEvent{}
}

// addEvent adds a domain event to the uncommitted events list
func (c *Category) addEvent(event shared.DomainEvent) {
	c.events = append(c.events, event)
}

// CategoryRepository defines the persistence methods required for a Category.
// This interface is part of the domain layer and dictates the contract for
// how category data is accessed, abstracting away the specific database implementation.
type CategoryRepository interface {
	// FindByID retrieves a single category by its unique ID for a given user.
	FindByID(ctx context.Context, userID string, id shared.CategoryID) (*Category, error)

	// ListByParentID retrieves all direct children of a given category for a user.
	ListByParentID(ctx context.Context, userID string, parentID shared.CategoryID) ([]*Category, error)

	// ListRoot retrieves all categories that do not have a parent for a user.
	ListRoot(ctx context.Context, userID string) ([]*Category, error)

	// Save persists a Category. It handles both creation and updates.
	Save(ctx context.Context, category *Category) error

	// Delete removes a category by its ID for a given user.
	Delete(ctx context.Context, userID string, id shared.CategoryID) error
}
