package main

import (
	"context"
	"log"

	"brain2-backend/internal/di"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
)

var chiLambda *chiadapter.ChiLambdaV2
var container *di.Container

func init() {
	var err error
	container, err = di.InitializeContainer()
	if err != nil {
		log.Fatalf("Failed to initialize DI container: %v", err)
	}

	// Validate all dependencies are properly initialized
	if err := container.Validate(); err != nil {
		log.Fatalf("Container validation failed: %v", err)
	}

	// Get the router from the container
	router := container.GetRouter()
	chiLambda = chiadapter.NewV2(router)
	
	log.Println("Service initialized successfully with centralized DI container")
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
		return chiLambda.ProxyWithContextV2(ctx, req)
	})
}
