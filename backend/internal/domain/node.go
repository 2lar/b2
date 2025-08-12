package domain

import (
	"time"

	"github.com/google/uuid"
)

// Node represents a memory, thought, or piece of knowledge in a user's knowledge graph
type Node struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	Keywords  []string  `json:"keywords"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	Version   int       `json:"version"`
}

// NewNode creates a new node with consistent initialization.
// New nodes always start at Version 0 to ensure proper optimistic locking.
func NewNode(userID, content string, tags []string) Node {
	return Node{
		ID:        uuid.New().String(),
		UserID:    userID,
		Content:   content,
		Keywords:  []string{}, // Will be set by service layer
		Tags:      tags,
		CreatedAt: time.Now(),
		Version:   0, // ALWAYS start at 0 for new nodes
	}
}
