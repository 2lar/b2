// Package graph provides graph analysis implementations
package graph

import (
	"context"
	"fmt"
	
	"brain2-backend/internal/core/application/ports"
)

// SimpleGraphAnalyzer provides basic graph analysis capabilities
type SimpleGraphAnalyzer struct {
	nodeRepo ports.NodeRepository
	edgeRepo ports.EdgeRepository
	logger   ports.Logger
}

// NewSimpleGraphAnalyzer creates a new graph analyzer
func NewSimpleGraphAnalyzer(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	logger ports.Logger,
) *SimpleGraphAnalyzer {
	return &SimpleGraphAnalyzer{
		nodeRepo: nodeRepo,
		edgeRepo: edgeRepo,
		logger:   logger,
	}
}

// WouldCreateCycle checks if adding an edge would create a cycle
func (a *SimpleGraphAnalyzer) WouldCreateCycle(ctx context.Context, sourceID, targetID string) (bool, error) {
	// Simple implementation: check if there's already a path from target to source
	visited := make(map[string]bool)
	return a.hasCycleDFS(ctx, targetID, sourceID, visited)
}

// hasCycleDFS performs depth-first search to detect cycles
func (a *SimpleGraphAnalyzer) hasCycleDFS(ctx context.Context, current, target string, visited map[string]bool) (bool, error) {
	if current == target {
		return true, nil
	}
	
	if visited[current] {
		return false, nil
	}
	
	visited[current] = true
	
	// Get edges from current node
	edges, err := a.edgeRepo.FindEdgesByNode(ctx, current)
	if err != nil {
		return false, err
	}
	
	for _, edge := range edges {
		// Only follow outgoing edges
		if edge.SourceID == current {
			hasCycle, err := a.hasCycleDFS(ctx, edge.TargetID, target, visited)
			if err != nil {
				return false, err
			}
			if hasCycle {
				return true, nil
			}
		}
	}
	
	return false, nil
}

// UpdateCentrality updates centrality scores for nodes
func (a *SimpleGraphAnalyzer) UpdateCentrality(ctx context.Context, userID string, nodeIDs []string) error {
	// Simplified implementation - in production, would calculate degree centrality
	a.logger.Debug("Updating centrality scores",
		ports.Field{Key: "user_id", Value: userID},
		ports.Field{Key: "node_count", Value: len(nodeIDs)})
	
	// For now, just log the operation
	return nil
}

// UpdateClustering updates clustering coefficients
func (a *SimpleGraphAnalyzer) UpdateClustering(ctx context.Context, userID string, nodeID string) error {
	// Simplified implementation - in production, would calculate local clustering coefficient
	a.logger.Debug("Updating clustering coefficient",
		ports.Field{Key: "user_id", Value: userID},
		ports.Field{Key: "node_id", Value: nodeID})
	
	return nil
}

// FindShortestPath finds the shortest path between two nodes using BFS
func (a *SimpleGraphAnalyzer) FindShortestPath(ctx context.Context, sourceID, targetID string) ([]string, error) {
	if sourceID == targetID {
		return []string{sourceID}, nil
	}
	
	// BFS to find shortest path
	queue := [][]string{{sourceID}}
	visited := make(map[string]bool)
	visited[sourceID] = true
	
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		
		current := path[len(path)-1]
		
		// Get edges from current node
		edges, err := a.edgeRepo.FindEdgesByNode(ctx, current)
		if err != nil {
			return nil, err
		}
		
		for _, edge := range edges {
			var next string
			if edge.SourceID == current {
				next = edge.TargetID
			} else if edge.TargetID == current {
				next = edge.SourceID
			} else {
				continue
			}
			
			if next == targetID {
				// Found the target
				return append(path, next), nil
			}
			
			if !visited[next] {
				visited[next] = true
				newPath := make([]string, len(path)+1)
				copy(newPath, path)
				newPath[len(path)] = next
				queue = append(queue, newPath)
			}
		}
	}
	
	// No path found
	return nil, fmt.Errorf("no path found between %s and %s", sourceID, targetID)
}

// GetConnectedComponents finds connected components in the graph
func (a *SimpleGraphAnalyzer) GetConnectedComponents(ctx context.Context, userID string) ([][]string, error) {
	// This would need to fetch all nodes for the user first
	// For now, return empty as this is a placeholder
	a.logger.Debug("Getting connected components",
		ports.Field{Key: "user_id", Value: userID})
	
	return [][]string{}, nil
}