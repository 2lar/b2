// Package eventstore implements event sourcing persistence
package eventstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	
	"brain2-backend/internal/core/domain/events"
)

// EventStore is the main interface for event persistence
type EventStore interface {
	// SaveEvents persists new events for an aggregate
	SaveEvents(ctx context.Context, aggregateID string, events []events.DomainEvent, expectedVersion int64) error
	
	// LoadEvents retrieves all events for an aggregate
	LoadEvents(ctx context.Context, aggregateID string) ([]events.DomainEvent, error)
	
	// LoadEventsAfterVersion retrieves events after a specific version
	LoadEventsAfterVersion(ctx context.Context, aggregateID string, version int64) ([]events.DomainEvent, error)
	
	// GetSnapshot retrieves the latest snapshot for an aggregate
	GetSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error)
	
	// SaveSnapshot persists a snapshot
	SaveSnapshot(ctx context.Context, snapshot *Snapshot) error
	
	// GetEventStream creates a stream of events for real-time processing
	GetEventStream(ctx context.Context, fromPosition int64) (<-chan events.DomainEvent, error)
	
	// Close closes the event store
	Close() error
}

// Snapshot represents a point-in-time state snapshot
type Snapshot struct {
	AggregateID string    `json:"aggregate_id"`
	Version     int64     `json:"version"`
	Data        []byte    `json:"data"`
	CreatedAt   time.Time `json:"created_at"`
}

// StoredEvent represents an event as stored in the database
type StoredEvent struct {
	ID            string    `json:"id"`
	AggregateID   string    `json:"aggregate_id"`
	AggregateType string    `json:"aggregate_type"`
	EventType     string    `json:"event_type"`
	EventData     []byte    `json:"event_data"`
	EventMetadata []byte    `json:"event_metadata"`
	Version       int64     `json:"version"`
	Position      int64     `json:"position"`
	Timestamp     time.Time `json:"timestamp"`
}

// InMemoryEventStore is an in-memory implementation for testing
type InMemoryEventStore struct {
	mu         sync.RWMutex
	events     map[string][]StoredEvent
	snapshots  map[string]*Snapshot
	globalPos  int64
	streams    []chan events.DomainEvent
	serializer EventSerializer
	closed     bool
}

// EventSerializer handles event serialization/deserialization
type EventSerializer interface {
	// Serialize converts an event to bytes
	Serialize(event events.DomainEvent) ([]byte, error)
	
	// Deserialize converts bytes back to an event
	Deserialize(eventType string, data []byte) (events.DomainEvent, error)
}

// NewInMemoryEventStore creates a new in-memory event store
func NewInMemoryEventStore(serializer EventSerializer) *InMemoryEventStore {
	return &InMemoryEventStore{
		events:     make(map[string][]StoredEvent),
		snapshots:  make(map[string]*Snapshot),
		streams:    make([]chan events.DomainEvent, 0),
		serializer: serializer,
	}
}

// SaveEvents persists new events for an aggregate
func (s *InMemoryEventStore) SaveEvents(ctx context.Context, aggregateID string, domainEvents []events.DomainEvent, expectedVersion int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return fmt.Errorf("event store is closed")
	}
	
	// Check for concurrent modification
	currentEvents := s.events[aggregateID]
	currentVersion := int64(len(currentEvents))
	
	if expectedVersion >= 0 && currentVersion != expectedVersion {
		return &ConcurrencyError{
			AggregateID:     aggregateID,
			ExpectedVersion: expectedVersion,
			ActualVersion:   currentVersion,
		}
	}
	
	// Convert and store events
	storedEvents := make([]StoredEvent, 0, len(domainEvents))
	for _, event := range domainEvents {
		s.globalPos++
		currentVersion++
		
		eventData, err := s.serializer.Serialize(event)
		if err != nil {
			return fmt.Errorf("failed to serialize event: %w", err)
		}
		
		metadataData, err := json.Marshal(event.GetMetadata())
		if err != nil {
			return fmt.Errorf("failed to serialize metadata: %w", err)
		}
		
		stored := StoredEvent{
			ID:            event.GetEventID(),
			AggregateID:   aggregateID,
			AggregateType: event.GetAggregateType(),
			EventType:     event.GetEventType(),
			EventData:     eventData,
			EventMetadata: metadataData,
			Version:       currentVersion,
			Position:      s.globalPos,
			Timestamp:     event.GetTimestamp(),
		}
		
		storedEvents = append(storedEvents, stored)
		
		// Notify streams
		s.notifyStreams(event)
	}
	
	// Append events
	s.events[aggregateID] = append(s.events[aggregateID], storedEvents...)
	
	return nil
}

