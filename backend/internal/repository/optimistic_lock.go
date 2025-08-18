package repository

import (
	"context"
	"fmt"

	"brain2-backend/internal/domain/node"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/pkg/errors"
)

// AggregateVersionError represents a version conflict during aggregate optimistic locking
type AggregateVersionError struct {
	AggregateID      string
	ExpectedVersion  int
	ActualVersion    int
}

func (e *AggregateVersionError) Error() string {
	return fmt.Sprintf("optimistic lock violation for aggregate %s: expected version %d, actual version %d",
		e.AggregateID, e.ExpectedVersion, e.ActualVersion)
}

// IsAggregateVersionError checks if an error is an aggregate version error
func IsAggregateVersionError(err error) bool {
	_, ok := err.(*AggregateVersionError)
	return ok
}

// OptimisticLockingRepository wraps a repository with optimistic locking capabilities
type OptimisticLockingRepository struct {
	inner           interface{} // The underlying repository
	versionStore    VersionStore
	aggregateType   string
}

// VersionStore manages version information for aggregates
type VersionStore interface {
	// GetVersion retrieves the current version of an aggregate
	GetVersion(ctx context.Context, aggregateID string) (int, error)
	
	// SetVersion updates the version of an aggregate
	SetVersion(ctx context.Context, aggregateID string, version int) error
	
	// CompareAndSwap atomically updates version if current version matches expected
	CompareAndSwap(ctx context.Context, aggregateID string, expectedVersion, newVersion int) error
}

// NewOptimisticLockingRepository creates a new repository with optimistic locking
func NewOptimisticLockingRepository(inner interface{}, versionStore VersionStore, aggregateType string) *OptimisticLockingRepository {
	return &OptimisticLockingRepository{
		inner:         inner,
		versionStore:  versionStore,
		aggregateType: aggregateType,
	}
}

// SaveWithOptimisticLock saves an aggregate with optimistic locking
func (r *OptimisticLockingRepository) SaveWithOptimisticLock(ctx context.Context, aggregate shared.AggregateRoot, saveFunc func(context.Context) error) error {
	// Get the expected version from the aggregate
	expectedVersion := aggregate.GetVersion()
	aggregateID := aggregate.GetID()
	
	// Check current version in the store
	currentVersion, err := r.versionStore.GetVersion(ctx, aggregateID)
	if err != nil && !errors.IsNotFound(err) {
		return errors.Wrap(err, "failed to get current version")
	}
	
	// For new aggregates, current version should be 0
	if errors.IsNotFound(err) {
		currentVersion = 0
	}
	
	// Verify version match (for updates, not new creates)
	if expectedVersion > 0 && currentVersion != expectedVersion {
		return &AggregateVersionError{
			AggregateID:     aggregateID,
			ExpectedVersion: expectedVersion,
			ActualVersion:   currentVersion,
		}
	}
	
	// Execute the save operation
	if err := saveFunc(ctx); err != nil {
		return err
	}
	
	// Update version after successful save
	newVersion := expectedVersion + 1
	if err := r.versionStore.CompareAndSwap(ctx, aggregateID, expectedVersion, newVersion); err != nil {
		// If CAS fails, another process updated the aggregate concurrently
		return &AggregateVersionError{
			AggregateID:     aggregateID,
			ExpectedVersion: expectedVersion,
			ActualVersion:   -1, // Unknown actual version
		}
	}
	
	// Update the aggregate's version
	aggregate.IncrementVersion()
	
	return nil
}

// OptimisticLockingNodeRepository wraps NodeRepository with optimistic locking
type OptimisticLockingNodeRepository struct {
	*OptimisticLockingRepository
	inner NodeRepository
}

// NewOptimisticLockingNodeRepository creates a new node repository with optimistic locking
func NewOptimisticLockingNodeRepository(inner NodeRepository, versionStore VersionStore) *OptimisticLockingNodeRepository {
	return &OptimisticLockingNodeRepository{
		OptimisticLockingRepository: NewOptimisticLockingRepository(inner, versionStore, "Node"),
		inner:                        inner,
	}
}

// CreateNodeAndKeywords creates a node with optimistic locking
func (r *OptimisticLockingNodeRepository) CreateNodeAndKeywords(ctx context.Context, n interface{}) error {
	// Try to cast to node type
	nodeEntity, ok := n.(*node.Node)
	if !ok {
		return fmt.Errorf("expected *node.Node, got %T", n)
	}
	
	// Check if it's also an aggregate root
	aggregate, ok := n.(shared.AggregateRoot)
	if !ok {
		// Fall back to regular create if not an aggregate root
		return r.inner.CreateNodeAndKeywords(ctx, nodeEntity)
	}
	
	return r.SaveWithOptimisticLock(ctx, aggregate, func(ctx context.Context) error {
		return r.inner.CreateNodeAndKeywords(ctx, nodeEntity)
	})
}

// Delegate other methods to inner repository
func (r *OptimisticLockingNodeRepository) FindNodeByID(ctx context.Context, userID, nodeID string) (*node.Node, error) {
	return r.inner.FindNodeByID(ctx, userID, nodeID)
}

func (r *OptimisticLockingNodeRepository) FindNodes(ctx context.Context, query NodeQuery) ([]*node.Node, error) {
	return r.inner.FindNodes(ctx, query)
}

func (r *OptimisticLockingNodeRepository) DeleteNode(ctx context.Context, userID, nodeID string) error {
	return r.inner.DeleteNode(ctx, userID, nodeID)
}

func (r *OptimisticLockingNodeRepository) BatchDeleteNodes(ctx context.Context, userID string, nodeIDs []string) (deleted []string, failed []string, err error) {
	return r.inner.BatchDeleteNodes(ctx, userID, nodeIDs)
}

// InMemoryVersionStore provides a simple in-memory version store for testing
type InMemoryVersionStore struct {
	versions map[string]int
}

// NewInMemoryVersionStore creates a new in-memory version store
func NewInMemoryVersionStore() *InMemoryVersionStore {
	return &InMemoryVersionStore{
		versions: make(map[string]int),
	}
}

func (s *InMemoryVersionStore) GetVersion(ctx context.Context, aggregateID string) (int, error) {
	version, exists := s.versions[aggregateID]
	if !exists {
		return 0, errors.NewNotFound(fmt.Sprintf("aggregate %s not found", aggregateID))
	}
	return version, nil
}

func (s *InMemoryVersionStore) SetVersion(ctx context.Context, aggregateID string, version int) error {
	s.versions[aggregateID] = version
	return nil
}

func (s *InMemoryVersionStore) CompareAndSwap(ctx context.Context, aggregateID string, expectedVersion, newVersion int) error {
	currentVersion, exists := s.versions[aggregateID]
	
	// Handle new aggregates
	if !exists && expectedVersion == 0 {
		s.versions[aggregateID] = newVersion
		return nil
	}
	
	// Check version match
	if currentVersion != expectedVersion {
		return &AggregateVersionError{
			AggregateID:     aggregateID,
			ExpectedVersion: expectedVersion,
			ActualVersion:   currentVersion,
		}
	}
	
	// Update version
	s.versions[aggregateID] = newVersion
	return nil
}