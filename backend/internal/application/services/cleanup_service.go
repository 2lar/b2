// Package services contains application services including cleanup operations.
package services

import (
	"context"
	"fmt"
	"log"

	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
)

// CleanupService handles async cleanup of resources after node deletion.
// This service is designed to be called asynchronously after a node has been deleted,
// ensuring all related resources (edges, idempotency records, etc.) are cleaned up.
type CleanupService struct {
	nodeRepo         repository.NodeRepository
	edgeRepo         repository.EdgeRepository
	edgeWriter       repository.EdgeWriter
	idempotencyStore repository.IdempotencyStore
	uowFactory       repository.UnitOfWorkFactory
}

// NewCleanupService creates a new cleanup service with required dependencies.
func NewCleanupService(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	edgeWriter repository.EdgeWriter,
	idempotencyStore repository.IdempotencyStore,
	uowFactory repository.UnitOfWorkFactory,
) *CleanupService {
	return &CleanupService{
		nodeRepo:         nodeRepo,
		edgeRepo:         edgeRepo,
		edgeWriter:       edgeWriter,
		idempotencyStore: idempotencyStore,
		uowFactory:       uowFactory,
	}
}

// CleanupNodeResiduals removes all resources associated with a deleted node.
// This includes:
// - All edges where the node is either source or target
// - Any keyword indexes (handled by repository)
// - Any other related entities
func (s *CleanupService) CleanupNodeResiduals(ctx context.Context, userID, nodeID string) error {
	log.Printf("Starting cleanup for node: UserID=%s, NodeID=%s", userID, nodeID)

	// Validate inputs
	if userID == "" || nodeID == "" {
		return appErrors.NewValidation("userID and nodeID are required for cleanup")
	}

	// Parse domain identifiers
	userIDVO, err := shared.ParseUserID(userID)
	if err != nil {
		return appErrors.Wrap(err, "invalid user ID")
	}

	nodeIDVO, err := shared.ParseNodeID(nodeID)
	if err != nil {
		return appErrors.Wrap(err, "invalid node ID")
	}

	// No need to add userID to context - repositories use entity userIDs

	// Create a unit of work for transactional cleanup
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return appErrors.Wrap(err, "failed to create unit of work")
	}

	// Start transaction
	if err := uow.Begin(ctx); err != nil {
		return appErrors.Wrap(err, "failed to begin transaction")
	}

	// Track whether commit was called
	var commitCalled bool
	
	// Ensure proper cleanup
	defer func() {
		if r := recover(); r != nil {
			// Attempt rollback on panic
			if rollbackErr := uow.Rollback(); rollbackErr != nil {
				log.Printf("Failed to rollback on panic: %v", rollbackErr)
			}
			// Re-panic to let it bubble up
			panic(r)
		} else if !commitCalled {
			// Only rollback if commit wasn't called
			uow.Rollback()
		}
	}()

	// Step 1: Find and delete all edges connected to this node
	log.Printf("Cleaning up edges for node %s", nodeID)
	
	// Find all edges where this node is involved
	edgeQuery := repository.EdgeQuery{
		UserID: userID,
		// We need to find edges where node is either source or target
	}
	
	// Get edges where node is the source
	edgeQuery.SourceID = nodeID
	sourceEdges, err := uow.Edges().FindEdges(ctx, edgeQuery)
	if err != nil {
		log.Printf("WARNING: Failed to find source edges: %v", err)
		// Continue with cleanup even if this fails
	}

	// Get edges where node is the target
	edgeQuery.SourceID = ""
	edgeQuery.TargetID = nodeID
	targetEdges, err := uow.Edges().FindEdges(ctx, edgeQuery)
	if err != nil {
		log.Printf("WARNING: Failed to find target edges: %v", err)
		// Continue with cleanup even if this fails
	}

	// Combine all edges
	allEdges := append(sourceEdges, targetEdges...)
	
	// Delete each edge
	deletedCount := 0
	failedCount := 0
	
	for _, edge := range allEdges {
		// Use the EdgeWriter interface if available, otherwise try through repository
		if s.edgeWriter != nil {
			if err := s.edgeWriter.Delete(ctx, edge.UserID(), edge.ID); err != nil {
				log.Printf("WARNING: Failed to delete edge %s: %v", edge.ID.String(), err)
				failedCount++
			} else {
				deletedCount++
			}
		}
	}

	log.Printf("Edge cleanup complete: deleted=%d, failed=%d, total=%d", 
		deletedCount, failedCount, len(allEdges))

	// Step 2: Clean up any orphaned edges using canonical storage pattern
	// This handles edges that might be stored in the reverse direction
	if err := s.cleanupCanonicalEdges(ctx, userIDVO, nodeIDVO); err != nil {
		log.Printf("WARNING: Failed to cleanup canonical edges: %v", err)
		// Don't fail the entire operation
	}

	// Commit the transaction
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false
		return appErrors.Wrap(err, "failed to commit cleanup transaction")
	}

	log.Printf("Successfully cleaned up residuals for node %s", nodeID)
	return nil
}

