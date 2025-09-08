package rest

import (
	"net/http"
	"strings"

	"backend2/application/commands/bus"
	querybus "backend2/application/queries/bus"
	"backend2/interfaces/http/rest/handlers"
	"backend2/interfaces/http/rest/middleware"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
)

// Router creates and configures the HTTP router
type Router struct {
	commandBus *bus.CommandBus
	queryBus   *querybus.QueryBus
	logger     *zap.Logger
}

// NewRouter creates a new router instance
func NewRouter(
	commandBus *bus.CommandBus,
	queryBus *querybus.QueryBus,
	logger *zap.Logger,
) *Router {
	return &Router{
		commandBus: commandBus,
		queryBus:   queryBus,
		logger:     logger,
	}
}

// Setup configures all routes and middleware
func (rt *Router) Setup() http.Handler {
	router := chi.NewRouter()
	
	// Global middleware
	router.Use(chimiddleware.RequestID)
	router.Use(chimiddleware.RealIP)
	router.Use(chimiddleware.Recoverer)
	router.Use(middleware.Logger(rt.logger))
	router.Use(versionMiddleware)
	
	// CORS configuration
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "https://*.brain2.com"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	
	// Health check
	router.Get("/health", rt.healthCheck)
	router.Get("/ready", rt.readinessCheck)
	
	// API v1 routes (legacy - redirects to v2)
	router.Route("/api/v1", func(r chi.Router) {
		r.HandleFunc("/*", func(w http.ResponseWriter, req *http.Request) {
			// Redirect v1 requests to v2
			http.Redirect(w, req, strings.Replace(req.URL.Path, "/api/v1", "/api/v2", 1), http.StatusPermanentRedirect)
		})
	})

	// API v2 routes (current)
	router.Route("/api/v2", func(r chi.Router) {
		// Apply authentication middleware for API routes
		r.Use(middleware.Authenticate())
		
		// Node endpoints
		r.Route("/nodes", func(r chi.Router) {
			nodeHandler := handlers.NewNodeHandler(rt.commandBus, rt.queryBus, rt.logger)
			r.Post("/", nodeHandler.CreateNode)
			r.Get("/{nodeID}", nodeHandler.GetNode)
			r.Put("/{nodeID}", nodeHandler.UpdateNode)
			r.Delete("/{nodeID}", nodeHandler.DeleteNode)
			r.Get("/", nodeHandler.ListNodes)
			r.Post("/bulk-delete", nodeHandler.BulkDeleteNodes)
			
			// Category endpoints for nodes (stub)
			categoryHandler := handlers.NewCategoryHandler(rt.logger)
			r.Get("/{nodeID}/categories", categoryHandler.GetNodeCategories)
			r.Post("/{nodeID}/categories", categoryHandler.CategorizeNode)
		})
		
		// Graph endpoints
		r.Route("/graphs", func(r chi.Router) {
			graphHandler := handlers.NewGraphHandler(rt.queryBus, rt.logger)
			r.Get("/{graphID}", graphHandler.GetGraph)
			r.Get("/", graphHandler.ListGraphs)
		})
		
		// Edge endpoints
		r.Route("/edges", func(r chi.Router) {
			edgeHandler := handlers.NewEdgeHandler(rt.commandBus, rt.logger)
			r.Post("/", edgeHandler.CreateEdge)
			r.Delete("/{edgeID}", edgeHandler.DeleteEdge)
		})
		
		// Category endpoints (stub)
		r.Route("/categories", func(r chi.Router) {
			categoryHandler := handlers.NewCategoryHandler(rt.logger)
			r.Get("/", categoryHandler.ListCategories)
			r.Post("/rebuild", categoryHandler.RebuildCategories)
			r.Get("/suggest", categoryHandler.SuggestCategories)
		})
		
		// Search endpoint
		r.Get("/search", handlers.NewSearchHandler(rt.queryBus, rt.logger).Search)
		
		// Graph data endpoint for visualization
		r.Get("/graph-data", handlers.NewGraphHandler(rt.queryBus, rt.logger).GetGraphData)
	})
	
	return router
}

// healthCheck handles health check requests
func (rt *Router) healthCheck(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

// readinessCheck handles readiness check requests
func (rt *Router) readinessCheck(w http.ResponseWriter, req *http.Request) {
	// Check dependencies (database, etc.)
	// For now, always return ready
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

// versionMiddleware adds API version headers to all responses
func versionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Determine API version from path
		version := "v2" // default
		if strings.Contains(r.URL.Path, "/api/v1") {
			version = "v1"
		} else if strings.Contains(r.URL.Path, "/api/v2") {
			version = "v2"
		}
		
		// Add version headers
		w.Header().Set("X-API-Version", version)
		w.Header().Set("X-API-Latest", "v2")
		w.Header().Set("X-API-Deprecated", "false")
		
		// For v1, add deprecation notice
		if version == "v1" {
			w.Header().Set("X-API-Deprecated", "true")
			w.Header().Set("X-API-Deprecation-Date", "2024-06-01")
			w.Header().Set("X-API-Sunset-Date", "2024-12-01")
		}
		
		next.ServeHTTP(w, r)
	})
}