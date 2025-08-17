package di

import (
	"net/http"
	"time"

	"brain2-backend/pkg/api"
	"brain2-backend/internal/handlers"
	"brain2-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// setupRouter provides the HTTP router with all handlers.
// This is used internally by the container system.
func setupRouter(memoryHandler *handlers.MemoryHandler, categoryHandler *handlers.CategoryHandler) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware - applied to all routes
	r.Use(middleware.RequestID)           // Generate/extract request IDs
	r.Use(middleware.Recovery)            // Handle panics gracefully
	r.Use(middleware.Timeout(30 * time.Second)) // 30 second timeout for all requests

	// Public routes
	r.Group(func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			api.Success(w, http.StatusOK, map[string]string{"status": "ok"})
		})
	})

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Apply circuit breaker for API routes (protects against cascading failures)
		apiCircuitBreaker := middleware.CircuitBreaker(
			middleware.DefaultCircuitBreakerConfig("api-routes"))
		r.Use(apiCircuitBreaker)

		r.Use(handlers.Authenticator) // Apply authentication middleware

		// Node endpoints
		r.Route("/nodes", func(r chi.Router) {
			r.Post("/", memoryHandler.CreateNode)
			r.Get("/", memoryHandler.ListNodes)
			r.Get("/{nodeId}", memoryHandler.GetNode)
			r.Put("/{nodeId}", memoryHandler.UpdateNode)
			r.Delete("/{nodeId}", memoryHandler.DeleteNode)
			r.Post("/bulk-delete", memoryHandler.BulkDeleteNodes)
			
			// Node categorization routes
			r.Get("/{nodeId}/categories", categoryHandler.GetNodeCategories)
			r.Post("/{nodeId}/categories", categoryHandler.CategorizeNode)
		})

		// Category endpoints
		r.Route("/categories", func(r chi.Router) {
			r.Post("/", categoryHandler.CreateCategory)
			r.Get("/", categoryHandler.ListCategories)
			r.Get("/{categoryId}", categoryHandler.GetCategory)
			r.Put("/{categoryId}", categoryHandler.UpdateCategory)
			r.Delete("/{categoryId}", categoryHandler.DeleteCategory)
			r.Post("/{categoryId}/nodes", categoryHandler.AssignNodeToCategory)
			r.Get("/{categoryId}/nodes", categoryHandler.GetNodesInCategory)
			r.Delete("/{categoryId}/nodes/{nodeId}", categoryHandler.RemoveNodeFromCategory)
		})

		// Graph data endpoint
		r.Get("/graph-data", memoryHandler.GetGraphData)
	})

	return r
}

