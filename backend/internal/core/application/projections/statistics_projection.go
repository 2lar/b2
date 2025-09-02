package projections

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/domain/events"
)

// StatisticsProjection maintains aggregated statistics for analytics
type StatisticsProjection struct {
	store      ProjectionStore
	statsStore StatisticsStore
	logger     ports.Logger
	metrics    ports.Metrics
	checkpoint int64
	mutex      sync.RWMutex
}

// StatisticsStore handles statistics persistence
type StatisticsStore interface {
	// User statistics
	GetUserStats(ctx context.Context, userID string) (*UserStatistics, error)
	UpdateUserStats(ctx context.Context, userID string, updates map[string]interface{}) error
	
	// Global statistics
	GetGlobalStats(ctx context.Context) (*GlobalStatistics, error)
	UpdateGlobalStats(ctx context.Context, updates map[string]interface{}) error
	
	// Time-based statistics
	GetDailyStats(ctx context.Context, date time.Time) (*DailyStatistics, error)
	UpdateDailyStats(ctx context.Context, date time.Time, updates map[string]interface{}) error
	
	// Category statistics
	GetCategoryStats(ctx context.Context, categoryID string) (*CategoryStatistics, error)
	UpdateCategoryStats(ctx context.Context, categoryID string, updates map[string]interface{}) error
	
	// Trending analysis
	GetTrendingTags(ctx context.Context, limit int) ([]TagStatistics, error)
	GetTrendingNodes(ctx context.Context, limit int) ([]NodeStatistics, error)
	GetActiveUsers(ctx context.Context, since time.Time) ([]UserActivity, error)
}

