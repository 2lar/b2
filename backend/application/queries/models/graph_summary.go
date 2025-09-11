package models

import (
	"time"
)

// GraphSummaryReadModel is an optimized read model for graph summary views
// This model is designed for fast retrieval and display in dashboards/lists
type GraphSummaryReadModel struct {
	GraphID         string                 `json:"graph_id"`
	UserID          string                 `json:"user_id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	NodeCount       int                    `json:"node_count"`
	EdgeCount       int                    `json:"edge_count"`
	IsDefault       bool                   `json:"is_default"`
	IsPublic        bool                   `json:"is_public"`
	Tags            []string               `json:"tags"`
	MostConnected   []NodeSummary          `json:"most_connected_nodes"`
	RecentlyUpdated []NodeSummary          `json:"recently_updated_nodes"`
	Statistics      GraphStatistics        `json:"statistics"`
	ViewSettings    ViewSettings          `json:"view_settings"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	LastAccessedAt  *time.Time             `json:"last_accessed_at,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// NodeSummary represents a simplified node for summary views
type NodeSummary struct {
	NodeID      string    `json:"node_id"`
	Title       string    `json:"title"`
	Tags        []string  `json:"tags"`
	Connections int       `json:"connections"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GraphStatistics contains computed statistics for the graph
type GraphStatistics struct {
	AverageConnections float64            `json:"average_connections"`
	MaxConnections     int                `json:"max_connections"`
	ClusterCount       int                `json:"cluster_count"`
	Density            float64            `json:"density"` // Edge count / possible edges
	OrphanedNodes      int                `json:"orphaned_nodes"`
	EdgeTypeDistribution map[string]int   `json:"edge_type_distribution"`
	NodeTagDistribution  map[string]int   `json:"node_tag_distribution"`
}

// ViewSettings contains display preferences for the graph
type ViewSettings struct {
	Layout      string `json:"layout"`       // force_directed, hierarchical, circular, grid
	Theme       string `json:"theme"`        // light, dark, custom
	NodeSize    string `json:"node_size"`    // small, medium, large
	EdgeStyle   string `json:"edge_style"`   // straight, curved, animated
	ShowLabels  bool   `json:"show_labels"`
	ShowWeights bool   `json:"show_weights"`
}

// GraphListReadModel is optimized for listing multiple graphs
type GraphListReadModel struct {
	Graphs     []GraphSummaryReadModel `json:"graphs"`
	TotalCount int                     `json:"total_count"`
	PageInfo   PageInfo                `json:"page_info"`
	Filters    GraphFilters            `json:"filters_applied"`
}

// PageInfo contains pagination information
type PageInfo struct {
	CurrentPage  int    `json:"current_page"`
	PageSize     int    `json:"page_size"`
	TotalPages   int    `json:"total_pages"`
	HasNext      bool   `json:"has_next"`
	HasPrevious  bool   `json:"has_previous"`
	NextCursor   string `json:"next_cursor,omitempty"`
}

// GraphFilters represents applied filters
type GraphFilters struct {
	UserID       string    `json:"user_id,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	MinNodes     *int      `json:"min_nodes,omitempty"`
	MaxNodes     *int      `json:"max_nodes,omitempty"`
	IsPublic     *bool     `json:"is_public,omitempty"`
	CreatedAfter *time.Time `json:"created_after,omitempty"`
	UpdatedAfter *time.Time `json:"updated_after,omitempty"`
	SearchQuery  string    `json:"search_query,omitempty"`
}

// GraphDetailReadModel is optimized for detailed graph views
type GraphDetailReadModel struct {
	GraphSummaryReadModel
	Nodes          []NodeDetailReadModel `json:"nodes"`
	Edges          []EdgeDetailReadModel `json:"edges"`
	PathAnalysis   PathAnalysis          `json:"path_analysis"`
	CentralityData CentralityAnalysis    `json:"centrality_analysis"`
}

// NodeDetailReadModel represents detailed node information
type NodeDetailReadModel struct {
	NodeID       string                 `json:"node_id"`
	Title        string                 `json:"title"`
	Content      string                 `json:"content"`
	ContentFormat string                `json:"content_format"`
	Position     Position3D             `json:"position"`
	Tags         []string               `json:"tags"`
	Status       string                 `json:"status"`
	IncomingEdges []string              `json:"incoming_edges"`
	OutgoingEdges []string              `json:"outgoing_edges"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// EdgeDetailReadModel represents detailed edge information
type EdgeDetailReadModel struct {
	EdgeID        string                 `json:"edge_id"`
	SourceID      string                 `json:"source_id"`
	TargetID      string                 `json:"target_id"`
	Type          string                 `json:"type"`
	Weight        float64                `json:"weight"`
	Bidirectional bool                   `json:"bidirectional"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// Position3D represents a 3D position
type Position3D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// PathAnalysis contains path-related analytics
type PathAnalysis struct {
	AveragePathLength   float64           `json:"average_path_length"`
	LongestPath         []string          `json:"longest_path"`
	ShortestPaths       map[string][]string `json:"shortest_paths_sample"` // Sample of shortest paths
	UnreachablePairs    int               `json:"unreachable_pairs"`
}

// CentralityAnalysis contains centrality metrics
type CentralityAnalysis struct {
	MostCentral        []CentralityScore `json:"most_central_nodes"`
	BetweennessCentrality map[string]float64 `json:"betweenness_centrality"`
	DegreeCentrality   map[string]float64 `json:"degree_centrality"`
}

// CentralityScore represents a node's centrality score
type CentralityScore struct {
	NodeID string  `json:"node_id"`
	Score  float64 `json:"score"`
	Rank   int     `json:"rank"`
}

// GraphComparisonReadModel compares multiple graphs
type GraphComparisonReadModel struct {
	Graphs         []GraphSummaryReadModel `json:"graphs"`
	CommonNodes    []string                `json:"common_nodes"`
	UniqueNodes    map[string][]string     `json:"unique_nodes"` // GraphID -> NodeIDs
	SimilarityScore float64                `json:"similarity_score"`
	Differences    []GraphDifference       `json:"differences"`
}

// GraphDifference represents a difference between graphs
type GraphDifference struct {
	Type        string `json:"type"` // node_added, node_removed, edge_added, edge_removed
	GraphID     string `json:"graph_id"`
	EntityID    string `json:"entity_id"`
	Description string `json:"description"`
}

// GraphTimelineReadModel represents graph changes over time
type GraphTimelineReadModel struct {
	GraphID  string          `json:"graph_id"`
	Timeline []TimelineEntry `json:"timeline"`
	Growth   GrowthMetrics   `json:"growth_metrics"`
}

// TimelineEntry represents a point in the graph's timeline
type TimelineEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   string                 `json:"event_type"`
	Description string                 `json:"description"`
	NodeCount   int                    `json:"node_count"`
	EdgeCount   int                    `json:"edge_count"`
	Changes     map[string]interface{} `json:"changes,omitempty"`
}

// GrowthMetrics tracks graph growth over time
type GrowthMetrics struct {
	NodesPerDay     float64 `json:"nodes_per_day"`
	EdgesPerDay     float64 `json:"edges_per_day"`
	GrowthRate      float64 `json:"growth_rate_percentage"`
	ProjectedSize30Days int `json:"projected_size_30_days"`
}