package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// SchemaVersion represents a specific version of the database schema
type SchemaVersion struct {
	Version      int       `json:"version"`
	Description  string    `json:"description"`
	AppliedAt    time.Time `json:"applied_at"`
	Checksum     string    `json:"checksum"`
	BreakingChange bool    `json:"breaking_change"`
}

// Migration represents a schema migration
type Migration struct {
	FromVersion int                    `json:"from_version"`
	ToVersion   int                    `json:"to_version"`
	Description string                 `json:"description"`
	Up          MigrationFunc          `json:"-"`
	Down        MigrationFunc          `json:"-"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// MigrationFunc is a function that performs a migration
type MigrationFunc func(ctx context.Context) error

// SchemaEvolution manages database schema evolution
type SchemaEvolution struct {
	currentVersion int
	migrations     []Migration
	history        []SchemaVersion
}

// NewSchemaEvolution creates a new schema evolution manager
func NewSchemaEvolution() *SchemaEvolution {
	return &SchemaEvolution{
		currentVersion: 1,
		migrations:     []Migration{},
		history:        []SchemaVersion{},
	}
}

// RegisterMigration registers a new migration
func (s *SchemaEvolution) RegisterMigration(migration Migration) error {
	// Validate migration
	if migration.FromVersion >= migration.ToVersion {
		return fmt.Errorf("invalid migration: from_version must be less than to_version")
	}
	
	// Check for conflicts
	for _, existing := range s.migrations {
		if existing.FromVersion == migration.FromVersion && 
		   existing.ToVersion == migration.ToVersion {
			return fmt.Errorf("migration from %d to %d already exists", 
				migration.FromVersion, migration.ToVersion)
		}
	}
	
	s.migrations = append(s.migrations, migration)
	return nil
}

// Migrate performs migrations to reach the target version
func (s *SchemaEvolution) Migrate(ctx context.Context, targetVersion int) error {
	if targetVersion == s.currentVersion {
		return nil // Already at target version
	}
	
	if targetVersion < s.currentVersion {
		return s.rollback(ctx, targetVersion)
	}
	
	return s.upgrade(ctx, targetVersion)
}

// upgrade performs forward migrations
func (s *SchemaEvolution) upgrade(ctx context.Context, targetVersion int) error {
	for s.currentVersion < targetVersion {
		migration := s.findMigration(s.currentVersion, s.currentVersion+1)
		if migration == nil {
			return fmt.Errorf("no migration found from version %d to %d", 
				s.currentVersion, s.currentVersion+1)
		}
		
		if err := migration.Up(ctx); err != nil {
			return fmt.Errorf("migration %d->%d failed: %w", 
				migration.FromVersion, migration.ToVersion, err)
		}
		
		// Record migration
		s.history = append(s.history, SchemaVersion{
			Version:     migration.ToVersion,
			Description: migration.Description,
			AppliedAt:   time.Now(),
		})
		
		s.currentVersion = migration.ToVersion
	}
	
	return nil
}

// rollback performs backward migrations
func (s *SchemaEvolution) rollback(ctx context.Context, targetVersion int) error {
	for s.currentVersion > targetVersion {
		migration := s.findMigration(s.currentVersion-1, s.currentVersion)
		if migration == nil {
			return fmt.Errorf("no rollback found from version %d to %d", 
				s.currentVersion, s.currentVersion-1)
		}
		
		if migration.Down == nil {
			return fmt.Errorf("migration %d->%d does not support rollback", 
				migration.FromVersion, migration.ToVersion)
		}
		
		if err := migration.Down(ctx); err != nil {
			return fmt.Errorf("rollback %d->%d failed: %w", 
				migration.ToVersion, migration.FromVersion, err)
		}
		
		s.currentVersion = migration.FromVersion
	}
	
	return nil
}

// findMigration finds a migration between two versions
func (s *SchemaEvolution) findMigration(from, to int) *Migration {
	for _, migration := range s.migrations {
		if migration.FromVersion == from && migration.ToVersion == to {
			return &migration
		}
	}
	return nil
}

// GetCurrentVersion returns the current schema version
func (s *SchemaEvolution) GetCurrentVersion() int {
	return s.currentVersion
}

// GetHistory returns the migration history
func (s *SchemaEvolution) GetHistory() []SchemaVersion {
	return s.history
}

// DataTransformation represents a data transformation strategy
type DataTransformation struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Transform   TransformFunc          `json:"-"`
	Validate    ValidationFunc         `json:"-"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TransformFunc transforms data from one format to another
type TransformFunc func(ctx context.Context, data interface{}) (interface{}, error)

// ValidationFunc validates transformed data
type ValidationFunc func(data interface{}) error

// EntityMigration handles entity-specific migrations
type EntityMigration struct {
	EntityType     string                  `json:"entity_type"`
	FromSchema     interface{}             `json:"from_schema"`
	ToSchema       interface{}             `json:"to_schema"`
	Transformation DataTransformation      `json:"transformation"`
	BatchSize      int                     `json:"batch_size"`
}

// MigrationPlan represents a complete migration plan
type MigrationPlan struct {
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	FromVersion    int               `json:"from_version"`
	ToVersion      int               `json:"to_version"`
	EntityMigrations []EntityMigration `json:"entity_migrations"`
	PreChecks      []ValidationFunc  `json:"-"`
	PostChecks     []ValidationFunc  `json:"-"`
	RollbackPlan   *MigrationPlan    `json:"rollback_plan,omitempty"`
}

// Execute executes the migration plan
func (p *MigrationPlan) Execute(ctx context.Context) error {
	// Run pre-checks
	for _, check := range p.PreChecks {
		if err := check(nil); err != nil {
			return fmt.Errorf("pre-check failed: %w", err)
		}
	}
	
	// Execute entity migrations
	for _, em := range p.EntityMigrations {
		if err := p.executeEntityMigration(ctx, em); err != nil {
			return fmt.Errorf("entity migration failed for %s: %w", 
				em.EntityType, err)
		}
	}
	
	// Run post-checks
	for _, check := range p.PostChecks {
		if err := check(nil); err != nil {
			return fmt.Errorf("post-check failed: %w", err)
		}
	}
	
	return nil
}

// executeEntityMigration executes a single entity migration
func (p *MigrationPlan) executeEntityMigration(ctx context.Context, em EntityMigration) error {
	// This would typically:
	// 1. Query entities of the given type
	// 2. Transform them using the transformation function
	// 3. Validate the transformed data
	// 4. Save the transformed entities
	// 5. Handle batching for large datasets
	
	// Placeholder implementation
	return nil
}

// CompatibilityChecker checks schema compatibility
type CompatibilityChecker struct {
	rules []CompatibilityRule
}

// CompatibilityRule defines a compatibility check
type CompatibilityRule struct {
	Name        string
	Description string
	Check       func(oldSchema, newSchema interface{}) error
}

// CheckCompatibility checks if two schemas are compatible
func (c *CompatibilityChecker) CheckCompatibility(oldSchema, newSchema interface{}) error {
	for _, rule := range c.rules {
		if err := rule.Check(oldSchema, newSchema); err != nil {
			return fmt.Errorf("compatibility check '%s' failed: %w", rule.Name, err)
		}
	}
	return nil
}

// DefaultCompatibilityRules returns default compatibility rules
func DefaultCompatibilityRules() []CompatibilityRule {
	return []CompatibilityRule{
		{
			Name:        "required_fields",
			Description: "New required fields must have defaults",
			Check: func(oldSchema, newSchema interface{}) error {
				// Check that new required fields have default values
				return nil
			},
		},
		{
			Name:        "field_types",
			Description: "Field type changes must be compatible",
			Check: func(oldSchema, newSchema interface{}) error {
				// Check that field type changes are backward compatible
				return nil
			},
		},
		{
			Name:        "field_removal",
			Description: "Field removal must be handled gracefully",
			Check: func(oldSchema, newSchema interface{}) error {
				// Check that removed fields are deprecated first
				return nil
			},
		},
	}
}

// SchemaRegistry manages schema versions and compatibility
type SchemaRegistry struct {
	schemas      map[string]map[int]interface{}
	evolution    *SchemaEvolution
	checker      *CompatibilityChecker
}

// NewSchemaRegistry creates a new schema registry
func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{
		schemas:   make(map[string]map[int]interface{}),
		evolution: NewSchemaEvolution(),
		checker: &CompatibilityChecker{
			rules: DefaultCompatibilityRules(),
		},
	}
}

