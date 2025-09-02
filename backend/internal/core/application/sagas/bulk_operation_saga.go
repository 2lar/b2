package sagas

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"brain2-backend/internal/core/application/commands"
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
)

// BulkOperationSaga orchestrates bulk operations with partial failure handling
type BulkOperationSaga struct {
	*BaseSaga
	commandBus    cqrs.CommandBus
	nodeRepo      ports.NodeRepository
	eventBus      ports.EventBus
	userID        string
	operation     BulkOperationType
	nodeIDs       []string
	targetData    map[string]interface{}
	batchSize     int
	maxConcurrent int
	successCount  int
	failureCount  int
	results       []BulkOperationItemResult
	resultsMutex  sync.Mutex
}

// BulkOperationType defines the type of bulk operation
type BulkOperationType string

const (
	BulkOperationArchive     BulkOperationType = "archive"
	BulkOperationRestore     BulkOperationType = "restore"
	BulkOperationDelete      BulkOperationType = "delete"
	BulkOperationTag         BulkOperationType = "tag"
	BulkOperationCategorize  BulkOperationType = "categorize"
	BulkOperationMove        BulkOperationType = "move"
	BulkOperationUpdateField BulkOperationType = "update_field"
)

// BulkOperationItemResult represents the result of a single item operation
type BulkOperationItemResult struct {
	NodeID    string
	Success   bool
	Error     error
	Timestamp time.Time
	Retries   int
}

// NewBulkOperationSaga creates a new bulk operation saga
func NewBulkOperationSaga(
	commandBus cqrs.CommandBus,
	nodeRepo ports.NodeRepository,
	eventBus ports.EventBus,
	logger ports.Logger,
	metrics ports.Metrics,
	userID string,
	operation BulkOperationType,
	nodeIDs []string,
	targetData map[string]interface{},
) *BulkOperationSaga {
	saga := &BulkOperationSaga{
		BaseSaga:      NewBaseSaga(logger, metrics),
		commandBus:    commandBus,
		nodeRepo:      nodeRepo,
		eventBus:      eventBus,
		userID:        userID,
		operation:     operation,
		nodeIDs:       nodeIDs,
		targetData:    targetData,
		batchSize:     10,
		maxConcurrent: 5,
		results:       make([]BulkOperationItemResult, 0, len(nodeIDs)),
	}
	
	// Define saga steps
	saga.Steps = []SagaStep{
		&BaseStep{
			Name:           "ValidateOperation",
			Action:         saga.validateOperation,
			CompensateFunc: nil,
			Retryable:      false,
		},
		&BaseStep{
			Name:           "ValidateNodes",
			Action:         saga.validateNodes,
			CompensateFunc: nil,
			Retryable:      true,
			MaxRetries:     2,
		},
		&BaseStep{
			Name:           "CreateSnapshot",
			Action:         saga.createSnapshot,
			CompensateFunc: nil,
			Retryable:      true,
			MaxRetries:     3,
		},
		&BaseStep{
			Name:           "ProcessBatches",
			Action:         saga.processBatches,
			CompensateFunc: saga.compensateProcessBatches,
			Retryable:      false, // Individual items have their own retry logic
		},
		&BaseStep{
			Name:           "VerifyResults",
			Action:         saga.verifyResults,
			CompensateFunc: nil,
			Retryable:      false,
		},
		&BaseStep{
			Name:           "PublishResults",
			Action:         saga.publishResults,
			CompensateFunc: nil,
			Retryable:      true,
			MaxRetries:     3,
		},
	}
	
	return saga
}

// validateOperation validates the bulk operation parameters
func (s *BulkOperationSaga) validateOperation(ctx context.Context) error {
	if s.userID == "" {
		return fmt.Errorf("user ID is required")
	}
	
	if len(s.nodeIDs) == 0 {
		return fmt.Errorf("no nodes specified for bulk operation")
	}
	
	if len(s.nodeIDs) > 1000 {
		return fmt.Errorf("too many nodes for bulk operation (max 1000)")
	}
	
	// Validate operation-specific requirements
	switch s.operation {
	case BulkOperationTag:
		if tags, ok := s.targetData["tags"].([]string); !ok || len(tags) == 0 {
			return fmt.Errorf("tags required for tag operation")
		}
	case BulkOperationCategorize:
		if categoryID, ok := s.targetData["category_id"].(string); !ok || categoryID == "" {
			return fmt.Errorf("category_id required for categorize operation")
		}
	case BulkOperationUpdateField:
		if field, ok := s.targetData["field"].(string); !ok || field == "" {
			return fmt.Errorf("field required for update operation")
		}
	}
	
	s.logger.Info("Bulk operation validated",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "operation", Value: string(s.operation)},
		ports.Field{Key: "node_count", Value: len(s.nodeIDs)})
	
	return nil
}

