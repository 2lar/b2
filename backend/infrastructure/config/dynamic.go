package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DynamicConfigManager manages runtime configuration with hot-reload support
type DynamicConfigManager struct {
	// Static configuration (from environment)
	staticConfig *Config

	// Dynamic configuration (from file or DynamoDB)
	watcher *ConfigWatcher

	// Configuration store for persistence
	store ConfigStore

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc

	// Thread safety
	mu sync.RWMutex

	// Callbacks for configuration changes
	callbacks []ConfigChangeCallback

	logger *zap.Logger
}

// ConfigChangeCallback is called when configuration changes
type ConfigChangeCallback func(oldConfig, newConfig *DynamicConfig)

// ConfigStore interface for configuration persistence
type ConfigStore interface {
	Load(ctx context.Context) (*DynamicConfig, error)
	Save(ctx context.Context, config *DynamicConfig) error
	Watch(ctx context.Context, onChange func(*DynamicConfig)) error
}

// NewDynamicConfigManager creates a new dynamic configuration manager
func NewDynamicConfigManager(staticConfig *Config, configPath string, logger *zap.Logger) (*DynamicConfigManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create file watcher if path is provided
	var watcher *ConfigWatcher
	if configPath != "" {
		w, err := NewConfigWatcher(configPath, logger)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create config watcher: %w", err)
		}
		watcher = w
	}

	manager := &DynamicConfigManager{
		staticConfig: staticConfig,
		watcher:      watcher,
		ctx:          ctx,
		cancel:       cancel,
		callbacks:    make([]ConfigChangeCallback, 0),
		logger:       logger,
	}

	// Register default callback to update static config
	if watcher != nil {
		watcher.OnChange(func(newConfig *DynamicConfig) {
			manager.handleConfigChange(newConfig)
		})
	}

	return manager, nil
}

// Start begins watching for configuration changes
func (m *DynamicConfigManager) Start() error {
	if m.watcher != nil {
		m.watcher.Start()
	}

	// Start periodic health check
	go m.healthCheckLoop()

	m.logger.Info("Dynamic configuration manager started")
	return nil
}

// Stop stops the configuration manager
func (m *DynamicConfigManager) Stop() {
	m.cancel()

	if m.watcher != nil {
		m.watcher.Stop()
	}

	m.logger.Info("Dynamic configuration manager stopped")
}

// healthCheckLoop periodically checks configuration health
func (m *DynamicConfigManager) healthCheckLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// performHealthCheck validates current configuration
func (m *DynamicConfigManager) performHealthCheck() {
	if m.watcher == nil {
		return
	}

	current := m.watcher.GetCurrent()
	if err := m.watcher.validateConfig(current); err != nil {
		m.logger.Error("Configuration health check failed",
			zap.Error(err),
		)
	}
}

// handleConfigChange handles configuration changes
func (m *DynamicConfigManager) handleConfigChange(newConfig *DynamicConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get old config for comparison
	oldFeatures := m.staticConfig.Features
	oldLimits := m.staticConfig.EdgeCreation

	// Update static config with dynamic values
	m.staticConfig.Features = newConfig.Features
	m.staticConfig.EdgeCreation.SyncEdgeLimit = newConfig.Limits.SyncEdgeLimit
	m.staticConfig.EdgeCreation.MaxEdgesPerNode = newConfig.Limits.MaxEdgesPerNode

	// Log significant changes
	if oldFeatures.EnableSagaOrchestrator != newConfig.Features.EnableSagaOrchestrator {
		m.logger.Warn("EnableSagaOrchestrator flag is deprecated and will be forced on",
			zap.Bool("requested_enabled", newConfig.Features.EnableSagaOrchestrator),
		)
	}
	m.staticConfig.Features.EnableSagaOrchestrator = true

	if oldFeatures.EnableWebSocket != newConfig.Features.EnableWebSocket {
		m.logger.Info("WebSocket feature toggled",
			zap.Bool("enabled", newConfig.Features.EnableWebSocket),
		)
	}

	if oldLimits.SyncEdgeLimit != newConfig.Limits.SyncEdgeLimit {
		m.logger.Info("Sync edge limit changed",
			zap.Int("old", oldLimits.SyncEdgeLimit),
			zap.Int("new", newConfig.Limits.SyncEdgeLimit),
		)
	}

	// Notify callbacks
	for _, callback := range m.callbacks {
		go callback(nil, newConfig) // Run callbacks async to avoid blocking
	}
}

