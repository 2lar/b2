/**
 * Memory Service Package - Core Business Logic Layer
 * 
 * This package implements the heart of Brain2's memory management system.
 * It orchestrates complex operations involving memory nodes, keyword extraction,
 * and connection discovery using clean architecture principles.
 * 
 * KEY ARCHITECTURAL CONCEPTS:
 * 
 * 1. SERVICE LAYER PATTERN:
 *    - Encapsulates business logic independent of infrastructure
 *    - Coordinates between domain entities and repository layer
 *    - Handles complex workflows that span multiple operations
 *    - Provides transaction boundaries for data consistency
 * 
 * 2. DOMAIN-DRIVEN DESIGN:
 *    - Rich domain models with behavior, not just data
 *    - Business rules enforced at the service layer
 *    - Ubiquitous language shared between code and business
 *    - Separation of concerns between layers
 * 
 * 3. DEPENDENCY INVERSION:
 *    - Service depends on repository interface, not implementation
 *    - Enables testing with mock repositories
 *    - Supports different storage backends (DynamoDB, PostgreSQL, etc.)
 *    - Infrastructure details hidden from business logic
 * 
 * 4. ERROR HANDLING STRATEGY:
 *    - Structured error types with context
 *    - Graceful degradation for non-critical failures
 *    - Detailed logging for debugging and monitoring
 *    - User-friendly error messages for API responses
 * 
 * CORE BUSINESS OPERATIONS:
 * 
 * 1. MEMORY LIFECYCLE:
 *    - Create new memories with automatic keyword extraction
 *    - Update existing memories and recalculate connections
 *    - Delete memories with proper cleanup
 *    - Bulk operations for efficiency
 * 
 * 2. CONNECTION INTELLIGENCE:
 *    - Automatic discovery of related memories via keywords
 *    - Real-time graph updates as memories change
 *    - Semantic similarity algorithms (future enhancement)
 *    - Connection strength calculation
 * 
 * 3. KNOWLEDGE GRAPH MANAGEMENT:
 *    - Maintain consistency of node-edge relationships
 *    - Optimize for fast retrieval and visualization
 *    - Support for graph algorithms and analysis
 *    - Scalable architecture for large knowledge bases
 * 
 * LEARNING OBJECTIVES:
 * - Service layer patterns in Go
 * - Clean architecture implementation
 * - Domain-driven design principles
 * - Error handling best practices
 * - Business logic orchestration
 * - Natural language processing basics
 */
package memory

import (
	"context"     // For request cancellation and timeouts
	"log"         // For structured logging
	"regexp"      // For text processing and keyword extraction
	"strings"     // For string manipulation
	"time"        // For timestamp management

	// Internal packages - Clean Architecture dependency flow
	"brain2-backend/internal/domain"     // Core business entities
	"brain2-backend/internal/repository" // Data access interface
	appErrors "brain2-backend/pkg/errors" // Structured error handling

	// External dependencies
	"github.com/google/uuid" // UUID generation for unique identifiers
)

/**
 * Natural Language Processing - Stop Words Filter
 * 
 * Stop words are common words that appear frequently in text but carry little
 * semantic meaning for keyword extraction. Removing them improves the quality
 * of automatic keyword extraction and connection discovery.
 * 
 * WHY STOP WORDS MATTER:
 * 
 * 1. NOISE REDUCTION:
 *    - Words like "the", "and", "is" appear in almost every sentence
 *    - They don't help identify what a memory is actually about
 *    - Removing them focuses on meaningful content words
 * 
 * 2. CONNECTION QUALITY:
 *    - Two memories both containing "the" aren't meaningfully related
 *    - Connections based on content words (nouns, verbs, adjectives) are more valuable
 *    - Improves precision of automatic memory linking
 * 
 * 3. PERFORMANCE OPTIMIZATION:
 *    - Fewer keywords = faster database queries
 *    - Reduced storage requirements
 *    - More efficient graph computation
 * 
 * 4. SEARCH RELEVANCE:
 *    - Users searching for "machine learning" don't want matches for "the"
 *    - Focus on terms that actually distinguish content
 *    - Better user experience for memory discovery
 * 
 * LINGUISTIC CATEGORIES INCLUDED:
 * - Articles: the, a, an
 * - Conjunctions: and, or, but
 * - Prepositions: in, on, at, to, for, of, with
 * - Pronouns: I, you, he, she, it, they
 * - Auxiliary verbs: is, am, are, was, were, have, has, had
 * - Modal verbs: will, would, should, could
 * - Common adverbs: very, just, also, too
 * 
 * FUTURE ENHANCEMENTS:
 * - Language-specific stop word lists
 * - Domain-specific stop words (technical terms that are too common)
 * - Configurable stop word lists per user
 * - Statistical stop word detection based on user's corpus
 * 
 * ALTERNATIVE APPROACHES:
 * - TF-IDF (Term Frequency-Inverse Document Frequency)
 * - Word embeddings (Word2Vec, BERT)
 * - Named entity recognition
 * - Part-of-speech tagging
 */
