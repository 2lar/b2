package services

import (
	"testing"
	"time"

	"backend/domain/core/aggregates"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"
)

// testGraph is a helper to build a graph with nodes and edges for testing.
type testGraph struct {
	graph *aggregates.Graph
	nodes map[valueobjects.NodeID]*entities.Node
	ids   map[string]valueobjects.NodeID // short name → NodeID
}

func newTestGraph(t *testing.T, nodeNames []string) *testGraph {
	t.Helper()
	g, err := aggregates.NewGraph("test-user", "test-graph")
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	nodes := make(map[valueobjects.NodeID]*entities.Node, len(nodeNames))
	ids := make(map[string]valueobjects.NodeID, len(nodeNames))

	for _, name := range nodeNames {
		content, _ := valueobjects.NewNodeContent(name, "content of "+name, valueobjects.FormatPlainText)
		pos, _ := valueobjects.NewPosition2D(0, 0)
		node, err := entities.NewNode("test-user", content, pos)
		if err != nil {
			t.Fatalf("failed to create node %s: %v", name, err)
		}
		if err := g.LoadNode(node); err != nil {
			t.Fatalf("failed to load node %s: %v", name, err)
		}
		nodes[node.ID()] = node
		ids[name] = node.ID()
	}

	return &testGraph{graph: g, nodes: nodes, ids: ids}
}

func (tg *testGraph) addEdge(t *testing.T, from, to string, weight float64) {
	t.Helper()
	edge := &aggregates.Edge{
		ID:        from + "->" + to,
		SourceID:  tg.ids[from],
		TargetID:  tg.ids[to],
		Type:      entities.EdgeTypeNormal,
		Weight:    weight,
		CreatedAt: time.Now(),
	}
	if err := tg.graph.LoadEdge(edge); err != nil {
		t.Fatalf("failed to load edge %s->%s: %v", from, to, err)
	}
}

func (tg *testGraph) setCommunity(name, communityID string) {
	nid := tg.ids[name]
	tg.nodes[nid].SetCommunityID(communityID)
}

func TestTraceChains_EmptyGraph(t *testing.T) {
	tg := newTestGraph(t, []string{})
	svc := NewThoughtChainService()

	// No start node to trace from — create a dummy ID
	dummyID := valueobjects.NewNodeID()
	chains, err := svc.TraceChains(tg.graph, dummyID, tg.nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chains) != 0 {
		t.Errorf("expected 0 chains for empty graph, got %d", len(chains))
	}
}

func TestTraceChains_SingleNode(t *testing.T) {
	tg := newTestGraph(t, []string{"a"})
	svc := NewThoughtChainService()

	chains, err := svc.TraceChains(tg.graph, tg.ids["a"], tg.nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chains) != 0 {
		t.Errorf("expected 0 chains for single node (need ≥2 steps), got %d", len(chains))
	}
}

func TestTraceChains_LinearChain(t *testing.T) {
	tg := newTestGraph(t, []string{"a", "b", "c", "d"})
	tg.addEdge(t, "a", "b", 1.0)
	tg.addEdge(t, "b", "c", 1.0)
	tg.addEdge(t, "c", "d", 1.0)

	svc := NewThoughtChainService()
	chains, err := svc.TraceChains(tg.graph, tg.ids["a"], tg.nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chains) == 0 {
		t.Fatal("expected at least 1 chain")
	}

	// Should have a chain that covers all 4 nodes
	found4 := false
	for _, c := range chains {
		if len(c.Steps) == 4 {
			found4 = true
			if c.Steps[0] != tg.ids["a"].String() {
				t.Errorf("expected chain to start at 'a', got %s", c.Steps[0])
			}
		}
	}
	if !found4 {
		t.Errorf("expected a chain with 4 steps, longest was %d", len(chains[0].Steps))
	}
}

