// Package app provides application-level dependency container and initialization.
package app

import (
	"brain2-backend/internal/repository"
	"brain2-backend/internal/service/category"
	"brain2-backend/internal/service/llm"
	"brain2-backend/internal/service/memory"
	"brain2-backend/pkg/config"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/go-chi/chi/v5"
)

// Container holds all application dependencies.
type Container struct {
	Config             *config.Config
	DynamoDBClient     *dynamodb.Client
	EventBridgeClient  *eventbridge.Client
	Repository         repository.Repository
	MemoryService      memory.Service
	CategoryService    category.Service
	LLMService         *llm.Service
	Router             *chi.Mux
	ChiLambda         *chiadapter.ChiLambdaV2
}