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

func init() {
	router, err := di.InitializeAPI()
	if err != nil {
		log.Fatalf("Failed to initialize API: %v", err)
	}
	chiLambda = chiadapter.NewV2(router)
	log.Println("Service initialized successfully with Wire DI")
}

func main() {
	lambda.Start(func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		return chiLambda.ProxyWithContextV2(ctx, req)
	})
}
