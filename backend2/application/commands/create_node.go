package commands

import (
	"context"
	"errors"
	"strings"

	"backend2/application/ports"
	"backend2/domain/core/aggregates"
	"backend2/domain/core/entities"
	"backend2/domain/core/valueobjects"
	"go.uber.org/zap"
)

// CreateNodeCommand represents the command to create a new node
type CreateNodeCommand struct {
	NodeID   string  `json:"node_id" validate:"required"`
	UserID   string  `json:"user_id" validate:"required"`
	Title    string  `json:"title" validate:"required,min=1,max=200"`
	Content  string  `json:"content" validate:"max=50000"`
	Format   string  `json:"format" validate:"oneof=text markdown html json"`
	X        float64 `json:"x" validate:"required"`
	Y        float64 `json:"y" validate:"required"`
	Z        float64 `json:"z"`
	Tags     []string `json:"tags" validate:"max=20,dive,min=1,max=30"`
}

// CreateNodeHandler handles the CreateNodeCommand
type CreateNodeHandler struct {
	nodeRepo  ports.NodeRepository
	graphRepo ports.GraphRepository
	eventBus  ports.EventBus
	logger    *zap.Logger
}

// NewCreateNodeHandler creates a new handler instance
func NewCreateNodeHandler(
	nodeRepo ports.NodeRepository,
	graphRepo ports.GraphRepository,
	eventBus ports.EventBus,
	logger *zap.Logger,
) *CreateNodeHandler {
	return &CreateNodeHandler{
		nodeRepo:  nodeRepo,
		graphRepo: graphRepo,
		eventBus:  eventBus,
		logger:    logger,
	}
}

// Handle executes the create node command
func (h *CreateNodeHandler) Handle(ctx context.Context, cmd CreateNodeCommand) (*entities.Node, error) {
	// Create value objects
	content, err := valueobjects.NewNodeContent(
		cmd.Title,
		cmd.Content,
		valueobjects.ContentFormat(cmd.Format),
	)
	if err != nil {
		return nil, err
	}
	
	position, err := valueobjects.NewPosition3D(cmd.X, cmd.Y, cmd.Z)
	if err != nil {
		return nil, err
	}
	
	// Create the node entity
	node, err := entities.NewNode(cmd.UserID, content, position)
	if err != nil {
		return nil, err
	}
	
	// Add tags if provided
	for _, tag := range cmd.Tags {
		if err := node.AddTag(tag); err != nil {
			// Log warning but don't fail the operation
			continue
		}
	}
	
	// Get or create user's default graph (this prevents duplicates)
	graph, err := h.graphRepo.GetOrCreateDefaultGraph(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}
	
	// Set the graph ID on the node
	node.SetGraphID(graph.ID().String())
	
	// Save the node with its graph association
	if err := h.nodeRepo.Save(ctx, node); err != nil {
		return nil, err
	}
	
	// Add node to graph
	if err := graph.AddNode(node); err != nil {
		return nil, err
	}
	
	// Save the updated graph (updates node count, etc.)
	if err := h.graphRepo.Save(ctx, graph); err != nil {
		return nil, err
	}
	
	// Try to create edges to related nodes based on keywords/tags
	// Run synchronously to ensure edges are created before Lambda returns
	h.createRelatedEdges(ctx, cmd.UserID, graph.ID().String(), node)
	
	// Publish domain events
	events := node.GetUncommittedEvents()
	events = append(events, graph.GetUncommittedEvents()...)
	
	if err := h.eventBus.PublishBatch(ctx, events); err != nil {
		// Log error but don't fail - events can be retried
		// In production, you might want to use an outbox pattern
	}
	
	// Mark events as committed
	node.MarkEventsAsCommitted()
	graph.MarkEventsAsCommitted()
	
	return node, nil
}

// Validate validates the command
func (cmd CreateNodeCommand) Validate() error {
	if cmd.UserID == "" {
		return errors.New("user ID is required")
	}
	if cmd.Title == "" {
		return errors.New("title is required")
	}
	if len(cmd.Title) > MaxTitleLength {
		return errors.New("title exceeds maximum length")
	}
	if len(cmd.Content) > MaxContentLength {
		return errors.New("content exceeds maximum length")
	}
	return nil
}

const (
	MaxTitleLength   = 200
	MaxContentLength = 50000
)

