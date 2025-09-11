package services

import (
	"fmt"
	"math"
	"strings"

	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"backend/domain/config"
	pkgerrors "backend/pkg/errors"
)

// NodeRelationshipService manages node connections and relationships
// This service handles connection logic, similarity calculations, and edge management
type NodeRelationshipService struct {
	config *config.DomainConfig
}

// NewNodeRelationshipService creates a new node relationship service
func NewNodeRelationshipService(cfg *config.DomainConfig) *NodeRelationshipService {
	if cfg == nil {
		cfg = config.DefaultDomainConfig()
	}
	return &NodeRelationshipService{
		config: cfg,
	}
}

// CanConnect determines if two nodes can be connected based on business rules
func (s *NodeRelationshipService) CanConnect(
	graph *aggregates.Graph,
	sourceID, targetID valueobjects.NodeID,
) (bool, error) {
	// Check if nodes exist
	nodes, err := graph.Nodes()
	if err != nil {
		return false, err
	}

	sourceNode, sourceExists := nodes[sourceID]
	targetNode, targetExists := nodes[targetID]

	if !sourceExists || !targetExists {
		return false, pkgerrors.NewValidationError("both nodes must exist in graph")
	}

	// Check for self-reference
	if sourceID.Equals(targetID) {
		return false, nil
	}

	// Check if edge already exists
	edges := graph.Edges()
	edgeKey := s.makeEdgeKey(sourceID, targetID)
	if _, exists := edges[edgeKey]; exists {
		return false, nil
	}

	// Check node connection limits
	sourceConnections := s.countNodeConnections(edges, sourceID)
	if sourceConnections >= s.config.MaxConnectionsPerNode {
		return false, nil
	}

	// Check if nodes are compatible (can be extended with more rules)
	if !s.areNodesCompatible(sourceNode, targetNode) {
		return false, nil
	}

	// Check graph edge limit
	if len(edges) >= s.config.MaxEdgesPerGraph {
		return false, nil
	}

	return true, nil
}

// CalculateSimilarity calculates similarity between two nodes
// Returns a value between 0 (no similarity) and 1 (identical)
func (s *NodeRelationshipService) CalculateSimilarity(
	node1, node2 *entities.Node,
) float64 {
	if node1 == nil || node2 == nil {
		return 0.0
	}

	var similarityScore float64
	var weights float64

	// Compare content similarity (40% weight)
	contentSim := s.calculateContentSimilarity(node1.Content(), node2.Content())
	similarityScore += contentSim * 0.4
	weights += 0.4

	// Compare tags similarity (30% weight)
	tagsSim := s.calculateTagsSimilarity(node1.GetTags(), node2.GetTags())
	similarityScore += tagsSim * 0.3
	weights += 0.3

	// Compare position proximity (20% weight)
	positionSim := s.calculatePositionProximity(node1.Position(), node2.Position())
	similarityScore += positionSim * 0.2
	weights += 0.2

	// Compare metadata similarity (10% weight)
	metadataSim := s.calculateMetadataSimilarity(node1.GetMetadata(), node2.GetMetadata())
	similarityScore += metadataSim * 0.1
	weights += 0.1

	if weights > 0 {
		return similarityScore / weights
	}

	return 0.0
}

// SuggestConnections suggests potential connections for a node
func (s *NodeRelationshipService) SuggestConnections(
	graph *aggregates.Graph,
	nodeID valueobjects.NodeID,
	limit int,
) ([]ConnectionSuggestion, error) {
	nodes, err := graph.Nodes()
	if err != nil {
		return nil, err
	}

	sourceNode, exists := nodes[nodeID]
	if !exists {
		return nil, pkgerrors.NewNotFoundError("node")
	}

	edges := graph.Edges()
	suggestions := []ConnectionSuggestion{}

	// Check all other nodes for potential connections
	for targetID, targetNode := range nodes {
		// Skip self and already connected nodes
		if targetID.Equals(nodeID) {
			continue
		}

		if s.areNodesConnected(edges, nodeID, targetID) {
			continue
		}

		// Calculate similarity
		similarity := s.CalculateSimilarity(sourceNode, targetNode)

		// Only suggest if similarity is above threshold
		if similarity >= s.config.MinSimilarityThreshold {
			suggestions = append(suggestions, ConnectionSuggestion{
				TargetID:   targetID,
				Similarity: similarity,
				Reason:     s.generateConnectionReason(sourceNode, targetNode, similarity),
				EdgeType:   s.suggestEdgeType(similarity),
			})
		}
	}

	// Sort by similarity (highest first)
	s.sortSuggestions(suggestions)

	// Limit results
	if limit > 0 && len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}

	return suggestions, nil
}

