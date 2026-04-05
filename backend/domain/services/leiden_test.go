package services

import (
	"fmt"
	"testing"
)

func TestLeiden_EmptyGraph(t *testing.T) {
	g := NewLeidenGraph([]string{}, []LeidenEdge{})
	result := RunLeiden(g, nil)

	if len(result.Communities) != 0 {
		t.Errorf("expected 0 communities, got %d", len(result.Communities))
	}
}

func TestLeiden_SingleNode(t *testing.T) {
	g := NewLeidenGraph([]string{"a"}, []LeidenEdge{})
	result := RunLeiden(g, nil)

	if len(result.Communities) != 1 {
		t.Errorf("expected 1 community, got %d", len(result.Communities))
	}
	if result.NodeCommunity["a"] != 0 {
		t.Errorf("expected node 'a' in community 0, got %d", result.NodeCommunity["a"])
	}
}

func TestLeiden_TwoDisconnectedNodes(t *testing.T) {
	g := NewLeidenGraph([]string{"a", "b"}, []LeidenEdge{})
	cfg := DefaultLeidenConfig()
	cfg.MinCommunitySize = 1 // Don't merge singletons
	result := RunLeiden(g, cfg)

	if len(result.Communities) != 2 {
		t.Errorf("expected 2 communities for disconnected nodes, got %d", len(result.Communities))
	}
	if result.NodeCommunity["a"] == result.NodeCommunity["b"] {
		t.Error("disconnected nodes should be in different communities")
	}
}

func TestLeiden_TwoConnectedNodes(t *testing.T) {
	g := NewLeidenGraph(
		[]string{"a", "b"},
		[]LeidenEdge{{Source: "a", Target: "b", Weight: 1.0}},
	)
	result := RunLeiden(g, nil)

	if result.NodeCommunity["a"] != result.NodeCommunity["b"] {
		t.Error("connected nodes should be in the same community")
	}
}

func TestLeiden_TwoClusters(t *testing.T) {
	// Two dense clusters connected by a single weak edge.
	nodes := []string{"a1", "a2", "a3", "b1", "b2", "b3"}
	edges := []LeidenEdge{
		// Cluster A: dense
		{Source: "a1", Target: "a2", Weight: 1.0},
		{Source: "a2", Target: "a3", Weight: 1.0},
		{Source: "a1", Target: "a3", Weight: 1.0},
		// Cluster B: dense
		{Source: "b1", Target: "b2", Weight: 1.0},
		{Source: "b2", Target: "b3", Weight: 1.0},
		{Source: "b1", Target: "b3", Weight: 1.0},
		// Weak bridge
		{Source: "a3", Target: "b1", Weight: 0.1},
	}

	g := NewLeidenGraph(nodes, edges)
	cfg := DefaultLeidenConfig()
	cfg.Seed = 42
	cfg.MinCommunitySize = 1
	result := RunLeiden(g, cfg)

	// Should detect 2 communities.
	if len(result.Communities) < 2 {
		t.Errorf("expected at least 2 communities, got %d", len(result.Communities))
	}

	// All A nodes should be in the same community.
	if result.NodeCommunity["a1"] != result.NodeCommunity["a2"] ||
		result.NodeCommunity["a2"] != result.NodeCommunity["a3"] {
		t.Error("cluster A nodes should be in the same community")
	}

	// All B nodes should be in the same community.
	if result.NodeCommunity["b1"] != result.NodeCommunity["b2"] ||
		result.NodeCommunity["b2"] != result.NodeCommunity["b3"] {
		t.Error("cluster B nodes should be in the same community")
	}

	// A and B should be in different communities.
	if result.NodeCommunity["a1"] == result.NodeCommunity["b1"] {
		t.Error("clusters A and B should be in different communities")
	}

	// Modularity should be positive for a good partition.
	if result.Modularity <= 0 {
		t.Errorf("expected positive modularity, got %f", result.Modularity)
	}
}

func TestLeiden_LargerGraph(t *testing.T) {
	// 3 clusters of 10 nodes each, with intra-cluster density 0.8 and
	// inter-cluster connections at 0.05 weight.
	var nodes []string
	var edges []LeidenEdge

	for cluster := 0; cluster < 3; cluster++ {
		for i := 0; i < 10; i++ {
			nodes = append(nodes, fmt.Sprintf("c%d_n%d", cluster, i))
		}
	}

	// Dense intra-cluster edges.
	for cluster := 0; cluster < 3; cluster++ {
		for i := 0; i < 10; i++ {
			for j := i + 1; j < 10; j++ {
				edges = append(edges, LeidenEdge{
					Source: fmt.Sprintf("c%d_n%d", cluster, i),
					Target: fmt.Sprintf("c%d_n%d", cluster, j),
					Weight: 0.8,
				})
			}
		}
	}

	// Sparse inter-cluster edges.
	for i := 0; i < 3; i++ {
		edges = append(edges, LeidenEdge{
			Source: fmt.Sprintf("c%d_n0", i),
			Target: fmt.Sprintf("c%d_n0", (i+1)%3),
			Weight: 0.05,
		})
	}

	g := NewLeidenGraph(nodes, edges)
	cfg := DefaultLeidenConfig()
	cfg.Seed = 42
	result := RunLeiden(g, cfg)

	if len(result.Communities) < 3 {
		t.Errorf("expected at least 3 communities for 3 dense clusters, got %d", len(result.Communities))
	}

	// Check that nodes within each intended cluster mostly share the same community.
	for cluster := 0; cluster < 3; cluster++ {
		commCounts := make(map[int]int)
		for i := 0; i < 10; i++ {
			nid := fmt.Sprintf("c%d_n%d", cluster, i)
			commCounts[result.NodeCommunity[nid]]++
		}
		// The dominant community should have at least 8 of 10 nodes.
		maxCount := 0
		for _, c := range commCounts {
			if c > maxCount {
				maxCount = c
			}
		}
		if maxCount < 8 {
			t.Errorf("cluster %d: expected at least 8/10 nodes in dominant community, got %d", cluster, maxCount)
		}
	}
}

func TestCommunityKeywords(t *testing.T) {
	texts := []string{
		"machine learning deep neural networks",
		"deep learning training neural networks",
		"artificial intelligence machine learning models",
	}

	keywords := CommunityKeywords(texts, 3)
	if len(keywords) == 0 {
		t.Fatal("expected keywords, got none")
	}
	if len(keywords) > 3 {
		t.Errorf("expected at most 3 keywords, got %d", len(keywords))
	}
}

func TestCohesionScore(t *testing.T) {
	// Fully connected triangle: cohesion should be 1.0.
	g := NewLeidenGraph(
		[]string{"a", "b", "c"},
		[]LeidenEdge{
			{Source: "a", Target: "b", Weight: 1.0},
			{Source: "b", Target: "c", Weight: 1.0},
			{Source: "a", Target: "c", Weight: 1.0},
		},
	)

	score := CohesionScore(g, []string{"a", "b", "c"})
	if score != 1.0 {
		t.Errorf("expected cohesion 1.0 for complete triangle, got %f", score)
	}

	// Only one edge out of 3 possible: cohesion ~0.33.
	g2 := NewLeidenGraph(
		[]string{"a", "b", "c"},
		[]LeidenEdge{
			{Source: "a", Target: "b", Weight: 1.0},
		},
	)

	score2 := CohesionScore(g2, []string{"a", "b", "c"})
	if score2 < 0.3 || score2 > 0.4 {
		t.Errorf("expected cohesion ~0.33 for single-edge triangle, got %f", score2)
	}
}
