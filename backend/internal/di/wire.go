//go:build wireinject

package di

import (
	"context"
	"net/http"

	"brain2-backend/internal/config"
	"brain2-backend/internal/handlers"
	"brain2-backend/internal/repository"
	categoryService "brain2-backend/internal/service/category"
	memoryService "brain2-backend/internal/service/memory"
	"brain2-backend/infrastructure/dynamodb"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/go-chi/chi/v5"
	"github.com/google/wire"
	"brain2-backend/pkg/api"
)

// ProvideDynamoDBClient provides a DynamoDB client.
func ProvideDynamoDBClient() (*awsDynamodb.Client, error) {
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	return awsDynamodb.NewFromConfig(cfg), nil
}

// ProvideRepository provides the DynamoDB repository implementation.
func ProvideRepository(client *awsDynamodb.Client, cfg config.Config) repository.Repository {
	return dynamodb.NewRepository(client, cfg.TableName, cfg.IndexName)
}

// ProvideMemoryService provides the memory service.
func ProvideMemoryService(repo repository.Repository) memoryService.Service {
	return memoryService.NewService(repo)
}

// ProvideCategoryService provides the category service.
func ProvideCategoryService(repo repository.Repository) categoryService.Service {
	return categoryService.NewService(repo)
}

// ProvideEventBridgeClient provides an EventBridge client.
func ProvideEventBridgeClient() (*awsEventbridge.Client, error) {
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}
	return awsEventbridge.NewFromConfig(cfg), nil
}

// ProvideMemoryHandler provides the memory handler.
func ProvideMemoryHandler(svc memoryService.Service, eventBridgeClient *awsEventbridge.Client) *handlers.MemoryHandler {
	return handlers.NewMemoryHandler(svc, eventBridgeClient)
}

// ProvideCategoryHandler provides the category handler.
func ProvideCategoryHandler(svc categoryService.Service) *handlers.CategoryHandler {
	return handlers.NewCategoryHandler(svc)
}

// ProvideRouter provides the HTTP router with all handlers.
func ProvideRouter(memoryHandler *handlers.MemoryHandler, categoryHandler *handlers.CategoryHandler) *chi.Mux {
	r := chi.NewRouter()

	// Public routes
	r.Group(func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			api.Success(w, http.StatusOK, map[string]string{"status": "ok"})
		})
	})

	// Protected routes (example, assuming auth middleware is applied elsewhere)
	r.Group(func(r chi.Router) {
		// r.Use(authMiddleware) // Assuming an auth middleware is applied here

		r.Post("/api/nodes", memoryHandler.CreateNode)
		r.Get("/api/nodes", memoryHandler.ListNodes)
		r.Get("/api/nodes/{nodeId}", memoryHandler.GetNode)
		r.Put("/api/nodes/{nodeId}", memoryHandler.UpdateNode)
		r.Delete("/api/nodes/{nodeId}", memoryHandler.DeleteNode)
		r.Post("/api/nodes/bulk-delete", memoryHandler.BulkDeleteNodes)
		r.Get("/api/graph-data", memoryHandler.GetGraphData)

		r.Post("/api/categories", categoryHandler.CreateCategory)
		r.Get("/api/categories", categoryHandler.ListCategories)
		r.Get("/api/categories/{categoryId}", categoryHandler.GetCategory)
		r.Put("/api/categories/{categoryId}", categoryHandler.UpdateCategory)
		r.Delete("/api/categories/{categoryId}", categoryHandler.DeleteCategory)
		r.Post("/api/categories/{categoryId}/memories", categoryHandler.AddMemoryToCategory)
		r.Get("/api/categories/{categoryId}/memories", categoryHandler.GetMemoriesInCategory)
		r.Delete("/api/categories/{categoryId}/memories/{memoryId}", categoryHandler.RemoveMemoryFromCategory)
	})

	return r
}

// ProvideChiLambda provides the ChiLambdaV2 adapter.
func ProvideChiLambda(router *chi.Mux) *chiadapter.ChiLambdaV2 {
	return chiadapter.NewV2(router)
}

var ( 
	DynamoDBSet = wire.NewSet(
		ProvideDynamoDBClient,
		config.LoadConfig,
		ProvideRepository,
	)

	ServiceSet = wire.NewSet(
		ProvideMemoryService,
		ProvideCategoryService,
	)

	HandlerSet = wire.NewSet(
		ProvideMemoryHandler,
		ProvideCategoryHandler,
		ProvideEventBridgeClient,
	)
)

func InitializeAPI() (*chi.Mux, error) { // Changed return type to *chi.Mux
	wire.Build(
		DynamoDBSet,
		ServiceSet,
		HandlerSet,
		ProvideRouter,
		// ProvideChiLambda, // Removed from here
	)
	return nil, nil
}