// createRelatedEdges finds and creates edges to related nodes
func (h *CreateNodeHandler) createRelatedEdges(ctx context.Context, userID, graphID string, newNode *entities.Node) {
	// Log the start of edge creation
	if h.logger == nil {
		h.logger = zap.NewNop() // Fallback to no-op logger if not set
	}
	h.logger.Info("Starting edge creation for new node",
		zap.String("graphID", graphID),
		zap.String("newNodeID", newNode.ID().String()),
		zap.String("userID", userID),
	)

	// Get all nodes in the same graph
	existingNodes, err := h.nodeRepo.GetByGraphID(ctx, graphID)
	if err != nil {
		h.logger.Error("Failed to get existing nodes", zap.Error(err))
		return
	}
	
	if len(existingNodes) <= 1 {
		h.logger.Info("No other nodes to connect to", zap.Int("nodeCount", len(existingNodes)))
		return // No other nodes to connect to
	}
	
	// Get the graph aggregate and ensure it has all nodes loaded
	graph, err := h.graphRepo.GetByID(ctx, aggregates.GraphID(graphID))
	if err != nil {
		h.logger.Error("Failed to get graph for edge creation", zap.Error(err))
		return
	}
	
	// Ensure all nodes are in the graph aggregate's memory
	// CRITICAL: Add the new node first
	if err := graph.AddNode(newNode); err != nil {
		h.logger.Error("Failed to add new node to graph",
			zap.String("nodeID", newNode.ID().String()),
			zap.Error(err),
		)
	}
	
	for _, node := range existingNodes {
		if err := graph.AddNode(node); err != nil {
			// Node might already be in graph, continue
			h.logger.Debug("Node already in graph or failed to add",
				zap.String("nodeID", node.ID().String()),
				zap.Error(err),
			)
		}
	}
	
	h.logger.Info("Graph loaded with nodes",
		zap.String("graphID", graphID),
		zap.Int("nodeCount", len(graph.Nodes())),
		zap.Bool("hasNewNode", graph.HasNode(newNode.ID())),
	)
	
	// Extract keywords from the new node
	newKeywords := extractKeywords(newNode.Content().Title() + " " + newNode.Content().Body())
	newTags := newNode.GetTags()
	
	// Find similar nodes and create edges
	h.logger.Info("Checking nodes for similarity", zap.Int("totalNodes", len(existingNodes)))
	edgesCreated := 0
	
	for _, existingNode := range existingNodes {
		if existingNode.ID().String() == newNode.ID().String() {
			h.logger.Debug("Skipping self node")
			continue // Skip self
		}
		
		// Calculate similarity based on keywords and tags
		existingKeywords := extractKeywords(existingNode.Content().Title() + " " + existingNode.Content().Body())
		existingTags := existingNode.GetTags()
		
		// Check for shared keywords
		sharedKeywords := findSharedStrings(newKeywords, existingKeywords)
		sharedTags := findSharedStrings(newTags, existingTags)
		
		// Create edge if there's sufficient similarity
		if len(sharedKeywords) >= 2 || len(sharedTags) >= 1 {
			h.logger.Debug("Found similarity with existing node",
				zap.String("existingNodeID", existingNode.ID().String()),
				zap.Strings("sharedKeywords", sharedKeywords),
				zap.Strings("sharedTags", sharedTags),
			)
			
			// Calculate weight based on similarity
			weight := float64(len(sharedKeywords)*2 + len(sharedTags)*3) / 10.0
			if weight > 1.0 {
				weight = 1.0
			}
			
			// Log graph state before attempting edge creation
			h.logger.Info("Graph state before edge creation",
				zap.String("graphID", graphID),
				zap.Int("nodeCount", len(graph.Nodes())),
				zap.Bool("hasNewNode", graph.HasNode(newNode.ID())),
				zap.Bool("hasExistingNode", graph.HasNode(existingNode.ID())),
			)
			
			edge, err := graph.ConnectNodes(newNode.ID(), existingNode.ID(), entities.EdgeTypeSimilar)
			if err != nil {
				h.logger.Error("Failed to create edge - detailed error", 
					zap.Error(err),
					zap.String("errorString", err.Error()),
					zap.String("sourceID", newNode.ID().String()),
					zap.String("targetID", existingNode.ID().String()),
					zap.String("graphID", graphID),
					zap.Int("graphNodeCount", len(graph.Nodes())),
				)
				continue // Edge might already exist
			}
			
			edge.Weight = weight
			edge.Metadata = map[string]interface{}{
				"auto_created": true,
				"shared_keywords": sharedKeywords,
				"shared_tags": sharedTags,
			}
			
			edgesCreated++
			h.logger.Info("Successfully created edge in graph",
				zap.String("edgeID", edge.ID),
				zap.String("sourceID", newNode.ID().String()),
				zap.String("targetID", existingNode.ID().String()),
				zap.Float64("weight", weight),
			)
		}
	}
	
	// Save the graph once after all edges have been created
	if edgesCreated > 0 {
		if err := h.graphRepo.Save(ctx, graph); err != nil {
			h.logger.Error("Failed to save graph with new edges",
				zap.Error(err),
				zap.Int("edgesCreated", edgesCreated),
			)
		} else {
			h.logger.Info("Successfully saved graph with all new edges",
				zap.String("graphID", graphID),
				zap.Int("edgesCreated", edgesCreated),
				zap.Int("totalEdges", len(graph.GetEdges())),
			)
		}
		
		// Update the graph metadata with accurate counts from the database
		if err := h.graphRepo.UpdateGraphMetadata(ctx, graphID); err != nil {
			h.logger.Error("Failed to update graph metadata counts",
				zap.Error(err),
				zap.String("graphID", graphID),
			)
		} else {
			h.logger.Info("Updated graph metadata with accurate counts",
				zap.String("graphID", graphID),
			)
		}
	}
	
	h.logger.Info("Completed edge creation", zap.Int("edgesCreated", edgesCreated))
}

// extractKeywords extracts significant words from text
func extractKeywords(text string) []string {
	// Simple keyword extraction - in production, use NLP
	words := strings.Fields(strings.ToLower(text))
	keywords := []string{}
	
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
	}
	
	for _, word := range words {
		// Clean punctuation
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		
		// Skip short words and stop words
		if len(word) > 3 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}
	
	return removeDuplicates(keywords)
}

// findSharedStrings finds common strings between two slices
func findSharedStrings(a, b []string) []string {
	shared := []string{}
	bMap := make(map[string]bool)
	
	for _, str := range b {
		bMap[str] = true
	}
	
	for _, str := range a {
		if bMap[str] {
			shared = append(shared, str)
		}
	}
	
	return shared
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(strs []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	
	for _, str := range strs {
		if !seen[str] {
			seen[str] = true
			result = append(result, str)
		}
	}
	
	return result
}