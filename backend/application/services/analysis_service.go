package services

import (
	"context"
	"fmt"

	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	domainservices "backend/domain/services"
	"go.uber.org/zap"
)

// AnalysisService orchestrates thought chain tracing and impact analysis
// across a user's graph.
type AnalysisService struct {
	graphRepo    ports.GraphRepository
	nodeRepo     ports.NodeRepository
	edgeRepo     ports.EdgeRepository
	chainService *domainservices.ThoughtChainService
	impactService *domainservices.ImpactAnalysisService
	logger       *zap.Logger
}

// NewAnalysisService creates a new analysis service.
func NewAnalysisService(
	graphRepo ports.GraphRepository,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	logger *zap.Logger,
) *AnalysisService {
	return &AnalysisService{
		graphRepo:    graphRepo,
		nodeRepo:     nodeRepo,
		edgeRepo:     edgeRepo,
		chainService: domainservices.NewThoughtChainService(),
		impactService: domainservices.NewImpactAnalysisService(),
		logger:       logger,
	}
}

// ThoughtChainResult is the API response for thought chains.
type ThoughtChainResult struct {
	Chains     []domainservices.ThoughtChain `json:"chains"`
	TotalFound int                           `json:"total_found"`
	Hubs       []string                      `json:"hubs"`
}

// GetThoughtChains traces thought chains starting from a node.
func (s *AnalysisService) GetThoughtChains(
	ctx context.Context,
	userID string,
	nodeID string,
	maxDepth int,
	maxBranches int,
) (*ThoughtChainResult, error) {
	graph, nodes, err := s.loadGraph(ctx, userID)
	if err != nil {
		return nil, err
	}

	startID, err := valueobjects.NewNodeIDFromString(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}

	if _, ok := nodes[startID]; !ok {
		return nil, fmt.Errorf("node not found in graph")
	}

	cfg := domainservices.DefaultThoughtChainConfig()
	if maxDepth > 0 {
		cfg.MaxDepth = maxDepth
	}
	if maxBranches > 0 {
		cfg.MaxBranches = maxBranches
	}

	chains, err := s.chainService.TraceChains(graph, startID, nodes, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to trace chains: %w", err)
	}

	hubs := s.chainService.FindHubs(graph, 5)

	return &ThoughtChainResult{
		Chains:     chains,
		TotalFound: len(chains),
		Hubs:       hubs,
	}, nil
}

// ImpactResult is the API response for impact analysis.
type ImpactResult = domainservices.ImpactAnalysis

// GetImpactAnalysis computes the impact of removing a node.
func (s *AnalysisService) GetImpactAnalysis(
	ctx context.Context,
	userID string,
	nodeID string,
	maxDepth int,
) (*ImpactResult, error) {
	graph, nodes, err := s.loadGraph(ctx, userID)
	if err != nil {
		return nil, err
	}

	targetID, err := valueobjects.NewNodeIDFromString(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %w", err)
	}

	if _, ok := nodes[targetID]; !ok {
		return nil, fmt.Errorf("node not found in graph")
	}

	result, err := s.impactService.Analyze(graph, targetID, nodes, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("impact analysis failed: %w", err)
	}

	return result, nil
}

// loadGraph loads the user's default graph with all nodes and edges.
func (s *AnalysisService) loadGraph(
	ctx context.Context,
	userID string,
) (*aggregates.Graph, map[valueobjects.NodeID]*entities.Node, error) {
	graph, err := s.graphRepo.GetUserDefaultGraph(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get graph: %w", err)
	}

	nodesList, err := s.nodeRepo.GetByGraphID(ctx, graph.ID().String())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load nodes: %w", err)
	}

	edges, err := s.edgeRepo.GetByGraphID(ctx, graph.ID().String())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load edges: %w", err)
	}

	// Load into graph aggregate
	for _, node := range nodesList {
		if err := graph.LoadNode(node); err != nil {
			s.logger.Warn("Failed to load node into graph", zap.Error(err))
		}
	}
	for _, edge := range edges {
		if err := graph.LoadEdge(edge); err != nil {
			s.logger.Warn("Failed to load edge into graph", zap.Error(err))
		}
	}

	nodes := make(map[valueobjects.NodeID]*entities.Node, len(nodesList))
	for _, n := range nodesList {
		nodes[n.ID()] = n
	}

	return graph, nodes, nil
}
