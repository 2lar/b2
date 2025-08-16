// Package decorators provides decorator implementations for repository interfaces.
// This package demonstrates the Decorator pattern for adding cross-cutting concerns
// to repository operations without modifying the core business logic.
package decorators

import (
	"context"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggingNodeRepository is a decorator that adds comprehensive logging to NodeRepository operations.
//
// Key Concepts Illustrated:
//   1. Decorator Pattern: Wraps another repository to add logging behavior
//   2. Composition over Inheritance: Uses composition to extend functionality
//   3. Transparent Enhancement: Maintains the same interface as the wrapped repository
//   4. Cross-cutting Concerns: Separates logging from business logic
//   5. Performance Monitoring: Tracks operation duration and success/failure rates
//
// This decorator can wrap any NodeRepository implementation and adds:
//   - Operation timing and performance metrics
//   - Request/response logging with configurable detail levels
//   - Error logging with context information
//   - Audit trails for security and compliance
//   - Debug information for development and troubleshooting
//
// Example Usage:
//   baseRepo := dynamodb.NewNodeRepository(client, table, index)
//   loggedRepo := NewLoggingNodeRepository(baseRepo, logger, LoggingConfig{
//       LogRequests:  true,
//       LogResponses: false, // PII considerations
//       LogErrors:    true,
//       LogTiming:    true,
//   })
type LoggingNodeRepository struct {
	inner  repository.NodeRepository
	logger *zap.Logger
	config LoggingConfig
}

// LoggingConfig controls what information is logged
type LoggingConfig struct {
	LogRequests    bool          // Log input parameters
	LogResponses   bool          // Log output data (be careful with PII)
	LogErrors      bool          // Log errors with stack traces
	LogTiming      bool          // Log operation duration
	LogLevel       zapcore.Level // Minimum log level for operations
	SlowThreshold  time.Duration // Log warning for operations slower than this
}

// DefaultLoggingConfig returns sensible defaults for logging configuration
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		LogRequests:    true,
		LogResponses:   false, // Default to false for PII protection
		LogErrors:      true,
		LogTiming:      true,
		LogLevel:       zapcore.DebugLevel,
		SlowThreshold:  time.Second,
	}
}

// NewLoggingNodeRepository creates a new logging decorator for NodeRepository.
func NewLoggingNodeRepository(
	inner repository.NodeRepository,
	logger *zap.Logger,
	config LoggingConfig,
) repository.NodeRepository {
	return &LoggingNodeRepository{
		inner:  inner,
		logger: logger.Named("node_repository"),
		config: config,
	}
}

// CreateNodeAndKeywords wraps the create operation with logging
func (r *LoggingNodeRepository) CreateNodeAndKeywords(ctx context.Context, node *domain.Node) error {
	start := time.Now()
	operationID := generateOperationID()
	
	// Log the operation start
	logFields := []zap.Field{
		zap.String("operation", "create_node_and_keywords"),
		zap.String("operation_id", operationID),
		zap.String("user_id", node.UserID.String()),
		zap.String("node_id", node.ID.String()),
	}
	
	if r.config.LogRequests {
		logFields = append(logFields,
			zap.Int("content_length", len(node.Content.String())),
			zap.Int("keyword_count", len(node.Keywords().ToSlice())),
			zap.Int("tag_count", len(node.Tags.ToSlice())),
		)
	}
	
	r.logger.Debug("starting node creation", logFields...)
	
	// Execute the operation
	err := r.inner.CreateNodeAndKeywords(ctx, node)
	
	// Calculate duration
	duration := time.Since(start)
	logFields = append(logFields, zap.Duration("duration", duration))
	
	// Log completion
	if err != nil {
		if r.config.LogErrors {
			r.logger.Error("node creation failed",
				append(logFields, zap.Error(err))...,
			)
		}
	} else {
		logLevel := zap.DebugLevel
		message := "node creation completed"
		
		// Log slow operations as warnings
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow node creation completed"
		}
		
		r.logger.Check(logLevel, message).Write(logFields...)
	}
	
	return err
}

