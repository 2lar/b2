package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/valueobjects"
	"backend/infrastructure/config"
	"go.uber.org/zap"
)

// GraphLazyService manages lazy-loaded graph instances
type GraphLazyService struct {
	nodeRepo ports.NodeRepository
	edgeRepo ports.EdgeRepository
	config   *config.Config
	logger   *zap.Logger
	
	// Cache of lazy-loaded graphs per user
	graphCache map[string]*aggregates.GraphLazy
	cacheMu    sync.RWMutex
}

// NewGraphLazyService creates a new lazy graph service
func NewGraphLazyService(
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	config *config.Config,
	logger *zap.Logger,
) *GraphLazyService {
	return &GraphLazyService{
		nodeRepo:   nodeRepo,
		edgeRepo:   edgeRepo,
		config:     config,
		logger:     logger,
		graphCache: make(map[string]*aggregates.GraphLazy),
	}
}

// IsEnabled returns whether lazy loading is enabled
func (s *GraphLazyService) IsEnabled() bool {
	return s.config.EnableLazyLoading
}

// GetOrCreateForUser gets or creates a lazy-loaded graph for a user
func (s *GraphLazyService) GetOrCreateForUser(ctx context.Context, userID string, graphID string) (*aggregates.GraphLazy, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("lazy loading is not enabled")
	}

	cacheKey := fmt.Sprintf("%s:%s", userID, graphID)
	
	// Check cache first
	s.cacheMu.RLock()
	if cached, exists := s.graphCache[cacheKey]; exists {
		s.cacheMu.RUnlock()
		return cached, nil
	}
	s.cacheMu.RUnlock()

	// Create new lazy graph
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	
	// Double-check after acquiring write lock
	if cached, exists := s.graphCache[cacheKey]; exists {
		return cached, nil
	}

	// Create new lazy graph instance
	graph, err := aggregates.NewGraphLazy(userID, fmt.Sprintf("Graph-%s", graphID))
	if err != nil {
		return nil, fmt.Errorf("failed to create lazy graph: %w", err)
	}

	// Wire the loaders
	if nodeLoader, ok := s.nodeRepo.(aggregates.NodeLoader); ok {
		if edgeLoader, ok := s.edgeRepo.(aggregates.EdgeLoader); ok {
			graph.SetLoaders(nodeLoader, edgeLoader)
		} else {
			return nil, fmt.Errorf("edge repository does not implement EdgeLoader")
		}
	} else {
		return nil, fmt.Errorf("node repository does not implement NodeLoader")
	}

	// Cache the graph
	s.graphCache[cacheKey] = graph
	
	s.logger.Info("Created lazy-loaded graph",
		zap.String("userID", userID),
		zap.String("graphID", graphID),
	)

	return graph, nil
}

// LoadFromExisting loads a lazy graph from existing graph data
func (s *GraphLazyService) LoadFromExisting(
	ctx context.Context,
	graphID string,
	userID string,
	name string,
	description string,
	nodeIDs []string,
	edgeKeys []string,
) (*aggregates.GraphLazy, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("lazy loading is not enabled")
	}

	// Convert string IDs to NodeIDs
	nodeIDList := make([]valueobjects.NodeID, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		nodeID, err := valueobjects.NewNodeIDFromString(id)
		if err != nil {
			s.logger.Warn("Invalid node ID during reconstruction",
				zap.String("nodeID", id),
				zap.Error(err),
			)
			continue
		}
		nodeIDList = append(nodeIDList, nodeID)
	}

	// Reconstruct the lazy graph
	metadata := aggregates.GraphMetadata{
		NodeCount: len(nodeIDList),
		EdgeCount: len(edgeKeys),
		ViewSettings: aggregates.ViewSettings{
			Layout:     aggregates.LayoutForceDirected,
			ShowLabels: true,
		},
	}

	graph, err := aggregates.ReconstructGraphLazy(
		graphID,
		userID,
		name,
		description,
		nodeIDList,
		edgeKeys,
		metadata,
		time.Now(),
		time.Now(),
		1,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct lazy graph: %w", err)
	}

	// Wire the loaders
	if nodeLoader, ok := s.nodeRepo.(aggregates.NodeLoader); ok {
		if edgeLoader, ok := s.edgeRepo.(aggregates.EdgeLoader); ok {
			graph.SetLoaders(nodeLoader, edgeLoader)
		}
	}

	return graph, nil
}

// ClearCache clears the graph cache
func (s *GraphLazyService) ClearCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	
	s.graphCache = make(map[string]*aggregates.GraphLazy)
	s.logger.Info("Cleared lazy graph cache")
}

// GetCacheSize returns the number of cached graphs
func (s *GraphLazyService) GetCacheSize() int {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	
	return len(s.graphCache)
}