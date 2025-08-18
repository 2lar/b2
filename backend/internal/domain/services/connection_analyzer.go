package services

import (
	"sort"
	"time"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
)

// ConnectionAnalyzer is a domain service that encapsulates complex business logic
// for finding and analyzing potential connections between nodes.
//
// This demonstrates the Domain Service pattern - business logic that doesn't 
// naturally fit within a single entity but operates on multiple domain objects.
//
// Key Concepts Illustrated:
//   - Domain Service: Stateless service containing domain logic
//   - Business Rules: Encapsulates complex connection algorithms  
//   - Pure Domain Logic: No infrastructure dependencies
//   - Value Object Usage: Works with strongly-typed domain objects
type ConnectionAnalyzer struct {
	similarityThreshold   float64 // Minimum similarity required for connection
	maxConnectionsPerNode int     // Maximum connections to suggest per node
	recencyWeight        float64 // How much to weight recent content
}

// NewConnectionAnalyzer creates a new ConnectionAnalyzer with specified thresholds
func NewConnectionAnalyzer(similarityThreshold float64, maxConnections int, recencyWeight float64) *ConnectionAnalyzer {
	return &ConnectionAnalyzer{
		similarityThreshold:   similarityThreshold,
		maxConnectionsPerNode: maxConnections,
		recencyWeight:        recencyWeight,
	}
}

// ConnectionCandidate represents a potential connection with its relevance score
type ConnectionCandidate struct {
	Node            *node.Node
	RelevanceScore  float64
	SimilarityScore float64
	MatchingKeywords []string
	SharedTags      []string
	Reason          string
}

// FindPotentialConnections analyzes a node against candidates to find potential connections.
//
// Business Rules Applied:
//   - Only suggests connections above similarity threshold
//   - Respects maximum connections limit
//   - Considers recency of content
//   - Excludes archived nodes
//   - Orders by relevance score
func (ca *ConnectionAnalyzer) FindPotentialConnections(sourceNode *node.Node, candidates []*node.Node) ([]*ConnectionCandidate, error) {
	var connections []*ConnectionCandidate
	
	for _, candidate := range candidates {
		// Check basic business rules for connection eligibility
		if err := sourceNode.CanConnectTo(candidate); err != nil {
			continue // Skip if connection is not allowed
		}
		
		// Calculate connection metrics
		connectionCandidate := ca.analyzeConnection(sourceNode, candidate)
		
		// Apply similarity threshold
		if connectionCandidate.SimilarityScore >= ca.similarityThreshold {
			connections = append(connections, connectionCandidate)
		}
	}
	
	// Sort by relevance score (highest first)
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].RelevanceScore > connections[j].RelevanceScore
	})
	
	// Limit to maximum connections
	if len(connections) > ca.maxConnectionsPerNode {
		connections = connections[:ca.maxConnectionsPerNode]
	}
	
	return connections, nil
}

// analyzeConnection performs detailed analysis of connection potential between two nodes
func (ca *ConnectionAnalyzer) analyzeConnection(source, target *node.Node) *ConnectionCandidate {
	// Calculate similarity scores
	keywordSimilarity := source.Keywords().Overlap(target.Keywords())
	tagSimilarity := source.Tags.Overlap(target.Tags)
	
	// Find matching elements for explanation
	matchingKeywords := ca.findMatchingKeywords(source.Keywords(), target.Keywords())
	sharedTags := ca.findSharedTags(source.Tags, target.Tags)
	
	// Calculate base similarity (weighted combination)
	baseSimilarity := keywordSimilarity*0.7 + tagSimilarity*0.3
	
	// Calculate recency factor (more recent content gets higher score)
	recencyScore := ca.calculateRecencyScore(target.CreatedAt)
	
	// Calculate final relevance score
	relevanceScore := baseSimilarity*0.8 + recencyScore*ca.recencyWeight
	
	// Generate explanation for why this connection is suggested
	reason := ca.generateConnectionReason(keywordSimilarity, tagSimilarity, matchingKeywords, sharedTags)
	
	return &ConnectionCandidate{
		Node:             target,
		RelevanceScore:   relevanceScore,
		SimilarityScore:  baseSimilarity,
		MatchingKeywords: matchingKeywords,
		SharedTags:       sharedTags,
		Reason:           reason,
	}
}

