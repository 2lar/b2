package services

import (
	"context"
	"sort"
	"sync"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/infrastructure/concurrency"
)

// ParallelConnectionAnalyzer extends ConnectionAnalyzer with parallel processing capabilities
type ParallelConnectionAnalyzer struct {
	*ConnectionAnalyzer
	environment concurrency.RuntimeEnvironment
}

// NewParallelConnectionAnalyzer creates a new parallel-capable connection analyzer
func NewParallelConnectionAnalyzer(base *ConnectionAnalyzer) *ParallelConnectionAnalyzer {
	return &ParallelConnectionAnalyzer{
		ConnectionAnalyzer: base,
		environment:        concurrency.DetectEnvironment(),
	}
}

// connectionAnalysisItem represents a node pair to analyze
type connectionAnalysisItem struct {
	SourceNode *node.Node
	TargetNode *node.Node
	Index      int
}

func (c connectionAnalysisItem) GetID() string {
	return c.SourceNode.ID().String() + "-" + c.TargetNode.ID().String()
}

// FindPotentialConnectionsParallel analyzes connections in parallel
func (pca *ParallelConnectionAnalyzer) FindPotentialConnectionsParallel(
	ctx context.Context,
	sourceNode *node.Node,
	candidates []*node.Node,
) ([]*ConnectionCandidate, error) {
	
	if len(candidates) == 0 {
		return []*ConnectionCandidate{}, nil
	}
	
	// For small candidate sets or Lambda environment, use sequential processing
	if len(candidates) < 10 || pca.environment == concurrency.EnvironmentLambda {
		return pca.FindPotentialConnections(sourceNode, candidates)
	}
	
	// Prepare items for batch processing
	items := make([]concurrency.BatchItem, len(candidates))
	for i, candidate := range candidates {
		items[i] = connectionAnalysisItem{
			SourceNode: sourceNode,
			TargetNode: candidate,
			Index:      i,
		}
	}
	
	// Configure batch processor
	config := &concurrency.PoolConfig{
		Environment: pca.environment,
		// Auto-configure based on environment
	}
	
	processor := concurrency.NewBatchProcessor(ctx, config)
	
	// Thread-safe result collection
	var mu sync.Mutex
	connections := make([]*ConnectionCandidate, 0)
	
	// Process connections in parallel
	processFunc := func(ctx context.Context, item concurrency.BatchItem) error {
		analysisItem := item.(connectionAnalysisItem)
		
		// Check basic business rules for connection eligibility
		if err := analysisItem.SourceNode.CanConnectTo(analysisItem.TargetNode); err != nil {
			return nil // Skip if connection is not allowed
		}
		
		// Calculate connection metrics
		connectionCandidate := pca.analyzeConnection(analysisItem.SourceNode, analysisItem.TargetNode)
		
		// Apply similarity threshold
		if connectionCandidate.SimilarityScore >= pca.similarityThreshold {
			mu.Lock()
			connections = append(connections, connectionCandidate)
			mu.Unlock()
		}
		
		return nil
	}
	
	// Execute parallel processing
	_, err := processor.ProcessBatch(ctx, items, processFunc)
	if err != nil {
		// Fall back to sequential on error
		return pca.FindPotentialConnections(sourceNode, candidates)
	}
	
	// Sort and limit results
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].RelevanceScore > connections[j].RelevanceScore
	})
	
	// Limit to maximum connections
	if len(connections) > pca.maxConnectionsPerNode {
		connections = connections[:pca.maxConnectionsPerNode]
	}
	
	return connections, nil
}

// AnalyzeBulkConnectionsParallel analyzes connections for multiple node pairs in parallel
func (pca *ParallelConnectionAnalyzer) AnalyzeBulkConnectionsParallel(
	ctx context.Context,
	nodePairs []NodePair,
) (map[string]*BidirectionalAnalysis, error) {
	
	if len(nodePairs) == 0 {
		return make(map[string]*BidirectionalAnalysis), nil
	}
	
	// Configure based on environment
	maxWorkers := 2 // Lambda default
	if pca.environment == concurrency.EnvironmentECS {
		maxWorkers = 10
	} else if pca.environment == concurrency.EnvironmentLocal {
		maxWorkers = 5
	}
	
	// Create worker pool
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	
	// Results map with mutex protection
	results := make(map[string]*BidirectionalAnalysis)
	var resultsMu sync.Mutex
	
	for _, pair := range nodePairs {
		wg.Add(1)
		
		go func(p NodePair) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Analyze bidirectional connection
			analysis, err := pca.AnalyzeBidirectionalConnection(p.Source, p.Target)
			if err == nil {
				key := p.Source.ID().String() + "-" + p.Target.ID().String()
				resultsMu.Lock()
				results[key] = analysis
				resultsMu.Unlock()
			}
		}(pair)
	}
	
	wg.Wait()
	
	return results, nil
}

