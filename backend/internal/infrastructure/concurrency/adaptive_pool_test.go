package concurrency

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestDetectEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected RuntimeEnvironment
	}{
		{
			name: "Lambda environment",
			envVars: map[string]string{
				"AWS_LAMBDA_FUNCTION_NAME": "test-function",
			},
			expected: EnvironmentLambda,
		},
		{
			name: "ECS environment",
			envVars: map[string]string{
				"ECS_CONTAINER_METADATA_URI": "http://169.254.170.2/v3",
			},
			expected: EnvironmentECS,
		},
		{
			name: "ECS Fargate environment",
			envVars: map[string]string{
				"ECS_CONTAINER_METADATA_URI_V4": "http://169.254.170.2/v4",
			},
			expected: EnvironmentECS,
		},
		{
			name:     "Local environment",
			envVars:  map[string]string{},
			expected: EnvironmentLocal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()
			
			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			
			// Test detection
			result := DetectEnvironment()
			if result != tt.expected {
				t.Errorf("DetectEnvironment() = %v, want %v", result, tt.expected)
			}
			
			// Cleanup
			for k := range tt.envVars {
				os.Unsetenv(k)
			}
		})
	}
}

func TestGetOptimalWorkerCount(t *testing.T) {
	tests := []struct {
		name     string
		env      RuntimeEnvironment
		memoryMB int
		minExpected int
		maxExpected int
	}{
		{
			name:        "Lambda small memory",
			env:         EnvironmentLambda,
			memoryMB:    256,
			minExpected: 2,
			maxExpected: 2,
		},
		{
			name:        "Lambda medium memory",
			env:         EnvironmentLambda,
			memoryMB:    1024,
			minExpected: 3,
			maxExpected: 4,
		},
		{
			name:        "Lambda large memory",
			env:         EnvironmentLambda,
			memoryMB:    3008,
			minExpected: 6,
			maxExpected: 8,
		},
		{
			name:        "ECS environment",
			env:         EnvironmentECS,
			minExpected: 4,  // At least 1 CPU * 4
			maxExpected: 64, // Up to 16 CPUs * 4
		},
		{
			name:        "Local environment",
			env:         EnvironmentLocal,
			minExpected: 4,
			maxExpected: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env == EnvironmentLambda && tt.memoryMB > 0 {
				os.Setenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE", fmt.Sprintf("%d", tt.memoryMB))
			}
			
			result := GetOptimalWorkerCount(tt.env)
			
			if result < tt.minExpected || result > tt.maxExpected {
				t.Errorf("GetOptimalWorkerCount(%v) = %v, want between %v and %v",
					tt.env, result, tt.minExpected, tt.maxExpected)
			}
			
			os.Unsetenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE")
		})
	}
}

func TestAdaptiveWorkerPool_BasicOperation(t *testing.T) {
	ctx := context.Background()
	config := &PoolConfig{
		Environment: EnvironmentLocal,
		MaxWorkers:  4,
		QueueSize:   10,
	}
	
	pool := NewAdaptiveWorkerPool(ctx, config)
	
	// Start the pool
	err := pool.Start()
	if err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}
	defer pool.Stop()
	
	// Test task execution
	var counter int32
	tasksCount := 10
	done := make(chan bool, tasksCount)
	
	for i := 0; i < tasksCount; i++ {
		task := Task{
			ID: fmt.Sprintf("task_%d", i),
			Execute: func(ctx context.Context) error {
				atomic.AddInt32(&counter, 1)
				time.Sleep(10 * time.Millisecond) // Simulate work
				return nil
			},
			Callback: func(id string, err error) {
				if err != nil {
					t.Errorf("Task %s failed: %v", id, err)
				}
				done <- true
			},
		}
		
		err := pool.Submit(task)
		if err != nil {
			t.Errorf("Failed to submit task: %v", err)
		}
	}
	
	// Wait for all tasks to complete
	for i := 0; i < tasksCount; i++ {
		select {
		case <-done:
			// Task completed
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for tasks to complete")
		}
	}
	
	// Verify all tasks executed
	if atomic.LoadInt32(&counter) != int32(tasksCount) {
		t.Errorf("Expected %d tasks to execute, got %d", tasksCount, counter)
	}
}

func TestAdaptiveWorkerPool_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	config := &PoolConfig{
		Environment: EnvironmentLocal,
		MaxWorkers:  2,
		QueueSize:   5,
	}
	
	pool := NewAdaptiveWorkerPool(ctx, config)
	
	// Start the pool
	err := pool.Start()
	if err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}
	defer pool.Stop()
	
	// Test task with error
	errorReceived := make(chan error, 1)
	
	task := Task{
		ID: "error_task",
		Execute: func(ctx context.Context) error {
			return fmt.Errorf("simulated error")
		},
		Callback: func(id string, err error) {
			errorReceived <- err
		},
	}
	
	err = pool.Submit(task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}
	
	// Wait for error
	select {
	case err := <-errorReceived:
		if err == nil {
			t.Error("Expected error but got nil")
		}
		if err.Error() != "simulated error" {
			t.Errorf("Expected 'simulated error', got '%v'", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for error callback")
	}
}

func TestAdaptiveWorkerPool_LambdaSpecificBehavior(t *testing.T) {
	ctx := context.Background()
	config := &PoolConfig{
		Environment: EnvironmentLambda,
		// Let it auto-configure
	}
	
	pool := NewAdaptiveWorkerPool(ctx, config)
	
	// Verify Lambda-specific configuration
	if pool.config.MaxWorkers > 8 {
		t.Errorf("Lambda pool has too many workers: %d", pool.config.MaxWorkers)
	}
	
	if pool.config.QueueSize > 100 {
		t.Errorf("Lambda pool queue too large: %d", pool.config.QueueSize)
	}
	
	if pool.config.BatchSize > 25 {
		t.Errorf("Lambda batch size too large: %d", pool.config.BatchSize)
	}
}

func TestGetOptimalBatchSize(t *testing.T) {
	tests := []struct {
		env      RuntimeEnvironment
		expected int
	}{
		{EnvironmentLambda, 25},
		{EnvironmentECS, 100},
		{EnvironmentLocal, 50},
	}
	
	for _, tt := range tests {
		t.Run(string(tt.env), func(t *testing.T) {
			result := GetOptimalBatchSize(tt.env)
			if result != tt.expected {
				t.Errorf("GetOptimalBatchSize(%v) = %v, want %v", tt.env, result, tt.expected)
			}
		})
	}
}

func BenchmarkWorkerPool_Throughput(b *testing.B) {
	ctx := context.Background()
	config := &PoolConfig{
		Environment: EnvironmentLocal,
		MaxWorkers:  8,
		QueueSize:   1000,
	}
	
	pool := NewAdaptiveWorkerPool(ctx, config)
	pool.Start()
	defer pool.Stop()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		task := Task{
			ID: fmt.Sprintf("bench_%d", i),
			Execute: func(ctx context.Context) error {
				// Minimal work to measure overhead
				return nil
			},
		}
		
		pool.Submit(task)
	}
	
	// Wait for completion
	pool.Wait()
}