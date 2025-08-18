// Package services provides domain services for the Brain2 backend.
// This file implements graph analysis domain logic.
package services

import (
	"math"
	"sort"
	
	"brain2-backend/internal/domain/shared"
)

// GraphAnalyzer provides graph analysis domain logic.
// This is pure domain logic with no infrastructure dependencies.
type GraphAnalyzer struct {
	similarityThreshold float64
	clusteringAlgorithm ClusteringAlgorithm
}

// ClusteringAlgorithm defines the algorithm to use for clustering.
type ClusteringAlgorithm string

const (
	ClusteringLouvain    ClusteringAlgorithm = "louvain"
	ClusteringModularity ClusteringAlgorithm = "modularity"
	ClusteringKMeans     ClusteringAlgorithm = "kmeans"
)

// NewGraphAnalyzer creates a new graph analyzer.
func NewGraphAnalyzer(similarityThreshold float64, algorithm ClusteringAlgorithm) *GraphAnalyzer {
	return &GraphAnalyzer{
		similarityThreshold: similarityThreshold,
		clusteringAlgorithm: algorithm,
	}
}

// ============================================================================
// COMMUNITY DETECTION
// ============================================================================

// FindCommunities identifies node communities in the graph.
// This implements a simplified Louvain algorithm for community detection.
func (a *GraphAnalyzer) FindCommunities(graph *Graph) []shared.Community {
	if graph == nil || len(graph.Nodes) == 0 {
		return []shared.Community{}
	}
	
	// Initialize: each node is its own community (use index as community ID)
	communities := make(map[shared.NodeID]int)
	for i, node := range graph.Nodes {
		communities[node.ID] = i
	}
	
	// Build adjacency list
	adjacency := a.buildAdjacencyList(graph)
	
	// Iteratively optimize modularity
	improved := true
	for improved {
		improved = false
		
		for _, node := range graph.Nodes {
			currentCommunity := communities[node.ID]
			bestCommunity := currentCommunity
			bestModularity := a.calculateModularity(graph, communities)
			
			// Try moving node to neighboring communities
			for _, neighbor := range adjacency[node.ID] {
				neighborCommunity := communities[neighbor]
				if neighborCommunity != currentCommunity {
					// Temporarily move node
					communities[node.ID] = neighborCommunity
					modularity := a.calculateModularity(graph, communities)
					
					if modularity > bestModularity {
						bestModularity = modularity
						bestCommunity = neighborCommunity
						improved = true
					}
				}
			}
			
			// Move to best community
			communities[node.ID] = bestCommunity
		}
	}
	
	// Convert to Community objects
	return a.communitiesToDomain(communities, graph)
}

// calculateModularity calculates the modularity of a graph partition.
func (a *GraphAnalyzer) calculateModularity(graph *Graph, communities map[shared.NodeID]int) float64 {
	if len(graph.Edges) == 0 {
		return 0
	}
	
	totalEdges := float64(len(graph.Edges))
	modularity := 0.0
	
	// Calculate degree for each node
	degrees := make(map[shared.NodeID]int)
	for _, edge := range graph.Edges {
		degrees[edge.SourceID]++
		degrees[edge.TargetID]++
	}
	
	// Calculate modularity
	for _, edge := range graph.Edges {
		if communities[edge.SourceID] == communities[edge.TargetID] {
			// Nodes in same community
			ki := float64(degrees[edge.SourceID])
			kj := float64(degrees[edge.TargetID])
			modularity += 1 - (ki*kj)/(2*totalEdges)
		}
	}
	
	return modularity / (2 * totalEdges)
}

// ============================================================================
// PAGERANK CALCULATION
// ============================================================================

// CalculatePageRank computes the PageRank for all nodes in the graph.
func (a *GraphAnalyzer) CalculatePageRank(graph *Graph) map[shared.NodeID]float64 {
	if graph == nil || len(graph.Nodes) == 0 {
		return make(map[shared.NodeID]float64)
	}
	
	const (
		dampingFactor = 0.85
		maxIterations = 100
		convergence   = 0.0001
	)
	
	nodeCount := float64(len(graph.Nodes))
	pagerank := make(map[shared.NodeID]float64)
	newPagerank := make(map[shared.NodeID]float64)
	
	// Initialize PageRank values
	for _, node := range graph.Nodes {
		pagerank[node.ID] = 1.0 / nodeCount
	}
	
	// Build inbound links map
	inboundLinks := make(map[shared.NodeID][]shared.NodeID)
	outboundCount := make(map[shared.NodeID]int)
	
	for _, edge := range graph.Edges {
		inboundLinks[edge.TargetID] = append(inboundLinks[edge.TargetID], edge.SourceID)
		outboundCount[edge.SourceID]++
	}
	
	// Iterate until convergence
	for iteration := 0; iteration < maxIterations; iteration++ {
		converged := true
		
		for _, node := range graph.Nodes {
			rank := (1 - dampingFactor) / nodeCount
			
			// Sum contributions from inbound links
			for _, source := range inboundLinks[node.ID] {
				if count := outboundCount[source]; count > 0 {
					rank += dampingFactor * pagerank[source] / float64(count)
				}
			}
			
			newPagerank[node.ID] = rank
			
			// Check convergence
			if math.Abs(newPagerank[node.ID]-pagerank[node.ID]) > convergence {
				converged = false
			}
		}
		
		// Swap maps
		pagerank, newPagerank = newPagerank, pagerank
		
		if converged {
			break
		}
	}
	
	return pagerank
}