// DetermineEdgeWeight calculates the weight for an edge based on node properties
func (s *NodeRelationshipService) DetermineEdgeWeight(
	sourceNode, targetNode *entities.Node,
) float64 {
	// Base weight on similarity
	similarity := s.CalculateSimilarity(sourceNode, targetNode)

	// Adjust weight based on node importance (can be extended)
	sourceImportance := s.calculateNodeImportance(sourceNode)
	targetImportance := s.calculateNodeImportance(targetNode)

	// Weighted average
	weight := similarity*0.6 + ((sourceImportance+targetImportance)/2)*0.4

	// Ensure weight is between 0 and 1
	return math.Max(0.0, math.Min(1.0, weight))
}

// ValidateEdge validates if an edge meets business rules
func (s *NodeRelationshipService) ValidateEdge(edge *aggregates.Edge) error {
	if edge == nil {
		return pkgerrors.NewValidationError("edge cannot be nil")
	}

	// Validate edge weight
	if edge.Weight < 0 || edge.Weight > 1 {
		return pkgerrors.NewValidationError("edge weight must be between 0 and 1")
	}

	// Validate edge type
	if !s.isValidEdgeType(edge.Type) {
		return pkgerrors.NewValidationError(fmt.Sprintf("invalid edge type: %s", edge.Type))
	}

	// Check for self-loop
	if edge.SourceID.Equals(edge.TargetID) {
		return pkgerrors.NewValidationError("self-loops are not allowed")
	}

	return nil
}

// Helper types

// ConnectionSuggestion represents a suggested connection
type ConnectionSuggestion struct {
	TargetID   valueobjects.NodeID
	Similarity float64
	Reason     string
	EdgeType   entities.EdgeType
}

// Private helper methods

func (s *NodeRelationshipService) makeEdgeKey(sourceID, targetID valueobjects.NodeID) string {
	return sourceID.String() + "->" + targetID.String()
}

func (s *NodeRelationshipService) countNodeConnections(
	edges map[string]*aggregates.Edge,
	nodeID valueobjects.NodeID,
) int {
	count := 0
	for _, edge := range edges {
		if edge.SourceID.Equals(nodeID) || edge.TargetID.Equals(nodeID) {
			count++
		}
	}
	return count
}

func (s *NodeRelationshipService) areNodesCompatible(node1, node2 *entities.Node) bool {
	// Can be extended with more complex compatibility rules
	// For now, all nodes are compatible
	return true
}

func (s *NodeRelationshipService) areNodesConnected(
	edges map[string]*aggregates.Edge,
	node1ID, node2ID valueobjects.NodeID,
) bool {
	for _, edge := range edges {
		if (edge.SourceID.Equals(node1ID) && edge.TargetID.Equals(node2ID)) ||
			(edge.SourceID.Equals(node2ID) && edge.TargetID.Equals(node1ID)) {
			return true
		}
		if edge.Bidirectional &&
			((edge.SourceID.Equals(node1ID) && edge.TargetID.Equals(node2ID)) ||
				(edge.SourceID.Equals(node2ID) && edge.TargetID.Equals(node1ID))) {
			return true
		}
	}
	return false
}