// UserStatistics contains per-user statistics
type UserStatistics struct {
	UserID              string    `json:"user_id"`
	TotalNodes          int       `json:"total_nodes"`
	ActiveNodes         int       `json:"active_nodes"`
	ArchivedNodes       int       `json:"archived_nodes"`
	TotalEdges          int       `json:"total_edges"`
	TotalCategories     int       `json:"total_categories"`
	UniqueTagsUsed      int       `json:"unique_tags_used"`
	AverageNodeLength   float64   `json:"average_node_length"`
	AverageConnections  float64   `json:"average_connections"`
	LastActivityAt      time.Time `json:"last_activity_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	
	// Activity metrics
	DailyActiveNodes    int       `json:"daily_active_nodes"`
	WeeklyActiveNodes   int       `json:"weekly_active_nodes"`
	MonthlyActiveNodes  int       `json:"monthly_active_nodes"`
	
	// Graph metrics
	LargestComponentSize int      `json:"largest_component_size"`
	ComponentCount       int      `json:"component_count"`
	AverageCentrality    float64  `json:"average_centrality"`
	GraphDensity         float64  `json:"graph_density"`
}

// GlobalStatistics contains system-wide statistics
type GlobalStatistics struct {
	TotalUsers          int       `json:"total_users"`
	ActiveUsers         int       `json:"active_users"`
	TotalNodes          int       `json:"total_nodes"`
	TotalEdges          int       `json:"total_edges"`
	TotalCategories     int       `json:"total_categories"`
	AverageNodesPerUser float64   `json:"average_nodes_per_user"`
	AverageEdgesPerNode float64   `json:"average_edges_per_node"`
	LastUpdated         time.Time `json:"last_updated"`
	
	// Growth metrics
	DailyNewNodes       int       `json:"daily_new_nodes"`
	WeeklyNewNodes      int       `json:"weekly_new_nodes"`
	MonthlyNewNodes     int       `json:"monthly_new_nodes"`
	DailyNewUsers       int       `json:"daily_new_users"`
	WeeklyNewUsers      int       `json:"weekly_new_users"`
	MonthlyNewUsers     int       `json:"monthly_new_users"`
}

// DailyStatistics contains daily aggregated statistics
type DailyStatistics struct {
	Date               time.Time `json:"date"`
	NodesCreated       int       `json:"nodes_created"`
	NodesUpdated       int       `json:"nodes_updated"`
	NodesArchived      int       `json:"nodes_archived"`
	EdgesCreated       int       `json:"edges_created"`
	EdgesDeleted       int       `json:"edges_deleted"`
	ActiveUsers        int       `json:"active_users"`
	NewUsers           int       `json:"new_users"`
	TotalEvents        int       `json:"total_events"`
	PeakHour           int       `json:"peak_hour"`
	PeakHourEvents     int       `json:"peak_hour_events"`
}

// CategoryStatistics contains per-category statistics
type CategoryStatistics struct {
	CategoryID         string    `json:"category_id"`
	CategoryName       string    `json:"category_name"`
	NodeCount          int       `json:"node_count"`
	UserCount          int       `json:"user_count"`
	AverageNodeLength  float64   `json:"average_node_length"`
	TotalConnections   int       `json:"total_connections"`
	LastActivityAt     time.Time `json:"last_activity_at"`
	PopularTags        []string  `json:"popular_tags"`
}

// TagStatistics contains tag usage statistics
type TagStatistics struct {
	Tag               string    `json:"tag"`
	UsageCount        int       `json:"usage_count"`
	UserCount         int       `json:"user_count"`
	LastUsedAt        time.Time `json:"last_used_at"`
	TrendingScore     float64   `json:"trending_score"`
}

// NodeStatistics contains node-level statistics
type NodeStatistics struct {
	NodeID            string    `json:"node_id"`
	Title             string    `json:"title"`
	ConnectionCount   int       `json:"connection_count"`
	ViewCount         int       `json:"view_count"`
	UpdateCount       int       `json:"update_count"`
	TrendingScore     float64   `json:"trending_score"`
	LastActivityAt    time.Time `json:"last_activity_at"`
}

// UserActivity tracks user activity
type UserActivity struct {
	UserID            string    `json:"user_id"`
	LastActivityAt    time.Time `json:"last_activity_at"`
	ActivityCount     int       `json:"activity_count"`
	NodesCreated      int       `json:"nodes_created"`
	NodesUpdated      int       `json:"nodes_updated"`
	EdgesCreated      int       `json:"edges_created"`
}

// NewStatisticsProjection creates a new statistics projection
func NewStatisticsProjection(
	store ProjectionStore,
	statsStore StatisticsStore,
	logger ports.Logger,
	metrics ports.Metrics,
) *StatisticsProjection {
	return &StatisticsProjection{
		store:      store,
		statsStore: statsStore,
		logger:     logger,
		metrics:    metrics,
	}
}

// Handle processes an event to update statistics
func (p *StatisticsProjection) Handle(ctx context.Context, event events.DomainEvent) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	// Update general event counters
	if err := p.updateEventCounters(ctx, event); err != nil {
		p.logger.Warn("Failed to update event counters",
			ports.Field{Key: "event_type", Value: event.GetEventType()},
			ports.Field{Key: "error", Value: err.Error()})
	}
	
	// Handle specific event types
	switch event.GetEventType() {
	case "NodeCreated":
		return p.handleNodeCreated(ctx, event)
	case "NodeUpdated":
		return p.handleNodeUpdated(ctx, event)
	case "NodeArchived":
		return p.handleNodeArchived(ctx, event)
	case "NodeRestored":
		return p.handleNodeRestored(ctx, event)
	case "NodeConnected":
		return p.handleNodeConnected(ctx, event)
	case "NodeDisconnected":
		return p.handleNodeDisconnected(ctx, event)
	case "NodeTagged":
		return p.handleNodeTagged(ctx, event)
	case "NodeCategorized":
		return p.handleNodeCategorized(ctx, event)
	default:
		// Log unknown event type but don't fail
		p.logger.Debug("Unknown event type for statistics projection",
			ports.Field{Key: "event_type", Value: event.GetEventType()})
		return nil
	}
}

// updateEventCounters updates general event counters
func (p *StatisticsProjection) updateEventCounters(ctx context.Context, event events.DomainEvent) error {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	
	// Get or create daily statistics
	dailyStats, err := p.statsStore.GetDailyStats(ctx, today)
	if err != nil {
		dailyStats = &DailyStatistics{
			Date: today,
		}
	}
	
	dailyStats.TotalEvents++
	
	// Track peak hour
	hour := now.Hour()
	if dailyStats.PeakHour == 0 || dailyStats.PeakHourEvents < 1 {
		dailyStats.PeakHour = hour
		dailyStats.PeakHourEvents = 1
	}
	
	updates := map[string]interface{}{
		"total_events":     dailyStats.TotalEvents,
		"peak_hour":        dailyStats.PeakHour,
		"peak_hour_events": dailyStats.PeakHourEvents,
	}
	
	return p.statsStore.UpdateDailyStats(ctx, today, updates)
}

// handleNodeCreated handles NodeCreated events
func (p *StatisticsProjection) handleNodeCreated(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		UserID  string   `json:"user_id"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	// Update user statistics
	userStats, err := p.statsStore.GetUserStats(ctx, data.UserID)
	if err != nil {
		userStats = &UserStatistics{
			UserID:    data.UserID,
			CreatedAt: event.GetTimestamp(),
		}
	}
	
	userStats.TotalNodes++
	userStats.ActiveNodes++
	userStats.LastActivityAt = event.GetTimestamp()
	
	// Update average node length
	totalLength := userStats.AverageNodeLength * float64(userStats.TotalNodes-1)
	userStats.AverageNodeLength = (totalLength + float64(len(data.Content))) / float64(userStats.TotalNodes)
	
	// Update tag statistics
	uniqueTags := make(map[string]bool)
	for _, tag := range data.Tags {
		uniqueTags[tag] = true
	}
	userStats.UniqueTagsUsed += len(uniqueTags)
	
	userUpdates := map[string]interface{}{
		"total_nodes":         userStats.TotalNodes,
		"active_nodes":        userStats.ActiveNodes,
		"average_node_length": userStats.AverageNodeLength,
		"unique_tags_used":    userStats.UniqueTagsUsed,
		"last_activity_at":    userStats.LastActivityAt,
	}
	
	if err := p.statsStore.UpdateUserStats(ctx, data.UserID, userUpdates); err != nil {
		return fmt.Errorf("failed to update user stats: %w", err)
	}
	
	// Update global statistics
	globalStats, err := p.statsStore.GetGlobalStats(ctx)
	if err != nil {
		globalStats = &GlobalStatistics{}
	}
	
	globalStats.TotalNodes++
	globalStats.DailyNewNodes++
	globalStats.AverageNodesPerUser = float64(globalStats.TotalNodes) / float64(globalStats.TotalUsers)
	
	globalUpdates := map[string]interface{}{
		"total_nodes":           globalStats.TotalNodes,
		"daily_new_nodes":       globalStats.DailyNewNodes,
		"average_nodes_per_user": globalStats.AverageNodesPerUser,
		"last_updated":          event.GetTimestamp(),
	}
	
	if err := p.statsStore.UpdateGlobalStats(ctx, globalUpdates); err != nil {
		return fmt.Errorf("failed to update global stats: %w", err)
	}
	
	// Update daily statistics
	today := time.Date(event.GetTimestamp().Year(), event.GetTimestamp().Month(), 
		event.GetTimestamp().Day(), 0, 0, 0, 0, event.GetTimestamp().Location())
	
	dailyStats, err := p.statsStore.GetDailyStats(ctx, today)
	if err != nil {
		dailyStats = &DailyStatistics{Date: today}
	}
	
	dailyStats.NodesCreated++
	
	dailyUpdates := map[string]interface{}{
		"nodes_created": dailyStats.NodesCreated,
	}
	
	if err := p.statsStore.UpdateDailyStats(ctx, today, dailyUpdates); err != nil {
		return fmt.Errorf("failed to update daily stats: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.statistics.node_created")
	return nil
}

// handleNodeUpdated handles NodeUpdated events
func (p *StatisticsProjection) handleNodeUpdated(ctx context.Context, event events.DomainEvent) error {
	// Update daily statistics
	today := time.Date(event.GetTimestamp().Year(), event.GetTimestamp().Month(),
		event.GetTimestamp().Day(), 0, 0, 0, 0, event.GetTimestamp().Location())
	
	dailyStats, err := p.statsStore.GetDailyStats(ctx, today)
	if err != nil {
		dailyStats = &DailyStatistics{Date: today}
	}
	
	dailyStats.NodesUpdated++
	
	dailyUpdates := map[string]interface{}{
		"nodes_updated": dailyStats.NodesUpdated,
	}
	
	if err := p.statsStore.UpdateDailyStats(ctx, today, dailyUpdates); err != nil {
		return fmt.Errorf("failed to update daily stats: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.statistics.node_updated")
	return nil
}

// handleNodeArchived handles NodeArchived events
func (p *StatisticsProjection) handleNodeArchived(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		UserID string `json:"user_id"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		// Try to get user ID from event metadata
		data.UserID = event.GetMetadata().UserID
	}
	
	// Update user statistics
	userStats, err := p.statsStore.GetUserStats(ctx, data.UserID)
	if err == nil {
		userStats.ActiveNodes--
		userStats.ArchivedNodes++
		
		userUpdates := map[string]interface{}{
			"active_nodes":   userStats.ActiveNodes,
			"archived_nodes": userStats.ArchivedNodes,
		}
		
		if err := p.statsStore.UpdateUserStats(ctx, data.UserID, userUpdates); err != nil {
			return fmt.Errorf("failed to update user stats: %w", err)
		}
	}
	
	// Update daily statistics
	today := time.Date(event.GetTimestamp().Year(), event.GetTimestamp().Month(),
		event.GetTimestamp().Day(), 0, 0, 0, 0, event.GetTimestamp().Location())
	
	dailyStats, err := p.statsStore.GetDailyStats(ctx, today)
	if err != nil {
		dailyStats = &DailyStatistics{Date: today}
	}
	
	dailyStats.NodesArchived++
	
	dailyUpdates := map[string]interface{}{
		"nodes_archived": dailyStats.NodesArchived,
	}
	
	if err := p.statsStore.UpdateDailyStats(ctx, today, dailyUpdates); err != nil {
		return fmt.Errorf("failed to update daily stats: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.statistics.node_archived")
	return nil
}

// handleNodeRestored handles NodeRestored events
func (p *StatisticsProjection) handleNodeRestored(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		UserID string `json:"user_id"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		data.UserID = event.GetMetadata().UserID
	}
	
	// Update user statistics
	userStats, err := p.statsStore.GetUserStats(ctx, data.UserID)
	if err == nil {
		userStats.ActiveNodes++
		userStats.ArchivedNodes--
		
		userUpdates := map[string]interface{}{
			"active_nodes":   userStats.ActiveNodes,
			"archived_nodes": userStats.ArchivedNodes,
		}
		
		if err := p.statsStore.UpdateUserStats(ctx, data.UserID, userUpdates); err != nil {
			return fmt.Errorf("failed to update user stats: %w", err)
		}
	}
	
	p.metrics.IncrementCounter("projection.statistics.node_restored")
	return nil
}

// handleNodeConnected handles NodeConnected events
func (p *StatisticsProjection) handleNodeConnected(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		UserID string `json:"user_id"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		data.UserID = event.GetMetadata().UserID
	}
	
	// Update user statistics
	userStats, err := p.statsStore.GetUserStats(ctx, data.UserID)
	if err == nil {
		userStats.TotalEdges++
		userStats.AverageConnections = float64(userStats.TotalEdges) / float64(userStats.TotalNodes)
		
		userUpdates := map[string]interface{}{
			"total_edges":         userStats.TotalEdges,
			"average_connections": userStats.AverageConnections,
		}
		
		if err := p.statsStore.UpdateUserStats(ctx, data.UserID, userUpdates); err != nil {
			return fmt.Errorf("failed to update user stats: %w", err)
		}
	}
	
	// Update global statistics
	globalStats, err := p.statsStore.GetGlobalStats(ctx)
	if err == nil {
		globalStats.TotalEdges++
		globalStats.AverageEdgesPerNode = float64(globalStats.TotalEdges) / float64(globalStats.TotalNodes)
		
		globalUpdates := map[string]interface{}{
			"total_edges":           globalStats.TotalEdges,
			"average_edges_per_node": globalStats.AverageEdgesPerNode,
		}
		
		if err := p.statsStore.UpdateGlobalStats(ctx, globalUpdates); err != nil {
			return fmt.Errorf("failed to update global stats: %w", err)
		}
	}
	
	// Update daily statistics
	today := time.Date(event.GetTimestamp().Year(), event.GetTimestamp().Month(),
		event.GetTimestamp().Day(), 0, 0, 0, 0, event.GetTimestamp().Location())
	
	dailyStats, err := p.statsStore.GetDailyStats(ctx, today)
	if err != nil {
		dailyStats = &DailyStatistics{Date: today}
	}
	
	dailyStats.EdgesCreated++
	
	dailyUpdates := map[string]interface{}{
		"edges_created": dailyStats.EdgesCreated,
	}
	
	if err := p.statsStore.UpdateDailyStats(ctx, today, dailyUpdates); err != nil {
		return fmt.Errorf("failed to update daily stats: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.statistics.node_connected")
	return nil
}

// handleNodeDisconnected handles NodeDisconnected events
func (p *StatisticsProjection) handleNodeDisconnected(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		UserID string `json:"user_id"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		data.UserID = event.GetMetadata().UserID
	}
	
	// Update user statistics
	userStats, err := p.statsStore.GetUserStats(ctx, data.UserID)
	if err == nil {
		userStats.TotalEdges--
		userStats.AverageConnections = float64(userStats.TotalEdges) / float64(userStats.TotalNodes)
		
		userUpdates := map[string]interface{}{
			"total_edges":         userStats.TotalEdges,
			"average_connections": userStats.AverageConnections,
		}
		
		if err := p.statsStore.UpdateUserStats(ctx, data.UserID, userUpdates); err != nil {
			return fmt.Errorf("failed to update user stats: %w", err)
		}
	}
	
	// Update global statistics
	globalStats, err := p.statsStore.GetGlobalStats(ctx)
	if err == nil {
		globalStats.TotalEdges--
		globalStats.AverageEdgesPerNode = float64(globalStats.TotalEdges) / float64(globalStats.TotalNodes)
		
		globalUpdates := map[string]interface{}{
			"total_edges":           globalStats.TotalEdges,
			"average_edges_per_node": globalStats.AverageEdgesPerNode,
		}
		
		if err := p.statsStore.UpdateGlobalStats(ctx, globalUpdates); err != nil {
			return fmt.Errorf("failed to update global stats: %w", err)
		}
	}
	
	// Update daily statistics
	today := time.Date(event.GetTimestamp().Year(), event.GetTimestamp().Month(),
		event.GetTimestamp().Day(), 0, 0, 0, 0, event.GetTimestamp().Location())
	
	dailyStats, err := p.statsStore.GetDailyStats(ctx, today)
	if err != nil {
		dailyStats = &DailyStatistics{Date: today}
	}
	
	dailyStats.EdgesDeleted++
	
	dailyUpdates := map[string]interface{}{
		"edges_deleted": dailyStats.EdgesDeleted,
	}
	
	if err := p.statsStore.UpdateDailyStats(ctx, today, dailyUpdates); err != nil {
		return fmt.Errorf("failed to update daily stats: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.statistics.node_disconnected")
	return nil
}

// handleNodeTagged handles NodeTagged events
func (p *StatisticsProjection) handleNodeTagged(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		Tags []string `json:"tags"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	// Update tag statistics
	for range data.Tags {
		// This would update tag-specific statistics
		// Implementation depends on the stats store
		// TODO: Implement tag statistics update
	}
	
	p.metrics.IncrementCounter("projection.statistics.node_tagged")
	return nil
}

// handleNodeCategorized handles NodeCategorized events
func (p *StatisticsProjection) handleNodeCategorized(ctx context.Context, event events.DomainEvent) error {
	var data struct {
		CategoryID string `json:"category_id"`
	}
	
	if err := p.parseEventData(event, &data); err != nil {
		return err
	}
	
	// Update category statistics
	categoryStats, err := p.statsStore.GetCategoryStats(ctx, data.CategoryID)
	if err != nil {
		categoryStats = &CategoryStatistics{
			CategoryID: data.CategoryID,
		}
	}
	
	categoryStats.NodeCount++
	categoryStats.LastActivityAt = event.GetTimestamp()
	
	categoryUpdates := map[string]interface{}{
		"node_count":       categoryStats.NodeCount,
		"last_activity_at": categoryStats.LastActivityAt,
	}
	
	if err := p.statsStore.UpdateCategoryStats(ctx, data.CategoryID, categoryUpdates); err != nil {
		return fmt.Errorf("failed to update category stats: %w", err)
	}
	
	p.metrics.IncrementCounter("projection.statistics.node_categorized")
	return nil
}

// GetProjectionName returns the name of this projection
func (p *StatisticsProjection) GetProjectionName() string {
	return "StatisticsProjection"
}

// Reset clears and rebuilds the projection from events
func (p *StatisticsProjection) Reset(ctx context.Context) error {
	// This would clear the statistics store and replay all events
	return fmt.Errorf("not implemented")
}

// GetCheckpoint returns the last processed event position
func (p *StatisticsProjection) GetCheckpoint(ctx context.Context) (int64, error) {
	return p.store.GetCheckpoint(ctx, p.GetProjectionName())
}

// SaveCheckpoint saves the processing checkpoint
func (p *StatisticsProjection) SaveCheckpoint(ctx context.Context, position int64) error {
	p.checkpoint = position
	return p.store.SaveCheckpoint(ctx, p.GetProjectionName(), position)
}

// parseEventData parses event data into the target structure
func (p *StatisticsProjection) parseEventData(event events.DomainEvent, target interface{}) error {
	data, err := event.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse event data: %w", err)
	}
	
	return nil
}