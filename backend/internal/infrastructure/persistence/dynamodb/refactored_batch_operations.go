// Package dynamodb demonstrates refactored complex functions with improved naming
// and clarity, addressing code quality issues found in the analysis.
package dynamodb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// ============================================================================
// REFACTORED BATCH OPERATIONS WITH IMPROVED NAMING AND CLARITY
// ============================================================================

// BatchOperationConfig contains configuration for batch operations.
type BatchOperationConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	BackoffFactor   int
	MaxChunkSize    int
	TableName       string
}

// DefaultBatchConfig returns sensible defaults for batch operations.
func DefaultBatchConfig(tableName string) BatchOperationConfig {
	return BatchOperationConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		BackoffFactor: 2,
		MaxChunkSize:  25, // DynamoDB batch limit
		TableName:     tableName,
	}
}

// BatchDeleteResult contains the results of a batch delete operation.
type BatchDeleteResult struct {
	SuccessfullyDeleted []string
	FailedToDelete      []string
	TotalProcessed      int
	TotalRequested      int
}

// BatchDeleteOrchestrator manages the orchestration of batch delete operations.
// This replaces the poorly named "processBatchDeleteChunk" with a clear,
// responsibility-focused class that follows the Single Responsibility Principle.
type BatchDeleteOrchestrator struct {
	client *dynamodb.Client
	config BatchOperationConfig
	logger *zap.Logger
}

// NewBatchDeleteOrchestrator creates a new batch delete orchestrator.
func NewBatchDeleteOrchestrator(client *dynamodb.Client, config BatchOperationConfig, logger *zap.Logger) *BatchDeleteOrchestrator {
	return &BatchDeleteOrchestrator{
		client: client,
		config: config,
		logger: logger,
	}
}

// ExecuteBatchDelete executes a batch delete operation for multiple node IDs.
// This replaces the original BatchDeleteNodes method with a clearer name and structure.
func (o *BatchDeleteOrchestrator) ExecuteBatchDelete(ctx context.Context, userID string, nodeIDs []string) (*BatchDeleteResult, error) {
	result := &BatchDeleteResult{
		SuccessfullyDeleted: make([]string, 0, len(nodeIDs)),
		FailedToDelete:      make([]string, 0),
		TotalRequested:      len(nodeIDs),
	}

	o.logger.Info("Starting batch delete operation",
		zap.String("user_id", userID),
		zap.Int("total_nodes", len(nodeIDs)))

	// Process nodes in chunks to respect DynamoDB limits
	chunks := o.divideIntoChunks(nodeIDs)
	
	for i, chunk := range chunks {
		chunkResult, err := o.executeChunkWithRetry(ctx, userID, chunk, i+1, len(chunks))
		if err != nil {
			o.logger.Error("Chunk processing failed completely",
				zap.Int("chunk_number", i+1),
				zap.Error(err))
			// Mark all items in failed chunk as failed
			result.FailedToDelete = append(result.FailedToDelete, chunk...)
		} else {
			result.SuccessfullyDeleted = append(result.SuccessfullyDeleted, chunkResult.SuccessfullyDeleted...)
			result.FailedToDelete = append(result.FailedToDelete, chunkResult.FailedToDelete...)
		}
	}

	result.TotalProcessed = len(result.SuccessfullyDeleted) + len(result.FailedToDelete)

	o.logger.Info("Batch delete operation completed",
		zap.Int("successful", len(result.SuccessfullyDeleted)),
		zap.Int("failed", len(result.FailedToDelete)),
		zap.Int("total_processed", result.TotalProcessed))

	return result, nil
}

