package services

import (
	"sort"

	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
	"fmt"
)

// RiskLevel classifies the severity of an impact.
type RiskLevel string

const (
	RiskCritical RiskLevel = "CRITICAL"
	RiskHigh     RiskLevel = "HIGH"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskLow      RiskLevel = "LOW"
)

// ImpactTier classifies how a dependent node is affected.
type ImpactTier string

const (
	TierWillBreak      ImpactTier = "WILL_BREAK"
	TierLikelyAffected ImpactTier = "LIKELY_AFFECTED"
	TierMayAffect      ImpactTier = "MAY_AFFECT"
)

// ImpactAnalysis is the result of analyzing what happens if a node is removed.
type ImpactAnalysis struct {
	TargetNodeID           string            `json:"target_node_id"`
	RiskLevel              RiskLevel         `json:"risk_level"`
	AffectedCommunityCount int               `json:"affected_community_count"`
	TotalAffectedNodes     int               `json:"total_affected_nodes"`
	Summary                string            `json:"summary"`
	Dependents             []DependencyGroup `json:"dependents"`
}

// DependencyGroup groups affected nodes by tier and depth.
type DependencyGroup struct {
	Tier    ImpactTier `json:"tier"`
	Depth   int        `json:"depth"`
	NodeIDs []string   `json:"node_ids"`
}

// ImpactAnalysisService computes the blast radius of removing a node.
type ImpactAnalysisService struct{}

// NewImpactAnalysisService creates a new service.
func NewImpactAnalysisService() *ImpactAnalysisService {
	return &ImpactAnalysisService{}
}

// Analyze computes the impact of removing the target node from the graph.
func (s *ImpactAnalysisService) Analyze(
	graph *aggregates.Graph,
	targetID valueobjects.NodeID,
	nodes map[valueobjects.NodeID]*entities.Node,
	maxDepth int,
) (*ImpactAnalysis, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}

	targetStr := targetID.String()
	adj := buildAdjacency(graph)

	// Build string-keyed community lookup
	communityOf := make(map[string]string, len(nodes))
	for nid, node := range nodes {
		communityOf[nid.String()] = node.CommunityID()
	}

	// BFS from target, tracking depth
	type bfsEntry struct {
		id    string
		depth int
	}
	visited := map[string]int{targetStr: 0} // nodeID → depth
	queue := []bfsEntry{{id: targetStr, depth: 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth >= maxDepth {
			continue
		}
		for _, nb := range adj[cur.id] {
			if _, seen := visited[nb.id]; !seen {
				visited[nb.id] = cur.depth + 1
				queue = append(queue, bfsEntry{id: nb.id, depth: cur.depth + 1})
			}
		}
	}

	// Count direct connections to target
	directNeighbors := make(map[string]bool)
	for _, nb := range adj[targetStr] {
		directNeighbors[nb.id] = true
	}

	// Determine if target is a bridge node (connects different communities)
	targetCommunity := communityOf[targetStr]

	bridgesCommunities := false
	neighborCommunities := make(map[string]bool)
	for nbID := range directNeighbors {
		c := communityOf[nbID]
		if c != "" {
			neighborCommunities[c] = true
		}
	}
	if len(neighborCommunities) > 1 {
		bridgesCommunities = true
	}

	// Classify dependents by tier
	var groups []DependencyGroup
	willBreak := make(map[int][]string)  // depth → nodeIDs
	likelyAffected := make(map[int][]string)
	mayAffect := make(map[int][]string)

	allCommunities := make(map[string]bool)
	if targetCommunity != "" {
		allCommunities[targetCommunity] = true
	}

	for nodeID, depth := range visited {
		if nodeID == targetStr {
			continue
		}

		nodeCommunity := communityOf[nodeID]
		if nodeCommunity != "" {
			allCommunities[nodeCommunity] = true
		}

		if depth == 1 {
			// Direct connections always WILL_BREAK
			willBreak[depth] = append(willBreak[depth], nodeID)
		} else if depth == 2 {
			if nodeCommunity != "" && nodeCommunity == targetCommunity {
				likelyAffected[depth] = append(likelyAffected[depth], nodeID)
			} else {
				mayAffect[depth] = append(mayAffect[depth], nodeID)
			}
		} else {
			mayAffect[depth] = append(mayAffect[depth], nodeID)
		}
	}

	// Build groups
	for depth, ids := range willBreak {
		sort.Strings(ids)
		groups = append(groups, DependencyGroup{Tier: TierWillBreak, Depth: depth, NodeIDs: ids})
	}
	for depth, ids := range likelyAffected {
		sort.Strings(ids)
		groups = append(groups, DependencyGroup{Tier: TierLikelyAffected, Depth: depth, NodeIDs: ids})
	}
	for depth, ids := range mayAffect {
		sort.Strings(ids)
		groups = append(groups, DependencyGroup{Tier: TierMayAffect, Depth: depth, NodeIDs: ids})
	}

	// Sort groups by tier severity then depth
	tierOrder := map[ImpactTier]int{TierWillBreak: 0, TierLikelyAffected: 1, TierMayAffect: 2}
	sort.Slice(groups, func(i, j int) bool {
		if tierOrder[groups[i].Tier] != tierOrder[groups[j].Tier] {
			return tierOrder[groups[i].Tier] < tierOrder[groups[j].Tier]
		}
		return groups[i].Depth < groups[j].Depth
	})

	totalAffected := len(visited) - 1 // exclude target itself

	// Determine risk level
	risk := s.calculateRisk(len(directNeighbors), bridgesCommunities, totalAffected, len(allCommunities))

	summary := s.buildSummary(totalAffected, len(allCommunities), bridgesCommunities)

	return &ImpactAnalysis{
		TargetNodeID:           targetStr,
		RiskLevel:              risk,
		AffectedCommunityCount: len(allCommunities),
		TotalAffectedNodes:     totalAffected,
		Summary:                summary,
		Dependents:             groups,
	}, nil
}

func (s *ImpactAnalysisService) calculateRisk(directCount int, bridges bool, totalAffected int, communityCount int) RiskLevel {
	if bridges && directCount >= 5 {
		return RiskCritical
	}
	if bridges || directCount >= 4 {
		return RiskHigh
	}
	if directCount >= 2 || totalAffected >= 5 {
		return RiskMedium
	}
	return RiskLow
}

func (s *ImpactAnalysisService) buildSummary(totalAffected int, communityCount int, bridges bool) string {
	if totalAffected == 0 {
		return "This note has no connections and can be safely removed."
	}

	plural := ""
	if totalAffected != 1 {
		plural = "s"
	}

	communityNote := ""
	if communityCount > 1 {
		communityNote = fmt.Sprintf(" across %d communities", communityCount)
	}

	bridgeNote := ""
	if bridges {
		bridgeNote = " This note acts as a bridge between communities."
	}

	return fmt.Sprintf("Removing this note will affect %d note%s%s.%s", totalAffected, plural, communityNote, bridgeNote)
}