var stopWords = map[string]bool{
	// Articles and determiners
	"the": true, "a": true, "an": true,
	
	// Conjunctions
	"and": true, "or": true, "but": true,
	
	// Prepositions - spatial and temporal relationships
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "up": true, "about": true,
	"into": true, "through": true, "during": true, "before": true, "after": true,
	"above": true, "below": true, "between": true, "under": true,
	
	// Temporal and sequence words
	"again": true, "further": true, "then": true, "once": true,
	
	// Forms of "be" verb
	"is": true, "am": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true,
	
	// Auxiliary verbs
	"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
	
	// Modal verbs
	"will": true, "would": true, "should": true, "could": true, "ought": true,
	
	// Personal pronouns - first person
	"i": true, "me": true, "my": true, "myself": true,
	"we": true, "our": true, "ours": true, "ourselves": true,
	
	// Personal pronouns - second person
	"you": true, "your": true, "yours": true, "yourself": true, "yourselves": true,
	
	// Personal pronouns - third person
	"he": true, "him": true, "his": true, "himself": true,
	"she": true, "her": true, "hers": true, "herself": true,
	"it": true, "its": true, "itself": true,
	"they": true, "them": true, "their": true, "theirs": true, "themselves": true,
	
	// Question words and demonstratives
	"what": true, "which": true, "who": true, "whom": true,
	"this": true, "that": true, "these": true, "those": true,
	
	// Common function words
	"as": true, "if": true, "each": true, "how": true, "than": true,
	"too": true, "very": true, "can": true, "just": true, "also": true,
}

/**
 * Service Interface - Business Logic Contract
 * 
 * This interface defines the public API for memory-related business operations.
 * It represents the service layer in clean architecture, sitting between
 * the API handlers and the repository layer.
 * 
 * INTERFACE DESIGN PRINCIPLES:
 * 
 * 1. DEPENDENCY INVERSION:
 *    - High-level modules (handlers) depend on this interface
 *    - Low-level modules (implementations) also depend on this interface
 *    - Enables testing with mock implementations
 *    - Supports multiple implementations (different algorithms, optimizations)
 * 
 * 2. SINGLE RESPONSIBILITY:
 *    - Each method has one clear business purpose
 *    - Operations are atomic and focused
 *    - Easy to understand and test
 *    - Follows the Interface Segregation Principle
 * 
 * 3. CONTEXT-AWARE:
 *    - All methods accept context.Context for cancellation and timeouts
 *    - Enables graceful handling of client disconnections
 *    - Supports distributed tracing and request correlation
 *    - Follows Go best practices for service interfaces
 * 
 * 4. ERROR HANDLING:
 *    - Methods return structured errors with business context
 *    - Enables proper HTTP status code mapping
 *    - Supports different error types (validation, not found, internal)
 *    - Consistent error handling across all operations
 * 
 * METHOD CATEGORIES:
 * 
 * 1. CRUD OPERATIONS:
 *    - Create, read, update, delete for individual memories
 *    - Bulk operations for efficiency
 *    - Atomic operations with proper error handling
 * 
 * 2. GRAPH OPERATIONS:
 *    - Connection discovery and management
 *    - Graph data retrieval for visualization
 *    - Relationship traversal and analysis
 * 
 * 3. SEARCH AND DISCOVERY:
 *    - Keyword-based memory finding
 *    - Related memory suggestions
 *    - Content analysis and extraction
 */
