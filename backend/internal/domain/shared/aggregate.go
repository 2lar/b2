package shared

// AggregateRoot represents the root entity of an aggregate in DDD.
// It enforces consistency boundaries and manages domain events.
type AggregateRoot interface {
	// GetID returns the unique identifier of the aggregate
	GetID() string
	
	// GetVersion returns the current version for optimistic locking
	GetVersion() int
	
	// IncrementVersion increments the version after successful persistence
	IncrementVersion()
	
	// ValidateInvariants checks all business rules are satisfied
	ValidateInvariants() error
	
	// EventAggregate interface for event management
	EventAggregate
}

// BaseAggregateRoot provides common functionality for all aggregate roots
type BaseAggregateRoot struct {
	id      string
	version int
	events  []DomainEvent
}

// NewBaseAggregateRoot creates a new base aggregate root
func NewBaseAggregateRoot(id string) BaseAggregateRoot {
	return BaseAggregateRoot{
		id:      id,
		version: 0,
		events:  []DomainEvent{},
	}
}

// GetID returns the aggregate ID
func (a *BaseAggregateRoot) GetID() string {
	return a.id
}

// GetVersion returns the current version
func (a *BaseAggregateRoot) GetVersion() int {
	return a.version
}

// IncrementVersion increments the version
func (a *BaseAggregateRoot) IncrementVersion() {
	a.version++
}

// AddEvent adds a domain event to the aggregate
func (a *BaseAggregateRoot) AddEvent(event DomainEvent) {
	a.events = append(a.events, event)
}

// GetUncommittedEvents returns events that haven't been persisted
func (a *BaseAggregateRoot) GetUncommittedEvents() []DomainEvent {
	return a.events
}

// MarkEventsAsCommitted clears uncommitted events after persistence
func (a *BaseAggregateRoot) MarkEventsAsCommitted() {
	a.events = []DomainEvent{}
}

// ApplyEvent applies a domain event to rebuild aggregate state (for event sourcing)
func (a *BaseAggregateRoot) ApplyEvent(event DomainEvent) {
	// This would be overridden by specific aggregates to rebuild state
	a.version = event.Version()
}