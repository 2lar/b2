// Package services contains application services that orchestrate use cases.
// This CategoryService demonstrates the Application Service pattern with CQRS command handlers.
//
// Key Concepts Illustrated:
//   - Application Service Pattern: Orchestrates category-related business operations
//   - Command/Query Responsibility Segregation (CQRS): Uses command handlers for writes
//   - Command Handler Pattern: Delegates operations to specialized handlers
//   - AI Service Integration: Optional AI service with graceful fallback
//   - Domain Event Publishing: Communicates changes to other parts of the system
package services

import (
	"context"

	"brain2-backend/internal/application/commands"
	"brain2-backend/internal/application/dto"
)


// CategoryService implements the Application Service pattern for category operations
// using CQRS command handlers for write operations.
type CategoryService struct {
	// CQRS command handler for category operations
	commandHandler *commands.CategoryCommandHandler
}

// NewCategoryService creates a new CategoryService with all required dependencies.
// The AI service is optional and the service will work without it.
func NewCategoryService(
	commandHandler *commands.CategoryCommandHandler,
) *CategoryService {
	return &CategoryService{
		commandHandler: commandHandler,
	}
}

// CreateCategory implements the use case for creating a new category.
func (s *CategoryService) CreateCategory(ctx context.Context, cmd *commands.CreateCategoryCommand) (*dto.CreateCategoryResult, error) {
	// Delegate to command handler for CQRS pattern
	return s.commandHandler.HandleCreateCategory(ctx, cmd)
}

// UpdateCategory implements the use case for updating an existing category.
func (s *CategoryService) UpdateCategory(ctx context.Context, cmd *commands.UpdateCategoryCommand) (*dto.UpdateCategoryResult, error) {
	// Delegate to command handler for CQRS pattern
	return s.commandHandler.HandleUpdateCategory(ctx, cmd)
}

// DeleteCategory implements the use case for deleting a category.
func (s *CategoryService) DeleteCategory(ctx context.Context, cmd *commands.DeleteCategoryCommand) (*dto.DeleteCategoryResult, error) {
	// Delegate to command handler for CQRS pattern
	return s.commandHandler.HandleDeleteCategory(ctx, cmd)
}


// IsAIServiceAvailable checks if the AI service is available for use.
// This method demonstrates the fallback pattern - always check availability before using AI.
func (s *CategoryService) IsAIServiceAvailable() bool {
	return false // AI service integration removed
}

// GetAIServiceStatus returns information about the AI service status.
func (s *CategoryService) GetAIServiceStatus() string {
	return "AI service not configured - using domain-based fallback"
}