type Service interface {
	// CreateNodeAndKeywords saves a memory node with extracted keywords
	// Used by event-driven architecture for asynchronous memory processing
	CreateNodeAndKeywords(ctx context.Context, node domain.Node) error
	
	// CreateNodeWithEdges creates a new memory and immediately finds connections
	// Used for synchronous memory creation with instant relationship discovery
	CreateNodeWithEdges(ctx context.Context, userID, content string) (*domain.Node, error)
	
	// UpdateNode modifies an existing memory and recalculates its connections
	// Handles version management and maintains graph consistency
	UpdateNode(ctx context.Context, userID, nodeID, content string) (*domain.Node, error)
	
	// DeleteNode removes a memory and cleans up all its relationships
	// Ensures referential integrity in the knowledge graph
	DeleteNode(ctx context.Context, userID, nodeID string) error
	
	// BulkDeleteNodes efficiently removes multiple memories in a single operation
	// Returns success count and failed node IDs for partial failure handling
	BulkDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (int, []string, error)
	
	// GetNodeDetails retrieves a memory with its direct connections
	// Used for displaying detailed information in the UI
	GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []domain.Edge, error)
	
	// GetGraphData retrieves the complete knowledge graph for visualization
	// Returns nodes and edges optimized for graph rendering libraries
	GetGraphData(ctx context.Context, userID string) (*domain.Graph, error)
}

/**
 * Service Implementation - Concrete Business Logic
 * 
 * This struct implements the Service interface, providing the actual
 * business logic for memory management operations.
 * 
 * DEPENDENCY INJECTION PATTERN:
 * - Service receives repository interface via constructor
 * - Enables different storage implementations
 * - Supports testing with mock repositories
 * - Follows Dependency Inversion Principle
 */
type service struct {
	// Repository interface for data persistence operations
	// Could be DynamoDB, PostgreSQL, MongoDB, or in-memory for testing
	repo repository.Repository
}

/**
 * Constructor Function - Service Factory
 * 
 * Creates a new service instance with the provided repository.
 * This is the standard Go pattern for dependency injection.
 * 
 * USAGE EXAMPLE:
 * ```go
 * repo := dynamodb.NewRepository(client)
 * memoryService := memory.NewService(repo)
 * ```
 * 
 * @param repo Repository implementation for data persistence
 * @return Service interface implementation
 */
func NewService(repo repository.Repository) Service {
	return &service{repo: repo}
}

/**
 * CreateNodeAndKeywords - Event-Driven Memory Storage
 * 
 * This method stores a memory node that has already been processed
 * (keywords extracted, connections calculated). It's used in the
 * event-driven architecture where memory processing happens asynchronously.
 * 
 * USAGE CONTEXT:
 * 1. User submits memory via HTTP API
 * 2. API immediately returns success to user
 * 3. Background event processor calls this method
 * 4. Memory and keywords are persisted to database
 * 
 * VALIDATION STRATEGY:
 * - Business rule: memories must have content
 * - Fail fast with descriptive error message
 * - Structured error types for proper HTTP response codes
 * 
 * ERROR HANDLING:
 * - Validation errors: 400 Bad Request
 * - Repository errors: 500 Internal Server Error
 * - Context cancellation: 499 Client Closed Request
 * 
 * @param ctx Request context for cancellation and tracing
 * @param node Fully processed memory node with keywords
 * @return error if validation or persistence fails
 */
func (s *service) CreateNodeAndKeywords(ctx context.Context, node domain.Node) error {
	// Business Rule Validation: Content is required
	if node.Content == "" {
		return appErrors.NewValidation("content cannot be empty")
	}
	
	// Delegate to repository layer for persistence
	// Repository handles database-specific operations and error translation
	return s.repo.CreateNodeAndKeywords(ctx, node)
}