// FindNodeByID wraps the find operation with logging
func (r *LoggingNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error) {
	start := time.Now()
	operationID := generateOperationID()
	
	logFields := []zap.Field{
		zap.String("operation", "find_node_by_id"),
		zap.String("operation_id", operationID),
		zap.String("user_id", userID),
		zap.String("node_id", nodeID),
	}
	
	r.logger.Debug("starting node lookup", logFields...)
	
	// Execute the operation
	node, err := r.inner.FindNodeByID(ctx, userID, nodeID)
	
	// Calculate duration
	duration := time.Since(start)
	logFields = append(logFields, zap.Duration("duration", duration))
	
	// Log completion
	if err != nil {
		if r.config.LogErrors {
			// Check if it's a not found error (don't log as error level)
			if repository.IsNotFound(err) {
				r.logger.Debug("node not found", append(logFields, zap.Error(err))...)
			} else {
				r.logger.Error("node lookup failed", append(logFields, zap.Error(err))...)
			}
		}
	} else {
		logLevel := zap.DebugLevel
		message := "node lookup completed"
		
		if r.config.LogResponses && node != nil {
			logFields = append(logFields,
				zap.Int("content_length", len(node.Content.String())),
				zap.Int("keyword_count", len(node.Keywords().ToSlice())),
			)
		}
		
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow node lookup completed"
		}
		
		r.logger.Check(logLevel, message).Write(logFields...)
	}
	
	return node, err
}

// FindNodes wraps the find multiple nodes operation with logging
func (r *LoggingNodeRepository) FindNodes(ctx context.Context, query repository.NodeQuery) ([]*domain.Node, error) {
	start := time.Now()
	operationID := generateOperationID()
	
	logFields := []zap.Field{
		zap.String("operation", "find_nodes"),
		zap.String("operation_id", operationID),
		zap.String("user_id", query.UserID),
	}
	
	if r.config.LogRequests {
		logFields = append(logFields,
			zap.Int("keyword_count", len(query.Keywords)),
			zap.Int("node_id_count", len(query.NodeIDs)),
			zap.Int("limit", query.Limit),
			zap.Int("offset", query.Offset),
		)
	}
	
	r.logger.Debug("starting node search", logFields...)
	
	// Execute the operation
	nodes, err := r.inner.FindNodes(ctx, query)
	
	// Calculate duration
	duration := time.Since(start)
	logFields = append(logFields, zap.Duration("duration", duration))
	
	// Log completion
	if err != nil {
		if r.config.LogErrors {
			r.logger.Error("node search failed", append(logFields, zap.Error(err))...)
		}
	} else {
		resultCount := len(nodes)
		logFields = append(logFields, zap.Int("result_count", resultCount))
		
		logLevel := zap.DebugLevel
		message := "node search completed"
		
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow node search completed"
		}
		
		r.logger.Check(logLevel, message).Write(logFields...)
	}
	
	return nodes, err
}

// DeleteNode wraps the delete operation with logging
func (r *LoggingNodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	start := time.Now()
	operationID := generateOperationID()
	
	logFields := []zap.Field{
		zap.String("operation", "delete_node"),
		zap.String("operation_id", operationID),
		zap.String("user_id", userID),
		zap.String("node_id", nodeID),
	}
	
	r.logger.Info("starting node deletion", logFields...) // Deletion is always logged at info level
	
	// Execute the operation
	err := r.inner.DeleteNode(ctx, userID, nodeID)
	
	// Calculate duration
	duration := time.Since(start)
	logFields = append(logFields, zap.Duration("duration", duration))
	
	// Log completion
	if err != nil {
		if r.config.LogErrors {
			r.logger.Error("node deletion failed", append(logFields, zap.Error(err))...)
		}
	} else {
		logLevel := zap.InfoLevel // Successful deletions are important
		message := "node deletion completed"
		
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow node deletion completed"
		}
		
		r.logger.Check(logLevel, message).Write(logFields...)
	}
	
	return err
}

// GetNodesPage wraps the paginated query with logging
func (r *LoggingNodeRepository) GetNodesPage(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination) (*repository.NodePage, error) {
	start := time.Now()
	operationID := generateOperationID()
	
	logFields := []zap.Field{
		zap.String("operation", "get_nodes_page"),
		zap.String("operation_id", operationID),
		zap.String("user_id", query.UserID),
		zap.Int("page_limit", pagination.GetEffectiveLimit()),
		zap.Int("page_offset", pagination.Offset),
	}
	
	if r.config.LogRequests {
		logFields = append(logFields,
			zap.Bool("has_cursor", pagination.HasCursor()),
			zap.String("sort_by", pagination.SortBy),
			zap.String("sort_order", pagination.SortOrder),
		)
	}
	
	r.logger.Debug("starting paginated node query", logFields...)
	
	// Execute the operation
	page, err := r.inner.GetNodesPage(ctx, query, pagination)
	
	// Calculate duration
	duration := time.Since(start)
	logFields = append(logFields, zap.Duration("duration", duration))
	
	// Log completion
	if err != nil {
		if r.config.LogErrors {
			r.logger.Error("paginated node query failed", append(logFields, zap.Error(err))...)
		}
	} else {
		if page != nil {
			logFields = append(logFields,
				zap.Int("items_returned", len(page.Items)),
				zap.Bool("has_more", page.HasMore),
				zap.Int("current_page", page.PageInfo.CurrentPage),
			)
		}
		
		logLevel := zap.DebugLevel
		message := "paginated node query completed"
		
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow paginated node query completed"
		}
		
		r.logger.Check(logLevel, message).Write(logFields...)
	}
	
	return page, err
}

