// Package queries contains query services for category read operations.
// This CategoryQueryService demonstrates AI integration with graceful fallback.
//
// Key Concepts Illustrated:
//   - CQRS: Separates read operations from write operations
//   - AI Service Integration: Optional AI service with fallback mechanism
//   - Fallback Pattern: Domain-based suggestions when AI is unavailable
//   - Caching: Improves performance for frequently accessed data
//   - Query Service Pattern: Optimized for read scenarios
package queries

import (
	"context"
	"fmt"
	"strings"
	"time"

	"brain2-backend/internal/application/dto"
	"brain2-backend/internal/domain/category"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	appErrors "brain2-backend/pkg/errors"
	"go.uber.org/zap"
)


// CategoryQueryService handles read operations for categories with AI-powered suggestions.
type CategoryQueryService struct {
	// Repository readers for CQRS pattern
	categoryReader repository.CategoryReader
	nodeReader     repository.NodeReader
	logger         *zap.Logger
	cache          Cache // Cache interface for performance
	cacheHelper    *CacheHelper // Common cache operations
}

// NewCategoryQueryService creates a new CategoryQueryService with all required dependencies.
// The AI service is optional and the service will work without it.
func NewCategoryQueryService(
	categoryReader repository.CategoryReader,
	nodeReader repository.NodeReader,
	logger *zap.Logger,
	cache Cache,
) *CategoryQueryService {
	return &CategoryQueryService{
		categoryReader: categoryReader,
		nodeReader:     nodeReader,
		logger:         logger,
		cache:          cache,
		cacheHelper:    NewCacheHelper(cache),
	}
}

// GetCategory retrieves a single category with optional nodes and statistics.
func (s *CategoryQueryService) GetCategory(ctx context.Context, query *GetCategoryQuery) (*dto.GetCategoryResult, error) {
	// 1. Check cache first
	cacheKey := GenerateCacheKey("category", query.UserID, query.CategoryID, 
		fmt.Sprintf("nodes=%t", query.IncludeNodes), fmt.Sprintf("stats=%t", query.IncludeStats))
	
	var cachedResult dto.GetCategoryResult
	if found, _ := s.cacheHelper.GetCached(ctx, cacheKey, &cachedResult); found {
		return &cachedResult, nil
	}

	// 2. Parse and validate domain identifiers
	userID, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	categoryID, err := shared.ParseCategoryID(query.CategoryID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid category id: " + err.Error())
	}

	// 3. Retrieve category using reader
	category, err := s.categoryReader.FindByID(ctx, userID.String(), string(categoryID))
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve category")
	}
	if category == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	// 4. Build result with optional components
	result := &dto.GetCategoryResult{
		Category: dto.ToCategoryView(category),
	}

	// 5. Include nodes if requested
	if query.IncludeNodes {
		// For now, we'll get all user nodes and filter by category
		// In a real implementation, you'd want a proper node-category relationship
		nodes, err := s.nodeReader.FindByUser(ctx, userID)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to retrieve nodes")
		}
		// TODO: Filter nodes by category - this requires implementing node-category relationships
		result.Nodes = dto.ToNodeViews(nodes)
	}

	// 6. Include statistics if requested
	if query.IncludeStats {
		nodeCount := 0
		var lastNodeAdded time.Time
		var avgWordsPerNode float64
		var topKeywords []string

		if result.Nodes != nil {
			// Calculate stats from loaded nodes
			nodeCount = len(result.Nodes)
			if nodeCount > 0 {
				totalWords := 0
				keywordCount := make(map[string]int)
				
				for _, nodeView := range result.Nodes {
					// Count words (simple approximation)
					words := strings.Fields(nodeView.Content)
					totalWords += len(words)
					
					// Track keywords
					for _, keyword := range nodeView.Keywords {
						keywordCount[keyword]++
					}
					
					// Track most recent node
					if nodeView.CreatedAt.After(lastNodeAdded) {
						lastNodeAdded = nodeView.CreatedAt
					}
				}
				
				avgWordsPerNode = float64(totalWords) / float64(nodeCount)
				
				// Get top 5 keywords
				topKeywords = getTopKeywords(keywordCount, 5)
			}
		} else {
			// Get node count without loading all nodes
			// TODO: Implement proper node-category counting
			nodeCount = 0
		}

		result.Stats = &dto.CategoryStats{
			NodeCount:       nodeCount,
			LastNodeAdded:   lastNodeAdded,
			AvgWordsPerNode: avgWordsPerNode,
			TopKeywords:     topKeywords,
		}
	}

	// 7. Cache the result
	s.cacheHelper.SetCached(ctx, cacheKey, *result, 5*time.Minute)

	return result, nil
}

