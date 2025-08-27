package category

import (
	"context"
	"time"

	"brain2-backend/internal/domain/shared"
)

// Category represents a classification for a piece of memory or a node.
// This is a rich domain model that encapsulates business logic for categorization.
//
// Key Design Principles:
//   - Rich Domain Model: Contains behavior and validation logic
//   - Aggregate Root: Extends BaseAggregateRoot for consistency
//   - Domain Events: Tracks category creation, updates, and deletion
//   - Business Invariants: Ensures categories are always in a valid state
type Category struct {
	// Embedded base aggregate root for common functionality
	shared.BaseAggregateRoot
	
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
	Version     shared.Version `json:"-"`  // For optimistic locking
	
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
		BaseAggregateRoot: shared.NewBaseAggregateRoot(string(categoryID)),
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
		Version:     shared.NewVersion(),
		events:      []shared.DomainEvent{},
	}
	
	// Generate domain event for category creation
	createdEvent := shared.NewCategoryCreatedEvent(categoryID, userID, title, description, 0)
	category.addEvent(createdEvent)
	
	return category, nil
}

// UpdateTitle updates the category title with event generation
func (c *Category) UpdateTitle(title string) error {
	if title == "" {
		return shared.ErrValidation
	}
	
	// Check if title actually changed
	if c.Title == title {
		return nil // No change needed
	}
	
	c.Title = title
	c.Name = title
	c.UpdatedAt = time.Now()
	c.Version = c.Version.Next()
	
	// Generate domain event for category update
	updateEvent := NewCategoryUpdatedEvent(c)
	c.addEvent(updateEvent)
	
	return nil
}

// UpdateDescription updates the category description with event generation
func (c *Category) UpdateDescription(description string) error {
	// Check if description actually changed
	if c.Description == description {
		return nil // No change needed
	}
	
	c.Description = description
	c.UpdatedAt = time.Now()
	c.Version = c.Version.Next()
	
	// Generate domain event for category update
	updateEvent := NewCategoryUpdatedEvent(c)
	c.addEvent(updateEvent)
	
	return nil
}

// SetColor sets the category color with event generation
func (c *Category) SetColor(color string) error {
	// Check if color actually changed
	if c.Color != nil && *c.Color == color {
		return nil // No change needed
	}
	
	c.Color = &color
	c.UpdatedAt = time.Now()
	c.Version = c.Version.Next()
	
	// Generate domain event for category update
	updateEvent := NewCategoryUpdatedEvent(c)
	c.addEvent(updateEvent)
	
	return nil
}

// ValidateInvariants ensures all business rules are satisfied
func (c *Category) ValidateInvariants() error {
	// Title/Name must not be empty
	if c.Title == "" || c.Name == "" {
		return shared.NewDomainError("invalid_category_state", "category must have a title/name", nil)
	}
	
	// Category ID must be valid
	if string(c.ID) == "" {
		return shared.NewDomainError("invalid_category_state", "category must have a valid ID", nil)
	}
	
	// UserID must be valid
	if c.UserID == "" {
		return shared.NewDomainError("invalid_category_state", "category must have a valid user ID", nil)
	}
	
	// Level must be non-negative
	if c.Level < 0 {
		return shared.NewDomainError("invalid_category_state", "category level must be non-negative", nil)
	}
	
	// Note count must be non-negative
	if c.NoteCount < 0 {
		return shared.NewDomainError("invalid_category_state", "category note count must be non-negative", nil)
	}
	
	// Timestamps must be valid
	if c.CreatedAt.IsZero() {
		return shared.NewDomainError("invalid_category_state", "category must have a creation timestamp", nil)
	}
	
	if c.UpdatedAt.Before(c.CreatedAt) {
		return shared.NewDomainError("invalid_category_state", "category update timestamp cannot be before creation timestamp", nil)
	}
	
	// Version must be non-negative
	if c.Version.Int() < 0 {
		return shared.NewDomainError("invalid_category_state", "category version must be non-negative", nil)
	}
	
	// Hierarchy validation: ParentID cannot be self
	if c.ParentID != nil && *c.ParentID == c.ID {
		return shared.NewDomainError("invalid_category_state", "category cannot be its own parent", nil)
	}
	
	return nil
}

// SetParent sets the parent category with validation
func (c *Category) SetParent(parentID *shared.CategoryID, parentLevel int) error {
	// Validate parent is not self
	if parentID != nil && *parentID == c.ID {
		return shared.NewDomainError("invalid_parent", "category cannot be its own parent", nil)
	}
	
	// Set parent and update level
	c.ParentID = parentID
	if parentID == nil {
		c.Level = 0 // Root level
	} else {
		c.Level = parentLevel + 1 // Child level
	}
	
	c.UpdatedAt = time.Now()
	c.Version = c.Version.Next()
	
	// Generate domain event for category update
	updateEvent := NewCategoryUpdatedEvent(c)
	c.addEvent(updateEvent)
	
	return nil
}

// GetID returns the unique identifier of the category aggregate
func (c *Category) GetID() string {
	return string(c.ID)
}

// GetVersion returns the current version for optimistic locking
func (c *Category) GetVersion() int {
	return c.Version.Int()
}

// IncrementVersion increments the version after successful persistence
func (c *Category) IncrementVersion() {
	c.Version = c.Version.Next()
}

// Domain Events Implementation (EventAggregate interface)

// GetUncommittedEvents returns events that haven't been persisted yet
func (c *Category) GetUncommittedEvents() []shared.DomainEvent {
	// Use the BaseAggregateRoot's implementation if events are tracked there
	baseEvents := c.BaseAggregateRoot.GetUncommittedEvents()
	if len(baseEvents) > 0 {
		return baseEvents
	}
	// Fall back to local events for backward compatibility
	return c.events
}

// MarkEventsAsCommitted clears the events after persistence
func (c *Category) MarkEventsAsCommitted() {
	c.BaseAggregateRoot.MarkEventsAsCommitted()
	c.events = []shared.DomainEvent{}
}

// addEvent adds a domain event to the uncommitted events list
func (c *Category) addEvent(event shared.DomainEvent) {
	c.BaseAggregateRoot.AddEvent(event)
	c.events = append(c.events, event) // Keep for backward compatibility
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
