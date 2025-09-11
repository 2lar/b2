package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"backend/application/ports"
)

// InMemoryOperationStore provides an in-memory implementation of OperationStore
type InMemoryOperationStore struct {
	mu         sync.RWMutex
	operations map[string]*ports.OperationResult
	ttl        time.Duration
}

// NewInMemoryOperationStore creates a new in-memory operation store
func NewInMemoryOperationStore(ttl time.Duration) *InMemoryOperationStore {
	store := &InMemoryOperationStore{
		operations: make(map[string]*ports.OperationResult),
		ttl:        ttl,
	}
	
	// Start cleanup goroutine
	go store.cleanupRoutine()
	
	return store
}

// Store saves an operation result
func (s *InMemoryOperationStore) Store(ctx context.Context, result *ports.OperationResult) error {
	if result == nil || result.OperationID == "" {
		return fmt.Errorf("invalid operation result")
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.operations[result.OperationID] = result
	return nil
}

// Get retrieves an operation result by ID
func (s *InMemoryOperationStore) Get(ctx context.Context, operationID string) (*ports.OperationResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result, exists := s.operations[operationID]
	if !exists {
		return nil, fmt.Errorf("operation not found: %s", operationID)
	}
	
	// Check if expired
	if s.isExpired(result) {
		return nil, fmt.Errorf("operation expired: %s", operationID)
	}
	
	return result, nil
}

// Update updates an existing operation result
func (s *InMemoryOperationStore) Update(ctx context.Context, operationID string, result *ports.OperationResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.operations[operationID]; !exists {
		return fmt.Errorf("operation not found: %s", operationID)
	}
	
	s.operations[operationID] = result
	return nil
}

// Delete removes an operation result
func (s *InMemoryOperationStore) Delete(ctx context.Context, operationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.operations, operationID)
	return nil
}

// CleanupExpired removes operations older than the given duration
func (s *InMemoryOperationStore) CleanupExpired(ctx context.Context, olderThan time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	expiredIDs := []string{}
	
	for id, op := range s.operations {
		age := now.Sub(op.StartedAt)
		if age > olderThan {
			expiredIDs = append(expiredIDs, id)
		}
	}
	
	for _, id := range expiredIDs {
		delete(s.operations, id)
	}
	
	return nil
}

// isExpired checks if an operation has expired
func (s *InMemoryOperationStore) isExpired(result *ports.OperationResult) bool {
	age := time.Since(result.StartedAt)
	return age > s.ttl
}

// cleanupRoutine runs periodically to clean up expired operations
func (s *InMemoryOperationStore) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		s.CleanupExpired(context.Background(), s.ttl)
	}
}