// ListCategories retrieves a paginated list of categories.
func (s *CategoryQueryService) ListCategories(ctx context.Context, query *ListCategoriesQuery) (*dto.ListCategoriesResult, error) {
	// 1. Parse and validate domain identifiers
	_, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	// 2. Build repository query
	categoryQuery := repository.CategoryQuery{
		UserID: query.UserID,
	}

	// Add search filtering if specified
	if query.SearchQuery != "" {
		categoryQuery.SearchText = query.SearchQuery
	}

	// 3. Build pagination parameters
	pagination := repository.Pagination{
		Limit:  query.Limit,
		Cursor: query.NextToken,
	}

	// Add sorting if specified
	if query.SortBy != "" {
		pagination.SortBy = query.SortBy
		pagination.SortDirection = query.SortDirection
	}

	// 4. Execute query
	page, err := s.categoryReader.GetCategoriesPage(ctx, categoryQuery, pagination)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve categories page")
	}

	if page == nil {
		return &dto.ListCategoriesResult{
			Categories: []*dto.CategoryView{},
			HasMore:    false,
			Total:      0,
			Count:      0,
		}, nil
	}

	// 5. Convert domain categories to view models
	// Convert []category.Category to []*category.Category
	categoryPtrs := make([]*category.Category, len(page.Items))
	for i := range page.Items {
		categoryPtrs[i] = &page.Items[i]
	}
	categoryViews := dto.ToCategoryViews(categoryPtrs)

	// 6. Add node counts if requested
	if query.IncludeNodeCounts {
		for _, categoryView := range categoryViews {
			// TODO: Implement proper node-category counting
			categoryView.NodeCount = 0
		}
	}

	// 7. Get total count for pagination metadata
	total, err := s.categoryReader.CountByUser(ctx, query.UserID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to count total categories")
	}

	// 8. Build paginated result
	result := &dto.ListCategoriesResult{
		Categories: categoryViews,
		NextToken:  page.NextCursor,
		HasMore:    page.HasMore,
		Total:      total,
		Count:      len(categoryViews),
	}

	return result, nil
}

// GetNodesInCategory retrieves nodes in a specific category.
func (s *CategoryQueryService) GetNodesInCategory(ctx context.Context, query *GetNodesInCategoryQuery) (*dto.GetNodesInCategoryResult, error) {
	// 1. Parse and validate domain identifiers
	userID, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	categoryID, err := shared.ParseCategoryID(query.CategoryID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid category id: " + err.Error())
	}

	// 2. Verify category exists and user owns it
	category, err := s.categoryReader.FindByID(ctx, userID.String(), string(categoryID))
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find category")
	}
	if category == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	// 3. Build pagination parameters
	pagination := repository.Pagination{
		Limit:  query.Limit,
		Cursor: query.NextToken,
	}

	if query.SortBy != "" {
		pagination.SortBy = query.SortBy
		pagination.SortDirection = query.SortDirection
	}

	// 4. Get all nodes for the user (TODO: implement proper category filtering)
	nodes, err := s.nodeReader.FindByUser(ctx, userID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to retrieve nodes")
	}

	// 5. Convert to view models
	nodeViews := dto.ToNodeViews(nodes)

	// 6. Get total count
	total := len(nodeViews)

	// 7. Build result
	result := &dto.GetNodesInCategoryResult{
		CategoryID: query.CategoryID,
		Nodes:      nodeViews,
		Total:      total,
		Count:      len(nodeViews),
		NextToken:  "",
		HasMore:    false,
	}

	return result, nil
}

// GetCategoriesForNode retrieves categories for a specific node.
func (s *CategoryQueryService) GetCategoriesForNode(ctx context.Context, query *GetCategoriesForNodeQuery) (*dto.GetCategoriesForNodeResult, error) {
	// 1. Parse and validate domain identifiers
	userID, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	nodeID, err := shared.ParseNodeID(query.NodeID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid node id: " + err.Error())
	}

	// 2. Verify node exists and user owns it
	node, err := s.nodeReader.FindByID(ctx, userID, nodeID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find node")
	}
	if node == nil {
		return nil, appErrors.NewNotFound("node not found")
	}
	if !node.UserID().Equals(userID) {
		return nil, appErrors.NewUnauthorized("node belongs to different user")
	}

	// 3. Get categories for the node (TODO: implement proper node-category relationships)
	categories := []*category.Category{} // Empty for now

	// 4. Build result
	result := &dto.GetCategoriesForNodeResult{
		NodeID:     query.NodeID,
		Categories: dto.ToCategoryViews(categories),
		Count:      len(categories),
	}

	return result, nil
}

