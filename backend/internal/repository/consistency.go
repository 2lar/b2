package repository

import (
	"context"
	"fmt"
	"time"

	"brain2-backend/internal/domain"
)

// ConsistencyValidator validates data consistency across repository operations
type ConsistencyValidator struct {
	repo Repository
}

// NewConsistencyValidator creates a new consistency validator
func NewConsistencyValidator(repo Repository) *ConsistencyValidator {
	return &ConsistencyValidator{
		repo: repo,
	}
}

// ValidateNodeConsistency validates that a node's data is consistent
func (cv *ConsistencyValidator) ValidateNodeConsistency(ctx context.Context, userID, nodeID string) error {
	node, err := cv.repo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return fmt.Errorf("failed to find node for consistency check: %w", err)
	}

	if node == nil {
		return NewNotFoundError("node", nodeID, userID)
	}

	// Validate node data integrity
	if err := ValidateNode(*node); err != nil {
		return NewRepositoryErrorWithDetails(
			ErrCodeDataCorruption,
			"node data integrity validation failed",
			map[string]interface{}{
				"node_id": nodeID,
				"user_id": userID,
			},
			err,
		)
	}

	// Check for orphaned keywords
	if err := cv.validateNodeKeywords(ctx, userID, nodeID, node.Keywords); err != nil {
		return err
	}

	return nil
}

// ValidateGraphConsistency validates the entire graph consistency for a user
func (cv *ConsistencyValidator) ValidateGraphConsistency(ctx context.Context, userID string) error {
	query := GraphQuery{UserID: userID, IncludeEdges: true}
	graph, err := cv.repo.GetGraphData(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to get graph data for consistency check: %w", err)
	}

	// Create node index for fast lookup
	nodeIndex := make(map[string]bool)
	for _, node := range graph.Nodes {
		nodeIndex[node.ID] = true
	}

	// Validate all edges point to existing nodes
	for _, edge := range graph.Edges {
		if !nodeIndex[edge.SourceID] {
			return NewRepositoryErrorWithDetails(
				ErrCodeDataCorruption,
				"edge references non-existent source node",
				map[string]interface{}{
					"edge_source": edge.SourceID,
					"edge_target": edge.TargetID,
					"user_id":     userID,
				},
				nil,
			)
		}

		if !nodeIndex[edge.TargetID] {
			return NewRepositoryErrorWithDetails(
				ErrCodeDataCorruption,
				"edge references non-existent target node",
				map[string]interface{}{
					"edge_source": edge.SourceID,
					"edge_target": edge.TargetID,
					"user_id":     userID,
				},
				nil,
			)
		}
	}

	// Validate bidirectional edges
	if err := cv.validateBidirectionalEdges(ctx, userID, graph.Edges); err != nil {
		return err
	}

	return nil
}

// validateNodeKeywords validates that all keywords for a node are properly indexed
func (cv *ConsistencyValidator) validateNodeKeywords(_ context.Context, userID, nodeID string, keywords []string) error {
	// This would require a method to query keywords directly from the repository
	// For now, we'll validate that the keywords are properly formatted

	for _, keyword := range keywords {
		if keyword == "" {
			return NewRepositoryErrorWithDetails(
				ErrCodeDataCorruption,
				"node contains empty keyword",
				map[string]interface{}{
					"node_id": nodeID,
					"user_id": userID,
				},
				nil,
			)
		}
	}

	return nil
}

// validateBidirectionalEdges validates that all edges are properly bidirectional
func (cv *ConsistencyValidator) validateBidirectionalEdges(_ context.Context, userID string, edges []domain.Edge) error {
	edgeMap := make(map[string]bool)

	// Build edge map
	for _, edge := range edges {
		key := fmt.Sprintf("%s->%s", edge.SourceID, edge.TargetID)
		edgeMap[key] = true
	}

	// Check for missing reverse edges
	for _, edge := range edges {
		reverseKey := fmt.Sprintf("%s->%s", edge.TargetID, edge.SourceID)
		if !edgeMap[reverseKey] {
			return NewRepositoryErrorWithDetails(
				ErrCodeDataCorruption,
				"missing bidirectional edge",
				map[string]interface{}{
					"source_id":       edge.SourceID,
					"target_id":       edge.TargetID,
					"missing_reverse": reverseKey,
					"user_id":         userID,
				},
				nil,
			)
		}
	}

	return nil
}

// DataCleanupManager handles cleanup of orphaned and invalid data
type DataCleanupManager struct {
	repo Repository
}

// NewDataCleanupManager creates a new data cleanup manager
func NewDataCleanupManager(repo Repository) *DataCleanupManager {
	return &DataCleanupManager{
		repo: repo,
	}
}

