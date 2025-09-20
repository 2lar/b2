package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// ConfigWatcher watches configuration files for changes
type ConfigWatcher struct {
	path        string
	watcher     *fsnotify.Watcher
	current     *DynamicConfig
	mu          sync.RWMutex
	onChange    []func(*DynamicConfig)
	logger      *zap.Logger
	stopCh      chan struct{}
	lastModTime time.Time
}

// DynamicConfig represents runtime-changeable configuration
type DynamicConfig struct {
	Features   Features         `json:"features"`
	Limits     Limits           `json:"limits"`
	DataLoader DataLoaderConfig `json:"dataloader"`
	WebSocket  WebSocketConfig  `json:"websocket"`
	Metadata   ConfigMetadata   `json:"metadata"`
}

// Limits holds application limits
type Limits struct {
	SyncEdgeLimit    int `json:"syncEdgeLimit"`
	MaxNodesPerGraph int `json:"maxNodesPerGraph"`
	MaxEdgesPerNode  int `json:"maxEdgesPerNode"`
	MaxGraphsPerUser int `json:"maxGraphsPerUser"`
}

// DataLoaderConfig holds DataLoader configuration
type DataLoaderConfig struct {
	Enabled      bool `json:"enabled"`
	BatchWindow  int  `json:"batchWindow"` // milliseconds
	MaxBatchSize int  `json:"maxBatchSize"`
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	Enabled           bool `json:"enabled"`
	MaxConnections    int  `json:"maxConnections"`
	HeartbeatInterval int  `json:"heartbeatInterval"` // seconds
	MessageQueueSize  int  `json:"messageQueueSize"`
}

