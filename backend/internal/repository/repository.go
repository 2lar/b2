package repository

// Repository is the main repository interface that composes all segregated interfaces.
// This maintains backward compatibility while providing access to all repository operations.
type Repository interface {
	NodeRepository
	EdgeRepository
	KeywordRepository
	TransactionalRepository
	CategoryRepository
	GraphRepository
}