// CleanupOptions defines options for data cleanup
type CleanupOptions struct {
	DryRun            bool          // If true, only report what would be cleaned
	MaxAge            time.Duration // Maximum age for data to be considered for cleanup
	BatchSize         int           // Number of items to process in each batch
	CleanupOrphans    bool          // Whether to cleanup orphaned relationships
	CleanupInvalid    bool          // Whether to cleanup invalid data
	CleanupDuplicates bool          // Whether to cleanup duplicate data
}

// DefaultCleanupOptions returns default cleanup options
func DefaultCleanupOptions() CleanupOptions {
	return CleanupOptions{
		DryRun:            false,
		MaxAge:            30 * 24 * time.Hour, // 30 days
		BatchSize:         100,
		CleanupOrphans:    true,
		CleanupInvalid:    true,
		CleanupDuplicates: true,
	}
}

// CleanupResult represents the result of a cleanup operation
type CleanupResult struct {
	NodesProcessed    int      // Number of nodes processed
	EdgesProcessed    int      // Number of edges processed
	OrphanedEdges     int      // Number of orphaned edges found/removed
	InvalidNodes      int      // Number of invalid nodes found/removed
	DuplicateKeywords int      // Number of duplicate keywords found/removed
	Errors            []string // Errors encountered during cleanup
	DryRun            bool     // Whether this was a dry run
}

// CleanupUserData cleans up all data for a specific user
func (dcm *DataCleanupManager) CleanupUserData(ctx context.Context, userID string, options CleanupOptions) (*CleanupResult, error) {
	result := &CleanupResult{
		DryRun: options.DryRun,
	}

	// Get all user data
	query := GraphQuery{UserID: userID, IncludeEdges: true}
	graph, err := dcm.repo.GetGraphData(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get user data for cleanup: %w", err)
	}

	// Cleanup orphaned edges
	if options.CleanupOrphans {
		orphanedCount, err := dcm.cleanupOrphanedEdges(ctx, userID, graph, options)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("orphaned edges cleanup failed: %v", err))
		} else {
			result.OrphanedEdges = orphanedCount
		}
	}

	// Cleanup invalid nodes
	if options.CleanupInvalid {
		invalidCount, err := dcm.cleanupInvalidNodes(ctx, userID, graph.Nodes, options)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid nodes cleanup failed: %v", err))
		} else {
			result.InvalidNodes = invalidCount
		}
	}

	// Cleanup duplicate keywords
	if options.CleanupDuplicates {
		duplicateCount, err := dcm.cleanupDuplicateKeywords(ctx, userID, graph.Nodes, options)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate keywords cleanup failed: %v", err))
		} else {
			result.DuplicateKeywords = duplicateCount
		}
	}

	result.NodesProcessed = len(graph.Nodes)
	result.EdgesProcessed = len(graph.Edges)

	return result, nil
}

// cleanupOrphanedEdges removes edges that point to non-existent nodes
func (dcm *DataCleanupManager) cleanupOrphanedEdges(_ context.Context, _ string, graph *domain.Graph, options CleanupOptions) (int, error) {
	nodeIndex := make(map[string]bool)
	for _, node := range graph.Nodes {
		nodeIndex[node.ID] = true
	}

	orphanedCount := 0

	for _, edge := range graph.Edges {
		isOrphaned := false

		if !nodeIndex[edge.SourceID] {
			isOrphaned = true
		}

		if !nodeIndex[edge.TargetID] {
			isOrphaned = true
		}

		if isOrphaned {
			orphanedCount++

			if !options.DryRun {
				// Note: This would require a method to delete specific edges
				// For now, we'll log the orphaned edge
				fmt.Printf("Would remove orphaned edge: %s -> %s\n", edge.SourceID, edge.TargetID)
			}
		}
	}

	return orphanedCount, nil
}

// cleanupInvalidNodes removes nodes that fail validation
func (dcm *DataCleanupManager) cleanupInvalidNodes(ctx context.Context, userID string, nodes []domain.Node, options CleanupOptions) (int, error) {
	invalidCount := 0

	for _, node := range nodes {
		if err := ValidateNode(node); err != nil {
			invalidCount++

			if !options.DryRun {
				if deleteErr := dcm.repo.DeleteNode(ctx, userID, node.ID); deleteErr != nil {
					return invalidCount, fmt.Errorf("failed to delete invalid node %s: %w", node.ID, deleteErr)
				}
			}
		}
	}

	return invalidCount, nil
}

