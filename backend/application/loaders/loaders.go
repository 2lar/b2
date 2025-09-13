package loaders

import (
	"context"
	"time"

	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"go.uber.org/zap"
)

// NodeLoader provides batched loading of nodes
type NodeLoader struct {
	*Batcher[string, *entities.Node]
	repo   ports.NodeRepository
	logger *zap.Logger
}

// NewNodeLoader creates a new node loader
func NewNodeLoader(repo ports.NodeRepository, batchWindow time.Duration, maxBatchSize int, logger *zap.Logger) *NodeLoader {
	loader := &NodeLoader{
		repo:   repo,
		logger: logger,
	}

	// Create batch function
	batchFn := func(ctx context.Context, keys []string) (map[string]*entities.Node, error) {
		// Convert string IDs to NodeIDs
		nodeIDs := make([]valueobjects.NodeID, len(keys))
		for i, key := range keys {
			nodeID, err := valueobjects.NewNodeIDFromString(key)
			if err != nil {
				logger.Error("Invalid node ID in batch", zap.String("id", key), zap.Error(err))
				continue
			}
			nodeIDs[i] = nodeID
		}

		// Call batch repository method
		if batchRepo, ok := repo.(interface {
			GetNodesByIDs(context.Context, []valueobjects.NodeID) (map[valueobjects.NodeID]*entities.Node, error)
		}); ok {
			nodes, err := batchRepo.GetNodesByIDs(ctx, nodeIDs)
			if err != nil {
				return nil, err
			}

			// Convert back to string keys
			result := make(map[string]*entities.Node)
			for nodeID, node := range nodes {
				result[nodeID.String()] = node
			}
			return result, nil
		}

		// Fallback to individual loads if batch method not available
		result := make(map[string]*entities.Node)
		for _, nodeID := range nodeIDs {
			node, err := repo.GetByID(ctx, nodeID)
			if err == nil {
				result[nodeID.String()] = node
			}
		}
		return result, nil
	}

	loader.Batcher = NewBatcher(batchFn, batchWindow, maxBatchSize, logger)
	return loader
}

// LoadByID loads a node by its ID
func (l *NodeLoader) LoadByID(ctx context.Context, nodeID valueobjects.NodeID) (*entities.Node, error) {
	return l.Load(ctx, nodeID.String())
}

// LoadManyByIDs loads multiple nodes by their IDs
func (l *NodeLoader) LoadManyByIDs(ctx context.Context, nodeIDs []valueobjects.NodeID) (map[valueobjects.NodeID]*entities.Node, error) {
	keys := make([]string, len(nodeIDs))
	for i, id := range nodeIDs {
		keys[i] = id.String()
	}

	stringMap, err := l.LoadMany(ctx, keys)
	if err != nil {
		return nil, err
	}

	// Convert back to NodeID keys
	result := make(map[valueobjects.NodeID]*entities.Node)
	for key, node := range stringMap {
		nodeID, _ := valueobjects.NewNodeIDFromString(key)
		result[nodeID] = node
	}

	return result, nil
}

// EdgeLoader provides batched loading of edges
type EdgeLoader struct {
	*Batcher[string, []*aggregates.Edge]
	repo   ports.EdgeRepository
	logger *zap.Logger
}

// NewEdgeLoader creates a new edge loader
func NewEdgeLoader(repo ports.EdgeRepository, batchWindow time.Duration, maxBatchSize int, logger *zap.Logger) *EdgeLoader {
	loader := &EdgeLoader{
		repo:   repo,
		logger: logger,
	}

	// Create batch function
	batchFn := func(ctx context.Context, keys []string) (map[string][]*aggregates.Edge, error) {
		// Call batch repository method
		if batchRepo, ok := repo.(interface {
			GetEdgesByNodeIDs(context.Context, []string) (map[string][]*aggregates.Edge, error)
		}); ok {
			return batchRepo.GetEdgesByNodeIDs(ctx, keys)
		}

		// Fallback to individual loads if batch method not available
		result := make(map[string][]*aggregates.Edge)
		for _, nodeID := range keys {
			edges, err := repo.GetByNodeID(ctx, nodeID)
			if err == nil {
				result[nodeID] = edges
			}
		}
		return result, nil
	}

	loader.Batcher = NewBatcher(batchFn, batchWindow, maxBatchSize, logger)
	return loader
}

// LoadByNodeID loads all edges for a node
func (l *EdgeLoader) LoadByNodeID(ctx context.Context, nodeID string) ([]*aggregates.Edge, error) {
	return l.Load(ctx, nodeID)
}

