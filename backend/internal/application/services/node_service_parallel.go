package services

import (
	"context"
	"fmt"
	"log"
	"sync"

	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain/edge"
	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/errors"
	"brain2-backend/internal/infrastructure/concurrency"
)

// nodeCreationItem represents a node to be created in parallel
type nodeCreationItem struct {
	Index   int
	Content string
	Tags    []string
	NodeID  string // Will be set after creation
}

func (n nodeCreationItem) GetID() string {
	return fmt.Sprintf("node_%d", n.Index)
}

// BulkCreateNodesParallel implements parallel bulk node creation with environment-aware concurrency
func (s *NodeService) BulkCreateNodesParallel(ctx context.Context, cmd *commands.BulkCreateNodesCommand) (*dto.BulkCreateResult, error) {
	// 1. Create a new UnitOfWork instance for this request
	uow, err := s.uowFactory.Create(ctx)
	if err != nil {
		return nil, errors.ApplicationError(ctx, "CreateUnitOfWork", err)
	}
	
	// Start unit of work
	if err := uow.Begin(ctx); err != nil {
		return nil, errors.ApplicationError(ctx, "BeginTransaction", err)
	}
	
	// Track whether commit was called successfully
	var commitCalled bool
	
	// Ensure proper cleanup even on panic
	defer func() {
		if r := recover(); r != nil {
			// Attempt rollback on panic
			if rollbackErr := uow.Rollback(); rollbackErr != nil {
				// Log error but continue with panic
			}
			// Re-panic to let it bubble up
			panic(r)
		} else if !commitCalled {
			// Only rollback if commit wasn't called
			uow.Rollback()
		}
	}()

	// 2. Parse user ID
	userID, err := shared.ParseUserID(cmd.UserID)
	if err != nil {
		return nil, errors.ServiceValidationError("userID", err.Error(), cmd.UserID)
	}

	// 3. Prepare items for parallel processing
	items := make([]concurrency.BatchItem, len(cmd.Nodes))
	for i, nodeReq := range cmd.Nodes {
		items[i] = nodeCreationItem{
			Index:   i,
			Content: nodeReq.Content,
			Tags:    nodeReq.Tags,
		}
	}

	// 4. Get batch processor from pool manager
	var processor *concurrency.BatchProcessor
	if s.container != nil && s.container.GetPoolManager() != nil {
		processor = s.container.GetPoolManager().GetBatchProcessor()
	} else {
		// Fallback to creating new processor if pool manager not available
		config := &concurrency.PoolConfig{
			Environment: concurrency.DetectEnvironment(),
		}
		processor = concurrency.NewBatchProcessor(ctx, config)
	}
	
	// 5. Thread-safe result collection - pre-allocate with known capacity
	var mu sync.Mutex
	createdNodes := make([]*node.Node, 0, len(cmd.Nodes))
	nodesByIndex := make(map[int]*node.Node, len(cmd.Nodes))
	bulkErrors := make([]dto.BulkCreateError, 0, len(cmd.Nodes)/10) // Assume ~10% error rate
	
	// 6. Process nodes in parallel
	processFunc := func(ctx context.Context, item concurrency.BatchItem) error {
		nodeItem := item.(nodeCreationItem)
		
		// Create domain node
		content, err := shared.NewContent(nodeItem.Content)
		if err != nil {
			mu.Lock()
			bulkErrors = append(bulkErrors, dto.BulkCreateError{
				Index:   nodeItem.Index,
				Content: nodeItem.Content,
				Error:   "invalid content: " + err.Error(),
			})
			mu.Unlock()
			return err
		}

		tags := shared.NewTags(nodeItem.Tags...)
		title, _ := shared.NewTitle("") // Empty title for bulk create
		newNode, err := node.NewNode(userID, content, title, tags)
		if err != nil {
			mu.Lock()
			bulkErrors = append(bulkErrors, dto.BulkCreateError{
				Index:   nodeItem.Index,
				Content: nodeItem.Content,
				Error:   "failed to create node: " + err.Error(),
			})
			mu.Unlock()
			return err
		}

		// Save node through unit of work (thread-safe operation)
		if err := uow.Nodes().CreateNodeAndKeywords(ctx, newNode); err != nil {
			mu.Lock()
			bulkErrors = append(bulkErrors, dto.BulkCreateError{
				Index:   nodeItem.Index,
				Content: nodeItem.Content,
				Error:   "failed to save node: " + err.Error(),
			})
			mu.Unlock()
			return err
		}

		// Store successfully created node
		mu.Lock()
		createdNodes = append(createdNodes, newNode)
		nodesByIndex[nodeItem.Index] = newNode
		mu.Unlock()
		
		return nil
	}
	
	// Execute parallel processing
	result, err := processor.ProcessBatch(ctx, items, processFunc)
	if err != nil && result == nil {
		// Complete failure
		return nil, errors.ApplicationError(ctx, "ParallelBulkCreate", err)
	}

	// 7. Create connections in parallel if we have multiple successful nodes
	var connections []*edge.Edge
	var connectionMu sync.Mutex
	
	if len(createdNodes) > 1 {
		// Prepare connection pairs for parallel processing
		type connectionPair struct {
			SourceNode *node.Node
			TargetNode *node.Node
		}
		
		var pairs []connectionPair
		for i, sourceNode := range createdNodes {
			for j, targetNode := range createdNodes {
				if i >= j { // Avoid duplicates and self-connections
					continue
				}
				pairs = append(pairs, connectionPair{
					SourceNode: sourceNode,
					TargetNode: targetNode,
				})
			}
		}
		
		// Process connections using pool manager if available
		var connectionPool *concurrency.AdaptiveWorkerPool
		if s.container != nil && s.container.GetPoolManager() != nil {
			connectionPool = s.container.GetPoolManager().GetConnectionPool()
		}
		
		// If no pool available, use semaphore pattern
		var semaphore chan struct{}
		if connectionPool == nil {
			env := concurrency.DetectEnvironment()
			maxConcurrentConnections := 2 // Conservative for Lambda
			if env == concurrency.EnvironmentECS {
				maxConcurrentConnections = 10
			} else if env == concurrency.EnvironmentLocal {
				maxConcurrentConnections = 5
			}
			semaphore = make(chan struct{}, maxConcurrentConnections)
		}
		var wg sync.WaitGroup
		
		for _, pair := range pairs {
			if connectionPool != nil {
				// Use pool for connection processing
				wg.Add(1)
				task := concurrency.Task{
					ID: fmt.Sprintf("conn_%s_%s", pair.SourceNode.ID(), pair.TargetNode.ID()),
					Execute: func(ctx context.Context) error {
						// Use connection analyzer to determine if nodes should be connected
						analysis, err := s.connectionAnalyzer.AnalyzeBidirectionalConnection(pair.SourceNode, pair.TargetNode)
						if err == nil && analysis.ShouldConnect {
							// Create edge between nodes
							weight := analysis.ForwardConnection.RelevanceScore
							edge, err := edge.NewEdge(pair.SourceNode.ID(), pair.TargetNode.ID(), userID, weight)
							if err == nil {
								// Save edge through unit of work
								if err := uow.Edges().CreateEdge(ctx, edge); err == nil {
									connectionMu.Lock()
									connections = append(connections, edge)
									connectionMu.Unlock()
								}
							}
						}
						return nil
					},
					Callback: func(id string, err error) {
						defer wg.Done()
						if err != nil {
							log.Printf("Error processing connection %s: %v", id, err)
						}
					},
				}
				connectionPool.Submit(task)
			} else {
				// Fallback to semaphore pattern
				wg.Add(1)
				go func(p connectionPair) {
					defer wg.Done()
					
					// Acquire semaphore
					semaphore <- struct{}{}
					defer func() { <-semaphore }()
				
				// Use connection analyzer to determine if nodes should be connected
				analysis, err := s.connectionAnalyzer.AnalyzeBidirectionalConnection(p.SourceNode, p.TargetNode)
				if err == nil && analysis.ShouldConnect {
					// Create edge between nodes
					weight := analysis.ForwardConnection.RelevanceScore
					edge, err := edge.NewEdge(p.SourceNode.ID(), p.TargetNode.ID(), userID, weight)
					if err == nil {
						// Save edge through unit of work
						if err := uow.Edges().CreateEdge(ctx, edge); err == nil {
							connectionMu.Lock()
							connections = append(connections, edge)
							connectionMu.Unlock()
						}
					}
				}
				}(pair)
			}
		}
		
		wg.Wait()
	}

	// 8. Publish domain events for all created nodes
	for _, node := range createdNodes {
		for _, event := range node.GetUncommittedEvents() {
			if err := s.eventBus.Publish(ctx, event); err != nil {
				return nil, errors.ApplicationError(ctx, "PublishEvent", err)
			}
		}
		node.MarkEventsAsCommitted()
	}

	// 9. Commit transaction
	commitCalled = true
	if err := uow.Commit(); err != nil {
		commitCalled = false
		return nil, errors.ApplicationError(ctx, "CommitTransaction", err)
	}

	// 10. Convert to response DTO
	response := &dto.BulkCreateResult{
		CreatedNodes:    dto.ToNodeViews(createdNodes),
		CreatedCount:    len(createdNodes),
		Connections:     dto.ToConnectionViews(connections),
		ConnectionCount: len(connections),
		Failed:          bulkErrors,
		Message:         fmt.Sprintf("Successfully created %d nodes", len(createdNodes)),
	}

	if len(bulkErrors) > 0 {
		response.Message = fmt.Sprintf("Created %d nodes with %d failures", len(createdNodes), len(bulkErrors))
	}

	if len(connections) > 0 {
		response.Message += fmt.Sprintf(" and %d connections", len(connections))
	}

	// Log performance metrics
	if result != nil {
		// In production, send these to CloudWatch or your metrics service
		processingTime := result.Duration
		throughput := float64(result.SuccessCount) / processingTime.Seconds()
		_ = throughput // Use for metrics
	}

	return response, nil
}