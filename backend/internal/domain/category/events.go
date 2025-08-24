package category

import (
	"brain2-backend/internal/domain/shared"
)

// NewCategoryUpdatedEvent creates an event when a category is updated
func NewCategoryUpdatedEvent(cat *Category) shared.DomainEvent {
	return &CategoryUpdatedEvent{
		BaseEvent: shared.NewBaseEvent(
			"CategoryUpdated",
			string(cat.ID),
			cat.UserID,
			1, // Version
		),
		CategoryID:  string(cat.ID),
		Name:        cat.Name,
		Description: cat.Description,
		ParentID:    categoryIDToString(cat.ParentID),
		Level:       cat.Level,
	}
}

// NewCategoryDeletedEvent creates an event when a category is deleted
func NewCategoryDeletedEvent(categoryID, userID string) shared.DomainEvent {
	return &CategoryDeletedEvent{
		BaseEvent: shared.NewBaseEvent(
			"CategoryDeleted",
			categoryID,
			userID,
			1, // Version
		),
		CategoryID: categoryID,
	}
}

// CategoryUpdatedEvent represents a category update
type CategoryUpdatedEvent struct {
	shared.BaseEvent
	CategoryID  string  `json:"category_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ParentID    *string `json:"parent_id,omitempty"`
	Level       int     `json:"level"`
}

// EventData returns the event-specific data
func (e *CategoryUpdatedEvent) EventData() map[string]interface{} {
	data := map[string]interface{}{
		"category_id": e.CategoryID,
		"name":        e.Name,
		"description": e.Description,
		"level":       e.Level,
	}
	if e.ParentID != nil {
		data["parent_id"] = *e.ParentID
	}
	return data
}

// CategoryDeletedEvent represents a category deletion
type CategoryDeletedEvent struct {
	shared.BaseEvent
	CategoryID string `json:"category_id"`
}

// EventData returns the event-specific data
func (e *CategoryDeletedEvent) EventData() map[string]interface{} {
	return map[string]interface{}{
		"category_id": e.CategoryID,
	}
}

// Helper function to convert CategoryID pointer to string pointer
func categoryIDToString(id *shared.CategoryID) *string {
	if id == nil {
		return nil
	}
	s := string(*id)
	return &s
}