// ============================================================================
// CENTRALITY MEASURES
// ============================================================================

// CalculateBetweennessCentrality computes betweenness centrality for nodes.
func (a *GraphAnalyzer) CalculateBetweennessCentrality(graph *Graph) map[shared.NodeID]float64 {
	centrality := make(map[shared.NodeID]float64)
	
	// Initialize centrality values
	for _, node := range graph.Nodes {
		centrality[node.ID] = 0
	}
	
	// For each pair of nodes, find shortest paths and update centrality
	for _, source := range graph.Nodes {
		// Single-source shortest paths
		distances, paths := a.dijkstra(graph, source.ID)
		
		for _, target := range graph.Nodes {
			if source.ID == target.ID {
				continue
			}
			
			if distance, ok := distances[target.ID]; ok && distance < math.MaxFloat64 {
				// Update centrality for nodes on the shortest path
				for _, nodeID := range paths[target.ID] {
					if nodeID != source.ID && nodeID != target.ID {
						centrality[nodeID]++
					}
				}
			}
		}
	}
	
	// Normalize centrality values
	n := float64(len(graph.Nodes))
	if n > 2 {
		normFactor := 2.0 / ((n - 1) * (n - 2))
		for nodeID := range centrality {
			centrality[nodeID] *= normFactor
		}
	}
	
	return centrality
}

// CalculateClosenessCentrality computes closeness centrality for nodes.
func (a *GraphAnalyzer) CalculateClosenessCentrality(graph *Graph) map[shared.NodeID]float64 {
	centrality := make(map[shared.NodeID]float64)
	
	for _, node := range graph.Nodes {
		distances, _ := a.dijkstra(graph, node.ID)
		
		totalDistance := 0.0
		reachableNodes := 0
		
		for targetID, distance := range distances {
			if targetID != node.ID && distance < math.MaxFloat64 {
				totalDistance += distance
				reachableNodes++
			}
		}
		
		if reachableNodes > 0 && totalDistance > 0 {
			centrality[node.ID] = float64(reachableNodes) / totalDistance
		} else {
			centrality[node.ID] = 0
		}
	}
	
	return centrality
}

// ============================================================================
// CLUSTERING ALGORITHMS
// ============================================================================

// FindClusters identifies clusters of similar nodes.
func (a *GraphAnalyzer) FindClusters(graph *Graph, threshold float64) []shared.Cluster {
	switch a.clusteringAlgorithm {
	case ClusteringLouvain:
		return a.louvainClustering(graph, threshold)
	case ClusteringKMeans:
		return a.kMeansClustering(graph, threshold)
	default:
		return a.hierarchicalClustering(graph, threshold)
	}
}

// louvainClustering implements Louvain clustering algorithm.
func (a *GraphAnalyzer) louvainClustering(graph *Graph, threshold float64) []shared.Cluster {
	communities := a.FindCommunities(graph)
	clusters := make([]shared.Cluster, 0, len(communities))
	
	for _, community := range communities {
		if len(community.NodeIDs) >= 2 { // Only include clusters with at least 2 nodes
			cluster := shared.Cluster{
				ID:       shared.ClusterID(community.ID),
				NodeIDs:  community.NodeIDs,
				Centroid: a.calculateCentroid(graph, community.NodeIDs),
				Density:  a.calculateDensity(graph, community.NodeIDs),
			}
			clusters = append(clusters, cluster)
		}
	}
	
	return clusters
}

