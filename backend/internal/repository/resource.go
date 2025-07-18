package repository

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ResourceManager manages repository resources and enforces limits
type ResourceManager struct {
	maxConnections    int
	connectionTimeout time.Duration
	operationTimeout  time.Duration
	connections       chan struct{}
	activeOperations  map[string]*OperationContext
	mu                sync.RWMutex
}

// OperationContext tracks the context and metadata of an active operation
type OperationContext struct {
	ID        string
	UserID    string
	Operation string
	StartTime time.Time
	Timeout   time.Duration
	Cancel    context.CancelFunc
}

// ResourceConfig defines resource management configuration
type ResourceConfig struct {
	MaxConnections    int           // Maximum concurrent connections
	ConnectionTimeout time.Duration // Timeout for acquiring connections
	OperationTimeout  time.Duration // Default operation timeout
	CleanupInterval   time.Duration // Interval for cleanup operations
}

// DefaultResourceConfig returns default resource configuration
func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		MaxConnections:    100,
		ConnectionTimeout: 10 * time.Second,
		OperationTimeout:  30 * time.Second,
		CleanupInterval:   5 * time.Minute,
	}
}

// NewResourceManager creates a new resource manager
func NewResourceManager(config ResourceConfig) *ResourceManager {
	rm := &ResourceManager{
		maxConnections:    config.MaxConnections,
		connectionTimeout: config.ConnectionTimeout,
		operationTimeout:  config.OperationTimeout,
		connections:       make(chan struct{}, config.MaxConnections),
		activeOperations:  make(map[string]*OperationContext),
	}
	
	// Fill the connection pool
	for i := 0; i < config.MaxConnections; i++ {
		rm.connections <- struct{}{}
	}
	
	// Start cleanup goroutine
	go rm.cleanupExpiredOperations(config.CleanupInterval)
	
	return rm
}

// AcquireConnection acquires a connection from the pool
func (rm *ResourceManager) AcquireConnection(ctx context.Context) error {
	select {
	case <-rm.connections:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(rm.connectionTimeout):
		return NewTimeoutError("connection acquisition", rm.connectionTimeout)
	}
}

// ReleaseConnection releases a connection back to the pool
func (rm *ResourceManager) ReleaseConnection() {
	select {
	case rm.connections <- struct{}{}:
	default:
		// Pool is full, which shouldn't happen in normal operation
		fmt.Println("Warning: Connection pool is full")
	}
}

// StartOperation starts tracking an operation
func (rm *ResourceManager) StartOperation(ctx context.Context, userID, operation string) (context.Context, error) {
	operationID := fmt.Sprintf("%s_%s_%d", userID, operation, time.Now().UnixNano())
	
	// Create operation context with timeout
	operationCtx, cancel := context.WithTimeout(ctx, rm.operationTimeout)
	
	operationContext := &OperationContext{
		ID:        operationID,
		UserID:    userID,
		Operation: operation,
		StartTime: time.Now(),
		Timeout:   rm.operationTimeout,
		Cancel:    cancel,
	}
	
	rm.mu.Lock()
	rm.activeOperations[operationID] = operationContext
	rm.mu.Unlock()
	
	// Add operation ID to context
	operationCtx = context.WithValue(operationCtx, "operation_id", operationID)
	
	return operationCtx, nil
}

// EndOperation stops tracking an operation
func (rm *ResourceManager) EndOperation(ctx context.Context) {
	if operationID, ok := ctx.Value("operation_id").(string); ok {
		rm.mu.Lock()
		if operationContext, exists := rm.activeOperations[operationID]; exists {
			operationContext.Cancel()
			delete(rm.activeOperations, operationID)
		}
		rm.mu.Unlock()
	}
}

// GetActiveOperations returns a snapshot of active operations
func (rm *ResourceManager) GetActiveOperations() []OperationContext {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	operations := make([]OperationContext, 0, len(rm.activeOperations))
	for _, op := range rm.activeOperations {
		operations = append(operations, *op)
	}
	
	return operations
}

// CancelOperation cancels a specific operation
func (rm *ResourceManager) CancelOperation(operationID string) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	if operationContext, exists := rm.activeOperations[operationID]; exists {
		operationContext.Cancel()
		delete(rm.activeOperations, operationID)
		return true
	}
	
	return false
}

// CancelUserOperations cancels all operations for a specific user
func (rm *ResourceManager) CancelUserOperations(userID string) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	cancelled := 0
	for id, operationContext := range rm.activeOperations {
		if operationContext.UserID == userID {
			operationContext.Cancel()
			delete(rm.activeOperations, id)
			cancelled++
		}
	}
	
	return cancelled
}

// cleanupExpiredOperations removes expired operations
func (rm *ResourceManager) cleanupExpiredOperations(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for range ticker.C {
		rm.mu.Lock()
		now := time.Now()
		for id, operationContext := range rm.activeOperations {
			if now.Sub(operationContext.StartTime) > operationContext.Timeout {
				operationContext.Cancel()
				delete(rm.activeOperations, id)
			}
		}
		rm.mu.Unlock()
	}
}

// ConnectionPool manages database connections with health checking
type ConnectionPool struct {
	connections    chan Connection
	healthChecker  HealthChecker
	config         PoolConfig
	mu             sync.RWMutex
	closed         bool
}

// Connection represents a database connection
type Connection interface {
	IsHealthy() bool
	Close() error
	LastUsed() time.Time
}

// HealthChecker checks connection health
type HealthChecker interface {
	CheckHealth(ctx context.Context, conn Connection) error
}

