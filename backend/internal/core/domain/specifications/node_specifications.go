// Package specifications provides concrete specifications for the node domain.
package specifications

import (
	"context"
	"strings"
	"time"
	
	"brain2-backend/internal/core/domain/aggregates/node"
)

// ActiveNodeSpecification checks if a node is active (not archived)
type ActiveNodeSpecification struct {
	BaseSpecification[*node.Aggregate]
}

// NewActiveNodeSpecification creates a specification for active nodes
func NewActiveNodeSpecification() *ActiveNodeSpecification {
	spec := &ActiveNodeSpecification{}
	spec.BaseSpecification = BaseSpecification[*node.Aggregate]{
		IsSatisfiedByFunc: func(ctx context.Context, n *node.Aggregate) (bool, error) {
			return !n.IsArchived(), nil
		},
		GetSQLFunc: func() (string, []interface{}) {
			return "archived = ?", []interface{}{false}
		},
		Description: "Node is active",
	}
	return spec
}

// UserOwnedNodeSpecification checks if a node belongs to a specific user
type UserOwnedNodeSpecification struct {
	BaseSpecification[*node.Aggregate]
	userID string
}

// NewUserOwnedNodeSpecification creates a specification for user-owned nodes
func NewUserOwnedNodeSpecification(userID string) *UserOwnedNodeSpecification {
	spec := &UserOwnedNodeSpecification{userID: userID}
	spec.BaseSpecification = BaseSpecification[*node.Aggregate]{
		IsSatisfiedByFunc: func(ctx context.Context, n *node.Aggregate) (bool, error) {
			return n.GetUserID() == userID, nil
		},
		GetSQLFunc: func() (string, []interface{}) {
			return "user_id = ?", []interface{}{userID}
		},
		Description: "Node belongs to user " + userID,
	}
	return spec
}

// ContentContainsSpecification checks if node content contains specific text
type ContentContainsSpecification struct {
	BaseSpecification[*node.Aggregate]
	searchText string
}

// NewContentContainsSpecification creates a specification for content search
func NewContentContainsSpecification(searchText string) *ContentContainsSpecification {
	spec := &ContentContainsSpecification{searchText: searchText}
	spec.BaseSpecification = BaseSpecification[*node.Aggregate]{
		IsSatisfiedByFunc: func(ctx context.Context, n *node.Aggregate) (bool, error) {
			content := n.GetContent()
			return strings.Contains(strings.ToLower(content), strings.ToLower(searchText)), nil
		},
		GetSQLFunc: func() (string, []interface{}) {
			return "LOWER(content) LIKE ?", []interface{}{"%" + strings.ToLower(searchText) + "%"}
		},
		Description: "Content contains '" + searchText + "'",
	}
	return spec
}

// CreatedAfterSpecification checks if node was created after a specific date
type CreatedAfterSpecification struct {
	BaseSpecification[*node.Aggregate]
	date time.Time
}

// NewCreatedAfterSpecification creates a specification for creation date filtering
func NewCreatedAfterSpecification(date time.Time) *CreatedAfterSpecification {
	spec := &CreatedAfterSpecification{date: date}
	spec.BaseSpecification = BaseSpecification[*node.Aggregate]{
		IsSatisfiedByFunc: func(ctx context.Context, n *node.Aggregate) (bool, error) {
			return n.GetCreatedAt().After(date), nil
		},
		GetSQLFunc: func() (string, []interface{}) {
			return "created_at > ?", []interface{}{date}
		},
		Description: "Created after " + date.Format("2006-01-02"),
	}
	return spec
}

// HasTagSpecification checks if node has a specific tag
type HasTagSpecification struct {
	BaseSpecification[*node.Aggregate]
	tag string
}

// NewHasTagSpecification creates a specification for tag filtering
func NewHasTagSpecification(tag string) *HasTagSpecification {
	spec := &HasTagSpecification{tag: tag}
	spec.BaseSpecification = BaseSpecification[*node.Aggregate]{
		IsSatisfiedByFunc: func(ctx context.Context, n *node.Aggregate) (bool, error) {
			tags := n.GetTags()
			for _, t := range tags {
				if strings.EqualFold(t, tag) {
					return true, nil
				}
			}
			return false, nil
		},
		GetSQLFunc: func() (string, []interface{}) {
			// This assumes a JSON array column for tags
			return "tags @> ?", []interface{}{`["` + tag + `"]`}
		},
		Description: "Has tag '" + tag + "'",
	}
	return spec
}