/**
 * CreateNodeWithEdges - Synchronous Memory Creation with Instant Connections
 * 
 * This method provides the complete memory creation workflow in a single
 * synchronous operation. It demonstrates the complex orchestration that
 * the service layer handles.
 * 
 * BUSINESS WORKFLOW:
 * 1. Validate input content
 * 2. Extract meaningful keywords using NLP
 * 3. Create domain entity with unique ID and metadata
 * 4. Find existing memories with similar keywords
 * 5. Store new memory with connections to related memories
 * 6. Return created memory for immediate feedback
 * 
 * WHY SYNCHRONOUS:
 * - Immediate user feedback with connections
 * - Simpler error handling (no event processing)
 * - Better for interactive applications
 * - Legacy support during transition to event-driven architecture
 * 
 * KEYWORD MATCHING ALGORITHM:
 * 1. Extract keywords from new memory content
 * 2. Query existing memories that share any keywords
 * 3. Create bidirectional edges between related memories
 * 4. Connection strength based on keyword overlap (future enhancement)
 * 
 * ERROR RESILIENCE:
 * - Memory creation succeeds even if connection discovery fails
 * - Non-critical errors logged but don't fail the operation
 * - Graceful degradation for better user experience
 * 
 * PERFORMANCE CONSIDERATIONS:
 * - Single database transaction for consistency
 * - Parallel processing opportunities (finding relations while creating)
 * - Could be slow for users with many memories (why we have async version)
 * 
 * @param ctx Request context for cancellation and tracing
 * @param userID Owner of the memory (from JWT token)
 * @param content Text content of the memory
 * @return Created memory node with ID and metadata, or error
 */
func (s *service) CreateNodeWithEdges(ctx context.Context, userID, content string) (*domain.Node, error) {
	// Step 1: Input Validation
	if content == "" {
		return nil, appErrors.NewValidation("content cannot be empty")
	}

	// Step 2: Natural Language Processing - Keyword Extraction
	keywords := ExtractKeywords(content)
	
	// Step 3: Domain Entity Creation
	// Build the complete memory node with all required metadata
	node := domain.Node{
		ID:        uuid.New().String(), // Globally unique identifier
		UserID:    userID,              // Multi-tenant isolation
		Content:   content,             // Original user content
		Keywords:  keywords,            // Extracted meaningful terms
		CreatedAt: time.Now(),          // Temporal awareness
		Version:   0,                   // Initial version for optimistic locking
	}

	// Step 4: Connection Discovery - Find Related Memories
	// Query for existing memories that share keywords with this new memory
	query := repository.NodeQuery{
		UserID:   userID,   // Only search within user's memories
		Keywords: keywords, // Find memories with overlapping keywords
	}
	
	// Execute the search for related memories
	relatedNodes, err := s.repo.FindNodes(ctx, query)
	if err != nil {
		// NON-CRITICAL ERROR HANDLING:
		// If we can't find related nodes, we still create the memory
		// This ensures core functionality works even if search fails
		log.Printf("Non-critical error finding related nodes for new node: %v", err)
	}

	// Step 5: Extract Node IDs for Edge Creation
	// Convert node objects to ID strings for relationship storage
	var relatedNodeIDs []string
	for _, rn := range relatedNodes {
		relatedNodeIDs = append(relatedNodeIDs, rn.ID)
	}

	// Step 6: Atomic Storage Operation
	// Store the new memory and all its connections in a single transaction
	if err := s.repo.CreateNodeWithEdges(ctx, node, relatedNodeIDs); err != nil {
		return nil, appErrors.Wrap(err, "failed to create node in repository")
	}

	// Step 7: Return Created Memory
	// Provide immediate feedback to the user with full memory details
	return &node, nil
}

