package initialization

import (
	"log"
	"time"

	"brain2-backend/internal/config"
	"brain2-backend/internal/infrastructure/observability"

	"go.uber.org/zap"
)

// ObservabilityConfig holds configuration for observability initialization
type ObservabilityConfig struct {
	Config   *config.Config
	AppName  string
	Version  string
}

// ObservabilityServices holds all initialized observability services
type ObservabilityServices struct {
	Logger            *zap.Logger
	MetricsCollector  *observability.Collector
	TracingEnabled    bool
	TracerProvider    interface{} // Placeholder for tracer provider
}

// InitializeObservability sets up logging, metrics, and tracing
func InitializeObservability(config ObservabilityConfig) (*ObservabilityServices, error) {
	log.Println("Initializing observability...")
	startTime := time.Now()

	// Initialize structured logging
	logger, err := initializeLogger(config.Config)
	if err != nil {
		return nil, err
	}

	// Initialize metrics collection
	metricsCollector := observability.NewCollector(config.AppName)

	services := &ObservabilityServices{
		Logger:           logger,
		MetricsCollector: metricsCollector,
		TracingEnabled:   config.Config.Tracing.Enabled,
	}

	// Initialize tracing if enabled
	if config.Config.Tracing.Enabled {
		if err := initializeTracing(config, services); err != nil {
			log.Printf("Failed to initialize tracing: %v", err)
			// Don't fail startup, just log the error
		}
	}

	log.Printf("Observability initialized in %v", time.Since(startTime))
	return services, nil
}

// initializeLogger sets up structured logging with zap
func initializeLogger(config *config.Config) (*zap.Logger, error) {
	var zapConfig zap.Config

	if config.Environment == "production" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
	}

	// Set log level based on configuration
	switch config.Logging.Level {
	case "debug":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// initializeTracing sets up distributed tracing
func initializeTracing(_ ObservabilityConfig, services *ObservabilityServices) error {
	log.Println("Initializing tracing...")
	
	// Placeholder for actual tracing initialization
	// This would typically set up OpenTelemetry or similar
	services.TracerProvider = nil
	
	log.Println("Tracing initialized (placeholder)")
	return nil
}