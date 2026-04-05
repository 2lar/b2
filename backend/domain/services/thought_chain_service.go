package services

import (
	"sort"

	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
)

// ThoughtChain represents a traced path of ideas through the graph,
// optionally crossing community boundaries.
type ThoughtChain struct {
	EntryNodeID       string   `json:"entry_node_id"`
	Steps             []string `json:"steps"`
	CommunitiesCrossed int     `json:"communities_crossed"`
}

// ThoughtChainConfig holds parameters for chain tracing.
type ThoughtChainConfig struct {
	MaxDepth    int // Maximum hops to follow from the entry node
	MaxBranches int // Maximum neighbors to explore per node
	MaxChains   int // Maximum number of chains to return
}

// DefaultThoughtChainConfig returns sensible defaults.
func DefaultThoughtChainConfig() *ThoughtChainConfig {
	return &ThoughtChainConfig{
		MaxDepth:    10,
		MaxBranches: 4,
		MaxChains:   20,
	}
}

// ThoughtChainService traces how ideas flow through a knowledge graph.
type ThoughtChainService struct{}

// NewThoughtChainService creates a new service.
func NewThoughtChainService() *ThoughtChainService {
	return &ThoughtChainService{}
}

// TraceChains finds thought chains starting from the given node.
// It performs a bounded DFS, recording each root-to-leaf path and
// counting community transitions along the way.
func (s *ThoughtChainService) TraceChains(
	graph *aggregates.Graph,
	startID valueobjects.NodeID,
	nodes map[valueobjects.NodeID]*entities.Node,
	cfg *ThoughtChainConfig,
) ([]ThoughtChain, error) {
	if cfg == nil {
		cfg = DefaultThoughtChainConfig()
	}

	// Build adjacency list from edges
	adj := buildAdjacency(graph)

	// Get community map for crossing detection
	communityOf := make(map[string]string)
	for nid, node := range nodes {
		communityOf[nid.String()] = node.CommunityID()
	}

	var chains []ThoughtChain
	visited := make(map[string]bool)
	path := []string{startID.String()}
	visited[startID.String()] = true

	s.dfsTrace(adj, communityOf, startID.String(), path, visited, 0, cfg, &chains)

	// Sort: cross-community chains first, then by length descending
	sort.Slice(chains, func(i, j int) bool {
		if chains[i].CommunitiesCrossed != chains[j].CommunitiesCrossed {
			return chains[i].CommunitiesCrossed > chains[j].CommunitiesCrossed
		}
		return len(chains[i].Steps) > len(chains[j].Steps)
	})

	if len(chains) > cfg.MaxChains {
		chains = chains[:cfg.MaxChains]
	}

	return chains, nil
}

// FindHubs identifies hub nodes using degree centrality.
// Returns node IDs sorted by total degree (in+out) descending.
func (s *ThoughtChainService) FindHubs(
	graph *aggregates.Graph,
	topN int,
) []string {
	degree := make(map[string]int)
	for _, edge := range graph.Edges() {
		degree[edge.SourceID.String()]++
		degree[edge.TargetID.String()]++
	}

	type kv struct {
		id  string
		deg int
	}
	var sorted []kv
	for id, d := range degree {
		sorted = append(sorted, kv{id, d})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].deg > sorted[j].deg
	})

	if topN > len(sorted) {
		topN = len(sorted)
	}
	result := make([]string, topN)
	for i := 0; i < topN; i++ {
		result[i] = sorted[i].id
	}
	return result
}

func (s *ThoughtChainService) dfsTrace(
	adj map[string][]neighborEdge,
	communityOf map[string]string,
	current string,
	path []string,
	visited map[string]bool,
	depth int,
	cfg *ThoughtChainConfig,
	chains *[]ThoughtChain,
) {
	neighbors := adj[current]
	if depth >= cfg.MaxDepth || len(neighbors) == 0 {
		if len(path) >= 2 {
			chain := makeChain(path, communityOf)
			*chains = append(*chains, chain)
		}
		return
	}

	// Sort neighbors by edge weight descending to explore strongest first
	sort.Slice(neighbors, func(i, j int) bool {
		return neighbors[i].weight > neighbors[j].weight
	})

	explored := 0
	isLeaf := true
	for _, nb := range neighbors {
		if visited[nb.id] {
			continue
		}
		if explored >= cfg.MaxBranches {
			break
		}
		isLeaf = false
		explored++
		visited[nb.id] = true
		newPath := append(append([]string{}, path...), nb.id)
		s.dfsTrace(adj, communityOf, nb.id, newPath, visited, depth+1, cfg, chains)
		delete(visited, nb.id)
	}

	// If this is a leaf (no unvisited neighbors), record the path
	if isLeaf && len(path) >= 2 {
		chain := makeChain(path, communityOf)
		*chains = append(*chains, chain)
	}
}

type neighborEdge struct {
	id     string
	weight float64
}

func buildAdjacency(graph *aggregates.Graph) map[string][]neighborEdge {
	adj := make(map[string][]neighborEdge)
	for _, edge := range graph.Edges() {
		src := edge.SourceID.String()
		tgt := edge.TargetID.String()
		adj[src] = append(adj[src], neighborEdge{id: tgt, weight: edge.Weight})
		// Treat edges as bidirectional for traversal
		adj[tgt] = append(adj[tgt], neighborEdge{id: src, weight: edge.Weight})
	}
	return adj
}

func makeChain(path []string, communityOf map[string]string) ThoughtChain {
	steps := make([]string, len(path))
	copy(steps, path)

	crossed := 0
	for i := 1; i < len(steps); i++ {
		c1 := communityOf[steps[i-1]]
		c2 := communityOf[steps[i]]
		if c1 != "" && c2 != "" && c1 != c2 {
			crossed++
		}
	}

	return ThoughtChain{
		EntryNodeID:       steps[0],
		Steps:             steps,
		CommunitiesCrossed: crossed,
	}
}
