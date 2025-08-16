// Package config provides configuration management for the Brain2 backend.
// This file implements hot reloading of configuration in development.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// ConfigWatcher watches for configuration changes and hot reloads them.
// This is primarily used in development environments for faster iteration.
type ConfigWatcher struct {
	config    *Config
	callbacks []func(*Config)
	mu        sync.RWMutex
	logger    *zap.Logger
	watcher   *fsnotify.Watcher
	stopCh    chan struct{}
}

// NewConfigWatcher creates a new configuration watcher.
func NewConfigWatcher(initial *Config, logger *zap.Logger) (*ConfigWatcher, error) {
	watcher := &ConfigWatcher{
		config:    initial,
		callbacks: make([]func(*Config), 0),
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
	
	// Only enable hot reloading in development
	if initial.Environment == Development {
		fsWatcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, fmt.Errorf("failed to create file watcher: %w", err)
		}
		watcher.watcher = fsWatcher
		
		// Start watching configuration files
		if err := watcher.watchConfigFiles(); err != nil {
			fsWatcher.Close()
			return nil, fmt.Errorf("failed to watch config files: %w", err)
		}
		
		// Start the watcher goroutine
		go watcher.watchLoop()
		
		logger.Info("Configuration hot reloading enabled",
			zap.String("environment", string(initial.Environment)),
		)
	} else {
		logger.Info("Configuration hot reloading disabled",
			zap.String("environment", string(initial.Environment)),
		)
	}
	
	return watcher, nil
}

// watchConfigFiles adds configuration files to the watcher.
func (w *ConfigWatcher) watchConfigFiles() error {
	// Watch the main config directory
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "./config"
	}
	
	// Watch all YAML and JSON files in the config directory
	err := filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}
		
		// Watch directories and config files
		if info.IsDir() || isConfigFile(path) {
			if err := w.watcher.Add(path); err != nil {
				w.logger.Warn("Failed to watch file",
					zap.String("path", path),
					zap.Error(err),
				)
			} else {
				w.logger.Debug("Watching config file",
					zap.String("path", path),
				)
			}
		}
		
		return nil
	})
	
	if err != nil {
		return fmt.Errorf("failed to walk config directory: %w", err)
	}
	
	// Also watch environment-specific files
	envFile := fmt.Sprintf(".env.%s", w.config.Environment)
	if _, err := os.Stat(envFile); err == nil {
		if err := w.watcher.Add(envFile); err != nil {
			w.logger.Warn("Failed to watch env file",
				zap.String("file", envFile),
				zap.Error(err),
			)
		}
	}
	
	return nil
}

// watchLoop monitors for file changes and triggers reloads.
func (w *ConfigWatcher) watchLoop() {
	defer w.watcher.Close()
	
	// Debounce timer to avoid multiple rapid reloads
	var debounceTimer *time.Timer
	const debounceDelay = 500 * time.Millisecond
	
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			
			// Handle file change events
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				if isConfigFile(event.Name) {
					w.logger.Info("Configuration file changed",
						zap.String("file", event.Name),
						zap.String("operation", event.Op.String()),
					)
					
					// Cancel previous timer if exists
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					
					// Set new debounce timer
					debounceTimer = time.AfterFunc(debounceDelay, func() {
						w.reloadConfig()
					})
				}
			}
			
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.logger.Error("File watcher error", zap.Error(err))
			
		case <-w.stopCh:
			w.logger.Info("Stopping configuration watcher")
			return
		}
	}
}

// reloadConfig reloads the configuration from files.
func (w *ConfigWatcher) reloadConfig() {
	w.logger.Info("Reloading configuration...")
	
	// Load new configuration
	newConfig := LoadConfig()
	
	// Validate new configuration
	if err := newConfig.Validate(); err != nil {
		w.logger.Error("Invalid configuration after reload",
			zap.Error(err),
		)
		return
	}
	
	// Check if configuration actually changed
	if w.configsEqual(w.config, &newConfig) {
		w.logger.Debug("Configuration unchanged after reload")
		return
	}
	
	// Update configuration
	w.mu.Lock()
	oldConfig := w.config
	w.config = &newConfig
	w.mu.Unlock()
	
	// Log configuration changes
	w.logConfigChanges(oldConfig, &newConfig)
	
	// Notify all callbacks
	w.notifyCallbacks(&newConfig)
	
	w.logger.Info("Configuration reloaded successfully",
		zap.Int("callbacks_notified", len(w.callbacks)),
	)
}

// OnChange registers a callback to be called when configuration changes.
func (w *ConfigWatcher) OnChange(callback func(*Config)) {
	w.mu.Lock()
	w.callbacks = append(w.callbacks, callback)
	w.mu.Unlock()
	
	w.logger.Debug("Registered configuration change callback",
		zap.Int("total_callbacks", len(w.callbacks)),
	)
}

// GetConfig returns the current configuration.
func (w *ConfigWatcher) GetConfig() *Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}

// Stop stops the configuration watcher.
func (w *ConfigWatcher) Stop() {
	if w.watcher != nil {
		close(w.stopCh)
		w.watcher.Close()
	}
}

