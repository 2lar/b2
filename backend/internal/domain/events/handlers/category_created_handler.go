// Package handlers contains domain event handlers.
package handlers

import (
	"context"
	
	"brain2-backend/internal/domain/shared"
	"go.uber.org/zap"
)

// CategoryCreatedHandler handles category creation events.
// This is an example handler that demonstrates the event handler pattern.
type CategoryCreatedHandler struct {
	logger *zap.Logger
}

// NewCategoryCreatedHandler creates a new category created event handler
func NewCategoryCreatedHandler(logger *zap.Logger) *CategoryCreatedHandler {
	return &CategoryCreatedHandler{
		logger: logger,
	}
}

// Handle processes a category created event
func (h *CategoryCreatedHandler) Handle(ctx context.Context, event shared.DomainEvent) error {
	// Type assert to get the specific event
	categoryEvent, ok := event.(*shared.CategoryCreatedEvent)
	if !ok {
		return nil // Not our event type
	}
	
	// Log the event (in a real system, you might update read models, send notifications, etc.)
	h.logger.Info("Category created event handled",
		zap.String("category_id", categoryEvent.AggregateID()),
		zap.String("user_id", categoryEvent.UserID()),
		zap.String("name", categoryEvent.Name),
		zap.String("description", categoryEvent.Description),
		zap.Int("level", categoryEvent.Level),
		zap.Time("timestamp", categoryEvent.Timestamp()))
	
	// Here you could:
	// - Update a read model/projection
	// - Send a notification
	// - Trigger a workflow
	// - Update analytics
	
	return nil
}

// CanHandle checks if this handler can process the given event type
func (h *CategoryCreatedHandler) CanHandle(eventType string) bool {
	return eventType == "CategoryCreated"
}