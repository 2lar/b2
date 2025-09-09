package main

import (
	"context"
	"log"
	"time"

	"backend/domain/core/valueobjects"
	"backend/infrastructure/config"
	"backend/infrastructure/di"
	"backend/interfaces/http/rest"

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

	// Initialize context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

	// Pre-warm DynamoDB connection by executing a simple query
	// This reduces latency on first real request
	if container != nil && container.NodeRepo != nil {
		go func() {
			warmCtx, warmCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer warmCancel()
			// Simple ping to establish connection pool
			_, _ = container.NodeRepo.GetByID(warmCtx, valueobjects.NewNodeID())
		}()
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
	// Log ALL headers and authorizer context for debugging
	if container != nil && container.Logger != nil {
		container.Logger.Info("Lambda received request",
			zap.String("path", req.RequestContext.HTTP.Path),
			zap.String("method", req.RequestContext.HTTP.Method),
			zap.Any("headers", req.Headers),
			zap.String("request_id", req.RequestContext.RequestID),
			zap.Any("authorizer", req.RequestContext.Authorizer),
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

		// Extract user context from API Gateway authorizer
		// The authorizer context is available at req.RequestContext.Authorizer
		if req.RequestContext.Authorizer != nil && req.RequestContext.Authorizer.Lambda != nil {
			// Extract user information from Lambda authorizer context
			// The JWT authorizer returns: sub (user ID), email, and role
			lambdaClaims := req.RequestContext.Authorizer.Lambda

			if userID, ok := lambdaClaims["sub"].(string); ok && userID != "" {
				req.Headers["X-User-ID"] = userID
			}
			if email, ok := lambdaClaims["email"].(string); ok && email != "" {
				req.Headers["X-User-Email"] = email
			}
			if role, ok := lambdaClaims["role"].(string); ok && role != "" {
				req.Headers["X-User-Roles"] = role
			}

			// Set bypass headers to indicate pre-authorized request
			delete(req.Headers, "authorization")
			delete(req.Headers, "Authorization")
			req.Headers["Authorization"] = "Bearer api-gateway-validated"
			req.Headers["X-API-Gateway-Authorized"] = "true"

			if container != nil && container.Logger != nil {
				container.Logger.Info("Extracted user context from API Gateway authorizer",
					zap.String("user_id", req.Headers["X-User-ID"]),
					zap.String("email", req.Headers["X-User-Email"]),
					zap.String("roles", req.Headers["X-User-Roles"]),
					zap.String("path", req.RequestContext.HTTP.Path),
					zap.Any("authorizer_context", req.RequestContext.Authorizer.Lambda),
				)
			}
		} else {
			// No authorizer context - this shouldn't happen with API Gateway JWT authorizer
			if container != nil && container.Logger != nil {
				container.Logger.Warn("No authorizer context found in request",
					zap.String("path", req.RequestContext.HTTP.Path),
					zap.Bool("has_auth_header", hasAuth),
					zap.Any("authorizer", req.RequestContext.Authorizer),
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
