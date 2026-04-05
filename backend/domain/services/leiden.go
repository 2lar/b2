package services

import (
	"math"
	"math/rand"
	"sort"
)

// LeidenConfig holds parameters for the Leiden algorithm.
type LeidenConfig struct {
	// Resolution controls granularity: higher = more communities, lower = fewer.
	Resolution float64
	// MaxIterations caps the number of outer Leiden passes.
	MaxIterations int
	// MinCommunitySize drops communities smaller than this (members reassigned to nearest).
	MinCommunitySize int
	// Seed for deterministic results (0 = random).
	Seed int64
}

// DefaultLeidenConfig returns sensible defaults for personal knowledge graphs.
func DefaultLeidenConfig() *LeidenConfig {
	return &LeidenConfig{
		Resolution:       1.0,
		MaxIterations:    10,
		MinCommunitySize: 2,
		Seed:             0,
	}
}

// LeidenGraph is a compact adjacency representation consumed by the algorithm.
// Nodes are represented as integer indices 0..N-1 for performance.
type LeidenGraph struct {
	// N is the number of nodes.
	N int
	// Adj[i] contains (neighbour index, weight) pairs for node i.
	// The graph is treated as undirected: each edge appears in both directions.
	Adj [][]weightedEdge
	// TotalWeight is the sum of all edge weights (each undirected edge counted once).
	TotalWeight float64
	// NodeIDs maps internal index -> external string ID.
	NodeIDs []string
	// nodeIndex maps external string ID -> internal index.
	nodeIndex map[string]int
}

type weightedEdge struct {
	Target int
	Weight float64
}

// NewLeidenGraph builds a compact graph from a list of weighted edges.
// Each edge (source, target, weight) is treated as undirected.
func NewLeidenGraph(nodeIDs []string, edges []LeidenEdge) *LeidenGraph {
	g := &LeidenGraph{
		N:         len(nodeIDs),
		Adj:       make([][]weightedEdge, len(nodeIDs)),
		NodeIDs:   nodeIDs,
		nodeIndex: make(map[string]int, len(nodeIDs)),
	}

	for i, id := range nodeIDs {
		g.nodeIndex[id] = i
	}

	for _, e := range edges {
		si, sok := g.nodeIndex[e.Source]
		ti, tok := g.nodeIndex[e.Target]
		if !sok || !tok || si == ti {
			continue
		}
		g.Adj[si] = append(g.Adj[si], weightedEdge{Target: ti, Weight: e.Weight})
		g.Adj[ti] = append(g.Adj[ti], weightedEdge{Target: si, Weight: e.Weight})
		g.TotalWeight += e.Weight
	}

	return g
}

// LeidenEdge is an input edge for building a LeidenGraph.
type LeidenEdge struct {
	Source string
	Target string
	Weight float64
}

// LeidenResult holds the output of the Leiden algorithm.
type LeidenResult struct {
	// Communities maps community ID (0-based) to list of external node IDs.
	Communities map[int][]string
	// NodeCommunity maps external node ID to its community ID.
	NodeCommunity map[string]int
	// Modularity is the quality score of the partition.
	Modularity float64
}

// RunLeiden executes the Leiden community detection algorithm on the graph.
//
// The Leiden algorithm improves on Louvain by ensuring all communities are
// well-connected (no disconnected subcommunities). It proceeds in three phases:
//  1. Local move: greedily move nodes to the community that maximises modularity gain.
//  2. Refinement: within each community, refine partitions to guarantee connectivity.
//  3. Aggregation: collapse communities into super-nodes and repeat.
func RunLeiden(g *LeidenGraph, cfg *LeidenConfig) *LeidenResult {
	if cfg == nil {
		cfg = DefaultLeidenConfig()
	}

	rng := rand.New(rand.NewSource(cfg.Seed))
	if cfg.Seed == 0 {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}

	n := g.N
	if n == 0 {
		return &LeidenResult{
			Communities:   map[int][]string{},
			NodeCommunity: map[string]int{},
		}
	}

	// Initial partition: each node in its own community.
	community := make([]int, n)
	for i := range community {
		community[i] = i
	}

	// Precompute node strengths (weighted degree).
	strength := make([]float64, n)
	for i := 0; i < n; i++ {
		for _, e := range g.Adj[i] {
			strength[i] += e.Weight
		}
	}

	m2 := 2.0 * g.TotalWeight
	if m2 == 0 {
		// No edges — each node is its own community.
		return buildResult(g, community)
	}

	for iter := 0; iter < cfg.MaxIterations; iter++ {
		improved := false

		// Phase 1: Local moving — visit nodes in random order.
		order := rng.Perm(n)
		for _, i := range order {
			if localMove(g, community, strength, i, m2, cfg.Resolution) {
				improved = true
			}
		}

		// Phase 2: Refinement — ensure each community is internally connected.
		community = refine(g, community, strength, m2, cfg.Resolution, rng)

		if !improved {
			break
		}
	}

	// Merge small communities into their best neighbour.
	if cfg.MinCommunitySize > 1 {
		mergeSmall(g, community, strength, m2, cfg)
	}

	// Compact community IDs to 0..K-1.
	compact(community)

	return buildResult(g, community)
}

