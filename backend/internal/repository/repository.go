/**
 * =============================================================================
 * Repository Package - Data Access Layer and Clean Architecture Interface
 * =============================================================================
 * 
 * 📚 EDUCATIONAL OVERVIEW:
 * This package defines the repository layer interface for the Brain2 memory
 * management system. It demonstrates repository pattern implementation,
 * clean architecture principles, and database abstraction in Go.
 * 
 * 🏗️ KEY REPOSITORY PATTERN CONCEPTS:
 * 
 * 1. CLEAN ARCHITECTURE PRINCIPLES:
 *    - Repository interface defined in domain layer (dependency inversion)
 *    - Abstracts storage implementation details from business logic
 *    - Enables testing with mock implementations
 *    - Supports multiple storage backends (DynamoDB, PostgreSQL, etc.)
 * 
 * 2. REPOSITORY PATTERN BENEFITS:
 *    - Encapsulates data access logic and complexity
 *    - Provides domain-centric data operations
 *    - Centralizes query logic and optimization
 *    - Enables consistent error handling across storage operations
 * 
 * 3. DOMAIN-DRIVEN DESIGN INTEGRATION:
 *    - Operations match domain concepts and use cases
 *    - Query objects represent business query requirements
 *    - Error types reflect domain-level error conditions
 *    - Interface supports aggregate boundaries and consistency
 * 
 * 4. STORAGE ABSTRACTION:
 *    - No database-specific types exposed in interface
 *    - Context support for cancellation and timeout
 *    - Consistent error handling across different backends
 *    - Performance optimization through query objects
 * 
 * 🎯 LEARNING OBJECTIVES:
 * - Repository pattern implementation in Go
 * - Clean architecture data access layer design
 * - Database abstraction and interface design
 * - Domain-driven repository design
 * - Query object patterns and optimization
 */
package repository

import (
	"brain2-backend/internal/domain" // Domain entities and aggregates
	"context"                        // Request lifecycle and cancellation
)

/**
 * =============================================================================
 * Repository Interface - Data Access Contract for Memory Management
 * =============================================================================
 * 
 * REPOSITORY PATTERN IMPLEMENTATION:
 * This interface defines the complete data access contract for the Brain2
 * memory management system. It abstracts storage operations and provides
 * a clean boundary between business logic and data persistence.
 * 
 * INTERFACE DESIGN PRINCIPLES:
 * 
 * 1. DOMAIN-CENTRIC OPERATIONS:
 *    - Methods reflect business use cases, not database operations
 *    - Parameters and return types use domain entities
 *    - Operation names express business intent
 *    - Supports domain aggregate patterns
 * 
 * 2. CONTEXT-AWARE DESIGN:
 *    - All methods accept context.Context for cancellation
 *    - Enables timeout handling and request tracing
 *    - Supports transaction management in implementations
 *    - Consistent with Go best practices
 * 
 * 3. QUERY ABSTRACTION:
 *    - Query objects encapsulate complex query parameters
 *    - Type-safe query building and validation
 *    - Optimized for different access patterns
 *    - Extensible for future query requirements
 * 
 * 4. ERROR HANDLING:
 *    - Custom error types for repository-specific conditions
 *    - Structured errors with context and details
 *    - Supports proper HTTP status code mapping
 *    - Consistent error semantics across implementations
 * 
 * IMPLEMENTATION STRATEGIES:
 * - DynamoDB implementation for serverless scalability
 * - PostgreSQL implementation for complex queries
 * - In-memory implementation for testing
 * - Composite implementation for caching strategies
 */
