package services

import (
	"math"
	"testing"
)

func TestCentrality_EmptyGraph(t *testing.T) {
	tg := newTestGraph(t, []string{})
	svc := NewGraphAnalyticsService()

	result, err := svc.CalculateCentrality(tg.graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestCentrality_SingleNode(t *testing.T) {
	tg := newTestGraph(t, []string{"a"})
	svc := NewGraphAnalyticsService()

	result, err := svc.CalculateCentrality(tg.graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[tg.ids["a"]] != 0.0 {
		t.Errorf("expected 0 centrality for single node, got %f", result[tg.ids["a"]])
	}
}

func TestCentrality_LineGraph(t *testing.T) {
	// A - B - C: B is on every shortest path between A and C
	tg := newTestGraph(t, []string{"a", "b", "c"})
	tg.addEdge(t, "a", "b", 1.0)
	tg.addEdge(t, "b", "c", 1.0)

	svc := NewGraphAnalyticsService()
	result, err := svc.CalculateCentrality(tg.graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// B has highest centrality (normalized to 1.0), A and C have 0
	if result[tg.ids["b"]] != 1.0 {
		t.Errorf("expected B centrality = 1.0, got %f", result[tg.ids["b"]])
	}
	if result[tg.ids["a"]] != 0.0 {
		t.Errorf("expected A centrality = 0.0, got %f", result[tg.ids["a"]])
	}
	if result[tg.ids["c"]] != 0.0 {
		t.Errorf("expected C centrality = 0.0, got %f", result[tg.ids["c"]])
	}
}

func TestCentrality_Triangle(t *testing.T) {
	// Fully connected triangle: no node is "between" the other two
	// (direct paths exist for every pair)
	tg := newTestGraph(t, []string{"a", "b", "c"})
	tg.addEdge(t, "a", "b", 1.0)
	tg.addEdge(t, "b", "c", 1.0)
	tg.addEdge(t, "a", "c", 1.0)

	svc := NewGraphAnalyticsService()
	result, err := svc.CalculateCentrality(tg.graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All centralities should be equal (all 0 since direct paths exist)
	for name, nid := range tg.ids {
		if result[nid] != 0.0 {
			t.Errorf("expected 0 centrality for %s in triangle, got %f", name, result[nid])
		}
	}
}

func TestCentrality_StarGraph(t *testing.T) {
	// Center connected to 4 leaves: center is on all shortest paths between leaves
	tg := newTestGraph(t, []string{"center", "a", "b", "c", "d"})
	tg.addEdge(t, "center", "a", 1.0)
	tg.addEdge(t, "center", "b", 1.0)
	tg.addEdge(t, "center", "c", 1.0)
	tg.addEdge(t, "center", "d", 1.0)

	svc := NewGraphAnalyticsService()
	result, err := svc.CalculateCentrality(tg.graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Center should have the highest centrality (normalized to 1.0)
	if result[tg.ids["center"]] != 1.0 {
		t.Errorf("expected center centrality = 1.0, got %f", result[tg.ids["center"]])
	}
	// All leaves should have 0 centrality
	for _, leaf := range []string{"a", "b", "c", "d"} {
		if result[tg.ids[leaf]] != 0.0 {
			t.Errorf("expected leaf %s centrality = 0.0, got %f", leaf, result[tg.ids[leaf]])
		}
	}
}

func TestCentrality_DisconnectedGraph(t *testing.T) {
	// Two disconnected pairs: a-b and c-d
	tg := newTestGraph(t, []string{"a", "b", "c", "d"})
	tg.addEdge(t, "a", "b", 1.0)
	tg.addEdge(t, "c", "d", 1.0)

	svc := NewGraphAnalyticsService()
	result, err := svc.CalculateCentrality(tg.graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No node is between any other pair (all pairs are either directly connected or unreachable)
	for name, nid := range tg.ids {
		if result[nid] != 0.0 {
			t.Errorf("expected 0 centrality for %s in disconnected graph, got %f", name, result[nid])
		}
	}
}

func TestCentrality_LongerLine(t *testing.T) {
	// A - B - C - D - E: B, C, D have decreasing centrality
	tg := newTestGraph(t, []string{"a", "b", "c", "d", "e"})
	tg.addEdge(t, "a", "b", 1.0)
	tg.addEdge(t, "b", "c", 1.0)
	tg.addEdge(t, "c", "d", 1.0)
	tg.addEdge(t, "d", "e", 1.0)

	svc := NewGraphAnalyticsService()
	result, err := svc.CalculateCentrality(tg.graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// C should have the highest centrality (center of the line)
	cVal := result[tg.ids["c"]]
	bVal := result[tg.ids["b"]]
	dVal := result[tg.ids["d"]]

	if cVal != 1.0 {
		t.Errorf("expected C (center) centrality = 1.0, got %f", cVal)
	}
	// B and D should be equal (symmetric positions)
	if math.Abs(bVal-dVal) > 0.001 {
		t.Errorf("expected B and D to have equal centrality, got B=%f D=%f", bVal, dVal)
	}
	// B/D should be less than C
	if bVal >= cVal {
		t.Errorf("expected B centrality < C centrality, got B=%f C=%f", bVal, cVal)
	}
}
