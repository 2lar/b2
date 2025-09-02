// Package builders provides test builders for creating domain objects in tests
package builders

import (
	"time"

	"brain2-backend/internal/core/domain/valueobjects"
	"github.com/google/uuid"
)

// EdgeBuilder provides a fluent interface for building test edges
type EdgeBuilder struct {
	id       valueobjects.EdgeID
	userID   valueobjects.UserID
	sourceID valueobjects.NodeID
	targetID valueobjects.NodeID
	weight   float64
	metadata map[string]interface{}
	createdAt time.Time
	updatedAt time.Time
}

// NewEdgeBuilder creates a new edge builder with sensible defaults
func NewEdgeBuilder() *EdgeBuilder {
	return &EdgeBuilder{
		id:       valueobjects.NewEdgeID(uuid.New().String()),
		userID:   valueobjects.NewUserID("test-user-" + uuid.New().String()),
		sourceID: valueobjects.NewNodeID("source-" + uuid.New().String()),
		targetID: valueobjects.NewNodeID("target-" + uuid.New().String()),
		weight:   0.5,
		metadata: make(map[string]interface{}),
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
}

// WithID sets the edge ID
func (b *EdgeBuilder) WithID(id string) *EdgeBuilder {
	b.id = valueobjects.NewEdgeID(id)
	return b
}

// WithUserID sets the user ID
func (b *EdgeBuilder) WithUserID(userID string) *EdgeBuilder {
	b.userID = valueobjects.NewUserID(userID)
	return b
}

// WithSource sets the source node ID
func (b *EdgeBuilder) WithSource(nodeID string) *EdgeBuilder {
	b.sourceID = valueobjects.NewNodeID(nodeID)
	return b
}

// WithTarget sets the target node ID
func (b *EdgeBuilder) WithTarget(nodeID string) *EdgeBuilder {
	b.targetID = valueobjects.NewNodeID(nodeID)
	return b
}

// WithWeight sets the edge weight
func (b *EdgeBuilder) WithWeight(weight float64) *EdgeBuilder {
	b.weight = weight
	return b
}

// WithMetadata adds metadata key-value pairs
func (b *EdgeBuilder) WithMetadata(key string, value interface{}) *EdgeBuilder {
	b.metadata[key] = value
	return b
}

// WithCreatedAt sets the creation timestamp
func (b *EdgeBuilder) WithCreatedAt(t time.Time) *EdgeBuilder {
	b.createdAt = t
	return b
}

// WithUpdatedAt sets the update timestamp
func (b *EdgeBuilder) WithUpdatedAt(t time.Time) *EdgeBuilder {
	b.updatedAt = t
	return b
}

// Build creates the edge (returns a map representation for now)
// In a real implementation, this would return an Edge domain object
func (b *EdgeBuilder) Build() map[string]interface{} {
	return map[string]interface{}{
		"id":        b.id.String(),
		"userID":    b.userID.String(),
		"sourceID":  b.sourceID.String(),
		"targetID":  b.targetID.String(),
		"weight":    b.weight,
		"metadata":  b.metadata,
		"createdAt": b.createdAt,
		"updatedAt": b.updatedAt,
	}
}

// EdgeBuilderPresets provides common edge configurations
type EdgeBuilderPresets struct{}

// NewEdgeBuilderPresets creates a new presets helper
func NewEdgeBuilderPresets() *EdgeBuilderPresets {
	return &EdgeBuilderPresets{}
}

// SimpleEdge creates a basic edge between two nodes
func (p *EdgeBuilderPresets) SimpleEdge(userID, sourceID, targetID string) map[string]interface{} {
	return NewEdgeBuilder().
		WithUserID(userID).
		WithSource(sourceID).
		WithTarget(targetID).
		Build()
}

// StrongConnection creates an edge with high weight
func (p *EdgeBuilderPresets) StrongConnection(userID, sourceID, targetID string) map[string]interface{} {
	return NewEdgeBuilder().
		WithUserID(userID).
		WithSource(sourceID).
		WithTarget(targetID).
		WithWeight(0.9).
		WithMetadata("strength", "strong").
		Build()
}

// WeakConnection creates an edge with low weight
func (p *EdgeBuilderPresets) WeakConnection(userID, sourceID, targetID string) map[string]interface{} {
	return NewEdgeBuilder().
		WithUserID(userID).
		WithSource(sourceID).
		WithTarget(targetID).
		WithWeight(0.1).
		WithMetadata("strength", "weak").
		Build()
}

// AutoConnection creates an edge marked as automatically created
func (p *EdgeBuilderPresets) AutoConnection(userID, sourceID, targetID string) map[string]interface{} {
	return NewEdgeBuilder().
		WithUserID(userID).
		WithSource(sourceID).
		WithTarget(targetID).
		WithWeight(0.5).
		WithMetadata("auto_connected", true).
		WithMetadata("algorithm", "similarity").
		Build()
}