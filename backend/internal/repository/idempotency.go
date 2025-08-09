package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"brain2-backend/internal/domain"
)

// IdempotencyKey represents a unique key for idempotent operations
type IdempotencyKey struct {
	UserID    string
	Operation string
	Hash      string
	CreatedAt time.Time
}

// IdempotencyStore interface for managing idempotency keys
type IdempotencyStore interface {
	// Store stores an idempotency key with its result
	Store(ctx context.Context, key IdempotencyKey, result interface{}) error

	// Get retrieves a stored result for an idempotency key
	Get(ctx context.Context, key IdempotencyKey) (interface{}, bool, error)

	// Delete removes an idempotency key (for cleanup)
	Delete(ctx context.Context, key IdempotencyKey) error

	// Cleanup removes expired idempotency keys
	Cleanup(ctx context.Context, expiration time.Duration) error
}

// InMemoryIdempotencyStore is a simple in-memory implementation
type InMemoryIdempotencyStore struct {
	store map[string]idempotencyEntry
}

type idempotencyEntry struct {
	result    interface{}
	createdAt time.Time
}

// NewInMemoryIdempotencyStore creates a new in-memory idempotency store
func NewInMemoryIdempotencyStore() *InMemoryIdempotencyStore {
	return &InMemoryIdempotencyStore{
		store: make(map[string]idempotencyEntry),
	}
}

// Store implements IdempotencyStore
func (s *InMemoryIdempotencyStore) Store(ctx context.Context, key IdempotencyKey, result interface{}) error {
	keyStr := s.keyToString(key)
	s.store[keyStr] = idempotencyEntry{
		result:    result,
		createdAt: time.Now(),
	}
	return nil
}

// Get implements IdempotencyStore
func (s *InMemoryIdempotencyStore) Get(ctx context.Context, key IdempotencyKey) (interface{}, bool, error) {
	keyStr := s.keyToString(key)
	entry, exists := s.store[keyStr]
	if !exists {
		return nil, false, nil
	}
	return entry.result, true, nil
}

// Delete implements IdempotencyStore
func (s *InMemoryIdempotencyStore) Delete(ctx context.Context, key IdempotencyKey) error {
	keyStr := s.keyToString(key)
	delete(s.store, keyStr)
	return nil
}

// Cleanup implements IdempotencyStore
func (s *InMemoryIdempotencyStore) Cleanup(ctx context.Context, expiration time.Duration) error {
	cutoff := time.Now().Add(-expiration)
	for key, entry := range s.store {
		if entry.createdAt.Before(cutoff) {
			delete(s.store, key)
		}
	}
	return nil
}

func (s *InMemoryIdempotencyStore) keyToString(key IdempotencyKey) string {
	return fmt.Sprintf("%s:%s:%s", key.UserID, key.Operation, key.Hash)
}

// GenerateIdempotencyKey generates an idempotency key for a node operation
func GenerateIdempotencyKey(userID, operation string, node domain.Node) IdempotencyKey {
	// Create a hash of the node data to ensure uniqueness
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s:%s:%s:%v:%d",
		node.ID, node.UserID, node.Content, node.Keywords, node.Version)))
	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	return IdempotencyKey{
		UserID:    userID,
		Operation: operation,
		Hash:      hash,
		CreatedAt: time.Now(),
	}
}

// GenerateIdempotencyKeyForEdges generates an idempotency key for edge operations
func GenerateIdempotencyKeyForEdges(userID, operation, sourceNodeID string, relatedNodeIDs []string) IdempotencyKey {
	// Create a hash of the edge data
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s:%s:%v", sourceNodeID, operation, relatedNodeIDs)))
	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	return IdempotencyKey{
		UserID:    userID,
		Operation: operation,
		Hash:      hash,
		CreatedAt: time.Now(),
	}
}

// OptimisticLockError represents an optimistic locking conflict
type OptimisticLockError struct {
	ResourceID      string
	ExpectedVersion int
	ActualVersion   int
}