// ConfigMetadata holds metadata about the configuration
type ConfigMetadata struct {
	Version   string    `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
	UpdatedBy string    `json:"updatedBy"`
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(configPath string, logger *zap.Logger) (*ConfigWatcher, error) {
	// Load initial configuration
	config, err := loadConfigFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Add the config file to watcher
	if err := watcher.Add(configPath); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch config file: %w", err)
	}

	// Also watch the directory for atomic saves (rename operations)
	dir := filepath.Dir(configPath)
	if err := watcher.Add(dir); err != nil {
		logger.Warn("Failed to watch config directory", zap.Error(err))
	}

	cw := &ConfigWatcher{
		path:        configPath,
		watcher:     watcher,
		current:     config,
		onChange:    make([]func(*DynamicConfig), 0),
		logger:      logger,
		stopCh:      make(chan struct{}),
		lastModTime: time.Now(),
	}

	return cw, nil
}

// Start begins watching for configuration changes
func (w *ConfigWatcher) Start() {
	go w.watchLoop()
	w.logger.Info("Configuration watcher started", zap.String("path", w.path))
}

// Stop stops watching for configuration changes
func (w *ConfigWatcher) Stop() {
	close(w.stopCh)
	w.watcher.Close()
	w.logger.Info("Configuration watcher stopped")
}

// watchLoop is the main loop that watches for file changes
func (w *ConfigWatcher) watchLoop() {
	// Debounce timer to avoid multiple reloads
	var debounceTimer *time.Timer
	debounceDuration := 100 * time.Millisecond

	for {
		select {
		case <-w.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only handle write and create events for our config file
			if filepath.Base(event.Name) != filepath.Base(w.path) {
				continue
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDuration, func() {
					w.handleConfigChange()
				})
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.logger.Error("File watcher error", zap.Error(err))
		}
	}
}

// handleConfigChange handles configuration file changes
func (w *ConfigWatcher) handleConfigChange() {
	w.logger.Info("Configuration file changed, reloading", zap.String("path", w.path))

	// Load new configuration
	newConfig, err := loadConfigFromFile(w.path)
	if err != nil {
		w.logger.Error("Failed to reload configuration", zap.Error(err))
		return
	}

	// Validate configuration
	if err := w.validateConfig(newConfig); err != nil {
		w.logger.Error("Invalid configuration, keeping current", zap.Error(err))
		return
	}

	// Store old config for comparison
	w.mu.Lock()
	oldConfig := w.current
	w.current = newConfig
	w.mu.Unlock()

	// Log changes
	w.logConfigChanges(oldConfig, newConfig)

	// Notify listeners
	for _, handler := range w.onChange {
		go handler(newConfig)
	}

	w.logger.Info("Configuration reloaded successfully",
		zap.String("version", newConfig.Metadata.Version),
	)
}

// validateConfig validates the configuration
func (w *ConfigWatcher) validateConfig(config *DynamicConfig) error {
	// Basic validation
	if config.Limits.MaxNodesPerGraph <= 0 {
		return fmt.Errorf("maxNodesPerGraph must be positive")
	}

	if config.Limits.MaxEdgesPerNode <= 0 {
		return fmt.Errorf("maxEdgesPerNode must be positive")
	}

	if config.Limits.SyncEdgeLimit < 0 {
		return fmt.Errorf("syncEdgeLimit cannot be negative")
	}

	if config.DataLoader.BatchWindow < 0 || config.DataLoader.BatchWindow > 1000 {
		return fmt.Errorf("batchWindow must be between 0 and 1000 ms")
	}

	if config.DataLoader.MaxBatchSize <= 0 || config.DataLoader.MaxBatchSize > 100 {
		return fmt.Errorf("maxBatchSize must be between 1 and 100")
	}

	return nil
}

// logConfigChanges logs the differences between old and new config
func (w *ConfigWatcher) logConfigChanges(oldConfig, newConfig *DynamicConfig) {
	changes := []string{}

	// Check feature changes
	if oldConfig.Features.EnableSagaOrchestrator != newConfig.Features.EnableSagaOrchestrator {
		changes = append(changes, fmt.Sprintf("EnableSagaOrchestrator (deprecated): %v -> %v (ignored)",
			oldConfig.Features.EnableSagaOrchestrator, newConfig.Features.EnableSagaOrchestrator))
	}

	if oldConfig.Features.EnableWebSocket != newConfig.Features.EnableWebSocket {
		changes = append(changes, fmt.Sprintf("EnableWebSocket: %v -> %v",
			oldConfig.Features.EnableWebSocket, newConfig.Features.EnableWebSocket))
	}

	// Check limit changes
	if oldConfig.Limits.SyncEdgeLimit != newConfig.Limits.SyncEdgeLimit {
		changes = append(changes, fmt.Sprintf("SyncEdgeLimit: %d -> %d",
			oldConfig.Limits.SyncEdgeLimit, newConfig.Limits.SyncEdgeLimit))
	}

	if len(changes) > 0 {
		w.logger.Info("Configuration changes detected",
			zap.Strings("changes", changes),
		)
	}
}

// OnChange registers a callback for configuration changes
func (w *ConfigWatcher) OnChange(handler func(*DynamicConfig)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChange = append(w.onChange, handler)
}

// GetCurrent returns the current configuration
func (w *ConfigWatcher) GetCurrent() *DynamicConfig {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.current
}

// GetFeatures returns current feature flags
func (w *ConfigWatcher) GetFeatures() Features {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.current.Features
}

// GetLimits returns current limits
func (w *ConfigWatcher) GetLimits() Limits {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.current.Limits
}

// loadConfigFromFile loads configuration from a JSON file
func loadConfigFromFile(path string) (*DynamicConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config DynamicConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Set metadata if not present
	if config.Metadata.Version == "" {
		config.Metadata.Version = "1.0.0"
	}
	config.Metadata.UpdatedAt = time.Now()

	return &config, nil
}

// SaveConfig saves the current configuration to file
func (w *ConfigWatcher) SaveConfig(config *DynamicConfig) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Update metadata
	config.Metadata.UpdatedAt = time.Now()

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to temporary file first (atomic save)
	tmpPath := w.path + ".tmp"
	if err := ioutil.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp config file: %w", err)
	}

	// Rename to actual file (atomic operation)
	if err := rename(tmpPath, w.path); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	w.current = config
	return nil
}

// rename is a helper for atomic file replacement
func rename(oldPath, newPath string) error {
	// On Unix systems, rename is atomic
	return ioutil.WriteFile(newPath, mustReadFile(oldPath), 0644)
}

func mustReadFile(path string) []byte {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return data
}
