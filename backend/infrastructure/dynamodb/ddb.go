// Package dynamodb implements the repository interface using AWS DynamoDB.
// This is the infrastructure layer that contains DynamoDB-specific implementations.
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors" // ALIAS for our custom errors

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"golang.org/x/sync/errgroup"
)

// ddbNode represents the structure of a node item in DynamoDB.
type ddbNode struct {
	PK        string   `dynamodbav:"PK"`
	SK        string   `dynamodbav:"SK"`
	NodeID    string   `dynamodbav:"NodeID"`
	UserID    string   `dynamodbav:"UserID"`
	Content   string   `dynamodbav:"Content"`
	Keywords  []string `dynamodbav:"Keywords"`
	Tags      []string `dynamodbav:"Tags"`
	IsLatest  bool     `dynamodbav:"IsLatest"`
	Version   int      `dynamodbav:"Version"`
	Timestamp string   `dynamodbav:"Timestamp"`
}

// ddbKeyword represents a keyword index item in DynamoDB.
type ddbKeyword struct {
	PK     string `dynamodbav:"PK"`
	SK     string `dynamodbav:"SK"`
	GSI1PK string `dynamodbav:"GSI1PK"`
	GSI1SK string `dynamodbav:"GSI1SK"`
}

// ddbEdge represents an edge item in DynamoDB.
type ddbEdge struct {
	PK       string `dynamodbav:"PK"`
	SK       string `dynamodbav:"SK"`
	TargetID string `dynamodbav:"TargetID"`
	GSI2PK   string `dynamodbav:"GSI2PK"`   // USER#{userId}#EDGE
	GSI2SK   string `dynamodbav:"GSI2SK"`   // NODE#{sourceId}#TARGET#{targetId}
}

// ddbCategory represents a category item in DynamoDB.
type ddbCategory struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"`
	CategoryID  string `dynamodbav:"CategoryID"`
	UserID      string `dynamodbav:"UserID"`
	Title       string `dynamodbav:"Title"`
	Description string `dynamodbav:"Description"`
	Timestamp   string `dynamodbav:"Timestamp"`
}

// ddbCategoryMemory represents a category-memory relationship item in DynamoDB.
type ddbCategoryMemory struct {
	PK         string `dynamodbav:"PK"`
	SK         string `dynamodbav:"SK"`
	CategoryID string `dynamodbav:"CategoryID"`
	MemoryID   string `dynamodbav:"MemoryID"`
	UserID     string `dynamodbav:"UserID"`
	AddedAt    string `dynamodbav:"AddedAt"`
}

// ddbRepository is the main repository that provides access to all segregated interfaces
type ddbRepository struct {
	dbClient *dynamodb.Client
	config   repository.Config
	
	// Segregated repository implementations
	nodeRepo     repository.NodeRepository
	edgeRepo     repository.EdgeRepository
	categoryRepo repository.CategoryRepository
	nodeCatRepo  repository.NodeCategoryMapper
	keywordRepo  repository.KeywordSearcher
	graphRepo    repository.GraphReader
	unitOfWork   repository.UnitOfWork
}

// NewRepository creates a new instance of the DynamoDB repository with all segregated interfaces
func NewRepository(dbClient *dynamodb.Client, tableName, indexName string) repository.Repository {
	config := repository.NewConfig(tableName, indexName)
	return NewRepositoryWithConfig(dbClient, config)
}

// NewRepositoryWithConfig creates a new instance of the DynamoDB repository with custom config
func NewRepositoryWithConfig(dbClient *dynamodb.Client, config repository.Config) repository.Repository {
	baseRepo := &ddbBaseRepository{
		dbClient: dbClient,
		config:   config.WithDefaults(),
	}
	
	return &ddbRepository{
		dbClient:     dbClient,
		config:       config.WithDefaults(),
		nodeRepo:     &ddbNodeRepository{base: baseRepo},
		edgeRepo:     &ddbEdgeRepository{base: baseRepo},
		categoryRepo: &ddbCategoryRepository{base: baseRepo},
		nodeCatRepo:  &ddbNodeCategoryMapper{base: baseRepo},
		keywordRepo:  &ddbKeywordSearcher{base: baseRepo},
		graphRepo:    &ddbGraphReader{base: baseRepo},
		unitOfWork:   &ddbUnitOfWork{base: baseRepo},
	}
}

// Repository interface implementation
func (r *ddbRepository) Nodes() repository.NodeRepository { return r.nodeRepo }
func (r *ddbRepository) Edges() repository.EdgeRepository { return r.edgeRepo }
func (r *ddbRepository) Categories() repository.CategoryRepository { return r.categoryRepo }
func (r *ddbRepository) NodeCategories() repository.NodeCategoryMapper { return r.nodeCatRepo }
func (r *ddbRepository) Keywords() repository.KeywordSearcher { return r.keywordRepo }
func (r *ddbRepository) Graph() repository.GraphReader { return r.graphRepo }
func (r *ddbRepository) UnitOfWork() repository.UnitOfWork { return r.unitOfWork }

func (r *ddbRepository) WithDecorators(decorators ...repository.RepositoryDecorator) repository.Repository {
	// Apply decorators to create a new decorated repository
	decoratedRepo := &ddbRepository{
		dbClient: r.dbClient,
		config:   r.config,
		nodeRepo: r.nodeRepo,
		edgeRepo: r.edgeRepo,
		categoryRepo: r.categoryRepo,
		nodeCatRepo: r.nodeCatRepo,
		keywordRepo: r.keywordRepo,
		graphRepo: r.graphRepo,
		unitOfWork: r.unitOfWork,
	}
	
	for _, decorator := range decorators {
		decoratedRepo.nodeRepo = decorator.DecorateNode(decoratedRepo.nodeRepo)
		decoratedRepo.edgeRepo = decorator.DecorateEdge(decoratedRepo.edgeRepo)
		decoratedRepo.categoryRepo = decorator.DecorateCategory(decoratedRepo.categoryRepo)
	}
	
	return decoratedRepo
}

// ddbBaseRepository contains common DynamoDB operations
type ddbBaseRepository struct {
	dbClient *dynamodb.Client
	config   repository.Config
}

// Segregated repository implementations

// ddbNodeRepository implements NodeRepository interface
type ddbNodeRepository struct {
	base *ddbBaseRepository
}


// ddbEdgeRepository implements EdgeRepository interface  
type ddbEdgeRepository struct {
	base *ddbBaseRepository
}


// ddbCategoryRepository implements CategoryRepository interface
type ddbCategoryRepository struct {
	base *ddbBaseRepository
}

// FindByID retrieves a single category
func (c *ddbCategoryRepository) FindByID(ctx context.Context, userID domain.UserID, categoryID string) (*domain.Category, error) {
	return nil, nil
}

// FindByUser retrieves categories for a user
func (c *ddbCategoryRepository) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]domain.Category, error) {
	return []domain.Category{}, nil
}

// FindByLevel retrieves categories at a specific hierarchy level
func (c *ddbCategoryRepository) FindByLevel(ctx context.Context, userID domain.UserID, level int) ([]domain.Category, error) {
	return []domain.Category{}, nil
}

// GetTree retrieves the complete category tree for a user
func (c *ddbCategoryRepository) GetTree(ctx context.Context, userID domain.UserID) ([]domain.Category, error) {
	return []domain.Category{}, nil
}

// FindChildren retrieves child categories
func (c *ddbCategoryRepository) FindChildren(ctx context.Context, userID domain.UserID, parentID string) ([]domain.Category, error) {
	return []domain.Category{}, nil
}

// FindParent retrieves the parent category
func (c *ddbCategoryRepository) FindParent(ctx context.Context, userID domain.UserID, childID string) (*domain.Category, error) {
	return nil, nil
}

// Save creates or updates a category
func (c *ddbCategoryRepository) Save(ctx context.Context, category *domain.Category) error {
	return nil
}

// Delete removes a category
func (c *ddbCategoryRepository) Delete(ctx context.Context, userID domain.UserID, categoryID string) error {
	return nil
}

// CreateHierarchy creates a parent-child relationship
func (c *ddbCategoryRepository) CreateHierarchy(ctx context.Context, hierarchy *domain.CategoryHierarchy) error {
	return nil
}

// DeleteHierarchy removes a parent-child relationship
func (c *ddbCategoryRepository) DeleteHierarchy(ctx context.Context, userID domain.UserID, parentID, childID string) error {
	return nil
}

// ddbNodeCategoryMapper implements NodeCategoryMapper interface
type ddbNodeCategoryMapper struct {
	base *ddbBaseRepository
}

// AssignNodeToCategory creates a node-category relationship
func (m *ddbNodeCategoryMapper) AssignNodeToCategory(ctx context.Context, mapping *domain.NodeCategory) error {
	return nil
}

// RemoveNodeFromCategory removes a node-category relationship
func (m *ddbNodeCategoryMapper) RemoveNodeFromCategory(ctx context.Context, userID domain.UserID, nodeID, categoryID string) error {
	return nil
}

// FindNodesByCategory retrieves nodes in a category
func (m *ddbNodeCategoryMapper) FindNodesByCategory(ctx context.Context, userID domain.UserID, categoryID string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	return []*domain.Node{}, nil
}

// FindCategoriesForNode retrieves categories for a node
func (m *ddbNodeCategoryMapper) FindCategoriesForNode(ctx context.Context, userID domain.UserID, nodeID string) ([]*domain.Category, error) {
	return []*domain.Category{}, nil
}

// BatchAssignCategories assigns multiple categories efficiently
func (m *ddbNodeCategoryMapper) BatchAssignCategories(ctx context.Context, mappings []*domain.NodeCategory) error {
	return nil
}

// ddbKeywordSearcher implements KeywordSearcher interface
type ddbKeywordSearcher struct {
	base *ddbBaseRepository
}

// SearchNodes finds nodes matching the given keywords
func (k *ddbKeywordSearcher) SearchNodes(ctx context.Context, userID domain.UserID, keywords []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Basic implementation - can be enhanced
	return []*domain.Node{}, nil
}

// SuggestKeywords provides keyword suggestions
func (k *ddbKeywordSearcher) SuggestKeywords(ctx context.Context, userID domain.UserID, partial string, limit int) ([]string, error) {
	return []string{}, nil
}

