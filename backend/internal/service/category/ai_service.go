// Package category provides enhanced business logic for AI-powered category management.
package category

import (
	"context"
	"log"
	"strings"
	"time"

	"brain2-backend/internal/domain"
	"brain2-backend/internal/repository"
	"brain2-backend/internal/service/llm"
	appErrors "brain2-backend/pkg/errors"

	"github.com/google/uuid"
)

// EnhancedService extends the basic category service with AI-powered features
type EnhancedService interface {
	Service // Embed the basic service interface

	// AI-powered categorization
	CategorizeNode(ctx context.Context, node domain.Node) ([]domain.Category, error)
	SuggestCategories(ctx context.Context, content string, userID string) ([]domain.CategorySuggestion, error)

	// Hierarchy management
	BuildCategoryHierarchy(ctx context.Context, userID string) error
	GetCategoryTree(ctx context.Context, userID string) ([]domain.Category, error)
	CreateCategoryWithParent(ctx context.Context, userID, title, description string, parentID *string) (*domain.Category, error)

	// Category optimization
	MergeSimilarCategories(ctx context.Context, userID string, threshold float64) error
	OptimizeCategoryStructure(ctx context.Context, userID string) error

	// Analytics
	GenerateCategoryInsights(ctx context.Context, userID string) (*domain.CategoryInsights, error)
}

// enhancedService implements the EnhancedService interface
type enhancedService struct {
	repo   repository.Repository
	llmSvc *llm.Service
}

// NewEnhancedService creates a new enhanced category service
func NewEnhancedService(repo repository.Repository, llmSvc *llm.Service) EnhancedService {
	return &enhancedService{
		repo:   repo,
		llmSvc: llmSvc,
	}
}

// CategorizeNode automatically categorizes a node using AI and keyword matching
func (s *enhancedService) CategorizeNode(ctx context.Context, node domain.Node) ([]domain.Category, error) {
	// 1. Get existing categories for context
	existingCategories, err := s.repo.FindCategories(ctx, repository.CategoryQuery{
		UserID: node.UserID,
	})
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to fetch existing categories")
	}

	var finalCategories []domain.Category
	var mappings []domain.NodeCategory

	// 2. Try AI categorization first
	if s.llmSvc != nil && s.llmSvc.IsAvailable() {
		suggestions, err := s.llmSvc.SuggestCategories(ctx, node.Content, existingCategories)
		if err != nil {
			log.Printf("AI categorization failed for node %s: %v", node.ID, err)
		} else {
			// Process AI suggestions
			for _, suggestion := range suggestions {
				category, err := s.processAISuggestion(ctx, node.UserID, suggestion, existingCategories)
				if err != nil {
					log.Printf("Failed to process AI suggestion: %v", err)
					continue
				}
				if category != nil {
					finalCategories = append(finalCategories, *category)
					mappings = append(mappings, domain.NodeCategory{
						UserID:     node.UserID,
						NodeID:     node.ID,
						CategoryID: category.ID,
						Confidence: suggestion.Confidence,
						Method:     "ai",
						CreatedAt:  time.Now(),
					})
				}
			}
		}
	}

	// 3. Fallback to keyword-based categorization if no AI results
	if len(finalCategories) == 0 {
		keywordCategories, err := s.categorizeByKeywords(ctx, node, existingCategories)
		if err != nil {
			log.Printf("Keyword categorization failed: %v", err)
		} else {
			finalCategories = keywordCategories
			for _, category := range keywordCategories {
				mappings = append(mappings, domain.NodeCategory{
					UserID:     node.UserID,
					NodeID:     node.ID,
					CategoryID: category.ID,
					Confidence: 0.8, // Lower confidence for keyword matching
					Method:     "rule-based",
					CreatedAt:  time.Now(),
				})
			}
		}
	}

	// 4. Assign node to categories
	if len(mappings) > 0 {
		if err := s.repo.BatchAssignCategories(ctx, mappings); err != nil {
			return nil, appErrors.Wrap(err, "failed to assign categories to node")
		}

		// Update category note counts
		s.updateCategoryCounts(ctx, node.UserID, finalCategories)
	}

	return finalCategories, nil
}

