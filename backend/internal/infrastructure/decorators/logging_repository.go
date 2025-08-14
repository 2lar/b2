// Package decorators - Logging Repository Decorator
//
// This file demonstrates the Decorator pattern applied to add comprehensive logging
// functionality to any repository implementation. The logging decorator provides:
//   - Request/response logging
//   - Performance monitoring
//   - Error tracking
//   - Audit trail capabilities
//
// Educational Goals:
//   - Show how to add observability without changing existing code
//   - Demonstrate structured logging best practices
//   - Illustrate performance monitoring patterns
//   - Provide debugging and audit capabilities
package decorators

import (
	"context"
	"fmt"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
)

// Logger defines the interface for logging implementations
// This abstraction allows the decorator to work with different logging libraries
type Logger interface {
	Debug(ctx context.Context, message string, fields map[string]interface{})
	Info(ctx context.Context, message string, fields map[string]interface{})
	Warn(ctx context.Context, message string, fields map[string]interface{})
	Error(ctx context.Context, message string, fields map[string]interface{})
}

// LoggingNodeRepository is a decorator that adds comprehensive logging to any NodeRepository.
// This decorator demonstrates how to add observability concerns without modifying
// the original repository implementation.
//
// Features:
//   - Method entry/exit logging
//   - Performance metrics (duration tracking)
//   - Parameter and result logging (with PII protection)
//   - Error logging with context
//   - Structured logging for easy parsing
//   - Configurable log levels
type LoggingNodeRepository struct {
	// inner is the wrapped repository
	inner repository.NodeRepository
	
	// logger is the logging implementation
	logger Logger
	
	// logLevel controls verbosity (DEBUG, INFO, WARN, ERROR)
	logLevel LogLevel
	
	// logParams controls whether method parameters are logged
	logParams bool
	
	// logResults controls whether method results are logged
	logResults bool
	
	// Component name for structured logging
	component string
}

// LogLevel defines logging verbosity levels
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// NewLoggingNodeRepository creates a new logging decorator for NodeRepository
func NewLoggingNodeRepository(
	inner repository.NodeRepository,
	logger Logger,
	level LogLevel,
	logParams bool,
	logResults bool,
) repository.NodeRepository {
	return &LoggingNodeRepository{
		inner:      inner,
		logger:     logger,
		logLevel:   level,
		logParams:  logParams,
		logResults: logResults,
		component:  "NodeRepository",
	}
}

// Helper methods for logging

// logMethodEntry logs method entry with parameters
func (r *LoggingNodeRepository) logMethodEntry(ctx context.Context, method string, params map[string]interface{}) {
	if r.logLevel <= LogLevelDebug {
		fields := map[string]interface{}{
			"component": r.component,
			"method":    method,
			"action":    "entry",
		}
		
		if r.logParams && params != nil {
			// Sanitize sensitive information
			sanitized := r.sanitizeParams(params)
			fields["params"] = sanitized
		}
		
		r.logger.Debug(ctx, fmt.Sprintf("%s.%s entry", r.component, method), fields)
	}
}

// logMethodExit logs successful method completion
func (r *LoggingNodeRepository) logMethodExit(ctx context.Context, method string, duration time.Duration, result interface{}) {
	if r.logLevel <= LogLevelInfo {
		fields := map[string]interface{}{
			"component":     r.component,
			"method":        method,
			"action":        "success",
			"duration_ms":   duration.Milliseconds(),
			"duration_ns":   duration.Nanoseconds(),
		}
		
		if r.logResults && result != nil {
			// Add result metadata without sensitive data
			fields["result"] = r.sanitizeResult(result)
		}
		
		r.logger.Info(ctx, fmt.Sprintf("%s.%s completed successfully", r.component, method), fields)
	}
}

// logMethodError logs method errors
func (r *LoggingNodeRepository) logMethodError(ctx context.Context, method string, duration time.Duration, err error, params map[string]interface{}) {
	if r.logLevel <= LogLevelError {
		fields := map[string]interface{}{
			"component":   r.component,
			"method":      method,
			"action":      "error",
			"duration_ms": duration.Milliseconds(),
			"error":       err.Error(),
			"error_type":  fmt.Sprintf("%T", err),
		}
		
		// Add sanitized parameters for debugging
		if r.logParams && params != nil {
			fields["params"] = r.sanitizeParams(params)
		}
		
		r.logger.Error(ctx, fmt.Sprintf("%s.%s failed", r.component, method), fields)
	}
}

