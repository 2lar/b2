package aggregates

import (
	"time"

	"backend/domain/events"
	pkgerrors "backend/pkg/errors"
)

// GraphMetadataAggregate manages graph metadata separately from the graph structure
// This allows updating metadata without loading the entire graph
type GraphMetadataAggregate struct {
	id          GraphID
	userID      string
	name        string
	description string
	metadata    GraphMetadata
	statistics  GraphStatistics
	createdAt   time.Time
	updatedAt   time.Time
	version     int
	events      []events.DomainEvent
}

// GraphStatistics contains computed statistics about the graph
type GraphStatistics struct {
	NodeCount          int
	EdgeCount          int
	OrphanedNodeCount  int
	AverageConnections float64
	MaxConnections     int
	ClusterCount       int
	Density            float64
	LastAnalyzedAt     time.Time
}

// NewGraphMetadataAggregate creates a new metadata aggregate
func NewGraphMetadataAggregate(graphID GraphID, userID, name, description string) (*GraphMetadataAggregate, error) {
	if graphID == "" || userID == "" || name == "" {
		return nil, pkgerrors.NewValidationError("required fields missing")
	}

	now := time.Now()
	return &GraphMetadataAggregate{
		id:          graphID,
		userID:      userID,
		name:        name,
		description: description,
		metadata: GraphMetadata{
			ViewSettings: ViewSettings{
				Layout:     LayoutForceDirected,
				ShowLabels: true,
			},
		},
		statistics: GraphStatistics{
			LastAnalyzedAt: now,
		},
		createdAt: now,
		updatedAt: now,
		version:   1,
		events:    []events.DomainEvent{},
	}, nil
}

// ID returns the graph ID
func (m *GraphMetadataAggregate) ID() GraphID {
	return m.id
}

// UserID returns the owner's ID
func (m *GraphMetadataAggregate) UserID() string {
	return m.userID
}

// Name returns the graph name
func (m *GraphMetadataAggregate) Name() string {
	return m.name
}

// Description returns the graph description
func (m *GraphMetadataAggregate) Description() string {
	return m.description
}

// UpdateName updates the graph name
func (m *GraphMetadataAggregate) UpdateName(name string) error {
	if name == "" {
		return pkgerrors.NewValidationError("name cannot be empty")
	}

	oldName := m.name
	m.name = name
	m.updatedAt = time.Now()
	m.version++

	// Add event for name change
	m.addEvent(events.DomainEvent(struct {
		events.BaseEvent
		GraphID string `json:"graph_id"`
		OldName string `json:"old_name"`
		NewName string `json:"new_name"`
	}{
		BaseEvent: events.BaseEvent{
			AggregateID: m.id.String(),
			EventType:   "graph.name_updated",
			Timestamp:   m.updatedAt,
			Version:     1,
		},
		GraphID: m.id.String(),
		OldName: oldName,
		NewName: name,
	}))

	return nil
}

// UpdateDescription updates the graph description
func (m *GraphMetadataAggregate) UpdateDescription(description string) error {
	m.description = description
	m.updatedAt = time.Now()
	m.version++
	return nil
}

// UpdateViewSettings updates the display preferences
func (m *GraphMetadataAggregate) UpdateViewSettings(settings ViewSettings) error {
	m.metadata.ViewSettings = settings
	m.updatedAt = time.Now()
	m.version++
	return nil
}

// UpdateTags updates the graph tags
func (m *GraphMetadataAggregate) UpdateTags(tags []string) error {
	if len(tags) > 20 {
		return pkgerrors.NewValidationError("too many tags (max 20)")
	}

	m.metadata.Tags = tags
	m.updatedAt = time.Now()
	m.version++
	return nil
}

