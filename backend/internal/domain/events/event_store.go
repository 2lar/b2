// Package events provides domain event handling with Event Sourcing capabilities.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"brain2-backend/internal/domain/shared"
)

// ============================================================================
// EVENT STORE INTERFACE - Persistent event storage
// ============================================================================

// EventStore provides persistent storage for domain events.
// This enables Event Sourcing and audit logging.
type EventStore interface {
	// Save an event to the store
	Save(ctx context.Context, event shared.DomainEvent) error
	
	// Save multiple events atomically
	SaveBatch(ctx context.Context, events []shared.DomainEvent) error
	
	// Load events for an aggregate
	LoadEvents(ctx context.Context, aggregateID string, fromVersion int) ([]shared.DomainEvent, error)
	
	// Load events by type
	LoadEventsByType(ctx context.Context, eventType string, from, to time.Time) ([]shared.DomainEvent, error)
	
	// Load all events in a time range
	LoadEventsByTimeRange(ctx context.Context, from, to time.Time) ([]shared.DomainEvent, error)
	
	// Get the current version of an aggregate
	GetAggregateVersion(ctx context.Context, aggregateID string) (int, error)
	
	// Create a snapshot of an aggregate state
	SaveSnapshot(ctx context.Context, aggregateID string, version int, state interface{}) error
	
	// Load the latest snapshot for an aggregate
	LoadSnapshot(ctx context.Context, aggregateID string) (*AggregateSnapshot, error)
}

// ============================================================================
// EVENT SYNCHRONIZER - Ensures consistency between store and bus
// ============================================================================

// EventSynchronizer ensures events are consistently stored and published.
// It implements the Transactional Outbox pattern.
type EventSynchronizer struct {
	store    EventStore
	bus      shared.EventBus
	outbox   OutboxStore
	retryPolicy RetryPolicy
}

// NewEventSynchronizer creates a new event synchronizer.
func NewEventSynchronizer(
	store EventStore,
	bus shared.EventBus,
	outbox OutboxStore,
	retryPolicy RetryPolicy,
) *EventSynchronizer {
	return &EventSynchronizer{
		store:    store,
		bus:      bus,
		outbox:   outbox,
		retryPolicy: retryPolicy,
	}
}

// PublishEvent ensures an event is stored and published atomically.
func (s *EventSynchronizer) PublishEvent(ctx context.Context, event shared.DomainEvent) error {
	// 1. Save to event store
	if err := s.store.Save(ctx, event); err != nil {
		return fmt.Errorf("failed to save event: %w", err)
	}
	
	// 2. Save to outbox for guaranteed delivery
	outboxEntry := &OutboxEntry{
		EventID:   event.EventID(),
		EventType: event.EventType(),
		Payload:   event,
		Status:    OutboxStatusPending,
		CreatedAt: time.Now(),
	}
	
	if err := s.outbox.Save(ctx, outboxEntry); err != nil {
		return fmt.Errorf("failed to save to outbox: %w", err)
	}
	
	// 3. Attempt immediate publishing (best effort)
	if err := s.bus.Publish(ctx, event); err != nil {
		// Don't fail - outbox processor will retry
		outboxEntry.Status = OutboxStatusFailed
		outboxEntry.Error = err.Error()
		s.outbox.Update(ctx, outboxEntry)
		return nil
	}
	
	// 4. Mark as published in outbox
	outboxEntry.Status = OutboxStatusPublished
	outboxEntry.PublishedAt = &[]time.Time{time.Now()}[0]
	s.outbox.Update(ctx, outboxEntry)
	
	return nil
}

// PublishEvents publishes multiple events atomically.
func (s *EventSynchronizer) PublishEvents(ctx context.Context, events []shared.DomainEvent) error {
	// Save all events to store
	if err := s.store.SaveBatch(ctx, events); err != nil {
		return fmt.Errorf("failed to save events: %w", err)
	}
	
	// Save all to outbox
	entries := make([]*OutboxEntry, len(events))
	for i, event := range events {
		entries[i] = &OutboxEntry{
			EventID:   event.EventID(),
			EventType: event.EventType(),
			Payload:   event,
			Status:    OutboxStatusPending,
			CreatedAt: time.Now(),
		}
	}
	
	if err := s.outbox.SaveBatch(ctx, entries); err != nil {
		return fmt.Errorf("failed to save to outbox: %w", err)
	}
	
	// Attempt to publish all
	for i, event := range events {
		if err := s.bus.Publish(ctx, event); err != nil {
			entries[i].Status = OutboxStatusFailed
			entries[i].Error = err.Error()
		} else {
			entries[i].Status = OutboxStatusPublished
			entries[i].PublishedAt = &[]time.Time{time.Now()}[0]
		}
		s.outbox.Update(ctx, entries[i])
	}
	
	return nil
}