// AnalyzeBidirectionalConnection checks if two nodes should be connected in both directions
func (ca *ConnectionAnalyzer) AnalyzeBidirectionalConnection(node1, node2 *node.Node) (*BidirectionalAnalysis, error) {
	// Check if connections are allowed
	if err := node1.CanConnectTo(node2); err != nil {
		return nil, err
	}
	
	// Analyze connection from node1 to node2
	forward := ca.analyzeConnection(node1, node2)
	
	// Analyze connection from node2 to node1  
	backward := ca.analyzeConnection(node2, node1)
	
	return &BidirectionalAnalysis{
		ForwardConnection:  forward,
		BackwardConnection: backward,
		ShouldConnect:      forward.SimilarityScore >= ca.similarityThreshold,
		IsSymmetric:        ca.isSymmetricConnection(forward, backward),
	}, nil
}

// BidirectionalAnalysis contains analysis of connections in both directions
type BidirectionalAnalysis struct {
	ForwardConnection  *ConnectionCandidate
	BackwardConnection *ConnectionCandidate
	ShouldConnect      bool
	IsSymmetric        bool
}

// CalculateGraphDensity calculates the density of connections in a node cluster
func (ca *ConnectionAnalyzer) CalculateGraphDensity(nodes []*node.Node) float64 {
	if len(nodes) < 2 {
		return 0
	}
	
	totalPossibleConnections := len(nodes) * (len(nodes) - 1) / 2
	actualConnections := 0
	
	// Count how many nodes could potentially connect
	for i, node1 := range nodes {
		for j := i + 1; j < len(nodes); j++ {
			node2 := nodes[j]
			if similarity := node1.CalculateSimilarityTo(node2); similarity >= ca.similarityThreshold {
				actualConnections++
			}
		}
	}
	
	return float64(actualConnections) / float64(totalPossibleConnections)
}

// FindOptimalConnections finds the best set of connections for a node considering global graph health
func (ca *ConnectionAnalyzer) FindOptimalConnections(sourceNode *node.Node, candidates []*node.Node, existingConnections int) ([]*ConnectionCandidate, error) {
	// Adjust max connections based on existing connections
	maxNew := ca.maxConnectionsPerNode - existingConnections
	if maxNew <= 0 {
		return []*ConnectionCandidate{}, nil
	}
	
	// Find all potential connections
	potentials, err := ca.FindPotentialConnections(sourceNode, candidates)
	if err != nil {
		return nil, err
	}
	
	// Apply diminishing returns algorithm - prefer diversity over similarity clustering
	optimal := ca.selectDiverseConnections(potentials, maxNew)
	
	return optimal, nil
}

// Private helper methods

// findMatchingKeywords finds keywords that appear in both sets
func (ca *ConnectionAnalyzer) findMatchingKeywords(keywords1, keywords2 shared.Keywords) []string {
	var matching []string
	
	for _, keyword := range keywords1.ToSlice() {
		if keywords2.Contains(keyword) {
			matching = append(matching, keyword)
		}
	}
	
	return matching
}

// findSharedTags finds tags that appear in both sets
func (ca *ConnectionAnalyzer) findSharedTags(tags1, tags2 shared.Tags) []string {
	var shared []string
	
	for _, tag := range tags1.ToSlice() {
		if tags2.Contains(tag) {
			shared = append(shared, tag)
		}
	}
	
	return shared
}

// calculateRecencyScore gives higher scores to more recent content
func (ca *ConnectionAnalyzer) calculateRecencyScore(createdAt time.Time) float64 {
	daysSinceCreation := time.Since(createdAt).Hours() / 24
	
	// Exponential decay: more recent content gets higher scores
	// Score approaches 0 as content gets older
	return 1.0 / (1.0 + daysSinceCreation*0.1)
}

// generateConnectionReason creates a human-readable explanation for the connection suggestion
func (ca *ConnectionAnalyzer) generateConnectionReason(keywordSim, tagSim float64, keywords, tags []string) string {
	if len(keywords) > 0 && len(tags) > 0 {
		return "Shares similar keywords and tags"
	} else if len(keywords) > 0 {
		return "Contains related keywords"
	} else if len(tags) > 0 {
		return "Has matching tags"
	} else if keywordSim > 0 {
		return "Similar content themes"
	}
	return "Potentially related content"
}

