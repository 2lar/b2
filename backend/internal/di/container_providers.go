//go:build !wireinject
// +build !wireinject

// Package di provides Wire providers for the new clean container architecture.
// This file contains providers that create the focused containers replacing the God Container.
package di

import (
	"brain2-backend/internal/config"
	
	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/go-chi/chi/v5"
	"github.com/google/wire"
	"go.uber.org/zap"
)

// ============================================================================
// CONTAINER PROVIDER SETS
// ============================================================================

// CleanContainerProviders provides all clean container providers.
var CleanContainerProviders = wire.NewSet(
	provideInfrastructureContainer,
	provideRepositoryContainer,
	provideServiceContainer,
	provideHandlerContainer,
	provideApplicationContainer,
)

// ============================================================================
// INFRASTRUCTURE CONTAINER PROVIDERS
// ============================================================================

// provideInfrastructureContainer creates a fully initialized infrastructure container.
func provideInfrastructureContainer(cfg *config.Config) (*InfrastructureContainer, error) {
	return NewInfrastructureContainer(cfg)
}

// ============================================================================
// REPOSITORY CONTAINER PROVIDERS  
// ============================================================================

// provideRepositoryContainer creates a fully initialized repository container.
func provideRepositoryContainer(infra *InfrastructureContainer) (*RepositoryContainer, error) {
	return NewRepositoryContainer(infra)
}

// ============================================================================
// SERVICE CONTAINER PROVIDERS
// ============================================================================

// provideServiceContainer creates a fully initialized service container.
func provideServiceContainer(repos *RepositoryContainer, infra *InfrastructureContainer) (*ServiceContainer, error) {
	return NewServiceContainer(repos, infra)
}

// ============================================================================
// HANDLER CONTAINER PROVIDERS
// ============================================================================

// provideHandlerContainer creates a fully initialized handler container.
func provideHandlerContainer(services *ServiceContainer, infra *InfrastructureContainer) (*HandlerContainer, error) {
	return NewHandlerContainer(services, infra)
}

// ============================================================================
// APPLICATION CONTAINER PROVIDERS
// ============================================================================

// provideApplicationContainer creates the root application container.
func provideApplicationContainer(cfg *config.Config) (*ApplicationContainer, error) {
	return NewApplicationContainer(cfg)
}

// ============================================================================
// BACKWARD COMPATIBILITY PROVIDERS
// ============================================================================

// These providers maintain backward compatibility with existing Wire setup
// by extracting components from the clean containers.

// provideRouterFromApplicationContainer extracts the router from the application container.
func provideRouterFromApplicationContainer(app *ApplicationContainer) *chi.Mux {
	if router, ok := app.Handlers.Router.(*chi.Mux); ok {
		return router
	}
	return nil
}

// provideLoggerFromInfrastructure extracts logger from infrastructure container.
func provideLoggerFromInfrastructure(infra *InfrastructureContainer) *zap.Logger {
	return infra.Logger
}

// provideConfigFromInfrastructure extracts config from infrastructure container.
func provideConfigFromInfrastructure(infra *InfrastructureContainer) *config.Config {
	return infra.Config
}

// provideDynamoDBClientFromInfrastructure extracts DynamoDB client.
func provideDynamoDBClientFromInfrastructure(infra *InfrastructureContainer) *awsDynamodb.Client {
	return infra.DynamoDBClient
}

// provideEventBridgeClientFromInfrastructure extracts EventBridge client.
func provideEventBridgeClientFromInfrastructure(infra *InfrastructureContainer) *awsEventbridge.Client {
	return infra.EventBridgeClient
}