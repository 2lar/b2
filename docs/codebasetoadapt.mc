Plan to Make Existing Codebase Compatible with Advanced Repository Patterns

  Current Issues Analysis

  The build is failing because:
  1. Interface Method Conflicts: Advanced interfaces have different method signatures than existing code expects
  2. Missing Methods: New advanced interfaces expect methods that don't exist in current implementations
  3. Type Mismatches: Advanced patterns use domain types (NodeID, UserID) while existing code uses strings
  4. Missing Error Types: Advanced patterns reference error functions that don't exist

  Compatibility Plan

  Phase 1: Create Adapter Layer (Immediate Fix)

  1. Repository Adapter Pattern: Create adapters that bridge old and new interfaces
  2. Method Signature Bridge: Add wrapper methods that convert between string and domain types
  3. Error Type Definitions: Define missing error helper functions
  4. Type Aliases: Create aliases for backward compatibility

  Phase 2: Gradual Interface Migration

  1. Extend Existing Interfaces: Add new methods to existing interfaces without breaking changes
  2. Default Implementations: Provide default implementations for new methods in existing structs
  3. Optional Advanced Features: Make advanced patterns opt-in rather than required

  Phase 3: Implementation Updates

  1. DynamoDB Repository Updates: Update the concrete DynamoDB implementation to support both old and new methods
  2. Mock Repository Updates: Update mocks to support new interface methods
  3. Service Layer Updates: Update services to use adapter pattern initially

  Detailed Implementation Steps

  Step 1: Create Repository Adapter (repository/adapter.go)

  // RepositoryAdapter bridges legacy and modern repository interfaces
  type RepositoryAdapter struct {
      legacy Repository // existing interface
      modern *ModernRepositoryImpl // new advanced patterns
  }

  // Bridge methods that convert between string and domain types
  func (r *RepositoryAdapter) FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) {
      return r.legacy.FindNodeByID(ctx, "", id.String())
  }

  Step 2: Extend Repository Interface Gradually

  type Repository interface {
      // Existing methods (keep all current)
      FindNodeByID(ctx context.Context, userID, nodeID string) (*domain.Node, error)
      DeleteNode(ctx context.Context, userID, nodeID string) error
      // ... all existing methods

      // New optional methods with default implementations
      FindByID(ctx context.Context, id domain.NodeID) (*domain.Node, error) // optional, defaults to FindNodeByID
      Save(ctx context.Context, node *domain.Node) error // optional, defaults to existing save method
  }

  Step 3: Add Missing Error Functions

  // Add to validation.go or create errors.go
  func IsNotFoundError(err error) bool { /* implementation */ }
  func IsConflictError(err error) bool { /* implementation */ }
  func IsTimeoutError(err error) bool { /* implementation */ }
  func IsConnectionError(err error) bool { /* implementation */ }

  Step 4: Fix NodeQuery Compatibility

  // Keep both versions but make them compatible
  type NodeQuery struct {
      // Public fields for backward compatibility
      UserID   string
      Keywords []string
      NodeIDs  []string
      Limit    int
      Offset   int

      // Private advanced fields
      userID domain.UserID
      // ... other advanced fields
  }

  // Add compatibility methods
  func (q *NodeQuery) HasNodeIDs() bool { return len(q.NodeIDs) > 0 }
  func (q *NodeQuery) HasKeywords() bool { return len(q.Keywords) > 0 }
  func (q *NodeQuery) HasPagination() bool { return q.Limit > 0 || q.Offset > 0 }

  Step 5: Pagination Compatibility

  // Create type aliases for backward compatibility
  type NodePage = PaginatedResult[*domain.Node]
  type EdgePage = PaginatedResult[*domain.Edge]

  Implementation Priority

  1. High Priority (Build Blockers):
    - Add missing error functions
    - Fix NodeQuery method compatibility
    - Add pagination type aliases
    - Fix interface method signatures
  2. Medium Priority (Feature Compatibility):
    - Create repository adapter layer
    - Update DynamoDB implementation
    - Fix service layer compatibility
  3. Low Priority (Advanced Features):
    - Enable full advanced pattern usage
    - Performance optimizations
    - Complete migration to domain types

  Migration Strategy

  1. Backward Compatibility First: Ensure all existing code continues to work
  2. Additive Changes: Add new functionality without removing old
  3. Gradual Adoption: Allow teams to migrate to new patterns over time
  4. Documentation: Provide clear migration guides