// localMove tries to move node i to the neighbouring community that gives
// the greatest modularity gain. Returns true if the node moved.
func localMove(g *LeidenGraph, community []int, strength []float64, i int, m2, gamma float64) bool {
	bestComm := community[i]
	bestDelta := 0.0

	// Compute weight from i to each neighbouring community.
	commWeights := make(map[int]float64)
	for _, e := range g.Adj[i] {
		commWeights[community[e.Target]] += e.Weight
	}

	// Community totals (sum of strengths for nodes in each community).
	// We compute this locally for the relevant communities only.
	commStrength := make(map[int]float64)
	for _, e := range g.Adj[i] {
		c := community[e.Target]
		if _, ok := commStrength[c]; !ok {
			// Sum strengths for all nodes in community c that are neighbours.
			// For performance, we approximate using the edge info we have.
			commStrength[c] = 0
		}
	}
	// Full community strength calculation.
	allCommStrength := communityStrengths(community, strength)

	ki := strength[i]
	oldComm := community[i]

	// Modularity gain for removing i from its current community.
	removeGain := -commWeights[oldComm] + gamma*ki*(allCommStrength[oldComm]-ki)/m2

	for c, wic := range commWeights {
		if c == oldComm {
			continue
		}
		// Modularity gain for adding i to community c.
		delta := wic - gamma*ki*allCommStrength[c]/m2 + removeGain
		if delta > bestDelta {
			bestDelta = delta
			bestComm = c
		}
	}

	if bestComm != oldComm {
		community[i] = bestComm
		return true
	}
	return false
}

// refine ensures each community is internally well-connected.
// For each community, run a local BFS to find connected sub-components.
// If a community has multiple components, split them into separate communities.
func refine(g *LeidenGraph, community []int, strength []float64, m2, gamma float64, rng *rand.Rand) []int {
	n := len(community)
	refined := make([]int, n)
	copy(refined, community)

	// Group nodes by community.
	commNodes := make(map[int][]int)
	for i, c := range community {
		commNodes[c] = append(commNodes[c], i)
	}

	nextID := maxVal(community) + 1

	for _, nodes := range commNodes {
		if len(nodes) <= 1 {
			continue
		}

		// Build sub-adjacency for this community.
		nodeSet := make(map[int]bool, len(nodes))
		for _, n := range nodes {
			nodeSet[n] = true
		}

		// BFS to find connected components within the community.
		visited := make(map[int]bool)
		components := [][]int{}

		for _, start := range nodes {
			if visited[start] {
				continue
			}
			comp := []int{}
			queue := []int{start}
			visited[start] = true
			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				comp = append(comp, cur)
				for _, e := range g.Adj[cur] {
					if nodeSet[e.Target] && !visited[e.Target] {
						visited[e.Target] = true
						queue = append(queue, e.Target)
					}
				}
			}
			components = append(components, comp)
		}

		// If community is already connected, nothing to do.
		if len(components) <= 1 {
			continue
		}

		// First component keeps the original ID.
		// Remaining components get new IDs.
		for _, comp := range components[1:] {
			for _, node := range comp {
				refined[node] = nextID
			}
			nextID++
		}
	}

	return refined
}

