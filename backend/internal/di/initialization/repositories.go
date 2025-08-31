package initialization

import (
	"log"
	"time"

	domainServices "brain2-backend/internal/domain/services"
	"brain2-backend/internal/domain/shared"
	"brain2-backend/internal/infrastructure/persistence"
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

	// Initialize store for compatibility with legacy query services
	storeConfig := persistence.StoreConfig{
		TableName: config.TableName,
		IndexNames: map[string]string{
			"GSI1": config.IndexName,
		},
	}
	store := persistence.NewDynamoDBStore(config.DynamoDBClient, storeConfig, config.Logger)

	// Initialize repositories
	nodeRepo := infradynamodb.NewNodeRepository(config.DynamoDBClient, config.TableName, config.IndexName, config.Logger)
	edgeRepo := infradynamodb.NewEdgeRepository(config.DynamoDBClient, config.TableName, config.IndexName, config.Logger)
	categoryRepo := infradynamodb.NewCategoryRepository(config.DynamoDBClient, config.TableName, config.IndexName, config.Logger)
	
	// Create graph repository for unified access
	graphRepo := infradynamodb.NewGraphRepository(config.DynamoDBClient, config.TableName, config.IndexName, config.Logger)
	
	// Create keyword repository for keyword-based search
	keywordRepo := infradynamodb.NewKeywordRepository(config.DynamoDBClient, config.TableName, config.IndexName)
	
	// Create transactional repository for complex transactional operations
	transactionalRepo := infradynamodb.NewTransactionalRepository(config.DynamoDBClient, config.TableName, config.IndexName, config.Logger)
	
	// Create node-category repository (stub for now until full implementation)
	nodeCategoryRepo := infradynamodb.NewNodeCategoryRepositoryStub()

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
		Store:                   store,
		NodeRepository:          nodeRepo,
		EdgeRepository:          edgeRepo, 
		CategoryRepository:      categoryRepo,
		GraphRepository:         graphRepo,
		KeywordRepository:       keywordRepo,
		TransactionalRepository: transactionalRepo,
		NodeCategoryRepository:  nodeCategoryRepo,
		ConnectionAnalyzer:      connectionAnalyzer,
		IdempotencyStore:        idempotencyStore,
		UnitOfWorkFactory:       unitOfWorkFactory,
	}

	log.Printf("Repository layer initialized in %v", time.Since(startTime))
	return services, nil
}

// RepositoryServices holds all initialized repository-layer services
type RepositoryServices struct {
	Store                   persistence.Store
	NodeRepository          repository.NodeRepository
	EdgeRepository          repository.EdgeRepository
	CategoryRepository      repository.CategoryRepository
	GraphRepository         repository.GraphRepository
	KeywordRepository       repository.KeywordRepository
	TransactionalRepository repository.TransactionalRepository
	NodeCategoryRepository  repository.NodeCategoryRepository
	ConnectionAnalyzer      *domainServices.ConnectionAnalyzer
	IdempotencyStore        repository.IdempotencyStore
	UnitOfWorkFactory       repository.UnitOfWorkFactory
}

// Note: SafeGetNodeReader/Writer, SafeGetEdgeReader/Writer, SafeGetCategoryReader/Writer removed
// Use NodeRepository, EdgeRepository, CategoryRepository directly

// InitializeCQRSServices sets up CQRS-related services
func (s *RepositoryServices) InitializeCQRSServices() {
	log.Println("Initializing CQRS services...")
	
	// For future CQRS implementation
	// This matches the existing pattern in container.go
}