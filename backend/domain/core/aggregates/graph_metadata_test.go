package aggregates

import (
	"fmt"
	"testing"

	"backend/domain/core/entities"
	"github.com/stretchr/testify/assert"
)

func TestGraphMetadata_Initialization(t *testing.T) {
	metadata := GraphMetadata{
		NodeCount: 10,
		EdgeCount: 15,
		MaxDepth:  5,
		IsPublic:  true,
		Tags:      []string{"knowledge", "graph", "test"},
		ViewSettings: ViewSettings{
			Layout:     LayoutForceDirected,
			Theme:      "dark",
			NodeSize:   "medium",
			EdgeStyle:  "curved",
			ShowLabels: true,
		},
	}

	assert.Equal(t, 10, metadata.NodeCount)
	assert.Equal(t, 15, metadata.EdgeCount)
	assert.Equal(t, 5, metadata.MaxDepth)
	assert.True(t, metadata.IsPublic)
	assert.Len(t, metadata.Tags, 3)
	assert.Contains(t, metadata.Tags, "knowledge")
}

func TestViewSettings_Defaults(t *testing.T) {
	tests := []struct {
		name     string
		settings ViewSettings
		expected ViewSettings
	}{
		{
			name: "force directed layout",
			settings: ViewSettings{
				Layout:     LayoutForceDirected,
				ShowLabels: true,
			},
			expected: ViewSettings{
				Layout:     LayoutForceDirected,
				ShowLabels: true,
			},
		},
		{
			name: "hierarchical layout",
			settings: ViewSettings{
				Layout:     LayoutHierarchical,
				Theme:      "light",
				ShowLabels: false,
			},
			expected: ViewSettings{
				Layout:     LayoutHierarchical,
				Theme:      "light",
				ShowLabels: false,
			},
		},
		{
			name: "circular layout",
			settings: ViewSettings{
				Layout:     LayoutCircular,
				NodeSize:   "large",
				EdgeStyle:  "straight",
				ShowLabels: true,
			},
			expected: ViewSettings{
				Layout:     LayoutCircular,
				NodeSize:   "large",
				EdgeStyle:  "straight",
				ShowLabels: true,
			},
		},
		{
			name: "grid layout",
			settings: ViewSettings{
				Layout:     LayoutGrid,
				Theme:      "custom",
				NodeSize:   "small",
				ShowLabels: false,
			},
			expected: ViewSettings{
				Layout:     LayoutGrid,
				Theme:      "custom",
				NodeSize:   "small",
				ShowLabels: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected.Layout, tt.settings.Layout)
			assert.Equal(t, tt.expected.Theme, tt.settings.Theme)
			assert.Equal(t, tt.expected.NodeSize, tt.settings.NodeSize)
			assert.Equal(t, tt.expected.EdgeStyle, tt.settings.EdgeStyle)
			assert.Equal(t, tt.expected.ShowLabels, tt.settings.ShowLabels)
		})
	}
}

func TestLayoutType_Validation(t *testing.T) {
	validLayouts := []LayoutType{
		LayoutForceDirected,
		LayoutHierarchical,
		LayoutCircular,
		LayoutGrid,
	}

	for _, layout := range validLayouts {
		assert.NotEmpty(t, string(layout))
	}

	// Test that layout types are distinct
	layoutMap := make(map[LayoutType]bool)
	for _, layout := range validLayouts {
		assert.False(t, layoutMap[layout], "Duplicate layout type: %s", layout)
		layoutMap[layout] = true
	}
}

func TestGraphMetadata_Update(t *testing.T) {
	graph := createTestGraph(t)

	// Initial metadata should be empty
	// Check initial counts
	assert.Equal(t, 0, graph.NodeCount())
	assert.Equal(t, 0, graph.EdgeCount())

	// Add nodes
	node1 := createTestNode(t, "Node 1")
	node2 := createTestNode(t, "Node 2")
	node3 := createTestNode(t, "Node 3")

	err := graph.AddNode(node1)
	assert.NoError(t, err)
	err = graph.AddNode(node2)
	assert.NoError(t, err)
	err = graph.AddNode(node3)
	assert.NoError(t, err)

	// Connect nodes
	_, err = graph.ConnectNodes(node1.ID(), node2.ID(), entities.EdgeTypeNormal)
	assert.NoError(t, err)
	_, err = graph.ConnectNodes(node2.ID(), node3.ID(), entities.EdgeTypeNormal)
	assert.NoError(t, err)

	// Check updated counts
	assert.Equal(t, 3, graph.NodeCount())
	assert.Equal(t, 2, graph.EdgeCount())
}