// UpdateNode orchestrates updating a node's content and reconnecting it.
func (s *service) UpdateNode(ctx context.Context, userID, nodeID, content string) (*domain.Node, error) {
	if content == "" {
		return nil, appErrors.NewValidation("content cannot be empty")
	}

	existingNode, err := s.repo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to check for existing node")
	}
	if existingNode == nil {
		return nil, appErrors.NewNotFound("node not found")
	}

	keywords := ExtractKeywords(content)
	updatedNode := domain.Node{
		ID:        nodeID,
		UserID:    userID,
		Content:   content,
		Keywords:  keywords,
		CreatedAt: time.Now(),
		Version:   existingNode.Version + 1,
	}

	query := repository.NodeQuery{
		UserID:   userID,
		Keywords: keywords,
	}
	relatedNodes, err := s.repo.FindNodes(ctx, query)
	if err != nil {
		log.Printf("Non-critical error finding related nodes for updated node %s: %v", nodeID, err)
	}

	var relatedNodeIDs []string
	for _, rn := range relatedNodes {
		if rn.ID != nodeID {
			relatedNodeIDs = append(relatedNodeIDs, rn.ID)
		}
	}

	if err := s.repo.UpdateNodeAndEdges(ctx, updatedNode, relatedNodeIDs); err != nil {
		return nil, appErrors.Wrap(err, "failed to update node in repository")
	}

	return &updatedNode, nil
}

// DeleteNode orchestrates deleting a node.
func (s *service) DeleteNode(ctx context.Context, userID, nodeID string) error {
	existingNode, err := s.repo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return appErrors.Wrap(err, "failed to check for existing node before delete")
	}
	if existingNode == nil {
		return appErrors.NewNotFound("node not found")
	}
	return s.repo.DeleteNode(ctx, userID, nodeID)
}

// BulkDeleteNodes orchestrates deleting multiple nodes.
func (s *service) BulkDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (int, []string, error) {
	if len(nodeIDs) == 0 {
		return 0, nil, appErrors.NewValidation("nodeIds cannot be empty")
	}
	
	if len(nodeIDs) > 100 {
		return 0, nil, appErrors.NewValidation("cannot delete more than 100 nodes at once")
	}

	var failedNodeIDs []string
	deletedCount := 0

	for _, nodeID := range nodeIDs {
		// Check if node exists and belongs to user
		existingNode, err := s.repo.FindNodeByID(ctx, userID, nodeID)
		if err != nil {
			log.Printf("Error checking node %s for user %s: %v", nodeID, userID, err)
			failedNodeIDs = append(failedNodeIDs, nodeID)
			continue
		}
		if existingNode == nil {
			log.Printf("Node %s not found for user %s", nodeID, userID)
			failedNodeIDs = append(failedNodeIDs, nodeID)
			continue
		}

		// Delete the node
		if err := s.repo.DeleteNode(ctx, userID, nodeID); err != nil {
			log.Printf("Error deleting node %s for user %s: %v", nodeID, userID, err)
			failedNodeIDs = append(failedNodeIDs, nodeID)
			continue
		}

		deletedCount++
	}

	return deletedCount, failedNodeIDs, nil
}

// GetNodeDetails retrieves a node and its direct connections.
func (s *service) GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []domain.Edge, error) {
	node, err := s.repo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, "failed to get node from repository")
	}
	if node == nil {
		return nil, nil, appErrors.NewNotFound("node not found")
	}

	edgeQuery := repository.EdgeQuery{
		UserID:   userID,
		SourceID: nodeID,
	}
	edges, err := s.repo.FindEdges(ctx, edgeQuery)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, "failed to get edges from repository")
	}

	return node, edges, nil
}

// GetGraphData retrieves all nodes and edges for a user.
func (s *service) GetGraphData(ctx context.Context, userID string) (*domain.Graph, error) {
	graphQuery := repository.GraphQuery{
		UserID:       userID,
		IncludeEdges: true,
	}
	graph, err := s.repo.GetGraphData(ctx, graphQuery)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get all graph data from repository")
	}
	return graph, nil
}

