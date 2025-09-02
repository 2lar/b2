// Package outbox implements the transactional outbox pattern for guaranteed event delivery
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/events"
	"github.com/google/uuid"
)

// OutboxEntry represents an event in the outbox
type OutboxEntry struct {
	ID            string                 `json:"id"`
	AggregateID   string                 `json:"aggregate_id"`
	EventType     string                 `json:"event_type"`
	EventData     []byte                 `json:"event_data"`
	Metadata      map[string]interface{} `json:"metadata"`
	CreatedAt     time.Time              `json:"created_at"`
	ProcessedAt   *time.Time             `json:"processed_at"`
	RetryCount    int                    `json:"retry_count"`
	MaxRetries    int                    `json:"max_retries"`
	NextRetryAt   *time.Time             `json:"next_retry_at"`
	Error         string                 `json:"error,omitempty"`
	Status        OutboxStatus           `json:"status"`
}

// OutboxStatus represents the status of an outbox entry
type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusProcessing OutboxStatus = "processing"
	OutboxStatusProcessed  OutboxStatus = "processed"
	OutboxStatusFailed     OutboxStatus = "failed"
	OutboxStatusDead       OutboxStatus = "dead"
)

// OutboxStore is the interface for outbox persistence
type OutboxStore interface {
	// SaveEntry saves an outbox entry within a transaction
	SaveEntry(ctx context.Context, entry *OutboxEntry) error
	
	// GetPendingEntries retrieves entries ready for processing
	GetPendingEntries(ctx context.Context, limit int) ([]*OutboxEntry, error)
	
	// MarkAsProcessed marks an entry as successfully processed
	MarkAsProcessed(ctx context.Context, entryID string) error
	
	// MarkAsFailed marks an entry as failed with error details
	MarkAsFailed(ctx context.Context, entryID string, err error) error
	
	// UpdateRetry updates retry information for an entry
	UpdateRetry(ctx context.Context, entryID string, nextRetryAt time.Time) error
	
	// DeleteOldEntries deletes processed entries older than retention period
	DeleteOldEntries(ctx context.Context, before time.Time) error
}

// Outbox manages transactional event publishing
type Outbox struct {
	store      OutboxStore
	publisher  EventPublisher
	logger     ports.Logger
	metrics    ports.Metrics
	processor  *OutboxProcessor
	mu         sync.RWMutex
	closed     bool
}

// EventPublisher publishes events to external systems
type EventPublisher interface {
	// Publish sends an event to the message bus
	Publish(ctx context.Context, event events.DomainEvent) error
}

// NewOutbox creates a new outbox
func NewOutbox(store OutboxStore, publisher EventPublisher, logger ports.Logger, metrics ports.Metrics) *Outbox {
	return &Outbox{
		store:     store,
		publisher: publisher,
		logger:    logger,
		metrics:   metrics,
	}
}

// SaveWithTransaction saves an event to the outbox within a transaction
func (o *Outbox) SaveWithTransaction(ctx context.Context, tx interface{}, event events.DomainEvent) error {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	if o.closed {
		return fmt.Errorf("outbox is closed")
	}
	
	// Serialize event
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}
	
	// Create outbox entry
	entry := &OutboxEntry{
		ID:          uuid.New().String(),
		AggregateID: event.GetAggregateID(),
		EventType:   event.GetEventType(),
		EventData:   eventData,
		Metadata: map[string]interface{}{
			"correlation_id": event.GetMetadata().CorrelationID,
			"user_id":        event.GetMetadata().UserID,
		},
		CreatedAt:  time.Now(),
		MaxRetries: 3,
		Status:     OutboxStatusPending,
	}
	
	// Save to store (within the same transaction)
	if err := o.store.SaveEntry(ctx, entry); err != nil {
		o.metrics.IncrementCounter("outbox.save.failed")
		return fmt.Errorf("failed to save outbox entry: %w", err)
	}
	
	o.metrics.IncrementCounter("outbox.entry.created",
		ports.Tag{Key: "event_type", Value: event.GetEventType()})
	
	return nil
}

// StartProcessor starts the background processor for outbox entries
func (o *Outbox) StartProcessor(ctx context.Context, interval time.Duration) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	if o.closed {
		return fmt.Errorf("outbox is closed")
	}
	
	if o.processor != nil {
		return fmt.Errorf("processor already started")
	}
	
	o.processor = &OutboxProcessor{
		outbox:   o,
		interval: interval,
		stop:     make(chan struct{}),
	}
	
	go o.processor.Run(ctx)
	
	o.logger.Info("Outbox processor started",
		ports.Field{Key: "interval", Value: interval})
	
	return nil
}

// StopProcessor stops the background processor
func (o *Outbox) StopProcessor() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	if o.processor == nil {
		return nil
	}
	
	close(o.processor.stop)
	o.processor = nil
	
	o.logger.Info("Outbox processor stopped")
	
	return nil
}

// Close closes the outbox
func (o *Outbox) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	
	o.closed = true
	
	if o.processor != nil {
		close(o.processor.stop)
		o.processor = nil
	}
	
	return nil
}

// OutboxProcessor processes outbox entries in the background
type OutboxProcessor struct {
	outbox   *Outbox
	interval time.Duration
	stop     chan struct{}
}