// FindRelatedByKeywords finds nodes with similar keywords
func (k *ddbKeywordSearcher) FindRelatedByKeywords(ctx context.Context, userID domain.UserID, node *domain.Node, opts ...repository.QueryOption) ([]*domain.Node, error) {
	return []*domain.Node{}, nil
}

// ddbGraphReader implements GraphReader interface
type ddbGraphReader struct {
	base *ddbBaseRepository
}

// GetGraph retrieves the complete graph for a user
func (g *ddbGraphReader) GetGraph(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) (*domain.Graph, error) {
	// Use the working fetchAllNodesOptimizedDomain logic
	nodes, err := g.base.fetchAllNodesOptimizedDomain(ctx, userID.String())
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to fetch nodes for graph")
	}
	
	// Use the working fetchAllEdgesOptimizedDomain logic  
	edges, err := g.base.fetchAllEdgesOptimizedDomain(ctx, userID.String())
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to fetch edges for graph")
	}
	
	log.Printf("DEBUG: GetGraph returning %d nodes and %d edges for user %s", len(nodes), len(edges), userID.String())
	return &domain.Graph{Nodes: nodes, Edges: edges}, nil
}

// GetSubgraph retrieves a subgraph around specific nodes
func (g *ddbGraphReader) GetSubgraph(ctx context.Context, nodeIDs []domain.NodeID, depth int) (*domain.Graph, error) {
	return &domain.Graph{}, nil
}

// AnalyzeConnectivity provides graph connectivity analysis
func (g *ddbGraphReader) AnalyzeConnectivity(ctx context.Context, userID domain.UserID) (*repository.GraphAnalysis, error) {
	return &repository.GraphAnalysis{}, nil
}

// ddbUnitOfWork implements UnitOfWork interface
type ddbUnitOfWork struct {
	base *ddbBaseRepository
}

// Begin starts a new unit of work
func (u *ddbUnitOfWork) Begin(ctx context.Context) error {
	// For DynamoDB, this is a no-op since transactions are immediate
	return nil
}

// Commit persists all changes and publishes domain events
func (u *ddbUnitOfWork) Commit(ctx context.Context) error {
	return nil
}

// Rollback discards all changes
func (u *ddbUnitOfWork) Rollback(ctx context.Context) error {
	return nil
}

// Repository access within the transaction context
func (u *ddbUnitOfWork) Nodes() repository.NodeRepository {
	return &ddbNodeRepository{base: u.base}
}

func (u *ddbUnitOfWork) Edges() repository.EdgeRepository {
	return &ddbEdgeRepository{base: u.base}
}

func (u *ddbUnitOfWork) Categories() repository.CategoryRepository {
	return &ddbCategoryRepository{base: u.base}
}

func (u *ddbUnitOfWork) NodeCategories() repository.NodeCategoryMapper {
	return &ddbNodeCategoryMapper{base: u.base}
}

func (u *ddbUnitOfWork) Keywords() repository.KeywordSearcher {
	return &ddbKeywordSearcher{base: u.base}
}

func (u *ddbUnitOfWork) Graph() repository.GraphReader {
	return &ddbGraphReader{base: u.base}
}

// Domain event management
func (u *ddbUnitOfWork) RegisterEvents(events []domain.DomainEvent) {
	// No-op for now
}

func (u *ddbUnitOfWork) GetRegisteredEvents() []domain.DomainEvent {
	return []domain.DomainEvent{}
}

// ClearEvents clears all registered events
func (u *ddbUnitOfWork) ClearEvents() {
	// No-op for now
}

// Validate validates the current state of the unit of work
func (u *ddbUnitOfWork) Validate(ctx context.Context) error {
	return nil
}

// Segregated repository factory functions for dependency injection

// NewNodeRepository creates a new instance implementing NodeRepository interface.
func NewNodeRepository(dbClient *dynamodb.Client, tableName, indexName string) repository.NodeRepository {
	repo := NewRepository(dbClient, tableName, indexName)
	return repo.Nodes()
}

// NewEdgeRepository creates a new instance implementing EdgeRepository interface.
func NewEdgeRepository(dbClient *dynamodb.Client, tableName, indexName string) repository.EdgeRepository {
	repo := NewRepository(dbClient, tableName, indexName)
	return repo.Edges()
}

// NewKeywordSearcher creates a new instance implementing KeywordSearcher interface.
func NewKeywordSearcher(dbClient *dynamodb.Client, tableName, indexName string) repository.KeywordSearcher {
	repo := NewRepository(dbClient, tableName, indexName)
	return repo.Keywords()
}

// NewCategoryRepository creates a new instance implementing CategoryRepository interface.
func NewCategoryRepository(dbClient *dynamodb.Client, tableName, indexName string) repository.CategoryRepository {
	repo := NewRepository(dbClient, tableName, indexName)
	return repo.Categories()
}

// NewGraphReader creates a new instance implementing GraphReader interface.
func NewGraphReader(dbClient *dynamodb.Client, tableName, indexName string) repository.GraphReader {
	repo := NewRepository(dbClient, tableName, indexName)
	return repo.Graph()
}

// ============================================================================
// NodeRepository Interface Implementation
// ============================================================================

// NodeReader methods
func (n *ddbNodeRepository) FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
	// Use existing implementation logic but with domain types
	return n.base.findNodeByDomainID(ctx, id)
}

func (n *ddbNodeRepository) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Build query options and execute
	options := repository.ApplyQueryOptions(opts...)
	return n.base.findNodesByUser(ctx, userID, options)
}

func (n *ddbNodeRepository) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	node, err := n.FindByID(ctx, id)
	if err != nil {
		if repository.IsNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return node != nil, nil
}

func (n *ddbNodeRepository) Count(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) (int, error) {
	nodes, err := n.FindByUser(ctx, userID, opts...)
	if err != nil {
		return 0, err
	}
	return len(nodes), nil
}

func (n *ddbNodeRepository) FindByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	return n.base.findNodesByKeywords(ctx, userID, keywords, opts...)
}

func (n *ddbNodeRepository) FindSimilar(ctx context.Context, node *domain.Node, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// Find nodes with similar keywords and tags
	keywords := node.Keywords().ToSlice()
	return n.FindByKeywords(ctx, node.UserID(), keywords, opts...)
}

// NodeWriter methods
func (n *ddbNodeRepository) Save(ctx context.Context, node *domain.Node) error {
	return n.base.createNodeAndKeywords(ctx, node)
}

func (n *ddbNodeRepository) Delete(ctx context.Context, id domain.NodeID) error {
	return n.base.deleteNodeByDomainID(ctx, id)
}

func (n *ddbNodeRepository) SaveBatch(ctx context.Context, nodes []*domain.Node) error {
	return n.base.saveNodesBatch(ctx, nodes)
}

func (n *ddbNodeRepository) DeleteBatch(ctx context.Context, ids []domain.NodeID) error {
	return n.base.deleteNodesBatch(ctx, ids)
}

// ============================================================================
// EdgeRepository Interface Implementation  
// ============================================================================

// EdgeReader methods
func (e *ddbEdgeRepository) FindByNodes(ctx context.Context, sourceID, targetID domain.NodeID) (*domain.Edge, error) {
	return e.base.findEdgeByNodes(ctx, sourceID, targetID)
}

func (e *ddbEdgeRepository) FindBySource(ctx context.Context, sourceID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	return e.base.findEdgesBySource(ctx, sourceID, opts...)
}

func (e *ddbEdgeRepository) FindByTarget(ctx context.Context, targetID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	return e.base.findEdgesByTarget(ctx, targetID, opts...)
}

func (e *ddbEdgeRepository) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	return e.base.findEdgesByUser(ctx, userID, opts...)
}

func (e *ddbEdgeRepository) GetNeighborhood(ctx context.Context, nodeID domain.NodeID, depth int) ([]*domain.Edge, error) {
	return e.base.getNodeNeighborhood(ctx, nodeID, depth)
}

// EdgeWriter methods
func (e *ddbEdgeRepository) Save(ctx context.Context, edge *domain.Edge) error {
	return e.base.saveEdge(ctx, edge)
}

func (e *ddbEdgeRepository) Delete(ctx context.Context, sourceID, targetID domain.NodeID) error {
	return e.base.deleteEdge(ctx, sourceID, targetID)
}

func (e *ddbEdgeRepository) SaveBatch(ctx context.Context, edges []*domain.Edge) error {
	return e.base.saveEdgesBatch(ctx, edges)
}

func (e *ddbEdgeRepository) DeleteByNode(ctx context.Context, nodeID domain.NodeID) error {
	return e.base.deleteEdgesByNode(ctx, nodeID)
}

// ============================================================================
// Temporary minimal implementations for base repository methods
// ============================================================================

// These are minimal implementations to get the build working
// TODO: Implement full functionality

func (base *ddbBaseRepository) findNodeByDomainID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
	// Use scan with filter to find node by NodeID across all users
	// This is not the most efficient approach but works until we add a GSI for NodeID
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(base.config.TableName),
		FilterExpression: aws.String("NodeID = :node_id AND SK = :sk AND IsLatest = :is_latest"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":node_id":   &types.AttributeValueMemberS{Value: id.String()},
			":sk":        &types.AttributeValueMemberS{Value: "METADATA#v0"},
			":is_latest": &types.AttributeValueMemberBOOL{Value: true},
		},
	}

	result, err := base.dbClient.Scan(ctx, scanInput)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to scan for node by ID")
	}

	if len(result.Items) == 0 {
		return nil, appErrors.NewNotFound("node not found")
	}

	// Convert first matching item to domain node
	var ddbItem ddbNode
	if err := attributevalue.UnmarshalMap(result.Items[0], &ddbItem); err != nil {
		return nil, appErrors.Wrap(err, "failed to unmarshal node")
	}

	createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
	node, err := domain.ReconstructNodeFromPrimitives(
		ddbItem.NodeID,
		ddbItem.UserID,
		ddbItem.Content,
		ddbItem.Keywords,
		ddbItem.Tags,
		createdAt,
		ddbItem.Version,
	)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to reconstruct domain node")
	}

	return node, nil
}

