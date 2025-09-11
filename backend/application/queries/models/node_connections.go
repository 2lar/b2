package models

import (
	"time"
)

// NodeConnectionsReadModel is optimized for viewing node connections
// This model provides fast access to a node's relationships and network
type NodeConnectionsReadModel struct {
	NodeID          string              `json:"node_id"`
	NodeTitle       string              `json:"node_title"`
	GraphID         string              `json:"graph_id"`
	IncomingEdges   []EdgeConnection    `json:"incoming_edges"`
	OutgoingEdges   []EdgeConnection    `json:"outgoing_edges"`
	TotalDegree     int                 `json:"total_degree"`
	InDegree        int                 `json:"in_degree"`
	OutDegree       int                 `json:"out_degree"`
	ConnectedNodes  []ConnectedNode     `json:"connected_nodes"`
	ConnectionStats ConnectionStatistics `json:"connection_statistics"`
	Suggestions     []ConnectionSuggestion `json:"suggested_connections"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

// EdgeConnection represents a single edge connection
type EdgeConnection struct {
	EdgeID        string    `json:"edge_id"`
	NodeID        string    `json:"node_id"`
	NodeTitle     string    `json:"node_title"`
	EdgeType      string    `json:"edge_type"`
	Weight        float64   `json:"weight"`
	Direction     string    `json:"direction"` // incoming, outgoing, bidirectional
	CreatedAt     time.Time `json:"created_at"`
}

// ConnectedNode represents a node connected to the current node
type ConnectedNode struct {
	NodeID       string   `json:"node_id"`
	Title        string   `json:"title"`
	Tags         []string `json:"tags"`
	Distance     int      `json:"distance"` // Number of hops from source
	EdgeTypes    []string `json:"edge_types"`
	TotalWeight  float64  `json:"total_weight"`
	SharedTags   []string `json:"shared_tags"`
	Similarity   float64  `json:"similarity_score"`
}

// ConnectionStatistics provides statistics about connections
type ConnectionStatistics struct {
	AverageWeight       float64           `json:"average_weight"`
	StrongestConnection EdgeConnection    `json:"strongest_connection"`
	WeakestConnection   EdgeConnection    `json:"weakest_connection"`
	EdgeTypeDistribution map[string]int   `json:"edge_type_distribution"`
	ClusteringCoefficient float64         `json:"clustering_coefficient"`
	CommonNeighbors     []string          `json:"common_neighbors"`
}

// ConnectionSuggestion represents a suggested new connection
type ConnectionSuggestion struct {
	TargetNodeID   string   `json:"target_node_id"`
	TargetTitle    string   `json:"target_title"`
	SimilarityScore float64 `json:"similarity_score"`
	Reason         string   `json:"reason"`
	CommonTags     []string `json:"common_tags"`
	SuggestedType  string   `json:"suggested_edge_type"`
	SuggestedWeight float64 `json:"suggested_weight"`
}

// NodeNetworkReadModel represents a node's extended network
type NodeNetworkReadModel struct {
	CenterNode      NodeConnectionsReadModel `json:"center_node"`
	NetworkLayers   []NetworkLayer           `json:"network_layers"`
	TotalNodes      int                      `json:"total_nodes"`
	TotalEdges      int                      `json:"total_edges"`
	NetworkDensity  float64                  `json:"network_density"`
	Paths           []PathInfo               `json:"important_paths"`
}

// NetworkLayer represents a layer of nodes at a certain distance
type NetworkLayer struct {
	Distance int              `json:"distance"`
	Nodes    []ConnectedNode  `json:"nodes"`
	EdgeCount int             `json:"edge_count"`
}

// PathInfo represents information about a path in the network
type PathInfo struct {
	StartNodeID string   `json:"start_node_id"`
	EndNodeID   string   `json:"end_node_id"`
	Path        []string `json:"path"`
	Length      int      `json:"length"`
	TotalWeight float64  `json:"total_weight"`
	PathType    string   `json:"path_type"` // shortest, strongest, etc.
}

// NodeSearchReadModel is optimized for node search results
type NodeSearchReadModel struct {
	Query       string             `json:"query"`
	Results     []NodeSearchResult `json:"results"`
	TotalCount  int                `json:"total_count"`
	Facets      SearchFacets       `json:"facets"`
	PageInfo    PageInfo           `json:"page_info"`
	SearchTime  int64              `json:"search_time_ms"`
}

// NodeSearchResult represents a single search result
type NodeSearchResult struct {
	NodeID      string                 `json:"node_id"`
	GraphID     string                 `json:"graph_id"`
	Title       string                 `json:"title"`
	ContentSnippet string              `json:"content_snippet"`
	Tags        []string               `json:"tags"`
	Score       float64                `json:"relevance_score"`
	Highlights  map[string][]string    `json:"highlights"` // field -> highlighted snippets
	Position    Position3D             `json:"position"`
	Connections int                    `json:"connection_count"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// SearchFacets provides faceted search information
type SearchFacets struct {
	Tags        map[string]int `json:"tags"`
	Graphs      map[string]int `json:"graphs"`
	EdgeTypes   map[string]int `json:"edge_types"`
	DateRanges  DateFacets     `json:"date_ranges"`
}

// DateFacets represents date-based facets
type DateFacets struct {
	Today      int `json:"today"`
	ThisWeek   int `json:"this_week"`
	ThisMonth  int `json:"this_month"`
	ThisYear   int `json:"this_year"`
	Older      int `json:"older"`
}

// BulkNodeReadModel is optimized for bulk node operations
type BulkNodeReadModel struct {
	Nodes       []NodeDetailReadModel `json:"nodes"`
	TotalCount  int                   `json:"total_count"`
	GraphID     string                `json:"graph_id"`
	LastUpdated time.Time             `json:"last_updated"`
}

// NodeHistoryReadModel tracks node changes over time
type NodeHistoryReadModel struct {
	NodeID      string           `json:"node_id"`
	CurrentVersion NodeDetailReadModel `json:"current_version"`
	History     []NodeVersion    `json:"history"`
	TotalVersions int            `json:"total_versions"`
}

// NodeVersion represents a historical version of a node
type NodeVersion struct {
	Version     int                    `json:"version"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Tags        []string               `json:"tags"`
	Position    Position3D             `json:"position"`
	ChangedBy   string                 `json:"changed_by"`
	ChangedAt   time.Time              `json:"changed_at"`
	ChangeType  string                 `json:"change_type"` // created, updated, moved, tagged
	Changes     map[string]interface{} `json:"changes"`
}

// NodeRelationshipMatrix represents relationships between multiple nodes
type NodeRelationshipMatrix struct {
	NodeIDs     []string              `json:"node_ids"`
	NodeTitles  map[string]string     `json:"node_titles"`
	Matrix      [][]RelationshipCell  `json:"relationship_matrix"`
	Statistics  MatrixStatistics      `json:"statistics"`
}

// RelationshipCell represents a cell in the relationship matrix
type RelationshipCell struct {
	HasEdge      bool    `json:"has_edge"`
	EdgeType     string  `json:"edge_type,omitempty"`
	Weight       float64 `json:"weight,omitempty"`
	Bidirectional bool   `json:"bidirectional,omitempty"`
}

// MatrixStatistics provides statistics about the relationship matrix
type MatrixStatistics struct {
	TotalPossibleEdges int     `json:"total_possible_edges"`
	ActualEdges        int     `json:"actual_edges"`
	Density            float64 `json:"density"`
	AverageWeight      float64 `json:"average_weight"`
	StrongConnections  int     `json:"strong_connections"`
	WeakConnections    int     `json:"weak_connections"`
}