// executeChunkWithRetry processes a single chunk with retry logic.
// This function replaces the original "processBatchDeleteChunk" with a clearer name
// and improved structure that separates concerns.
func (o *BatchDeleteOrchestrator) executeChunkWithRetry(ctx context.Context, userID string, nodeIDs []string, chunkNumber, totalChunks int) (*BatchDeleteResult, error) {
	result := &BatchDeleteResult{
		SuccessfullyDeleted: make([]string, 0, len(nodeIDs)),
		FailedToDelete:      make([]string, 0),
		TotalRequested:      len(nodeIDs),
	}

	unprocessedNodes := nodeIDs
	retryDelay := o.config.InitialDelay

	o.logger.Debug("Processing chunk with retry logic",
		zap.Int("chunk_number", chunkNumber),
		zap.Int("total_chunks", totalChunks),
		zap.Int("chunk_size", len(nodeIDs)))

	for attempt := 0; attempt <= o.config.MaxRetries && len(unprocessedNodes) > 0; attempt++ {
		if attempt > 0 {
			o.waitForRetry(retryDelay, attempt, len(unprocessedNodes))
			retryDelay = o.calculateNextRetryDelay(retryDelay)
		}

		// Attempt to delete the unprocessed nodes
		attemptResult, err := o.executeSingleDeleteAttempt(ctx, userID, unprocessedNodes, attempt)
		if err != nil {
			o.logger.Error("Delete attempt failed",
				zap.Int("attempt", attempt),
				zap.Error(err))
			// On complete failure, mark remaining nodes as failed
			result.FailedToDelete = append(result.FailedToDelete, unprocessedNodes...)
			break
		}

		// Update results and determine what still needs processing
		result.SuccessfullyDeleted = append(result.SuccessfullyDeleted, attemptResult.SuccessfullyDeleted...)
		unprocessedNodes = attemptResult.UnprocessedNodes

		o.logger.Debug("Delete attempt completed",
			zap.Int("attempt", attempt),
			zap.Int("successfully_deleted", len(attemptResult.SuccessfullyDeleted)),
			zap.Int("still_unprocessed", len(unprocessedNodes)))
	}

	// Any remaining unprocessed nodes are considered failed
	if len(unprocessedNodes) > 0 {
		result.FailedToDelete = append(result.FailedToDelete, unprocessedNodes...)
		o.logger.Warn("Some nodes could not be deleted after all retry attempts",
			zap.Int("failed_count", len(unprocessedNodes)))
	}

	result.TotalProcessed = len(result.SuccessfullyDeleted) + len(result.FailedToDelete)
	return result, nil
}

// SingleDeleteAttemptResult contains the result of a single delete attempt.
type SingleDeleteAttemptResult struct {
	SuccessfullyDeleted []string
	UnprocessedNodes    []string
}

// executeSingleDeleteAttempt performs a single delete attempt on DynamoDB.
// This function extracts the core deletion logic with a clear, descriptive name.
func (o *BatchDeleteOrchestrator) executeSingleDeleteAttempt(ctx context.Context, userID string, nodeIDs []string, _ int) (*SingleDeleteAttemptResult, error) {
	// Build DynamoDB write requests
	writeRequests := o.buildDeleteRequests(userID, nodeIDs)
	
	// Execute the batch write operation
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			o.config.TableName: writeRequests,
		},
	}

	output, err := o.client.BatchWriteItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("batch write operation failed: %w", err)
	}

	// Analyze the results to determine success/failure
	unprocessedNodeIDs := o.extractUnprocessedNodeIDs(output)
	successfullyDeleted := o.calculateSuccessfullyDeleted(nodeIDs, unprocessedNodeIDs)

	return &SingleDeleteAttemptResult{
		SuccessfullyDeleted: successfullyDeleted,
		UnprocessedNodes:    unprocessedNodeIDs,
	}, nil
}

// buildDeleteRequests creates DynamoDB delete requests for the given node IDs.
// This function extracts request building logic with a clear, descriptive name.
func (o *BatchDeleteOrchestrator) buildDeleteRequests(userID string, nodeIDs []string) []types.WriteRequest {
	writeRequests := make([]types.WriteRequest, 0, len(nodeIDs))

	for _, nodeID := range nodeIDs {
		deleteRequest := types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: o.buildUserPartitionKey(userID)},
					"SK": &types.AttributeValueMemberS{Value: o.buildNodeSortKey(nodeID)},
				},
			},
		}
		writeRequests = append(writeRequests, deleteRequest)
	}

	return writeRequests
}

// extractUnprocessedNodeIDs extracts node IDs from unprocessed items in DynamoDB response.
// This function extracts response parsing logic with a clear, descriptive name.
func (o *BatchDeleteOrchestrator) extractUnprocessedNodeIDs(output *dynamodb.BatchWriteItemOutput) []string {
	if output.UnprocessedItems == nil {
		return []string{}
	}

	unprocessedRequests, exists := output.UnprocessedItems[o.config.TableName]
	if !exists || len(unprocessedRequests) == 0 {
		return []string{}
	}

	unprocessedNodeIDs := make([]string, 0, len(unprocessedRequests))

	for _, request := range unprocessedRequests {
		if request.DeleteRequest != nil {
			nodeID := o.extractNodeIDFromSortKey(request.DeleteRequest.Key)
			if nodeID != "" {
				unprocessedNodeIDs = append(unprocessedNodeIDs, nodeID)
			}
		}
	}

	return unprocessedNodeIDs
}

// calculateSuccessfullyDeleted determines which nodes were successfully deleted.
// This function clearly calculates success by exclusion with a descriptive name.
func (o *BatchDeleteOrchestrator) calculateSuccessfullyDeleted(requestedNodes, unprocessedNodes []string) []string {
	unprocessedSet := make(map[string]bool, len(unprocessedNodes))
	for _, nodeID := range unprocessedNodes {
		unprocessedSet[nodeID] = true
	}

	successfullyDeleted := make([]string, 0, len(requestedNodes)-len(unprocessedNodes))
	for _, nodeID := range requestedNodes {
		if !unprocessedSet[nodeID] {
			successfullyDeleted = append(successfullyDeleted, nodeID)
		}
	}

	return successfullyDeleted
}

