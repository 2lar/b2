package domain

import "time"

// CategoryMemory represents the many-to-many relationship between categories and memories
// DEPRECATED: Use NodeCategory instead for enhanced functionality
type CategoryMemory struct {
	ID         string    `json:"id"`
	CategoryID string    `json:"category_id"`
	MemoryID   string    `json:"memory_id"`
	AddedAt    time.Time `json:"added_at"`
}

// ConvertToNodeCategory converts a CategoryMemory to NodeCategory for migration
func (cm *CategoryMemory) ConvertToNodeCategory(userID string) *NodeCategory {
	return &NodeCategory{
		UserID:     userID,
		NodeID:     cm.MemoryID,
		CategoryID: cm.CategoryID,
		Confidence: 1.0, // Manual assignments have high confidence
		Source:     "manual",
		AssignedAt: cm.AddedAt,
	}
}