// GetNodeNeighborhood wraps the neighborhood query with logging
func (r *LoggingNodeRepository) GetNodeNeighborhood(ctx context.Context, userID, nodeID string, depth int) (*domain.Graph, error) {
	start := time.Now()
	operationID := generateOperationID()
	
	logFields := []zap.Field{
		zap.String("operation", "get_node_neighborhood"),
		zap.String("operation_id", operationID),
		zap.String("user_id", userID),
		zap.String("node_id", nodeID),
		zap.Int("depth", depth),
	}
	
	r.logger.Debug("starting neighborhood query", logFields...)
	
	// Execute the operation
	graph, err := r.inner.GetNodeNeighborhood(ctx, userID, nodeID, depth)
	
	// Calculate duration
	duration := time.Since(start)
	logFields = append(logFields, zap.Duration("duration", duration))
	
	// Log completion
	if err != nil {
		if r.config.LogErrors {
			r.logger.Error("neighborhood query failed", append(logFields, zap.Error(err))...)
		}
	} else {
		if graph != nil && r.config.LogResponses {
			logFields = append(logFields,
				zap.Int("node_count", len(graph.Nodes)),
				zap.Int("edge_count", len(graph.Edges)),
			)
		}
		
		logLevel := zap.DebugLevel
		message := "neighborhood query completed"
		
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow neighborhood query completed"
		}
		
		r.logger.Check(logLevel, message).Write(logFields...)
	}
	
	return graph, err
}

// CountNodes wraps the count operation with logging
func (r *LoggingNodeRepository) CountNodes(ctx context.Context, userID string) (int, error) {
	start := time.Now()
	operationID := generateOperationID()
	
	logFields := []zap.Field{
		zap.String("operation", "count_nodes"),
		zap.String("operation_id", operationID),
		zap.String("user_id", userID),
	}
	
	r.logger.Debug("starting node count", logFields...)
	
	// Execute the operation
	count, err := r.inner.CountNodes(ctx, userID)
	
	// Calculate duration
	duration := time.Since(start)
	logFields = append(logFields, zap.Duration("duration", duration))
	
	// Log completion
	if err != nil {
		if r.config.LogErrors {
			r.logger.Error("node count failed", append(logFields, zap.Error(err))...)
		}
	} else {
		logFields = append(logFields, zap.Int("count", count))
		
		logLevel := zap.DebugLevel
		message := "node count completed"
		
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow node count completed"
		}
		
		r.logger.Check(logLevel, message).Write(logFields...)
	}
	
	return count, err
}

// Utility functions

// generateOperationID creates a unique identifier for tracking operations across logs
func generateOperationID() string {
	// This is a simplified implementation - in practice, you might use:
	// - UUIDs for true uniqueness
	// - Request tracing IDs from context
	// - Distributed tracing correlation IDs
	return time.Now().Format("20060102150405") + "_" + randomString(6)
}

// randomString generates a random string of the specified length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}

// LoggingEdgeRepository provides logging for edge operations
type LoggingEdgeRepository struct {
	inner  repository.EdgeRepository
	logger *zap.Logger
	config LoggingConfig
}

// NewLoggingEdgeRepository creates a new logging decorator for EdgeRepository
func NewLoggingEdgeRepository(
	inner repository.EdgeRepository,
	logger *zap.Logger,
	config LoggingConfig,
) repository.EdgeRepository {
	return &LoggingEdgeRepository{
		inner:  inner,
		logger: logger.Named("edge_repository"),
		config: config,
	}
}

