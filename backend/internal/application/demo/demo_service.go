// Package demo contains a demonstration of Phase 3 CQRS pattern implementation.
// This package shows how the application service layer should be structured
// using Command/Query Responsibility Segregation (CQRS) pattern.
//
// Key Concepts Demonstrated:
//   - Application Service Pattern: Orchestrates use cases
//   - CQRS: Separates commands (writes) from queries (reads)
//   - Command Objects: Encapsulate write operations
//   - Query Objects: Encapsulate read operations
//   - DTOs: Data Transfer Objects for API boundaries
//   - Transaction Management: Unit of Work pattern
//
// This demo works with the existing repository interfaces and serves as
// a reference for how Phase 3 services should be implemented.
package demo

import (
	"context"
	"time"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
)

// CreateNodeCommand demonstrates the Command pattern for write operations.
type CreateNodeCommand struct {
	UserID  string   `json:"user_id"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

// GetNodeQuery demonstrates the Query pattern for read operations.
type GetNodeQuery struct {
	UserID string `json:"user_id"`
	NodeID string `json:"node_id"`
}

// NodeView demonstrates a read-optimized view model.
type NodeView struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Keywords  []string  `json:"keywords"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateNodeResult demonstrates command result DTOs.
type CreateNodeResult struct {
	Node        *NodeView `json:"node"`
	Message     string    `json:"message"`
	Success     bool      `json:"success"`
}

// GetNodeResult demonstrates query result DTOs.
type GetNodeResult struct {
	Node    *NodeView `json:"node"`
	Found   bool      `json:"found"`
}

// DemoNodeService demonstrates the Application Service pattern with CQRS.
// This service shows how to separate command and query responsibilities
// while orchestrating domain operations and repository access.
type DemoNodeService struct {
	// Repository dependencies (using existing interfaces)
	nodeRepo repository.NodeRepository
	
	// In a full implementation, these would be:
	// - unitOfWork repository.UnitOfWork
	// - eventBus shared.EventBus
	// - domainServices (connection analyzer, etc.)
}

// NewDemoNodeService creates a new demonstration service.
func NewDemoNodeService(nodeRepo repository.NodeRepository) *DemoNodeService {
	return &DemoNodeService{
		nodeRepo: nodeRepo,
	}
}

// CreateNode demonstrates a command handler in the application service layer.
// This method shows the full CQRS command pattern:
// 1. Validate command
// 2. Convert to domain objects
// 3. Apply business logic
// 4. Persist changes
// 5. Return result DTO
func (s *DemoNodeService) CreateNode(ctx context.Context, cmd CreateNodeCommand) (*CreateNodeResult, error) {
	// 1. Command validation
	if cmd.UserID == "" {
		return nil, appErrors.NewValidation("user_id is required")
	}
	if cmd.Content == "" {
		return nil, appErrors.NewValidation("content is required")
	}

	// 2. Convert to domain objects (Application -> Domain boundary)
	userID, err := shared.NewUserID(cmd.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user_id: " + err.Error())
	}

	content, err := shared.NewContent(cmd.Content)
	if err != nil {
		return nil, appErrors.NewValidation("invalid content: " + err.Error())
	}

	tags := shared.NewTags(cmd.Tags...)

	// 3. Apply business logic using domain factory
	title, _ := shared.NewTitle("") // Empty title for demo nodes
	node, err := node.NewNode(userID, content, title, tags)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to create node")
	}

	// 4. Persist using repository (in real implementation, this would be in UnitOfWork)
	if err := s.nodeRepo.CreateNodeAndKeywords(ctx, node); err != nil {
		return nil, appErrors.Wrap(err, "failed to save node")
	}

	// 5. Convert domain object to response DTO (Domain -> Application boundary)
	nodeView := &NodeView{
		ID:        node.ID.String(),
		Content:   node.Content.String(),
		Keywords:  node.Keywords().ToSlice(),
		Tags:      node.Tags.ToSlice(),
		CreatedAt: node.CreatedAt,
		UpdatedAt: node.UpdatedAt,
	}

	// 6. Return command result
	return &CreateNodeResult{
		Node:    nodeView,
		Message: "Node created successfully",
		Success: true,
	}, nil
}

