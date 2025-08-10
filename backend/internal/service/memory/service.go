// Package memory provides business logic for memory node management and connection discovery.
package memory

import (
	"context"
	"log"
	"regexp"
	"strings"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"

	"github.com/google/uuid"
)

// stopWords contains common words filtered out during keyword extraction
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true,
	"and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "up": true, "about": true,
	"into": true, "through": true, "during": true, "before": true, "after": true,
	"above": true, "below": true, "between": true, "under": true,
	"again": true, "further": true, "then": true, "once": true,
	"is": true, "am": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
	"will": true, "would": true, "should": true, "could": true, "ought": true,
	"i": true, "me": true, "my": true, "myself": true,
	"we": true, "our": true, "ours": true, "ourselves": true,
	"you": true, "your": true, "yours": true, "yourself": true, "yourselves": true,
	"he": true, "him": true, "his": true, "himself": true,
	"she": true, "her": true, "hers": true, "herself": true,
	"it": true, "its": true, "itself": true,
	"they": true, "them": true, "their": true, "theirs": true, "themselves": true,
	"what": true, "which": true, "who": true, "whom": true,
	"this": true, "that": true, "these": true, "those": true,
	"as": true, "if": true, "each": true, "how": true, "than": true,
	"too": true, "very": true, "can": true, "just": true, "also": true,
}

// Service defines the interface for memory-related business operations.
type Service interface {
	// CreateNodeAndKeywords saves a memory node with extracted keywords
	CreateNodeAndKeywords(ctx context.Context, node domain.Node) error

	// CreateNodeWithEdges creates a new memory and immediately finds connections
	CreateNodeWithEdges(ctx context.Context, userID, content string, tags []string) (*domain.Node, []domain.Edge, error)

	// UpdateNode modifies an existing memory and recalculates its connections
	UpdateNode(ctx context.Context, userID, nodeID, content string, tags []string) (*domain.Node, error)

	// DeleteNode removes a memory and cleans up all its relationships
	DeleteNode(ctx context.Context, userID, nodeID string) error

	// BulkDeleteNodes efficiently removes multiple memories in a single operation
	BulkDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (int, []string, error)

	// GetNodeDetails retrieves a memory with its direct connections
	GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []domain.Edge, error)

	// GetGraphData retrieves the complete knowledge graph for visualization
	GetGraphData(ctx context.Context, userID string) (*domain.Graph, error)

	// Enhanced methods for performance
	GetNodesPage(ctx context.Context, userID string, pagination repository.Pagination) (*repository.NodePage, error)
	GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error)
	GetGraphDataPaginated(ctx context.Context, userID string, pagination repository.Pagination) (*domain.Graph, string, error)

	// Optimistic locking methods for safe concurrent updates
	UpdateNodeWithRetry(ctx context.Context, userID, nodeID string, updateFn func(*domain.Node) error) (*domain.Node, error)
	UpdateNodeWithEdgesRetry(ctx context.Context, userID, nodeID string, relatedNodeIDs []string, updateFn func(*domain.Node) error) (*domain.Node, error)
	SafeUpdateNode(ctx context.Context, userID, nodeID, newContent string, newTags []string) (*domain.Node, error)
	SafeUpdateNodeWithConnections(ctx context.Context, userID, nodeID, newContent string, newTags []string, relatedNodeIDs []string) (*domain.Node, error)
}

// service implements the Service interface with concrete business logic.
type service struct {
	repo repository.Repository
}

// NewService creates a new memory service with the provided repository.
func NewService(repo repository.Repository) Service {
	return &service{repo: repo}
}

// CreateNodeAndKeywords stores a memory node with pre-extracted keywords.
func (s *service) CreateNodeAndKeywords(ctx context.Context, node domain.Node) error {
	if node.Content == "" {
		return appErrors.NewValidation("content cannot be empty")
	}

	return s.repo.CreateNodeAndKeywords(ctx, node)
}

