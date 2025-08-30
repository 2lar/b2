package initialization

import (
	"log"
	"net/http"
	"time"

	"brain2-backend/internal/config"
	middleware "brain2-backend/internal/interfaces/http/v1/middleware"

	"go.uber.org/zap"
)

// MiddlewareConfig holds configuration for middleware initialization
type MiddlewareConfig struct {
	Config        *config.Config
	Logger        *zap.Logger
	ColdStartTime *time.Time
}

// MiddlewareServices holds all initialized middleware services
type MiddlewareServices struct {
	Pipeline *middleware.Pipeline
	Config   map[string]any
}

// InitializeMiddleware sets up all HTTP middleware components using the pipeline pattern
func InitializeMiddleware(config MiddlewareConfig) *MiddlewareServices {
	log.Println("Initializing middleware...")
	startTime := time.Now()

	// Create middleware pipeline
	pipeline := middleware.BuildAPIMiddleware(config.Logger)

	services := &MiddlewareServices{
		Pipeline: pipeline,
		Config:   make(map[string]any),
	}

	// Store configuration for monitoring
	services.Config["cors"] = config.Config.CORS
	services.Config["security"] = config.Config.Security
	services.Config["rateLimit"] = config.Config.RateLimit

	// Add cold start information if available
	if config.ColdStartTime != nil {
		services.Config["coldStart"] = map[string]interface{}{
			"timestamp": *config.ColdStartTime,
			"duration":  time.Since(*config.ColdStartTime),
		}
	}

	log.Printf("Middleware initialized in %v", time.Since(startTime))
	return services
}

// BuildHandler builds an HTTP handler with the middleware pipeline
func (m *MiddlewareServices) BuildHandler(handler http.HandlerFunc) http.HandlerFunc {
	return m.Pipeline.Build(handler)
}