// Run starts the processor loop
func (p *OutboxProcessor) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stop:
			return
		case <-ticker.C:
			p.processEntries(ctx)
		}
	}
}

// processEntries processes pending outbox entries
func (p *OutboxProcessor) processEntries(ctx context.Context) {
	// Get pending entries
	entries, err := p.outbox.store.GetPendingEntries(ctx, 100)
	if err != nil {
		p.outbox.logger.Error("Failed to get pending entries", err)
		return
	}
	
	if len(entries) == 0 {
		return
	}
	
	p.outbox.logger.Debug("Processing outbox entries",
		ports.Field{Key: "count", Value: len(entries)})
	
	// Process each entry
	for _, entry := range entries {
		if err := p.processEntry(ctx, entry); err != nil {
			p.outbox.logger.Error("Failed to process entry",
				err,
				ports.Field{Key: "entry_id", Value: entry.ID},
				ports.Field{Key: "event_type", Value: entry.EventType})
		}
	}
}

// processEntry processes a single outbox entry
func (p *OutboxProcessor) processEntry(ctx context.Context, entry *OutboxEntry) error {
	// Deserialize event
	var event events.BaseEvent
	if err := json.Unmarshal(entry.EventData, &event); err != nil {
		// Mark as dead if can't deserialize
		p.outbox.store.MarkAsFailed(ctx, entry.ID, err)
		return fmt.Errorf("failed to deserialize event: %w", err)
	}
	
	// Publish event
	if err := p.outbox.publisher.Publish(ctx, &event); err != nil {
		// Handle failure
		entry.RetryCount++
		
		if entry.RetryCount >= entry.MaxRetries {
			// Mark as dead letter
			p.outbox.store.MarkAsFailed(ctx, entry.ID, err)
			p.outbox.metrics.IncrementCounter("outbox.entry.dead",
				ports.Tag{Key: "event_type", Value: entry.EventType})
		} else {
			// Schedule retry with exponential backoff
			nextRetry := time.Now().Add(time.Duration(entry.RetryCount) * time.Minute)
			p.outbox.store.UpdateRetry(ctx, entry.ID, nextRetry)
			p.outbox.metrics.IncrementCounter("outbox.entry.retry",
				ports.Tag{Key: "event_type", Value: entry.EventType})
		}
		
		return err
	}
	
	// Mark as processed
	if err := p.outbox.store.MarkAsProcessed(ctx, entry.ID); err != nil {
		return fmt.Errorf("failed to mark as processed: %w", err)
	}
	
	p.outbox.metrics.IncrementCounter("outbox.entry.processed",
		ports.Tag{Key: "event_type", Value: entry.EventType})
	
	return nil
}

// InMemoryOutboxStore is an in-memory implementation for testing
type InMemoryOutboxStore struct {
	mu      sync.RWMutex
	entries map[string]*OutboxEntry
}

// NewInMemoryOutboxStore creates a new in-memory outbox store
func NewInMemoryOutboxStore() *InMemoryOutboxStore {
	return &InMemoryOutboxStore{
		entries: make(map[string]*OutboxEntry),
	}
}

// SaveEntry saves an outbox entry
func (s *InMemoryOutboxStore) SaveEntry(ctx context.Context, entry *OutboxEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.entries[entry.ID] = entry
	return nil
}

// GetPendingEntries retrieves entries ready for processing
func (s *InMemoryOutboxStore) GetPendingEntries(ctx context.Context, limit int) ([]*OutboxEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var pending []*OutboxEntry
	now := time.Now()
	
	for _, entry := range s.entries {
		if entry.Status == OutboxStatusPending {
			// Check if ready for retry
			if entry.NextRetryAt == nil || entry.NextRetryAt.Before(now) {
				pending = append(pending, entry)
				if len(pending) >= limit {
					break
				}
			}
		}
	}
	
	return pending, nil
}

// MarkAsProcessed marks an entry as successfully processed
func (s *InMemoryOutboxStore) MarkAsProcessed(ctx context.Context, entryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	entry, exists := s.entries[entryID]
	if !exists {
		return fmt.Errorf("entry not found: %s", entryID)
	}
	
	now := time.Now()
	entry.Status = OutboxStatusProcessed
	entry.ProcessedAt = &now
	
	return nil
}

// MarkAsFailed marks an entry as failed
func (s *InMemoryOutboxStore) MarkAsFailed(ctx context.Context, entryID string, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	entry, exists := s.entries[entryID]
	if !exists {
		return fmt.Errorf("entry not found: %s", entryID)
	}
	
	entry.Status = OutboxStatusFailed
	if err != nil {
		entry.Error = err.Error()
	}
	
	return nil
}

// UpdateRetry updates retry information for an entry
func (s *InMemoryOutboxStore) UpdateRetry(ctx context.Context, entryID string, nextRetryAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	entry, exists := s.entries[entryID]
	if !exists {
		return fmt.Errorf("entry not found: %s", entryID)
	}
	
	entry.NextRetryAt = &nextRetryAt
	
	return nil
}

// DeleteOldEntries deletes processed entries older than retention period
func (s *InMemoryOutboxStore) DeleteOldEntries(ctx context.Context, before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for id, entry := range s.entries {
		if entry.Status == OutboxStatusProcessed && entry.CreatedAt.Before(before) {
			delete(s.entries, id)
		}
	}
	
	return nil
}