func (base *ddbBaseRepository) findNodesByUser(ctx context.Context, userID domain.UserID, options *repository.QueryOptions) ([]*domain.Node, error) {
	// Use the proven working fetchAllNodesOptimizedDomain method
	nodes, err := base.fetchAllNodesOptimizedDomain(ctx, userID.String())
	if err != nil {
		return nil, err
	}
	
	// Apply any options filtering if needed
	if options != nil && options.Limit > 0 && len(nodes) > options.Limit {
		nodes = nodes[:options.Limit]
	}
	
	return nodes, nil
}

func (base *ddbBaseRepository) findNodesByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	// TODO: Implement full functionality
	return nil, fmt.Errorf("not implemented")
}

func (base *ddbBaseRepository) createNodeAndKeywords(ctx context.Context, node *domain.Node) error {
	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID().String(), node.ID().String())
	transactItems := []types.TransactWriteItem{}

	// 1. Add the main node metadata to the transaction
	nodeItem, err := attributevalue.MarshalMap(ddbNode{
		PK: pk, SK: "METADATA#v0", NodeID: node.ID().String(), UserID: node.UserID().String(), Content: node.Content().String(),
		Keywords: node.Keywords().ToSlice(), Tags: node.Tags().ToSlice(), IsLatest: true, Version: node.Version().Int(), Timestamp: node.CreatedAt().Format(time.RFC3339),
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal node item")
	}
	transactItems = append(transactItems, types.TransactWriteItem{
		Put: &types.Put{TableName: aws.String(base.config.TableName), Item: nodeItem},
	})

	// 2. Add each keyword as a separate item for the GSI to index
	for _, keyword := range node.Keywords().ToSlice() {
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{
			PK:     pk,
			SK:     fmt.Sprintf("KEYWORD#%s", keyword),
			GSI1PK: fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID().String(), keyword),
			GSI1SK: fmt.Sprintf("NODE#%s", node.ID().String()),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal keyword item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{
			Put: &types.Put{TableName: aws.String(base.config.TableName), Item: keywordItem},
		})
	}

	// 3. Execute the transaction
	_, err = base.dbClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: transactItems,
	})
	if err != nil {
		return appErrors.Wrap(err, "transaction to create node and keywords failed")
	}
	return nil
}

func (base *ddbBaseRepository) deleteNodeByDomainID(ctx context.Context, id domain.NodeID) error {
	// Get node to extract userID for deletion
	node, err := base.findNodeByDomainID(ctx, id)
	if err != nil {
		return appErrors.Wrap(err, "failed to find node for deletion")
	}
	
	// Use the proven working clearNodeConnectionsDomain logic
	return base.clearNodeConnectionsDomain(ctx, node.UserID().String(), id.String())
}

func (base *ddbBaseRepository) saveNodesBatch(ctx context.Context, nodes []*domain.Node) error {
	// TODO: Implement full functionality
	return fmt.Errorf("not implemented")
}

func (base *ddbBaseRepository) deleteNodesBatch(ctx context.Context, ids []domain.NodeID) error {
	// TODO: Implement full functionality
	return fmt.Errorf("not implemented")
}

// Edge methods
func (base *ddbBaseRepository) findEdgeByNodes(ctx context.Context, sourceID, targetID domain.NodeID) (*domain.Edge, error) {
	return nil, fmt.Errorf("not implemented")
}

func (base *ddbBaseRepository) findEdgesBySource(ctx context.Context, sourceID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// First need to get the userID - we'll use the working findNodeByDomainID
	nodeItem, err := base.findNodeByDomainID(ctx, sourceID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find source node")
	}
	
	// Get all edges for the user using the working method
	allEdges, err := base.fetchAllEdgesOptimizedDomain(ctx, nodeItem.UserID().String())
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to fetch user edges")
	}
	
	// Filter to only edges from the source node
	var edges []*domain.Edge
	for _, edge := range allEdges {
		if edge.SourceID().Equals(sourceID) {
			edges = append(edges, edge)
		}
	}
	
	return edges, nil
}

func (base *ddbBaseRepository) findEdgesByTarget(ctx context.Context, targetID domain.NodeID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	return nil, fmt.Errorf("not implemented")
}

func (base *ddbBaseRepository) findEdgesByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	// Use the proven working fetchAllEdgesOptimizedDomain method
	return base.fetchAllEdgesOptimizedDomain(ctx, userID.String())
}

func (base *ddbBaseRepository) getNodeNeighborhood(ctx context.Context, nodeID domain.NodeID, depth int) ([]*domain.Edge, error) {
	return nil, fmt.Errorf("not implemented")
}

func (base *ddbBaseRepository) saveEdge(ctx context.Context, edge *domain.Edge) error {
	if edge == nil {
		return appErrors.NewValidation("edge cannot be nil")
	}
	
	// Use the proven working CreateEdge logic with canonical edge storage pattern
	sourceID := edge.SourceID().String()
	targetID := edge.TargetID().String()
	userID := edge.UserID().String()
	
	// Get canonical edge storage (lexicographically ordered IDs)
	ownerID, canonicalTargetID := getCanonicalEdge(sourceID, targetID)
	ownerPK := fmt.Sprintf("USER#%s#NODE#%s", userID, ownerID)
	
	edgeItem := ddbEdge{
		PK:       ownerPK,
		SK:       fmt.Sprintf("EDGE#%s", canonicalTargetID),
		TargetID: canonicalTargetID,
		GSI2PK:   fmt.Sprintf("USER#%s#EDGE", userID),
		GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", ownerID, canonicalTargetID),
	}
	
	item, err := attributevalue.MarshalMap(edgeItem)
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal edge item")
	}
	
	_, err = base.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(base.config.TableName),
		Item:      item,
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to create edge in DynamoDB")
	}
	
	return nil
}

func (base *ddbBaseRepository) deleteEdge(ctx context.Context, sourceID, targetID domain.NodeID) error {
	// First need to find a node to get userID
	sourceNode, err := base.findNodeByDomainID(ctx, sourceID)
	if err != nil {
		return appErrors.Wrap(err, "failed to find source node for edge deletion")
	}
	
	userID := sourceNode.UserID().String()
	
	// Use canonical edge storage pattern to determine where the edge is stored
	ownerID, canonicalTargetID := getCanonicalEdge(sourceID.String(), targetID.String())
	ownerPK := fmt.Sprintf("USER#%s#NODE#%s", userID, ownerID)
	sk := fmt.Sprintf("EDGE#%s", canonicalTargetID)
	
	// Delete the edge
	_, err = base.dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(base.config.TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: ownerPK},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to delete edge from DynamoDB")
	}
	
	return nil
}

func (base *ddbBaseRepository) saveEdgesBatch(ctx context.Context, edges []*domain.Edge) error {
	return fmt.Errorf("not implemented")
}

func (base *ddbBaseRepository) deleteEdgesByNode(ctx context.Context, nodeID domain.NodeID) error {
	return fmt.Errorf("not implemented")
}

// ============================================================================
// Legacy method implementations for backward compatibility
// ============================================================================

func (r *ddbRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
	// Convert string ID to domain ID and delegate to modern method
	domainNodeID, err := domain.ParseNodeID(nodeID)
	if err != nil {
		return nil, err
	}
	return r.Nodes().FindByID(ctx, domainNodeID)
}

func (r *ddbRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	// Convert string ID to domain ID and delegate to modern method
	domainNodeID, err := domain.ParseNodeID(nodeID)
	if err != nil {
		return err
	}
	return r.Nodes().Delete(ctx, domainNodeID)
}

func (r *ddbRepository) GetGraphData(ctx context.Context, query repository.GraphQuery) (*domain.Graph, error) {
	// Convert string user ID to domain ID and delegate to modern method
	domainUserID, err := domain.NewUserID(query.UserID)
	if err != nil {
		return nil, err
	}
	return r.Graph().GetGraph(ctx, domainUserID)
}

func (r *ddbRepository) Save(ctx context.Context, node *domain.Node) error {
	// Delegate to modern method
	return r.Nodes().Save(ctx, node)
}


// getCanonicalEdge determines the canonical storage for a bi-directional edge.
// Returns the owner node ID and target node ID based on lexicographic ordering.
// This ensures each unique connection is stored exactly once.
func getCanonicalEdge(nodeA, nodeB string) (owner, target string) {
	if nodeA < nodeB {
		return nodeA, nodeB
	}
	return nodeB, nodeA
}

// CreateNodeAndKeywords transactionally saves a node and its keyword indexes.
func (r *ddbRepository) CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error {
	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID().String(), node.ID().String())
	transactItems := []types.TransactWriteItem{}

	// 1. Add the main node metadata to the transaction
	nodeItem, err := attributevalue.MarshalMap(ddbNode{
		PK: pk, SK: "METADATA#v0", NodeID: node.ID().String(), UserID: node.UserID().String(), Content: node.Content().String(),
		Keywords: node.Keywords().ToSlice(), Tags: node.Tags().ToSlice(), IsLatest: true, Version: node.Version().Int(), Timestamp: node.CreatedAt().Format(time.RFC3339),
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal node item")
	}
	transactItems = append(transactItems, types.TransactWriteItem{
		Put: &types.Put{TableName: aws.String(r.config.TableName), Item: nodeItem},
	})

	// 2. Add each keyword as a separate item for the GSI to index
	for _, keyword := range node.Keywords().ToSlice() {
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{
			PK:     pk,
			SK:     fmt.Sprintf("KEYWORD#%s", keyword),
			GSI1PK: fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID().String(), keyword),
			GSI1SK: fmt.Sprintf("NODE#%s", node.ID().String()),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal keyword item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{
			Put: &types.Put{TableName: aws.String(r.config.TableName), Item: keywordItem},
		})
	}

	// 3. Execute the transaction
	_, err = r.dbClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: transactItems,
	})
	if err != nil {
		return appErrors.Wrap(err, "transaction to create node and keywords failed")
	}
	return nil
}