// OnChange registers a callback for configuration changes
func (m *DynamicConfigManager) OnChange(callback ConfigChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// GetConfig returns the current merged configuration
func (m *DynamicConfigManager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.staticConfig
}

// GetDynamicConfig returns the current dynamic configuration
func (m *DynamicConfigManager) GetDynamicConfig() *DynamicConfig {
	if m.watcher == nil {
		// Return defaults if no watcher
		return &DynamicConfig{
			Features: m.staticConfig.Features,
			Limits: Limits{
				SyncEdgeLimit:    m.staticConfig.EdgeCreation.SyncEdgeLimit,
				MaxNodesPerGraph: 10000,
				MaxEdgesPerNode:  m.staticConfig.EdgeCreation.MaxEdgesPerNode,
				MaxGraphsPerUser: 100,
			},
			DataLoader: DataLoaderConfig{
				Enabled:      true,
				BatchWindow:  10,
				MaxBatchSize: 25,
			},
			WebSocket: WebSocketConfig{
				Enabled:           m.staticConfig.Features.EnableWebSocket,
				MaxConnections:    10000,
				HeartbeatInterval: 30,
				MessageQueueSize:  1000,
			},
		}
	}

	return m.watcher.GetCurrent()
}

// IsFeatureEnabled checks if a feature is enabled
func (m *DynamicConfigManager) IsFeatureEnabled(feature string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch feature {
	case "saga_orchestrator":
		return m.staticConfig.Features.EnableSagaOrchestrator
	case "websocket":
		return m.staticConfig.Features.EnableWebSocket
	case "async_deletion":
		return m.staticConfig.Features.EnableAsyncDeletion
	case "auto_connect":
		return m.staticConfig.Features.EnableAutoConnect
	default:
		return false
	}
}

// GetLimit returns a specific limit value
func (m *DynamicConfigManager) GetLimit(limit string) int {
	if m.watcher == nil {
		// Return defaults
		switch limit {
		case "sync_edge_limit":
			return m.staticConfig.EdgeCreation.SyncEdgeLimit
		case "max_nodes_per_graph":
			return 10000
		case "max_edges_per_node":
			return m.staticConfig.EdgeCreation.MaxEdgesPerNode
		case "max_graphs_per_user":
			return 100
		default:
			return 0
		}
	}

	limits := m.watcher.GetLimits()
	switch limit {
	case "sync_edge_limit":
		return limits.SyncEdgeLimit
	case "max_nodes_per_graph":
		return limits.MaxNodesPerGraph
	case "max_edges_per_node":
		return limits.MaxEdgesPerNode
	case "max_graphs_per_user":
		return limits.MaxGraphsPerUser
	default:
		return 0
	}
}

// UpdateFeature updates a feature flag dynamically
func (m *DynamicConfigManager) UpdateFeature(feature string, enabled bool) error {
	if m.watcher == nil {
		return fmt.Errorf("dynamic configuration not available")
	}

	config := m.watcher.GetCurrent()

	switch feature {
	case "saga_orchestrator":
		config.Features.EnableSagaOrchestrator = enabled
	case "websocket":
		config.Features.EnableWebSocket = enabled
	case "async_deletion":
		config.Features.EnableAsyncDeletion = enabled
	case "auto_connect":
		config.Features.EnableAutoConnect = enabled
	default:
		return fmt.Errorf("unknown feature: %s", feature)
	}

	// Save updated configuration
	if err := m.watcher.SaveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	m.logger.Info("Feature updated",
		zap.String("feature", feature),
		zap.Bool("enabled", enabled),
	)

	return nil
}

// UpdateLimit updates a limit value dynamically
func (m *DynamicConfigManager) UpdateLimit(limit string, value int) error {
	if m.watcher == nil {
		return fmt.Errorf("dynamic configuration not available")
	}

	config := m.watcher.GetCurrent()

	switch limit {
	case "sync_edge_limit":
		config.Limits.SyncEdgeLimit = value
	case "max_nodes_per_graph":
		config.Limits.MaxNodesPerGraph = value
	case "max_edges_per_node":
		config.Limits.MaxEdgesPerNode = value
	case "max_graphs_per_user":
		config.Limits.MaxGraphsPerUser = value
	default:
		return fmt.Errorf("unknown limit: %s", limit)
	}

	// Save updated configuration
	if err := m.watcher.SaveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	m.logger.Info("Limit updated",
		zap.String("limit", limit),
		zap.Int("value", value),
	)

	return nil
}
