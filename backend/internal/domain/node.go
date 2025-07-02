/**
 * =============================================================================
 * Domain Package - Core Business Entities and Domain-Driven Design
 * =============================================================================
 * 
 * 📚 EDUCATIONAL OVERVIEW:
 * This package contains the core domain entities for the Brain2 memory management
 * system. It demonstrates domain-driven design principles, clean architecture
 * patterns, and rich domain modeling in Go.
 * 
 * 🏗️ KEY DOMAIN-DRIVEN DESIGN CONCEPTS:
 * 
 * 1. DOMAIN ENTITIES:
 *    - Rich objects with identity, behavior, and business rules
 *    - Independent of infrastructure concerns (databases, APIs)
 *    - Express the ubiquitous language of the business domain
 *    - Encapsulate business logic and invariants
 * 
 * 2. CLEAN ARCHITECTURE PRINCIPLES:
 *    - Domain layer is the center of the application
 *    - No dependencies on external frameworks or infrastructure
 *    - Pure business logic without technical concerns
 *    - Stable interfaces that change only when business rules change
 * 
 * 3. INFRASTRUCTURE INDEPENDENCE:
 *    - No database-specific annotations or ORM dependencies
 *    - JSON tags only for serialization, not persistence
 *    - Can be persisted in any storage system
 *    - Business rules enforced at the domain level
 * 
 * 🎯 LEARNING OBJECTIVES:
 * - Domain-driven design entity modeling
 * - Clean architecture domain layer design
 * - Infrastructure-independent business logic
 * - Rich domain model patterns in Go
 * - Entity identity and lifecycle management
 */
package domain

import "time" // Standard library for temporal domain concepts

/**
 * =============================================================================
 * Node Entity - Memory/Thought Representation in Knowledge Graph
 * =============================================================================
 * 
 * DOMAIN CONCEPT:
 * A Node represents a single memory, thought, idea, or piece of knowledge
 * in a user's personal knowledge graph. It's the fundamental building block
 * of the Brain2 memory management system.
 * 
 * ENTITY CHARACTERISTICS:
 * 
 * 1. IDENTITY:
 *    - Has a unique identifier (ID) that persists throughout its lifecycle
 *    - Identity remains constant even when content changes
 *    - Enables tracking and referencing across the system
 * 
 * 2. RICH BEHAVIOR:
 *    - Contains both data (content) and metadata (keywords, timestamps)
 *    - Supports keyword extraction for intelligent connection discovery
 *    - Versioning for optimistic concurrency control
 * 
 * 3. BUSINESS RULES:
 *    - Must belong to a specific user (multi-tenant isolation)
 *    - Content cannot be empty (business invariant)
 *    - Keywords enable automatic relationship discovery
 *    - Creation timestamp for temporal organization
 * 
 * 4. GRAPH PARTICIPATION:
 *    - Nodes connect to other nodes via edges
 *    - Forms a knowledge graph of interconnected memories
 *    - Supports traversal and discovery algorithms
 * 
 * KNOWLEDGE GRAPH THEORY:
 * In graph theory, nodes (vertices) are entities connected by edges (relationships).
 * Brain2 implements a personal knowledge graph where:
 * - Nodes = memories/thoughts/ideas
 * - Edges = relationships between memories
 * - Graph = complete user knowledge network
 * 
 * FUTURE ENHANCEMENTS:
 * - Tags for categorical organization
 * - Importance/priority scoring
 * - Last accessed timestamp
 * - Content type classification
 * - Embedding vectors for semantic search
 */
type Node struct {
	// ==========================================================================
	// ENTITY IDENTITY AND OWNERSHIP
	// ==========================================================================
	
	// ID is the unique identifier for this memory node
	// CHARACTERISTICS:
	// - UUID format for global uniqueness
	// - Immutable throughout entity lifecycle
	// - Used for references and relationships
	// - Enables distributed system coordination
	ID string `json:"id"`
	
	// UserID identifies the owner of this memory
	// MULTI-TENANCY:
	// - Ensures data isolation between users
	// - Enables user-specific operations and queries
	// - Required for all business operations
	// - Links to authentication system user identity
	UserID string `json:"user_id"`
	
	// ==========================================================================
	// CORE BUSINESS DATA
	// ==========================================================================
	
	// Content is the actual memory, thought, or knowledge
	// BUSINESS RULES:
	// - Cannot be empty (domain invariant)
	// - Free-form text for maximum flexibility
	// - Source for automatic keyword extraction
	// - Primary searchable content
	Content string `json:"content"`
	
	// Keywords are automatically extracted meaningful terms
	// KNOWLEDGE DISCOVERY:
	// - Enable automatic connection discovery between nodes
	// - Support fast keyword-based search operations
	// - Generated via natural language processing
	// - Foundation for recommendation algorithms
	Keywords []string `json:"keywords"`
	
	// ==========================================================================
	// TEMPORAL AND VERSIONING METADATA
	// ==========================================================================
	
	// CreatedAt timestamp for temporal organization
	// TEMPORAL FEATURES:
	// - Enables chronological organization of memories
	// - Supports time-based queries and filtering
	// - Audit trail for memory creation
	// - Basis for temporal analysis and insights
	CreatedAt time.Time `json:"created_at"`
	
	// Version number for optimistic concurrency control
	// CONCURRENCY CONTROL:
	// - Prevents lost updates in concurrent environments
	// - Enables conflict detection and resolution
	// - Supports versioning and change tracking
	// - Essential for distributed system consistency
	Version int `json:"version"`
}
