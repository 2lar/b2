package di

import (
	"net/http"

	"brain2-backend/pkg/api"
	"brain2-backend/internal/handlers"
	"github.com/go-chi/chi/v5"
)

// setupRouter provides the HTTP router with all handlers.
// This is used internally by the container system.
func setupRouter(memoryHandler *handlers.MemoryHandler, categoryHandler *handlers.CategoryHandler) *chi.Mux {
	r := chi.NewRouter()

	// Public routes
	r.Group(func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			api.Success(w, http.StatusOK, map[string]string{"status": "ok"})
		})
	})

	// Protected routes (require authentication)
	r.Group(func(r chi.Router) {
		r.Use(handlers.Authenticator) // Apply authentication middleware

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
		r.Post("/api/categories/{categoryId}/nodes", categoryHandler.AssignNodeToCategory)
		r.Get("/api/categories/{categoryId}/nodes", categoryHandler.GetNodesInCategory)
		r.Delete("/api/categories/{categoryId}/nodes/{nodeId}", categoryHandler.RemoveNodeFromCategory)

		// Node categorization routes
		r.Get("/api/nodes/{nodeId}/categories", categoryHandler.GetNodeCategories)
		r.Post("/api/nodes/{nodeId}/categories", categoryHandler.CategorizeNode)
	})

	return r
}