// NodePair represents a pair of nodes to analyze
type NodePair struct {
	Source *node.Node
	Target *node.Node
}

// nodeBatchItem is a BatchItem implementation for batch processing nodes
type nodeBatchItem struct {
	ID   string
	Node *node.Node
}

// GetID implements BatchItem interface
func (n nodeBatchItem) GetID() string {
	return n.ID
}

// BatchFindConnections finds connections for multiple nodes in parallel
func (pca *ParallelConnectionAnalyzer) BatchFindConnections(
	ctx context.Context,
	nodes []*node.Node,
	allNodes []*node.Node,
) (map[string][]*ConnectionCandidate, error) {
	
	if len(nodes) == 0 || len(allNodes) == 0 {
		return make(map[string][]*ConnectionCandidate), nil
	}
	
	// Prepare batch items
	type batchItem struct {
		SourceNode *node.Node
		NodeID     string
	}
	
	// Create batch items
	items := make([]concurrency.BatchItem, len(nodes))
	for i, n := range nodes {
		items[i] = nodeBatchItem{
			ID:   n.ID().String(),
			Node: n,
		}
	}
	
	// Configure processor
	config := &concurrency.PoolConfig{
		Environment: pca.environment,
	}
	
	processor := concurrency.NewBatchProcessor(ctx, config)
	
	// Results collection
	results := make(map[string][]*ConnectionCandidate)
	var mu sync.Mutex
	
	// Process each node's connections
	processFunc := func(ctx context.Context, item concurrency.BatchItem) error {
		nodeItem := item.(nodeBatchItem)
		
		// Find connections for this node
		connections, err := pca.FindPotentialConnections(nodeItem.Node, allNodes)
		if err != nil {
			return err
		}
		
		mu.Lock()
		results[nodeItem.ID] = connections
		mu.Unlock()
		
		return nil
	}
	
	// Execute batch processing
	_, err := processor.ProcessBatch(ctx, items, processFunc)
	if err != nil {
		return results, err
	}
	
	return results, nil
}

// OptimizeConnectionGraph optimizes connections for an entire graph
func (pca *ParallelConnectionAnalyzer) OptimizeConnectionGraph(
	ctx context.Context,
	nodes []*node.Node,
	maxConnectionsPerNode int,
) ([][2]*node.Node, error) {
	
	// For Lambda, use more conservative approach
	if pca.environment == concurrency.EnvironmentLambda && len(nodes) > 50 {
		// Process in smaller chunks to avoid timeout
		return pca.optimizeConnectionGraphChunked(ctx, nodes, maxConnectionsPerNode)
	}
	
	// Find all potential connections in parallel
	allConnections, err := pca.BatchFindConnections(ctx, nodes, nodes)
	if err != nil {
		return nil, err
	}
	
	// Build optimized connection pairs
	var connections [][2]*node.Node
	processedPairs := make(map[string]bool)
	
	for sourceID, candidates := range allConnections {
		for _, candidate := range candidates {
			// Create canonical pair ID to avoid duplicates
			pairID := canonicalPairID(sourceID, candidate.Node.ID().String())
			
			if !processedPairs[pairID] {
				processedPairs[pairID] = true
				
				// Find source node
				var sourceNode *node.Node
				for _, n := range nodes {
					if n.ID().String() == sourceID {
						sourceNode = n
						break
					}
				}
				
				if sourceNode != nil {
					connections = append(connections, [2]*node.Node{sourceNode, candidate.Node})
				}
			}
			
			// Limit connections per node
			if len(connections) >= maxConnectionsPerNode {
				break
			}
		}
	}
	
	return connections, nil
}

// optimizeConnectionGraphChunked processes large graphs in chunks (for Lambda)
func (pca *ParallelConnectionAnalyzer) optimizeConnectionGraphChunked(
	ctx context.Context,
	nodes []*node.Node,
	maxConnectionsPerNode int,
) ([][2]*node.Node, error) {
	
	chunkSize := 25 // Lambda-optimized chunk size
	var allConnections [][2]*node.Node
	
	for i := 0; i < len(nodes); i += chunkSize {
		end := i + chunkSize
		if end > len(nodes) {
			end = len(nodes)
		}
		
		chunk := nodes[i:end]
		
		// Process chunk
		connections, err := pca.OptimizeConnectionGraph(ctx, chunk, maxConnectionsPerNode)
		if err != nil {
			return allConnections, err
		}
		
		allConnections = append(allConnections, connections...)
		
		// Check context for timeout
		select {
		case <-ctx.Done():
			return allConnections, ctx.Err()
		default:
			// Continue processing
		}
	}
	
	return allConnections, nil
}

// canonicalPairID creates a consistent ID for a node pair regardless of order
func canonicalPairID(id1, id2 string) string {
	if id1 < id2 {
		return id1 + "-" + id2
	}
	return id2 + "-" + id1
}