// ============================================================================
// UTILITY METHODS WITH CLEAR NAMES
// ============================================================================

// divideIntoChunks divides a slice of node IDs into chunks of maximum size.
func (o *BatchDeleteOrchestrator) divideIntoChunks(nodeIDs []string) [][]string {
	var chunks [][]string
	
	for i := 0; i < len(nodeIDs); i += o.config.MaxChunkSize {
		end := i + o.config.MaxChunkSize
		if end > len(nodeIDs) {
			end = len(nodeIDs)
		}
		chunks = append(chunks, nodeIDs[i:end])
	}
	
	return chunks
}

// waitForRetry implements exponential backoff for retry delays.
func (o *BatchDeleteOrchestrator) waitForRetry(delay time.Duration, attemptNumber, unprocessedCount int) {
	o.logger.Debug("Waiting before retry",
		zap.Duration("delay", delay),
		zap.Int("attempt", attemptNumber),
		zap.Int("unprocessed_count", unprocessedCount))
	
	time.Sleep(delay)
}

// calculateNextRetryDelay calculates the next retry delay using exponential backoff.
func (o *BatchDeleteOrchestrator) calculateNextRetryDelay(currentDelay time.Duration) time.Duration {
	return time.Duration(int(currentDelay) * o.config.BackoffFactor)
}

// buildUserPartitionKey creates a consistent partition key for a user.
func (o *BatchDeleteOrchestrator) buildUserPartitionKey(userID string) string {
	return fmt.Sprintf("USER#%s", userID)
}

// buildNodeSortKey creates a consistent sort key for a node.
func (o *BatchDeleteOrchestrator) buildNodeSortKey(nodeID string) string {
	return fmt.Sprintf("NODE#%s", nodeID)
}

// extractNodeIDFromSortKey extracts the node ID from a DynamoDB sort key.
func (o *BatchDeleteOrchestrator) extractNodeIDFromSortKey(key map[string]types.AttributeValue) string {
	sk, exists := key["SK"]
	if !exists {
		return ""
	}

	skValue, ok := sk.(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}

	if strings.HasPrefix(skValue.Value, "NODE#") {
		return strings.TrimPrefix(skValue.Value, "NODE#")
	}

	return ""
}

// ============================================================================
// REFACTORED NODE REPOSITORY INTEGRATION
// ============================================================================

// RefactoredNodeRepository demonstrates how to integrate the improved batch operations.
type RefactoredNodeRepository struct {
	client             *dynamodb.Client
	tableName          string
	batchOrchestrator  *BatchDeleteOrchestrator
	logger             *zap.Logger
}

// NewRefactoredNodeRepository creates a new node repository with improved batch operations.
func NewRefactoredNodeRepository(client *dynamodb.Client, tableName string, logger *zap.Logger) *RefactoredNodeRepository {
	config := DefaultBatchConfig(tableName)
	batchOrchestrator := NewBatchDeleteOrchestrator(client, config, logger)

	return &RefactoredNodeRepository{
		client:            client,
		tableName:         tableName,
		batchOrchestrator: batchOrchestrator,
		logger:            logger,
	}
}

// BatchDeleteNodes deletes multiple nodes using the improved batch orchestrator.
// This method replaces the original complex method with a clear delegation pattern.
func (r *RefactoredNodeRepository) BatchDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string, err error) {
	// Input validation with clear error messages
	if userID == "" {
		return nil, nil, fmt.Errorf("user ID cannot be empty")
	}
	
	if len(nodeIDs) == 0 {
		return []string{}, []string{}, nil
	}

	r.logger.Info("Starting batch delete operation",
		zap.String("user_id", userID),
		zap.Int("node_count", len(nodeIDs)))

	// Delegate to the specialized batch orchestrator
	result, err := r.batchOrchestrator.ExecuteBatchDelete(ctx, userID, nodeIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("batch delete operation failed: %w", err)
	}

	return result.SuccessfullyDeleted, result.FailedToDelete, nil
}

// ============================================================================
// EXAMPLE OF OTHER REFACTORED FUNCTIONS WITH BETTER NAMING
// ============================================================================

