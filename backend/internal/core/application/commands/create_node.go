// Package commands contains CQRS command implementations for write operations
package commands

import (
	"context"
	"fmt"
	"strings"
	"time"
	
	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/aggregates/node"
	"brain2-backend/internal/core/domain/events"
	"brain2-backend/internal/core/domain/valueobjects"
)

// CreateNodeCommand represents a command to create a new node
type CreateNodeCommand struct {
	cqrs.BaseCommand
	Content        string   `json:"content"`
	Title          string   `json:"title"`
	Tags           []string `json:"tags"`
	CategoryIDs    []string `json:"category_ids"`
	IdempotencyKey string   `json:"idempotency_key"`
}

// GetCommandName returns the command name
func (c CreateNodeCommand) GetCommandName() string {
	return "CreateNode"
}

// Validate validates the command
func (c CreateNodeCommand) Validate() error {
	if c.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if c.Content == "" {
		return fmt.Errorf("content is required")
	}
	if len(c.Content) > 10000 {
		return fmt.Errorf("content exceeds maximum length")
	}
	if len(c.Title) > 200 {
		return fmt.Errorf("title exceeds maximum length")
	}
	if len(c.Tags) > 20 {
		return fmt.Errorf("too many tags (maximum 20)")
	}
	return nil
}