func (s *NodeRelationshipService) calculateContentSimilarity(
	content1, content2 valueobjects.NodeContent,
) float64 {
	// Simple Jaccard similarity for content
	words1 := s.tokenize(content1.Title() + " " + content1.Body())
	words2 := s.tokenize(content2.Title() + " " + content2.Body())

	if len(words1) == 0 && len(words2) == 0 {
		return 1.0
	}
	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	intersection := 0
	wordSet1 := make(map[string]bool)
	for _, word := range words1 {
		wordSet1[word] = true
	}

	for _, word := range words2 {
		if wordSet1[word] {
			intersection++
		}
	}

	union := len(words1) + len(words2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

func (s *NodeRelationshipService) calculateTagsSimilarity(tags1, tags2 []string) float64 {
	if len(tags1) == 0 && len(tags2) == 0 {
		return 1.0
	}
	if len(tags1) == 0 || len(tags2) == 0 {
		return 0.0
	}

	// Jaccard similarity for tags
	tagSet1 := make(map[string]bool)
	for _, tag := range tags1 {
		tagSet1[strings.ToLower(tag)] = true
	}

	intersection := 0
	for _, tag := range tags2 {
		if tagSet1[strings.ToLower(tag)] {
			intersection++
		}
	}

	union := len(tags1) + len(tags2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

func (s *NodeRelationshipService) calculatePositionProximity(
	pos1, pos2 valueobjects.Position,
) float64 {
	// Calculate Euclidean distance
	dx := pos1.X() - pos2.X()
	dy := pos1.Y() - pos2.Y()
	dz := pos1.Z() - pos2.Z()

	distance := math.Sqrt(dx*dx + dy*dy + dz*dz)

	// Convert distance to similarity (closer = higher similarity)
	// Using exponential decay
	maxDistance := 1000.0 // Maximum expected distance
	if distance >= maxDistance {
		return 0.0
	}

	return math.Exp(-distance / 100.0) // Decay factor of 100
}

func (s *NodeRelationshipService) calculateMetadataSimilarity(
	meta1, meta2 map[string]interface{},
) float64 {
	if len(meta1) == 0 && len(meta2) == 0 {
		return 1.0
	}
	if len(meta1) == 0 || len(meta2) == 0 {
		return 0.0
	}

	// Count matching keys with same values
	matches := 0
	total := 0

	for key, val1 := range meta1 {
		total++
		if val2, exists := meta2[key]; exists {
			if fmt.Sprintf("%v", val1) == fmt.Sprintf("%v", val2) {
				matches++
			}
		}
	}

	for key := range meta2 {
		if _, exists := meta1[key]; !exists {
			total++
		}
	}

	if total == 0 {
		return 0.0
	}

	return float64(matches) / float64(total)
}

func (s *NodeRelationshipService) calculateNodeImportance(node *entities.Node) float64 {
	// Simple importance calculation based on content length and tags
	importance := 0.0

	content := node.Content()
	contentLength := len(content.Title()) + len(content.Body())
	if contentLength > 0 {
		// Normalize content length to 0-1 range
		importance += math.Min(float64(contentLength)/1000.0, 1.0) * 0.5
	}

	// More tags = more important
	tags := node.GetTags()
	if len(tags) > 0 {
		importance += math.Min(float64(len(tags))/10.0, 1.0) * 0.5
	}

	return importance
}

func (s *NodeRelationshipService) tokenize(text string) []string {
	// Simple tokenization
	words := strings.Fields(strings.ToLower(text))
	result := []string{}

	for _, word := range words {
		// Clean punctuation
		cleaned := strings.Trim(word, ".,!?;:\"'()[]{}#@$%^&*+=<>/\\|`~")
		if len(cleaned) > 2 { // Skip very short words
			result = append(result, cleaned)
		}
	}

	return result
}

func (s *NodeRelationshipService) generateConnectionReason(
	sourceNode, targetNode *entities.Node,
	similarity float64,
) string {
	reasons := []string{}

	// Check tag overlap
	sourceTags := sourceNode.GetTags()
	targetTags := targetNode.GetTags()
	commonTags := s.findCommonTags(sourceTags, targetTags)
	if len(commonTags) > 0 {
		reasons = append(reasons, fmt.Sprintf("shared tags: %s", strings.Join(commonTags, ", ")))
	}

	// Check content similarity
	if similarity > 0.7 {
		reasons = append(reasons, "highly similar content")
	} else if similarity > 0.4 {
		reasons = append(reasons, "related content")
	}

	// Check position proximity
	distance := s.calculateDistance(sourceNode.Position(), targetNode.Position())
	if distance < 100 {
		reasons = append(reasons, "nearby position")
	}

	if len(reasons) == 0 {
		return "potential connection"
	}

	return strings.Join(reasons, "; ")
}

func (s *NodeRelationshipService) findCommonTags(tags1, tags2 []string) []string {
	tagSet := make(map[string]bool)
	for _, tag := range tags1 {
		tagSet[strings.ToLower(tag)] = true
	}

	common := []string{}
	for _, tag := range tags2 {
		if tagSet[strings.ToLower(tag)] {
			common = append(common, tag)
		}
	}

	return common
}

func (s *NodeRelationshipService) calculateDistance(pos1, pos2 valueobjects.Position) float64 {
	dx := pos1.X() - pos2.X()
	dy := pos1.Y() - pos2.Y()
	dz := pos1.Z() - pos2.Z()
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func (s *NodeRelationshipService) suggestEdgeType(similarity float64) entities.EdgeType {
	if similarity > 0.8 {
		return entities.EdgeTypeStrong
	} else if similarity > 0.5 {
		return entities.EdgeTypeNormal
	}
	return entities.EdgeTypeWeak
}

func (s *NodeRelationshipService) isValidEdgeType(edgeType entities.EdgeType) bool {
	validTypes := []entities.EdgeType{
		entities.EdgeTypeNormal,
		entities.EdgeTypeStrong,
		entities.EdgeTypeWeak,
		entities.EdgeTypeReference,
		entities.EdgeTypeHierarchical,
		entities.EdgeTypeTemporal,
	}

	for _, valid := range validTypes {
		if edgeType == valid {
			return true
		}
	}

	return false
}

func (s *NodeRelationshipService) sortSuggestions(suggestions []ConnectionSuggestion) {
	// Simple bubble sort for small lists
	n := len(suggestions)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if suggestions[j].Similarity < suggestions[j+1].Similarity {
				suggestions[j], suggestions[j+1] = suggestions[j+1], suggestions[j]
			}
		}
	}
}