package di

import (
	"context"
	"fmt"
	"time"

	"backend/application/ports"
	"backend/application/queries"
	query_handlers "backend/application/queries/handlers"
	querybus "backend/application/queries/bus"
	"backend/infrastructure/persistence/memory"
	"go.uber.org/zap"
)

// ProvideOperationStore creates an operation store for async operation tracking
func ProvideOperationStore() ports.OperationStore {
	// Use in-memory store with 1 hour TTL
	// In production, you might want to use Redis or DynamoDB
	return memory.NewInMemoryOperationStore(1 * time.Hour)
}

// RegisterOperationQueries registers operation-related query handlers
func RegisterOperationQueries(queryBus *querybus.QueryBus, operationStore ports.OperationStore, logger *zap.Logger) {
	// Register GetOperationStatusQuery handler
	operationStatusHandler := query_handlers.NewGetOperationStatusHandler(operationStore, logger)
	
	queryBus.Register(queries.GetOperationStatusQuery{}, &QueryHandlerAdapter{
		handler: func(ctx context.Context, query querybus.Query) (interface{}, error) {
			statusQuery, ok := query.(queries.GetOperationStatusQuery)
			if !ok {
				return nil, fmt.Errorf("invalid query type")
			}
			return operationStatusHandler.Handle(ctx, statusQuery)
		},
	})
}