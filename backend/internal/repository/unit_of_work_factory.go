package repository

import (
	"context"
)

// UnitOfWorkFactory creates new UnitOfWork instances for each request.
// This pattern ensures that each request gets its own isolated transaction context,
// preventing state corruption in serverless environments like AWS Lambda.
type UnitOfWorkFactory interface {
	// Create returns a new UnitOfWork instance for a single request/transaction.
	// Each call creates a fresh instance with clean state.
	Create(ctx context.Context) (UnitOfWork, error)
}