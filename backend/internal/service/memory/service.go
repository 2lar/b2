// Package memory contains the core business logic for handling nodes and their connections.
package memory

import (
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository/ddb" // CORRECTED IMPORT PATH
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true, "in": true, "on": true, "at": true, "to": true, "for": true, "of": true, "with": true, "by": true, "from": true, "up": true, "about": true, "into": true, "through": true, "during": true, "before": true, "after": true, "above": true, "below": true, "between": true, "under": true, "again": true, "further": true, "then": true, "once": true, "is": true, "am": true, "are": true, "was": true, "were": true, "be": true, "been": true, "being": true, "have": true, "has": true, "had": true, "do": true, "does": true, "did": true, "will": true, "would": true, "should": true, "could": true, "ought": true, "i": true, "me": true, "my": true, "myself": true, "we": true, "our": true, "ours": true, "ourselves": true, "you": true, "your": true, "yours": true, "yourself": true, "yourselves": true, "he": true, "him": true, "his": true, "himself": true, "she": true, "her": true, "hers": true, "herself": true, "it": true, "its": true, "itself": true, "they": true, "them": true, "their": true, "theirs": true, "themselves": true, "what": true, "which": true, "who": true, "whom": true, "this": true, "that": true, "these": true, "those": true, "as": true, "if": true, "each": true, "how": true, "than": true, "too": true, "very": true, "can": true, "just": true, "also": true,
}

// Service defines the contract for memory-related business logic.
type Service interface {
	CreateNode(ctx context.Context, userID, content string) (*domain.Node, error)
	UpdateNode(ctx context.Context, userID, nodeID, content string) (*domain.Node, error)
	DeleteNode(ctx context.Context, userID, nodeID string) error
	GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []domain.Edge, error)
	GetGraphData(ctx context.Context, userID string) (*domain.Graph, error)
}

type service struct {
	repo ddb.Repository
}

// NewService creates a new memory service.
func NewService(repo ddb.Repository) Service {
	return &service{repo: repo}
}

// CreateNode orchestrates the creation of a new node and its connections.
func (s *service) CreateNode(ctx context.Context, userID, content string) (*domain.Node, error) {
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}

	keywords := extractKeywords(content)
	node := domain.Node{
		ID:        uuid.New().String(),
		UserID:    userID,
		Content:   content,
		Keywords:  keywords,
		CreatedAt: time.Now(),
		Version:   0,
	}

	relatedNodes, err := s.repo.FindNodesByKeywords(ctx, userID, keywords)
	if err != nil {
		// Log but don't fail, as finding connections is non-critical on create
		fmt.Printf("could not find related nodes for new node: %v", err)
	}

	var relatedNodeIDs []string
	for _, rn := range relatedNodes {
		relatedNodeIDs = append(relatedNodeIDs, rn.ID)
	}

	if err := s.repo.CreateNodeWithEdges(ctx, node, relatedNodeIDs); err != nil {
		return nil, fmt.Errorf("failed to create node in repository: %w", err)
	}

	return &node, nil
}

// UpdateNode orchestrates updating a node's content and reconnecting it.
func (s *service) UpdateNode(ctx context.Context, userID, nodeID, content string) (*domain.Node, error) {
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}

	// Ensure the node exists first
	existingNode, err := s.repo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing node: %w", err)
	}
	if existingNode == nil {
		return nil, fmt.Errorf("node not found")
	}

	keywords := extractKeywords(content)
	updatedNode := domain.Node{
		ID:        nodeID,
		UserID:    userID,
		Content:   content,
		Keywords:  keywords,
		CreatedAt: time.Now(), // Update timestamp
		Version:   existingNode.Version + 1,
	}

	relatedNodes, err := s.repo.FindNodesByKeywords(ctx, userID, keywords)
	if err != nil {
		fmt.Printf("could not find related nodes for updated node %s: %v", nodeID, err)
	}

	var relatedNodeIDs []string
	for _, rn := range relatedNodes {
		if rn.ID != nodeID { // Don't connect a node to itself
			relatedNodeIDs = append(relatedNodeIDs, rn.ID)
		}
	}

	if err := s.repo.UpdateNodeAndEdges(ctx, updatedNode, relatedNodeIDs); err != nil {
		return nil, fmt.Errorf("failed to update node in repository: %w", err)
	}

	return &updatedNode, nil
}

// DeleteNode orchestrates deleting a node.
func (s *service) DeleteNode(ctx context.Context, userID, nodeID string) error {
	// Business logic could be added here, e.g., checking for permissions,
	// or if the node is locked. For now, it's a direct pass-through.
	return s.repo.DeleteNode(ctx, userID, nodeID)
}

// GetNodeDetails retrieves a node and its direct connections.
func (s *service) GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []domain.Edge, error) {
	node, err := s.repo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, nil, err
	}
	if node == nil {
		return nil, nil, fmt.Errorf("node not found")
	}

	edges, err := s.repo.FindEdgesByNode(ctx, userID, nodeID)
	if err != nil {
		return nil, nil, err
	}

	return node, edges, nil
}

// GetGraphData retrieves all nodes and edges for a user.
func (s *service) GetGraphData(ctx context.Context, userID string) (*domain.Graph, error) {
	return s.repo.GetAllGraphData(ctx, userID)
}

// extractKeywords is a pure business logic function.
func extractKeywords(content string) []string {
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