// CreateNodeWithEdges creates a new memory node and finds related connections synchronously.
func (s *service) CreateNodeWithEdges(ctx context.Context, userID, content string, tags []string) (*domain.Node, []domain.Edge, error) {
	if content == "" {
		return nil, nil, appErrors.NewValidation("content cannot be empty")
	}

	keywords := ExtractKeywords(content)
	log.Printf("DEBUG CreateNodeWithEdges: content='%s', extracted keywords=%v", content, keywords)

	node := domain.Node{
		ID:        uuid.New().String(),
		UserID:    userID,
		Content:   content,
		Keywords:  keywords,
		Tags:      tags,
		CreatedAt: time.Now(),
		Version:   1,
	}

	log.Printf("DEBUG CreateNodeWithEdges: created node ID=%s, searching for related nodes with keywords=%v", node.ID, keywords)

	query := repository.NodeQuery{
		UserID:   userID,
		Keywords: keywords,
	}

	relatedNodes, err := s.repo.FindNodes(ctx, query)
	if err != nil {
		log.Printf("ERROR finding related nodes for new node %s: %v", node.ID, err)
	}

	log.Printf("DEBUG CreateNodeWithEdges: found %d related nodes for node %s", len(relatedNodes), node.ID)

	var relatedNodeIDs []string
	for _, rn := range relatedNodes {
		log.Printf("DEBUG CreateNodeWithEdges: related node found: ID=%s, content='%s', keywords=%v", rn.ID, rn.Content, rn.Keywords)
		relatedNodeIDs = append(relatedNodeIDs, rn.ID)
	}

	log.Printf("DEBUG CreateNodeWithEdges: creating node %s with %d edges to nodes: %v", node.ID, len(relatedNodeIDs), relatedNodeIDs)

	if err := s.repo.CreateNodeWithEdges(ctx, node, relatedNodeIDs); err != nil {
		return nil, nil, appErrors.Wrap(err, "failed to create node in repository")
	}

	// Create edges to return with the response
	var edges []domain.Edge
	for _, relatedID := range relatedNodeIDs {
		edge := domain.Edge{
			SourceID: node.ID,
			TargetID: relatedID,
		}
		edges = append(edges, edge)
	}

	log.Printf("DEBUG CreateNodeWithEdges: successfully created node %s with %d edges", node.ID, len(edges))
	return &node, edges, nil
}

// UpdateNode orchestrates updating a node's content and reconnecting it.
func (s *service) UpdateNode(ctx context.Context, userID, nodeID, content string, tags []string) (*domain.Node, error) {
	if content == "" {
		return nil, appErrors.NewValidation("content cannot be empty")
	}

	existingNode, err := s.repo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to check for existing node")
	}
	if existingNode == nil {
		return nil, appErrors.NewNotFound("node not found")
	}

	keywords := ExtractKeywords(content)
	updatedNode := domain.Node{
		ID:        nodeID,
		UserID:    userID,
		Content:   content,
		Keywords:  keywords,
		Tags:      tags,
		CreatedAt: time.Now(),
		Version:   existingNode.Version + 1,
	}

	query := repository.NodeQuery{
		UserID:   userID,
		Keywords: keywords,
	}
	relatedNodes, err := s.repo.FindNodes(ctx, query)
	if err != nil {
		log.Printf("Non-critical error finding related nodes for updated node %s: %v", nodeID, err)
	}

	var relatedNodeIDs []string
	for _, rn := range relatedNodes {
		if rn.ID != nodeID {
			relatedNodeIDs = append(relatedNodeIDs, rn.ID)
		}
	}

	if err := s.repo.UpdateNodeAndEdges(ctx, updatedNode, relatedNodeIDs); err != nil {
		return nil, appErrors.Wrap(err, "failed to update node in repository")
	}

	return &updatedNode, nil
}

// DeleteNode orchestrates deleting a node.
func (s *service) DeleteNode(ctx context.Context, userID, nodeID string) error {
	existingNode, err := s.repo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return appErrors.Wrap(err, "failed to check for existing node before delete")
	}
	if existingNode == nil {
		return appErrors.NewNotFound("node not found")
	}
	return s.repo.DeleteNode(ctx, userID, nodeID)
}

