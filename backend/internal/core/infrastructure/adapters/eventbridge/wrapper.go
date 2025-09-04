// Package eventbridge provides wrapper for messaging EventBridge publisher
package eventbridge

import (
	"context"
	
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/events"
	"brain2-backend/internal/infrastructure/messaging"
	
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
)

// EventBusAdapter adapts the messaging EventBridgePublisher to ports.EventBus
type EventBusAdapter struct {
	publisher *messaging.EventBridgePublisher
	logger    ports.Logger
}

// NewEventBridgePublisher creates a new EventBridge publisher that implements ports.EventBus
func NewEventBridgePublisher(
	client *eventbridge.Client,
	eventBusName string,
	source string,
	logger ports.Logger,
) ports.EventBus {
	// Create the EventBridge publisher using the concrete constructor
	// This ensures we get the concrete type directly
	publisher := messaging.NewConcreteEventBridgePublisher(client, eventBusName, source)
	
	return &EventBusAdapter{
		publisher: publisher,
		logger:    logger,
	}
}

// Publish sends an event to all subscribers
func (a *EventBusAdapter) Publish(ctx context.Context, event events.DomainEvent) error {
	// The event already implements what we need, just publish it directly
	// The EventBridgePublisher will handle the conversion
	return a.publisher.PublishDomainEvent(ctx, event)
}

// PublishBatch sends multiple events
func (a *EventBusAdapter) PublishBatch(ctx context.Context, domainEvents []events.DomainEvent) error {
	if len(domainEvents) == 0 {
		return nil
	}
	
	// The events already implement what we need, just publish them directly
	// The EventBridgePublisher will handle the conversion
	return a.publisher.PublishDomainEvents(ctx, domainEvents)
}

// Subscribe registers a handler for specific event types
func (a *EventBusAdapter) Subscribe(eventType string, handler ports.EventHandler) error {
	// EventBridge is a push-based system, subscription happens at the infrastructure level
	// This would be configured in AWS, not in code
	a.logger.Info("Subscribe called for EventBridge (configured at AWS level)",
		ports.Field{Key: "event_type", Value: eventType})
	return nil
}

// SubscribeAll registers a handler for all events
func (a *EventBusAdapter) SubscribeAll(handler ports.EventHandler) error {
	// EventBridge is a push-based system, subscription happens at the infrastructure level
	a.logger.Info("SubscribeAll called for EventBridge (configured at AWS level)")
	return nil
}

// Unsubscribe removes a handler
func (a *EventBusAdapter) Unsubscribe(eventType string, handler ports.EventHandler) error {
	// EventBridge is a push-based system, subscription happens at the infrastructure level
	a.logger.Info("Unsubscribe called for EventBridge (configured at AWS level)",
		ports.Field{Key: "event_type", Value: eventType})
	return nil
}

// Start begins processing events
func (a *EventBusAdapter) Start(ctx context.Context) error {
	// EventBridge is always running, no start needed
	a.logger.Info("EventBridge event bus started")
	return nil
}

// Stop stops processing events
func (a *EventBusAdapter) Stop(ctx context.Context) error {
	// EventBridge is always running, no stop needed
	a.logger.Info("EventBridge event bus stopped")
	return nil
}

// SimpleEventBus is a fallback implementation
type SimpleEventBus struct {
	logger ports.Logger
}

// Publish sends an event
func (s *SimpleEventBus) Publish(ctx context.Context, event events.DomainEvent) error {
	s.logger.Info("Event published (no-op)",
		ports.Field{Key: "event_type", Value: event.GetEventType()})
	return nil
}

// PublishBatch sends multiple events
func (s *SimpleEventBus) PublishBatch(ctx context.Context, events []events.DomainEvent) error {
	for _, event := range events {
		if err := s.Publish(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe registers a handler
func (s *SimpleEventBus) Subscribe(eventType string, handler ports.EventHandler) error {
	return nil
}

// SubscribeAll registers a handler for all events
func (s *SimpleEventBus) SubscribeAll(handler ports.EventHandler) error {
	return nil
}

// Unsubscribe removes a handler
func (s *SimpleEventBus) Unsubscribe(eventType string, handler ports.EventHandler) error {
	return nil
}

// Start begins processing events
func (s *SimpleEventBus) Start(ctx context.Context) error {
	return nil
}

// Stop stops processing events
func (s *SimpleEventBus) Stop(ctx context.Context) error {
	return nil
}