package dynamodb

import (
	"context"
	
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/repository"
	
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.uber.org/zap"
)

// DynamoDBUnitOfWorkFactory creates new DynamoDBUnitOfWork instances.
// This factory ensures that each request gets its own isolated UnitOfWork,
// preventing state corruption in serverless environments.
type DynamoDBUnitOfWorkFactory struct {
	client    *dynamodb.Client
	tableName string
	indexName string
	eventBus  shared.EventBus
	logger    *zap.Logger
}

// NewDynamoDBUnitOfWorkFactory creates a new factory for DynamoDBUnitOfWork instances.
func NewDynamoDBUnitOfWorkFactory(
	client *dynamodb.Client,
	tableName, indexName string,
	eventBus shared.EventBus,
	logger *zap.Logger,
) repository.UnitOfWorkFactory {
	return &DynamoDBUnitOfWorkFactory{
		client:    client,
		tableName: tableName,
		indexName: indexName,
		eventBus:  eventBus,
		logger:    logger,
	}
}

// Create returns a new DynamoDBUnitOfWork instance with fresh state.
// Each call creates a completely new instance, ensuring no state is shared
// between requests in Lambda warm containers.
func (f *DynamoDBUnitOfWorkFactory) Create(ctx context.Context) (repository.UnitOfWork, error) {
	// Create a new UnitOfWork instance with fresh state
	uow := NewDynamoDBUnitOfWork(
		f.client,
		f.tableName,
		f.indexName,
		f.eventBus,
		f.logger,
	)
	
	// Log creation for debugging
	f.logger.Debug("Created new UnitOfWork instance",
		zap.String("table", f.tableName),
		zap.String("index", f.indexName),
	)
	
	return uow, nil
}