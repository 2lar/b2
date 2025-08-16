// Package transactions provides transaction management for DynamoDB operations.
package transactions

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"brain2-backend/internal/repository"
)

// DynamoDBTransactionProvider implements TransactionProvider using DynamoDB transactions
type DynamoDBTransactionProvider struct {
	client *dynamodb.Client
	mu     sync.Mutex
}

// NewDynamoDBTransactionProvider creates a new DynamoDB transaction provider
func NewDynamoDBTransactionProvider(client *dynamodb.Client) repository.TransactionProvider {
	return &DynamoDBTransactionProvider{
		client: client,
	}
}

// BeginTransaction starts a new transaction
func (p *DynamoDBTransactionProvider) BeginTransaction(ctx context.Context) (repository.Transaction, error) {
	return &DynamoDBTransaction{
		client:    p.client,
		items:     make([]types.TransactWriteItem, 0),
		ctx:       ctx,
		committed: false,
	}, nil
}

// DynamoDBTransaction implements Transaction interface for DynamoDB
type DynamoDBTransaction struct {
	client    *dynamodb.Client
	items     []types.TransactWriteItem
	ctx       context.Context
	mu        sync.Mutex
	committed bool
	rolledBack bool
}

// Commit commits the transaction
func (t *DynamoDBTransaction) Commit() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.committed {
		return fmt.Errorf("transaction already committed")
	}
	
	if t.rolledBack {
		return fmt.Errorf("transaction already rolled back")
	}
	
	if len(t.items) == 0 {
		t.committed = true
		return nil
	}
	
	// Execute the transaction using the stored context
	_, err := t.client.TransactWriteItems(t.ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: t.items,
	})
	
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	t.committed = true
	return nil
}

// Rollback rolls back the transaction
func (t *DynamoDBTransaction) Rollback() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.committed {
		return fmt.Errorf("cannot rollback committed transaction")
	}
	
	// DynamoDB transactions auto-rollback on failure
	// We just need to clear the items and mark as rolled back
	t.items = nil
	t.rolledBack = true
	return nil
}

// AddWriteItem adds a write item to the transaction
func (t *DynamoDBTransaction) AddWriteItem(item types.TransactWriteItem) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.committed {
		return fmt.Errorf("cannot add items to committed transaction")
	}
	
	if t.rolledBack {
		return fmt.Errorf("cannot add items to rolled back transaction")
	}
	
	// DynamoDB has a limit of 100 items per transaction
	if len(t.items) >= 100 {
		return fmt.Errorf("transaction item limit reached (100 items)")
	}
	
	t.items = append(t.items, item)
	return nil
}

// GetContext returns the transaction context
func (t *DynamoDBTransaction) GetContext() context.Context {
	return t.ctx
}

// IsActive returns whether the transaction is still active
func (t *DynamoDBTransaction) IsActive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.committed && !t.rolledBack
}

// GetItemCount returns the number of items in the transaction
func (t *DynamoDBTransaction) GetItemCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.items)
}

// TransactionManager manages multiple transactions
type TransactionManager struct {
	provider     repository.TransactionProvider
	transactions map[string]repository.Transaction
	mu           sync.RWMutex
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(provider repository.TransactionProvider) *TransactionManager {
	return &TransactionManager{
		provider:     provider,
		transactions: make(map[string]repository.Transaction),
	}
}

// BeginTransaction starts a new named transaction
func (m *TransactionManager) BeginTransaction(ctx context.Context, name string) (repository.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.transactions[name]; exists {
		return nil, fmt.Errorf("transaction %s already exists", name)
	}
	
	tx, err := m.provider.BeginTransaction(ctx)
	if err != nil {
		return nil, err
	}
	
	m.transactions[name] = tx
	return tx, nil
}

// GetTransaction retrieves an existing transaction by name
func (m *TransactionManager) GetTransaction(name string) (repository.Transaction, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	tx, exists := m.transactions[name]
	return tx, exists
}

// CommitTransaction commits a named transaction
func (m *TransactionManager) CommitTransaction(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	tx, exists := m.transactions[name]
	if !exists {
		return fmt.Errorf("transaction %s not found", name)
	}
	
	if err := tx.Commit(); err != nil {
		return err
	}
	
	delete(m.transactions, name)
	return nil
}

// RollbackTransaction rolls back a named transaction
func (m *TransactionManager) RollbackTransaction(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	tx, exists := m.transactions[name]
	if !exists {
		return fmt.Errorf("transaction %s not found", name)
	}
	
	if err := tx.Rollback(); err != nil {
		return err
	}
	
	delete(m.transactions, name)
	return nil
}

// CleanupInactiveTransactions removes all inactive transactions
func (m *TransactionManager) CleanupInactiveTransactions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for name, tx := range m.transactions {
		if dynamoTx, ok := tx.(*DynamoDBTransaction); ok {
			if !dynamoTx.IsActive() {
				delete(m.transactions, name)
			}
		}
	}
}