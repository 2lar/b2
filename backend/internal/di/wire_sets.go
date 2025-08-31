// Package di provides Wire provider sets.
// This file contains the provider sets used by Wire to generate dependency injection code.
package di

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/wire"
)

// CleanSuperSet is the provider set using the clean container architecture.
// This provides all dependencies through focused sub-containers.
var CleanSuperSet = wire.NewSet(
	ConfigProviders,
	CleanContainerProviders,
	
	// Extractors for specific types needed by Wire
	provideRouterFromApplicationContainer,
	provideDynamoDBClientFromInfrastructure,
	provideEventBridgeClientFromInfrastructure,
	
	wire.Bind(new(http.Handler), new(*chi.Mux)), // Bind router as http.Handler
)

// ConfigProviders provides configuration-related dependencies.
// These are the foundation that other layers depend upon.
var ConfigProviders = wire.NewSet(
	provideConfig,
	provideLogger,
	provideEnvironment,
	provideContext,
)