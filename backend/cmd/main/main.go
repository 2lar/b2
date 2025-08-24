// Package main provides the AWS Lambda entry point for the Brain2 backend API.
//
// This Lambda function serves as the main HTTP API gateway for the Brain2 application,
// handling all REST API requests through a single Lambda function. It demonstrates
// advanced Lambda patterns including:
//   - Cold start optimization for better performance
//   - Dependency injection container management
//   - Request/response transformation via Chi router
//   - Comprehensive logging and monitoring
//   - Graceful error handling and recovery
//
// # Lambda Architecture
//
// The application uses a "Lambda-lith" pattern - a single Lambda function that handles
// multiple HTTP routes rather than one Lambda per endpoint. This approach provides:
//   - Reduced cold start frequency
//   - Shared connection pools and caches
//   - Simplified deployment and monitoring
//   - Better cost optimization for moderate traffic
//
// # Cold Start Optimization
//
// Several techniques are employed to minimize cold start impact:
//   - Connection pool reuse across invocations
//   - Lazy initialization of expensive resources
//   - Dependency injection container caching
//   - Pre-compilation of common objects
//   - Cold start duration monitoring and alerting
package main

import (
	"context"
	"log"
	"time"

	"brain2-backend/internal/di"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
)

// Global variables for Lambda lifecycle management
var (
	// chiLambda wraps the Chi router for AWS Lambda integration
	// This adapter handles the conversion between APIGatewayV2HTTPRequest/Response
	// and standard Go http.Request/ResponseWriter interfaces
	chiLambda *chiadapter.ChiLambdaV2
	
	// container holds the dependency injection container with all application services
	// Initialized once during cold start and reused across warm invocations
	container *di.Container
	
	// coldStart tracks whether this is a cold start invocation
	// Used for performance monitoring and optimization decisions
	coldStart = true
	
	// coldStartTime records when the cold start began
	// Used for measuring initialization performance and identifying slow starts
	coldStartTime time.Time
)

// init runs during Lambda cold start before the first invocation.
// This is where we perform expensive initialization that can be shared
// across multiple invocations, including:
//   - Dependency injection container setup
//   - Database connection pool creation
//   - AWS SDK client initialization
//   - Configuration loading and validation
func init() {
	if coldStart {
		coldStartTime = time.Now()
		log.Println("Cold start detected - starting Lambda function initialization...")
	}
	
	initStart := time.Now()
	
	// Initialize the dependency injection container
	// This creates and wires all application services, repositories, and handlers
	// The container pattern ensures proper dependency management and lifecycle
	var err error
	container, err = di.InitializeContainer()
	if err != nil {
		log.Fatalf("Failed to initialize DI container: %v", err)
	}

	// Validate that all dependencies are properly initialized
	// This catches configuration errors and missing dependencies early
	if err := container.Validate(); err != nil {
		log.Fatalf("Container validation failed: %v", err)
	}

	// Provide cold start context to other components for optimization decisions
	// Components can adjust behavior during cold starts (e.g., skip cache warming)
	container.SetColdStartInfo(coldStartTime, coldStart)
	
	// Extract the configured Chi router from the container
	// The router contains all HTTP routes and middleware configured via DI
	router := container.GetRouter()
	
	// Create the Lambda adapter that bridges Chi router and AWS Lambda runtime
	// This adapter handles AWS-specific request/response format conversion
	chiLambda = chiadapter.NewV2(router)
	
	initDuration := time.Since(initStart)
	
	if coldStart {
		totalColdStartDuration := time.Since(coldStartTime)
		log.Printf("Cold start completed: initialization took %v (total cold start: %v)", initDuration, totalColdStartDuration)
		
		// Log warning for slow cold starts
		if totalColdStartDuration > 15*time.Second {
			log.Printf("WARNING: Cold start took %v, which may cause timeout issues", totalColdStartDuration)
		}
		
		coldStart = false // Mark that we're no longer in cold start
		container.IsColdStart = false // Update container state too
	} else {
		log.Printf("Service initialized successfully with centralized DI container in %v", initDuration)
		
		// Log initialization warning if it took too long
		if initDuration > 10*time.Second {
			log.Printf("WARNING: Initialization took %v, which may cause cold start timeouts", initDuration)
		}
	}
}