// LoadManyByNodeIDs loads edges for multiple nodes
func (l *EdgeLoader) LoadManyByNodeIDs(ctx context.Context, nodeIDs []string) (map[string][]*aggregates.Edge, error) {
	return l.LoadMany(ctx, nodeIDs)
}

// GraphLoader provides batched loading of graphs
type GraphLoader struct {
	*Batcher[string, *aggregates.Graph]
	repo   ports.GraphRepository
	logger *zap.Logger
}

// NewGraphLoader creates a new graph loader
func NewGraphLoader(repo ports.GraphRepository, batchWindow time.Duration, maxBatchSize int, logger *zap.Logger) *GraphLoader {
	loader := &GraphLoader{
		repo:   repo,
		logger: logger,
	}

	// Create batch function
	batchFn := func(ctx context.Context, keys []string) (map[string]*aggregates.Graph, error) {
		// Call batch repository method if available
		if batchRepo, ok := repo.(interface {
			GetGraphsByIDs(context.Context, []aggregates.GraphID) (map[aggregates.GraphID]*aggregates.Graph, error)
		}); ok {
			graphIDs := make([]aggregates.GraphID, len(keys))
			for i, key := range keys {
				graphIDs[i] = aggregates.GraphID(key)
			}

			graphs, err := batchRepo.GetGraphsByIDs(ctx, graphIDs)
			if err != nil {
				return nil, err
			}

			// Convert back to string keys
			result := make(map[string]*aggregates.Graph)
			for graphID, graph := range graphs {
				result[string(graphID)] = graph
			}
			return result, nil
		}

		// Fallback to individual loads
		result := make(map[string]*aggregates.Graph)
		for _, graphID := range keys {
			graph, err := repo.GetByID(ctx, aggregates.GraphID(graphID))
			if err == nil {
				result[graphID] = graph
			}
		}
		return result, nil
	}

	loader.Batcher = NewBatcher(batchFn, batchWindow, maxBatchSize, logger)
	return loader
}

// LoadByID loads a graph by its ID
func (l *GraphLoader) LoadByID(ctx context.Context, graphID aggregates.GraphID) (*aggregates.Graph, error) {
	return l.Load(ctx, string(graphID))
}

// LoadManyByIDs loads multiple graphs by their IDs
func (l *GraphLoader) LoadManyByIDs(ctx context.Context, graphIDs []aggregates.GraphID) (map[aggregates.GraphID]*aggregates.Graph, error) {
	keys := make([]string, len(graphIDs))
	for i, id := range graphIDs {
		keys[i] = string(id)
	}

	stringMap, err := l.LoadMany(ctx, keys)
	if err != nil {
		return nil, err
	}

	// Convert back to GraphID keys
	result := make(map[aggregates.GraphID]*aggregates.Graph)
	for key, graph := range stringMap {
		result[aggregates.GraphID(key)] = graph
	}

	return result, nil
}

// DataLoaderService provides all data loaders
type DataLoaderService struct {
	NodeLoader  *NodeLoader
	EdgeLoader  *EdgeLoader
	GraphLoader *GraphLoader
	enabled     bool
	logger      *zap.Logger
}

// NewDataLoaderService creates a new data loader service
func NewDataLoaderService(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	graphRepo ports.GraphRepository,
	batchWindow time.Duration,
	maxBatchSize int,
	logger *zap.Logger,
) *DataLoaderService {
	return &DataLoaderService{
		NodeLoader:  NewNodeLoader(nodeRepo, batchWindow, maxBatchSize, logger),
		EdgeLoader:  NewEdgeLoader(edgeRepo, batchWindow, maxBatchSize, logger),
		GraphLoader: NewGraphLoader(graphRepo, batchWindow, maxBatchSize, logger),
		enabled:     true,
		logger:      logger,
	}
}

// SetEnabled enables or disables the data loader service
func (s *DataLoaderService) SetEnabled(enabled bool) {
	s.enabled = enabled
	if enabled {
		s.logger.Info("DataLoader service enabled")
	} else {
		s.logger.Info("DataLoader service disabled")
	}
}

// IsEnabled returns whether the data loader service is enabled
func (s *DataLoaderService) IsEnabled() bool {
	return s.enabled
}

// GetMetrics returns metrics for all loaders
func (s *DataLoaderService) GetMetrics() map[string]BatcherMetrics {
	return map[string]BatcherMetrics{
		"nodes":  s.NodeLoader.GetMetrics(),
		"edges":  s.EdgeLoader.GetMetrics(),
		"graphs": s.GraphLoader.GetMetrics(),
	}
}