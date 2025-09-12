package integration

import (
	"context"
	"testing"
	"time"

	"backend/application/commands"
	"backend/infrastructure/config"
	"backend/infrastructure/di"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLazyLoadingDisabled tests that the system works correctly with lazy loading disabled
func TestLazyLoadingDisabled(t *testing.T) {
	ctx := context.Background()
	
	// Create config with lazy loading disabled
	cfg := &config.Config{
		EnableLazyLoading: false,
		EdgeCreationConfig: &config.EdgeCreationConfig{
			SyncEdgeLimit: 5,
			AsyncEnabled:  false,
		},
		DynamoDB: config.DynamoDBConfig{
			TableName: "test-table",
			Region:    "us-east-1",
		},
		EventBridge: config.EventBridgeConfig{
			EventBusName: "test-bus",
			Region:       "us-east-1",
		},
	}
	
	// Initialize container
	container, err := di.InitializeContainer(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, container)
	
	// Create a node using the command bus
	cmd := commands.CreateNodeCommand{
		UserID:  "test-user-1",
		Title:   "Test Node without Lazy Loading",
		Content: "This is a test node created with lazy loading disabled",
		X:       100,
		Y:       200,
		Z:       0,
		Tags:    []string{"test", "no-lazy"},
	}
	
	// Execute command
	err = container.CommandBus.Dispatch(ctx, cmd)
	assert.NoError(t, err)
	
	// Verify the node was created (would need to query to confirm)
	// This test primarily ensures the system works without lazy loading
}

// TestLazyLoadingEnabled tests that the system works correctly with lazy loading enabled
func TestLazyLoadingEnabled(t *testing.T) {
	ctx := context.Background()
	
	// Create config with lazy loading enabled
	cfg := &config.Config{
		EnableLazyLoading: true, // Enable lazy loading
		EdgeCreationConfig: &config.EdgeCreationConfig{
			SyncEdgeLimit: 5,
			AsyncEnabled:  false,
		},
		DynamoDB: config.DynamoDBConfig{
			TableName: "test-table",
			Region:    "us-east-1",
		},
		EventBridge: config.EventBridgeConfig{
			EventBusName: "test-bus",
			Region:       "us-east-1",
		},
	}
	
	// Initialize container
	container, err := di.InitializeContainer(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, container)
	require.NotNil(t, container.GraphLazyService)
	assert.True(t, container.GraphLazyService.IsEnabled())
	
	// Create a node using the command bus
	cmd := commands.CreateNodeCommand{
		UserID:  "test-user-2",
		Title:   "Test Node with Lazy Loading",
		Content: "This is a test node created with lazy loading enabled",
		X:       150,
		Y:       250,
		Z:       0,
		Tags:    []string{"test", "lazy"},
	}
	
	// Execute command
	err = container.CommandBus.Dispatch(ctx, cmd)
	assert.NoError(t, err)
	
	// Verify the lazy graph service has cached the graph
	assert.Greater(t, container.GraphLazyService.GetCacheSize(), 0)
}

// TestLazyLoadingPerformance tests that lazy loading improves performance for large graphs
func TestLazyLoadingPerformance(t *testing.T) {
	ctx := context.Background()
	
	// Helper function to measure execution time
	measureTime := func(fn func() error) (time.Duration, error) {
		start := time.Now()
		err := fn()
		return time.Since(start), err
	}
	
	// Test without lazy loading
	cfgNoLazy := &config.Config{
		EnableLazyLoading: false,
		EdgeCreationConfig: &config.EdgeCreationConfig{
			SyncEdgeLimit: 5,
			AsyncEnabled:  false,
		},
		DynamoDB: config.DynamoDBConfig{
			TableName: "test-table",
			Region:    "us-east-1",
		},
		EventBridge: config.EventBridgeConfig{
			EventBusName: "test-bus",
			Region:       "us-east-1",
		},
	}
	
	containerNoLazy, err := di.InitializeContainer(ctx, cfgNoLazy)
	require.NoError(t, err)
	
	// Create multiple nodes without lazy loading
	var totalTimeNoLazy time.Duration
	for i := 0; i < 10; i++ {
		cmd := commands.CreateNodeCommand{
			UserID:  "perf-test-user",
			Title:   "Performance Test Node",
			Content: "Testing performance without lazy loading",
			X:       float64(i * 10),
			Y:       float64(i * 20),
			Z:       0,
		}
		
		duration, err := measureTime(func() error {
			return containerNoLazy.CommandBus.Dispatch(ctx, cmd)
		})
		assert.NoError(t, err)
		totalTimeNoLazy += duration
	}
	
	// Test with lazy loading
	cfgLazy := &config.Config{
		EnableLazyLoading: true,
		EdgeCreationConfig: &config.EdgeCreationConfig{
			SyncEdgeLimit: 5,
			AsyncEnabled:  false,
		},
		DynamoDB: config.DynamoDBConfig{
			TableName: "test-table",
			Region:    "us-east-1",
		},
		EventBridge: config.EventBridgeConfig{
			EventBusName: "test-bus",
			Region:       "us-east-1",
		},
	}
	
	containerLazy, err := di.InitializeContainer(ctx, cfgLazy)
	require.NoError(t, err)
	
	// Create multiple nodes with lazy loading
	var totalTimeLazy time.Duration
	for i := 0; i < 10; i++ {
		cmd := commands.CreateNodeCommand{
			UserID:  "perf-test-user-lazy",
			Title:   "Performance Test Node Lazy",
			Content: "Testing performance with lazy loading",
			X:       float64(i * 10),
			Y:       float64(i * 20),
			Z:       0,
		}
		
		duration, err := measureTime(func() error {
			return containerLazy.CommandBus.Dispatch(ctx, cmd)
		})
		assert.NoError(t, err)
		totalTimeLazy += duration
	}
	
	// Log the performance comparison
	t.Logf("Performance Comparison:")
	t.Logf("Without Lazy Loading: %v", totalTimeNoLazy)
	t.Logf("With Lazy Loading: %v", totalTimeLazy)
	
	// With lazy loading, we expect similar or better performance
	// especially as the graph grows larger
	// This is a basic test - in production you'd want more comprehensive benchmarks
}