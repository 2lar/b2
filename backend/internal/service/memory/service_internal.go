package memory

import (
	"context"
	"log"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/domain/services"
	appErrors "brain2-backend/pkg/errors"
)

const (
	maxRetries = 3
	baseDelay  = 100 * time.Millisecond
)

// Internal methods that won't be exposed in the interface

// createNodeCore handles the actual node creation logic using rich domain models
func (s *service) createNodeCore(ctx context.Context, userID, content string, tags []string) (*domain.Node, []*domain.Edge, error) {
	// Create rich domain node with full business rule validation
	userIDVO, err := domain.NewUserID(userID)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, "invalid user ID")
	}
	
	contentVO, err := domain.NewContent(content)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, "invalid content")
	}
	
	tagsVO := domain.NewTags(tags...)
	
	// Create rich domain node - this enforces all business rules
	node, err := domain.NewNode(userIDVO, contentVO, tagsVO)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, "failed to create node")
	}

	log.Printf("DEBUG createNodeCore: created node ID=%s, searching for related nodes with keywords=%v", 
		node.ID().String(), node.Keywords().ToSlice())

	// Find related nodes using keywords from rich domain model
	relatedNodes, err := s.keywordRepo.FindNodesByKeywords(ctx, userID, node.Keywords().ToSlice())
	if err != nil {
		// Log but don't fail - connections are non-critical
		log.Printf("WARN: Failed to find related nodes: %v", err)
		relatedNodes = []*domain.Node{}
	}

	// Use domain service for connection analysis
	connectionAnalyzer := services.NewConnectionAnalyzer(0.3, 10, 0.2)
	potentialConnections, err := connectionAnalyzer.FindPotentialConnections(node, relatedNodes)
	if err != nil {
		log.Printf("WARN: Failed to analyze connections: %v", err)
		potentialConnections = []*services.ConnectionCandidate{}
	}

	// Create edges from potential connections
	edges := make([]*domain.Edge, 0)
	var relatedNodeIDs []string
	
	for _, connection := range potentialConnections {
		// Create rich domain edge with calculated weight
		edge, err := domain.NewEdge(node.ID(), connection.Node.ID(), userIDVO, connection.SimilarityScore)
		if err != nil {
			log.Printf("WARN: Failed to create edge: %v", err)
			continue
		}
		
		edges = append(edges, edge)
		relatedNodeIDs = append(relatedNodeIDs, connection.Node.ID().String())
	}

	log.Printf("DEBUG createNodeCore: creating node %s with %d edges to nodes: %v", 
		node.ID().String(), len(relatedNodeIDs), relatedNodeIDs)

	// Create node with edges in transaction
	if err := s.transactionRepo.CreateNodeWithEdges(ctx, node, relatedNodeIDs); err != nil {
		return nil, nil, appErrors.Wrap(err, "failed to create node with edges")
	}
	
	// Create individual edges in repository (if the repository supports it)
	for _, edge := range edges {
		if err := s.edgeRepo.CreateEdge(ctx, edge); err != nil {
			log.Printf("WARN: Failed to create edge: %v", err)
			// Continue - don't fail entire operation for edge creation
		}
	}

	// Mark domain events as committed (in a real implementation, you'd publish them first)
	node.MarkEventsAsCommitted()
	for _, edge := range edges {
		edge.MarkEventsAsCommitted()
	}

	log.Printf("DEBUG createNodeCore: created node %s with %d edges", node.ID().String(), len(edges))
	return node, edges, nil
}

// updateNodeCore handles the actual update logic with optimistic locking retry
func (s *service) updateNodeCore(ctx context.Context, userID, nodeID, content string, tags []string) (*domain.Node, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Fetch current node
		existingNode, err := s.nodeRepo.FindNodeByID(ctx, userID, nodeID)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to find node")
		}
		if existingNode == nil {
			return nil, appErrors.NewNotFound("node not found")
		}

		// Update content if provided
		if content != "" {
			newContent, err := domain.NewContent(content)
			if err != nil {
				return nil, appErrors.Wrap(err, "invalid content")
			}
			
			if err := existingNode.UpdateContent(newContent); err != nil {
				return nil, appErrors.Wrap(err, "failed to update content")
			}
		}

		// Update tags if provided
		if len(tags) > 0 {
			newTags := domain.NewTags(tags...)
			if err := existingNode.UpdateTags(newTags); err != nil {
				return nil, appErrors.Wrap(err, "failed to update tags")
			}
		}

		// Try to save - this might fail due to optimistic locking
		if err := s.nodeRepo.CreateNodeAndKeywords(ctx, existingNode); err != nil {
			if isConflictError(err) && attempt < maxRetries-1 {
				// Wait and retry
				time.Sleep(baseDelay * time.Duration(1<<attempt))
				log.Printf("DEBUG updateNodeCore: optimistic lock conflict, retrying attempt %d", attempt+1)
				continue
			}
			return nil, appErrors.Wrap(err, "failed to update node")
		}

		// Mark domain events as committed
		existingNode.MarkEventsAsCommitted()

		log.Printf("DEBUG updateNodeCore: updated node %s after %d attempts", existingNode.ID().String(), attempt+1)
		return existingNode, nil
	}

	return nil, appErrors.NewValidation("failed to update node after maximum retries due to version conflicts")
}

// bulkDeleteCore handles the core bulk delete logic
func (s *service) bulkDeleteCore(ctx context.Context, userID string, nodeIDs []string) (int, []string, error) {
	var failed []string
	successCount := 0

	for _, nodeID := range nodeIDs {
		if err := s.nodeRepo.DeleteNode(ctx, userID, nodeID); err != nil {
			log.Printf("WARN: Failed to delete node %s: %v", nodeID, err)
			failed = append(failed, nodeID)
		} else {
			successCount++
		}
	}

	log.Printf("DEBUG bulkDeleteCore: deleted %d nodes, %d failed", successCount, len(failed))
	return successCount, failed, nil
}

// Helper functions

func isConflictError(err error) bool {
	// Check if error is a version conflict/optimistic locking error
	// This would depend on your specific error types
	return false // Simplified for now
}