// cleanupDuplicateKeywords removes duplicate keywords from nodes
func (dcm *DataCleanupManager) cleanupDuplicateKeywords(ctx context.Context, _ string, nodes []domain.Node, options CleanupOptions) (int, error) {
	duplicateCount := 0

	for _, node := range nodes {
		originalKeywords := node.Keywords
		sanitizedKeywords := SanitizeKeywords(originalKeywords)

		if len(sanitizedKeywords) < len(originalKeywords) {
			duplicateCount += len(originalKeywords) - len(sanitizedKeywords)

			if !options.DryRun {
				// Update the node with sanitized keywords
				node.Keywords = sanitizedKeywords
				if err := dcm.repo.UpdateNodeAndEdges(ctx, node, []string{}); err != nil {
					return duplicateCount, fmt.Errorf("failed to update node %s with sanitized keywords: %w", node.ID, err)
				}
			}
		}
	}

	return duplicateCount, nil
}

// IntegrityChecker performs deep integrity checks on repository data
type IntegrityChecker struct {
	repo Repository
}

// NewIntegrityChecker creates a new integrity checker
func NewIntegrityChecker(repo Repository) *IntegrityChecker {
	return &IntegrityChecker{
		repo: repo,
	}
}

// IntegrityReport represents the result of an integrity check
type IntegrityReport struct {
	TotalNodes      int           // Total number of nodes checked
	TotalEdges      int           // Total number of edges checked
	CorruptedNodes  int           // Number of corrupted nodes found
	OrphanedEdges   int           // Number of orphaned edges found
	MissingEdges    int           // Number of missing bidirectional edges
	InvalidKeywords int           // Number of invalid keywords found
	Errors          []string      // Detailed errors found
	CheckDuration   time.Duration // Time taken for the check
}

// CheckIntegrity performs a comprehensive integrity check
func (ic *IntegrityChecker) CheckIntegrity(ctx context.Context, userID string) (*IntegrityReport, error) {
	startTime := time.Now()

	report := &IntegrityReport{}

	// Get all user data
	query := GraphQuery{UserID: userID, IncludeEdges: true}
	graph, err := ic.repo.GetGraphData(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get user data for integrity check: %w", err)
	}

	report.TotalNodes = len(graph.Nodes)
	report.TotalEdges = len(graph.Edges)

	// Check node integrity
	nodeIndex := make(map[string]bool)
	for _, node := range graph.Nodes {
		nodeIndex[node.ID] = true

		// Validate node data
		if err := ValidateNode(node); err != nil {
			report.CorruptedNodes++
			report.Errors = append(report.Errors, fmt.Sprintf("Node %s validation failed: %v", node.ID, err))
		}

		// Check for invalid keywords
		for _, keyword := range node.Keywords {
			if keyword == "" {
				report.InvalidKeywords++
				report.Errors = append(report.Errors, fmt.Sprintf("Node %s has empty keyword", node.ID))
			}
		}
	}

	// Check edge integrity
	edgeMap := make(map[string]bool)
	for _, edge := range graph.Edges {
		// Check for orphaned edges
		if !nodeIndex[edge.SourceID] {
			report.OrphanedEdges++
			report.Errors = append(report.Errors, fmt.Sprintf("Edge references non-existent source node: %s", edge.SourceID))
		}

		if !nodeIndex[edge.TargetID] {
			report.OrphanedEdges++
			report.Errors = append(report.Errors, fmt.Sprintf("Edge references non-existent target node: %s", edge.TargetID))
		}

		// Build edge map for bidirectional check
		key := fmt.Sprintf("%s->%s", edge.SourceID, edge.TargetID)
		edgeMap[key] = true
	}

	// Check for missing bidirectional edges
	for _, edge := range graph.Edges {
		reverseKey := fmt.Sprintf("%s->%s", edge.TargetID, edge.SourceID)
		if !edgeMap[reverseKey] {
			report.MissingEdges++
			report.Errors = append(report.Errors, fmt.Sprintf("Missing bidirectional edge: %s -> %s", edge.TargetID, edge.SourceID))
		}
	}

	report.CheckDuration = time.Since(startTime)

	return report, nil
}

// RepairRepository attempts to repair common data integrity issues
func (ic *IntegrityChecker) RepairRepository(ctx context.Context, userID string) (*IntegrityReport, error) {
	// First, get integrity report
	report, err := ic.CheckIntegrity(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check integrity before repair: %w", err)
	}

	// If no issues found, return early
	if report.CorruptedNodes == 0 && report.OrphanedEdges == 0 && report.MissingEdges == 0 && report.InvalidKeywords == 0 {
		return report, nil
	}

	// Perform repairs using cleanup manager
	cleanupManager := NewDataCleanupManager(ic.repo)
	cleanupOptions := DefaultCleanupOptions()
	cleanupOptions.DryRun = false // Actually perform repairs

	_, err = cleanupManager.CleanupUserData(ctx, userID, cleanupOptions)
	if err != nil {
		return report, fmt.Errorf("failed to repair repository: %w", err)
	}

	// Generate post-repair report
	return ic.CheckIntegrity(ctx, userID)
}
