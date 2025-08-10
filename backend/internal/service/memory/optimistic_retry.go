package memory

import (
	"context"
	"time"
	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
)

const (
	maxRetries = 3
	baseDelay  = 100 * time.Millisecond
)

// UpdateNodeWithRetry performs an optimistic update with automatic retry logic.
// It fetches the latest version of the node, applies the update function, and retries on version conflicts.
func (s *service) UpdateNodeWithRetry(ctx context.Context, userID, nodeID string, updateFn func(*domain.Node) error) (*domain.Node, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Fetch the latest version of the node
		node, err := s.nodeRepo.FindNodeByID(ctx, userID, nodeID)
		if err != nil {
			return nil, err
		}
		
		if node == nil {
			return nil, repository.NewNotFoundError("node", nodeID, userID)
		}

		// Apply the update function to modify the node
		if err := updateFn(node); err != nil {
			return nil, err
		}

		// Try to save the updated node
		err = s.transactionRepo.UpdateNodeAndEdges(ctx, *node, []string{}) // No edge updates in retry logic
		if err == nil {
			// Success - return the updated node
			return node, nil
		}

		// Check if it's an optimistic lock error
		if repository.IsConflict(err) && attempt < maxRetries-1 {
			// Wait with exponential backoff before retrying
			delay := baseDelay * time.Duration(1<<attempt) // 100ms, 200ms, 400ms
			time.Sleep(delay)
			continue
		}

		// Non-retryable error or max retries exceeded
		return nil, err
	}

	return nil, repository.NewRepositoryError(repository.ErrCodeOptimisticLock, "max retries exceeded for node update", nil)
}

// UpdateNodeWithEdgesRetry performs an optimistic update with edges and automatic retry logic.
func (s *service) UpdateNodeWithEdgesRetry(ctx context.Context, userID, nodeID string, relatedNodeIDs []string, updateFn func(*domain.Node) error) (*domain.Node, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Fetch the latest version of the node
		node, err := s.nodeRepo.FindNodeByID(ctx, userID, nodeID)
		if err != nil {
			return nil, err
		}
		
		if node == nil {
			return nil, repository.NewNotFoundError("node", nodeID, userID)
		}

		// Apply the update function to modify the node
		if err := updateFn(node); err != nil {
			return nil, err
		}

		// Try to save the updated node with edges
		err = s.transactionRepo.UpdateNodeAndEdges(ctx, *node, relatedNodeIDs)
		if err == nil {
			// Success - return the updated node
			return node, nil
		}

		// Check if it's an optimistic lock error
		if repository.IsConflict(err) && attempt < maxRetries-1 {
			// Wait with exponential backoff before retrying
			delay := baseDelay * time.Duration(1<<attempt) // 100ms, 200ms, 400ms
			time.Sleep(delay)
			continue
		}

		// Non-retryable error or max retries exceeded
		return nil, err
	}

	return nil, repository.NewRepositoryError(repository.ErrCodeOptimisticLock, "max retries exceeded for node update with edges", nil)
}

// SafeUpdateNode provides a safe way to update node content with optimistic locking.
func (s *service) SafeUpdateNode(ctx context.Context, userID, nodeID, newContent string, newTags []string) (*domain.Node, error) {
	return s.UpdateNodeWithRetry(ctx, userID, nodeID, func(node *domain.Node) error {
		// Update the node fields
		node.Content = newContent
		node.Tags = newTags
		// Keywords will be extracted and updated automatically
		node.Keywords = ExtractKeywords(newContent)
		return nil
	})
}

// SafeUpdateNodeWithConnections provides a safe way to update node and its connections with optimistic locking.
func (s *service) SafeUpdateNodeWithConnections(ctx context.Context, userID, nodeID, newContent string, newTags []string, relatedNodeIDs []string) (*domain.Node, error) {
	return s.UpdateNodeWithEdgesRetry(ctx, userID, nodeID, relatedNodeIDs, func(node *domain.Node) error {
		// Update the node fields
		node.Content = newContent
		node.Tags = newTags
		// Keywords will be extracted and updated automatically
		node.Keywords = ExtractKeywords(newContent)
		return nil
	})
}