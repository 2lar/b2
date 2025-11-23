package rest

import (
	"net/http"

	"backend/application/mediator"
	"backend/interfaces/http/rest/handlers"
	"backend/interfaces/http/rest/middleware"
	"backend/pkg/errors"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
)

// Router creates and configures the HTTP router
type Router struct {
	mediator       mediator.IMediator
	logger         *zap.Logger
	errorHandler   *errors.ErrorHandler
	authMiddleware func(http.Handler) http.Handler
}

// NewRouter creates a new router instance
func NewRouter(
	med mediator.IMediator,
	logger *zap.Logger,
	errorHandler *errors.ErrorHandler,
	authMiddleware func(http.Handler) http.Handler,
) *Router {
	// Defnsive check: Ensure auth middleware is never nil to avoid panics
	if authMiddleware == nil {
		authMiddleware = func(next http.Handler) http.Handler { return next }
	}
	return &Router{
		mediator:       med,
		logger:         logger,
		errorHandler:   errorHandler,
		authMiddleware: authMiddleware,
	}
}

// Setup configures all routes and middleware
func (rt *Router) Setup() http.Handler {
	// 1. Initialize Handlers ONCE at startup (Optimization)
	// This prevents re-creating structs on every request or router rebuild
	nodeHandler := handlers.NewNodeHandler(rt.mediator, rt.logger, rt.errorHandler)
	graphHandler := handlers.NewGraphHandler(rt.mediator, rt.logger, rt.errorHandler)
	edgeHandler := handlers.NewEdgeHandler(rt.mediator, rt.logger, rt.errorHandler)
	categoryHandler := handlers.NewCategoryHandler(rt.logger)
	searchHandler := handlers.NewSearchHandler(rt.mediator, rt.logger, rt.errorHandler)
	operationHandler := handlers.NewOperationHandler(rt.mediator, rt.logger)

	router := chi.NewRouter()

	// 2. Global Middleware (Applies to EVERYTHING)
	router.Use(chimiddleware.RequestID)
	// RealIP is critical for AWS Fargate behind NLB to see the actual user IP
	router.Use(chimiddleware.RealIP)
	router.Use(chimiddleware.Recoverer)
	router.Use(middleware.Logger(rt.logger))

	// 3. CORS Configuration
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "https://*.brain2.com"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// 4. Infrastructure Routes (No Auth required)
	router.Get("/health", rt.healthCheck)
	router.Get("/ready", rt.readinessCheck)

	// 5. API v1 Routes
	router.Route("/api/v1", func(r chi.Router) {
		// Scoped Middleware: Only apply v1 versioning headers here
		r.Use(versionMiddleware)
		// Apply authentication middleware for all API routes
		r.Use(rt.authMiddleware)

		// Node endpoints
		r.Route("/nodes", func(r chi.Router) {
			r.Post("/", nodeHandler.CreateNode)
			r.Get("/{nodeID}", nodeHandler.GetNode)
			r.Put("/{nodeID}", nodeHandler.UpdateNode)
			r.Delete("/{nodeID}", nodeHandler.DeleteNode)
			r.Get("/", nodeHandler.ListNodes)
			r.Post("/bulk-delete", nodeHandler.BulkDeleteNodes)

			r.Get("/{nodeID}/categories", categoryHandler.GetNodeCategories)
			r.Post("/{nodeID}/categories", categoryHandler.CategorizeNode)
		})

		// Graph endpoints
		r.Route("/graphs", func(r chi.Router) {
			r.Get("/{graphID}", graphHandler.GetGraph)
			r.Get("/{graphID}/stats", graphHandler.GetGraphStats)
			r.Get("/", graphHandler.ListGraphs)
		})

		// Edge endpoints
		r.Route("/edges", func(r chi.Router) {
			r.Post("/", edgeHandler.CreateEdge)
			r.Delete("/{edgeID}", edgeHandler.DeleteEdge)
		})

		// Category endpoints
		r.Route("/categories", func(r chi.Router) {
			r.Get("/", categoryHandler.ListCategories)
			r.Post("/rebuild", categoryHandler.RebuildCategories)
			r.Get("/suggest", categoryHandler.SuggestCategories)
		})

		// Search endpoint
		r.Get("/search", searchHandler.Search)

		// Graph data endpoint for visualization
		r.Get("/graph-data", graphHandler.GetGraphData)

		// Operation status endpoint
		r.Route("/operations", func(r chi.Router) {
			r.Get("/{operationID}", operationHandler.GetOperationStatus)
		})
	})

	return router
}

// healthCheck handles liveness checks (Is the binary running?)
func (rt *Router) healthCheck(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

// readinessCheck handles readiness checks (Can we take traffic?)
func (rt *Router) readinessCheck(w http.ResponseWriter, req *http.Request) {
	// CRITICAL: Actually check if the database/dependencies are connected.
	// If the database is down, we must return 503 so the Load Balancer
	// stops sending us traffic.
	ctx := req.Context()
	
	// Assuming your mediator has a Health/Ping method. 
	// If not, you should add one to the IMediator interface.
	if err := rt.mediator.CheckHealth(ctx); err != nil {
		rt.logger.Error("Readiness check failed", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not_ready", "error": "dependency_failure"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

// versionMiddleware adds API version headers to responses in this router scope
func versionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version := "v1"

		w.Header().Set("X-API-Version", version)
		w.Header().Set("X-API-Latest", "v2")

		// Deprecation logic specific to v1
		w.Header().Set("X-API-Deprecated", "true")
		w.Header().Set("X-API-Deprecation-Date", "2024-06-01")
		w.Header().Set("X-API-Sunset-Date", "2024-12-01")

		next.ServeHTTP(w, r)
	})
}