// SuggestCategories provides AI-powered category suggestions for content
func (s *enhancedService) SuggestCategories(ctx context.Context, content string, userID string) ([]domain.CategorySuggestion, error) {
	if s.llmSvc == nil || !s.llmSvc.IsAvailable() {
		return nil, appErrors.NewValidation("AI categorization service is not available")
	}

	existingCategories, err := s.repo.FindCategories(ctx, repository.CategoryQuery{
		UserID: userID,
	})
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to fetch existing categories")
	}

	suggestions, err := s.llmSvc.SuggestCategories(ctx, content, existingCategories)
	if err != nil {
		return nil, appErrors.Wrap(err, "AI categorization failed")
	}

	return suggestions, nil
}

// BuildCategoryHierarchy analyzes existing categories and creates parent-child relationships
func (s *enhancedService) BuildCategoryHierarchy(ctx context.Context, userID string) error {
	categories, err := s.repo.FindCategories(ctx, repository.CategoryQuery{
		UserID: userID,
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to fetch categories")
	}

	if s.llmSvc != nil && s.llmSvc.IsAvailable() {
		hierarchyMap, err := s.llmSvc.AnalyzeCategoryHierarchy(ctx, categories)
		if err != nil {
			log.Printf("AI hierarchy analysis failed: %v", err)
			return nil // Don't fail the whole operation
		}

		// Apply hierarchy suggestions
		for childID, parentID := range hierarchyMap {
			hierarchy := domain.CategoryHierarchy{
				UserID:    userID,
				ParentID:  parentID,
				ChildID:   childID,
				CreatedAt: time.Now(),
			}

			if err := s.repo.CreateCategoryHierarchy(ctx, hierarchy); err != nil {
				log.Printf("Failed to create hierarchy %s -> %s: %v", parentID, childID, err)
			}
		}
	}

	return nil
}

// GetCategoryTree retrieves the hierarchical category structure
func (s *enhancedService) GetCategoryTree(ctx context.Context, userID string) ([]domain.Category, error) {
	return s.repo.GetCategoryTree(ctx, userID)
}

// CreateCategoryWithParent creates a category with a specified parent
func (s *enhancedService) CreateCategoryWithParent(ctx context.Context, userID, title, description string, parentID *string) (*domain.Category, error) {
	// Basic validation
	if userID == "" {
		return nil, appErrors.NewValidation("userID cannot be empty")
	}
	if title == "" {
		return nil, appErrors.NewValidation("title cannot be empty")
	}

	// Determine level based on parent
	level := 0
	if parentID != nil {
		parent, err := s.repo.FindCategoryByID(ctx, userID, *parentID)
		if err != nil {
			return nil, appErrors.Wrap(err, "failed to find parent category")
		}
		if parent == nil {
			return nil, appErrors.NewNotFound("parent category not found")
		}
		level = parent.Level + 1
	}

	category := domain.Category{
		ID:          uuid.New().String(),
		UserID:      userID,
		Title:       title,
		Description: description,
		Level:       level,
		ParentID:    parentID,
		AIGenerated: false,
		NoteCount:   0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateCategory(ctx, category); err != nil {
		return nil, appErrors.Wrap(err, "failed to create category")
	}

	// Create hierarchy relationship if parent exists
	if parentID != nil {
		hierarchy := domain.CategoryHierarchy{
			UserID:    userID,
			ParentID:  *parentID,
			ChildID:   category.ID,
			CreatedAt: time.Now(),
		}
		if err := s.repo.CreateCategoryHierarchy(ctx, hierarchy); err != nil {
			log.Printf("Failed to create hierarchy relationship: %v", err)
		}
	}

	return &category, nil
}

// MergeSimilarCategories finds and merges categories that are too similar
func (s *enhancedService) MergeSimilarCategories(ctx context.Context, userID string, threshold float64) error {
	if s.llmSvc == nil || !s.llmSvc.IsAvailable() {
		return nil // Skip if AI is not available
	}

	categories, err := s.repo.FindCategories(ctx, repository.CategoryQuery{
		UserID: userID,
	})
	if err != nil {
		return appErrors.Wrap(err, "failed to fetch categories")
	}

	similarPairs, err := s.llmSvc.DetectSimilarCategories(ctx, categories, threshold)
	if err != nil {
		log.Printf("Similar category detection failed: %v", err)
		return nil // Don't fail the operation
	}

	for _, pair := range similarPairs {
		err := s.mergeTwoCategories(ctx, userID, pair.Category1ID, pair.Category2ID)
		if err != nil {
			log.Printf("Failed to merge categories %s and %s: %v",
				pair.Category1ID, pair.Category2ID, err)
		}
	}

	return nil
}

// OptimizeCategoryStructure performs various optimizations on the category structure
func (s *enhancedService) OptimizeCategoryStructure(ctx context.Context, userID string) error {
	// Build hierarchy
	if err := s.BuildCategoryHierarchy(ctx, userID); err != nil {
		log.Printf("Failed to build hierarchy: %v", err)
	}

	// Merge similar categories
	if err := s.MergeSimilarCategories(ctx, userID, 0.8); err != nil {
		log.Printf("Failed to merge similar categories: %v", err)
	}

	return nil
}

// GenerateCategoryInsights provides analytics about category usage
func (s *enhancedService) GenerateCategoryInsights(ctx context.Context, userID string) (*domain.CategoryInsights, error) {
	// This is a placeholder implementation
	// In a full implementation, this would analyze usage patterns, growth trends, etc.
	insights := &domain.CategoryInsights{
		MostActiveCategories: []domain.CategoryActivity{},
		CategoryGrowthTrends: []domain.CategoryGrowthTrend{},
		SuggestedConnections: []domain.CategoryConnection{},
		KnowledgeGaps:        []domain.KnowledgeGap{},
	}

	return insights, nil
}

// Helper methods

// processAISuggestion processes an AI category suggestion
func (s *enhancedService) processAISuggestion(ctx context.Context, userID string, suggestion domain.CategorySuggestion, existingCategories []domain.Category) (*domain.Category, error) {
	// Check if suggestion matches existing category
	for _, existing := range existingCategories {
		if strings.EqualFold(existing.Title, suggestion.Name) {
			return &existing, nil
		}
	}

	// Create new category from suggestion
	category := domain.Category{
		ID:          uuid.New().String(),
		UserID:      userID,
		Title:       suggestion.Name,
		Description: suggestion.Reason,
		Level:       suggestion.Level,
		ParentID:    suggestion.ParentID,
		AIGenerated: true,
		NoteCount:   0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateCategory(ctx, category); err != nil {
		return nil, err
	}

	return &category, nil
}

// categorizeByKeywords provides keyword-based categorization as fallback
func (s *enhancedService) categorizeByKeywords(ctx context.Context, node domain.Node, existingCategories []domain.Category) ([]domain.Category, error) {
	content := strings.ToLower(node.Content)
	var matchedCategories []domain.Category

	// Simple keyword matching against existing categories
	for _, category := range existingCategories {
		categoryKeywords := strings.ToLower(category.Title + " " + category.Description)

		// Check for keyword overlap
		if s.hasKeywordOverlap(content, categoryKeywords) {
			matchedCategories = append(matchedCategories, category)
		}
	}

	// If no existing categories match, create a general one
	if len(matchedCategories) == 0 {
		general, err := s.findOrCreateGeneralCategory(ctx, node.UserID)
		if err != nil {
			return nil, err
		}
		if general != nil {
			matchedCategories = append(matchedCategories, *general)
		}
	}

	return matchedCategories, nil
}

// hasKeywordOverlap checks if there's significant keyword overlap
func (s *enhancedService) hasKeywordOverlap(content, categoryKeywords string) bool {
	contentWords := strings.Fields(content)
	categoryWords := strings.Fields(categoryKeywords)

	if len(contentWords) == 0 || len(categoryWords) == 0 {
		return false
	}

	matches := 0
	for _, contentWord := range contentWords {
		if len(contentWord) > 3 { // Only consider words longer than 3 characters
			for _, categoryWord := range categoryWords {
				if contentWord == categoryWord {
					matches++
					break
				}
			}
		}
	}

	// Require at least 20% word overlap
	threshold := len(contentWords) / 5
	if threshold < 1 {
		threshold = 1
	}

	return matches >= threshold
}

// findOrCreateGeneralCategory finds or creates a general category
func (s *enhancedService) findOrCreateGeneralCategory(ctx context.Context, userID string) (*domain.Category, error) {
	// Try to find existing general category
	categories, err := s.repo.FindCategories(ctx, repository.CategoryQuery{
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}

	for _, category := range categories {
		if strings.EqualFold(category.Title, "general") || strings.EqualFold(category.Title, "misc") {
			return &category, nil
		}
	}

	// Create new general category
	category := domain.Category{
		ID:          uuid.New().String(),
		UserID:      userID,
		Title:       "General",
		Description: "General uncategorized content",
		Level:       0,
		AIGenerated: false,
		NoteCount:   0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateCategory(ctx, category); err != nil {
		return nil, err
	}

	return &category, nil
}

// mergeTwoCategories merges two categories by moving all nodes to the first one
func (s *enhancedService) mergeTwoCategories(ctx context.Context, userID, keepID, mergeID string) error {
	// Get nodes from the category being merged
	nodes, err := s.repo.FindNodesByCategory(ctx, userID, mergeID)
	if err != nil {
		return err
	}

	// Move all nodes to the keep category
	var mappings []domain.NodeCategory
	for _, node := range nodes {
		mappings = append(mappings, domain.NodeCategory{
			UserID:     userID,
			NodeID:     node.ID,
			CategoryID: keepID,
			Confidence: 0.9, // High confidence for manual merge
			Method:     "manual",
			CreatedAt:  time.Now(),
		})
	}

	if len(mappings) > 0 {
		if err := s.repo.BatchAssignCategories(ctx, mappings); err != nil {
			return err
		}
	}

	// Delete the merged category
	return s.repo.DeleteCategory(ctx, userID, mergeID)
}

// updateCategoryCounts updates the note counts for categories
func (s *enhancedService) updateCategoryCounts(ctx context.Context, userID string, categories []domain.Category) {
	counts := make(map[string]int)
	for _, category := range categories {
		nodes, err := s.repo.FindNodesByCategory(ctx, userID, category.ID)
		if err != nil {
			continue
		}
		counts[category.ID] = len(nodes)
	}

	if len(counts) > 0 {
		if err := s.repo.UpdateCategoryNoteCounts(ctx, userID, counts); err != nil {
			log.Printf("Failed to update category counts: %v", err)
		}
	}
}

// Implement the basic Service interface by delegating to a basic service
func (s *enhancedService) CreateCategory(ctx context.Context, userID, title, description string) (*domain.Category, error) {
	return s.CreateCategoryWithParent(ctx, userID, title, description, nil)
}

func (s *enhancedService) UpdateCategory(ctx context.Context, userID, categoryID, title, description string) (*domain.Category, error) {
	existing, err := s.repo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to find category")
	}
	if existing == nil {
		return nil, appErrors.NewNotFound("category not found")
	}

	updated := *existing
	updated.Title = title
	updated.Description = description
	updated.UpdatedAt = time.Now()

	if err := s.repo.UpdateCategory(ctx, updated); err != nil {
		return nil, appErrors.Wrap(err, "failed to update category")
	}

	return &updated, nil
}

func (s *enhancedService) DeleteCategory(ctx context.Context, userID, categoryID string) error {
	return s.repo.DeleteCategory(ctx, userID, categoryID)
}

func (s *enhancedService) GetCategory(ctx context.Context, userID, categoryID string) (*domain.Category, error) {
	category, err := s.repo.FindCategoryByID(ctx, userID, categoryID)
	if err != nil {
		return nil, appErrors.Wrap(err, "failed to get category")
	}
	if category == nil {
		return nil, appErrors.NewNotFound("category not found")
	}
	return category, nil
}

func (s *enhancedService) ListCategories(ctx context.Context, userID string) ([]domain.Category, error) {
	return s.repo.FindCategories(ctx, repository.CategoryQuery{UserID: userID})
}

func (s *enhancedService) AssignNodeToCategory(ctx context.Context, userID, categoryID, nodeID string) error {
	mapping := domain.NodeCategory{
		UserID:     userID,
		NodeID:     nodeID,
		CategoryID: categoryID,
		Confidence: 1.0,
		Method:     "manual",
		CreatedAt:  time.Now(),
	}
	return s.repo.AssignNodeToCategory(ctx, mapping)
}

func (s *enhancedService) RemoveNodeFromCategory(ctx context.Context, userID, categoryID, nodeID string) error {
	return s.repo.RemoveNodeFromCategory(ctx, userID, nodeID, categoryID)
}

func (s *enhancedService) GetNodesInCategory(ctx context.Context, userID, categoryID string) ([]domain.Node, error) {
	return s.repo.FindNodesByCategory(ctx, userID, categoryID)
}

func (s *enhancedService) GetCategoriesForNode(ctx context.Context, userID, nodeID string) ([]domain.Category, error) {
	return s.repo.FindCategoriesForNode(ctx, userID, nodeID)
}