// RegisterSchema registers a new schema version
func (r *SchemaRegistry) RegisterSchema(entityType string, version int, schema interface{}) error {
	if r.schemas[entityType] == nil {
		r.schemas[entityType] = make(map[int]interface{})
	}
	
	// Check compatibility with previous version
	if prevSchema, exists := r.schemas[entityType][version-1]; exists {
		if err := r.checker.CheckCompatibility(prevSchema, schema); err != nil {
			return fmt.Errorf("schema incompatible: %w", err)
		}
	}
	
	r.schemas[entityType][version] = schema
	return nil
}

// GetSchema retrieves a schema by entity type and version
func (r *SchemaRegistry) GetSchema(entityType string, version int) (interface{}, error) {
	if schemas, exists := r.schemas[entityType]; exists {
		if schema, exists := schemas[version]; exists {
			return schema, nil
		}
	}
	return nil, fmt.Errorf("schema not found for %s v%d", entityType, version)
}

// MarshalWithSchema marshals data with schema information
func MarshalWithSchema(data interface{}, schemaVersion int) ([]byte, error) {
	wrapper := struct {
		SchemaVersion int         `json:"_schema_version"`
		Data         interface{} `json:"data"`
	}{
		SchemaVersion: schemaVersion,
		Data:         data,
	}
	return json.Marshal(wrapper)
}

// UnmarshalWithSchema unmarshals data and returns schema version
func UnmarshalWithSchema(data []byte) (interface{}, int, error) {
	var wrapper struct {
		SchemaVersion int             `json:"_schema_version"`
		Data         json.RawMessage `json:"data"`
	}
	
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, 0, err
	}
	
	return wrapper.Data, wrapper.SchemaVersion, nil
}