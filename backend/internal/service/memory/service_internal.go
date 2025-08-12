package memory

import (
	"context"
	"log"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
)

const (
	maxRetries = 3
	baseDelay  = 100 * time.Millisecond
)

// Internal methods that won't be exposed in the interface

// createNodeCore handles the actual node creation logic
func (s *service) createNodeCore(ctx context.Context, userID, content string, tags []string) (*domain.Node, []domain.Edge, error) {
	if content == "" {
		return nil, nil, appErrors.NewValidation("content cannot be empty")
	}

	keywords := ExtractKeywords(content)
	node := domain.NewNode(userID, content, tags)
	node.Keywords = keywords

	log.Printf("DEBUG createNodeCore: created node ID=%s, searching for related nodes with keywords=%v", node.ID, keywords)

	// Find related nodes
	relatedNodes, err := s.keywordRepo.FindNodesByKeywords(ctx, userID, keywords)
	if err != nil {
		// Log but don't fail - connections are non-critical
		log.Printf("WARN: Failed to find related nodes: %v", err)
		relatedNodes = []domain.Node{}
	}

	var relatedNodeIDs []string
	for _, rn := range relatedNodes {
		if rn.ID != node.ID {
			relatedNodeIDs = append(relatedNodeIDs, rn.ID)
		}
	}

	log.Printf("DEBUG createNodeCore: creating node %s with %d edges to nodes: %v", node.ID, len(relatedNodeIDs), relatedNodeIDs)

	// Create node with edges in transaction
	if err := s.transactionRepo.CreateNodeWithEdges(ctx, node, relatedNodeIDs); err != nil {
		return nil, nil, appErrors.Wrap(err, "failed to create node in repository")
	}

	// Build edge list for response
	edges := make([]domain.Edge, 0, len(relatedNodeIDs))
	for _, relatedID := range relatedNodeIDs {
		edges = append(edges, domain.Edge{
			SourceID: node.ID,
			TargetID: relatedID,
		})
	}

	log.Printf("DEBUG createNodeCore: successfully created node %s with %d edges", node.ID, len(edges))
	return &node, edges, nil
}

// updateNodeCore handles the actual update logic with optimistic locking retry
func (s *service) updateNodeCore(ctx context.Context, userID, nodeID, content string, tags []string) (*domain.Node, error) {
	if content == "" {
		return nil, appErrors.NewValidation("content cannot be empty")
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Fetch current node
		existingNode, err := s.nodeRepo.FindNodeByID(ctx, userID, nodeID)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to find node")
		}
		if existingNode == nil {
			return nil, appErrors.NewNotFound("node not found")
		}

		// Prepare updated node
		keywords := ExtractKeywords(content)
		updatedNode := *existingNode // Copy existing node
		updatedNode.Content = content
		updatedNode.Keywords = keywords
		updatedNode.Tags = tags
		// Version will be incremented by repository layer

		// Find new connections
		relatedNodes, err := s.keywordRepo.FindNodesByKeywords(ctx, userID, keywords)
		if err != nil {
			log.Printf("WARN: Failed to find related nodes for update: %v", err)
			relatedNodes = []domain.Node{}
		}

		var relatedNodeIDs []string
		for _, rn := range relatedNodes {
			if rn.ID != nodeID {
				relatedNodeIDs = append(relatedNodeIDs, rn.ID)
			}
		}

		// Update with optimistic locking
		err = s.transactionRepo.UpdateNodeAndEdges(ctx, updatedNode, relatedNodeIDs)
		if err == nil {
			// Success - update version for return
			updatedNode.Version++
			return &updatedNode, nil
		}

		// Check if it's an optimistic lock error
		if !repository.IsOptimisticLockError(err) {
			return nil, err
		}

		// Retry with exponential backoff
		if attempt < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<attempt)
			log.Printf("WARN: Optimistic lock conflict on attempt %d, retrying after %v", attempt+1, delay)
			time.Sleep(delay)
		}
	}

	return nil, appErrors.NewValidation("max retries exceeded for optimistic locking")
}

// bulkDeleteCore handles the actual bulk delete logic
func (s *service) bulkDeleteCore(ctx context.Context, userID string, nodeIDs []string) (int, []string, error) {
	if len(nodeIDs) == 0 {
		return 0, nil, appErrors.NewValidation("nodeIds cannot be empty")
	}
	if len(nodeIDs) > 100 {
		return 0, nil, appErrors.NewValidation("cannot delete more than 100 nodes at once")
	}

	var failedNodeIDs []string
	deletedCount := 0

	for _, nodeID := range nodeIDs {
		if err := s.nodeRepo.DeleteNode(ctx, userID, nodeID); err != nil {
			log.Printf("Failed to delete node %s: %v", nodeID, err)
			failedNodeIDs = append(failedNodeIDs, nodeID)
			continue
		}
		deletedCount++
	}

	return deletedCount, failedNodeIDs, nil
}