// CreateEdges wraps the create edges operation with logging
func (r *LoggingEdgeRepository) CreateEdges(ctx context.Context, userID, sourceNodeID string, relatedNodeIDs []string) error {
	start := time.Now()
	operationID := generateOperationID()
	
	logFields := []zap.Field{
		zap.String("operation", "create_edges"),
		zap.String("operation_id", operationID),
		zap.String("user_id", userID),
		zap.String("source_node_id", sourceNodeID),
		zap.Int("target_count", len(relatedNodeIDs)),
	}
	
	r.logger.Debug("starting edge creation", logFields...)
	
	// Execute the operation
	err := r.inner.CreateEdges(ctx, userID, sourceNodeID, relatedNodeIDs)
	
	// Calculate duration
	duration := time.Since(start)
	logFields = append(logFields, zap.Duration("duration", duration))
	
	// Log completion
	if err != nil {
		if r.config.LogErrors {
			r.logger.Error("edge creation failed", append(logFields, zap.Error(err))...)
		}
	} else {
		logLevel := zap.DebugLevel
		message := "edge creation completed"
		
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow edge creation completed"
		}
		
		r.logger.Check(logLevel, message).Write(logFields...)
	}
	
	return err
}

// CreateEdge wraps the create single edge operation with logging
func (r *LoggingEdgeRepository) CreateEdge(ctx context.Context, edge *domain.Edge) error {
	start := time.Now()
	operationID := generateOperationID()
	
	logFields := []zap.Field{
		zap.String("operation", "create_edge"),
		zap.String("operation_id", operationID),
		zap.String("user_id", edge.UserID().String()),
		zap.String("source_id", edge.SourceID.String()),
		zap.String("target_id", edge.TargetID.String()),
		zap.Float64("weight", edge.Weight()),
	}
	
	r.logger.Debug("starting single edge creation", logFields...)
	
	// Execute the operation
	err := r.inner.CreateEdge(ctx, edge)
	
	// Calculate duration
	duration := time.Since(start)
	logFields = append(logFields, zap.Duration("duration", duration))
	
	// Log completion
	if err != nil {
		if r.config.LogErrors {
			r.logger.Error("single edge creation failed", append(logFields, zap.Error(err))...)
		}
	} else {
		logLevel := zap.DebugLevel
		message := "single edge creation completed"
		
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow single edge creation completed"
		}
		
		r.logger.Check(logLevel, message).Write(logFields...)
	}
	
	return err
}

// FindEdges wraps the find edges operation with logging
func (r *LoggingEdgeRepository) FindEdges(ctx context.Context, query repository.EdgeQuery) ([]*domain.Edge, error) {
	start := time.Now()
	operationID := generateOperationID()
	
	logFields := []zap.Field{
		zap.String("operation", "find_edges"),
		zap.String("operation_id", operationID),
		zap.String("user_id", query.UserID),
	}
	
	if r.config.LogRequests {
		logFields = append(logFields,
			zap.Int("node_id_count", len(query.NodeIDs)),
			zap.String("source_id", query.SourceID),
			zap.String("target_id", query.TargetID),
			zap.Int("limit", query.Limit),
		)
	}
	
	r.logger.Debug("starting edge search", logFields...)
	
	// Execute the operation
	edges, err := r.inner.FindEdges(ctx, query)
	
	// Calculate duration
	duration := time.Since(start)
	logFields = append(logFields, zap.Duration("duration", duration))
	
	// Log completion
	if err != nil {
		if r.config.LogErrors {
			r.logger.Error("edge search failed", append(logFields, zap.Error(err))...)
		}
	} else {
		logFields = append(logFields, zap.Int("result_count", len(edges)))
		
		logLevel := zap.DebugLevel
		message := "edge search completed"
		
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow edge search completed"
		}
		
		r.logger.Check(logLevel, message).Write(logFields...)
	}
	
	return edges, err
}

// GetEdgesPage wraps the paginated edge query with logging
func (r *LoggingEdgeRepository) GetEdgesPage(ctx context.Context, query repository.EdgeQuery, pagination repository.Pagination) (*repository.EdgePage, error) {
	start := time.Now()
	operationID := generateOperationID()
	
	logFields := []zap.Field{
		zap.String("operation", "get_edges_page"),
		zap.String("operation_id", operationID),
		zap.String("user_id", query.UserID),
		zap.Int("page_limit", pagination.GetEffectiveLimit()),
		zap.Int("page_offset", pagination.Offset),
	}
	
	r.logger.Debug("starting paginated edge query", logFields...)
	
	// Execute the operation
	page, err := r.inner.GetEdgesPage(ctx, query, pagination)
	
	// Calculate duration
	duration := time.Since(start)
	logFields = append(logFields, zap.Duration("duration", duration))
	
	// Log completion
	if err != nil {
		if r.config.LogErrors {
			r.logger.Error("paginated edge query failed", append(logFields, zap.Error(err))...)
		}
	} else {
		if page != nil {
			logFields = append(logFields,
				zap.Int("items_returned", len(page.Items)),
				zap.Bool("has_more", page.HasMore),
			)
		}
		
		logLevel := zap.DebugLevel
		message := "paginated edge query completed"
		
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow paginated edge query completed"
		}
		
		r.logger.Check(logLevel, message).Write(logFields...)
	}
	
	return page, err
}

