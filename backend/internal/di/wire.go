//go:build wireinject
// +build wireinject

// Package di provides dependency injection configuration using Wire.
package di

import (
	"context"
	"log"

	infraDynamoDB "brain2-backend/infrastructure/dynamodb"
	"brain2-backend/internal/app"
	"brain2-backend/internal/repository"
	"brain2-backend/internal/service/category"
	"brain2-backend/internal/service/llm"
	"brain2-backend/internal/service/memory"
	"brain2-backend/pkg/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/go-chi/chi/v5"
	"github.com/google/wire"
)

// ProviderSets define the dependency injection sets.
var (
	// ConfigSet provides configuration-related dependencies
	ConfigSet = wire.NewSet(
		ProvideConfig,
	)

	// AWSSet provides AWS service clients
	AWSSet = wire.NewSet(
		ProvideAWSConfig,
		ProvideDynamoDBClient,
		ProvideEventBridgeClient,
	)

	// RepositorySet provides repository implementations
	RepositorySet = wire.NewSet(
		ProvideRepository,
	)

	// ServiceSet provides business logic services
	ServiceSet = wire.NewSet(
		ProvideMemoryService,
		ProvideLLMService,
		ProvideCategoryService,
	)

	// HTTPSet provides HTTP-related dependencies
	HTTPSet = wire.NewSet(
		ProvideRouter,
		ProvideChiLambda,
	)

	// AllSet combines all provider sets
	AllSet = wire.NewSet(
		ConfigSet,
		AWSSet,
		RepositorySet,
		ServiceSet,
		HTTPSet,
	)
)

// Provider functions for dependency injection

// ProvideConfig creates and returns the application configuration.
func ProvideConfig() *config.Config {
	return config.New()
}

// ProvideAWSConfig loads the AWS configuration.
func ProvideAWSConfig(cfg *config.Config) (aws.Config, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.Region))
	if err != nil {
		log.Printf("unable to load SDK config: %v", err)
		return aws.Config{}, err
	}
	return awsCfg, nil
}

// ProvideDynamoDBClient creates and returns a DynamoDB client.
func ProvideDynamoDBClient(awsCfg aws.Config) *dynamodb.Client {
	return dynamodb.NewFromConfig(awsCfg)
}

// ProvideEventBridgeClient creates and returns an EventBridge client.
func ProvideEventBridgeClient(awsCfg aws.Config) *eventbridge.Client {
	return eventbridge.NewFromConfig(awsCfg)
}

// ProvideRepository creates and returns a repository implementation.
func ProvideRepository(client *dynamodb.Client, cfg *config.Config) repository.Repository {
	return infraDynamoDB.NewRepository(client, cfg.TableName, cfg.KeywordIndexName)
}

// ProvideMemoryService creates and returns a memory service.
func ProvideMemoryService(repo repository.Repository) memory.Service {
	return memory.NewService(repo)
}

// ProvideLLMService creates and returns an LLM service with mock provider.
func ProvideLLMService() *llm.Service {
	return llm.NewService(llm.NewMockProvider())
}

// ProvideCategoryService creates and returns an enhanced category service.
func ProvideCategoryService(repo repository.Repository, llmSvc *llm.Service) category.Service {
	return category.NewEnhancedService(repo, llmSvc)
}

// ProvideRouter creates and configures the HTTP router with all routes and middleware.
func ProvideRouter(
	memorySvc memory.Service,
	categorySvc category.Service,
	eventBridgeClient *eventbridge.Client,
) *chi.Mux {
	return SetupRouter(memorySvc, categorySvc, eventBridgeClient)
}

// ProvideChiLambda creates a Chi Lambda adapter.
func ProvideChiLambda(router *chi.Mux) *chiadapter.ChiLambdaV2 {
	return chiadapter.NewV2(router)
}

// InitializeContainer wires together all dependencies and returns a complete Container.
func InitializeContainer() (*app.Container, error) {
	wire.Build(
		AllSet,
		wire.Struct(new(app.Container), "*"),
	)
	return nil, nil
}