func TestTraceChains_CrossCommunity(t *testing.T) {
	tg := newTestGraph(t, []string{"a", "b", "c"})
	tg.addEdge(t, "a", "b", 1.0)
	tg.addEdge(t, "b", "c", 1.0)
	tg.setCommunity("a", "comm1")
	tg.setCommunity("b", "comm1")
	tg.setCommunity("c", "comm2")

	svc := NewThoughtChainService()
	chains, err := svc.TraceChains(tg.graph, tg.ids["a"], tg.nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chains) == 0 {
		t.Fatal("expected at least 1 chain")
	}

	// The chain a→b→c crosses from comm1 to comm2
	foundCross := false
	for _, c := range chains {
		if c.CommunitiesCrossed > 0 {
			foundCross = true
			break
		}
	}
	if !foundCross {
		t.Error("expected at least one chain with communities_crossed > 0")
	}
}

func TestTraceChains_SortByCrossCommunity(t *testing.T) {
	// Build: a→b→c (cross-community), a→d (no cross)
	tg := newTestGraph(t, []string{"a", "b", "c", "d"})
	tg.addEdge(t, "a", "b", 1.0)
	tg.addEdge(t, "b", "c", 1.0)
	tg.addEdge(t, "a", "d", 1.0)
	tg.setCommunity("a", "comm1")
	tg.setCommunity("b", "comm1")
	tg.setCommunity("c", "comm2")
	tg.setCommunity("d", "comm1")

	svc := NewThoughtChainService()
	chains, err := svc.TraceChains(tg.graph, tg.ids["a"], tg.nodes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chains) < 2 {
		t.Fatalf("expected at least 2 chains, got %d", len(chains))
	}

	// First chain should have higher cross-community count
	if chains[0].CommunitiesCrossed < chains[len(chains)-1].CommunitiesCrossed {
		t.Error("chains should be sorted with cross-community chains first")
	}
}

func TestTraceChains_MaxDepth(t *testing.T) {
	tg := newTestGraph(t, []string{"a", "b", "c", "d", "e"})
	tg.addEdge(t, "a", "b", 1.0)
	tg.addEdge(t, "b", "c", 1.0)
	tg.addEdge(t, "c", "d", 1.0)
	tg.addEdge(t, "d", "e", 1.0)

	svc := NewThoughtChainService()
	cfg := &ThoughtChainConfig{MaxDepth: 2, MaxBranches: 4, MaxChains: 20}
	chains, err := svc.TraceChains(tg.graph, tg.ids["a"], tg.nodes, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, c := range chains {
		if len(c.Steps) > 3 { // start + 2 hops = 3 steps max
			t.Errorf("chain exceeds maxDepth=2, got %d steps", len(c.Steps))
		}
	}
}

func TestTraceChains_MaxBranches(t *testing.T) {
	// Star graph: center connected to 5 leaves
	names := []string{"center", "l1", "l2", "l3", "l4", "l5"}
	tg := newTestGraph(t, names)
	for _, leaf := range names[1:] {
		tg.addEdge(t, "center", leaf, 1.0)
	}

	svc := NewThoughtChainService()
	cfg := &ThoughtChainConfig{MaxDepth: 2, MaxBranches: 2, MaxChains: 20}
	chains, err := svc.TraceChains(tg.graph, tg.ids["center"], tg.nodes, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With MaxBranches=2, should explore at most 2 leaves
	if len(chains) > 2 {
		t.Errorf("expected at most 2 chains with maxBranches=2, got %d", len(chains))
	}
}

func TestFindHubs(t *testing.T) {
	// Star graph: center connected to 4 leaves
	tg := newTestGraph(t, []string{"center", "a", "b", "c", "d"})
	tg.addEdge(t, "center", "a", 1.0)
	tg.addEdge(t, "center", "b", 1.0)
	tg.addEdge(t, "center", "c", 1.0)
	tg.addEdge(t, "center", "d", 1.0)

	svc := NewThoughtChainService()
	hubs := svc.FindHubs(tg.graph, 3)

	if len(hubs) == 0 {
		t.Fatal("expected at least 1 hub")
	}

	// Center should be the top hub (degree 4, others degree 1)
	if hubs[0] != tg.ids["center"].String() {
		t.Errorf("expected center node as top hub, got %s", hubs[0])
	}
}