// notifyCallbacks notifies all registered callbacks of configuration change.
func (w *ConfigWatcher) notifyCallbacks(newConfig *Config) {
	w.mu.RLock()
	callbacks := make([]func(*Config), len(w.callbacks))
	copy(callbacks, w.callbacks)
	w.mu.RUnlock()
	
	for i, callback := range callbacks {
		// Run callbacks in goroutines to avoid blocking
		go func(idx int, cb func(*Config)) {
			defer func() {
				if r := recover(); r != nil {
					w.logger.Error("Callback panicked",
						zap.Int("callback_index", idx),
						zap.Any("panic", r),
					)
				}
			}()
			
			cb(newConfig)
		}(i, callback)
	}
}

// configsEqual checks if two configurations are equal.
func (w *ConfigWatcher) configsEqual(a, b *Config) bool {
	// Simple comparison - in production, use deep equality
	return a.Environment == b.Environment &&
		a.Server.Port == b.Server.Port &&
		a.Database.TableName == b.Database.TableName &&
		a.AWS.Region == b.AWS.Region
}

// logConfigChanges logs what changed between configurations.
func (w *ConfigWatcher) logConfigChanges(old, new *Config) {
	changes := make([]string, 0)
	
	if old.Server.Port != new.Server.Port {
		changes = append(changes, fmt.Sprintf("port: %d -> %d", old.Server.Port, new.Server.Port))
	}
	
	if old.Database.TableName != new.Database.TableName {
		changes = append(changes, fmt.Sprintf("table: %s -> %s", old.Database.TableName, new.Database.TableName))
	}
	
	if old.Features.EnableCaching != new.Features.EnableCaching {
		changes = append(changes, fmt.Sprintf("caching: %v -> %v", old.Features.EnableCaching, new.Features.EnableCaching))
	}
	
	if old.Features.EnableMetrics != new.Features.EnableMetrics {
		changes = append(changes, fmt.Sprintf("metrics: %v -> %v", old.Features.EnableMetrics, new.Features.EnableMetrics))
	}
	
	if len(changes) > 0 {
		w.logger.Info("Configuration changes detected",
			zap.Strings("changes", changes),
		)
	}
}

// isConfigFile checks if a file is a configuration file.
func isConfigFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".env"
}

// ============================================================================
// CONFIGURATION RELOADER FOR SPECIFIC COMPONENTS
// ============================================================================

// ComponentReloader handles configuration reloading for specific components.
type ComponentReloader struct {
	name     string
	reloadFn func(*Config) error
	logger   *zap.Logger
}

// NewComponentReloader creates a new component reloader.
func NewComponentReloader(name string, reloadFn func(*Config) error, logger *zap.Logger) *ComponentReloader {
	return &ComponentReloader{
		name:     name,
		reloadFn: reloadFn,
		logger:   logger,
	}
}

// Reload reloads the component with new configuration.
func (r *ComponentReloader) Reload(config *Config) {
	r.logger.Info("Reloading component",
		zap.String("component", r.name),
	)
	
	if err := r.reloadFn(config); err != nil {
		r.logger.Error("Failed to reload component",
			zap.String("component", r.name),
			zap.Error(err),
		)
	} else {
		r.logger.Info("Component reloaded successfully",
			zap.String("component", r.name),
		)
	}
}

// ============================================================================
// CONFIGURATION MANAGER WITH HOT RELOAD
// ============================================================================

// ConfigManager manages configuration with hot reload support.
type ConfigManager struct {
	watcher    *ConfigWatcher
	reloaders  map[string]*ComponentReloader
	mu         sync.RWMutex
	logger     *zap.Logger
}

// NewConfigManager creates a new configuration manager.
func NewConfigManager(config *Config, logger *zap.Logger) (*ConfigManager, error) {
	watcher, err := NewConfigWatcher(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create config watcher: %w", err)
	}
	
	manager := &ConfigManager{
		watcher:   watcher,
		reloaders: make(map[string]*ComponentReloader),
		logger:    logger,
	}
	
	// Register for configuration changes
	watcher.OnChange(manager.handleConfigChange)
	
	return manager, nil
}

// RegisterComponent registers a component for configuration reloading.
func (m *ConfigManager) RegisterComponent(name string, reloadFn func(*Config) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	reloader := NewComponentReloader(name, reloadFn, m.logger)
	m.reloaders[name] = reloader
	
	m.logger.Info("Registered component for hot reload",
		zap.String("component", name),
		zap.Int("total_components", len(m.reloaders)),
	)
}

// handleConfigChange handles configuration changes.
func (m *ConfigManager) handleConfigChange(config *Config) {
	m.mu.RLock()
	reloaders := make([]*ComponentReloader, 0, len(m.reloaders))
	for _, reloader := range m.reloaders {
		reloaders = append(reloaders, reloader)
	}
	m.mu.RUnlock()
	
	// Reload all components
	for _, reloader := range reloaders {
		reloader.Reload(config)
	}
}

// GetConfig returns the current configuration.
func (m *ConfigManager) GetConfig() *Config {
	return m.watcher.GetConfig()
}

// Stop stops the configuration manager.
func (m *ConfigManager) Stop() {
	m.watcher.Stop()
}