// Phase 2 Enhanced Methods - Added for interface compatibility

// FindNodesWithOptions adds logging to enhanced node queries with options
func (r *LoggingNodeRepository) FindNodesWithOptions(ctx context.Context, query repository.NodeQuery, opts ...repository.QueryOption) ([]*domain.Node, error) {
	if r.config.LogRequests {
		r.logger.Debug("executing FindNodesWithOptions", 
			zap.String("userID", query.UserID),
			zap.Int("optionCount", len(opts)))
	}
	
	start := time.Now()
	nodes, err := r.inner.FindNodesWithOptions(ctx, query, opts...)
	duration := time.Since(start)
	
	logLevel := r.config.LogLevel
	message := "FindNodesWithOptions completed"
	
	logFields := []zap.Field{
		zap.String("userID", query.UserID),
		zap.Duration("duration", duration),
		zap.Int("nodeCount", len(nodes)),
		zap.Int("optionCount", len(opts)),
	}
	
	if err != nil && r.config.LogErrors {
		logFields = append(logFields, zap.Error(err))
		logLevel = zap.ErrorLevel
		message = "FindNodesWithOptions failed"
	} else {
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow FindNodesWithOptions completed"
		}
	}
	
	r.logger.Check(logLevel, message).Write(logFields...)
	return nodes, err
}

// FindNodesPageWithOptions adds logging to enhanced paginated node queries with options
func (r *LoggingNodeRepository) FindNodesPageWithOptions(ctx context.Context, query repository.NodeQuery, pagination repository.Pagination, opts ...repository.QueryOption) (*repository.NodePage, error) {
	if r.config.LogRequests {
		r.logger.Debug("executing FindNodesPageWithOptions",
			zap.String("userID", query.UserID),
			zap.Int("limit", pagination.Limit),
			zap.Int("optionCount", len(opts)))
	}
	
	start := time.Now()
	page, err := r.inner.FindNodesPageWithOptions(ctx, query, pagination, opts...)
	duration := time.Since(start)
	
	logLevel := r.config.LogLevel
	message := "FindNodesPageWithOptions completed"
	
	logFields := []zap.Field{
		zap.String("userID", query.UserID),
		zap.Duration("duration", duration),
		zap.Int("optionCount", len(opts)),
	}
	
	if page != nil {
		logFields = append(logFields, zap.Int("nodeCount", len(page.Items)))
	}
	
	if err != nil && r.config.LogErrors {
		logFields = append(logFields, zap.Error(err))
		logLevel = zap.ErrorLevel
		message = "FindNodesPageWithOptions failed"
	} else {
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow FindNodesPageWithOptions completed"
		}
	}
	
	r.logger.Check(logLevel, message).Write(logFields...)
	return page, err
}

// FindEdgesWithOptions adds logging to enhanced edge queries with options
func (r *LoggingEdgeRepository) FindEdgesWithOptions(ctx context.Context, query repository.EdgeQuery, opts ...repository.QueryOption) ([]*domain.Edge, error) {
	if r.config.LogRequests {
		r.logger.Debug("executing FindEdgesWithOptions",
			zap.String("userID", query.UserID),
			zap.Int("optionCount", len(opts)))
	}
	
	start := time.Now()
	edges, err := r.inner.FindEdgesWithOptions(ctx, query, opts...)
	duration := time.Since(start)
	
	logLevel := r.config.LogLevel
	message := "FindEdgesWithOptions completed"
	
	logFields := []zap.Field{
		zap.String("userID", query.UserID),
		zap.Duration("duration", duration),
		zap.Int("edgeCount", len(edges)),
		zap.Int("optionCount", len(opts)),
	}
	
	if err != nil && r.config.LogErrors {
		logFields = append(logFields, zap.Error(err))
		logLevel = zap.ErrorLevel
		message = "FindEdgesWithOptions failed"
	} else {
		if r.config.LogTiming && duration > r.config.SlowThreshold {
			logLevel = zap.WarnLevel
			message = "slow FindEdgesWithOptions completed"
		}
	}
	
	r.logger.Check(logLevel, message).Write(logFields...)
	return edges, err
}