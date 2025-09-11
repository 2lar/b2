package services

import (
	"backend/domain/core/aggregates"
	"backend/domain/core/valueobjects"
	pkgerrors "backend/pkg/errors"
)

// GraphAnalyticsService handles complex graph analysis operations
// This service extracts analytical operations from the Graph aggregate
// to maintain single responsibility principle
type GraphAnalyticsService struct {
	// Could add configuration or dependencies here
}

// NewGraphAnalyticsService creates a new graph analytics service
func NewGraphAnalyticsService() *GraphAnalyticsService {
	return &GraphAnalyticsService{}
}

// FindPath finds a path between two nodes using BFS algorithm
// Returns the shortest path as a slice of node IDs
func (s *GraphAnalyticsService) FindPath(
	graph *aggregates.Graph,
	startID, endID valueobjects.NodeID,
) ([]valueobjects.NodeID, error) {
	// Validate nodes exist
	nodes, err := graph.Nodes()
	if err != nil {
		return nil, err
	}

	if _, exists := nodes[startID]; !exists {
		return nil, pkgerrors.NewNotFoundError("start node")
	}
	if _, exists := nodes[endID]; !exists {
		return nil, pkgerrors.NewNotFoundError("end node")
	}

	if startID.Equals(endID) {
		return []valueobjects.NodeID{startID}, nil
	}

	// BFS implementation
	visited := make(map[valueobjects.NodeID]bool)
	parent := make(map[valueobjects.NodeID]valueobjects.NodeID)
	queue := []valueobjects.NodeID{startID}
	visited[startID] = true

	edges := graph.Edges()

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Check all edges from current node
		for _, edge := range edges {
			var next valueobjects.NodeID

			if edge.SourceID.Equals(current) {
				next = edge.TargetID
			} else if edge.Bidirectional && edge.TargetID.Equals(current) {
				next = edge.SourceID
			} else {
				continue
			}

			if !visited[next] {
				visited[next] = true
				parent[next] = current
				queue = append(queue, next)

				if next.Equals(endID) {
					// Reconstruct path
					return s.reconstructPath(startID, endID, parent), nil
				}
			}
		}
	}

	return nil, pkgerrors.NewNotFoundError("path between nodes")
}

// GetClusters identifies clusters of connected nodes
// Returns groups of node IDs that are connected to each other
func (s *GraphAnalyticsService) GetClusters(graph *aggregates.Graph) ([][]valueobjects.NodeID, error) {
	nodes, err := graph.Nodes()
	if err != nil {
		return nil, err
	}

	visited := make(map[valueobjects.NodeID]bool)
	var clusters [][]valueobjects.NodeID

	for nodeID := range nodes {
		if !visited[nodeID] {
			cluster := s.dfs(graph, nodeID, visited)
			clusters = append(clusters, cluster)
		}
	}

	return clusters, nil
}

// GetNodeDegree calculates the degree (number of connections) for a node
func (s *GraphAnalyticsService) GetNodeDegree(
	graph *aggregates.Graph,
	nodeID valueobjects.NodeID,
) (inDegree, outDegree int, err error) {
	nodes, err := graph.Nodes()
	if err != nil {
		return 0, 0, err
	}

	if _, exists := nodes[nodeID]; !exists {
		return 0, 0, pkgerrors.NewNotFoundError("node")
	}

	edges := graph.Edges()
	for _, edge := range edges {
		if edge.SourceID.Equals(nodeID) {
			outDegree++
		}
		if edge.TargetID.Equals(nodeID) {
			inDegree++
		}
		// Handle bidirectional edges
		if edge.Bidirectional {
			if edge.SourceID.Equals(nodeID) {
				inDegree++
			}
			if edge.TargetID.Equals(nodeID) {
				outDegree++
			}
		}
	}

	return inDegree, outDegree, nil
}

