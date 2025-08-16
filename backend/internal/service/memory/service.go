// Package memory provides business logic for memory node management and connection discovery.
package memory

import (
	"context"
	"regexp"
	"strings"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
)

// stopWords contains common words filtered out during keyword extraction
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true,
	"and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "up": true, "about": true,
	"into": true, "through": true, "during": true, "before": true, "after": true,
	"above": true, "below": true, "between": true, "under": true,
	"again": true, "further": true, "then": true, "once": true,
	"is": true, "am": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
	"will": true, "would": true, "should": true, "could": true, "ought": true,
	"i": true, "me": true, "my": true, "myself": true,
	"we": true, "our": true, "ours": true, "ourselves": true,
	"you": true, "your": true, "yours": true, "yourself": true, "yourselves": true,
	"he": true, "him": true, "his": true, "himself": true,
	"she": true, "her": true, "hers": true, "herself": true,
	"it": true, "its": true, "itself": true,
	"they": true, "them": true, "their": true, "theirs": true, "themselves": true,
	"what": true, "which": true, "who": true, "whom": true,
	"this": true, "that": true, "these": true, "those": true,
	"as": true, "if": true, "each": true, "how": true, "than": true,
	"too": true, "very": true, "can": true, "just": true, "also": true,
}

// Service defines the consolidated interface for memory-related business operations.
type Service interface {
	// Core operations - simplified interface with built-in idempotency and retry
	CreateNode(ctx context.Context, userID, content string, tags []string) (*domain.Node, []*domain.Edge, error)
	UpdateNode(ctx context.Context, userID, nodeID, content string, tags []string) (*domain.Node, error)
	DeleteNode(ctx context.Context, userID, nodeID string) error
	BulkDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (int, []string, error)
	
	// Query operations
	GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []*domain.Edge, error)
	GetNodes(ctx context.Context, userID string, pageReq repository.PageRequest) (*repository.PageResponse, error)
	GetGraphData(ctx context.Context, userID string) (*domain.Graph, error)
}

// service implements the Service interface with concrete business logic using segregated repositories.
type service struct {
	// Segregated repository dependencies - only what this service needs
	nodeRepo         repository.NodeRepository
	edgeRepo         repository.EdgeRepository
	keywordRepo      repository.KeywordRepository
	transactionRepo  repository.TransactionalRepository
	graphRepo        repository.GraphRepository
	idempotencyStore repository.IdempotencyStore
}

// NewService creates a new memory service with segregated repositories.
func NewService(nodeRepo repository.NodeRepository, edgeRepo repository.EdgeRepository, keywordRepo repository.KeywordRepository, transactionRepo repository.TransactionalRepository, graphRepo repository.GraphRepository) Service {
	return &service{
		nodeRepo:        nodeRepo,
		edgeRepo:        edgeRepo,
		keywordRepo:     keywordRepo,
		transactionRepo: transactionRepo,
		graphRepo:       graphRepo,
	}
}

// NewServiceWithIdempotency creates a new memory service with segregated repositories and idempotency support.
func NewServiceWithIdempotency(nodeRepo repository.NodeRepository, edgeRepo repository.EdgeRepository, keywordRepo repository.KeywordRepository, transactionRepo repository.TransactionalRepository, graphRepo repository.GraphRepository, idempotencyStore repository.IdempotencyStore) Service {
	return &service{
		nodeRepo:         nodeRepo,
		edgeRepo:         edgeRepo,
		keywordRepo:      keywordRepo,
		transactionRepo:  transactionRepo,
		graphRepo:        graphRepo,
		idempotencyStore: idempotencyStore,
	}
}

// NewServiceFromRepository creates a memory service from a monolithic repository (for backward compatibility).
func NewServiceFromRepository(repo repository.Repository) Service {
	return &service{
		nodeRepo:        repo,
		edgeRepo:        repo,
		keywordRepo:     repo,
		transactionRepo: repo,
		graphRepo:       repo,
	}
}

// NewServiceFromRepositoryWithIdempotency creates a memory service from a monolithic repository with idempotency (for backward compatibility).
func NewServiceFromRepositoryWithIdempotency(repo repository.Repository, idempotencyStore repository.IdempotencyStore) Service {
	return &service{
		nodeRepo:         repo,
		edgeRepo:         repo,
		keywordRepo:      repo,
		transactionRepo:  repo,
		graphRepo:        repo,
		idempotencyStore: idempotencyStore,
	}
}

// CreateNode creates a new node with automatic edge discovery and idempotency
func (s *service) CreateNode(ctx context.Context, userID, content string, tags []string) (*domain.Node, []*domain.Edge, error) {
	// Check for idempotency key in context
	if idempotencyKey := GetIdempotencyKeyFromContext(ctx); idempotencyKey != "" && s.idempotencyStore != nil {
		key := repository.IdempotencyKey{
			UserID:    userID,
			Operation: "CREATE_NODE",
			Hash:      idempotencyKey,
			CreatedAt: time.Now(),
		}

		// Check if already processed
		if result, exists, _ := s.idempotencyStore.Get(ctx, key); exists {
			if nodeResult, ok := result.(*domain.Node); ok {
				// Return cached result
				edges, _ := s.edgeRepo.FindEdges(ctx, repository.EdgeQuery{
					UserID:   userID,
					SourceID: nodeResult.ID.String(),
				})
				return nodeResult, edges, nil
			}
		}

		// Execute and store
		node, edges, err := s.createNodeCore(ctx, userID, content, tags)
		if err != nil {
			return nil, nil, err
		}

		s.idempotencyStore.Store(ctx, key, node)
		return node, edges, nil
	}

	// Non-idempotent path
	return s.createNodeCore(ctx, userID, content, tags)
}

