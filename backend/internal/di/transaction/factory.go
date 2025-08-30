package transaction

import (
	"brain2-backend/internal/repository"
)

// transactionalRepositoryFactory implements repository.TransactionalRepositoryFactory
// with proper transaction support
type transactionalRepositoryFactory struct {
	nodeRepo     repository.NodeRepository
	edgeRepo     repository.EdgeRepository
	categoryRepo repository.CategoryRepository
	transaction  repository.Transaction
}

// NewTransactionalRepositoryFactory creates a new transactional repository factory
func NewTransactionalRepositoryFactory(
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	categoryRepo repository.CategoryRepository,
) repository.TransactionalRepositoryFactory {
	return &transactionalRepositoryFactory{
		nodeRepo:     nodeRepo,
		edgeRepo:     edgeRepo,
		categoryRepo: categoryRepo,
	}
}

func (f *transactionalRepositoryFactory) WithTransaction(tx repository.Transaction) repository.TransactionalRepositoryFactory {
	f.transaction = tx
	return f
}

func (f *transactionalRepositoryFactory) CreateNodeRepository(tx repository.Transaction) repository.NodeRepository {
	// If we have a transaction, wrap the repository
	if tx != nil {
		return NewTransactionalNodeWrapper(f.nodeRepo, tx)
	}
	return f.nodeRepo
}

func (f *transactionalRepositoryFactory) CreateEdgeRepository(tx repository.Transaction) repository.EdgeRepository {
	if tx != nil {
		return NewTransactionalEdgeWrapper(f.edgeRepo, tx)
	}
	return f.edgeRepo
}

func (f *transactionalRepositoryFactory) CreateCategoryRepository(tx repository.Transaction) repository.CategoryRepository {
	if tx != nil {
		return NewTransactionalCategoryWrapper(f.categoryRepo, tx)
	}
	return f.categoryRepo
}

func (f *transactionalRepositoryFactory) CreateKeywordRepository(tx repository.Transaction) repository.KeywordRepository {
	// Return nil as we don't have a keyword repository yet
	return nil
}

func (f *transactionalRepositoryFactory) CreateGraphRepository(tx repository.Transaction) repository.GraphRepository {
	// Return nil as we don't have a graph repository for transactions yet
	return nil
}

func (f *transactionalRepositoryFactory) CreateNodeCategoryRepository(tx repository.Transaction) repository.NodeCategoryRepository {
	// Return nil as we'll use the existing mock
	return nil
}