func (e OptimisticLockError) Error() string {
	return fmt.Sprintf("optimistic lock conflict for resource %s: expected version %d, actual version %d",
		e.ResourceID, e.ExpectedVersion, e.ActualVersion)
}

// IsOptimisticLockError checks if an error is an optimistic lock error
func IsOptimisticLockError(err error) bool {
	_, ok := err.(OptimisticLockError)
	return ok
}

// VersionedNode represents a node with version control
type VersionedNode struct {
	domain.Node
	ETag string // Entity tag for optimistic locking
}

// ConflictResolutionStrategy defines how to resolve conflicts
type ConflictResolutionStrategy int

const (
	// ConflictReject rejects the operation when a conflict is detected
	ConflictReject ConflictResolutionStrategy = iota

	// ConflictRetry retries the operation with the latest version
	ConflictRetry

	// ConflictMerge attempts to merge the changes
	ConflictMerge
)

// ConflictResolver defines how to resolve conflicts
type ConflictResolver interface {
	// ResolveConflict resolves a conflict between two versions of a node
	ResolveConflict(ctx context.Context, current, incoming domain.Node) (domain.Node, error)
}

// LastWriteWinsResolver implements a simple last-write-wins strategy
type LastWriteWinsResolver struct{}

// ResolveConflict implements ConflictResolver
func (r *LastWriteWinsResolver) ResolveConflict(ctx context.Context, current, incoming domain.Node) (domain.Node, error) {
	// Simple last-write-wins: return the incoming node with incremented version
	incoming.Version = current.Version + 1
	return incoming, nil
}

// MergeResolver implements a merge-based conflict resolution
type MergeResolver struct{}

// ResolveConflict implements ConflictResolver
func (r *MergeResolver) ResolveConflict(ctx context.Context, current, incoming domain.Node) (domain.Node, error) {
	// Merge strategy: combine keywords and use incoming content
	merged := incoming
	merged.Version = current.Version + 1

	// Merge keywords
	keywordSet := make(map[string]bool)
	for _, keyword := range current.Keywords {
		keywordSet[keyword] = true
	}
	for _, keyword := range incoming.Keywords {
		keywordSet[keyword] = true
	}

	var mergedKeywords []string
	for keyword := range keywordSet {
		mergedKeywords = append(mergedKeywords, keyword)
	}
	merged.Keywords = mergedKeywords

	return merged, nil
}

// IdempotentOperation represents an operation that can be made idempotent
type IdempotentOperation[T any] struct {
	store     IdempotencyStore
	key       IdempotencyKey
	operation func() (T, error)
}

// NewIdempotentOperation creates a new idempotent operation
func NewIdempotentOperation[T any](store IdempotencyStore, key IdempotencyKey, operation func() (T, error)) *IdempotentOperation[T] {
	return &IdempotentOperation[T]{
		store:     store,
		key:       key,
		operation: operation,
	}
}

// Execute executes the operation idempotently
func (op *IdempotentOperation[T]) Execute(ctx context.Context) (T, error) {
	var zero T

	// Check if we already have a result for this key
	result, exists, err := op.store.Get(ctx, op.key)
	if err != nil {
		return zero, fmt.Errorf("failed to check idempotency store: %w", err)
	}

	if exists {
		// Return the cached result
		if typedResult, ok := result.(T); ok {
			return typedResult, nil
		}
		return zero, fmt.Errorf("idempotency store returned unexpected type")
	}

	// Execute the operation
	result, err = op.operation()
	if err != nil {
		return zero, err
	}

	// Store the result for future idempotency checks
	if storeErr := op.store.Store(ctx, op.key, result); storeErr != nil {
		// Log the error but don't fail the operation
		// The operation succeeded, we just couldn't store the idempotency key
		fmt.Printf("Warning: failed to store idempotency key: %v\n", storeErr)
	}

	return result.(T), nil
}