// hierarchicalClustering implements hierarchical clustering.
func (a *GraphAnalyzer) hierarchicalClustering(graph *Graph, threshold float64) []shared.Cluster {
	if len(graph.Nodes) == 0 {
		return []shared.Cluster{}
	}
	
	// Initialize: each node is its own cluster
	clusters := make([]shared.Cluster, 0, len(graph.Nodes))
	for i, node := range graph.Nodes {
		clusters = append(clusters, shared.Cluster{
			ID:      shared.ClusterID(i),
			NodeIDs: []shared.NodeID{node.ID},
			Density: 1.0,
		})
	}
	
	// Build similarity matrix
	similarities := a.buildSimilarityMatrix(graph)
	
	// Merge clusters until threshold is reached
	for len(clusters) > 1 {
		maxSim := 0.0
		mergeI, mergeJ := -1, -1
		
		// Find most similar pair of clusters
		for i := 0; i < len(clusters); i++ {
			for j := i + 1; j < len(clusters); j++ {
				sim := a.clusterSimilarity(clusters[i], clusters[j], similarities)
				if sim > maxSim {
					maxSim = sim
					mergeI, mergeJ = i, j
				}
			}
		}
		
		// Stop if no clusters are similar enough
		if maxSim < threshold {
			break
		}
		
		// Merge clusters
		if mergeI >= 0 && mergeJ >= 0 {
			merged := a.mergeClusters(clusters[mergeI], clusters[mergeJ], graph)
			
			// Remove old clusters and add merged
			newClusters := make([]shared.Cluster, 0, len(clusters)-1)
			for i, cluster := range clusters {
				if i != mergeI && i != mergeJ {
					newClusters = append(newClusters, cluster)
				}
			}
			newClusters = append(newClusters, merged)
			clusters = newClusters
		} else {
			break
		}
	}
	
	return clusters
}

// kMeansClustering implements k-means clustering (simplified for graph data).
func (a *GraphAnalyzer) kMeansClustering(graph *Graph, threshold float64) []shared.Cluster {
	// Estimate k based on graph size
	k := int(math.Sqrt(float64(len(graph.Nodes))))
	if k < 2 {
		k = 2
	}
	if k > len(graph.Nodes)/2 {
		k = len(graph.Nodes) / 2
	}
	
	// This is a simplified implementation
	// In production, use proper k-means with node embeddings
	return a.hierarchicalClustering(graph, threshold)
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// buildAdjacencyList builds an adjacency list from edges.
func (a *GraphAnalyzer) buildAdjacencyList(graph *Graph) map[shared.NodeID][]shared.NodeID {
	adjacency := make(map[shared.NodeID][]shared.NodeID)
	
	for _, edge := range graph.Edges {
		adjacency[edge.SourceID] = append(adjacency[edge.SourceID], edge.TargetID)
		adjacency[edge.TargetID] = append(adjacency[edge.TargetID], edge.SourceID)
	}
	
	return adjacency
}

// dijkstra implements Dijkstra's shortest path algorithm.
func (a *GraphAnalyzer) dijkstra(graph *Graph, source shared.NodeID) (map[shared.NodeID]float64, map[shared.NodeID][]shared.NodeID) {
	distances := make(map[shared.NodeID]float64)
	paths := make(map[shared.NodeID][]shared.NodeID)
	visited := make(map[shared.NodeID]bool)
	
	// Initialize distances
	for _, node := range graph.Nodes {
		distances[node.ID] = math.MaxFloat64
		paths[node.ID] = []shared.NodeID{}
	}
	distances[source] = 0
	paths[source] = []shared.NodeID{source}
	
	// Build adjacency list with weights
	adjacency := make(map[shared.NodeID]map[shared.NodeID]float64)
	for _, edge := range graph.Edges {
		if adjacency[edge.SourceID] == nil {
			adjacency[edge.SourceID] = make(map[shared.NodeID]float64)
		}
		weight := 1.0
		if edge.Strength > 0 {
			weight = 1.0 / edge.Strength // Inverse weight for stronger connections
		}
		adjacency[edge.SourceID][edge.TargetID] = weight
	}
	
	// Find shortest paths
	for len(visited) < len(graph.Nodes) {
		// Find unvisited node with minimum distance
		minDist := math.MaxFloat64
		var current shared.NodeID
		found := false
		
		for _, node := range graph.Nodes {
			if !visited[node.ID] && distances[node.ID] < minDist {
				minDist = distances[node.ID]
				current = node.ID
				found = true
			}
		}
		
		if !found {
			break
		}
		
		visited[current] = true
		
		// Update distances to neighbors
		if neighbors, ok := adjacency[current]; ok {
			for neighbor, weight := range neighbors {
				alt := distances[current] + weight
				if alt < distances[neighbor] {
					distances[neighbor] = alt
					paths[neighbor] = append(paths[current], neighbor)
				}
			}
		}
	}
	
	return distances, paths
}

// communitiesToDomain converts community map to domain objects.
func (a *GraphAnalyzer) communitiesToDomain(communities map[shared.NodeID]int, graph *Graph) []shared.Community {
	// Group nodes by community
	groups := make(map[int][]shared.NodeID)
	for nodeID, communityID := range communities {
		groups[communityID] = append(groups[communityID], nodeID)
	}
	
	// Create Community objects
	result := make([]shared.Community, 0, len(groups))
	for communityID, nodeIDs := range groups {
		result = append(result, shared.Community{
			ID:      shared.CommunityID(communityID),
			NodeIDs: nodeIDs,
			Size:    len(nodeIDs),
		})
	}
	
	// Sort by size (largest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Size > result[j].Size
	})
	
	return result
}

