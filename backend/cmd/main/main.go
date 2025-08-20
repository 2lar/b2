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

var chiLambda *chiadapter.ChiLambdaV2
var container *di.Container
var coldStart = true
var coldStartTime time.Time

func init() {
	if coldStart {
		coldStartTime = time.Now()
		log.Println("Cold start detected - starting Lambda function initialization...")
	}
	
	initStart := time.Now()
	var err error
	container, err = di.InitializeContainer()
	if err != nil {
		log.Fatalf("Failed to initialize DI container: %v", err)
	}

	// Validate all dependencies are properly initialized
	if err := container.Validate(); err != nil {
		log.Fatalf("Container validation failed: %v", err)
	}

	// Set cold start information in container for other components to use
	container.SetColdStartInfo(coldStartTime, coldStart)
	
	// Get the router from the container
	router := container.GetRouter()
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

func main() {
	// Ensure graceful shutdown of the container when Lambda terminates
	defer func() {
		if container != nil {
			ctx := context.Background()
			if err := container.Shutdown(ctx); err != nil {
				log.Printf("Error during container shutdown: %v", err)
			}
		}
	}()

	lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		requestStart := time.Now()
		
		// Check if this request is happening shortly after cold start
		timeSinceColdStart := time.Since(coldStartTime)
		isPostColdStartRequest := timeSinceColdStart < 30*time.Second && !coldStart
		
		if isPostColdStartRequest {
			log.Printf("Processing POST-COLD-START request (%v after cold start): %s %s", 
				timeSinceColdStart, req.RequestContext.HTTP.Method, req.RequestContext.HTTP.Path)
		} else {
			log.Printf("Processing request: %s %s", req.RequestContext.HTTP.Method, req.RequestContext.HTTP.Path)
		}
		
		response, err := chiLambda.ProxyWithContextV2(ctx, req)
		
		// Add cold start indicator to response headers
		if isPostColdStartRequest {
			if response.Headers == nil {
				response.Headers = make(map[string]string)
			}
			response.Headers["X-Cold-Start"] = "true"
			response.Headers["X-Cold-Start-Age"] = timeSinceColdStart.String()
		}
		
		duration := time.Since(requestStart)
		
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
		
		// Log slow requests for monitoring with thresholds
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