// HasMinimumConnectionsSpecification checks if node has minimum number of connections
type HasMinimumConnectionsSpecification struct {
	BaseSpecification[*node.Aggregate]
	minConnections int
}

// NewHasMinimumConnectionsSpecification creates a specification for connection count
func NewHasMinimumConnectionsSpecification(minConnections int) *HasMinimumConnectionsSpecification {
	spec := &HasMinimumConnectionsSpecification{minConnections: minConnections}
	spec.BaseSpecification = BaseSpecification[*node.Aggregate]{
		IsSatisfiedByFunc: func(ctx context.Context, n *node.Aggregate) (bool, error) {
			// This would need to be injected or passed through context
			// For now, we'll assume the aggregate tracks this
			return n.GetConnectionCount() >= minConnections, nil
		},
		GetSQLFunc: func() (string, []interface{}) {
			// This would require a subquery or join
			return "(SELECT COUNT(*) FROM edges WHERE source_node_id = nodes.id OR target_node_id = nodes.id) >= ?", 
				[]interface{}{minConnections}
		},
		Description: "Has at least " + string(rune(minConnections)) + " connections",
	}
	return spec
}

// ModifiedInLastDaysSpecification checks if node was modified in the last N days
type ModifiedInLastDaysSpecification struct {
	BaseSpecification[*node.Aggregate]
	days int
}

// NewModifiedInLastDaysSpecification creates a specification for recent modifications
func NewModifiedInLastDaysSpecification(days int) *ModifiedInLastDaysSpecification {
	spec := &ModifiedInLastDaysSpecification{days: days}
	cutoffDate := time.Now().AddDate(0, 0, -days)
	
	spec.BaseSpecification = BaseSpecification[*node.Aggregate]{
		IsSatisfiedByFunc: func(ctx context.Context, n *node.Aggregate) (bool, error) {
			return n.GetUpdatedAt().After(cutoffDate), nil
		},
		GetSQLFunc: func() (string, []interface{}) {
			return "updated_at > ?", []interface{}{cutoffDate}
		},
		Description: "Modified in last " + string(rune(days)) + " days",
	}
	return spec
}

// CategorySpecification checks if node belongs to a specific category
type CategorySpecification struct {
	BaseSpecification[*node.Aggregate]
	categoryID string
}

// NewCategorySpecification creates a specification for category filtering
func NewCategorySpecification(categoryID string) *CategorySpecification {
	spec := &CategorySpecification{categoryID: categoryID}
	spec.BaseSpecification = BaseSpecification[*node.Aggregate]{
		IsSatisfiedByFunc: func(ctx context.Context, n *node.Aggregate) (bool, error) {
			categories := n.GetCategories()
			for _, c := range categories {
				if c == categoryID {
					return true, nil
				}
			}
			return false, nil
		},
		GetSQLFunc: func() (string, []interface{}) {
			// Assuming a junction table for node-category relationships
			return "EXISTS (SELECT 1 FROM node_categories WHERE node_id = nodes.id AND category_id = ?)", 
				[]interface{}{categoryID}
		},
		Description: "In category " + categoryID,
	}
	return spec
}

// ComplexNodeSpecification demonstrates combining multiple specifications
type ComplexNodeSpecification struct {
	Specification[*node.Aggregate]
}

// NewRecentActiveUserNodesSpecification creates a complex specification
// for active nodes from a specific user created in the last 30 days
func NewRecentActiveUserNodesSpecification(userID string) *ComplexNodeSpecification {
	activeSpec := NewActiveNodeSpecification()
	userSpec := NewUserOwnedNodeSpecification(userID)
	recentSpec := NewCreatedAfterSpecification(time.Now().AddDate(0, 0, -30))
	
	// Combine specifications: active AND user-owned AND recent
	combined := activeSpec.And(userSpec).And(recentSpec)
	
	return &ComplexNodeSpecification{
		Specification: combined,
	}
}

// NewTaggedContentSpecification creates a specification for nodes with specific tags and content
func NewTaggedContentSpecification(tags []string, searchText string) Specification[*node.Aggregate] {
	var spec Specification[*node.Aggregate]
	
	// Start with content specification
	if searchText != "" {
		spec = NewContentContainsSpecification(searchText)
	}
	
	// Add tag specifications
	for _, tag := range tags {
		tagSpec := NewHasTagSpecification(tag)
		if spec == nil {
			spec = tagSpec
		} else {
			spec = spec.And(tagSpec)
		}
	}
	
	if spec == nil {
		// Return always true if no criteria
		return &AlwaysTrue[*node.Aggregate]{}
	}
	
	return spec
}