// isSymmetricConnection checks if the connection strength is similar in both directions
func (ca *ConnectionAnalyzer) isSymmetricConnection(forward, backward *ConnectionCandidate) bool {
	diff := forward.SimilarityScore - backward.SimilarityScore
	if diff < 0 {
		diff = -diff
	}
	
	// Consider symmetric if difference is less than 20%
	return diff < 0.2
}

// selectDiverseConnections uses a diversity algorithm to avoid creating echo chambers
func (ca *ConnectionAnalyzer) selectDiverseConnections(candidates []*ConnectionCandidate, maxConnections int) []*ConnectionCandidate {
	if len(candidates) <= maxConnections {
		return candidates
	}
	
	var selected []*ConnectionCandidate
	remaining := make([]*ConnectionCandidate, len(candidates))
	copy(remaining, candidates)
	
	// Always take the highest scoring candidate first
	selected = append(selected, remaining[0])
	remaining = remaining[1:]
	
	// For remaining slots, prefer candidates that are different from already selected
	for len(selected) < maxConnections && len(remaining) > 0 {
		bestIndex := 0
		bestDiversityScore := ca.calculateDiversityScore(remaining[0], selected)
		
		for i := 1; i < len(remaining); i++ {
			diversityScore := ca.calculateDiversityScore(remaining[i], selected)
			if diversityScore > bestDiversityScore {
				bestIndex = i
				bestDiversityScore = diversityScore
			}
		}
		
		// Add the most diverse candidate
		selected = append(selected, remaining[bestIndex])
		
		// Remove from remaining
		remaining = append(remaining[:bestIndex], remaining[bestIndex+1:]...)
	}
	
	return selected
}

// calculateDiversityScore calculates how different a candidate is from already selected connections
func (ca *ConnectionAnalyzer) calculateDiversityScore(candidate *ConnectionCandidate, selected []*ConnectionCandidate) float64 {
	if len(selected) == 0 {
		return candidate.RelevanceScore
	}
	
	// Calculate average similarity to already selected nodes
	totalSimilarity := 0.0
	for _, s := range selected {
		similarity := candidate.Node.CalculateSimilarityTo(s.Node)
		totalSimilarity += similarity
	}
	avgSimilarity := totalSimilarity / float64(len(selected))
	
	// Diversity score: high relevance but low similarity to existing selections
	diversityScore := candidate.RelevanceScore * (1.0 - avgSimilarity)
	
	return diversityScore
}

// EdgeWeightCalculator is a value object for calculating edge weights based on node similarity
type EdgeWeightCalculator struct {
	keywordWeight float64
	tagWeight     float64
	recencyWeight float64
}

// NewEdgeWeightCalculator creates a new weight calculator with specified weights
func NewEdgeWeightCalculator(keywordWeight, tagWeight, recencyWeight float64) EdgeWeightCalculator {
	return EdgeWeightCalculator{
		keywordWeight: keywordWeight,
		tagWeight:     tagWeight,
		recencyWeight: recencyWeight,
	}
}

// CalculateWeight calculates the edge weight between two nodes
func (calc EdgeWeightCalculator) CalculateWeight(source, target *node.Node) float64 {
	keywordSimilarity := source.Keywords().Overlap(target.Keywords())
	tagSimilarity := source.Tags.Overlap(target.Tags)
	
	// Calculate recency factor (more recent connections get higher weight)
	timeDiff := source.CreatedAt.Sub(target.CreatedAt)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	
	// Recency score decreases as time difference increases
	daysDiff := timeDiff.Hours() / 24
	recencyScore := 1.0 / (1.0 + daysDiff*0.1)
	
	// Weighted combination
	weight := keywordSimilarity*calc.keywordWeight + 
			  tagSimilarity*calc.tagWeight + 
			  recencyScore*calc.recencyWeight
	
	// Ensure weight is within valid range
	if weight > 1.0 {
		weight = 1.0
	}
	if weight < 0.0 {
		weight = 0.0
	}
	
	return weight
}