// validateNodes ensures all nodes exist and are accessible
func (s *BulkOperationSaga) validateNodes(ctx context.Context) error {
	validNodes := []string{}
	invalidNodes := []string{}
	
	// Check nodes in batches to avoid overwhelming the database
	for i := 0; i < len(s.nodeIDs); i += s.batchSize {
		end := i + s.batchSize
		if end > len(s.nodeIDs) {
			end = len(s.nodeIDs)
		}
		
		batch := s.nodeIDs[i:end]
		
		for _, nodeID := range batch {
			node, err := s.nodeRepo.GetByID(ctx, nodeID)
			if err != nil {
				invalidNodes = append(invalidNodes, nodeID)
				s.results = append(s.results, BulkOperationItemResult{
					NodeID:    nodeID,
					Success:   false,
					Error:     fmt.Errorf("node not found"),
					Timestamp: time.Now(),
				})
				continue
			}
			
			// Check ownership
			if node.GetUserID() != s.userID {
				invalidNodes = append(invalidNodes, nodeID)
				s.results = append(s.results, BulkOperationItemResult{
					NodeID:    nodeID,
					Success:   false,
					Error:     fmt.Errorf("user does not own node"),
					Timestamp: time.Now(),
				})
				continue
			}
			
			// Check operation-specific constraints
			if s.operation == BulkOperationRestore && !node.IsArchived() {
				invalidNodes = append(invalidNodes, nodeID)
				s.results = append(s.results, BulkOperationItemResult{
					NodeID:    nodeID,
					Success:   false,
					Error:     fmt.Errorf("node is not archived"),
					Timestamp: time.Now(),
				})
				continue
			}
			
			if s.operation == BulkOperationArchive && node.IsArchived() {
				invalidNodes = append(invalidNodes, nodeID)
				s.results = append(s.results, BulkOperationItemResult{
					NodeID:    nodeID,
					Success:   false,
					Error:     fmt.Errorf("node is already archived"),
					Timestamp: time.Now(),
				})
				continue
			}
			
			validNodes = append(validNodes, nodeID)
		}
	}
	
	// Update node list to only include valid nodes
	s.nodeIDs = validNodes
	s.Metadata["valid_nodes"] = len(validNodes)
	s.Metadata["invalid_nodes"] = len(invalidNodes)
	
	if len(validNodes) == 0 {
		return fmt.Errorf("no valid nodes to process")
	}
	
	s.logger.Info("Nodes validated",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "valid", Value: len(validNodes)},
		ports.Field{Key: "invalid", Value: len(invalidNodes)})
	
	return nil
}

// createSnapshot creates a snapshot of the current state for rollback
func (s *BulkOperationSaga) createSnapshot(ctx context.Context) error {
	snapshot := make(map[string]interface{})
	
	// Store current state of all nodes
	for _, nodeID := range s.nodeIDs {
		node, err := s.nodeRepo.GetByID(ctx, nodeID)
		if err != nil {
			continue // Skip if can't get node
		}
		
		snapshot[nodeID] = map[string]interface{}{
			"archived": node.IsArchived(),
			"version":  node.GetVersion(),
			"tags":     node.GetTags(),
		}
	}
	
	s.Metadata["snapshot"] = snapshot
	s.Metadata["snapshot_time"] = time.Now()
	
	s.logger.Info("Snapshot created",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "nodes", Value: len(snapshot)})
	
	return nil
}

// processBatches processes nodes in batches with concurrency control
func (s *BulkOperationSaga) processBatches(ctx context.Context) error {
	// Create a semaphore for concurrency control
	semaphore := make(chan struct{}, s.maxConcurrent)
	
	// Create a wait group for batch processing
	var wg sync.WaitGroup
	
	// Process nodes in batches
	for i := 0; i < len(s.nodeIDs); i += s.batchSize {
		end := i + s.batchSize
		if end > len(s.nodeIDs) {
			end = len(s.nodeIDs)
		}
		
		batch := s.nodeIDs[i:end]
		
		for _, nodeID := range batch {
			wg.Add(1)
			
			// Acquire semaphore
			semaphore <- struct{}{}
			
			go func(id string) {
				defer wg.Done()
				defer func() { <-semaphore }()
				
				// Process individual node with retry logic
				result := s.processNode(ctx, id)
				
				// Store result
				s.resultsMutex.Lock()
				s.results = append(s.results, result)
				if result.Success {
					s.successCount++
				} else {
					s.failureCount++
				}
				s.resultsMutex.Unlock()
			}(nodeID)
		}
	}
	
	// Wait for all operations to complete
	wg.Wait()
	
	s.Metadata["success_count"] = s.successCount
	s.Metadata["failure_count"] = s.failureCount
	
	s.logger.Info("Batches processed",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "success", Value: s.successCount},
		ports.Field{Key: "failure", Value: s.failureCount})
	
	// Decide if we should fail the saga based on failure rate
	failureRate := float64(s.failureCount) / float64(len(s.nodeIDs))
	if failureRate > 0.5 {
		return fmt.Errorf("too many failures: %d/%d", s.failureCount, len(s.nodeIDs))
	}
	
	return nil
}