// sanitizeParams removes or masks sensitive information from parameters
func (r *LoggingNodeRepository) sanitizeParams(params map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	
	for key, value := range params {
		switch key {
		case "userID":
			// Mask user ID for privacy (keep first few characters)
			if str, ok := value.(string); ok && len(str) > 4 {
				sanitized[key] = str[:4] + "***"
			} else {
				sanitized[key] = "***"
			}
		case "content":
			// Log content length instead of actual content for privacy
			if str, ok := value.(string); ok {
				sanitized[key+"_length"] = len(str)
			}
		case "nodeID", "id":
			// Node IDs are generally safe to log
			sanitized[key] = value
		case "keywords", "tags":
			// Keywords and tags might be okay to log (depending on policy)
			sanitized[key] = value
		case "opts":
			// Query options are usually safe
			sanitized[key] = value
		default:
			// For unknown fields, be conservative
			sanitized[key] = fmt.Sprintf("<%T>", value)
		}
	}
	
	return sanitized
}

// sanitizeResult creates safe-to-log representation of results
func (r *LoggingNodeRepository) sanitizeResult(result interface{}) interface{} {
	switch v := result.(type) {
	case *domain.Node:
		return map[string]interface{}{
			"type":         "Node",
			"id":           v.ID().String(),
			"userID":       r.maskUserID(v.UserID().String()),
			"content_length": len(v.Content().String()),
			"keywords_count": v.Keywords().Count(),
			"tags_count":    v.Tags().Count(),
		}
	case []*domain.Node:
		return map[string]interface{}{
			"type":  "Node[]",
			"count": len(v),
		}
	case bool:
		return v
	case int:
		return v
	default:
		return fmt.Sprintf("<%T>", result)
	}
}

// maskUserID masks user ID for privacy
func (r *LoggingNodeRepository) maskUserID(userID string) string {
	if len(userID) > 4 {
		return userID[:4] + "***"
	}
	return "***"
}

// Repository method implementations with logging

// FindByID retrieves a node by ID with full logging
func (r *LoggingNodeRepository) FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
	start := time.Now()
	method := "FindByID"
	
	params := map[string]interface{}{
		"id": id.String(),
	}
	
	r.logMethodEntry(ctx, method, params)
	
	// Call the wrapped repository
	result, err := r.inner.FindByID(ctx, id)
	
	duration := time.Since(start)
	
	if err != nil {
		r.logMethodError(ctx, method, duration, err, params)
		return nil, err
	}
	
	r.logMethodExit(ctx, method, duration, result)
	return result, nil
}

// FindByUser retrieves nodes for a user with logging
func (r *LoggingNodeRepository) FindByUser(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) ([]*domain.Node, error) {
	start := time.Now()
	method := "FindByUser"
	
	params := map[string]interface{}{
		"userID":    userID.String(),
		"opts_count": len(opts),
	}
	
	r.logMethodEntry(ctx, method, params)
	
	result, err := r.inner.FindByUser(ctx, userID, opts...)
	
	duration := time.Since(start)
	
	if err != nil {
		r.logMethodError(ctx, method, duration, err, params)
		return nil, err
	}
	
	r.logMethodExit(ctx, method, duration, result)
	return result, nil
}

// Exists checks if a node exists with logging
func (r *LoggingNodeRepository) Exists(ctx context.Context, id domain.NodeID) (bool, error) {
	start := time.Now()
	method := "Exists"
	
	params := map[string]interface{}{
		"id": id.String(),
	}
	
	r.logMethodEntry(ctx, method, params)
	
	result, err := r.inner.Exists(ctx, id)
	
	duration := time.Since(start)
	
	if err != nil {
		r.logMethodError(ctx, method, duration, err, params)
		return false, err
	}
	
	r.logMethodExit(ctx, method, duration, result)
	return result, nil
}

