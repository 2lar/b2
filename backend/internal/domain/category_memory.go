package domain

import "time"

// CategoryMemory represents the many-to-many relationship between categories and memories
type CategoryMemory struct {
	ID         string    `json:"id"`
	CategoryID string    `json:"category_id"`
	MemoryID   string    `json:"memory_id"`
	AddedAt    time.Time `json:"added_at"`
}