// ProcessOutbox processes pending events in the outbox.
// This should be called periodically to ensure eventual consistency.
func (s *EventSynchronizer) ProcessOutbox(ctx context.Context) error {
	// Get pending events from outbox
	pending, err := s.outbox.GetPending(ctx, 100)
	if err != nil {
		return fmt.Errorf("failed to get pending events: %w", err)
	}
	
	for _, entry := range pending {
		// Check retry policy
		if !s.retryPolicy.ShouldRetry(entry) {
			// Mark as dead letter
			entry.Status = OutboxStatusDeadLetter
			s.outbox.Update(ctx, entry)
			continue
		}
		
		// Attempt to publish
		if err := s.bus.Publish(ctx, entry.Payload); err != nil {
			entry.Status = OutboxStatusFailed
			entry.Error = err.Error()
			entry.RetryCount++
			entry.LastRetryAt = &[]time.Time{time.Now()}[0]
		} else {
			entry.Status = OutboxStatusPublished
			entry.PublishedAt = &[]time.Time{time.Now()}[0]
		}
		
		s.outbox.Update(ctx, entry)
	}
	
	return nil
}

// ============================================================================
// OUTBOX PATTERN - Transactional outbox for guaranteed delivery
// ============================================================================

// OutboxStatus represents the status of an outbox entry.
type OutboxStatus string

const (
	OutboxStatusPending     OutboxStatus = "PENDING"
	OutboxStatusPublished   OutboxStatus = "PUBLISHED"
	OutboxStatusFailed      OutboxStatus = "FAILED"
	OutboxStatusDeadLetter  OutboxStatus = "DEAD_LETTER"
)

// OutboxEntry represents an event in the outbox.
type OutboxEntry struct {
	EventID     string
	EventType   string
	Payload     shared.DomainEvent
	Status      OutboxStatus
	Error       string
	RetryCount  int
	CreatedAt   time.Time
	PublishedAt *time.Time
	LastRetryAt *time.Time
}

// OutboxStore provides storage for the transactional outbox.
type OutboxStore interface {
	// Save an entry to the outbox
	Save(ctx context.Context, entry *OutboxEntry) error
	
	// Save multiple entries atomically
	SaveBatch(ctx context.Context, entries []*OutboxEntry) error
	
	// Update an existing entry
	Update(ctx context.Context, entry *OutboxEntry) error
	
	// Get pending entries
	GetPending(ctx context.Context, limit int) ([]*OutboxEntry, error)
	
	// Get failed entries
	GetFailed(ctx context.Context, limit int) ([]*OutboxEntry, error)
	
	// Clean up old published entries
	CleanupPublished(ctx context.Context, olderThan time.Time) error
}

// ============================================================================
// RETRY POLICY - Configurable retry behavior
// ============================================================================

// RetryPolicy determines retry behavior for failed events.
type RetryPolicy interface {
	ShouldRetry(entry *OutboxEntry) bool
	GetBackoffDuration(retryCount int) time.Duration
}

// ExponentialRetryPolicy implements exponential backoff.
type ExponentialRetryPolicy struct {
	MaxRetries     int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	BackoffFactor  float64
}

// ShouldRetry determines if an entry should be retried.
func (p *ExponentialRetryPolicy) ShouldRetry(entry *OutboxEntry) bool {
	if entry.RetryCount >= p.MaxRetries {
		return false
	}
	
	if entry.LastRetryAt != nil {
		backoff := p.GetBackoffDuration(entry.RetryCount)
		nextRetryTime := entry.LastRetryAt.Add(backoff)
		if time.Now().Before(nextRetryTime) {
			return false
		}
	}
	
	return true
}

// GetBackoffDuration calculates the backoff duration for a retry count.
func (p *ExponentialRetryPolicy) GetBackoffDuration(retryCount int) time.Duration {
	delay := float64(p.BaseDelay) * p.BackoffFactor * float64(retryCount)
	if delay > float64(p.MaxDelay) {
		return p.MaxDelay
	}
	return time.Duration(delay)
}

// ============================================================================
// EVENT REPLAY - Rebuild state from events
// ============================================================================

// EventReplayer rebuilds aggregate state from events.
type EventReplayer struct {
	store EventStore
}

// NewEventReplayer creates a new event replayer.
func NewEventReplayer(store EventStore) *EventReplayer {
	return &EventReplayer{store: store}
}

