// Package memory contains the core business logic for handling nodes and their connections.
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

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true, "in": true, "on": true, "at": true, "to": true, "for": true, "of": true, "with": true, "by": true, "from": true, "up": true, "about": true, "into": true, "through": true, "during": true, "before": true, "after": true, "above": true, "below": true, "between": true, "under": true, "again": true, "further": true, "then": true, "once": true, "is": true, "am": true, "are": true, "was": true, "were": true, "be": true, "been": true, "being": true, "have": true, "has": true, "had": true, "do": true, "does": true, "did": true, "will": true, "would": true, "should": true, "could": true, "ought": true, "i": true, "me": true, "my": true, "myself": true, "we": true, "our": true, "ours": true, "ourselves": true, "you": true, "your": true, "yours": true, "yourself": true, "yourselves": true, "he": true, "him": true, "his": true, "himself": true, "she": true, "her": true, "hers": true, "herself": true, "it": true, "its": true, "itself": true, "they": true, "them": true, "their": true, "theirs": true, "themselves": true, "what": true, "which": true, "who": true, "whom": true, "this": true, "that": true, "these": true, "those": true, "as": true, "if": true, "each": true, "how": true, "than": true, "too": true, "very": true, "can": true, "just": true, "also": true,
}

// Service defines the contract for memory-related business logic.
type Service interface {
	CreateNodeAndKeywords(ctx context.Context, node domain.Node) error
	CreateNodeWithEdges(ctx context.Context, userID, content string) (*domain.Node, error)
	UpdateNode(ctx context.Context, userID, nodeID, content string) (*domain.Node, error)
	DeleteNode(ctx context.Context, userID, nodeID string) error
	GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []domain.Edge, error)
	GetGraphData(ctx context.Context, userID string) (*domain.Graph, error)
}

type service struct {
	repo repository.Repository
}

// NewService creates a new memory service.
func NewService(repo repository.Repository) Service {
	return &service{repo: repo}
}

// CreateNodeAndKeywords saves the node metadata and keywords.
func (s *service) CreateNodeAndKeywords(ctx context.Context, node domain.Node) error {
	if node.Content == "" {
		return appErrors.NewValidation("content cannot be empty")
	}
	// This now calls the correct method defined in the repository interface.
	return s.repo.CreateNodeAndKeywords(ctx, node)
}

// CreateNodeWithEdges orchestrates the creation of a new node and its connections (legacy synchronous flow).
func (s *service) CreateNodeWithEdges(ctx context.Context, userID, content string) (*domain.Node, error) {
	if content == "" {
		return nil, appErrors.NewValidation("content cannot be empty")
	}

	keywords := ExtractKeywords(content)
	node := domain.Node{
		ID:        uuid.New().String(),
		UserID:    userID,
		Content:   content,
		Keywords:  keywords,
		CreatedAt: time.Now(),
		Version:   0,
	}

	query := repository.NodeQuery{
		UserID:   userID,
		Keywords: keywords,
	}
	relatedNodes, err := s.repo.FindNodes(ctx, query)
	if err != nil {
		log.Printf("Non-critical error finding related nodes for new node: %v", err)
	}

	var relatedNodeIDs []string
	for _, rn := range relatedNodes {
		relatedNodeIDs = append(relatedNodeIDs, rn.ID)
	}

	if err := s.repo.CreateNodeWithEdges(ctx, node, relatedNodeIDs); err != nil {
		return nil, appErrors.Wrap(err, "failed to create node in repository")
	}

	return &node, nil
}

// UpdateNode orchestrates updating a node's content and reconnecting it.
func (s *service) UpdateNode(ctx context.Context, userID, nodeID, content string) (*domain.Node, error) {
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

// ExtractKeywords takes a string of content and returns a slice of unique, meaningful keywords.
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