// buildSimilarityMatrix builds a similarity matrix for nodes.
func (a *GraphAnalyzer) buildSimilarityMatrix(graph *Graph) map[shared.NodeID]map[shared.NodeID]float64 {
	matrix := make(map[shared.NodeID]map[shared.NodeID]float64)
	
	// Initialize matrix
	for _, node := range graph.Nodes {
		matrix[node.ID] = make(map[shared.NodeID]float64)
	}
	
	// Fill matrix based on edges
	for _, edge := range graph.Edges {
		matrix[edge.SourceID][edge.TargetID] = edge.Strength
		matrix[edge.TargetID][edge.SourceID] = edge.Strength
	}
	
	return matrix
}

// clusterSimilarity calculates similarity between two clusters.
func (a *GraphAnalyzer) clusterSimilarity(c1, c2 shared.Cluster, similarities map[shared.NodeID]map[shared.NodeID]float64) float64 {
	totalSim := 0.0
	count := 0
	
	for _, n1 := range c1.NodeIDs {
		for _, n2 := range c2.NodeIDs {
			if sim, ok := similarities[n1][n2]; ok {
				totalSim += sim
				count++
			}
		}
	}
	
	if count == 0 {
		return 0
	}
	
	return totalSim / float64(count)
}

// mergeClusters merges two clusters.
func (a *GraphAnalyzer) mergeClusters(c1, c2 shared.Cluster, graph *Graph) shared.Cluster {
	nodeIDs := append(c1.NodeIDs, c2.NodeIDs...)
	
	return shared.Cluster{
		ID:       c1.ID, // Keep first cluster's ID
		NodeIDs:  nodeIDs,
		Centroid: a.calculateCentroid(graph, nodeIDs),
		Density:  a.calculateDensity(graph, nodeIDs),
	}
}

// calculateCentroid calculates the centroid of a cluster.
func (a *GraphAnalyzer) calculateCentroid(graph *Graph, nodeIDs []shared.NodeID) shared.NodeID {
	if len(nodeIDs) == 0 {
		return shared.NodeID{}
	}
	
	// Find node with highest average similarity to others
	maxAvgSim := 0.0
	var centroid shared.NodeID
	
	for _, candidate := range nodeIDs {
		totalSim := 0.0
		count := 0
		
		for _, other := range nodeIDs {
			if !candidate.Equals(other) {
				// Calculate similarity based on connections
				for _, edge := range graph.Edges {
					if (edge.SourceID.Equals(candidate) && edge.TargetID.Equals(other)) ||
					   (edge.TargetID.Equals(candidate) && edge.SourceID.Equals(other)) {
						totalSim += edge.Strength
						count++
					}
				}
			}
		}
		
		if count > 0 {
			avgSim := totalSim / float64(count)
			if avgSim > maxAvgSim {
				maxAvgSim = avgSim
				centroid = candidate
			}
		}
	}
	
	if centroid.IsEmpty() && len(nodeIDs) > 0 {
		centroid = nodeIDs[0]
	}
	
	return centroid
}

// calculateDensity calculates the density of a cluster.
func (a *GraphAnalyzer) calculateDensity(graph *Graph, nodeIDs []shared.NodeID) float64 {
	if len(nodeIDs) <= 1 {
		return 0
	}
	
	// Count edges within the cluster
	edgeCount := 0
	for _, edge := range graph.Edges {
		source := false
		target := false
		
		for _, nodeID := range nodeIDs {
			if edge.SourceID == nodeID {
				source = true
			}
			if edge.TargetID == nodeID {
				target = true
			}
		}
		
		if source && target {
			edgeCount++
		}
	}
	
	// Calculate density
	n := float64(len(nodeIDs))
	maxEdges := n * (n - 1) / 2
	
	return float64(edgeCount) / maxEdges
}