// PoolConfig defines connection pool configuration
type PoolConfig struct {
	MinConnections    int           // Minimum number of connections
	MaxConnections    int           // Maximum number of connections
	MaxIdleTime       time.Duration // Maximum idle time before closing
	HealthCheckPeriod time.Duration // Health check interval
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MinConnections:    5,
		MaxConnections:    50,
		MaxIdleTime:       30 * time.Minute,
		HealthCheckPeriod: 5 * time.Minute,
	}
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config PoolConfig, healthChecker HealthChecker) *ConnectionPool {
	pool := &ConnectionPool{
		connections:   make(chan Connection, config.MaxConnections),
		healthChecker: healthChecker,
		config:        config,
	}
	
	// Start health checker
	go pool.healthCheckLoop()
	
	return pool
}

// Get retrieves a connection from the pool
func (cp *ConnectionPool) Get(ctx context.Context) (Connection, error) {
	cp.mu.RLock()
	if cp.closed {
		cp.mu.RUnlock()
		return nil, fmt.Errorf("connection pool is closed")
	}
	cp.mu.RUnlock()
	
	select {
	case conn := <-cp.connections:
		if conn.IsHealthy() {
			return conn, nil
		}
		// Connection is unhealthy, close it and try again
		conn.Close()
		return cp.Get(ctx)
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// No connections available, might need to create a new one
		return nil, fmt.Errorf("no connections available")
	}
}

// Put returns a connection to the pool
func (cp *ConnectionPool) Put(conn Connection) {
	cp.mu.RLock()
	if cp.closed {
		cp.mu.RUnlock()
		conn.Close()
		return
	}
	cp.mu.RUnlock()
	
	if !conn.IsHealthy() {
		conn.Close()
		return
	}
	
	select {
	case cp.connections <- conn:
	default:
		// Pool is full, close the connection
		conn.Close()
	}
}

// Close closes all connections in the pool
func (cp *ConnectionPool) Close() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	if cp.closed {
		return nil
	}
	
	cp.closed = true
	
	// Close all connections
	close(cp.connections)
	for conn := range cp.connections {
		conn.Close()
	}
	
	return nil
}

// healthCheckLoop periodically checks connection health
func (cp *ConnectionPool) healthCheckLoop() {
	ticker := time.NewTicker(cp.config.HealthCheckPeriod)
	defer ticker.Stop()
	
	for range ticker.C {
		cp.mu.RLock()
		if cp.closed {
			cp.mu.RUnlock()
			return
		}
		cp.mu.RUnlock()
		
		cp.performHealthCheck()
	}
}

// performHealthCheck checks health of all connections
func (cp *ConnectionPool) performHealthCheck() {
	var healthyConnections []Connection
	
	// Check all connections
	for {
		select {
		case conn := <-cp.connections:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := cp.healthChecker.CheckHealth(ctx, conn); err != nil {
				conn.Close()
			} else {
				healthyConnections = append(healthyConnections, conn)
			}
			cancel()
		default:
			// No more connections to check
			goto done
		}
	}
	
done:
	// Return healthy connections to pool
	for _, conn := range healthyConnections {
		select {
		case cp.connections <- conn:
		default:
			// Pool is full
			conn.Close()
		}
	}
}

// TimeoutContext creates a context with timeout and cleanup
type TimeoutContext struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewTimeoutContext creates a new timeout context
func NewTimeoutContext(parent context.Context, timeout time.Duration) *TimeoutContext {
	ctx, cancel := context.WithTimeout(parent, timeout)
	
	tc := &TimeoutContext{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	
	// Start timeout monitor
	go tc.monitor()
	
	return tc
}

// Context returns the underlying context
func (tc *TimeoutContext) Context() context.Context {
	return tc.ctx
}

// Cancel cancels the context
func (tc *TimeoutContext) Cancel() {
	tc.cancel()
	close(tc.done)
}

// monitor monitors the context for completion
func (tc *TimeoutContext) monitor() {
	select {
	case <-tc.ctx.Done():
		// Context completed (timeout or cancellation)
	case <-tc.done:
		// Manual cancellation
	}
	
	// Cleanup resources
	tc.cancel()
}

// RateLimiter implements rate limiting for operations
type RateLimiter struct {
	tokens   chan struct{}
	refill   time.Duration
	capacity int
	mu       sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(capacity int, refillRate time.Duration) *RateLimiter {
	rl := &RateLimiter{
		tokens:   make(chan struct{}, capacity),
		refill:   refillRate,
		capacity: capacity,
	}
	
	// Fill initial tokens
	for i := 0; i < capacity; i++ {
		rl.tokens <- struct{}{}
	}
	
	// Start refill goroutine
	go rl.refillTokens()
	
	return rl
}

// Allow checks if an operation is allowed
func (rl *RateLimiter) Allow(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return NewRateLimitError("rate limit exceeded", rl.refill)
	}
}

// refillTokens periodically refills the token bucket
func (rl *RateLimiter) refillTokens() {
	ticker := time.NewTicker(rl.refill)
	defer ticker.Stop()
	
	for range ticker.C {
		select {
		case rl.tokens <- struct{}{}:
		default:
			// Bucket is full
		}
	}
}

// ResourceMonitor monitors resource usage
type ResourceMonitor struct {
	metrics map[string]int64
	mu      sync.RWMutex
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		metrics: make(map[string]int64),
	}
}

// Increment increments a metric
func (rm *ResourceMonitor) Increment(metric string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.metrics[metric]++
}

// Decrement decrements a metric
func (rm *ResourceMonitor) Decrement(metric string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.metrics[metric]--
}

// Get gets a metric value
func (rm *ResourceMonitor) Get(metric string) int64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.metrics[metric]
}

// GetAll gets all metrics
func (rm *ResourceMonitor) GetAll() map[string]int64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	result := make(map[string]int64, len(rm.metrics))
	for k, v := range rm.metrics {
		result[k] = v
	}
	return result
}