// SetPublic sets the graph visibility
func (m *GraphMetadataAggregate) SetPublic(isPublic bool) error {
	m.metadata.IsPublic = isPublic
	m.updatedAt = time.Now()
	m.version++

	// Add event for visibility change
	m.addEvent(events.DomainEvent(struct {
		events.BaseEvent
		GraphID  string `json:"graph_id"`
		IsPublic bool   `json:"is_public"`
	}{
		BaseEvent: events.BaseEvent{
			AggregateID: m.id.String(),
			EventType:   "graph.visibility_changed",
			Timestamp:   m.updatedAt,
			Version:     1,
		},
		GraphID:  m.id.String(),
		IsPublic: isPublic,
	}))

	return nil
}

// UpdateStatistics updates the computed statistics
func (m *GraphMetadataAggregate) UpdateStatistics(stats GraphStatistics) error {
	m.statistics = stats
	m.statistics.LastAnalyzedAt = time.Now()
	m.updatedAt = time.Now()
	m.version++
	return nil
}

// UpdateNodeCount updates just the node count
func (m *GraphMetadataAggregate) UpdateNodeCount(count int) error {
	if count < 0 {
		return pkgerrors.NewValidationError("node count cannot be negative")
	}

	m.metadata.NodeCount = count
	m.statistics.NodeCount = count
	m.updatedAt = time.Now()
	m.version++
	return nil
}

// UpdateEdgeCount updates just the edge count
func (m *GraphMetadataAggregate) UpdateEdgeCount(count int) error {
	if count < 0 {
		return pkgerrors.NewValidationError("edge count cannot be negative")
	}

	m.metadata.EdgeCount = count
	m.statistics.EdgeCount = count
	m.updatedAt = time.Now()
	m.version++
	return nil
}

// GetMetadata returns the graph metadata
func (m *GraphMetadataAggregate) GetMetadata() GraphMetadata {
	return m.metadata
}

// GetStatistics returns the graph statistics
func (m *GraphMetadataAggregate) GetStatistics() GraphStatistics {
	return m.statistics
}

// GetViewSettings returns the display preferences
func (m *GraphMetadataAggregate) GetViewSettings() ViewSettings {
	return m.metadata.ViewSettings
}

// IsPublic returns whether the graph is public
func (m *GraphMetadataAggregate) IsPublic() bool {
	return m.metadata.IsPublic
}

// GetTags returns the graph tags
func (m *GraphMetadataAggregate) GetTags() []string {
	return m.metadata.Tags
}

// Version returns the aggregate version for optimistic locking
func (m *GraphMetadataAggregate) Version() int {
	return m.version
}

// CreatedAt returns when the metadata was created
func (m *GraphMetadataAggregate) CreatedAt() time.Time {
	return m.createdAt
}

// UpdatedAt returns when the metadata was last updated
func (m *GraphMetadataAggregate) UpdatedAt() time.Time {
	return m.updatedAt
}

// GetUncommittedEvents returns all uncommitted domain events
func (m *GraphMetadataAggregate) GetUncommittedEvents() []events.DomainEvent {
	return m.events
}

// MarkEventsAsCommitted clears all uncommitted events
func (m *GraphMetadataAggregate) MarkEventsAsCommitted() {
	m.events = []events.DomainEvent{}
}

// Private helper methods

func (m *GraphMetadataAggregate) addEvent(event events.DomainEvent) {
	m.events = append(m.events, event)
}

// ReconstructGraphMetadata recreates metadata from stored data
func ReconstructGraphMetadata(
	id string,
	userID string,
	name string,
	description string,
	metadata GraphMetadata,
	statistics GraphStatistics,
	createdAt time.Time,
	updatedAt time.Time,
	version int,
) (*GraphMetadataAggregate, error) {
	if id == "" || userID == "" || name == "" {
		return nil, pkgerrors.NewValidationError("required fields missing")
	}

	return &GraphMetadataAggregate{
		id:          GraphID(id),
		userID:      userID,
		name:        name,
		description: description,
		metadata:    metadata,
		statistics:  statistics,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
		version:     version,
		events:      []events.DomainEvent{},
	}, nil
}