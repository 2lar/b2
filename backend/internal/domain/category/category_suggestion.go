package category

import "time"

// CategorySuggestion represents an AI-generated suggestion for categorizing content
type CategorySuggestion struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	NodeID     string     `json:"node_id"`
	Name       string     `json:"name"`       // Category name/title
	Level      int        `json:"level"`      // Hierarchy level
	ParentID   *string    `json:"parent_id,omitempty"` // Optional parent category ID
	Confidence float64    `json:"confidence"` // 0.0 - 1.0
	Reason     string     `json:"reason"`     // AI explanation
	Status     string     `json:"status"`     // "pending", "accepted", "rejected"
	CreatedAt  time.Time  `json:"created_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
}

// CategoryConnection represents a suggested connection between categories
type CategoryConnection struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	SourceID     string    `json:"source_category_id"`
	TargetID     string    `json:"target_category_id"`
	Relationship string    `json:"relationship"` // "parent", "child", "related"
	Confidence   float64   `json:"confidence"`
	Reasoning    string    `json:"reasoning"`
	Status       string    `json:"status"` // "pending", "accepted", "rejected"
	CreatedAt    time.Time `json:"created_at"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
}

// NewCategorySuggestion creates a new category suggestion
func NewCategorySuggestion(userID, nodeID, name string, level int, confidence float64, reason string) *CategorySuggestion {
	return &CategorySuggestion{
		UserID:     userID,
		NodeID:     nodeID,
		Name:       name,
		Level:      level,
		Confidence: confidence,
		Reason:     reason,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
}

// NewCategoryConnection creates a new category connection suggestion
func NewCategoryConnection(userID, sourceID, targetID, relationship string, confidence float64, reasoning string) *CategoryConnection {
	return &CategoryConnection{
		UserID:       userID,
		SourceID:     sourceID,
		TargetID:     targetID,
		Relationship: relationship,
		Confidence:   confidence,
		Reasoning:    reasoning,
		Status:       "pending",
		CreatedAt:    time.Now(),
	}
}

// Accept marks the suggestion as accepted
func (cs *CategorySuggestion) Accept() {
	cs.Status = "accepted"
	now := time.Now()
	cs.ProcessedAt = &now
}

// Reject marks the suggestion as rejected
func (cs *CategorySuggestion) Reject() {
	cs.Status = "rejected"
	now := time.Now()
	cs.ProcessedAt = &now
}

// IsHighConfidence returns true if confidence >= 0.8
func (cs *CategorySuggestion) IsHighConfidence() bool {
	return cs.Confidence >= 0.8
}

// Accept marks the connection as accepted
func (cc *CategoryConnection) Accept() {
	cc.Status = "accepted"
	now := time.Now()
	cc.ProcessedAt = &now
}

// Reject marks the connection as rejected
func (cc *CategoryConnection) Reject() {
	cc.Status = "rejected"
	now := time.Now()
	cc.ProcessedAt = &now
}

// IsHighConfidence returns true if confidence >= 0.8
func (cc *CategoryConnection) IsHighConfidence() bool {
	return cc.Confidence >= 0.8
}

// CategoryInsights represents analytical insights about categories
type CategoryInsights struct {
	UserID                string                    `json:"user_id"`
	TotalCategories       int                       `json:"total_categories"`
	MaxDepth              int                       `json:"max_depth"`
	AverageDepth          float64                   `json:"average_depth"`
	CategoryCounts        map[string]int            `json:"category_counts"`   // category_id -> note_count
	HierarchyStats        map[string]int            `json:"hierarchy_stats"`   // level -> category_count
	Suggestions           []CategorySuggestion      `json:"suggestions"`
	Connections           []CategoryConnection      `json:"connections"`
	MostActiveCategories  []CategoryActivity        `json:"most_active_categories"`
	CategoryGrowthTrends  []CategoryGrowthTrend     `json:"category_growth_trends"`
	SuggestedConnections  []CategoryConnection      `json:"suggested_connections"`
	KnowledgeGaps         []KnowledgeGap            `json:"knowledge_gaps"`
	GeneratedAt           time.Time                 `json:"generated_at"`
}

// NewCategoryInsights creates new category insights
func NewCategoryInsights(userID string) *CategoryInsights {
	return &CategoryInsights{
		UserID:                userID,
		CategoryCounts:        make(map[string]int),
		HierarchyStats:        make(map[string]int),
		Suggestions:           []CategorySuggestion{},
		Connections:           []CategoryConnection{},
		MostActiveCategories:  []CategoryActivity{},
		CategoryGrowthTrends:  []CategoryGrowthTrend{},
		SuggestedConnections:  []CategoryConnection{},
		KnowledgeGaps:         []KnowledgeGap{},
		GeneratedAt:           time.Now(),
	}
}

// CategoryActivity represents activity metrics for a category
type CategoryActivity struct {
	CategoryID   string    `json:"category_id"`
	CategoryName string    `json:"category_name"`
	NodeCount    int       `json:"node_count"`
	LastActivity time.Time `json:"last_activity"`
	GrowthRate   float64   `json:"growth_rate"`
}

// CategoryGrowthTrend represents growth trends for categories
type CategoryGrowthTrend struct {
	CategoryID   string    `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Period       string    `json:"period"`       // "week", "month", "quarter"
	GrowthRate   float64   `json:"growth_rate"`  // Percentage growth
	NodeCount    int       `json:"node_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// KnowledgeGap represents identified gaps in the knowledge structure
type KnowledgeGap struct {
	ID          string   `json:"id"`
	UserID      string   `json:"user_id"`
	GapType     string   `json:"gap_type"`     // "missing_category", "isolated_nodes", "sparse_connections"
	Description string   `json:"description"`
	Severity    string   `json:"severity"`     // "low", "medium", "high"
	NodeIDs     []string `json:"node_ids"`     // Related nodes
	Suggestions []string `json:"suggestions"`  // Recommended actions
	CreatedAt   time.Time `json:"created_at"`
}