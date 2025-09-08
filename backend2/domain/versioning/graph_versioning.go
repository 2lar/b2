package versioning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"backend2/domain/core/aggregates"
	"backend2/domain/core/entities"
)

// GraphVersion represents a specific version of a graph
type GraphVersion struct {
	GraphID     string    `json:"graph_id"`
	Version     int       `json:"version"`
	Checksum    string    `json:"checksum"`
	NodeCount   int       `json:"node_count"`
	EdgeCount   int       `json:"edge_count"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
	Description string    `json:"description"`
	Metadata    Metadata  `json:"metadata"`
}

// Metadata contains additional version information
type Metadata struct {
	Tags       []string               `json:"tags,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Changes    []Change               `json:"changes,omitempty"`
}

// Change represents a change in this version
type Change struct {
	Type        ChangeType `json:"type"`
	EntityID    string     `json:"entity_id"`
	Description string     `json:"description"`
	Timestamp   time.Time  `json:"timestamp"`
}

// ChangeType represents the type of change
type ChangeType string

const (
	ChangeTypeNodeAdded   ChangeType = "node_added"
	ChangeTypeNodeRemoved ChangeType = "node_removed"
	ChangeTypeNodeUpdated ChangeType = "node_updated"
	ChangeTypeEdgeAdded   ChangeType = "edge_added"
	ChangeTypeEdgeRemoved ChangeType = "edge_removed"
	ChangeTypeEdgeUpdated ChangeType = "edge_updated"
	ChangeTypeMetadata    ChangeType = "metadata_updated"
)

// VersioningService manages graph versions
type VersioningService struct {
	maxVersions int
	autoVersion bool
}

// NewVersioningService creates a new versioning service
func NewVersioningService(maxVersions int, autoVersion bool) *VersioningService {
	return &VersioningService{
		maxVersions: maxVersions,
		autoVersion: autoVersion,
	}
}

// CreateVersion creates a new version of a graph
func (s *VersioningService) CreateVersion(
	graph *aggregates.Graph,
	userID string,
	description string,
) (*GraphVersion, error) {
	if graph == nil {
		return nil, fmt.Errorf("graph cannot be nil")
	}

	// Calculate checksum
	checksum, err := s.calculateChecksum(graph)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Get nodes count safely
	nodes, err := graph.GetNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes for version: %w", err)
	}

	// Create version
	version := &GraphVersion{
		GraphID:     graph.ID().String(),
		Version:     graph.Version() + 1,
		Checksum:    checksum,
		NodeCount:   len(nodes),
		EdgeCount:   len(graph.GetEdges()),
		CreatedAt:   time.Now(),
		CreatedBy:   userID,
		Description: description,
		Metadata: Metadata{
			Tags:       []string{},
			Properties: make(map[string]interface{}),
			Changes:    []Change{},
		},
	}

	return version, nil
}

// CompareVersions compares two graph versions
func (s *VersioningService) CompareVersions(v1, v2 *GraphVersion) (*VersionDiff, error) {
	if v1 == nil || v2 == nil {
		return nil, fmt.Errorf("versions cannot be nil")
	}

	diff := &VersionDiff{
		FromVersion: v1.Version,
		ToVersion:   v2.Version,
		NodesDiff: NodesDiff{
			Added:   v2.NodeCount - v1.NodeCount,
			Removed: 0,
			Updated: 0,
		},
		EdgesDiff: EdgesDiff{
			Added:   v2.EdgeCount - v1.EdgeCount,
			Removed: 0,
			Updated: 0,
		},
		TimeDiff: v2.CreatedAt.Sub(v1.CreatedAt),
	}

	// Analyze changes for more detailed diff
	for _, change := range v2.Metadata.Changes {
		switch change.Type {
		case ChangeTypeNodeAdded:
			diff.NodesDiff.Added++
		case ChangeTypeNodeRemoved:
			diff.NodesDiff.Removed++
		case ChangeTypeNodeUpdated:
			diff.NodesDiff.Updated++
		case ChangeTypeEdgeAdded:
			diff.EdgesDiff.Added++
		case ChangeTypeEdgeRemoved:
			diff.EdgesDiff.Removed++
		case ChangeTypeEdgeUpdated:
			diff.EdgesDiff.Updated++
		}
	}

	return diff, nil
}

// RestoreVersion restores a graph to a specific version
func (s *VersioningService) RestoreVersion(
	version *GraphVersion,
	currentGraph *aggregates.Graph,
) error {
	if version == nil || currentGraph == nil {
		return fmt.Errorf("version and graph cannot be nil")
	}

	// Verify the version belongs to the same graph
	if version.GraphID != currentGraph.ID().String() {
		return fmt.Errorf("version does not belong to this graph")
	}

	// Note: Actual restoration would require storing the full graph state
	// This is a simplified implementation
	return fmt.Errorf("restore not implemented: requires full graph state storage")
}

// calculateChecksum calculates a checksum for a graph
func (s *VersioningService) calculateChecksum(graph *aggregates.Graph) (string, error) {
	// Get nodes safely
	nodes, err := graph.GetNodes()
	if err != nil {
		return "", fmt.Errorf("failed to get nodes for checksum: %w", err)
	}

	// Create a deterministic representation of the graph
	data := struct {
		ID        string           `json:"id"`
		Nodes     []*entities.Node `json:"nodes"`
		EdgeCount int              `json:"edge_count"`
	}{
		ID:        graph.ID().String(),
		Nodes:     nodes,
		EdgeCount: len(graph.GetEdges()),
	}

	// Marshal to JSON for consistent representation
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:]), nil
}

// VersionDiff represents the difference between two versions
type VersionDiff struct {
	FromVersion int           `json:"from_version"`
	ToVersion   int           `json:"to_version"`
	NodesDiff   NodesDiff     `json:"nodes_diff"`
	EdgesDiff   EdgesDiff     `json:"edges_diff"`
	TimeDiff    time.Duration `json:"time_diff"`
}

// NodesDiff represents changes in nodes
type NodesDiff struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Updated int `json:"updated"`
}

// EdgesDiff represents changes in edges
type EdgesDiff struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Updated int `json:"updated"`
}

// VersioningPolicy defines versioning behavior
type VersioningPolicy struct {
	AutoVersion        bool          `json:"auto_version"`
	MaxVersions        int           `json:"max_versions"`
	RetentionPeriod    time.Duration `json:"retention_period"`
	VersionOnNodeCount int           `json:"version_on_node_count"`
	VersionOnTimeElapsed time.Duration `json:"version_on_time_elapsed"`
}

// DefaultVersioningPolicy returns the default versioning policy
func DefaultVersioningPolicy() VersioningPolicy {
	return VersioningPolicy{
		AutoVersion:        true,
		MaxVersions:        10,
		RetentionPeriod:    30 * 24 * time.Hour, // 30 days
		VersionOnNodeCount: 100,                  // Version every 100 nodes
		VersionOnTimeElapsed: 24 * time.Hour,     // Version daily
	}
}

// ShouldCreateVersion determines if a new version should be created
func (p *VersioningPolicy) ShouldCreateVersion(
	lastVersion *GraphVersion,
	currentNodeCount int,
	currentTime time.Time,
) bool {
	if !p.AutoVersion {
		return false
	}

	if lastVersion == nil {
		return true
	}

	// Check node count threshold
	if currentNodeCount-lastVersion.NodeCount >= p.VersionOnNodeCount {
		return true
	}

	// Check time threshold
	if currentTime.Sub(lastVersion.CreatedAt) >= p.VersionOnTimeElapsed {
		return true
	}

	return false
}