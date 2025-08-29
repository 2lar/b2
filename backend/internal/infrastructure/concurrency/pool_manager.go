package concurrency

import (
	"context"
	"log"
	"sync"
	"time"

	"brain2-backend/internal/config"
)

// Global pool manager instance that survives Lambda warm invocations
var (
	globalPoolManager *PoolManager
	poolInitOnce      sync.Once
)

// PoolManager manages all worker pools and provides centralized access
// This is designed to survive Lambda warm invocations for better performance
type PoolManager struct {
	// Pools for different workload types
	nodePool       *AdaptiveWorkerPool
	connectionPool *AdaptiveWorkerPool
	generalPool    *AdaptiveWorkerPool
	
	// Batch processors
	batchProcessor *BatchProcessor
	
	// Configuration
	config      *config.Config
	environment RuntimeEnvironment
	
	// Metrics and monitoring
	metrics       *PoolMetrics
	coldStartTime time.Time
	isColdStart   bool
	
	// Synchronization
	mu sync.RWMutex
}

// GetPoolManager returns the singleton pool manager instance
// Uses lazy initialization to avoid cold start overhead
func GetPoolManager(cfg *config.Config) *PoolManager {
	poolInitOnce.Do(func() {
		globalPoolManager = newPoolManager(cfg)
		log.Printf("Pool manager initialized for environment: %s", globalPoolManager.environment)
	})
	return globalPoolManager
}

// newPoolManager creates a new pool manager instance
func newPoolManager(cfg *config.Config) *PoolManager {
	env := DetectEnvironment()
	
	manager := &PoolManager{
		config:      cfg,
		environment: env,
		isColdStart: true,
	}
	
	// Initialize metrics collector
	manager.metrics = NewPoolMetrics(env, "global")
	
	// Create pools but don't start workers yet (lazy initialization)
	manager.initializePools()
	
	return manager
}

// initializePools creates pool instances without starting workers
func (m *PoolManager) initializePools() {
	// Get environment-specific configuration
	concurrencyConfig := m.getEnvironmentConfig()
	
	// Create pool configurations
	nodePoolConfig := &PoolConfig{
		Environment: m.environment,
		MaxWorkers:  concurrencyConfig.MaxWorkers,
		BatchSize:   concurrencyConfig.BatchSize,
		QueueSize:   concurrencyConfig.QueueSize,
	}
	
	// For connection analysis, use slightly different settings
	connectionPoolConfig := &PoolConfig{
		Environment: m.environment,
		MaxWorkers:  max(2, concurrencyConfig.MaxWorkers/2), // Fewer workers for connections
		BatchSize:   concurrencyConfig.BatchSize,
		QueueSize:   concurrencyConfig.QueueSize / 2,
	}
	
	// General purpose pool
	generalPoolConfig := &PoolConfig{
		Environment: m.environment,
		MaxWorkers:  concurrencyConfig.MaxWorkers,
		BatchSize:   concurrencyConfig.BatchSize,
		QueueSize:   concurrencyConfig.QueueSize,
	}
	
	// Create pools (workers will be started on first use)
	ctx := context.Background()
	m.nodePool = NewAdaptiveWorkerPool(ctx, nodePoolConfig)
	m.connectionPool = NewAdaptiveWorkerPool(ctx, connectionPoolConfig)
	m.generalPool = NewAdaptiveWorkerPool(ctx, generalPoolConfig)
	
	// Create batch processor
	m.batchProcessor = NewBatchProcessor(ctx, generalPoolConfig)
}

// getEnvironmentConfig returns configuration for the current environment
func (m *PoolManager) getEnvironmentConfig() config.EnvironmentConcurrency {
	if m.config == nil {
		// Return defaults if config not available
		return m.getDefaultConfig()
	}
	
	switch m.environment {
	case EnvironmentLambda:
		if envConfig := m.config.GetLambdaConcurrency(); envConfig != nil {
			return *envConfig
		}
	case EnvironmentECS:
		if envConfig := m.config.GetECSConcurrency(); envConfig != nil {
			return *envConfig
		}
	case EnvironmentLocal:
		if envConfig := m.config.GetLocalConcurrency(); envConfig != nil {
			return *envConfig
		}
	}
	
	return m.getDefaultConfig()
}