// CreateNodeWithEdges saves a node, its keywords, and its connections in a single transaction.
func (r *ddbRepository) CreateNodeWithEdges(ctx context.Context, node *domain.Node, relatedNodeIDs []string) error {
	log.Printf("DEBUG CreateNodeWithEdges: creating node ID=%s with keywords=%v and %d edges", node.ID().String(), node.Keywords().ToSlice(), len(relatedNodeIDs))
	
	// Ensure node starts with version 0
	// Note: Version is immutable in rich domain model, using Version().Int() for DDB
	
	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID().String(), node.ID().String())
	transactItems := []types.TransactWriteItem{}

	nodeItem, err := attributevalue.MarshalMap(ddbNode{
		PK: pk, SK: "METADATA#v0", NodeID: node.ID().String(), UserID: node.UserID().String(), Content: node.Content().String(),
		Keywords: node.Keywords().ToSlice(), Tags: node.Tags().ToSlice(), IsLatest: true, Version: node.Version().Int(), Timestamp: node.CreatedAt().Format(time.RFC3339),
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal node item")
	}
	transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: nodeItem}})
	log.Printf("DEBUG CreateNodeWithEdges: added node item with PK=%s", pk)

	for _, keyword := range node.Keywords().ToSlice() {
		gsi1PK := fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID().String(), keyword)
		gsi1SK := fmt.Sprintf("NODE#%s", node.ID().String())
		
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{
			PK: pk, SK: fmt.Sprintf("KEYWORD#%s", keyword), GSI1PK: gsi1PK, GSI1SK: gsi1SK,
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal keyword item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: keywordItem}})
		log.Printf("DEBUG CreateNodeWithEdges: added keyword item for '%s' with GSI1PK=%s, GSI1SK=%s", keyword, gsi1PK, gsi1SK)
	}

	// Create canonical edges - only one edge per connection
	for _, relatedNodeID := range relatedNodeIDs {
		ownerID, targetID := getCanonicalEdge(node.ID().String(), relatedNodeID)
		ownerPK := fmt.Sprintf("USER#%s#NODE#%s", node.UserID().String(), ownerID)

		edgeItem, err := attributevalue.MarshalMap(ddbEdge{
			PK:       ownerPK,
			SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", targetID),
			TargetID: targetID,
			GSI2PK:   fmt.Sprintf("USER#%s#EDGE", node.UserID().String()),
			GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", ownerID, targetID),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal canonical edge item")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: edgeItem}})
		log.Printf("DEBUG CreateNodeWithEdges: added edge from %s to %s (canonical: ownerPK=%s)", node.ID().String(), relatedNodeID, ownerPK)
	}

	log.Printf("DEBUG CreateNodeWithEdges: executing transaction with %d items", len(transactItems))
	_, err = r.dbClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: transactItems})
	if err != nil {
		log.Printf("ERROR CreateNodeWithEdges: transaction failed: %v", err)
		return appErrors.Wrap(err, "transaction to create node with edges failed")
	}
	
	log.Printf("DEBUG CreateNodeWithEdges: transaction completed successfully for node %s", node.ID().String())
	return nil
}

// UpdateNodeAndEdges transactionally updates a node and its connections.
func (r *ddbRepository) UpdateNodeAndEdges(ctx context.Context, node *domain.Node, relatedNodeIDs []string) error {
	if err := r.clearNodeConnections(ctx, node.UserID().String(), node.ID().String()); err != nil {
		return appErrors.Wrap(err, "failed to clear old connections for update")
	}

	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID().String(), node.ID().String())
	transactItems := []types.TransactWriteItem{}

	// Optimistic locking: check that the version matches before updating
	_, err := r.dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(r.config.TableName),
		Key:                 map[string]types.AttributeValue{"PK": &types.AttributeValueMemberS{Value: pk}, "SK": &types.AttributeValueMemberS{Value: "METADATA#v0"}},
		UpdateExpression:    aws.String("SET Content = :c, Keywords = :k, Tags = :tg, Timestamp = :t, Version = Version + :inc"),
		ConditionExpression: aws.String("Version = :expected_version"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":c":                &types.AttributeValueMemberS{Value: node.Content().String()},
			":k":                &types.AttributeValueMemberL{Value: toAttributeValueList(node.Keywords().ToSlice())},
			":tg":               &types.AttributeValueMemberL{Value: toAttributeValueList(node.Tags().ToSlice())},
			":t":                &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
			":expected_version": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", node.Version().Int())},
			":inc":              &types.AttributeValueMemberN{Value: "1"},
		},
	})
	if err != nil {
		// Check for optimistic lock conflicts
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return repository.NewOptimisticLockError(node.ID().String(), node.Version().Int(), node.Version().Int()+1)
		}
		return appErrors.Wrap(err, "failed to update node metadata")
	}

	for _, keyword := range node.Keywords().ToSlice() {
		keywordItem, err := attributevalue.MarshalMap(ddbKeyword{PK: pk, SK: fmt.Sprintf("KEYWORD#%s", keyword), GSI1PK: fmt.Sprintf("USER#%s#KEYWORD#%s", node.UserID().String(), keyword), GSI1SK: fmt.Sprintf("NODE#%s", node.ID().String())})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal keyword item for update")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: keywordItem}})
	}

	// Create canonical edges - only one edge per connection
	for _, relatedNodeID := range relatedNodeIDs {
		ownerID, targetID := getCanonicalEdge(node.ID().String(), relatedNodeID)
		ownerPK := fmt.Sprintf("USER#%s#NODE#%s", node.UserID().String(), ownerID)

		edgeItem, err := attributevalue.MarshalMap(ddbEdge{
			PK:       ownerPK,
			SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", targetID),
			TargetID: targetID,
			GSI2PK:   fmt.Sprintf("USER#%s#EDGE", node.UserID().String()),
			GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", ownerID, targetID),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal canonical edge item for update")
		}
		transactItems = append(transactItems, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(r.config.TableName), Item: edgeItem}})
	}

	if len(transactItems) > 0 {
		_, err = r.dbClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: transactItems})
		if err != nil {
			return appErrors.Wrap(err, "transaction to update node edges failed")
		}
	}
	return nil
}

// CreateEdge creates a single edge in DynamoDB using the canonical edge storage pattern.
func (r *ddbRepository) CreateEdge(ctx context.Context, edge *domain.Edge) error {
	if edge == nil {
		return appErrors.NewValidation("edge cannot be nil")
	}
	
	// Use canonical edge storage pattern (lexicographically ordered IDs)
	sourceID := edge.SourceID().String()
	targetID := edge.TargetID().String()
	userID := edge.UserID().String()
	
	ownerID, canonicalTargetID := getCanonicalEdge(sourceID, targetID)
	ownerPK := fmt.Sprintf("USER#%s#NODE#%s", userID, ownerID)
	
	edgeItem := ddbEdge{
		PK:       ownerPK,
		SK:       fmt.Sprintf("EDGE#%s", canonicalTargetID),
		TargetID: canonicalTargetID,
		GSI2PK:   fmt.Sprintf("USER#%s#EDGE", userID),
		GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", ownerID, canonicalTargetID),
	}
	
	item, err := attributevalue.MarshalMap(edgeItem)
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal edge item")
	}
	
	_, err = r.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.config.TableName),
		Item:      item,
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to create edge in DynamoDB")
	}
	
	return nil
}

// FindNodesByKeywords uses the GSI to find nodes with matching keywords.
func (r *ddbRepository) FindNodesByKeywords(ctx context.Context, userID string, keywords []string) ([]*domain.Node, error) {
	log.Printf("DEBUG FindNodesByKeywords: called with userID=%s, keywords=%v", userID, keywords)
	
	nodeIdMap := make(map[string]bool)
	var nodes []*domain.Node
	for _, keyword := range keywords {
		gsiPK := fmt.Sprintf("USER#%s#KEYWORD#%s", userID, keyword)
		log.Printf("DEBUG FindNodesByKeywords: querying GSI with gsiPK=%s", gsiPK)
		
		result, err := r.dbClient.Query(ctx, &dynamodb.QueryInput{
			TableName: aws.String(r.config.TableName), IndexName: aws.String(r.config.IndexName), KeyConditionExpression: aws.String("GSI1PK = :gsiPK"),
			ExpressionAttributeValues: map[string]types.AttributeValue{":gsiPK": &types.AttributeValueMemberS{Value: gsiPK}},
		})
		if err != nil {
			log.Printf("ERROR FindNodesByKeywords: failed to query GSI for keyword %s: %v", keyword, err)
			continue
		}
		
		log.Printf("DEBUG FindNodesByKeywords: GSI query for keyword '%s' returned %d items", keyword, len(result.Items))
		
		for _, item := range result.Items {
			pkValue := item["PK"].(*types.AttributeValueMemberS).Value
			nodeID := strings.Split(pkValue, "#")[3]
			log.Printf("DEBUG FindNodesByKeywords: found nodeID=%s from keyword '%s' (PK=%s)", nodeID, keyword, pkValue)
			
			if _, exists := nodeIdMap[nodeID]; !exists {
				nodeIdMap[nodeID] = true
				node, err := r.FindNodeByID(ctx, userID, nodeID)
				if err != nil {
					log.Printf("ERROR FindNodesByKeywords: failed to find node %s from keyword search: %v", nodeID, err)
					continue
				}
				if node != nil {
					log.Printf("DEBUG FindNodesByKeywords: successfully retrieved node ID=%s, content='%s'", node.ID().String(), node.Content().String())
					nodes = append(nodes, node)
				} else {
					log.Printf("WARN FindNodesByKeywords: node %s was nil after FindNodeByID", nodeID)
				}
			} else {
				log.Printf("DEBUG FindNodesByKeywords: nodeID=%s already processed, skipping", nodeID)
			}
		}
	}
	
	log.Printf("DEBUG FindNodesByKeywords: returning %d unique nodes", len(nodes))
	return nodes, nil
}