// processNode processes a single node with retry logic
func (s *BulkOperationSaga) processNode(ctx context.Context, nodeID string) BulkOperationItemResult {
	maxRetries := 3
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		
		err := s.executeNodeOperation(ctx, nodeID)
		if err == nil {
			return BulkOperationItemResult{
				NodeID:    nodeID,
				Success:   true,
				Timestamp: time.Now(),
				Retries:   attempt,
			}
		}
		
		lastErr = err
		
		// Check if error is retryable
		if !isRetryable(err) {
			break
		}
	}
	
	return BulkOperationItemResult{
		NodeID:    nodeID,
		Success:   false,
		Error:     lastErr,
		Timestamp: time.Now(),
		Retries:   maxRetries,
	}
}

// executeNodeOperation executes the operation on a single node
func (s *BulkOperationSaga) executeNodeOperation(ctx context.Context, nodeID string) error {
	switch s.operation {
	case BulkOperationArchive:
		cmd := &commands.ArchiveNodeCommand{}
		cmd.UserID = s.userID
		cmd.NodeID = nodeID
		cmd.Reason = fmt.Sprintf("Bulk operation: %s", s.ID)
		return s.commandBus.Send(ctx, cmd)
		
	case BulkOperationRestore:
		cmd := &commands.RestoreNodeCommand{}
		cmd.UserID = s.userID
		cmd.NodeID = nodeID
		return s.commandBus.Send(ctx, cmd)
		
	case BulkOperationTag:
		tags := s.targetData["tags"].([]string)
		cmd := &AddTagsCommand{
			UserID: s.userID,
			NodeID: nodeID,
			Tags:   tags,
		}
		return s.commandBus.Send(ctx, cmd)
		
	case BulkOperationCategorize:
		categoryID := s.targetData["category_id"].(string)
		cmd := &CategorizeNodeCommand{
			UserID:     s.userID,
			NodeID:     nodeID,
			CategoryID: categoryID,
		}
		return s.commandBus.Send(ctx, cmd)
		
	case BulkOperationDelete:
		// Soft delete by archiving with special flag
		cmd := &commands.ArchiveNodeCommand{}
		cmd.UserID = s.userID
		cmd.NodeID = nodeID
		cmd.Reason = "Bulk delete operation"
		return s.commandBus.Send(ctx, cmd)
		
	default:
		return fmt.Errorf("unsupported operation: %s", s.operation)
	}
}

// compensateProcessBatches attempts to rollback processed operations
func (s *BulkOperationSaga) compensateProcessBatches(ctx context.Context) error {
	snapshot, ok := s.Metadata["snapshot"].(map[string]interface{})
	if !ok {
		s.logger.Warn("No snapshot available for compensation",
			ports.Field{Key: "saga_id", Value: s.ID})
		return nil
	}
	
	compensatedCount := 0
	failedCompensations := 0
	
	// Rollback successful operations
	for _, result := range s.results {
		if !result.Success {
			continue // Skip failed operations
		}
		
		nodeSnapshot, ok := snapshot[result.NodeID].(map[string]interface{})
		if !ok {
			continue
		}
		
		// Attempt to restore original state
		if err := s.restoreNodeState(ctx, result.NodeID, nodeSnapshot); err != nil {
			s.logger.Error("Failed to compensate node",
				err,
				ports.Field{Key: "node_id", Value: result.NodeID})
			failedCompensations++
		} else {
			compensatedCount++
		}
	}
	
	s.logger.Info("Batch operations compensated",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "compensated", Value: compensatedCount},
		ports.Field{Key: "failed", Value: failedCompensations})
	
	return nil
}