func TestGraphMetadata_Tags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected int
	}{
		{
			name:     "no tags",
			tags:     []string{},
			expected: 0,
		},
		{
			name:     "single tag",
			tags:     []string{"research"},
			expected: 1,
		},
		{
			name:     "multiple tags",
			tags:     []string{"research", "science", "biology", "chemistry"},
			expected: 4,
		},
		{
			name:     "duplicate tags",
			tags:     []string{"research", "research", "science"},
			expected: 3, // Duplicates allowed in slice
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := GraphMetadata{
				Tags: tt.tags,
			}
			assert.Len(t, metadata.Tags, tt.expected)
		})
	}
}

func TestGraphMetadata_Visibility(t *testing.T) {
	tests := []struct {
		name     string
		isPublic bool
	}{
		{
			name:     "public graph",
			isPublic: true,
		},
		{
			name:     "private graph",
			isPublic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := GraphMetadata{
				IsPublic: tt.isPublic,
			}
			assert.Equal(t, tt.isPublic, metadata.IsPublic)
		})
	}
}

func TestGraphMetadata_ComplexScenario(t *testing.T) {
	graph := createTestGraph(t)

	// Create a complex graph structure
	var nodes []*entities.Node
	for i := 0; i < 10; i++ {
		node := createTestNode(t, fmt.Sprintf("Node %d", i))
		nodes = append(nodes, node)
		err := graph.AddNode(node)
		assert.NoError(t, err)
	}

	// Create a hub-and-spoke pattern
	hub := nodes[0]
	for i := 1; i < 10; i++ {
		_, err := graph.ConnectNodes(hub.ID(), nodes[i].ID(), entities.EdgeTypeNormal)
		assert.NoError(t, err)
	}

	// Create some additional connections
	_, err := graph.ConnectNodes(nodes[1].ID(), nodes[2].ID(), entities.EdgeTypeNormal)
	assert.NoError(t, err)
	_, err = graph.ConnectNodes(nodes[3].ID(), nodes[4].ID(), entities.EdgeTypeStrong)
	assert.NoError(t, err)

	// Verify counts
	assert.Equal(t, 10, graph.NodeCount())
	assert.Equal(t, 11, graph.EdgeCount()) // 9 from hub + 2 additional

	// Test metadata map
	metadata := graph.Metadata()
	assert.NotNil(t, metadata)
}

func TestViewSettings_ThemeVariations(t *testing.T) {
	themes := []string{"light", "dark", "auto", "custom", ""}

	for _, theme := range themes {
		settings := ViewSettings{
			Theme: theme,
		}
		assert.Equal(t, theme, settings.Theme)
	}
}

func TestViewSettings_NodeSizeVariations(t *testing.T) {
	sizes := []string{"small", "medium", "large", "auto", ""}

	for _, size := range sizes {
		settings := ViewSettings{
			NodeSize: size,
		}
		assert.Equal(t, size, settings.NodeSize)
	}
}

func TestViewSettings_EdgeStyleVariations(t *testing.T) {
	styles := []string{"straight", "curved", "orthogonal", "bezier", ""}

	for _, style := range styles {
		settings := ViewSettings{
			EdgeStyle: style,
		}
		assert.Equal(t, style, settings.EdgeStyle)
	}
}

// Benchmarks

func BenchmarkGraphMetadata_Update(b *testing.B) {
	graph := createTestGraph(&testing.T{})

	// Add many nodes
	for i := 0; i < 100; i++ {
		node := createTestNode(&testing.T{}, fmt.Sprintf("Node %d", i))
		_ = graph.AddNode(node)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Just check the metadata
		_ = graph.Metadata()
	}
}

func BenchmarkGraphMetadata_TagOperations(b *testing.B) {
	tags := []string{"tag1", "tag2", "tag3", "tag4", "tag5"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metadata := GraphMetadata{
			Tags: tags,
		}
		_ = len(metadata.Tags)
	}
}

// Helper functions are defined in graph_test.go