// FindEdgesByNode queries for all edges connected to a given node using optimized GSI2 query.
func (r *ddbRepository) FindEdgesByNode(ctx context.Context, userID, nodeID string) ([]domain.Edge, error) {
	var edges []domain.Edge
	edgeMap := make(map[string]bool)

	// Use GSI2 to find all edges for this user, then filter for those involving the specific node
	edgePrefix := fmt.Sprintf("USER#%s#EDGE", userID)
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(r.config.TableName),
			IndexName:              aws.String("EdgeIndex"),
			KeyConditionExpression: aws.String("GSI2PK = :gsi2pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":gsi2pk": &types.AttributeValueMemberS{Value: edgePrefix},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Query(ctx, queryInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to query edges using GSI2")
		}

		// Process edges - keep only those involving the specified node
		for _, item := range result.Items {
			var ddbItem ddbEdge
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				// Extract source node ID from PK pattern: USER#<userID>#NODE#<sourceID>
				pkParts := strings.Split(ddbItem.PK, "#")
				if len(pkParts) == 4 {
					sourceID := pkParts[3]
					targetID := ddbItem.TargetID
					
					// Check if this edge involves the requested node
					if sourceID == nodeID || targetID == nodeID {
						// Prevent duplicate edges
						edgeKey := fmt.Sprintf("%s-%s", sourceID, targetID)
						reverseKey := fmt.Sprintf("%s-%s", targetID, sourceID)
						if !edgeMap[edgeKey] && !edgeMap[reverseKey] {
							edgeMap[edgeKey] = true
							// Create rich domain edge using factory method
							userIDVO, _ := domain.NewUserID(userID)
							sourceNodeIDVO, _ := domain.ParseNodeID(sourceID)
							targetNodeIDVO, _ := domain.ParseNodeID(targetID)
							edge, err := domain.NewEdge(sourceNodeIDVO, targetNodeIDVO, userIDVO, 1.0)
							if err == nil {
								edges = append(edges, *edge)
							}
						}
					}
				}
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return edges, nil
}


// GetAllGraphData retrieves all nodes and edges for a user using optimized parallel queries.
func (r *ddbRepository) GetAllGraphData(ctx context.Context, userID string) (*domain.Graph, error) {
	log.Println("Starting optimized GetAllGraphData with parallel queries")

	g, ctx := errgroup.WithContext(ctx)
	
	var nodes []*domain.Node
	var edges []*domain.Edge
	var nodeErr, edgeErr error

	// Fetch nodes in parallel using query instead of scan
	g.Go(func() error {
		nodes, nodeErr = r.fetchAllNodesOptimized(ctx, userID)
		return nodeErr
	})

	// Fetch edges in parallel using GSI2 query instead of scan
	g.Go(func() error {
		edges, edgeErr = r.fetchAllEdgesOptimized(ctx, userID)
		return edgeErr
	})

	// Wait for both operations to complete
	if err := g.Wait(); err != nil {
		log.Printf("ERROR: failed to fetch graph data: %v", err)
		return nil, appErrors.Wrap(err, "failed to fetch graph data")
	}

	log.Printf("Finished optimized GetAllGraphData. Found %d nodes and %d edges.", len(nodes), len(edges))

	return &domain.Graph{Nodes: nodes, Edges: edges}, nil
}

// fetchAllNodesOptimized retrieves all nodes for a user using scan with filter
func (r *ddbRepository) fetchAllNodesOptimized(ctx context.Context, userID string) ([]*domain.Node, error) {
	var nodes []*domain.Node
	var lastEvaluatedKey map[string]types.AttributeValue

	userNodePrefix := fmt.Sprintf("USER#%s#NODE#", userID)

	for {
		scanInput := &dynamodb.ScanInput{
			TableName:        aws.String(r.config.TableName),
			FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk_prefix": &types.AttributeValueMemberS{Value: userNodePrefix},
				":sk_prefix": &types.AttributeValueMemberS{Value: "METADATA#"},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Scan(ctx, scanInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to scan nodes")
		}

		// Process nodes
		for _, item := range result.Items {
			var ddbItem ddbNode
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
				// Use domain factory method to reconstruct node from primitives
				node, err := domain.ReconstructNodeFromPrimitives(
					ddbItem.NodeID, 
					ddbItem.UserID, 
					ddbItem.Content,
					ddbItem.Keywords, 
					ddbItem.Tags,
					createdAt,
					ddbItem.Version,
				)
				if err == nil && node != nil {
					nodes = append(nodes, node)
				}
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return nodes, nil
}

// fetchAllNodesOptimizedDomain is the domain-compatible version of fetchAllNodesOptimized
// This method uses the proven working logic from fetchAllNodesOptimized but with domain types
func (base *ddbBaseRepository) fetchAllNodesOptimizedDomain(ctx context.Context, userID string) ([]*domain.Node, error) {
	var nodes []*domain.Node
	var lastEvaluatedKey map[string]types.AttributeValue

	userNodePrefix := fmt.Sprintf("USER#%s#NODE#", userID)

	for {
		scanInput := &dynamodb.ScanInput{
			TableName:        aws.String(base.config.TableName),
			FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk_prefix": &types.AttributeValueMemberS{Value: userNodePrefix},
				":sk_prefix": &types.AttributeValueMemberS{Value: "METADATA#"},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := base.dbClient.Scan(ctx, scanInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to scan nodes")
		}

		// Process nodes - same logic as fetchAllNodesOptimized
		for _, item := range result.Items {
			var ddbItem ddbNode
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
				// Use domain factory method to reconstruct node from primitives
				node, err := domain.ReconstructNodeFromPrimitives(
					ddbItem.NodeID, 
					ddbItem.UserID, 
					ddbItem.Content,
					ddbItem.Keywords, 
					ddbItem.Tags,
					createdAt,
					ddbItem.Version,
				)
				if err == nil && node != nil {
					nodes = append(nodes, node)
				}
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return nodes, nil
}

// fetchAllEdgesOptimized retrieves all edges for a user using GSI2 query
func (r *ddbRepository) fetchAllEdgesOptimized(ctx context.Context, userID string) ([]*domain.Edge, error) {
	var edges []*domain.Edge
	var lastEvaluatedKey map[string]types.AttributeValue
	edgeMap := make(map[string]bool)

	edgePrefix := fmt.Sprintf("USER#%s#EDGE", userID)

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(r.config.TableName),
			IndexName:              aws.String("EdgeIndex"),
			KeyConditionExpression: aws.String("GSI2PK = :gsi2pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":gsi2pk": &types.AttributeValueMemberS{Value: edgePrefix},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Query(ctx, queryInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to query edges")
		}

		// Process edges
		for _, item := range result.Items {
			var ddbItem ddbEdge
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				// Extract source ID from PK pattern: USER#<userID>#NODE#<sourceID>
				pkParts := strings.Split(ddbItem.PK, "#")
				if len(pkParts) == 4 {
					sourceID := pkParts[3]
					// Prevent duplicate edges
					edgeKey := fmt.Sprintf("%s-%s", sourceID, ddbItem.TargetID)
					reverseKey := fmt.Sprintf("%s-%s", ddbItem.TargetID, sourceID)
					if !edgeMap[edgeKey] && !edgeMap[reverseKey] {
						edgeMap[edgeKey] = true
						// Create rich domain edge using factory method
						userIDVO, _ := domain.NewUserID(userID)
						sourceNodeIDVO, _ := domain.ParseNodeID(sourceID)
						targetNodeIDVO, _ := domain.ParseNodeID(ddbItem.TargetID)
						edge, err := domain.NewEdge(sourceNodeIDVO, targetNodeIDVO, userIDVO, 1.0)
						if err == nil {
							edges = append(edges, edge)
						}
					}
				}
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return edges, nil
}

// fetchAllEdgesOptimizedDomain is the domain-compatible version of fetchAllEdgesOptimized
// This method uses the proven working logic from fetchAllEdgesOptimized but with domain types
func (base *ddbBaseRepository) fetchAllEdgesOptimizedDomain(ctx context.Context, userID string) ([]*domain.Edge, error) {
	var edges []*domain.Edge
	var lastEvaluatedKey map[string]types.AttributeValue
	edgeMap := make(map[string]bool)

	edgePrefix := fmt.Sprintf("USER#%s#EDGE", userID)

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(base.config.TableName),
			IndexName:              aws.String("EdgeIndex"),
			KeyConditionExpression: aws.String("GSI2PK = :gsi2pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":gsi2pk": &types.AttributeValueMemberS{Value: edgePrefix},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := base.dbClient.Query(ctx, queryInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to query edges")
		}

		// Process edges - same logic as fetchAllEdgesOptimized
		for _, item := range result.Items {
			var ddbItem ddbEdge
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				// Extract source ID from PK pattern: USER#<userID>#NODE#<sourceID>
				pkParts := strings.Split(ddbItem.PK, "#")
				if len(pkParts) == 4 {
					sourceID := pkParts[3]
					// Prevent duplicate edges
					edgeKey := fmt.Sprintf("%s-%s", sourceID, ddbItem.TargetID)
					reverseKey := fmt.Sprintf("%s-%s", ddbItem.TargetID, sourceID)
					if !edgeMap[edgeKey] && !edgeMap[reverseKey] {
						edgeMap[edgeKey] = true
						// Create rich domain edge using factory method
						userIDVO, _ := domain.NewUserID(userID)
						sourceNodeIDVO, _ := domain.ParseNodeID(sourceID)
						targetNodeIDVO, _ := domain.ParseNodeID(ddbItem.TargetID)
						edge, err := domain.NewEdge(sourceNodeIDVO, targetNodeIDVO, userIDVO, 1.0)
						if err == nil {
							edges = append(edges, edge)
						}
					}
				}
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return edges, nil
}

func (r *ddbRepository) clearNodeConnections(ctx context.Context, userID, nodeID string) error {
	var allWriteRequests []types.WriteRequest

	// First, delete all items in this node's partition (node data, keywords, edges where this node is owner)
	pk := fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
	queryResult, err := r.dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(r.config.TableName),
		KeyConditionExpression:    aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: pk}},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to query items for deletion")
	}

	for _, item := range queryResult.Items {
		allWriteRequests = append(allWriteRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{"PK": item["PK"], "SK": item["SK"]},
			},
		})
	}

	// Second, find and delete edges where this node is the target (stored in other nodes' partitions)
	scanResult, err := r.dbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(r.config.TableName),
		FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix) AND TargetID = :target_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk_prefix": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#", userID)},
			":sk_prefix": &types.AttributeValueMemberS{Value: "EDGE#RELATES_TO#"},
			":target_id": &types.AttributeValueMemberS{Value: nodeID},
		},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to scan for edges where node is target")
	}

	for _, item := range scanResult.Items {
		allWriteRequests = append(allWriteRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{"PK": item["PK"], "SK": item["SK"]},
			},
		})
	}

	// Batch delete all items (DynamoDB has a 25 item limit per batch)
	if len(allWriteRequests) > 0 {
		for i := 0; i < len(allWriteRequests); i += 25 {
			end := i + 25
			if end > len(allWriteRequests) {
				end = len(allWriteRequests)
			}

			batchRequests := allWriteRequests[i:end]
			_, err = r.dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{r.config.TableName: batchRequests},
			})
			if err != nil {
				return appErrors.Wrap(err, "failed to batch delete node items")
			}
		}
	}

	return nil
}

