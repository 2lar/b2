package v1

import (
	"net/http"

	"backend2/interfaces/http/rest/handlers"
	"backend2/interfaces/http/rest/middleware"
	"github.com/gorilla/mux"
)

// NewRouter creates the v1 API router
func NewRouter(
	nodeHandler *handlers.NodeHandler,
	graphHandler *handlers.GraphHandler,
	edgeHandler *handlers.EdgeHandler,
	searchHandler *handlers.SearchHandler,
) *mux.Router {
	router := mux.NewRouter()
	v1 := router.PathPrefix("/api/v1").Subrouter()

	// Apply middleware
	v1.Use(middleware.Logging())
	v1.Use(middleware.CORS())
	v1.Use(middleware.RequestID())
	v1.Use(middleware.Authenticate())

	// Node endpoints
	v1.HandleFunc("/nodes", nodeHandler.CreateNode).Methods("POST")
	v1.HandleFunc("/nodes", nodeHandler.ListNodes).Methods("GET")
	v1.HandleFunc("/nodes/{id}", nodeHandler.GetNode).Methods("GET")
	v1.HandleFunc("/nodes/{id}", nodeHandler.UpdateNode).Methods("PUT")
	v1.HandleFunc("/nodes/{id}", nodeHandler.DeleteNode).Methods("DELETE")
	v1.HandleFunc("/nodes/bulk-delete", nodeHandler.BulkDeleteNodes).Methods("POST")
	v1.HandleFunc("/nodes/{id}/connect", nodeHandler.ConnectNodes).Methods("POST")
	v1.HandleFunc("/nodes/{id}/disconnect", nodeHandler.DisconnectNodes).Methods("POST")

	// Graph endpoints
	v1.HandleFunc("/graphs", graphHandler.CreateGraph).Methods("POST")
	v1.HandleFunc("/graphs", graphHandler.ListGraphs).Methods("GET")
	v1.HandleFunc("/graphs/{id}", graphHandler.GetGraph).Methods("GET")
	v1.HandleFunc("/graphs/{id}", graphHandler.UpdateGraph).Methods("PUT")
	v1.HandleFunc("/graphs/{id}", graphHandler.DeleteGraph).Methods("DELETE")
	v1.HandleFunc("/graphs/{id}/nodes", graphHandler.GetGraphNodes).Methods("GET")
	v1.HandleFunc("/graphs/{id}/edges", graphHandler.GetGraphEdges).Methods("GET")
	v1.HandleFunc("/graphs/{id}/data", graphHandler.GetGraphData).Methods("GET")

	// Edge endpoints
	v1.HandleFunc("/edges", edgeHandler.CreateEdge).Methods("POST")
	v1.HandleFunc("/edges/{id}", edgeHandler.GetEdge).Methods("GET")
	v1.HandleFunc("/edges/{id}", edgeHandler.UpdateEdge).Methods("PUT")
	v1.HandleFunc("/edges/{id}", edgeHandler.DeleteEdge).Methods("DELETE")

	// Search endpoints
	v1.HandleFunc("/search", searchHandler.Search).Methods("POST")
	v1.HandleFunc("/search/nodes", searchHandler.SearchNodes).Methods("GET")
	v1.HandleFunc("/search/similar", searchHandler.FindSimilarNodes).Methods("POST")

	// Health check
	v1.HandleFunc("/health", healthCheck).Methods("GET")

	// Add version header middleware
	v1.Use(versionHeaders)

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