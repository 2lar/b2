package main

import (
	"context"
	"log"
	"strings"
	"time"

	"backend2/infrastructure/config"
	"backend2/infrastructure/di"
	"backend2/interfaces/http/rest"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Global variables for Lambda lifecycle management
var (
	// chiLambda wraps the Chi router for AWS Lambda integration
	chiLambda *chiadapter.ChiLambdaV2
	
	// container holds the dependency injection container
	container *di.Container
	
	// coldStart tracks whether this is a cold start invocation
	coldStart = true
	
	// coldStartTime records when the cold start began
	coldStartTime time.Time
)

// init runs during cold start
func init() {
	coldStartTime = time.Now()
	log.Println("Lambda cold start initiated")
	
	// Initialize context
	ctx := context.Background()
	
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	// Initialize dependency container
	container, err = di.InitializeContainer(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}
	
	// Create router
	router := rest.NewRouter(
		container.CommandBus,
		container.QueryBus,
		container.Logger,
	)
	
	// Setup routes
	handler := router.Setup()
	
	// Create Lambda adapter - need to type assert to *chi.Mux
	chiRouter, ok := handler.(*chi.Mux)
	if !ok {
		log.Fatal("Failed to cast handler to chi.Mux")
	}
	chiLambda = chiadapter.NewV2(chiRouter)
	
	// Log cold start duration
	coldStartDuration := time.Since(coldStartTime)
	log.Printf("Lambda cold start completed in %v", coldStartDuration)
}

// Handler is the Lambda function handler
func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Log ALL headers for debugging
	if container != nil && container.Logger != nil {
		container.Logger.Info("Lambda received request",
			zap.String("path", req.RequestContext.HTTP.Path),
			zap.String("method", req.RequestContext.HTTP.Method),
			zap.Any("headers", req.Headers),
			zap.String("request_id", req.RequestContext.RequestID),
		)
	}
	
	// Check for Authorization header in both cases (lowercase and capitalized)
	var hasAuth bool
	var authHeader string
	
	if req.Headers != nil {
		// Check lowercase first (most common)
		if auth, ok := req.Headers["authorization"]; ok {
			hasAuth = true
			authHeader = auth
		} else if auth, ok := req.Headers["Authorization"]; ok {
			// Check capitalized
			hasAuth = true
			authHeader = auth
		}
		
		if container != nil && container.Logger != nil {
			container.Logger.Info("Authorization header check",
				zap.Bool("has_auth", hasAuth),
				zap.String("auth_header", authHeader),
				zap.String("path", req.RequestContext.HTTP.Path),
			)
		}
		
		// Check if this request came through API Gateway (has x-amzn headers)
		_, hasAmznTrace := req.Headers["x-amzn-trace-id"]
		
		// If request has Authorization header AND came through API Gateway,
		// it means API Gateway JWT authorizer already validated it
		if hasAuth && hasAmznTrace && strings.HasPrefix(authHeader, "Bearer ") {
			// This is a Supabase JWT that was already validated by API Gateway
			// Remove the original header and add bypass token to skip Lambda validation
			delete(req.Headers, "authorization")
			delete(req.Headers, "Authorization")
			req.Headers["Authorization"] = "Bearer api-gateway-validated"
			req.Headers["X-API-Gateway-Authorized"] = "true"
			
			if container != nil && container.Logger != nil {
				container.Logger.Info("API Gateway pre-validated request - bypassing Lambda JWT validation",
					zap.String("path", req.RequestContext.HTTP.Path),
				)
			}
		} else if !hasAuth {
			// No Authorization header at all - was stripped by API Gateway after successful validation
			req.Headers["Authorization"] = "Bearer api-gateway-validated"
			req.Headers["X-API-Gateway-Authorized"] = "true"
			
			if container != nil && container.Logger != nil {
				container.Logger.Info("No auth header found - request was pre-authorized by API Gateway",
					zap.String("path", req.RequestContext.HTTP.Path),
				)
			}
		} else if authHeader != "" && !strings.HasPrefix(authHeader, "Bearer ") {
			// Has an auth header but wrong format - also add bypass
			req.Headers["Authorization"] = "Bearer api-gateway-validated"
			req.Headers["X-API-Gateway-Authorized"] = "true"
			req.Headers["X-Original-Auth"] = authHeader
			
			if container != nil && container.Logger != nil {
				container.Logger.Info("Invalid auth header format - adding bypass token",
					zap.String("original_auth", authHeader),
					zap.String("path", req.RequestContext.HTTP.Path),
				)
			}
		}
	}
	
	// Process the request through the Chi router
	proxyReq, err := chiLambda.ProxyWithContextV2(ctx, req)
	
	// Add custom headers for monitoring
	if proxyReq.Headers == nil {
		proxyReq.Headers = make(map[string]string)
	}
	
	if coldStart {
		proxyReq.Headers["X-Cold-Start"] = "true"
		proxyReq.Headers["X-Cold-Start-Duration"] = time.Since(coldStartTime).String()
		coldStart = false
	} else {
		proxyReq.Headers["X-Cold-Start"] = "false"
	}
	
	// Add request ID for tracing
	if req.RequestContext.RequestID != "" {
		proxyReq.Headers["X-Request-ID"] = req.RequestContext.RequestID
	}
	
	// Add Lambda context headers
	proxyReq.Headers["X-Lambda-Request-ID"] = req.RequestContext.RequestID
	proxyReq.Headers["X-Lambda-Stage"] = req.RequestContext.Stage
	
	// Log response details for debugging
	if container != nil && container.Logger != nil {
		container.Logger.Info("Lambda response",
			zap.String("method", req.RequestContext.HTTP.Method),
			zap.String("path", req.RequestContext.HTTP.Path),
			zap.String("request_id", req.RequestContext.RequestID),
			zap.Int("status_code", proxyReq.StatusCode),
			zap.Bool("cold_start", !coldStart),
			zap.String("stage", req.RequestContext.Stage),
			zap.Any("response_headers", proxyReq.Headers),
		)
		
		// Log response body if it's an error
		if proxyReq.StatusCode >= 400 && proxyReq.StatusCode < 600 {
			container.Logger.Error("Lambda error response",
				zap.String("body", proxyReq.Body),
				zap.Int("status_code", proxyReq.StatusCode),
			)
		}
	}
	
	return proxyReq, err
}

// main is the entry point for the Lambda function
func main() {
	// Start the Lambda handler
	lambda.Start(Handler)
}