// Count returns node count with logging
func (r *LoggingNodeRepository) Count(ctx context.Context, userID domain.UserID, opts ...repository.QueryOption) (int, error) {
	start := time.Now()
	method := "Count"
	
	params := map[string]interface{}{
		"userID":    userID.String(),
		"opts_count": len(opts),
	}
	
	r.logMethodEntry(ctx, method, params)
	
	result, err := r.inner.Count(ctx, userID, opts...)
	
	duration := time.Since(start)
	
	if err != nil {
		r.logMethodError(ctx, method, duration, err, params)
		return 0, err
	}
	
	r.logMethodExit(ctx, method, duration, result)
	return result, nil
}

// FindByKeywords searches by keywords with logging
func (r *LoggingNodeRepository) FindByKeywords(ctx context.Context, userID domain.UserID, keywords []string, opts ...repository.QueryOption) ([]*domain.Node, error) {
	start := time.Now()
	method := "FindByKeywords"
	
	params := map[string]interface{}{
		"userID":         userID.String(),
		"keywords":       keywords,
		"keywords_count": len(keywords),
		"opts_count":     len(opts),
	}
	
	r.logMethodEntry(ctx, method, params)
	
	result, err := r.inner.FindByKeywords(ctx, userID, keywords, opts...)
	
	duration := time.Since(start)
	
	if err != nil {
		r.logMethodError(ctx, method, duration, err, params)
		return nil, err
	}
	
	r.logMethodExit(ctx, method, duration, result)
	return result, nil
}

// FindSimilar finds similar nodes with logging
func (r *LoggingNodeRepository) FindSimilar(ctx context.Context, node *domain.Node, opts ...repository.QueryOption) ([]*domain.Node, error) {
	start := time.Now()
	method := "FindSimilar"
	
	params := map[string]interface{}{
		"reference_node_id": node.ID().String(),
		"opts_count":        len(opts),
	}
	
	r.logMethodEntry(ctx, method, params)
	
	result, err := r.inner.FindSimilar(ctx, node, opts...)
	
	duration := time.Since(start)
	
	if err != nil {
		r.logMethodError(ctx, method, duration, err, params)
		return nil, err
	}
	
	r.logMethodExit(ctx, method, duration, result)
	return result, nil
}

// Write operations with audit logging

// Save creates or updates a node with audit logging
func (r *LoggingNodeRepository) Save(ctx context.Context, node *domain.Node) error {
	start := time.Now()
	method := "Save"
	
	params := map[string]interface{}{
		"nodeID":         node.ID().String(),
		"userID":         node.UserID().String(),
		"content_length": len(node.Content().String()),
		"version":        node.Version().Int(),
	}
	
	r.logMethodEntry(ctx, method, params)
	
	err := r.inner.Save(ctx, node)
	
	duration := time.Since(start)
	
	if err != nil {
		r.logMethodError(ctx, method, duration, err, params)
		return err
	}
	
	// For write operations, always log success for audit purposes
	fields := map[string]interface{}{
		"component":   r.component,
		"method":      method,
		"action":      "audit",
		"nodeID":      node.ID().String(),
		"userID":      r.maskUserID(node.UserID().String()),
		"duration_ms": duration.Milliseconds(),
	}
	
	r.logger.Info(ctx, fmt.Sprintf("Node saved: %s", node.ID().String()), fields)
	r.logMethodExit(ctx, method, duration, "success")
	
	return nil
}

// Delete removes a node with audit logging
func (r *LoggingNodeRepository) Delete(ctx context.Context, id domain.NodeID) error {
	start := time.Now()
	method := "Delete"
	
	params := map[string]interface{}{
		"id": id.String(),
	}
	
	r.logMethodEntry(ctx, method, params)
	
	err := r.inner.Delete(ctx, id)
	
	duration := time.Since(start)
	
	if err != nil {
		r.logMethodError(ctx, method, duration, err, params)
		return err
	}
	
	// Audit log for delete operations
	fields := map[string]interface{}{
		"component":   r.component,
		"method":      method,
		"action":      "audit",
		"nodeID":      id.String(),
		"duration_ms": duration.Milliseconds(),
	}
	
	r.logger.Info(ctx, fmt.Sprintf("Node deleted: %s", id.String()), fields)
	r.logMethodExit(ctx, method, duration, "success")
	
	return nil
}

