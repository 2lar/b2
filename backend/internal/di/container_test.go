// Package di provides dependency injection container tests.
package di

import (
	"context"
	"testing"
	"time"
)

// TestNewContainer tests the creation of a new container.
func TestNewContainer(t *testing.T) {
	container, err := NewContainer()
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	
	if container == nil {
		t.Fatal("Container should not be nil")
	}
	
	// Test validation
	if err := container.Validate(); err != nil {
		t.Errorf("Container validation failed: %v", err)
	}
}

// TestContainerValidation tests container validation logic.
func TestContainerValidation(t *testing.T) {
	// Test empty container validation
	emptyContainer := &Container{}
	err := emptyContainer.Validate()
	if err == nil {
		t.Error("Empty container should fail validation")
	}
	
	// Test properly initialized container
	container, err := NewContainer()
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	
	err = container.Validate()
	if err != nil {
		t.Errorf("Valid container should pass validation: %v", err)
	}
}

// TestContainerComponents tests that all components are properly initialized.
func TestContainerComponents(t *testing.T) {
	container, err := NewContainer()
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	
	// Test config
	if container.Config == nil {
		t.Error("Config should be initialized")
	}
	
	// Test AWS clients
	if container.DynamoDBClient == nil {
		t.Error("DynamoDBClient should be initialized")
	}
	
	if container.EventBridgeClient == nil {
		t.Error("EventBridgeClient should be initialized")
	}
	
	// Test repositories
	if container.NodeRepository == nil {
		t.Error("NodeRepository should be initialized")
	}
	if container.EdgeRepository == nil {
		t.Error("EdgeRepository should be initialized")
	}
	if container.CategoryRepository == nil {
		t.Error("CategoryRepository should be initialized")
	}
	
	// Test CQRS services
	if container.NodeAppService == nil {
		t.Error("NodeAppService should be initialized")
	}
	
	if container.CategoryAppService == nil {
		t.Error("CategoryAppService should be initialized")
	}
	
	// Test handlers
	if container.MemoryHandler == nil {
		t.Error("MemoryHandler should be initialized")
	}
	
	if container.CategoryHandler == nil {
		t.Error("CategoryHandler should be initialized")
	}
	
	// Test router
	if container.Router == nil {
		t.Error("Router should be initialized")
	}
}

// TestContainerRouter tests that the router is properly configured.
func TestContainerRouter(t *testing.T) {
	container, err := NewContainer()
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	
	router := container.GetRouter()
	if router == nil {
		t.Fatal("Router should not be nil")
	}
	
	// Test that router is the same as internal router
	if router != container.Router {
		t.Error("GetRouter should return the internal router")
	}
}

// TestContainerHealth tests the health check functionality.
func TestContainerHealth(t *testing.T) {
	container, err := NewContainer()
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	
	ctx := context.Background()
	health := container.Health(ctx)
	
	if health == nil {
		t.Fatal("Health check should return a map")
	}
	
	// Check expected health keys
	expectedKeys := []string{"container", "config", "dynamodb", "eventbridge"}
	for _, key := range expectedKeys {
		if _, exists := health[key]; !exists {
			t.Errorf("Health check should contain key: %s", key)
		}
	}
	
	// Container should be healthy
	if health["container"] != "healthy" {
		t.Errorf("Container should be healthy, got: %s", health["container"])
	}
}

// TestContainerShutdown tests graceful shutdown functionality.
func TestContainerShutdown(t *testing.T) {
	container, err := NewContainer()
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	
	// Add a test shutdown function
	shutdownCalled := false
	container.AddShutdownFunction(func() error {
		shutdownCalled = true
		return nil
	})
	
	// Test shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = container.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown should not fail: %v", err)
	}
	
	// Verify shutdown function was called
	if !shutdownCalled {
		t.Error("Shutdown function should have been called")
	}
	
	// Test multiple shutdown calls (should be safe)
	err = container.Shutdown(ctx)
	if err != nil {
		t.Errorf("Multiple shutdowns should be safe: %v", err)
	}
}

// TestContainerShutdownTimeout tests shutdown with timeout.
func TestContainerShutdownTimeout(t *testing.T) {
	container, err := NewContainer()
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	
	// Add a shutdown function that takes too long
	container.AddShutdownFunction(func() error {
		time.Sleep(100 * time.Millisecond) // Longer than timeout
		return nil
	})
	
	// Test shutdown with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	
	err = container.Shutdown(ctx)
	// Shutdown should still complete, but may log timeout
	if err != nil {
		t.Logf("Shutdown completed with potential timeout: %v", err)
	}
}

// TestAddShutdownFunction tests adding shutdown functions.
func TestAddShutdownFunction(t *testing.T) {
	container, err := NewContainer()
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}
	
	callCount := 0
	
	// Add multiple shutdown functions
	container.AddShutdownFunction(func() error {
		callCount++
		return nil
	})
	
	container.AddShutdownFunction(func() error {
		callCount++
		return nil
	})
	
	// Shutdown and verify both functions are called
	ctx := context.Background()
	err = container.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown should not fail: %v", err)
	}
	
	if callCount != 2 {
		t.Errorf("Expected 2 shutdown functions to be called, got %d", callCount)
	}
}

// TestInitializeContainer tests the package-level initialization function.
func TestInitializeContainer(t *testing.T) {
	container, err := InitializeContainer()
	if err != nil {
		t.Fatalf("InitializeContainer should not fail: %v", err)
	}
	
	if container == nil {
		t.Fatal("InitializeContainer should return a container")
	}
	
	// Test that it's properly validated
	if err := container.Validate(); err != nil {
		t.Errorf("Initialized container should be valid: %v", err)
	}
}