package initialization

import (
	"log"
	"time"

	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/domain/shared"
	infradynamodb "brain2-backend/internal/infrastructure/persistence/dynamodb"
	"brain2-backend/internal/repository"

	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.uber.org/zap"
)

// RepositoryConfig holds configuration for repository initialization
type RepositoryConfig struct {
	TableName       string
	IndexName       string
	DynamoDBClient  *awsDynamodb.Client
	Logger          *zap.Logger
	EventBus        shared.EventBus
	EnableCaching   bool
}

// InitializeRepositoryLayer sets up the repository layer with all required dependencies
func InitializeRepositoryLayer(config RepositoryConfig) (*RepositoryServices, error) {
	log.Println("Initializing repository layer...")
	startTime := time.Now()

	// No store initialization needed - repositories work directly with DynamoDB client

	// Initialize repositories
	nodeRepo := infradynamodb.NewNodeRepository(config.DynamoDBClient, config.TableName, config.IndexName, config.Logger)
	edgeRepo := infradynamodb.NewEdgeRepository(config.DynamoDBClient, config.TableName, config.IndexName, config.Logger)
	categoryRepo := infradynamodb.NewCategoryRepository(config.DynamoDBClient, config.TableName, config.IndexName, config.Logger)
	
	// Create graph repository for unified access
	graphRepo := infradynamodb.NewGraphRepository(config.DynamoDBClient, config.TableName, config.IndexName, config.Logger)

	// Initialize domain services
	connectionAnalyzer := domainServices.NewConnectionAnalyzer(0.3, 5, 0.2)
	idempotencyStore := infradynamodb.NewIdempotencyStore(config.DynamoDBClient, config.TableName, 24*time.Hour)
	
	// Initialize DynamoDB event store
	eventStore := infradynamodb.NewDynamoDBEventStore(config.DynamoDBClient, config.TableName)
	
	// Create UnitOfWorkFactory
	unitOfWorkFactory := infradynamodb.NewDynamoDBUnitOfWorkFactory(
		config.DynamoDBClient,
		config.TableName,
		config.IndexName,
		config.EventBus,
		eventStore,
		config.Logger,
	)

	services := &RepositoryServices{
		NodeRepository:     nodeRepo,
		EdgeRepository:     edgeRepo, 
		CategoryRepository: categoryRepo,
		GraphRepository:    graphRepo,
		ConnectionAnalyzer: connectionAnalyzer,
		IdempotencyStore:   idempotencyStore,
		UnitOfWorkFactory:  unitOfWorkFactory,
	}

	log.Printf("Repository layer initialized in %v", time.Since(startTime))
	return services, nil
}

// RepositoryServices holds all initialized repository-layer services
type RepositoryServices struct {
	NodeRepository     repository.NodeRepository
	EdgeRepository     repository.EdgeRepository
	CategoryRepository repository.CategoryRepository
	GraphRepository    repository.GraphRepository
	ConnectionAnalyzer *domainServices.ConnectionAnalyzer
	IdempotencyStore   repository.IdempotencyStore
	UnitOfWorkFactory  repository.UnitOfWorkFactory
}

// safeGetNodeReader safely converts NodeRepository to NodeReader interface
func (s *RepositoryServices) SafeGetNodeReader() repository.NodeReader {
	if reader, ok := s.NodeRepository.(repository.NodeReader); ok {
		return reader
	}
	return nil
}

// safeGetEdgeReader safely converts EdgeRepository to EdgeReader interface
func (s *RepositoryServices) SafeGetEdgeReader() repository.EdgeReader {
	if reader, ok := s.EdgeRepository.(repository.EdgeReader); ok {
		return reader
	}
	return nil
}

// safeGetCategoryReader safely converts CategoryRepository to CategoryReader interface
func (s *RepositoryServices) SafeGetCategoryReader() repository.CategoryReader {
	if reader, ok := s.CategoryRepository.(repository.CategoryReader); ok {
		return reader
	}
	return nil
}

// safeGetCategoryWriter safely converts CategoryRepository to CategoryWriter interface
func (s *RepositoryServices) SafeGetCategoryWriter() repository.CategoryWriter {
	if writer, ok := s.CategoryRepository.(repository.CategoryWriter); ok {
		return writer
	}
	return nil
}

// safeGetEdgeWriter safely converts EdgeRepository to EdgeWriter interface
func (s *RepositoryServices) SafeGetEdgeWriter() repository.EdgeWriter {
	if writer, ok := s.EdgeRepository.(repository.EdgeWriter); ok {
		return writer
	}
	return nil
}

// InitializeCQRSServices sets up CQRS-related services
func (s *RepositoryServices) InitializeCQRSServices() {
	log.Println("Initializing CQRS services...")
	
	// For future CQRS implementation
	// This matches the existing pattern in container.go
}