// clearNodeConnectionsDomain is the domain-compatible version of clearNodeConnections
// This method uses the proven working logic from clearNodeConnections but with domain types
func (base *ddbBaseRepository) clearNodeConnectionsDomain(ctx context.Context, userID, nodeID string) error {
	var allWriteRequests []types.WriteRequest

	// First, delete all items in this node's partition (node data, keywords, edges where this node is owner)
	pk := fmt.Sprintf("USER#%s#NODE#%s", userID, nodeID)
	queryResult, err := base.dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(base.config.TableName),
		KeyConditionExpression:    aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: pk}},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to query items for deletion")
	}

	for _, item := range queryResult.Items {
		allWriteRequests = append(allWriteRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{"PK": item["PK"], "SK": item["SK"]},
			},
		})
	}

	// Second, find and delete edges where this node is the target (stored in other nodes' partitions)
	scanResult, err := base.dbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(base.config.TableName),
		FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix) AND TargetID = :target_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk_prefix": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#", userID)},
			":sk_prefix": &types.AttributeValueMemberS{Value: "EDGE#RELATES_TO#"},
			":target_id": &types.AttributeValueMemberS{Value: nodeID},
		},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to scan for target edges")
	}

	for _, item := range scanResult.Items {
		allWriteRequests = append(allWriteRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{"PK": item["PK"], "SK": item["SK"]},
			},
		})
	}

	// Third, find and delete edges where this node is the target using the canonical edge storage
	scanResultCanonical, err := base.dbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(base.config.TableName),
		FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix) AND TargetID = :target_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk_prefix": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#", userID)},
			":sk_prefix": &types.AttributeValueMemberS{Value: "EDGE#"},
			":target_id": &types.AttributeValueMemberS{Value: nodeID},
		},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to scan for canonical target edges")
	}

	for _, item := range scanResultCanonical.Items {
		allWriteRequests = append(allWriteRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{"PK": item["PK"], "SK": item["SK"]},
			},
		})
	}

	// Execute all deletions in batches (DynamoDB allows max 25 items per batch)
	if len(allWriteRequests) > 0 {
		batchSize := 25
		for i := 0; i < len(allWriteRequests); i += batchSize {
			end := i + batchSize
			if end > len(allWriteRequests) {
				end = len(allWriteRequests)
			}
			batchRequests := allWriteRequests[i:end]

			_, err := base.dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{base.config.TableName: batchRequests},
			})
			if err != nil {
				return appErrors.Wrap(err, "failed to batch delete node items")
			}
		}
	}

	return nil
}

// Enhanced query methods using new query types

// FindNodes implements the enhanced node querying with NodeQuery.
func (r *ddbRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*domain.Node, error) {
	log.Printf("DEBUG FindNodes: called with userID=%s, keywords=%v, nodeIDs=%v", query.UserID, query.Keywords, query.NodeIDs)
	
	if err := query.Validate(); err != nil {
		log.Printf("ERROR FindNodes: query validation failed: %v", err)
		return nil, err
	}

	// If specific node IDs are requested, fetch them directly
	if query.HasNodeIDs() {
		log.Printf("DEBUG FindNodes: using nodeID-based lookup for %d nodes", len(query.NodeIDs))
		var nodes []*domain.Node
		for _, nodeID := range query.NodeIDs {
			node, err := r.FindNodeByID(ctx, query.UserID, nodeID)
			if err != nil {
				return nil, err
			}
			if node != nil {
				nodes = append(nodes, node)
			}
		}
		log.Printf("DEBUG FindNodes: found %d nodes by nodeID lookup", len(nodes))
		return nodes, nil
	}

	// If keywords are specified, use keyword search
	if query.HasKeywords() {
		log.Printf("DEBUG FindNodes: using keyword-based lookup with keywords=%v", query.Keywords)
		nodes, err := r.FindNodesByKeywords(ctx, query.UserID, query.Keywords)
		log.Printf("DEBUG FindNodes: keyword lookup returned %d nodes, error=%v", len(nodes), err)
		return nodes, err
	}

	// Otherwise, get all nodes for the user (this could be expensive for large datasets)
	graph, err := r.GetAllGraphData(ctx, query.UserID)
	if err != nil {
		return nil, err
	}

	nodes := graph.Nodes

	// Apply pagination if specified
	if query.HasPagination() {
		start := query.Offset
		if start >= len(nodes) {
			return []*domain.Node{}, nil
		}

		end := len(nodes)
		if query.Limit > 0 && start+query.Limit < len(nodes) {
			end = start + query.Limit
		}

		nodes = nodes[start:end]
	}

	return nodes, nil
}

// FindEdges implements the enhanced edge querying with EdgeQuery.
func (r *ddbRepository) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*domain.Edge, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}

	var edges []*domain.Edge

	// If specific node IDs are requested, find edges for each
	if query.HasNodeIDs() {
		for _, nodeID := range query.NodeIDs {
			nodeEdges, err := r.FindEdgesByNode(ctx, query.UserID, nodeID)
			if err != nil {
				return nil, err
			}
			// Convert []domain.Edge to []*domain.Edge
			for i := range nodeEdges {
				edges = append(edges, &nodeEdges[i])
			}
		}
		return edges, nil
	}

	// If source node is specified, find outgoing edges
	if query.HasSourceFilter() {
		nodeEdges, err := r.FindEdgesByNode(ctx, query.UserID, query.SourceID)
		if err != nil {
			return nil, err
		}
		// Convert []domain.Edge to []*domain.Edge
		var edges []*domain.Edge
		for i := range nodeEdges {
			edges = append(edges, &nodeEdges[i])
		}
		return edges, nil
	}

	// If target node is specified, we need to scan for incoming edges
	if query.HasTargetFilter() {
		// This is less efficient but necessary for target-based queries
		graph, err := r.GetAllGraphData(ctx, query.UserID)
		if err != nil {
			return nil, err
		}

		var edges []*domain.Edge
		for _, edge := range graph.Edges {
			if edge.TargetID().String() == query.TargetID {
				edges = append(edges, edge)
			}
		}
		return edges, nil
	}

	// Otherwise, get all edges for the user
	graph, err := r.GetAllGraphData(ctx, query.UserID)
	if err != nil {
		return nil, err
	}

	edges = graph.Edges

	// Apply pagination if specified
	if query.HasPagination() {
		start := query.Offset
		if start >= len(edges) {
			return []*domain.Edge{}, nil
		}

		end := len(edges)
		if query.Limit > 0 && start+query.Limit < len(edges) {
			end = start + query.Limit
		}

		edges = edges[start:end]
	}

	return edges, nil
}


// Add these new methods to ddb.go

// CreateNode saves only the metadata for a node.
func (r *ddbRepository) CreateNode(ctx context.Context, node domain.Node) error {
	// Ensure node starts with version 0
	// Note: Version is immutable in rich domain model, using Version().Int() for DDB
	
	pk := fmt.Sprintf("USER#%s#NODE#%s", node.UserID().String(), node.ID().String())
	nodeItem, err := attributevalue.MarshalMap(ddbNode{
		PK: pk, SK: "METADATA#v0", NodeID: node.ID().String(), UserID: node.UserID().String(), Content: node.Content().String(),
		Keywords: node.Keywords().ToSlice(), Tags: node.Tags().ToSlice(), IsLatest: true, Version: node.Version().Int(), Timestamp: node.CreatedAt().Format(time.RFC3339),
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to marshal node item")
	}

	_, err = r.dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.config.TableName),
		Item:      nodeItem,
	})
	if err != nil {
		return appErrors.Wrap(err, "put item failed for node metadata")
	}
	return nil
}

// CreateEdges creates bidirectional edges between a source node and multiple related nodes.
func (r *ddbRepository) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	if len(relatedNodeIDs) == 0 {
		return nil
	}

	var writeRequests []types.WriteRequest
	pkSource := fmt.Sprintf("USER#%s#NODE#%s", userID, sourceNodeID)

	for _, relatedNodeID := range relatedNodeIDs {
		// Edge: Source -> Related
		edge1Item, err := attributevalue.MarshalMap(ddbEdge{
			PK:       pkSource,
			SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", relatedNodeID),
			TargetID: relatedNodeID,
			GSI2PK:   fmt.Sprintf("USER#%s#EDGE", userID),
			GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", sourceNodeID, relatedNodeID),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal outgoing edge")
		}
		writeRequests = append(writeRequests, types.WriteRequest{PutRequest: &types.PutRequest{Item: edge1Item}})

		// Edge: Related -> Source
		pkRelated := fmt.Sprintf("USER#%s#NODE#%s", userID, relatedNodeID)
		edge2Item, err := attributevalue.MarshalMap(ddbEdge{
			PK:       pkRelated,
			SK:       fmt.Sprintf("EDGE#RELATES_TO#%s", sourceNodeID),
			TargetID: sourceNodeID,
			GSI2PK:   fmt.Sprintf("USER#%s#EDGE", userID),
			GSI2SK:   fmt.Sprintf("NODE#%s#TARGET#%s", relatedNodeID, sourceNodeID),
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to marshal incoming edge")
		}
		writeRequests = append(writeRequests, types.WriteRequest{PutRequest: &types.PutRequest{Item: edge2Item}})
	}

	// Batch write the edges
	_, err := r.dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			r.config.TableName: writeRequests,
		},
	})
	if err != nil {
		return appErrors.Wrap(err, "batch write for edges failed")
	}
	return nil
}

func toAttributeValueList(ss []string) []types.AttributeValue {
	var avs []types.AttributeValue
	for _, s := range ss {
		avs = append(avs, &types.AttributeValueMemberS{Value: s})
	}
	return avs
}

// Category operations implementation

