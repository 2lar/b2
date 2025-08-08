// Brain2 Main HTTP API - RESTful Memory Management Server
package main

import (
	"context"
	"log"

	"brain2-backend/internal/app"
	"brain2-backend/internal/di"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var container *app.Container

func init() {
	var err error
	container, err = di.InitializeContainer()
	if err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}
	
	log.Println("Service initialized successfully with Wire DI")
}

func main() {
	lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		return container.ChiLambda.ProxyWithContextV2(ctx, req)
	})
}