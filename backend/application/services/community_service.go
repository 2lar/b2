package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"backend/application/ports"
	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	domainservices "backend/domain/services"
	"go.uber.org/zap"
)

// CommunityDetectionService orchestrates Leiden community detection
// across a user's graph, assigns community IDs to nodes, and extracts
// keyword-based names for each community.
type CommunityDetectionService struct {
	graphRepo ports.GraphRepository
	nodeRepo  ports.NodeRepository
	edgeRepo  ports.EdgeRepository
	config    *domainservices.LeidenConfig
	logger    *zap.Logger
}

// NewCommunityDetectionService creates a new service.
func NewCommunityDetectionService(
	graphRepo ports.GraphRepository,
	nodeRepo ports.NodeRepository,
	edgeRepo ports.EdgeRepository,
	logger *zap.Logger,
) *CommunityDetectionService {
	return &CommunityDetectionService{
		graphRepo: graphRepo,
		nodeRepo:  nodeRepo,
		edgeRepo:  edgeRepo,
		config:    domainservices.DefaultLeidenConfig(),
		logger:    logger,
	}
}

// DetectionResult holds the full output of community detection.
type DetectionResult struct {
	Communities []CommunityInfo `json:"communities"`
	Modularity  float64         `json:"modularity"`
	NodeCount   int             `json:"node_count"`
}

// CommunityInfo is the per-community metadata returned by detection.
type CommunityInfo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Keywords      []string `json:"keywords"`
	CohesionScore float64  `json:"cohesion_score"`
	MemberCount   int      `json:"member_count"`
	MemberIDs     []string `json:"member_ids"`
}

// DetectCommunities runs Leiden on a user's default graph,
// assigns community IDs to nodes, and returns community metadata.
func (s *CommunityDetectionService) DetectCommunities(ctx context.Context, userID string) (*DetectionResult, error) {
	// Load graph
	graph, err := s.graphRepo.GetUserDefaultGraph(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get graph: %w", err)
	}

	// Load nodes and edges
	nodes, err := s.nodeRepo.GetByGraphID(ctx, graph.ID().String())
	if err != nil {
		return nil, fmt.Errorf("failed to load nodes: %w", err)
	}

	edges, err := s.edgeRepo.GetByGraphID(ctx, graph.ID().String())
	if err != nil {
		return nil, fmt.Errorf("failed to load edges: %w", err)
	}

	if len(nodes) == 0 {
		return &DetectionResult{Communities: []CommunityInfo{}}, nil
	}

	// Build Leiden graph from domain objects.
	leidenGraph, nodeMap := buildLeidenGraph(nodes, edges)

	// Run Leiden.
	result := domainservices.RunLeiden(leidenGraph, s.config)

	s.logger.Info("Leiden community detection complete",
		zap.String("userID", userID),
		zap.Int("nodes", len(nodes)),
		zap.Int("communities", len(result.Communities)),
		zap.Float64("modularity", result.Modularity),
	)

	// Build community metadata and assign to nodes.
	communities := make([]CommunityInfo, 0, len(result.Communities))

	for commID, memberIDs := range result.Communities {
		// Extract text from members for keyword naming.
		texts := make([]string, 0, len(memberIDs))
		for _, nid := range memberIDs {
			if n, ok := nodeMap[nid]; ok {
				c := n.Content()
				texts = append(texts, c.Title()+" "+c.Body())
			}
		}

		keywords := domainservices.CommunityKeywords(texts, 5)
		cohesion := domainservices.CohesionScore(leidenGraph, memberIDs)

		name := "Cluster " + strconv.Itoa(commID)
		if len(keywords) > 0 {
			name = keywords[0]
			if len(keywords) > 1 {
				name += " & " + keywords[1]
			}
		}

		commIDStr := strconv.Itoa(commID)
		communities = append(communities, CommunityInfo{
			ID:            commIDStr,
			Name:          name,
			Keywords:      keywords,
			CohesionScore: cohesion,
			MemberCount:   len(memberIDs),
			MemberIDs:     memberIDs,
		})

		// Assign community ID to each member node.
		for _, nid := range memberIDs {
			if n, ok := nodeMap[nid]; ok {
				n.SetCommunityID(commIDStr)
			}
		}
	}

	// Persist updated nodes (community assignments).
	for _, node := range nodes {
		if err := s.nodeRepo.Save(ctx, node); err != nil {
			s.logger.Warn("Failed to persist community assignment",
				zap.String("nodeID", node.ID().String()),
				zap.Error(err),
			)
		}
	}

	return &DetectionResult{
		Communities: communities,
		Modularity:  result.Modularity,
		NodeCount:   len(nodes),
	}, nil
}

// buildLeidenGraph converts domain objects to the compact Leiden representation.
func buildLeidenGraph(
	nodes []*entities.Node,
	edges []*aggregates.Edge,
) (*domainservices.LeidenGraph, map[string]*entities.Node) {
	nodeIDs := make([]string, len(nodes))
	nodeMap := make(map[string]*entities.Node, len(nodes))
	for i, n := range nodes {
		id := n.ID().String()
		nodeIDs[i] = id
		nodeMap[id] = n
	}

	leidenEdges := make([]domainservices.LeidenEdge, 0, len(edges))
	for _, e := range edges {
		leidenEdges = append(leidenEdges, domainservices.LeidenEdge{
			Source: e.SourceID.String(),
			Target: e.TargetID.String(),
			Weight: e.Weight,
		})
	}

	return domainservices.NewLeidenGraph(nodeIDs, leidenEdges), nodeMap
}

// CreateCommunityEntities converts DetectionResult into domain Community entities.
func CreateCommunityEntities(graphID string, result *DetectionResult) []*entities.Community {
	now := time.Now()
	communities := make([]*entities.Community, 0, len(result.Communities))

	for _, ci := range result.Communities {
		communities = append(communities, &entities.Community{
			ID:            ci.ID,
			GraphID:       graphID,
			Name:          ci.Name,
			Keywords:      ci.Keywords,
			CohesionScore: ci.CohesionScore,
			MemberCount:   ci.MemberCount,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}

	return communities
}