// CreateNodeResult represents the result of node creation
type CreateNodeResult struct {
	NodeID          string    `json:"node_id"`
	Version         int64     `json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	ExtractedKeywords []string `json:"extracted_keywords"`
	SuggestedConnections []string `json:"suggested_connections"`
}

// CreateNodeHandler handles the CreateNodeCommand
type CreateNodeHandler struct {
	nodeRepo      ports.NodeRepository
	eventStore    ports.EventStore
	eventBus      ports.EventBus
	uowFactory    ports.UnitOfWorkFactory
	cache         ports.Cache
	logger        ports.Logger
	metrics       ports.Metrics
}

// NewCreateNodeHandler creates a new CreateNodeHandler
func NewCreateNodeHandler(
	nodeRepo ports.NodeRepository,
	eventStore ports.EventStore,
	eventBus ports.EventBus,
	uowFactory ports.UnitOfWorkFactory,
	logger ports.Logger,
	metrics ports.Metrics,
) *CreateNodeHandler {
	return &CreateNodeHandler{
		nodeRepo:   nodeRepo,
		eventStore: eventStore,
		eventBus:   eventBus,
		uowFactory: uowFactory,
		cache:      nil, // Cache will be injected if available
		logger:     logger,
		metrics:    metrics,
	}
}

// Handle processes the CreateNodeCommand
func (h *CreateNodeHandler) Handle(ctx context.Context, cmd cqrs.Command) error {
	command, ok := cmd.(*CreateNodeCommand)
	if !ok {
		return fmt.Errorf("invalid command type")
	}
	
	// Start unit of work
	uow, err := h.uowFactory.Create(ctx)
	if err != nil {
		return fmt.Errorf("failed to create unit of work: %w", err)
	}
	defer uow.Rollback()
	
	// Check idempotency
	if command.IdempotencyKey != "" {
		exists, err := h.checkIdempotency(ctx, command.IdempotencyKey)
		if err != nil {
			return err
		}
		if exists {
			h.logger.Info("Idempotent request detected, skipping",
				ports.Field{Key: "idempotency_key", Value: command.IdempotencyKey})
			return nil
		}
	}
	
	// Create value objects
	nodeID := valueobjects.NewNodeID("")
	userID := valueobjects.NewUserID(command.UserID)
	content := valueobjects.NewContent(command.Content)
	title := valueobjects.NewTitle(command.Title)
	tags := valueobjects.NewTags(command.Tags)
	
	// Create aggregate
	aggregate, err := node.NewAggregate(nodeID, userID, content, title, tags)
	if err != nil {
		h.metrics.IncrementCounter("node.creation.failed",
			ports.Tag{Key: "reason", Value: "validation"})
		return fmt.Errorf("failed to create node aggregate: %w", err)
	}
	
	// Add categories if specified
	for _, categoryID := range command.CategoryIDs {
		if err := aggregate.Categorize(categoryID); err != nil {
			h.logger.Warn("Failed to categorize node",
				ports.Field{Key: "category_id", Value: categoryID},
				ports.Field{Key: "error", Value: err.Error()})
		}
	}
	
	// Get uncommitted events
	domainEvents := aggregate.GetUncommittedEvents()
	
	// Save to event store
	if err := h.eventStore.SaveEvents(ctx, aggregate.GetID(), domainEvents, 0); err != nil {
		h.metrics.IncrementCounter("node.creation.failed",
			ports.Tag{Key: "reason", Value: "event_store"})
		return fmt.Errorf("failed to save events: %w", err)
	}
	
	// Save aggregate to repository
	if err := uow.NodeRepository().Save(ctx, aggregate); err != nil {
		h.metrics.IncrementCounter("node.creation.failed",
			ports.Tag{Key: "reason", Value: "repository"})
		return fmt.Errorf("failed to save node: %w", err)
	}
	
	// Commit unit of work
	if err := uow.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	
	// Mark events as committed
	aggregate.MarkEventsAsCommitted()
	
	// Edge creation is handled asynchronously via EventBridge/connect-node Lambda
	// This provides better scalability for users with thousands of memories
	
	// Invalidate cache for user's nodes and graph (synchronous to ensure consistency)
	if h.cache != nil {
		h.invalidateUserCache(ctx, command.UserID)
	}
	
	// Publish events synchronously to ensure NodeCreated event triggers edge creation
	// Use keywords from the aggregate - these are already properly extracted
	aggregateKeywords := aggregate.GetKeywords()
	
	// Create NodeCreated event with keywords
	nodeCreatedEvent := events.NewNodeCreatedEvent(
		aggregate.GetID(),
		command.UserID,
		aggregate.GetContent(),
		aggregate.GetTitle(),
	)
	nodeCreatedEvent.Keywords = aggregateKeywords
	
	// Publish the NodeCreated event to trigger connect-node Lambda
	if err := h.eventBus.Publish(ctx, nodeCreatedEvent); err != nil {
		h.logger.Error("Failed to publish NodeCreated event", err,
			ports.Field{Key: "node_id", Value: aggregate.GetID()})
	}
	
	// Publish other domain events asynchronously
	go h.publishEvents(context.Background(), domainEvents)
	
	// Record metrics
	h.metrics.IncrementCounter("node.created",
		ports.Tag{Key: "user_id", Value: command.UserID},
		ports.Tag{Key: "has_tags", Value: fmt.Sprintf("%v", len(command.Tags) > 0)})
	
	h.logger.Info("Node created successfully",
		ports.Field{Key: "node_id", Value: aggregate.GetID()},
		ports.Field{Key: "user_id", Value: command.UserID})
	
	return nil
}

// CanHandle checks if this handler can handle the command
func (h *CreateNodeHandler) CanHandle(cmd cqrs.Command) bool {
	_, ok := cmd.(*CreateNodeCommand)
	return ok
}

// checkIdempotency checks if a request with the given key was already processed
func (h *CreateNodeHandler) checkIdempotency(ctx context.Context, key string) (bool, error) {
	// Implementation would check an idempotency store
	// For now, return false to indicate not processed
	return false, nil
}

// extractKeywords extracts relevant keywords from content and tags for edge creation
func (h *CreateNodeHandler) extractKeywords(content string, tags []string) []string {
	keywordMap := make(map[string]bool)
	
	// Priority 1: Add all user-provided tags (highest relevance)
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized != "" {
			keywordMap[normalized] = true
		}
	}
	
	// Priority 2: Extract meaningful words from content
	// Convert to lowercase and split by spaces and punctuation
	content = strings.ToLower(content)
	// Replace common punctuation with spaces for better word extraction
	for _, punct := range []string{".", ",", "!", "?", ";", ":", "\"", "'", "(", ")", "[", "]", "{", "}"} {
		content = strings.ReplaceAll(content, punct, " ")
	}
	
	// Common stop words to filter out
	stopWords := map[string]bool{
		"the": true, "and": true, "is": true, "it": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "at": true,
		"with": true, "by": true, "from": true, "an": true, "as": true,
		"or": true, "but": true, "not": true, "this": true, "that": true,
		"was": true, "will": true, "are": true, "been": true, "have": true,
		"had": true, "were": true, "said": true, "each": true, "which": true,
		"she": true, "their": true, "would": true, "there": true, "could": true,
		"only": true, "other": true, "than": true, "when": true,
		"make": true, "made": true, "after": true, "also": true, "before": true,
	}
	
	// Extract words and count frequency
	wordFreq := make(map[string]int)
	words := strings.Fields(content)
	for _, word := range words {
		word = strings.TrimSpace(word)
		// Keep words that are 4+ characters and not stop words
		if len(word) >= 4 && !stopWords[word] {
			wordFreq[word]++
		}
	}
	
	// Sort words by frequency and take top keywords
	type wordCount struct {
		word  string
		count int
	}
	var sortedWords []wordCount
	for word, count := range wordFreq {
		sortedWords = append(sortedWords, wordCount{word, count})
	}
	
	// Sort by count (descending)
	for i := 0; i < len(sortedWords); i++ {
		for j := i + 1; j < len(sortedWords); j++ {
			if sortedWords[j].count > sortedWords[i].count {
				sortedWords[i], sortedWords[j] = sortedWords[j], sortedWords[i]
			}
		}
	}
	
	// Add top frequent words to keywords (limit to avoid too many)
	maxContentKeywords := 10 - len(tags) // Reserve space for tags
	if maxContentKeywords < 5 {
		maxContentKeywords = 5
	}
	
	for i, wc := range sortedWords {
		if i >= maxContentKeywords {
			break
		}
		// Add meaningful words - relaxed criteria for better edge creation
		// Include words that are 5+ chars (even if they appear once)
		if wc.count > 1 || len(wc.word) >= 5 {
			keywordMap[wc.word] = true
		}
	}
	
	// Convert map to slice
	var keywords []string
	for keyword := range keywordMap {
		keywords = append(keywords, keyword)
	}
	
	// Limit total keywords to 15 for efficiency
	if len(keywords) > 15 {
		keywords = keywords[:15]
	}
	
	return keywords
}

// SetCache sets the cache instance for the handler
func (h *CreateNodeHandler) SetCache(cache ports.Cache) {
	h.cache = cache
}

// invalidateUserCache invalidates cached data for a user
func (h *CreateNodeHandler) invalidateUserCache(ctx context.Context, userID string) {
	// Clear ALL cache patterns for the user to ensure consistency
	patterns := []string{
		fmt.Sprintf("nodes:user:%s:*", userID),    // User's nodes list
		fmt.Sprintf("graph:user:%s:*", userID),      // User's graph data  
		fmt.Sprintf("node:%s:*", userID),           // Individual nodes
		fmt.Sprintf("user:%s:*", userID),           // Any user-specific cache
		fmt.Sprintf("*:%s:*", userID),              // Any cache with userID
		fmt.Sprintf("*%s*", userID),                // Catch-all for userID anywhere
	}
	
	for _, pattern := range patterns {
		if err := h.cache.Delete(ctx, pattern); err != nil {
			h.logger.Debug("Failed to invalidate cache",
				ports.Field{Key: "pattern", Value: pattern},
				ports.Field{Key: "error", Value: err.Error()})
		}
	}
	
	// For Lambda environments, also try to clear the entire cache
	// This ensures no stale data persists between invocations
	if err := h.cache.Delete(ctx, "*"); err != nil {
		h.logger.Debug("Failed to clear all cache",
			ports.Field{Key: "error", Value: err.Error()})
	}
}

// publishEvents publishes events to the event bus
func (h *CreateNodeHandler) publishEvents(ctx context.Context, events []events.DomainEvent) {
	for _, event := range events {
		if err := h.eventBus.Publish(ctx, event); err != nil {
			h.logger.Error("Failed to publish event",
				err,
				ports.Field{Key: "event_type", Value: event.GetEventType()})
		}
	}
}