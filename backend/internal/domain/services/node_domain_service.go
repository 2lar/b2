// Package services contains domain services that encapsulate complex business logic.
package services

import (
	"fmt"
	"strings"
	"time"
)

// NodeDomainService encapsulates complex business logic for node operations.
// This service contains pure domain logic without any infrastructure concerns.
type NodeDomainService struct {
	similarityThreshold   float64
	maxConnectionsPerNode int
	recencyWeight         float64
	diversityThreshold    float64
}

// NewNodeDomainService creates a new node domain service.
func NewNodeDomainService(
	similarityThreshold float64,
	maxConnectionsPerNode int,
	recencyWeight float64,
	diversityThreshold float64,
) *NodeDomainService {
	return &NodeDomainService{
		similarityThreshold:   similarityThreshold,
		maxConnectionsPerNode: maxConnectionsPerNode,
		recencyWeight:         recencyWeight,
		diversityThreshold:    diversityThreshold,
	}
}

// ValidateNodeContent validates node content according to business rules.
func (s *NodeDomainService) ValidateNodeContent(content string, maxLength int) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("node content cannot be empty")
	}
	
	if len(content) > maxLength {
		return fmt.Errorf("node content exceeds maximum length of %d characters", maxLength)
	}
	
	// Additional business rules
	if len(strings.Fields(content)) < 3 {
		return fmt.Errorf("node content must contain at least 3 words")
	}
	
	return nil
}

// CalculateNodeImportance calculates the importance score of a node.
// This is pure business logic that belongs in the domain layer.
func (s *NodeDomainService) CalculateNodeImportance(
	contentLength int,
	createdAt time.Time,
	connectionCount int,
	recentActivityCount int,
) float64 {
	// Base importance from content length (normalized)
	contentScore := float64(contentLength) / 10000.0
	if contentScore > 1.0 {
		contentScore = 1.0
	}
	
	// Connection score (normalized by max connections)
	connectionScore := float64(connectionCount) / float64(s.maxConnectionsPerNode)
	if connectionScore > 1.0 {
		connectionScore = 1.0
	}
	
	// Recency score
	daysSinceCreation := time.Since(createdAt).Hours() / 24
	recencyScore := 1.0 / (1.0 + daysSinceCreation/30) // Decay over 30 days
	
	// Activity score
	activityScore := float64(recentActivityCount) / 10.0
	if activityScore > 1.0 {
		activityScore = 1.0
	}
	
	// Weighted combination
	importance := contentScore*0.2 + 
		connectionScore*0.3 + 
		recencyScore*s.recencyWeight + 
		activityScore*(0.5-s.recencyWeight)
	
	return importance
}

// ShouldAutoConnect determines if two nodes should be automatically connected.
// This encapsulates the business rules for automatic connection creation.
func (s *NodeDomainService) ShouldAutoConnect(
	sourceUserID string,
	targetUserID string,
	sourceCreatedAt time.Time,
	targetCreatedAt time.Time,
	similarity float64,
	existingConnectionCount int,
) (bool, string) {
	// Check similarity threshold
	if similarity < s.similarityThreshold {
		return false, "similarity below threshold"
	}
	
	// Check connection limit
	if existingConnectionCount >= s.maxConnectionsPerNode {
		return false, "connection limit reached"
	}
	
	// Check if nodes are from same user (business rule)
	if sourceUserID != targetUserID {
		return false, "cross-user connections not allowed"
	}
	
	// Check time proximity (don't connect very old nodes to very new ones automatically)
	timeDiff := sourceCreatedAt.Sub(targetCreatedAt)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > 365*24*time.Hour { // More than a year apart
		if similarity < s.similarityThreshold*1.5 { // Require higher similarity for old connections
			return false, "temporal distance too large for similarity level"
		}
	}
	
	// Check for diversity (avoid creating echo chambers)
	if similarity > (1.0 - s.diversityThreshold) {
		return false, "nodes too similar (diversity threshold)"
	}
	
	return true, ""
}

// CalculateEdgeWeight calculates the weight for an edge between two nodes.
// This is pure domain logic that determines connection strength.
func (s *NodeDomainService) CalculateEdgeWeight(
	sourceCreatedAt time.Time,
	targetCreatedAt time.Time,
	sourceContentLength int,
	targetContentLength int,
	similarity float64,
) float64 {
	// Base weight from similarity
	weight := similarity
	
	// Adjust for temporal proximity
	timeDiff := sourceCreatedAt.Sub(targetCreatedAt)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	
	// Boost weight for temporally close nodes
	if timeDiff < 24*time.Hour {
		weight *= 1.2
	} else if timeDiff < 7*24*time.Hour {
		weight *= 1.1
	} else if timeDiff > 30*24*time.Hour {
		weight *= 0.9
	}
	
	// Adjust for content length similarity
	lengthRatio := float64(sourceContentLength) / float64(targetContentLength)
	if lengthRatio > 1 {
		lengthRatio = 1 / lengthRatio
	}
	weight *= (0.8 + 0.2*lengthRatio) // Small adjustment based on length similarity
	
	// Ensure weight is in valid range [0, 1]
	if weight > 1.0 {
		weight = 1.0
	} else if weight < 0.0 {
		weight = 0.0
	}
	
	return weight
}

// DetermineNodeCategory suggests a category for a node based on its content.
// This is business logic for auto-categorization.
func (s *NodeDomainService) DetermineNodeCategory(
	contentWords []string,
	existingCategories []string,
) string {
	// This is simplified business logic
	// Real implementation would use NLP/ML
	
	// Map of keywords to categories
	categoryKeywords := map[string][]string{
		"technology": {"code", "software", "programming", "api", "database"},
		"business":   {"meeting", "project", "client", "revenue", "strategy"},
		"personal":   {"todo", "reminder", "note", "thought", "idea"},
		"research":   {"study", "paper", "analysis", "data", "finding"},
	}
	
	// Count keyword matches for each category
	scores := make(map[string]int)
	for category, keywords := range categoryKeywords {
		for _, word := range contentWords {
			wordLower := strings.ToLower(word)
			for _, keyword := range keywords {
				if strings.Contains(wordLower, keyword) {
					scores[category]++
				}
			}
		}
	}
	
	// Find category with highest score
	maxScore := 0
	bestCategory := "general"
	for category, score := range scores {
		if score > maxScore {
			maxScore = score
			bestCategory = category
		}
	}
	
	return bestCategory
}

