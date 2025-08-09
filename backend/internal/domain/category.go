package domain

import "time"

// Category represents a user-defined category for organizing memories
type Category struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Level       int       `json:"level"`        // 0 = top level, 1 = sub, 2 = sub-sub
	ParentID    *string   `json:"parent_id"`    // ID of parent category (nil for root categories)
	Color       *string   `json:"color"`        // Hex color code for UI
	Icon        *string   `json:"icon"`         // Icon identifier for UI
	AIGenerated bool      `json:"ai_generated"` // Whether this category was created by AI
	NoteCount   int       `json:"note_count"`   // Number of memories in this category
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CategoryHierarchy represents parent-child relationships between categories
type CategoryHierarchy struct {
	UserID    string    `json:"user_id"`
	ParentID  string    `json:"parent_id"`
	ChildID   string    `json:"child_id"`
	CreatedAt time.Time `json:"created_at"`
}

// NodeCategory represents the many-to-many relationship between nodes and categories
type NodeCategory struct {
	UserID     string    `json:"user_id"`
	NodeID     string    `json:"node_id"`
	CategoryID string    `json:"category_id"`
	Confidence float64   `json:"confidence"` // AI confidence score (0.0-1.0)
	Method     string    `json:"method"`     // "ai", "manual", "rule-based"
	CreatedAt  time.Time `json:"created_at"`
}

// CategorySuggestion represents an AI suggestion for categorizing content
type CategorySuggestion struct {
	Name       string  `json:"name"`
	Level      int     `json:"level"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
	ParentID   *string `json:"parent_id,omitempty"`
}

// CategoryInsights provides analytics and insights about category usage
type CategoryInsights struct {
	MostActiveCategories []CategoryActivity    `json:"most_active_categories"`
	CategoryGrowthTrends []CategoryGrowthTrend `json:"category_growth_trends"`
	SuggestedConnections []CategoryConnection  `json:"suggested_connections"`
	KnowledgeGaps        []KnowledgeGap        `json:"knowledge_gaps"`
}

// CategoryActivity represents usage statistics for a category
type CategoryActivity struct {
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
	MemoryCount  int    `json:"memory_count"`
	RecentAdds   int    `json:"recent_adds"`
}

// CategoryGrowthTrend represents growth patterns over time
type CategoryGrowthTrend struct {
	CategoryID   string    `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Date         time.Time `json:"date"`
	MemoryCount  int       `json:"memory_count"`
}

// CategoryConnection suggests relationships between categories
type CategoryConnection struct {
	Category1ID   string  `json:"category1_id"`
	Category1Name string  `json:"category1_name"`
	Category2ID   string  `json:"category2_id"`
	Category2Name string  `json:"category2_name"`
	Strength      float64 `json:"strength"`
	Reason        string  `json:"reason"`
}

// KnowledgeGap identifies potential areas for knowledge expansion
type KnowledgeGap struct {
	Topic               string   `json:"topic"`
	Confidence          float64  `json:"confidence"`
	SuggestedCategories []string `json:"suggested_categories"`
	Reason              string   `json:"reason"`
}
