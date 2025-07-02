/**
 * =============================================================================
 * Edge Entity - Relationship Representation in Knowledge Graph
 * =============================================================================
 * 
 * 📚 EDUCATIONAL OVERVIEW:
 * The Edge entity represents connections and relationships between memory nodes
 * in the Brain2 knowledge graph. It demonstrates graph theory concepts,
 * relationship modeling, and network analysis foundations.
 * 
 * 🏗️ KEY GRAPH THEORY CONCEPTS:
 * 
 * 1. GRAPH RELATIONSHIPS:
 *    - Edges connect two nodes (vertices) in a graph structure
 *    - Define the topology and connectivity of the knowledge network
 *    - Enable traversal algorithms and path finding
 *    - Foundation for recommendation and discovery systems
 * 
 * 2. DIRECTED GRAPH IMPLEMENTATION:
 *    - Source → Target relationship directionality
 *    - Asymmetric relationships (A connects to B ≠ B connects to A)
 *    - Supports complex relationship modeling
 *    - Enables graph algorithms like shortest path, centrality
 * 
 * 3. KNOWLEDGE DISCOVERY:
 *    - Automatic relationship creation based on keyword similarity
 *    - Semantic connections between related memories
 *    - Network effects for knowledge organization
 *    - Basis for intelligent memory recommendations
 * 
 * 🔗 RELATIONSHIP SEMANTICS:
 * 
 * CURRENT IMPLEMENTATION:
 * - Bidirectional semantic relationships (if A→B exists, B→A typically exists)
 * - Keyword-based connection discovery
 * - Equal-weight relationships (no strength/confidence scoring)
 * - Automatic creation during memory processing
 * 
 * FUTURE ENHANCEMENTS:
 * - Relationship types (similar, contradicts, builds-on, etc.)
 * - Relationship strength/confidence scoring
 * - Temporal relationships (before/after, causes/effects)
 * - User-defined explicit relationships
 * - Relationship metadata (creation source, confidence)
 * 
 * 🎯 LEARNING OBJECTIVES:
 * - Graph theory and network modeling
 * - Relationship representation in domain models
 * - Knowledge graph construction principles
 * - Network analysis foundations
 * - Semantic relationship modeling
 */
package domain

/**
 * =============================================================================
 * Edge Entity - Graph Relationship Between Memory Nodes
 * =============================================================================
 * 
 * DOMAIN CONCEPT:
 * An Edge represents a directed relationship between two memory nodes in a
 * user's knowledge graph. It captures semantic connections that enable
 * knowledge discovery and intelligent memory organization.
 * 
 * RELATIONSHIP CHARACTERISTICS:
 * 
 * 1. DIRECTIONALITY:
 *    - Source node points to target node
 *    - Supports asymmetric relationships if needed
 *    - Currently used for bidirectional semantic similarity
 *    - Foundation for future relationship type expansion
 * 
 * 2. SIMPLICITY:
 *    - Minimal data model focuses on core relationship
 *    - No relationship metadata in current version
 *    - Extensible design for future enhancements
 *    - Clean separation from relationship discovery logic
 * 
 * 3. DISCOVERY AUTOMATION:
 *    - Created automatically during memory processing
 *    - Based on keyword overlap and semantic similarity
 *    - No manual relationship management required
 *    - Intelligent connection suggestions for users
 * 
 * GRAPH OPERATIONS ENABLED:
 * - Find all memories connected to a specific memory
 * - Traverse relationship networks for discovery
 * - Analyze connection patterns and clusters
 * - Recommend related memories to users
 * - Visualize knowledge network topology
 * 
 * STORAGE CONSIDERATIONS:
 * - Lightweight structure for efficient storage
 * - Bidirectional relationships stored as two edges
 * - No self-referential edges (nodes don't connect to themselves)
 * - Supports graph visualization libraries
 */
type Edge struct {
	// ==========================================================================
	// RELATIONSHIP ENDPOINTS
	// ==========================================================================
	
	// SourceID identifies the starting node of the relationship
	// RELATIONSHIP SEMANTICS:
	// - The memory node that "points to" or "relates to" another
	// - In bidirectional relationships, arbitrary assignment
	// - Used for graph traversal and query operations
	// - Must reference an existing Node.ID
	SourceID string `json:"source_id"`
	
	// TargetID identifies the ending node of the relationship
	// RELATIONSHIP SEMANTICS:
	// - The memory node that is "pointed to" or "related from" another
	// - In bidirectional relationships, arbitrary assignment
	// - Used for graph traversal and query operations
	// - Must reference an existing Node.ID
	TargetID string `json:"target_id"`
	
	// ==========================================================================
	// DESIGN NOTES FOR FUTURE ENHANCEMENTS:
	// ==========================================================================
	//
	// POTENTIAL ADDITIONAL FIELDS:
	// - RelationshipType: string (semantic similarity, temporal, causal, etc.)
	// - Strength: float64 (0.0-1.0 confidence score)
	// - CreatedAt: time.Time (when relationship was discovered)
	// - CreatedBy: string (system auto-discovery vs user-defined)
	// - Metadata: map[string]interface{} (extensible relationship data)
	// - UserID: string (for multi-tenant relationship isolation)
	//
	// GRAPH ALGORITHMS SUPPORTABLE:
	// - Shortest path between memories
	// - Community detection (memory clusters)
	// - Centrality analysis (most connected memories)
	// - Recommendation algorithms
	// - Knowledge graph embeddings
}