// SaveBatch saves multiple nodes with batch logging
func (r *LoggingNodeRepository) SaveBatch(ctx context.Context, nodes []*domain.Node) error {
	start := time.Now()
	method := "SaveBatch"
	
	nodeIDs := make([]string, len(nodes))
	userIDs := make(map[string]bool)
	
	for i, node := range nodes {
		nodeIDs[i] = node.ID().String()
		userIDs[node.UserID().String()] = true
	}
	
	params := map[string]interface{}{
		"node_count":   len(nodes),
		"unique_users": len(userIDs),
	}
	
	r.logMethodEntry(ctx, method, params)
	
	err := r.inner.SaveBatch(ctx, nodes)
	
	duration := time.Since(start)
	
	if err != nil {
		r.logMethodError(ctx, method, duration, err, params)
		return err
	}
	
	// Audit log for batch operations
	fields := map[string]interface{}{
		"component":   r.component,
		"method":      method,
		"action":      "audit",
		"node_count":  len(nodes),
		"duration_ms": duration.Milliseconds(),
	}
	
	r.logger.Info(ctx, fmt.Sprintf("Batch save completed: %d nodes", len(nodes)), fields)
	r.logMethodExit(ctx, method, duration, fmt.Sprintf("%d nodes saved", len(nodes)))
	
	return nil
}

// DeleteBatch removes multiple nodes with batch logging
func (r *LoggingNodeRepository) DeleteBatch(ctx context.Context, ids []domain.NodeID) error {
	start := time.Now()
	method := "DeleteBatch"
	
	idStrings := make([]string, len(ids))
	for i, id := range ids {
		idStrings[i] = id.String()
	}
	
	params := map[string]interface{}{
		"node_count": len(ids),
	}
	
	r.logMethodEntry(ctx, method, params)
	
	err := r.inner.DeleteBatch(ctx, ids)
	
	duration := time.Since(start)
	
	if err != nil {
		r.logMethodError(ctx, method, duration, err, params)
		return err
	}
	
	// Audit log for batch delete
	fields := map[string]interface{}{
		"component":   r.component,
		"method":      method,
		"action":      "audit",
		"node_count":  len(ids),
		"duration_ms": duration.Milliseconds(),
	}
	
	r.logger.Info(ctx, fmt.Sprintf("Batch delete completed: %d nodes", len(ids)), fields)
	r.logMethodExit(ctx, method, duration, fmt.Sprintf("%d nodes deleted", len(ids)))
	
	return nil
}

// Performance monitoring helpers

// logSlowQuery logs queries that exceed performance thresholds
func (r *LoggingNodeRepository) logSlowQuery(ctx context.Context, method string, duration time.Duration, threshold time.Duration, params map[string]interface{}) {
	if duration > threshold {
		fields := map[string]interface{}{
			"component":       r.component,
			"method":          method,
			"action":          "slow_query",
			"duration_ms":     duration.Milliseconds(),
			"threshold_ms":    threshold.Milliseconds(),
			"performance":     "degraded",
		}
		
		// Add sanitized parameters for debugging slow queries
		if params != nil {
			fields["params"] = r.sanitizeParams(params)
		}
		
		r.logger.Warn(ctx, fmt.Sprintf("Slow query detected in %s.%s", r.component, method), fields)
	}
}

// Example usage with multiple decorators:
//
// // Create base repository
// baseRepo := dynamodb.NewNodeRepository(client)
//
// // Add logging
// loggedRepo := NewLoggingNodeRepository(
//     baseRepo,
//     logger,
//     LogLevelInfo,
//     true,  // log parameters
//     false, // don't log results (for privacy)
// )
//
// // Add caching on top of logging
// cachedRepo := NewCachingNodeRepository(
//     loggedRepo,
//     cache,
//     10*time.Minute,
//     "brain2",
// )
//
// // Now we have: Cache -> Logging -> Base Repository
// // All operations are cached AND logged automatically!

// This demonstrates how decorators can be composed to add multiple
// cross-cutting concerns without any changes to the base repository.