// main is the Lambda function entry point.
// It handles the Lambda runtime integration and coordinates request processing.
// This function demonstrates proper Lambda lifecycle management including:
//   - Graceful resource cleanup on shutdown
//   - Request processing with monitoring
//   - Performance tracking and alerting
//   - Error handling and recovery
func main() {
	// Register cleanup handler to ensure graceful resource deallocation
	// This is crucial for proper connection pool cleanup and metric flushing
	defer func() {
		if container != nil {
			ctx := context.Background()
			if err := container.Shutdown(ctx); err != nil {
				log.Printf("Error during container shutdown: %v", err)
			}
		}
	}()

	// Start the Lambda runtime with our request handler function
	// The handler processes each incoming API Gateway request
	lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		requestStart := time.Now()
		
		// Track post-cold-start performance characteristics
		// Requests immediately after cold start may have different performance profiles
		// due to JIT compilation, cache warming, and connection pool initialization
		timeSinceColdStart := time.Since(coldStartTime)
		isPostColdStartRequest := timeSinceColdStart < 30*time.Second && !coldStart
		
		// Log request details with cold start context for monitoring and debugging
		if isPostColdStartRequest {
			log.Printf("Processing POST-COLD-START request (%v after cold start): %s %s", 
				timeSinceColdStart, req.RequestContext.HTTP.Method, req.RequestContext.HTTP.Path)
		} else {
			log.Printf("Processing request: %s %s", req.RequestContext.HTTP.Method, req.RequestContext.HTTP.Path)
		}
		
		// Process the request through the Chi router via the Lambda adapter
		// This converts API Gateway events to standard HTTP requests and back
		response, err := chiLambda.ProxyWithContextV2(ctx, req)
		
		// Add observability headers to help with debugging and monitoring
		// These headers indicate Lambda performance characteristics to clients
		if isPostColdStartRequest {
			if response.Headers == nil {
				response.Headers = make(map[string]string)
			}
			response.Headers["X-Cold-Start"] = "true"
			response.Headers["X-Cold-Start-Age"] = timeSinceColdStart.String()
		}
		
		duration := time.Since(requestStart)
		
		// Log request completion with performance context
		// Different logging for post-cold-start requests helps identify performance patterns
		if isPostColdStartRequest {
			log.Printf("Post-cold-start request completed in %v: %s %s -> %d (cold start age: %v)", 
				duration, 
				req.RequestContext.HTTP.Method, 
				req.RequestContext.HTTP.Path,
				response.StatusCode,
				timeSinceColdStart)
		} else {
			log.Printf("Request completed in %v: %s %s -> %d", 
				duration, 
				req.RequestContext.HTTP.Method, 
				req.RequestContext.HTTP.Path,
				response.StatusCode)
		}
		
		// Performance monitoring with graduated alerting thresholds
		// These logs help identify performance degradation and trigger alerts
		// Thresholds are tuned for API Gateway timeout limits and user expectations
		if duration > 10*time.Second {
			log.Printf("CRITICAL: VERY SLOW REQUEST: %s %s took %v (status: %d, request_id: %s)", 
				req.RequestContext.HTTP.Method, 
				req.RequestContext.HTTP.Path, 
				duration,
				response.StatusCode,
				req.RequestContext.RequestID)
		} else if duration > 5*time.Second {
			log.Printf("WARNING: SLOW REQUEST: %s %s took %v (status: %d, request_id: %s)", 
				req.RequestContext.HTTP.Method, 
				req.RequestContext.HTTP.Path, 
				duration,
				response.StatusCode,
				req.RequestContext.RequestID)
		} else if duration > 2*time.Second {
			log.Printf("NOTICE: Request taking longer than expected: %s %s took %v (status: %d)", 
				req.RequestContext.HTTP.Method, 
				req.RequestContext.HTTP.Path, 
				duration,
				response.StatusCode)
		}
		
		return response, err
	})
}
