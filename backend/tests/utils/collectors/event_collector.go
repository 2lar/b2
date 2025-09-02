// Package collectors provides test utilities for collecting and asserting on events
package collectors

import (
	"sync"
	"testing"
	"time"

	"brain2-backend/internal/core/domain/events"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EventCollector collects domain events for testing
type EventCollector struct {
	mu        sync.RWMutex
	events    []events.DomainEvent
	eventChan chan events.DomainEvent
	done      chan bool
	t         *testing.T
}

// NewEventCollector creates a new event collector
func NewEventCollector(t *testing.T) *EventCollector {
	ec := &EventCollector{
		events:    []events.DomainEvent{},
		eventChan: make(chan events.DomainEvent, 100),
		done:      make(chan bool),
		t:         t,
	}
	
	// Start collecting events
	go ec.collect()
	
	return ec
}

// collect runs in the background collecting events
func (ec *EventCollector) collect() {
	for {
		select {
		case event := <-ec.eventChan:
			ec.mu.Lock()
			ec.events = append(ec.events, event)
			ec.mu.Unlock()
		case <-ec.done:
			return
		}
	}
}

// Collect adds an event to the collector
func (ec *EventCollector) Collect(event events.DomainEvent) {
	select {
	case ec.eventChan <- event:
	case <-time.After(time.Second):
		ec.t.Fatal("Timeout collecting event")
	}
}

// CollectMany adds multiple events to the collector
func (ec *EventCollector) CollectMany(events []events.DomainEvent) {
	for _, event := range events {
		ec.Collect(event)
	}
}

// GetEvents returns all collected events
func (ec *EventCollector) GetEvents() []events.DomainEvent {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	
	result := make([]events.DomainEvent, len(ec.events))
	copy(result, ec.events)
	return result
}

// GetEventsByType returns events of a specific type
func (ec *EventCollector) GetEventsByType(eventType string) []events.DomainEvent {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	
	var result []events.DomainEvent
	for _, event := range ec.events {
		if event.GetEventType() == eventType {
			result = append(result, event)
		}
	}
	return result
}

// GetEventsByAggregateID returns events for a specific aggregate
func (ec *EventCollector) GetEventsByAggregateID(aggregateID string) []events.DomainEvent {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	
	var result []events.DomainEvent
	for _, event := range ec.events {
		if event.GetAggregateID() == aggregateID {
			result = append(result, event)
		}
	}
	return result
}

// Count returns the total number of collected events
func (ec *EventCollector) Count() int {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return len(ec.events)
}

// CountByType returns the number of events of a specific type
func (ec *EventCollector) CountByType(eventType string) int {
	return len(ec.GetEventsByType(eventType))
}

// Clear removes all collected events
func (ec *EventCollector) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.events = []events.DomainEvent{}
}

// Stop stops the event collector
func (ec *EventCollector) Stop() {
	close(ec.done)
}

// WaitForEvents waits for a specific number of events to be collected
func (ec *EventCollector) WaitForEvents(count int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		if ec.Count() >= count {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	
	return false
}

// WaitForEventType waits for a specific event type to be collected
func (ec *EventCollector) WaitForEventType(eventType string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		if ec.CountByType(eventType) > 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	
	return false
}

// AssertEventPublished asserts that an event of a specific type was published
func (ec *EventCollector) AssertEventPublished(eventType string, msgAndArgs ...interface{}) {
	events := ec.GetEventsByType(eventType)
	assert.NotEmpty(ec.t, events, append([]interface{}{"Event " + eventType + " should have been published"}, msgAndArgs...)...)
}

// AssertEventNotPublished asserts that an event of a specific type was not published
func (ec *EventCollector) AssertEventNotPublished(eventType string, msgAndArgs ...interface{}) {
	events := ec.GetEventsByType(eventType)
	assert.Empty(ec.t, events, append([]interface{}{"Event " + eventType + " should not have been published"}, msgAndArgs...)...)
}

// AssertEventCount asserts the total number of events
func (ec *EventCollector) AssertEventCount(expected int, msgAndArgs ...interface{}) {
	assert.Equal(ec.t, expected, ec.Count(), append([]interface{}{"Event count should match"}, msgAndArgs...)...)
}

// AssertEventTypeCount asserts the number of events of a specific type
func (ec *EventCollector) AssertEventTypeCount(eventType string, expected int, msgAndArgs ...interface{}) {
	assert.Equal(ec.t, expected, ec.CountByType(eventType), 
		append([]interface{}{"Event type " + eventType + " count should match"}, msgAndArgs...)...)
}

// AssertEventSequence asserts that events were published in a specific order
func (ec *EventCollector) AssertEventSequence(expectedTypes []string, msgAndArgs ...interface{}) {
	events := ec.GetEvents()
	require.Len(ec.t, events, len(expectedTypes), "Event count should match expected sequence length")
	
	for i, expectedType := range expectedTypes {
		assert.Equal(ec.t, expectedType, events[i].GetEventType(), 
			append([]interface{}{"Event at position " + string(rune(i)) + " should be " + expectedType}, msgAndArgs...)...)
	}
}

// AssertLastEvent asserts properties of the last published event
func (ec *EventCollector) AssertLastEvent(expectedType string, msgAndArgs ...interface{}) events.DomainEvent {
	events := ec.GetEvents()
	require.NotEmpty(ec.t, events, "No events collected")
	
	lastEvent := events[len(events)-1]
	assert.Equal(ec.t, expectedType, lastEvent.GetEventType(), 
		append([]interface{}{"Last event type should be " + expectedType}, msgAndArgs...)...)
	
	return lastEvent
}

// GetLastEvent returns the last collected event
func (ec *EventCollector) GetLastEvent() events.DomainEvent {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	
	if len(ec.events) == 0 {
		return nil
	}
	
	return ec.events[len(ec.events)-1]
}

// MockEventBus implements a mock event bus that uses the collector
type MockEventBus struct {
	collector *EventCollector
}

// NewMockEventBus creates a new mock event bus
func NewMockEventBus(collector *EventCollector) *MockEventBus {
	return &MockEventBus{collector: collector}
}

// Publish publishes an event to the collector
func (m *MockEventBus) Publish(event events.DomainEvent) error {
	m.collector.Collect(event)
	return nil
}

// PublishBatch publishes multiple events to the collector
func (m *MockEventBus) PublishBatch(events []events.DomainEvent) error {
	m.collector.CollectMany(events)
	return nil
}