/**
 * ExtractKeywords - Natural Language Processing for Connection Discovery
 * 
 * This function implements a basic but effective keyword extraction algorithm
 * that identifies meaningful terms from memory content. These keywords enable
 * automatic discovery of relationships between memories.
 * 
 * ALGORITHM OVERVIEW:
 * 1. Text normalization (lowercase, punctuation removal)
 * 2. Tokenization (split into individual words)
 * 3. Stop word filtering (remove common, meaningless words)
 * 4. Length filtering (remove very short words)
 * 5. Deduplication (ensure unique keywords)
 * 
 * WHY KEYWORD EXTRACTION MATTERS:
 * 
 * 1. CONNECTION DISCOVERY:
 *    - Two memories sharing keywords are likely related
 *    - Enables automatic graph construction
 *    - Foundation for recommendation systems
 * 
 * 2. SEARCH OPTIMIZATION:
 *    - Keywords enable fast database queries
 *    - Users can find memories by concept, not exact text
 *    - Supports semantic search capabilities
 * 
 * 3. CONTENT UNDERSTANDING:
 *    - Keywords represent core concepts in memories
 *    - Enable content categorization and organization
 *    - Support for AI-powered insights and analysis
 * 
 * 4. SCALABILITY:
 *    - Keyword-based indexing scales better than full-text search
 *    - Efficient storage and query performance
 *    - Enables graph algorithms on keyword networks
 * 
 * CURRENT ALGORITHM LIMITATIONS:
 * 
 * 1. LINGUISTIC SOPHISTICATION:
 *    - No stemming ("running" and "run" treated as different)
 *    - No lemmatization ("better" and "good" not connected)
 *    - No part-of-speech tagging (keeps all word types)
 * 
 * 2. SEMANTIC UNDERSTANDING:
 *    - No synonym detection ("car" and "automobile")
 *    - No context awareness ("bank" = financial vs. river)
 *    - No entity recognition ("Apple" = company vs. fruit)
 * 
 * 3. DOMAIN SPECIFICITY:
 *    - Generic stop word list (not domain-adapted)
 *    - No technical term recognition
 *    - No user-specific vocabulary learning
 * 
 * FUTURE ENHANCEMENTS:
 * 
 * 1. ADVANCED NLP:
 *    - Integration with spaCy or NLTK
 *    - Named entity recognition
 *    - Part-of-speech tagging for noun/verb focus
 * 
 * 2. MACHINE LEARNING:
 *    - TF-IDF scoring for keyword importance
 *    - Word embeddings (Word2Vec, BERT) for semantic similarity
 *    - User-specific keyword models
 * 
 * 3. DOMAIN ADAPTATION:
 *    - Professional vocabulary detection
 *    - Custom stop word lists per user
 *    - Industry-specific term extraction
 * 
 * @param content Raw text content from user's memory
 * @return Slice of unique, meaningful keywords
 */
func ExtractKeywords(content string) []string {
	// Step 1: Text Normalization - Consistent Processing
	// Convert to lowercase for case-insensitive matching
	content = strings.ToLower(content)
	
	// Step 2: Punctuation Removal - Clean Text
	// Remove all characters except letters, numbers, and spaces
	// This handles punctuation, emojis, special characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	content = reg.ReplaceAllString(content, "")
	
	// Step 3: Tokenization - Split into Words
	// Break the cleaned text into individual words
	// strings.Fields handles multiple spaces and leading/trailing whitespace
	words := strings.Fields(content)
	
	// Step 4: Filtering and Deduplication
	// Use map for O(1) lookup and automatic deduplication
	uniqueWords := make(map[string]bool)
	
	for _, word := range words {
		// Filter 1: Stop Words - Remove common, meaningless words
		// Filter 2: Length - Remove very short words (often not meaningful)
		if !stopWords[word] && len(word) > 2 {
			uniqueWords[word] = true
		}
	}
	
	// Step 5: Convert to Slice - Return Format
	// Pre-allocate slice with known capacity for efficiency
	keywords := make([]string, 0, len(uniqueWords))
	for word := range uniqueWords {
		keywords = append(keywords, word)
	}
	
	return keywords
}
