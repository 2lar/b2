package services

import (
	"strings"
	"testing"
)

func TestAnalyze_IsolatedNode(t *testing.T) {
	tg := newTestGraph(t, []string{"a"})
	svc := NewImpactAnalysisService()

	result, err := svc.Analyze(tg.graph, tg.ids["a"], tg.nodes, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalAffectedNodes != 0 {
		t.Errorf("expected 0 affected nodes, got %d", result.TotalAffectedNodes)
	}
	if result.RiskLevel != RiskLow {
		t.Errorf("expected LOW risk, got %s", result.RiskLevel)
	}
	if !strings.Contains(result.Summary, "safely removed") {
		t.Errorf("expected 'safely removed' in summary, got: %s", result.Summary)
	}
}

func TestAnalyze_SingleConnection(t *testing.T) {
	tg := newTestGraph(t, []string{"a", "b"})
	tg.addEdge(t, "a", "b", 1.0)
	svc := NewImpactAnalysisService()

	result, err := svc.Analyze(tg.graph, tg.ids["a"], tg.nodes, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalAffectedNodes != 1 {
		t.Errorf("expected 1 affected node, got %d", result.TotalAffectedNodes)
	}

	// Direct connection should be WILL_BREAK
	foundWillBreak := false
	for _, g := range result.Dependents {
		if g.Tier == TierWillBreak {
			foundWillBreak = true
			if len(g.NodeIDs) != 1 {
				t.Errorf("expected 1 WILL_BREAK node, got %d", len(g.NodeIDs))
			}
		}
	}
	if !foundWillBreak {
		t.Error("expected a WILL_BREAK dependency group")
	}
}

func TestAnalyze_HubNode(t *testing.T) {
	// Hub with 5 connections → should be HIGH or CRITICAL
	names := []string{"hub", "a", "b", "c", "d", "e"}
	tg := newTestGraph(t, names)
	for _, leaf := range names[1:] {
		tg.addEdge(t, "hub", leaf, 1.0)
	}

	svc := NewImpactAnalysisService()
	result, err := svc.Analyze(tg.graph, tg.ids["hub"], tg.nodes, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RiskLevel != RiskHigh && result.RiskLevel != RiskCritical {
		t.Errorf("expected HIGH or CRITICAL risk for 5-connection hub, got %s", result.RiskLevel)
	}
	if result.TotalAffectedNodes != 5 {
		t.Errorf("expected 5 affected nodes, got %d", result.TotalAffectedNodes)
	}
}

func TestAnalyze_BridgeNode(t *testing.T) {
	// Bridge node connecting two different communities
	tg := newTestGraph(t, []string{"bridge", "a1", "a2", "a3", "b1", "b2", "b3"})
	// Community A connections
	tg.addEdge(t, "bridge", "a1", 1.0)
	tg.addEdge(t, "bridge", "a2", 1.0)
	tg.addEdge(t, "bridge", "a3", 1.0)
	// Community B connections
	tg.addEdge(t, "bridge", "b1", 1.0)
	tg.addEdge(t, "bridge", "b2", 1.0)
	tg.addEdge(t, "bridge", "b3", 1.0)

	tg.setCommunity("bridge", "commA")
	tg.setCommunity("a1", "commA")
	tg.setCommunity("a2", "commA")
	tg.setCommunity("a3", "commA")
	tg.setCommunity("b1", "commB")
	tg.setCommunity("b2", "commB")
	tg.setCommunity("b3", "commB")

	svc := NewImpactAnalysisService()
	result, err := svc.Analyze(tg.graph, tg.ids["bridge"], tg.nodes, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RiskLevel != RiskCritical {
		t.Errorf("expected CRITICAL risk for bridge node with 6 connections across communities, got %s", result.RiskLevel)
	}
	if !strings.Contains(result.Summary, "bridge") {
		t.Errorf("expected 'bridge' in summary, got: %s", result.Summary)
	}
	if result.AffectedCommunityCount < 2 {
		t.Errorf("expected at least 2 affected communities, got %d", result.AffectedCommunityCount)
	}
}

func TestAnalyze_DepthLimit(t *testing.T) {
	// Chain: target→a→b→c→d
	tg := newTestGraph(t, []string{"target", "a", "b", "c", "d"})
	tg.addEdge(t, "target", "a", 1.0)
	tg.addEdge(t, "a", "b", 1.0)
	tg.addEdge(t, "b", "c", 1.0)
	tg.addEdge(t, "c", "d", 1.0)

	svc := NewImpactAnalysisService()

	// maxDepth=2: should see a (depth 1) and b (depth 2), not c or d
	result, err := svc.Analyze(tg.graph, tg.ids["target"], tg.nodes, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalAffectedNodes != 2 {
		t.Errorf("expected 2 affected nodes at maxDepth=2, got %d", result.TotalAffectedNodes)
	}
}

func TestAnalyze_TierClassification(t *testing.T) {
	// target→a (depth 1, WILL_BREAK), a→b (depth 2, same community → LIKELY_AFFECTED)
	tg := newTestGraph(t, []string{"target", "a", "b"})
	tg.addEdge(t, "target", "a", 1.0)
	tg.addEdge(t, "a", "b", 1.0)
	tg.setCommunity("target", "comm1")
	tg.setCommunity("a", "comm1")
	tg.setCommunity("b", "comm1")

	svc := NewImpactAnalysisService()
	result, err := svc.Analyze(tg.graph, tg.ids["target"], tg.nodes, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tierCounts := make(map[ImpactTier]int)
	for _, g := range result.Dependents {
		tierCounts[g.Tier] += len(g.NodeIDs)
	}

	if tierCounts[TierWillBreak] != 1 {
		t.Errorf("expected 1 WILL_BREAK, got %d", tierCounts[TierWillBreak])
	}
	if tierCounts[TierLikelyAffected] != 1 {
		t.Errorf("expected 1 LIKELY_AFFECTED (same community, depth 2), got %d", tierCounts[TierLikelyAffected])
	}
}