// ReplayAggregate rebuilds an aggregate from its events.
func (r *EventReplayer) ReplayAggregate(ctx context.Context, aggregateID string, aggregate EventSourcedAggregate) error {
	// Try to load snapshot first
	snapshot, err := r.store.LoadSnapshot(ctx, aggregateID)
	if err == nil && snapshot != nil {
		// Restore from snapshot
		if err := aggregate.RestoreFromSnapshot(snapshot); err != nil {
			return fmt.Errorf("failed to restore from snapshot: %w", err)
		}
	}
	
	// Load events after snapshot
	fromVersion := 0
	if snapshot != nil {
		fromVersion = snapshot.Version + 1
	}
	
	events, err := r.store.LoadEvents(ctx, aggregateID, fromVersion)
	if err != nil {
		return fmt.Errorf("failed to load events: %w", err)
	}
	
	// Apply events to aggregate
	for _, event := range events {
		if err := aggregate.Apply(event); err != nil {
			return fmt.Errorf("failed to apply event: %w", err)
		}
	}
	
	return nil
}

// ============================================================================
// AGGREGATE SNAPSHOT - Optimization for event replay
// ============================================================================

// AggregateSnapshot represents a point-in-time snapshot of an aggregate.
type AggregateSnapshot struct {
	AggregateID string
	Version     int
	State       json.RawMessage
	CreatedAt   time.Time
}

// EventSourcedAggregate represents an aggregate that can be rebuilt from events.
type EventSourcedAggregate interface {
	Apply(event shared.DomainEvent) error
	RestoreFromSnapshot(snapshot *AggregateSnapshot) error
	CreateSnapshot() (*AggregateSnapshot, error)
}

// ============================================================================
// EVENT PROJECTION - Build read models from events
// ============================================================================

// Projection builds and maintains a read model from events.
type Projection interface {
	// Handle an event and update the read model
	Handle(ctx context.Context, event shared.DomainEvent) error
	
	// Get the name of the projection
	GetName() string
	
	// Get the last processed event position
	GetLastPosition(ctx context.Context) (int64, error)
	
	// Set the last processed event position
	SetLastPosition(ctx context.Context, position int64) error
	
	// Reset the projection
	Reset(ctx context.Context) error
}

// ProjectionManager manages multiple projections.
type ProjectionManager struct {
	store       EventStore
	projections []Projection
}

// NewProjectionManager creates a new projection manager.
func NewProjectionManager(store EventStore, projections ...Projection) *ProjectionManager {
	return &ProjectionManager{
		store:       store,
		projections: projections,
	}
}

// ProcessEvents processes new events through all projections.
func (m *ProjectionManager) ProcessEvents(ctx context.Context) error {
	// Process each projection
	for _, projection := range m.projections {
		// Get last position
		lastPos, err := projection.GetLastPosition(ctx)
		if err != nil {
			return fmt.Errorf("failed to get last position for %s: %w", projection.GetName(), err)
		}
		
		// Load events after last position
		events, err := m.store.LoadEventsByTimeRange(ctx, time.Unix(lastPos, 0), time.Now())
		if err != nil {
			return fmt.Errorf("failed to load events: %w", err)
		}
		
		// Process events
		for _, event := range events {
			if err := projection.Handle(ctx, event); err != nil {
				return fmt.Errorf("projection %s failed to handle event: %w", projection.GetName(), err)
			}
		}
		
		// Update position
		if len(events) > 0 {
			lastEvent := events[len(events)-1]
			if err := projection.SetLastPosition(ctx, lastEvent.Timestamp().Unix()); err != nil {
				return fmt.Errorf("failed to update position: %w", err)
			}
		}
	}
	
	return nil
}

// RebuildProjection rebuilds a specific projection from scratch.
func (m *ProjectionManager) RebuildProjection(ctx context.Context, name string) error {
	// Find projection
	var projection Projection
	for _, p := range m.projections {
		if p.GetName() == name {
			projection = p
			break
		}
	}
	
	if projection == nil {
		return fmt.Errorf("projection %s not found", name)
	}
	
	// Reset projection
	if err := projection.Reset(ctx); err != nil {
		return fmt.Errorf("failed to reset projection: %w", err)
	}
	
	// Load all events
	events, err := m.store.LoadEventsByTimeRange(ctx, time.Time{}, time.Now())
	if err != nil {
		return fmt.Errorf("failed to load events: %w", err)
	}
	
	// Process all events
	for _, event := range events {
		if err := projection.Handle(ctx, event); err != nil {
			return fmt.Errorf("failed to handle event: %w", err)
		}
	}
	
	return nil
}