// getDefaultConfig returns default configuration based on environment
func (m *PoolManager) getDefaultConfig() config.EnvironmentConcurrency {
	switch m.environment {
	case EnvironmentLambda:
		return config.EnvironmentConcurrency{
			MaxWorkers:    GetOptimalWorkerCount(EnvironmentLambda),
			BatchSize:     GetOptimalBatchSize(EnvironmentLambda),
			QueueSize:     100,
			TimeoutBuffer: 10 * time.Second,
			EnableMetrics: true,
		}
	case EnvironmentECS:
		return config.EnvironmentConcurrency{
			MaxWorkers:    GetOptimalWorkerCount(EnvironmentECS),
			BatchSize:     GetOptimalBatchSize(EnvironmentECS),
			QueueSize:     1000,
			TimeoutBuffer: 30 * time.Second,
			EnableMetrics: true,
		}
	default:
		return config.EnvironmentConcurrency{
			MaxWorkers:    GetOptimalWorkerCount(EnvironmentLocal),
			BatchSize:     GetOptimalBatchSize(EnvironmentLocal),
			QueueSize:     500,
			TimeoutBuffer: 60 * time.Second,
			EnableMetrics: false,
		}
	}
}

// GetNodePool returns the pool optimized for node operations
func (m *PoolManager) GetNodePool() *AdaptiveWorkerPool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nodePool
}

// GetConnectionPool returns the pool optimized for connection analysis
func (m *PoolManager) GetConnectionPool() *AdaptiveWorkerPool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connectionPool
}

// GetGeneralPool returns the general purpose worker pool
func (m *PoolManager) GetGeneralPool() *AdaptiveWorkerPool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.generalPool
}

// GetBatchProcessor returns the batch processor instance
func (m *PoolManager) GetBatchProcessor() *BatchProcessor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.batchProcessor
}

// SetColdStartTime records when the cold start began
func (m *PoolManager) SetColdStartTime(startTime time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.coldStartTime = startTime
	m.isColdStart = true
	
	// Log cold start duration when first request arrives
	if !startTime.IsZero() {
		duration := time.Since(startTime)
		log.Printf("Cold start duration: %v", duration)
		
		if m.metrics != nil {
			m.metrics.RecordColdStart(duration)
		}
	}
}

// MarkWarmStart indicates that we're no longer in a cold start
func (m *PoolManager) MarkWarmStart() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isColdStart = false
}

// IsWarmStart returns true if the Lambda is warm
func (m *PoolManager) IsWarmStart() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return !m.isColdStart
}

// Shutdown gracefully shuts down all pools
func (m *PoolManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	log.Println("Shutting down pool manager...")
	
	// Stop all pools
	if m.nodePool != nil {
		m.nodePool.Stop()
	}
	if m.connectionPool != nil {
		m.connectionPool.Stop()
	}
	if m.generalPool != nil {
		m.generalPool.Stop()
	}
	
	// Emit final metrics
	if m.metrics != nil && m.environment == EnvironmentLambda {
		m.metrics.EmitFinalMetrics()
	}
	
	log.Println("Pool manager shutdown complete")
	return nil
}

// GetStats returns statistics for all pools
func (m *PoolManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := map[string]interface{}{
		"environment":   string(m.environment),
		"is_cold_start": m.isColdStart,
	}
	
	if m.nodePool != nil {
		stats["node_pool"] = m.nodePool.GetStats()
	}
	if m.connectionPool != nil {
		stats["connection_pool"] = m.connectionPool.GetStats()
	}
	if m.generalPool != nil {
		stats["general_pool"] = m.generalPool.GetStats()
	}
	
	if !m.coldStartTime.IsZero() {
		stats["time_since_cold_start"] = time.Since(m.coldStartTime).String()
	}
	
	return stats
}

// Helper function to get max of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}