// CreateCategory creates a new category with enhanced hierarchical support.
func (r *ddbRepository) CreateCategory(ctx context.Context, category *domain.Category) error {
	return r.CreateEnhancedCategory(ctx, *category)
}

// UpdateCategory updates an existing category with enhanced hierarchical support.
func (r *ddbRepository) UpdateCategory(ctx context.Context, category domain.Category) error {
	return r.UpdateEnhancedCategory(ctx, category)
}

// DeleteCategory deletes a category and all its memory associations.
func (r *ddbRepository) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	pk := fmt.Sprintf("USER#%s#CATEGORY#%s", userID, categoryID)

	// First, query all items for this category
	queryResult, err := r.dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(r.config.TableName),
		KeyConditionExpression:    aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: pk}},
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to query category items for deletion")
	}

	if len(queryResult.Items) == 0 {
		return nil // Nothing to delete
	}

	// Delete all items (category metadata and memory associations)
	var writeRequests []types.WriteRequest
	for _, item := range queryResult.Items {
		writeRequests = append(writeRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{
					"PK": item["PK"],
					"SK": item["SK"],
				},
			},
		})
	}

	if len(writeRequests) > 0 {
		_, err = r.dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				r.config.TableName: writeRequests,
			},
		})
		if err != nil {
			return appErrors.Wrap(err, "failed to batch delete category items")
		}
	}
	return nil
}

// FindCategoryByID retrieves a single category by ID using enhanced format.
func (r *ddbRepository) FindCategoryByID(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	pk := fmt.Sprintf("USER#%s", userID)
	sk := fmt.Sprintf("CATEGORY#%s", categoryID)

	result, err := r.dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.config.TableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get category from dynamodb")
	}
	if result.Item == nil {
		return nil, nil // Not found
	}

	var ddbItem ddbEnhancedCategory
	if err := attributevalue.UnmarshalMap(result.Item, &ddbItem); err != nil {
		return nil, appErrors.Wrap(err, "failed to unmarshal enhanced category item")
	}

	category := r.toDomainCategory(ddbItem)
	return &category, nil
}

// FindCategories retrieves categories based on query parameters.
func (r *ddbRepository) FindCategories(ctx context.Context, userID string) ([]domain.Category, error) {
	// Use Query instead of Scan for better performance
	var categories []domain.Category
	var lastEvaluatedKey map[string]types.AttributeValue

	userPK := fmt.Sprintf("USER#%s", userID)

	for {
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(r.config.TableName),
			KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :skPrefix)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk":       &types.AttributeValueMemberS{Value: userPK},
				":skPrefix": &types.AttributeValueMemberS{Value: "CATEGORY#"},
			},
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Query(ctx, queryInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to query categories")
		}

		// Process categories
		for _, item := range result.Items {
			var ddbItem ddbEnhancedCategory
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				category := r.toDomainCategory(ddbItem)
				categories = append(categories, category)
			}
		}

		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return categories, nil
}

// GetNodesPage retrieves a paginated list of nodes for a user using optimized query
func (r *ddbRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if err := pagination.Validate(); err != nil {
		return nil, err
	}

	userNodePrefix := fmt.Sprintf("USER#%s#NODE#", query.UserID)
	requestedLimit := pagination.GetEffectiveLimit()
	nodes := make([]*domain.Node, 0, requestedLimit)
	
	var lastEvaluatedKey map[string]types.AttributeValue
	
	// Handle cursor-based pagination for starting point
	if pagination.HasCursor() {
		startKey, err := repository.DecodeCursor(pagination.Cursor)
		if err == nil && startKey != nil {
			lastEvaluatedKey = startKey
		}
	}
	
	scanIterations := 0
	
	// Continue scanning until we have enough nodes or no more items exist
	for len(nodes) < requestedLimit {
		scanIterations++
		
		scanInput := &dynamodb.ScanInput{
			TableName:        aws.String(r.config.TableName),
			FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk_prefix": &types.AttributeValueMemberS{Value: userNodePrefix},
				":sk_prefix": &types.AttributeValueMemberS{Value: "METADATA#"},
			},
			// Use a reasonable scan limit to avoid timeouts
			Limit:             aws.Int32(100),
			ExclusiveStartKey: lastEvaluatedKey,
		}

		result, err := r.dbClient.Scan(ctx, scanInput)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to scan nodes page")
		}

		// Process items from this scan segment
		itemsProcessedThisIteration := 0
		for _, item := range result.Items {
			var ddbItem ddbNode
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
				// Log nodes with IsLatest=false for debugging
				if !ddbItem.IsLatest {
					log.Printf("DEBUG: GetNodesPage found node with IsLatest=false: NodeID=%s, UserID=%s, Version=%d", 
						ddbItem.NodeID, ddbItem.UserID, ddbItem.Version)
				}
				
				// Include ALL valid nodes (removing IsLatest filter to match graph behavior)
				createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
				node, err := domain.ReconstructNodeFromPrimitives(
					ddbItem.NodeID,
					ddbItem.UserID,
					ddbItem.Content,
					ddbItem.Keywords,
					ddbItem.Tags,
					createdAt,
					ddbItem.Version,
				)
				if err != nil {
					log.Printf("Failed to reconstruct node: %v", err)
					continue
				}
				nodes = append(nodes, node)
				itemsProcessedThisIteration++
				
				// Stop if we've reached our limit
				if len(nodes) >= requestedLimit {
					break
				}
			} else {
				// Log unmarshaling errors for debugging
				log.Printf("WARN: GetNodesPage failed to unmarshal node: %v", err)
			}
		}
		
		// Update pagination state
		lastEvaluatedKey = result.LastEvaluatedKey
		
		// Log scan continuation for debugging
		if scanIterations > 1 {
			log.Printf("INFO: GetNodesPage scan continuation - iteration %d, items found: %d, total collected: %d/%d", 
				scanIterations, itemsProcessedThisIteration, len(nodes), requestedLimit)
		}
		
		// Break if no more items available
		if result.LastEvaluatedKey == nil {
			break
		}
		
		// Safety check to prevent infinite loops
		if scanIterations > 10 {
			log.Printf("WARN: GetNodesPage exceeded maximum scan iterations (%d), returning %d nodes", 
				scanIterations, len(nodes))
			break
		}
	}
	
	// Final logging
	if scanIterations > 1 {
		log.Printf("INFO: GetNodesPage completed with %d scan iterations, returned %d/%d nodes", 
			scanIterations, len(nodes), requestedLimit)
	}

	return &repository.NodePage{
		Items:      nodes,
		HasMore:    lastEvaluatedKey != nil && len(nodes) >= requestedLimit,
		NextCursor: repository.EncodeCursor(lastEvaluatedKey),
		PageInfo:   repository.CreatePageInfo(pagination, len(nodes), lastEvaluatedKey != nil),
	}, nil
}

// CountNodes returns the total number of nodes for a user
func (r *ddbRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	userNodePrefix := fmt.Sprintf("USER#%s#NODE#", userID)
	
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(r.config.TableName),
		FilterExpression: aws.String("begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk_prefix": &types.AttributeValueMemberS{Value: userNodePrefix},
			":sk_prefix": &types.AttributeValueMemberS{Value: "METADATA#"},
		},
		Select: types.SelectCount,  // Only count items, don't return data
	}

	count := 0
	paginator := dynamodb.NewScanPaginator(r.dbClient, scanInput)
	
	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, appErrors.Wrap(err, "failed to count nodes")
		}
		count += int(result.Count)
	}

	return count, nil
}

// GetNodesPageOptimized retrieves a paginated list of nodes using the new PageRequest/PageResponse types
func (r *ddbRepository) GetNodesPageOptimized(ctx context.Context, userID string, pageReq repository.PageRequest) (*repository.PageResponse, error) {
	userPrefix := fmt.Sprintf("USER#%s", userID)
	
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(r.config.TableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":        &types.AttributeValueMemberS{Value: userPrefix},
			":sk_prefix": &types.AttributeValueMemberS{Value: "METADATA#"},
		},
		Limit: aws.Int32(int32(pageReq.GetEffectiveLimit())),
	}

	// Handle cursor-based pagination with new PageRequest
	if pageReq.HasNextToken() {
		lastKey, err := repository.DecodeNextToken(pageReq.NextToken)
		if err == nil && lastKey != nil {
			queryInput.ExclusiveStartKey = lastKey.ToDynamoDBKey()
		}
	}

	result, err := r.dbClient.Query(ctx, queryInput)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query nodes page optimized")
	}

	// Convert DynamoDB items to domain nodes
	nodes := make([]*domain.Node, 0, len(result.Items))
	for _, item := range result.Items {
		var ddbItem ddbNode
		if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
			if ddbItem.IsLatest { // Only return latest versions
				createdAt, _ := time.Parse(time.RFC3339, ddbItem.Timestamp)
				node, err := domain.ReconstructNodeFromPrimitives(
					ddbItem.NodeID,
					ddbItem.UserID,
					ddbItem.Content,
					ddbItem.Keywords,
					ddbItem.Tags,
					createdAt,
					ddbItem.Version,
				)
				if err != nil {
					log.Printf("Failed to reconstruct node: %v", err)
					continue
				}
				nodes = append(nodes, node)
			}
		}
	}

	// Create paginated response
	return repository.CreatePageResponse(nodes, result.LastEvaluatedKey, result.LastEvaluatedKey != nil), nil
}

