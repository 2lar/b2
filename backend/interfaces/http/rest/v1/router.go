package v1

import (
	"net/http"

	"backend/interfaces/http/rest/handlers"
	"backend/interfaces/http/rest/middleware"

	"github.com/go-chi/chi/v5"
)

// NewRouter creates the v1 API router
func NewRouter(
	nodeHandler *handlers.NodeHandler,
	graphHandler *handlers.GraphHandler,
	edgeHandler *handlers.EdgeHandler,
	searchHandler *handlers.SearchHandler,
) chi.Router {
	router := chi.NewRouter()
	
	// Create v1 subrouter
	router.Route("/api/v1", func(r chi.Router) {
		// Apply middleware
		r.Use(middleware.Logging())
		r.Use(middleware.CORS())
		r.Use(middleware.RequestID())
		r.Use(middleware.Authenticate())
		r.Use(versionHeaders)

		// Node endpoints
		r.Post("/nodes", nodeHandler.CreateNode)
		r.Get("/nodes", nodeHandler.ListNodes)
		r.Get("/nodes/{id}", nodeHandler.GetNode)
		r.Put("/nodes/{id}", nodeHandler.UpdateNode)
		r.Delete("/nodes/{id}", nodeHandler.DeleteNode)
		r.Post("/nodes/bulk-delete", nodeHandler.BulkDeleteNodes)
		// TODO: Implement node connection endpoints
		// r.Post("/nodes/{id}/connect", nodeHandler.ConnectNodes)
		// r.Post("/nodes/{id}/disconnect", nodeHandler.DisconnectNodes)

		// Graph endpoints
		// TODO: Implement graph CRUD operations
		// r.Post("/graphs", graphHandler.CreateGraph)
		r.Get("/graphs", graphHandler.ListGraphs)
		r.Get("/graphs/{id}", graphHandler.GetGraph)
		// r.Put("/graphs/{id}", graphHandler.UpdateGraph)
		// r.Delete("/graphs/{id}", graphHandler.DeleteGraph)
		// r.Get("/graphs/{id}/nodes", graphHandler.GetGraphNodes)
		// r.Get("/graphs/{id}/edges", graphHandler.GetGraphEdges)
		r.Get("/graphs/{id}/data", graphHandler.GetGraphData)

		// Edge endpoints
		r.Post("/edges", edgeHandler.CreateEdge)
		// TODO: Implement edge read and update operations
		// r.Get("/edges/{id}", edgeHandler.GetEdge)
		// r.Put("/edges/{id}", edgeHandler.UpdateEdge)
		r.Delete("/edges/{id}", edgeHandler.DeleteEdge)

		// Search endpoints
		r.Post("/search", searchHandler.Search)
		// TODO: Implement additional search endpoints
		// r.Get("/search/nodes", searchHandler.SearchNodes)
		// r.Post("/search/similar", searchHandler.FindSimilarNodes)

		// Health check
		r.Get("/health", healthCheck)
	})

	return router
}

// versionHeaders adds API version headers to responses
func versionHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-API-Version", "v1")
		w.Header().Set("X-API-Deprecated", "false")
		next.ServeHTTP(w, r)
	})
}

// healthCheck provides a health check endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","version":"v1"}`))
}