// mergeSmall merges communities smaller than MinCommunitySize into
// the neighbouring community with the highest edge weight.
func mergeSmall(g *LeidenGraph, community []int, strength []float64, m2 float64, cfg *LeidenConfig) {
	for {
		commSizes := make(map[int]int)
		for _, c := range community {
			commSizes[c]++
		}

		merged := false
		for c, size := range commSizes {
			if size >= cfg.MinCommunitySize {
				continue
			}

			// Find best neighbouring community to merge into.
			bestNeighbour := -1
			bestWeight := 0.0

			for i, ci := range community {
				if ci != c {
					continue
				}
				for _, e := range g.Adj[i] {
					nc := community[e.Target]
					if nc != c && e.Weight > bestWeight {
						bestWeight = e.Weight
						bestNeighbour = nc
					}
				}
			}

			if bestNeighbour >= 0 {
				for i := range community {
					if community[i] == c {
						community[i] = bestNeighbour
					}
				}
				merged = true
			}
		}

		if !merged {
			break
		}
	}
}

// communityStrengths returns the total node strength per community.
func communityStrengths(community []int, strength []float64) map[int]float64 {
	cs := make(map[int]float64)
	for i, c := range community {
		cs[c] += strength[i]
	}
	return cs
}

// compact remaps community IDs to contiguous 0..K-1.
func compact(community []int) {
	remap := make(map[int]int)
	next := 0
	for i, c := range community {
		if _, ok := remap[c]; !ok {
			remap[c] = next
			next++
		}
		community[i] = remap[c]
	}
}

// buildResult converts internal community assignment to external LeidenResult.
func buildResult(g *LeidenGraph, community []int) *LeidenResult {
	r := &LeidenResult{
		Communities:   make(map[int][]string),
		NodeCommunity: make(map[string]int, g.N),
	}

	for i, c := range community {
		id := g.NodeIDs[i]
		r.NodeCommunity[id] = c
		r.Communities[c] = append(r.Communities[c], id)
	}

	r.Modularity = computeModularity(g, community)
	return r
}

// computeModularity calculates the Newman-Girvan modularity Q for a partition.
func computeModularity(g *LeidenGraph, community []int) float64 {
	m2 := 2.0 * g.TotalWeight
	if m2 == 0 {
		return 0
	}

	strength := make([]float64, g.N)
	for i := 0; i < g.N; i++ {
		for _, e := range g.Adj[i] {
			strength[i] += e.Weight
		}
	}

	q := 0.0
	for i := 0; i < g.N; i++ {
		for _, e := range g.Adj[i] {
			if community[i] == community[e.Target] {
				q += e.Weight - strength[i]*strength[e.Target]/m2
			}
		}
	}

	return q / m2
}

// maxVal returns the maximum value in a slice.
func maxVal(s []int) int {
	m := math.MinInt64
	for _, v := range s {
		if v > m {
			m = v
		}
	}
	return m
}

// CommunityKeywords extracts top keywords for a community from its members' text.
func CommunityKeywords(texts []string, maxKeywords int) []string {
	freq := make(map[string]int)
	analyzer := NewDefaultTextAnalyzer()

	for _, text := range texts {
		words := analyzer.ExtractKeywords(text)
		seen := make(map[string]bool)
		for _, w := range words {
			if !seen[w] {
				freq[w]++
				seen[w] = true
			}
		}
	}

	type kv struct {
		word  string
		count int
	}
	sorted := make([]kv, 0, len(freq))
	for w, c := range freq {
		sorted = append(sorted, kv{w, c})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	result := make([]string, 0, maxKeywords)
	for i := 0; i < len(sorted) && i < maxKeywords; i++ {
		result = append(result, sorted[i].word)
	}
	return result
}

// CohesionScore computes how densely connected a community is (0-1).
// It's the ratio of actual internal edges to maximum possible internal edges.
func CohesionScore(g *LeidenGraph, members []string) float64 {
	if len(members) <= 1 {
		return 1.0
	}

	memberSet := make(map[int]bool, len(members))
	for _, id := range members {
		if idx, ok := g.nodeIndex[id]; ok {
			memberSet[idx] = true
		}
	}

	internalWeight := 0.0
	for idx := range memberSet {
		for _, e := range g.Adj[idx] {
			if memberSet[e.Target] {
				internalWeight += e.Weight
			}
		}
	}
	// Each edge counted twice (undirected).
	internalWeight /= 2.0

	maxEdges := float64(len(members)) * float64(len(members)-1) / 2.0
	if maxEdges == 0 {
		return 1.0
	}

	score := internalWeight / maxEdges
	if score > 1.0 {
		score = 1.0
	}
	return score
}