// ProcessComplexValidation demonstrates refactoring a complex validation function.
// Original name might have been: validateAndProcessNodeInput
func (r *RefactoredNodeRepository) ValidateNodeDataAndApplyBusinessRules(ctx context.Context, nodeData map[string]interface{}) (*ValidationResult, error) {
	validator := NewNodeDataValidator()
	
	// Step 1: Validate structure
	structuralErrors := validator.ValidateNodeStructure(nodeData)
	if len(structuralErrors) > 0 {
		return &ValidationResult{
			IsValid: false,
			Errors:  structuralErrors,
		}, nil
	}
	
	// Step 2: Apply business rules
	businessRuleErrors := validator.ApplyBusinessRules(nodeData)
	if len(businessRuleErrors) > 0 {
		return &ValidationResult{
			IsValid: false,
			Errors:  businessRuleErrors,
		}, nil
	}
	
	// Step 3: Normalize data
	normalizedData := validator.NormalizeNodeData(nodeData)
	
	return &ValidationResult{
		IsValid:        true,
		NormalizedData: normalizedData,
	}, nil
}

// TransformAndEnrichNodeForResponse demonstrates refactoring a complex transformation function.
// Original name might have been: processNodeForOutput
func (r *RefactoredNodeRepository) TransformAndEnrichNodeForResponse(ctx context.Context, rawNode map[string]interface{}, includeRelations bool) (*EnrichedNodeResponse, error) {
	transformer := NewNodeTransformer()
	
	// Step 1: Basic transformation
	basicNode, err := transformer.TransformRawDataToNodeModel(rawNode)
	if err != nil {
		return nil, fmt.Errorf("failed to transform raw node data: %w", err)
	}
	
	// Step 2: Enrich with metadata
	enrichedNode := transformer.AddMetadataToNode(basicNode)
	
	// Step 3: Load relationships if requested
	if includeRelations {
		relationships, err := r.loadNodeRelationships(ctx, basicNode.ID)
		if err != nil {
			r.logger.Warn("Failed to load node relationships", zap.Error(err))
			// Continue without relationships rather than failing
		} else {
			enrichedNode.Relationships = relationships
		}
	}
	
	return enrichedNode, nil
}

// loadNodeRelationships loads relationships for a node with a clear, descriptive name.
func (r *RefactoredNodeRepository) loadNodeRelationships(_ context.Context, _ string) (*NodeRelationships, error) {
	// Implementation would load edges, categories, etc.
	return &NodeRelationships{}, nil
}

// ============================================================================
// SUPPORTING TYPES FOR REFACTORED FUNCTIONS
// ============================================================================

// ValidationResult represents the result of node validation.
type ValidationResult struct {
	IsValid        bool
	Errors         []ValidationError
	NormalizedData map[string]interface{}
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string
	Message string
}

// EnrichedNodeResponse represents a fully enriched node response.
type EnrichedNodeResponse struct {
	ID            string
	Content       string
	Metadata      map[string]interface{}
	Relationships *NodeRelationships
}

// NodeRelationships represents relationships for a node.
type NodeRelationships struct {
	Edges      []string
	Categories []string
}

// Supporting validator and transformer classes would be implemented similarly
// with clear, focused responsibilities and descriptive names.

// NodeDataValidator provides focused node validation.
type NodeDataValidator struct{}

// NewNodeDataValidator creates a new node data validator.
func NewNodeDataValidator() *NodeDataValidator {
	return &NodeDataValidator{}
}

// ValidateNodeStructure validates the basic structure of node data.
func (v *NodeDataValidator) ValidateNodeStructure(data map[string]interface{}) []ValidationError {
	// Implementation would validate required fields, types, etc.
	return []ValidationError{}
}

// ApplyBusinessRules applies business rules to node data.
func (v *NodeDataValidator) ApplyBusinessRules(data map[string]interface{}) []ValidationError {
	// Implementation would apply domain-specific business rules
	return []ValidationError{}
}

// NormalizeNodeData normalizes node data according to business rules.
func (v *NodeDataValidator) NormalizeNodeData(data map[string]interface{}) map[string]interface{} {
	// Implementation would normalize data (trim strings, format dates, etc.)
	return data
}

// NodeTransformer provides focused node transformation.
type NodeTransformer struct{}

// NewNodeTransformer creates a new node transformer.
func NewNodeTransformer() *NodeTransformer {
	return &NodeTransformer{}
}

// TransformRawDataToNodeModel transforms raw data to a node model.
func (t *NodeTransformer) TransformRawDataToNodeModel(data map[string]interface{}) (*BasicNode, error) {
	// Implementation would transform raw data to domain model
	return &BasicNode{}, nil
}

// AddMetadataToNode adds metadata to a node.
func (t *NodeTransformer) AddMetadataToNode(node *BasicNode) *EnrichedNodeResponse {
	// Implementation would add computed metadata
	return &EnrichedNodeResponse{}
}

// BasicNode represents a basic node model.
type BasicNode struct {
	ID      string
	Content string
}