type Repository interface {
	
	// ==========================================================================
	// EVENT-DRIVEN OPERATIONS - Asynchronous Processing Support
	// ==========================================================================
	//
	// These operations support the event-driven architecture where memory
	// creation and relationship discovery happen asynchronously for better
	// performance and user experience.
	
	// CreateNodeAndKeywords stores a memory node with extracted keywords
	// EVENT-DRIVEN USAGE:
	// - Called by background event processor after keyword extraction
	// - Node already contains computed keywords and metadata
	// - Optimized for bulk processing and high throughput
	// - Supports eventual consistency patterns
	CreateNodeAndKeywords(ctx context.Context, node domain.Node) error
	
	// CreateEdges establishes relationships between memories
	// RELATIONSHIP MANAGEMENT:
	// - Creates bidirectional edges between related memories
	// - Supports bulk edge creation for efficiency
	// - Maintains referential integrity
	// - Enables graph topology updates
	CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error
	
	// ==========================================================================
	// SYNCHRONOUS OPERATIONS - Direct Processing Support
	// ==========================================================================
	//
	// These operations provide immediate processing for use cases requiring
	// instant feedback and consistency. May be deprecated in favor of
	// event-driven alternatives for better scalability.
	
	// CreateNodeWithEdges creates memory and relationships in single operation
	// ATOMIC OPERATION:
	// - Creates node and discovers/creates relationships atomically
	// - Provides immediate consistency for user feedback
	// - Higher latency but simpler error handling
	// - Suitable for small-scale or development environments
	CreateNodeWithEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
	
	// UpdateNodeAndEdges modifies memory and recalculates relationships
	// CONSISTENCY MANAGEMENT:
	// - Updates node content and recomputes all relationships
	// - Removes outdated edges and creates new ones
	// - Maintains graph consistency during updates
	// - Supports optimistic concurrency control
	UpdateNodeAndEdges(ctx context.Context, node domain.Node, relatedNodeIDs []string) error
	
	// DeleteNode removes memory and cleans up all relationships
	// CLEANUP OPERATION:
	// - Deletes node and all associated edges
	// - Maintains referential integrity
	// - Supports cascade deletion patterns
	// - Handles orphaned relationship cleanup
	DeleteNode(ctx context.Context, userID, nodeID string) error
	
	// ==========================================================================
	// QUERY OPERATIONS - Data Retrieval and Search
	// ==========================================================================
	//
	// These operations provide flexible data retrieval capabilities optimized
	// for different access patterns and use cases in the memory management system.
	
	// FindNodeByID retrieves a specific memory by its unique identifier
	// SINGLE-ENTITY RETRIEVAL:
	// - Optimized for primary key lookups
	// - Supports multi-tenant isolation
	// - Returns nil if not found (not an error condition)
	// - Used for ownership verification and detail views
	FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)
	
	// FindNodes searches for memories using flexible query criteria
	// FLEXIBLE QUERY SUPPORT:
	// - Keyword-based search for content discovery
	// - Pagination support for large result sets
	// - Multiple filter criteria combination
	// - Optimized for search and recommendation use cases
	FindNodes(ctx context.Context, query NodeQuery) ([]domain.Node, error)
	
	// FindEdges searches for relationships using query criteria
	// RELATIONSHIP DISCOVERY:
	// - Find connections for specific nodes
	// - Support directional relationship queries
	// - Enables graph traversal algorithms
	// - Optimized for visualization and analysis
	FindEdges(ctx context.Context, query EdgeQuery) ([]domain.Edge, error)
	
	// GetGraphData retrieves complete or filtered graph structure
	// GRAPH VISUALIZATION SUPPORT:
	// - Returns nodes and edges for graph rendering
	// - Supports subgraph extraction and filtering
	// - Optimized for visualization libraries
	// - Enables network analysis and exploration
	GetGraphData(ctx context.Context, query GraphQuery) (*domain.Graph, error)
	
	// FindNodesByKeywords searches memories by keyword matching
	// KEYWORD-BASED DISCOVERY:
	// - Efficient keyword-based search implementation
	// - Supports automatic relationship discovery
	// - Optimized for content-based recommendations
	// - Foundation for semantic search capabilities
	FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]domain.Node, error)
}