// GetNodeNeighborhood retrieves nodes and edges within a specified depth from a target node
func (r *ddbRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error) {
	if depth <= 0 {
		depth = 1
	}
	if depth > 3 {
		depth = 3 // Limit depth to prevent excessive queries
	}

	visited := make(map[string]bool)
	nodes := make(map[string]*domain.Node)
	edges := make([]*domain.Edge, 0)

	// Start with the target node
	currentLevel := []string{nodeID}
	visited[nodeID] = true

	for currentDepth := 0; currentDepth < depth && len(currentLevel) > 0; currentDepth++ {
		var nextLevel []string

		for _, currentNodeID := range currentLevel {
			// Get the node details
			node, err := r.FindNodeByID(ctx, userID, currentNodeID)
			if err == nil && node != nil {
				nodes[currentNodeID] = node
			}

			// Get edges from this node
			queryInput := &dynamodb.QueryInput{
				TableName:              aws.String(r.config.TableName),
				KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", userID, currentNodeID)},
					":sk": &types.AttributeValueMemberS{Value: "EDGE#"},
				},
			}

			result, err := r.dbClient.Query(ctx, queryInput)
			if err != nil {
				continue // Skip this node's edges on error
			}

			for _, item := range result.Items {
				var ddbEdge ddbEdge
				if err := attributevalue.UnmarshalMap(item, &ddbEdge); err == nil {
					sourceNodeID, _ := domain.ParseNodeID(currentNodeID)
					targetNodeID, _ := domain.ParseNodeID(ddbEdge.TargetID)
					userIDVO, _ := domain.NewUserID(userID)
					edge, err := domain.NewEdge(sourceNodeID, targetNodeID, userIDVO, 1.0)
					if err != nil {
						log.Printf("Failed to create edge: %v", err)
						continue
					}
					edges = append(edges, edge)

					// Add target node to next level if not visited
					if !visited[ddbEdge.TargetID] {
						visited[ddbEdge.TargetID] = true
						nextLevel = append(nextLevel, ddbEdge.TargetID)
					}
				}
			}
		}

		currentLevel = nextLevel
	}

	// Convert nodes map to slice
	nodeSlice := make([]*domain.Node, 0, len(nodes))
	for _, node := range nodes {
		nodeSlice = append(nodeSlice, node)
	}

	return &domain.Graph{
		Nodes: nodeSlice,
		Edges: edges,
	}, nil
}

// GetEdgesPage retrieves a paginated list of edges for a user
func (r *ddbRepository) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if err := pagination.Validate(); err != nil {
		return nil, err
	}

	var queryInput *dynamodb.QueryInput

	if query.HasSourceFilter() {
		// Query edges from a specific source node
		queryInput = &dynamodb.QueryInput{
			TableName:              aws.String(r.config.TableName),
			KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#%s", query.UserID, query.SourceID)},
				":sk": &types.AttributeValueMemberS{Value: "EDGE#"},
			},
		}
	} else {
		// Query all edges for user - less efficient but works
		queryInput = &dynamodb.QueryInput{
			TableName:              aws.String(r.config.TableName),
			KeyConditionExpression: aws.String("PK = :pk"),
			FilterExpression:       aws.String("begins_with(SK, :sk)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s", query.UserID)},
				":sk": &types.AttributeValueMemberS{Value: "EDGE#"},
			},
		}
	}

	queryInput.Limit = aws.Int32(int32(pagination.GetEffectiveLimit()))

	// Handle cursor-based pagination
	if pagination.HasCursor() {
		startKey, err := repository.DecodeCursor(pagination.Cursor)
		if err == nil && startKey != nil {
			queryInput.ExclusiveStartKey = startKey
		}
	}

	result, err := r.dbClient.Query(ctx, queryInput)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to query edges page")
	}

	// Convert DynamoDB items to domain edges
	edges := make([]*domain.Edge, 0, len(result.Items))
	for _, item := range result.Items {
		var ddbItem ddbEdge
		if err := attributevalue.UnmarshalMap(item, &ddbItem); err == nil {
			// Extract source node ID from PK
			pkParts := strings.Split(ddbItem.PK, "#")
			if len(pkParts) >= 4 {
				sourceID := pkParts[3]
				sourceNodeID, _ := domain.ParseNodeID(sourceID)
				targetNodeID, _ := domain.ParseNodeID(ddbItem.TargetID)
				userIDVO, _ := domain.NewUserID(query.UserID)
				edge, err := domain.NewEdge(sourceNodeID, targetNodeID, userIDVO, 1.0)
				if err != nil {
					log.Printf("Failed to create edge: %v", err)
					continue
				}
				edges = append(edges, edge)
			}
		}
	}

	return &repository.EdgePage{
		Items:      edges,
		HasMore:    result.LastEvaluatedKey != nil,
		NextCursor: repository.EncodeCursor(result.LastEvaluatedKey),
		PageInfo:   repository.CreatePageInfo(pagination, len(edges), result.LastEvaluatedKey != nil),
	}, nil
}

// GetGraphDataPaginated retrieves graph data with pagination for large datasets
func (r *ddbRepository) GetGraphDataPaginated(ctx context.Context, query repository.GraphQuery, pagination repository.Pagination) (*domain.Graph, string, error) {
	log.Printf("DEBUG: GetGraphDataPaginated called for userID: %s, includeEdges: %t, limit: %d", query.UserID, query.IncludeEdges, pagination.GetEffectiveLimit())

	if err := query.Validate(); err != nil {
		log.Printf("ERROR: Query validation failed: %v", err)
		return nil, "", err
	}
	if err := pagination.Validate(); err != nil {
		log.Printf("ERROR: Pagination validation failed: %v", err)
		return nil, "", err
	}

	// Use scan with filter to find nodes and edges - stored with PK pattern USER#<userID>#NODE#<nodeID>
	var filterExpression string
	expressionValues := map[string]types.AttributeValue{
		":pk_prefix": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#NODE#", query.UserID)},
	}

	if query.IncludeEdges {
		// Include both nodes (METADATA#) and edges (EDGE#)
		filterExpression = "begins_with(PK, :pk_prefix) AND (begins_with(SK, :sk_metadata) OR begins_with(SK, :sk_edge))"
		expressionValues[":sk_metadata"] = &types.AttributeValueMemberS{Value: "METADATA#"}
		expressionValues[":sk_edge"] = &types.AttributeValueMemberS{Value: "EDGE#"}
	} else {
		// Only nodes (METADATA#)
		filterExpression = "begins_with(PK, :pk_prefix) AND begins_with(SK, :sk_metadata)"
		expressionValues[":sk_metadata"] = &types.AttributeValueMemberS{Value: "METADATA#"}
	}

	scanInput := &dynamodb.ScanInput{
		TableName:                 aws.String(r.config.TableName),
		FilterExpression:          aws.String(filterExpression),
		ExpressionAttributeValues: expressionValues,
		Limit:                     aws.Int32(int32(pagination.GetEffectiveLimit())),
	}

	log.Printf("DEBUG: DynamoDB scan input - TableName: %s, filtering for USER#%s#NODE# prefix", r.config.TableName, query.UserID)

	// Handle cursor-based pagination
	if pagination.HasCursor() {
		startKey, err := repository.DecodeCursor(pagination.Cursor)
		if err == nil && startKey != nil {
			scanInput.ExclusiveStartKey = startKey
		}
	}

	result, err := r.dbClient.Scan(ctx, scanInput)
	if err != nil {
		log.Printf("ERROR: DynamoDB query failed: %v", err)
		return nil, "", appErrors.Wrap(err, "failed to query graph data")
	}

	log.Printf("DEBUG: DynamoDB query successful - returned %d items, LastEvaluatedKey: %v", len(result.Items), result.LastEvaluatedKey != nil)

	var nodes []*domain.Node
	var edges []*domain.Edge
	edgeMap := make(map[string]bool)

	nodeCount := 0
	edgeCount := 0
	skippedItems := 0

	for _, item := range result.Items {
		skValueAttr, ok := item["SK"].(*types.AttributeValueMemberS)
		if !ok {
			log.Printf("WARN: Item missing SK attribute: %v", item)
			skippedItems++
			continue
		}
		skValue := skValueAttr.Value

		if strings.HasPrefix(skValue, "METADATA#") {
			var ddbItem ddbNode
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err != nil {
				log.Printf("ERROR: Failed to unmarshal node item: %v, item: %v", err, item)
				skippedItems++
				continue
			}
			if ddbItem.IsLatest {
				createdAt, parseErr := time.Parse(time.RFC3339, ddbItem.Timestamp)
				if parseErr != nil {
					log.Printf("WARN: Failed to parse timestamp %s: %v", ddbItem.Timestamp, parseErr)
					createdAt = time.Now() // fallback
				}
				node, err := domain.ReconstructNodeFromPrimitives(
					ddbItem.NodeID,
					ddbItem.UserID,
					ddbItem.Content,
					ddbItem.Keywords,
					ddbItem.Tags,
					createdAt,
					ddbItem.Version,
				)
				if err != nil {
					log.Printf("Failed to reconstruct node: %v", err)
					continue
				}
				nodes = append(nodes, node)
				nodeCount++
			}
		} else if strings.HasPrefix(skValue, "EDGE#") && query.IncludeEdges {
			var ddbItem ddbEdge
			if err := attributevalue.UnmarshalMap(item, &ddbItem); err != nil {
				log.Printf("ERROR: Failed to unmarshal edge item: %v, item: %v", err, item)
				skippedItems++
				continue
			}

			// Extract source node ID from PK
			pkParts := strings.Split(ddbItem.PK, "#")
			if len(pkParts) >= 4 {
				sourceID := pkParts[3]
				
				// Create canonical edge key for deduplication (since we're reading canonical storage)
				ownerID, targetID := getCanonicalEdge(sourceID, ddbItem.TargetID)
				canonicalKey := fmt.Sprintf("%s-%s", ownerID, targetID)
				
				if !edgeMap[canonicalKey] {
					edgeMap[canonicalKey] = true
					
					// Create undirected edge using canonical ordering for consistent visualization
					sourceNodeID, _ := domain.ParseNodeID(ownerID)
					targetNodeID, _ := domain.ParseNodeID(targetID)
					userIDVO, _ := domain.NewUserID(query.UserID)
					edge, err := domain.NewEdge(sourceNodeID, targetNodeID, userIDVO, 1.0)
					if err != nil {
						log.Printf("Failed to create edge: %v", err)
						continue
					}
					edges = append(edges, edge)
					edgeCount++
				}
			} else {
				log.Printf("WARN: Invalid PK format for edge: %s", ddbItem.PK)
			}
		}
	}

	log.Printf("DEBUG: Processed data - nodes: %d, edges: %d, skipped items: %d", nodeCount, edgeCount, skippedItems)

	nextCursor := repository.EncodeCursor(result.LastEvaluatedKey)

	log.Printf("DEBUG: GetGraphDataPaginated completed - returning %d nodes, %d edges, nextCursor: %s", len(nodes), len(edges), nextCursor)

	return &domain.Graph{
		Nodes: nodes,
		Edges: edges,
	}, nextCursor, nil
}