// restoreNodeState restores a node to its snapshot state
func (s *BulkOperationSaga) restoreNodeState(ctx context.Context, nodeID string, snapshot map[string]interface{}) error {
	// Restore based on operation type
	switch s.operation {
	case BulkOperationArchive:
		// If we archived it, restore it
		cmd := &commands.RestoreNodeCommand{}
		cmd.UserID = s.userID
		cmd.NodeID = nodeID
		return s.commandBus.Send(ctx, cmd)
		
	case BulkOperationRestore:
		// If we restored it, archive it again
		cmd := &commands.ArchiveNodeCommand{}
		cmd.UserID = s.userID
		cmd.NodeID = nodeID
		cmd.Reason = "Saga compensation"
		return s.commandBus.Send(ctx, cmd)
		
	default:
		// For other operations, log but don't fail
		s.logger.Warn("Cannot compensate operation",
			ports.Field{Key: "operation", Value: string(s.operation)},
			ports.Field{Key: "node_id", Value: nodeID})
		return nil
	}
}

// verifyResults verifies the operation results
func (s *BulkOperationSaga) verifyResults(ctx context.Context) error {
	// Calculate statistics
	totalProcessed := len(s.results)
	successRate := float64(s.successCount) / float64(totalProcessed)
	
	s.Metadata["total_processed"] = totalProcessed
	s.Metadata["success_rate"] = successRate
	
	// Log any failures for investigation
	if s.failureCount > 0 {
		failedNodes := []string{}
		for _, result := range s.results {
			if !result.Success {
				failedNodes = append(failedNodes, result.NodeID)
			}
		}
		s.Metadata["failed_nodes"] = failedNodes
	}
	
	s.logger.Info("Results verified",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "total", Value: totalProcessed},
		ports.Field{Key: "success_rate", Value: successRate})
	
	return nil
}

// publishResults publishes the bulk operation results
func (s *BulkOperationSaga) publishResults(ctx context.Context) error {
	// Note: EventBus publishing would need a proper DomainEvent implementation
	// For now, just log the completion
	s.logger.Info("Bulk operation completed",
		ports.Field{Key: "saga_id", Value: s.ID},
		ports.Field{Key: "operation", Value: string(s.operation)},
		ports.Field{Key: "total_nodes", Value: len(s.nodeIDs)},
		ports.Field{Key: "success_count", Value: s.successCount},
		ports.Field{Key: "failure_count", Value: s.failureCount})
	
	// Record metrics
	s.metrics.IncrementCounter("bulk_operation.completed",
		ports.Tag{Key: "operation", Value: string(s.operation)},
		ports.Tag{Key: "success", Value: fmt.Sprintf("%v", s.failureCount == 0)})
	
	s.metrics.RecordHistogram("bulk_operation.nodes_processed",
		float64(len(s.nodeIDs)),
		ports.Tag{Key: "operation", Value: string(s.operation)})
	
	s.metrics.RecordHistogram("bulk_operation.success_rate",
		float64(s.successCount)/float64(len(s.nodeIDs)),
		ports.Tag{Key: "operation", Value: string(s.operation)})
	
	return nil
}

// GetResult returns the result of the bulk operation
func (s *BulkOperationSaga) GetResult() *BulkOperationResult {
	return &BulkOperationResult{
		TotalNodes:    len(s.nodeIDs),
		SuccessCount:  s.successCount,
		FailureCount:  s.failureCount,
		Results:       s.results,
		Success:       s.State == SagaStateCompleted,
		PartialSuccess: s.successCount > 0 && s.failureCount > 0,
		Error:         s.Error,
	}
}

// BulkOperationResult contains the result of the BulkOperationSaga
type BulkOperationResult struct {
	TotalNodes     int
	SuccessCount   int
	FailureCount   int
	Results        []BulkOperationItemResult
	Success        bool
	PartialSuccess bool
	Error          error
}

// Placeholder command types for operations not yet defined
type AddTagsCommand struct {
	cqrs.BaseCommand
	UserID string
	NodeID string
	Tags   []string
}

func (c *AddTagsCommand) GetCommandName() string {
	return "AddTags"
}

func (c *AddTagsCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.NodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	if len(c.Tags) == 0 {
		return fmt.Errorf("at least one tag is required")
	}
	return nil
}

type CategorizeNodeCommand struct {
	cqrs.BaseCommand
	UserID     string
	NodeID     string
	CategoryID string
}

func (c *CategorizeNodeCommand) GetCommandName() string {
	return "CategorizeNode"
}

func (c *CategorizeNodeCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.NodeID == "" {
		return fmt.Errorf("node ID is required")
	}
	if c.CategoryID == "" {
		return fmt.Errorf("category ID is required")
	}
	return nil
}