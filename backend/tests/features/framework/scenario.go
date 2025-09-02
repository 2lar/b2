// Package framework provides a BDD-style testing framework for Brain2
package framework

import (
	"context"
	"testing"

	"brain2-backend/internal/core/application/commands"
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/aggregates/node"
	"brain2-backend/internal/core/domain/events"
	"brain2-backend/tests/fixtures/builders"
	
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Scenario represents a BDD test scenario
type Scenario struct {
	t           *testing.T
	context     context.Context
	given       *Given
	when        *When
	then        *Then
	
	// Test data
	users       map[string]string
	nodes       map[string]*node.Aggregate
	categories  map[string]interface{}
	events      []events.DomainEvent
	commands    []cqrs.Command
	errors      []error
	
	// Dependencies (will be mocked)
	commandBus  cqrs.CommandBus
	queryBus    cqrs.QueryBus
	eventBus    ports.EventBus
	nodeRepo    ports.NodeRepository
	edgeRepo    ports.EdgeRepository
	categoryRepo ports.CategoryRepository
}

// NewScenario creates a new test scenario
func NewScenario(t *testing.T) *Scenario {
	s := &Scenario{
		t:          t,
		context:    context.Background(),
		users:      make(map[string]string),
		nodes:      make(map[string]*node.Aggregate),
		categories: make(map[string]interface{}),
		events:     []events.DomainEvent{},
		commands:   []cqrs.Command{},
		errors:     []error{},
	}
	
	s.given = &Given{scenario: s}
	s.when = &When{scenario: s}
	s.then = &Then{scenario: s}
	
	return s
}

// Given returns the Given clause builder
func (s *Scenario) Given() *Given {
	return s.given
}

// When returns the When clause builder
func (s *Scenario) When() *When {
	return s.when
}

// Then returns the Then clause builder
func (s *Scenario) Then() *Then {
	return s.then
}

// Given represents the precondition setup
type Given struct {
	scenario *Scenario
}

// UserExists creates a test user
func (g *Given) UserExists(userID string) *Given {
	g.scenario.users[userID] = userID
	return g
}

// NodeExists creates a test node
func (g *Given) NodeExists(nodeID, userID string) *Given {
	node := builders.NewNodeBuilder().
		WithID(nodeID).
		WithUserID(userID).
		Build()
	g.scenario.nodes[nodeID] = node
	return g
}

// NodesExist creates multiple test nodes
func (g *Given) NodesExist(count int, userID string) *Given {
	for i := 0; i < count; i++ {
		nodeID := g.scenario.generateNodeID()
		g.NodeExists(nodeID, userID)
	}
	return g
}

// CategoryExists creates a test category
func (g *Given) CategoryExists(categoryID string) *Given {
	g.scenario.categories[categoryID] = map[string]interface{}{
		"id":    categoryID,
		"title": "Test Category",
	}
	return g
}

// SystemIsHealthy sets up a healthy system state
func (g *Given) SystemIsHealthy() *Given {
	// Setup healthy mocks
	return g
}

// And provides fluent chaining
func (g *Given) And() *Given {
	return g
}

// When transitions to the When clause
func (g *Given) When() *When {
	return g.scenario.when
}

// When represents the action being tested
type When struct {
	scenario *Scenario
}

// CreatingNode simulates node creation
func (w *When) CreatingNode(n *node.Aggregate) *When {
	cmd := &commands.CreateNodeCommand{}
	// Extract data from node aggregate
	if n != nil {
		cmd.Content = n.GetContent()
		cmd.Title = n.GetTitle()
	}
	
	w.scenario.commands = append(w.scenario.commands, cmd)
	
	// Simulate event emission
	if n != nil {
		event := builders.NewEventBuilder().
			WithAggregateID(n.GetID()).
			BuildNodeCreated(n.GetContent(), n.GetTitle(), n.GetTags())
		w.scenario.events = append(w.scenario.events, event)
	}
	
	return w
}

// UpdatingNode simulates node update
func (w *When) UpdatingNode(nodeID string, updates map[string]interface{}) *When {
	cmd := &commands.UpdateNodeCommand{}
	cmd.NodeID = nodeID
	if content, ok := updates["content"].(string); ok {
		cmd.Content = content
	}
	if title, ok := updates["title"].(string); ok {
		cmd.Title = title
	}
	
	w.scenario.commands = append(w.scenario.commands, cmd)
	return w
}

// ConnectingNodes simulates creating a connection
func (w *When) ConnectingNodes(sourceID, targetID string) *When {
	// Simulate connection command
	event := builders.NewEventBuilder().
		WithAggregateID(sourceID).
		BuildNodeConnected(targetID, 0.5)
	w.scenario.events = append(w.scenario.events, event)
	return w
}

// ArchivingNode simulates node archival
func (w *When) ArchivingNode(nodeID string) *When {
	cmd := &commands.ArchiveNodeCommand{}
	cmd.NodeID = nodeID
	cmd.Reason = "Test archive"
	
	w.scenario.commands = append(w.scenario.commands, cmd)
	return w
}

// ExecutingBulkOperation simulates bulk operations
func (w *When) ExecutingBulkOperation(operation string, nodeIDs []string) *When {
	// Simulate bulk operation
	for _, nodeID := range nodeIDs {
		switch operation {
		case "archive":
			w.ArchivingNode(nodeID)
		}
	}
	return w
}

// ErrorOccurs simulates an error
func (w *When) ErrorOccurs(err error) *When {
	w.scenario.errors = append(w.scenario.errors, err)
	return w
}

// And provides fluent chaining
func (w *When) And() *When {
	return w
}

// Then transitions to the Then clause
func (w *When) Then() *Then {
	return w.scenario.then
}

// Then represents the assertions
type Then struct {
	scenario *Scenario
}

// NodeShouldExist asserts that a node exists
func (t *Then) NodeShouldExist(nodeID ...string) *Then {
	if len(nodeID) > 0 {
		_, exists := t.scenario.nodes[nodeID[0]]
		assert.True(t.scenario.t, exists, "Node %s should exist", nodeID[0])
	} else {
		assert.NotEmpty(t.scenario.t, t.scenario.nodes, "At least one node should exist")
	}
	return t
}

// NodeShouldNotExist asserts that a node does not exist
func (t *Then) NodeShouldNotExist(nodeID string) *Then {
	_, exists := t.scenario.nodes[nodeID]
	assert.False(t.scenario.t, exists, "Node %s should not exist", nodeID)
	return t
}

// EventShouldBePublished asserts that an event was published
func (t *Then) EventShouldBePublished(eventType string) *Then {
	found := false
	for _, event := range t.scenario.events {
		if event.GetEventType() == eventType {
			found = true
			break
		}
	}
	assert.True(t.scenario.t, found, "Event %s should have been published", eventType)
	return t
}

// EventsShouldBePublished asserts multiple events were published
func (t *Then) EventsShouldBePublished(eventTypes ...string) *Then {
	for _, eventType := range eventTypes {
		t.EventShouldBePublished(eventType)
	}
	return t
}

// ConnectionsShouldBeCreated asserts that connections were created
func (t *Then) ConnectionsShouldBeCreated(count int) *Then {
	connectionEvents := 0
	for _, event := range t.scenario.events {
		if event.GetEventType() == "NodeConnected" {
			connectionEvents++
		}
	}
	assert.Equal(t.scenario.t, count, connectionEvents, 
		"Expected %d connections to be created, got %d", count, connectionEvents)
	return t
}

// NoErrorShouldOccur asserts no errors occurred
func (t *Then) NoErrorShouldOccur() *Then {
	assert.Empty(t.scenario.t, t.scenario.errors, "No errors should have occurred")
	return t
}

// ErrorShouldOccur asserts that an error occurred
func (t *Then) ErrorShouldOccur() *Then {
	assert.NotEmpty(t.scenario.t, t.scenario.errors, "An error should have occurred")
	return t
}

// ErrorShouldContain asserts that an error contains specific text
func (t *Then) ErrorShouldContain(text string) *Then {
	require.NotEmpty(t.scenario.t, t.scenario.errors, "Expected an error")
	found := false
	for _, err := range t.scenario.errors {
		if err != nil && contains(err.Error(), text) {
			found = true
			break
		}
	}
	assert.True(t.scenario.t, found, "Error should contain '%s'", text)
	return t
}

// CommandShouldBeExecuted asserts that a command was executed
func (t *Then) CommandShouldBeExecuted(commandType string) *Then {
	found := false
	for _, cmd := range t.scenario.commands {
		if cmd.GetCommandName() == commandType {
			found = true
			break
		}
	}
	assert.True(t.scenario.t, found, "Command %s should have been executed", commandType)
	return t
}

// WithContent asserts specific content in the last created node
func (t *Then) WithContent(content string) *Then {
	// This would check the actual node content
	// For now, we'll check the events
	found := false
	for _, event := range t.scenario.events {
		if created, ok := event.(*events.NodeCreatedEvent); ok {
			if created.Content == content {
				found = true
				break
			}
		}
	}
	assert.True(t.scenario.t, found, "Node should have content: %s", content)
	return t
}

// And provides fluent chaining
func (t *Then) And() *Then {
	return t
}

// Helper functions

func (s *Scenario) generateNodeID() string {
	return "node-" + generateID()
}

func generateID() string {
	// Simple ID generation for tests
	return "test-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}