// BulkDeleteNodes orchestrates deleting multiple nodes.
func (s *service) BulkDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (int, []string, error) {
	if len(nodeIDs) == 0 {
		return 0, nil, appErrors.NewValidation("nodeIds cannot be empty")
	}

	if len(nodeIDs) > 100 {
		return 0, nil, appErrors.NewValidation("cannot delete more than 100 nodes at once")
	}

	var failedNodeIDs []string
	deletedCount := 0

	for _, nodeID := range nodeIDs {
		// Check if node exists and belongs to user
		existingNode, err := s.repo.FindNodeByID(ctx, userID, nodeID)
		if err != nil {
			log.Printf("Error checking node %s for user %s: %v", nodeID, userID, err)
			failedNodeIDs = append(failedNodeIDs, nodeID)
			continue
		}
		if existingNode == nil {
			log.Printf("Node %s not found for user %s", nodeID, userID)
			failedNodeIDs = append(failedNodeIDs, nodeID)
			continue
		}

		// Delete the node
		if err := s.repo.DeleteNode(ctx, userID, nodeID); err != nil {
			log.Printf("Error deleting node %s for user %s: %v", nodeID, userID, err)
			failedNodeIDs = append(failedNodeIDs, nodeID)
			continue
		}

		deletedCount++
	}

	return deletedCount, failedNodeIDs, nil
}

// GetNodeDetails retrieves a node and its direct connections.
func (s *service) GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []domain.Edge, error) {
	node, err := s.repo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, "failed to get node from repository")
	}
	if node == nil {
		return nil, nil, appErrors.NewNotFound("node not found")
	}

	edgeQuery := repository.EdgeQuery{
		UserID:   userID,
		SourceID: nodeID,
	}
	edges, err := s.repo.FindEdges(ctx, edgeQuery)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, "failed to get edges from repository")
	}

	return node, edges, nil
}

// GetGraphData retrieves all nodes and edges for a user.
func (s *service) GetGraphData(ctx context.Context, userID string) (*domain.Graph, error) {
	graphQuery := repository.GraphQuery{
		UserID:       userID,
		IncludeEdges: true,
	}
	graph, err := s.repo.GetGraphData(ctx, graphQuery)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get all graph data from repository")
	}
	return graph, nil
}

// GetNodesPage retrieves a paginated list of nodes for better performance
func (s *service) GetNodesPage(ctx context.Context, userID string, pagination repository.Pagination) (*repository.NodePage, error) {
	query := repository.NodeQuery{
		UserID: userID,
	}

	page, err := s.repo.GetNodesPage(ctx, query, pagination)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get nodes page from repository")
	}

	return page, nil
}

// GetNodeNeighborhood retrieves a node's neighborhood with depth limiting for performance
func (s *service) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error) {
	// Validate that the node exists and belongs to the user
	existingNode, err := s.repo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to check for existing node")
	}
	if existingNode == nil {
		return nil, appErrors.NewNotFound("node not found")
	}

	// Get the neighborhood
	graph, err := s.repo.GetNodeNeighborhood(ctx, userID, nodeID, depth)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get node neighborhood from repository")
	}

	return graph, nil
}

// GetGraphDataPaginated retrieves graph data with pagination for performance with large datasets
func (s *service) GetGraphDataPaginated(ctx context.Context, userID string, pagination repository.Pagination) (*domain.Graph, string, error) {
	graphQuery := repository.GraphQuery{
		UserID:       userID,
		IncludeEdges: true,
	}

	graph, nextCursor, err := s.repo.GetGraphDataPaginated(ctx, graphQuery, pagination)
	if err != nil {
		return nil, "", appErrors.Wrap(err, "failed to get paginated graph data from repository")
	}

	return graph, nextCursor, nil
}

// ExtractKeywords extracts meaningful keywords from text content for connection discovery.
func ExtractKeywords(content string) []string {
	content = strings.ToLower(content)
	reg := regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	content = reg.ReplaceAllString(content, "")
	words := strings.Fields(content)

	uniqueWords := make(map[string]bool)
	for _, word := range words {
		if !stopWords[word] && len(word) > 2 {
			uniqueWords[word] = true
		}
	}

	keywords := make([]string, 0, len(uniqueWords))
	for word := range uniqueWords {
		keywords = append(keywords, word)
	}

	return keywords
}
