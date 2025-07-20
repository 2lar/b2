package domain

import "time"

// Node represents a memory, thought, or piece of knowledge in a user's knowledge graph
type Node struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	Keywords  []string  `json:"keywords"`
	CreatedAt time.Time `json:"created_at"`
	Version   int       `json:"version"`
}
