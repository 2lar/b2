package category

import "time"

// CategoryHierarchy represents the parent-child relationship between categories
type CategoryHierarchy struct {
	ParentID  string    `json:"parent_id"`
	ChildID   string    `json:"child_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	Level     int       `json:"level"` // Depth level in hierarchy
}

// NewCategoryHierarchy creates a new category hierarchy relationship
func NewCategoryHierarchy(userID, parentID, childID string, level int) *CategoryHierarchy {
	return &CategoryHierarchy{
		ParentID:  parentID,
		ChildID:   childID,
		UserID:    userID,
		CreatedAt: time.Now(),
		Level:     level,
	}
}

// IsRootLevel returns true if this is a root-level hierarchy (level 0)
func (ch *CategoryHierarchy) IsRootLevel() bool {
	return ch.Level == 0
}