// GetNode demonstrates a query handler in the application service layer.
// This method shows the CQRS query pattern:
// 1. Validate query
// 2. Execute read operation
// 3. Convert to view model
// 4. Return result DTO
func (s *DemoNodeService) GetNode(ctx context.Context, query GetNodeQuery) (*GetNodeResult, error) {
	// 1. Query validation
	if query.UserID == "" {
		return nil, appErrors.NewValidation("user_id is required")
	}
	if query.NodeID == "" {
		return nil, appErrors.NewValidation("node_id is required")
	}

	// 2. Execute read operation using repository
	node, err := s.nodeRepo.FindNodeByID(ctx, query.UserID, query.NodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find node")
	}

	// 3. Handle not found case
	if node == nil {
		return &GetNodeResult{
			Node:  nil,
			Found: false,
		}, nil
	}

	// 4. Convert domain object to view model (Domain -> Application boundary)
	nodeView := &NodeView{
		ID:        node.ID.String(),
		Content:   node.Content.String(),
		Keywords:  node.Keywords().ToSlice(),
		Tags:      node.Tags.ToSlice(),
		CreatedAt: node.CreatedAt,
		UpdatedAt: node.UpdatedAt,
	}

	// 5. Return query result
	return &GetNodeResult{
		Node:  nodeView,
		Found: true,
	}, nil
}

// DemoQueryService demonstrates a separate query service for complex read operations.
// This shows how CQRS can be used to optimize read paths independently from write paths.
type DemoQueryService struct {
	nodeRepo repository.NodeRepository
	// In a full implementation: cache, read-optimized repositories, etc.
}

// NewDemoQueryService creates a new query service.
func NewDemoQueryService(nodeRepo repository.NodeRepository) *DemoQueryService {
	return &DemoQueryService{
		nodeRepo: nodeRepo,
	}
}

// ListNodesQuery demonstrates a more complex query with filtering and pagination.
type ListNodesQuery struct {
	UserID    string `json:"user_id"`
	Limit     int    `json:"limit"`
	NextToken string `json:"next_token"`
}

// ListNodesResult demonstrates a paginated query result.
type ListNodesResult struct {
	Nodes     []*NodeView `json:"nodes"`
	NextToken string      `json:"next_token"`
	HasMore   bool        `json:"has_more"`
	Total     int         `json:"total"`
}

// ListNodes demonstrates a complex query operation with caching potential.
func (s *DemoQueryService) ListNodes(ctx context.Context, query ListNodesQuery) (*ListNodesResult, error) {
	// 1. Query validation
	if query.UserID == "" {
		return nil, appErrors.NewValidation("user_id is required")
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20 // Default limit
	}

	// 2. Build repository query
	nodeQuery := repository.NodeQuery{
		UserID: query.UserID,
	}

	pagination := repository.Pagination{
		Limit:  query.Limit,
		Cursor: query.NextToken,
	}

	// 3. Execute paginated query
	page, err := s.nodeRepo.GetNodesPage(ctx, nodeQuery, pagination)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get nodes page")
	}

	// 4. Convert to view models
	var nodeViews []*NodeView
	if page != nil {
		nodeViews = make([]*NodeView, len(page.Items))
		for i, node := range page.Items {
			nodeViews[i] = &NodeView{
				ID:        node.ID.String(),
				Content:   node.Content.String(),
				Keywords:  node.Keywords().ToSlice(),
				Tags:      node.Tags.ToSlice(),
				CreatedAt: node.CreatedAt,
				UpdatedAt: node.UpdatedAt,
			}
		}
	}

	// 5. Get total count for pagination metadata
	total, err := s.nodeRepo.CountNodes(ctx, query.UserID)
	if err != nil {
		total = 0 // Graceful degradation
	}

	// 6. Build paginated result
	result := &ListNodesResult{
		Nodes: nodeViews,
		Total: total,
	}

	if page != nil {
		result.NextToken = page.NextCursor
		result.HasMore = page.HasMore
	}

	return result, nil
}

// Phase3ArchitectureDemo demonstrates the complete Phase 3 architecture.
type Phase3ArchitectureDemo struct {
	// Command side (writes)
	NodeService *DemoNodeService
	
	// Query side (reads)
	QueryService *DemoQueryService
}

// NewPhase3ArchitectureDemo creates a complete demo of Phase 3 architecture.
func NewPhase3ArchitectureDemo(nodeRepo repository.NodeRepository) *Phase3ArchitectureDemo {
	return &Phase3ArchitectureDemo{
		NodeService:  NewDemoNodeService(nodeRepo),
		QueryService: NewDemoQueryService(nodeRepo),
	}
}

// Usage demonstrates how the services would be used:
/*
// In a handler:
func (h *Handler) CreateNodeHandler(w http.ResponseWriter, r *http.Request) {
	var cmd demo.CreateNodeCommand
	json.NewDecoder(r.Body).Decode(&cmd)
	
	result, err := h.demo.NodeService.CreateNode(r.Context(), cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) GetNodeHandler(w http.ResponseWriter, r *http.Request) {
	query := demo.GetNodeQuery{
		UserID: r.URL.Query().Get("user_id"),
		NodeID: chi.URLParam(r, "nodeID"),
	}
	
	result, err := h.demo.QueryService.GetNode(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	json.NewEncoder(w).Encode(result)
}
*/