// SuggestCategories provides AI-powered category suggestions with fallback to domain logic.
// This method demonstrates the AI integration pattern with graceful fallback.
func (s *CategoryQueryService) SuggestCategories(ctx context.Context, query *SuggestCategoriesQuery) (*dto.SuggestCategoriesResult, error) {
	// 1. Validate input
	userID, err := shared.ParseUserID(query.UserID)
	if err != nil {
		return nil, appErrors.NewValidation("invalid user id: " + err.Error())
	}

	// 2. Use domain-based suggestions (AI service integration removed)
	// TODO: Re-implement AI service integration when available
	suggestions := s.generateFallbackSuggestions(ctx, userID, query.Content, query.ExistingCategories)
	
	// Filter by confidence and limit
	filteredSuggestions := make([]*dto.CategorySuggestionView, 0)
	for _, suggestion := range suggestions {
		if suggestion.Confidence >= query.MinConfidence {
			filteredSuggestions = append(filteredSuggestions, suggestion)
		}
		if len(filteredSuggestions) >= query.MaxSuggestions {
			break
		}
	}

	return &dto.SuggestCategoriesResult{
		Suggestions: filteredSuggestions,
		Count:       len(filteredSuggestions),
		Message:     "Category suggestions generated using domain logic",
		Source:      "fallback",
	}, nil
}

// generateFallbackSuggestions creates category suggestions using domain logic
// when AI service is not available. This demonstrates the fallback pattern.
func (s *CategoryQueryService) generateFallbackSuggestions(ctx context.Context, userID shared.UserID, content string, existingCategories []string) []*dto.CategorySuggestionView {
	suggestions := make([]*dto.CategorySuggestionView, 0)
	
	// Extract keywords from content for analysis
	words := strings.Fields(strings.ToLower(content))
	
	// Define some basic category patterns based on keywords
	categoryPatterns := map[string][]string{
		"Work":       {"work", "project", "meeting", "deadline", "task", "job", "career"},
		"Personal":   {"personal", "family", "friend", "home", "life", "relationship"},
		"Learning":   {"learn", "study", "book", "course", "education", "skill", "knowledge"},
		"Health":     {"health", "fitness", "exercise", "medical", "doctor", "wellness"},
		"Finance":    {"money", "budget", "investment", "finance", "bank", "payment", "cost"},
		"Technology": {"tech", "software", "computer", "programming", "code", "digital", "app"},
		"Travel":     {"travel", "trip", "vacation", "journey", "flight", "hotel", "destination"},
		"Ideas":      {"idea", "thought", "inspiration", "creativity", "brainstorm", "concept"},
	}
	
	// Check which patterns match the content
	for categoryTitle, keywords := range categoryPatterns {
		matchCount := 0
		totalKeywords := len(keywords)
		
		for _, word := range words {
			for _, keyword := range keywords {
				if strings.Contains(word, keyword) {
					matchCount++
					break
				}
			}
		}
		
		if matchCount > 0 {
			confidence := float64(matchCount) / float64(totalKeywords)
			if confidence >= 0.1 { // Minimum threshold for fallback suggestions
				
				// Check if this category already exists
				isExisting := false
				for _, existing := range existingCategories {
					if strings.EqualFold(existing, categoryTitle) {
						isExisting = true
						break
					}
				}
				
				suggestion := &dto.CategorySuggestionView{
					Title:       categoryTitle,
					Description: fmt.Sprintf("Suggested based on content keywords related to %s", strings.ToLower(categoryTitle)),
					Confidence:  confidence,
					Reason:      fmt.Sprintf("Found %d relevant keywords in content", matchCount),
					IsExisting:  isExisting,
				}
				
				suggestions = append(suggestions, suggestion)
			}
		}
	}
	
	// Sort by confidence (highest first)
	for i := 0; i < len(suggestions)-1; i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[i].Confidence < suggestions[j].Confidence {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}
	
	return suggestions
}

// Helper function to get top keywords from a count map
func getTopKeywords(keywordCount map[string]int, limit int) []string {
	// Create slice of keyword-count pairs
	type keywordPair struct {
		keyword string
		count   int
	}
	
	pairs := make([]keywordPair, 0, len(keywordCount))
	for keyword, count := range keywordCount {
		pairs = append(pairs, keywordPair{keyword, count})
	}
	
	// Sort by count (descending)
	for i := 0; i < len(pairs)-1; i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[i].count < pairs[j].count {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	
	// Extract top keywords
	result := make([]string, 0, limit)
	for i := 0; i < len(pairs) && i < limit; i++ {
		result = append(result, pairs[i].keyword)
	}
	
	return result
}

// InvalidateCategoryCache invalidates cached data for categories.
func (s *CategoryQueryService) InvalidateCategoryCache(ctx context.Context, userID, categoryID string) {
	if s.cache == nil {
		return
	}

	patterns := []string{
		fmt.Sprintf("category:%s:%s:*", userID, categoryID),
		fmt.Sprintf("category:%s:*", userID),
	}

	for _, pattern := range patterns {
		s.cache.Delete(ctx, pattern)
	}
}