// GetConnectedNodes finds all nodes connected to a given node up to a certain depth
func (s *GraphAnalyticsService) GetConnectedNodes(
	graph *aggregates.Graph,
	nodeID valueobjects.NodeID,
	maxDepth int,
) ([]valueobjects.NodeID, error) {
	nodes, err := graph.Nodes()
	if err != nil {
		return nil, err
	}

	if _, exists := nodes[nodeID]; !exists {
		return nil, pkgerrors.NewNotFoundError("node")
	}

	if maxDepth <= 0 {
		return []valueobjects.NodeID{}, nil
	}

	visited := make(map[valueobjects.NodeID]int) // Maps node to its depth
	visited[nodeID] = 0
	queue := []valueobjects.NodeID{nodeID}
	var result []valueobjects.NodeID

	edges := graph.Edges()

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		currentDepth := visited[current]

		if currentDepth >= maxDepth {
			continue
		}

		// Check all edges from current node
		for _, edge := range edges {
			var next valueobjects.NodeID

			if edge.SourceID.Equals(current) {
				next = edge.TargetID
			} else if edge.Bidirectional && edge.TargetID.Equals(current) {
				next = edge.SourceID
			} else {
				continue
			}

			if _, alreadyVisited := visited[next]; !alreadyVisited {
				visited[next] = currentDepth + 1
				queue = append(queue, next)
				result = append(result, next)
			}
		}
	}

	return result, nil
}

// FindOrphanedNodes identifies nodes with no connections
func (s *GraphAnalyticsService) FindOrphanedNodes(graph *aggregates.Graph) ([]valueobjects.NodeID, error) {
	nodes, err := graph.Nodes()
	if err != nil {
		return nil, err
	}

	// Track nodes that have connections
	hasConnections := make(map[valueobjects.NodeID]bool)

	edges := graph.Edges()
	for _, edge := range edges {
		hasConnections[edge.SourceID] = true
		hasConnections[edge.TargetID] = true
	}

	// Find nodes without connections
	var orphaned []valueobjects.NodeID
	for nodeID := range nodes {
		if !hasConnections[nodeID] {
			orphaned = append(orphaned, nodeID)
		}
	}

	return orphaned, nil
}

// CalculateCentrality calculates the betweenness centrality for nodes
// This identifies important nodes that act as bridges in the graph
func (s *GraphAnalyticsService) CalculateCentrality(
	graph *aggregates.Graph,
) (map[valueobjects.NodeID]float64, error) {
	nodes, err := graph.Nodes()
	if err != nil {
		return nil, err
	}

	centrality := make(map[valueobjects.NodeID]float64)

	// Initialize all centralities to 0
	for nodeID := range nodes {
		centrality[nodeID] = 0.0
	}

	// For each pair of nodes, find shortest paths and update centrality
	nodeList := make([]valueobjects.NodeID, 0, len(nodes))
	for nodeID := range nodes {
		nodeList = append(nodeList, nodeID)
	}

	for i, source := range nodeList {
		for j, target := range nodeList {
			if i >= j {
				continue // Skip same node and already processed pairs
			}

			path, err := s.FindPath(graph, source, target)
			if err != nil {
				continue // No path exists
			}

			// Update centrality for intermediate nodes
			for k := 1; k < len(path)-1; k++ {
				centrality[path[k]] += 1.0
			}
		}
	}

	// Normalize centrality values
	maxCentrality := 0.0
	for _, value := range centrality {
		if value > maxCentrality {
			maxCentrality = value
		}
	}

	if maxCentrality > 0 {
		for nodeID := range centrality {
			centrality[nodeID] /= maxCentrality
		}
	}

	return centrality, nil
}

// Private helper methods

func (s *GraphAnalyticsService) dfs(
	graph *aggregates.Graph,
	nodeID valueobjects.NodeID,
	visited map[valueobjects.NodeID]bool,
) []valueobjects.NodeID {
	cluster := []valueobjects.NodeID{nodeID}
	visited[nodeID] = true

	edges := graph.Edges()
	for _, edge := range edges {
		var next valueobjects.NodeID

		if edge.SourceID.Equals(nodeID) {
			next = edge.TargetID
		} else if edge.Bidirectional && edge.TargetID.Equals(nodeID) {
			next = edge.SourceID
		} else {
			continue
		}

		if !visited[next] {
			cluster = append(cluster, s.dfs(graph, next, visited)...)
		}
	}

	return cluster
}

func (s *GraphAnalyticsService) reconstructPath(
	startID, endID valueobjects.NodeID,
	parent map[valueobjects.NodeID]valueobjects.NodeID,
) []valueobjects.NodeID {
	path := []valueobjects.NodeID{}
	for n := endID; !n.IsZero(); n = parent[n] {
		path = append([]valueobjects.NodeID{n}, path...)
		if n.Equals(startID) {
			break
		}
	}
	return path
}