// LoadEvents retrieves all events for an aggregate
func (s *InMemoryEventStore) LoadEvents(ctx context.Context, aggregateID string) ([]events.DomainEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.closed {
		return nil, fmt.Errorf("event store is closed")
	}
	
	storedEvents, exists := s.events[aggregateID]
	if !exists {
		return []events.DomainEvent{}, nil
	}
	
	return s.deserializeEvents(storedEvents)
}

// LoadEventsAfterVersion retrieves events after a specific version
func (s *InMemoryEventStore) LoadEventsAfterVersion(ctx context.Context, aggregateID string, version int64) ([]events.DomainEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.closed {
		return nil, fmt.Errorf("event store is closed")
	}
	
	storedEvents, exists := s.events[aggregateID]
	if !exists {
		return []events.DomainEvent{}, nil
	}
	
	// Filter events after version
	var filtered []StoredEvent
	for _, event := range storedEvents {
		if event.Version > version {
			filtered = append(filtered, event)
		}
	}
	
	return s.deserializeEvents(filtered)
}

// GetSnapshot retrieves the latest snapshot for an aggregate
func (s *InMemoryEventStore) GetSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.closed {
		return nil, fmt.Errorf("event store is closed")
	}
	
	snapshot, exists := s.snapshots[aggregateID]
	if !exists {
		return nil, nil
	}
	
	return snapshot, nil
}

// SaveSnapshot persists a snapshot
func (s *InMemoryEventStore) SaveSnapshot(ctx context.Context, snapshot *Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return fmt.Errorf("event store is closed")
	}
	
	s.snapshots[snapshot.AggregateID] = snapshot
	return nil
}

// GetEventStream creates a stream of events for real-time processing
func (s *InMemoryEventStore) GetEventStream(ctx context.Context, fromPosition int64) (<-chan events.DomainEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.closed {
		return nil, fmt.Errorf("event store is closed")
	}
	
	stream := make(chan events.DomainEvent, 100)
	s.streams = append(s.streams, stream)
	
	// Send historical events
	go func() {
		s.mu.RLock()
		defer s.mu.RUnlock()
		
		for _, aggregateEvents := range s.events {
			for _, stored := range aggregateEvents {
				if stored.Position > fromPosition {
					event, err := s.serializer.Deserialize(stored.EventType, stored.EventData)
					if err != nil {
						continue
					}
					
					select {
					case stream <- event:
					case <-ctx.Done():
						close(stream)
						return
					}
				}
			}
		}
	}()
	
	return stream, nil
}

// Close closes the event store
func (s *InMemoryEventStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.closed = true
	
	// Close all streams
	for _, stream := range s.streams {
		close(stream)
	}
	
	return nil
}

// notifyStreams sends an event to all active streams
func (s *InMemoryEventStore) notifyStreams(event events.DomainEvent) {
	for _, stream := range s.streams {
		select {
		case stream <- event:
		default:
			// Stream is full, skip
		}
	}
}

// deserializeEvents converts stored events back to domain events
func (s *InMemoryEventStore) deserializeEvents(storedEvents []StoredEvent) ([]events.DomainEvent, error) {
	domainEvents := make([]events.DomainEvent, 0, len(storedEvents))
	
	for _, stored := range storedEvents {
		event, err := s.serializer.Deserialize(stored.EventType, stored.EventData)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize event: %w", err)
		}
		domainEvents = append(domainEvents, event)
	}
	
	return domainEvents, nil
}

// ConcurrencyError represents a version conflict error
type ConcurrencyError struct {
	AggregateID     string
	ExpectedVersion int64
	ActualVersion   int64
}

// Error implements the error interface
func (e *ConcurrencyError) Error() string {
	return fmt.Sprintf("concurrency error for aggregate %s: expected version %d, actual version %d",
		e.AggregateID, e.ExpectedVersion, e.ActualVersion)
}

// EventQuery represents a query for events
type EventQuery struct {
	AggregateID   string
	EventTypes    []string
	FromVersion   int64
	ToVersion     int64
	FromTimestamp time.Time
	ToTimestamp   time.Time
	Limit         int
}

// EventStoreWithQuery extends EventStore with query capabilities
type EventStoreWithQuery interface {
	EventStore
	
	// QueryEvents queries events based on criteria
	QueryEvents(ctx context.Context, query EventQuery) ([]events.DomainEvent, error)
	
	// GetAggregateIDs returns all aggregate IDs
	GetAggregateIDs(ctx context.Context) ([]string, error)
}