// cleanupCanonicalEdges handles cleanup of edges stored in canonical format
func (s *CleanupService) cleanupCanonicalEdges(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error {
	// If we have access to EdgeRepository with DeleteByNode method, use it
	type edgeDeleter interface {
		DeleteByNode(ctx context.Context, userID shared.UserID, nodeID shared.NodeID) error
	}

	if deleter, ok := s.edgeWriter.(edgeDeleter); ok {
		log.Printf("Using DeleteByNode for efficient edge cleanup")
		return deleter.DeleteByNode(ctx, userID, nodeID)
	}

	// Fallback: manual deletion (already handled above)
	return nil
}

// CleanupIdempotencyRecords removes idempotency records related to node operations.
// This is a best-effort operation to clean up old idempotency keys.
func (s *CleanupService) CleanupIdempotencyRecords(ctx context.Context, userID, nodeID string) error {
	if s.idempotencyStore == nil {
		log.Printf("No idempotency store configured, skipping cleanup")
		return nil
	}

	log.Printf("Cleaning up idempotency records for user %s, node %s", userID, nodeID)

	// Idempotency keys are typically structured as:
	// IDEMPOTENCY#{userID}#CREATE_NODE#{hash}
	// IDEMPOTENCY#{userID}#UPDATE_NODE#{hash}
	// etc.

	// Since we can't query by partial keys efficiently, and idempotency records
	// typically have TTL, we'll just log this for now
	// In production, these would expire naturally via TTL

	// If the idempotency store supports cleanup by pattern, use it
	type patternCleaner interface {
		CleanupByPattern(ctx context.Context, userID string, pattern string) error
	}

	if cleaner, ok := s.idempotencyStore.(patternCleaner); ok {
		// Try to cleanup records related to this node
		patterns := []string{
			fmt.Sprintf("*NODE*%s*", nodeID),
			fmt.Sprintf("*%s*", nodeID),
		}
		
		for _, pattern := range patterns {
			if err := cleaner.CleanupByPattern(ctx, userID, pattern); err != nil {
				log.Printf("WARNING: Failed to cleanup idempotency pattern %s: %v", pattern, err)
			}
		}
	}

	return nil
}

// CleanupOrphanedEdges finds and removes edges where either source or target node doesn't exist.
// This is a maintenance operation that can be run periodically.
func (s *CleanupService) CleanupOrphanedEdges(ctx context.Context, userID string) error {
	log.Printf("Starting orphaned edge cleanup for user %s", userID)

	// No need to add userID to context - repositories use entity userIDs

	// Query all edges for the user
	edgeQuery := repository.EdgeQuery{
		UserID: userID,
	}

	edges, err := s.edgeRepo.FindEdges(ctx, edgeQuery)
	if err != nil {
		return appErrors.Wrap(err, "failed to find edges")
	}

	orphanedCount := 0
	for _, edge := range edges {
		// Check if source node exists
		sourceExists := s.nodeExists(ctx, userID, edge.SourceID.String())
		
		// Check if target node exists
		targetExists := s.nodeExists(ctx, userID, edge.TargetID.String())

		// If either node doesn't exist, this edge is orphaned
		if !sourceExists || !targetExists {
			log.Printf("Found orphaned edge %s: source_exists=%v, target_exists=%v",
				edge.ID.String(), sourceExists, targetExists)

			// Delete the orphaned edge
			if s.edgeWriter != nil {
				if err := s.edgeWriter.Delete(ctx, edge.UserID(), edge.ID); err != nil {
					log.Printf("WARNING: Failed to delete orphaned edge %s: %v", 
						edge.ID.String(), err)
				} else {
					orphanedCount++
				}
			}
		}
	}

	log.Printf("Orphaned edge cleanup complete: removed %d edges", orphanedCount)
	return nil
}

// nodeExists checks if a node exists in the repository
func (s *CleanupService) nodeExists(ctx context.Context, userID, nodeID string) bool {
	query := repository.NodeQuery{
		UserID:  userID,
		NodeIDs: []string{nodeID},
	}

	nodes, err := s.nodeRepo.FindNodes(ctx, query)
	if err != nil || len(nodes) == 0 {
		return false
	}

	// Check if the specific node ID matches
	for _, node := range nodes {
		if node.ID().String() == nodeID {
			return true
		}
	}

	return false
}

// BulkCleanupNodes handles cleanup for multiple deleted nodes.
// This is more efficient than individual cleanup when deleting many nodes.
func (s *CleanupService) BulkCleanupNodes(ctx context.Context, userID string, nodeIDs []string) error {
	log.Printf("Starting bulk cleanup for %d nodes", len(nodeIDs))

	successCount := 0
	failedCount := 0

	for _, nodeID := range nodeIDs {
		if err := s.CleanupNodeResiduals(ctx, userID, nodeID); err != nil {
			log.Printf("Failed to cleanup node %s: %v", nodeID, err)
			failedCount++
		} else {
			successCount++
		}
	}

	log.Printf("Bulk cleanup complete: success=%d, failed=%d", successCount, failedCount)

	if failedCount > 0 {
		return fmt.Errorf("failed to cleanup %d out of %d nodes", failedCount, len(nodeIDs))
	}

	return nil
}