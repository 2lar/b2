package projections

import (
	"context"
	"fmt"
	"reflect"

	"backend/domain/events"
)

// ProjectionHandler is the base interface for all projection handlers.
// Projections listen to domain events and update read models accordingly.
// This demonstrates the "write model to read model" transformation in CQRS.
type ProjectionHandler interface {
	// Handle processes a domain event and updates the corresponding read model
	Handle(ctx context.Context, event events.DomainEvent) error
	
	// GetEventTypes returns the types of events this projection handles
	GetEventTypes() []string
	
	// GetProjectionName returns a unique name for this projection
	GetProjectionName() string
	
	// Reset clears the projection's read model (useful for replay scenarios)
	Reset(ctx context.Context) error
}

// BaseProjection provides common functionality for all projections
type BaseProjection struct {
	name       string
	eventTypes map[string]bool
}

// NewBaseProjection creates a new base projection
func NewBaseProjection(name string, eventTypes []string) *BaseProjection {
	typeMap := make(map[string]bool)
	for _, et := range eventTypes {
		typeMap[et] = true
	}
	
	return &BaseProjection{
		name:       name,
		eventTypes: typeMap,
	}
}

// GetProjectionName returns the projection's name
func (p *BaseProjection) GetProjectionName() string {
	return p.name
}

// GetEventTypes returns the event types this projection handles
func (p *BaseProjection) GetEventTypes() []string {
	types := make([]string, 0, len(p.eventTypes))
	for t := range p.eventTypes {
		types = append(types, t)
	}
	return types
}

// CanHandle checks if this projection can handle the given event type
func (p *BaseProjection) CanHandle(eventType string) bool {
	return p.eventTypes[eventType]
}

// ProjectionError represents an error that occurred during projection handling
type ProjectionError struct {
	ProjectionName string
	EventID        string
	EventType      string
	Err            error
}

func (e *ProjectionError) Error() string {
	return fmt.Sprintf("projection '%s' failed to handle event %s (type: %s): %v",
		e.ProjectionName, e.EventID, e.EventType, e.Err)
}

// GetEventType extracts the event type from a domain event using reflection
// This is used for routing events to the appropriate projections
func GetEventType(event events.DomainEvent) string {
	if event == nil {
		return ""
	}
	
	eventType := reflect.TypeOf(event)
	if eventType.Kind() == reflect.Ptr {
		eventType = eventType.Elem()
	}
	
	return eventType.Name()
}

// ProjectionPosition tracks the position of a projection in the event stream
// This is crucial for event replay and ensuring exactly-once processing
type ProjectionPosition struct {
	ProjectionName string `json:"projection_name"`
	Position       int64  `json:"position"`
	LastEventID    string `json:"last_event_id"`
	UpdatedAt      int64  `json:"updated_at"`
}

// CheckpointStore manages projection positions for event replay
type CheckpointStore interface {
	// SavePosition saves the current position of a projection
	SavePosition(ctx context.Context, position *ProjectionPosition) error
	
	// GetPosition retrieves the last saved position for a projection
	GetPosition(ctx context.Context, projectionName string) (*ProjectionPosition, error)
	
	// DeletePosition removes the position record for a projection (used during reset)
	DeletePosition(ctx context.Context, projectionName string) error
}

// ProjectionStats provides metrics about projection processing
type ProjectionStats struct {
	ProjectionName   string
	EventsProcessed  int64
	LastEventTime    int64
	AverageLatencyMs float64
	ErrorCount       int64
}