// UpdateNode updates a node with automatic retry on conflicts and optional idempotency
func (s *service) UpdateNode(ctx context.Context, userID, nodeID, content string, tags []string) (*domain.Node, error) {
	// Check for idempotency
	if idempotencyKey := GetIdempotencyKeyFromContext(ctx); idempotencyKey != "" && s.idempotencyStore != nil {
		key := repository.IdempotencyKey{
			UserID:    userID,
			Operation: "UPDATE_NODE",
			Hash:      idempotencyKey,
			CreatedAt: time.Now(),
		}

		if result, exists, _ := s.idempotencyStore.Get(ctx, key); exists {
			if nodeResult, ok := result.(*domain.Node); ok {
				return nodeResult, nil
			}
		}

		// Execute with retry and store result
		node, err := s.updateNodeCore(ctx, userID, nodeID, content, tags)
		if err != nil {
			return nil, err
		}

		s.idempotencyStore.Store(ctx, key, node)
		return node, nil
	}

	// Non-idempotent path with retry
	return s.updateNodeCore(ctx, userID, nodeID, content, tags)
}

// DeleteNode removes a single node
func (s *service) DeleteNode(ctx context.Context, userID, nodeID string) error {
	return s.nodeRepo.DeleteNode(ctx, userID, nodeID)
}

// BulkDeleteNodes removes multiple nodes with idempotency
func (s *service) BulkDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (int, []string, error) {
	if idempotencyKey := GetIdempotencyKeyFromContext(ctx); idempotencyKey != "" && s.idempotencyStore != nil {
		key := repository.IdempotencyKey{
			UserID:    userID,
			Operation: "BULK_DELETE",
			Hash:      idempotencyKey,
			CreatedAt: time.Now(),
		}

		if result, exists, _ := s.idempotencyStore.Get(ctx, key); exists {
			if deleteResult, ok := result.(map[string]interface{}); ok {
				count, countOk := deleteResult["count"].(int)
				if !countOk {
					count = 0
				}
				failed, failedOk := deleteResult["failed"].([]string)
				if !failedOk {
					failed = []string{}
				}
				return count, failed, nil
			}
		}

		count, failed, err := s.bulkDeleteCore(ctx, userID, nodeIDs)
		if err != nil {
			return 0, nil, err
		}

		s.idempotencyStore.Store(ctx, key, map[string]interface{}{
			"count":  count,
			"failed": failed,
		})
		return count, failed, nil
	}

	return s.bulkDeleteCore(ctx, userID, nodeIDs)
}

// GetNodeDetails retrieves a node and its direct connections.
func (s *service) GetNodeDetails(ctx context.Context, userID, nodeID string) (*domain.Node, []*domain.Edge, error) {
	node, err := s.nodeRepo.FindNodeByID(ctx, userID, nodeID)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, "failed to get node from repository")
	}
	if node == nil {
		return nil, nil, appErrors.NewNotFound("node not found")
	}

	// Fetch ALL edges connected to this node (both as source and target)
	// This ensures we get bidirectional connections properly
	edgeQuery := repository.EdgeQuery{
		UserID:  userID,
		NodeIDs: []string{nodeID}, // This will find edges where node is either source or target
	}
	edges, err := s.edgeRepo.FindEdges(ctx, edgeQuery)
	if err != nil {
		return nil, nil, appErrors.Wrap(err, "failed to get edges from repository")
	}

	return node, edges, nil
}

// GetNodes retrieves paginated nodes (single pagination method)
func (s *service) GetNodes(ctx context.Context, userID string, pageReq repository.PageRequest) (*repository.PageResponse, error) {
	// Validate input parameters
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	
	if pageReq.Limit <= 0 || pageReq.Limit > 100 {
		pageReq.Limit = 20 // Set default limit
	}

	query := repository.NodeQuery{
		UserID: userID,
	}
	
	// Convert to old pagination for now (until repository is updated)
	pagination := repository.Pagination{
		Limit:  pageReq.Limit,
		Cursor: pageReq.NextToken,
	}
	
	page, err := s.nodeRepo.GetNodesPage(ctx, query, pagination)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get nodes page")
	}
	
	// Validate page response
	if page == nil {
		return nil, appErrors.NewInternal("repository returned nil page", nil)
	}
	
	// Get total count for pagination metadata
	total, err := s.nodeRepo.CountNodes(ctx, userID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to count total nodes")
	}
	
	return &repository.PageResponse{
		Items:     page.Items,
		NextToken: page.NextCursor,
		HasMore:   page.HasMore,
		Total:     total,
	}, nil
}

// GetGraphData retrieves the complete graph
func (s *service) GetGraphData(ctx context.Context, userID string) (*domain.Graph, error) {
	// Validate input parameters
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	
	graph, err := s.graphRepo.GetGraphData(ctx, repository.GraphQuery{
		UserID:       userID,
		IncludeEdges: true,
	})
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get graph data")
	}
	
	// Validate graph response
	if graph == nil {
		// Return empty graph instead of nil to prevent nil pointer errors
		return &domain.Graph{
			Nodes: []*domain.Node{},
			Edges: []*domain.Edge{},
		}, nil
	}
	
	return graph, nil
}


// ExtractKeywords extracts meaningful keywords from text content for connection discovery.
func ExtractKeywords(content string) []string {
	content = strings.ToLower(content)
	reg := regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	content = reg.ReplaceAllString(content, "")
	words := strings.Fields(content)

	uniqueWords := make(map[string]bool)
	for _, word := range words {
		if !stopWords[word] && len(word) > 2 {
			uniqueWords[word] = true
		}
	}

	keywords := make([]string, 0, len(uniqueWords))
	for word := range uniqueWords {
		keywords = append(keywords, word)
	}

	return keywords
}

