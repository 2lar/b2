package listeners

import (
	"context"
	"fmt"

	appevents "backend/application/events"
	"backend/domain/events"
)

// OperationEventHandlerAdapter adapts the OperationEventListener to the EventHandler interface
type OperationEventHandlerAdapter struct {
	appevents.BaseEventHandler
	listener *OperationEventListener
}

// NewOperationEventHandlerAdapter creates a new adapter
func NewOperationEventHandlerAdapter(listener *OperationEventListener) *OperationEventHandlerAdapter {
	return &OperationEventHandlerAdapter{
		BaseEventHandler: appevents.NewBaseEventHandler(
			"OperationEventHandler",
			10, // priority
			[]string{"BulkNodesDeletedEvent"},
		),
		listener: listener,
	}
}

// Handle processes a domain event
func (a *OperationEventHandlerAdapter) Handle(ctx context.Context, event events.DomainEvent) error {
	switch e := event.(type) {
	case *events.BulkNodesDeletedEvent:
		return a.listener.HandleBulkNodesDeleted(ctx, e)
	default:
		return fmt.Errorf("unsupported event type: %T", event)
	}
}