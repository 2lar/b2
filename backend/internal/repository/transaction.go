package repository

import (
	"context"
	"fmt"
	"time"

	"brain2-backend/internal/domain"
)

// TransactionManager handles complex multi-step operations with rollback support
type TransactionManager struct {
	steps []TransactionStep
}

// TransactionStep represents a single step in a transaction
type TransactionStep struct {
	Name        string
	Execute     func(ctx context.Context) error
	Rollback    func(ctx context.Context) error
	Executed    bool
	RollbackRun bool
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager() *TransactionManager {
	return &TransactionManager{
		steps: make([]TransactionStep, 0),
	}
}

// AddStep adds a step to the transaction
func (tm *TransactionManager) AddStep(name string, execute, rollback func(ctx context.Context) error) {
	tm.steps = append(tm.steps, TransactionStep{
		Name:     name,
		Execute:  execute,
		Rollback: rollback,
	})
}

// Execute runs all transaction steps with automatic rollback on failure
func (tm *TransactionManager) Execute(ctx context.Context) error {
	// Execute all steps
	for i := range tm.steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		step := &tm.steps[i]
		if err := step.Execute(ctx); err != nil {
			// Mark as executed even on failure for rollback purposes
			step.Executed = true
			
			// Rollback all executed steps
			rollbackErr := tm.rollback(ctx, i)
			if rollbackErr != nil {
				return NewTransactionError(
					fmt.Sprintf("step '%s' failed and rollback failed", step.Name),
					fmt.Errorf("original error: %w, rollback error: %v", err, rollbackErr),
				)
			}
			
			return NewTransactionError(
				fmt.Sprintf("step '%s' failed", step.Name),
				err,
			)
		}
		
		step.Executed = true
	}
	
	return nil
}

// rollback executes rollback operations for all executed steps in reverse order
func (tm *TransactionManager) rollback(ctx context.Context, failedStepIndex int) error {
	var rollbackErrors []error
	
	// Rollback in reverse order
	for i := failedStepIndex; i >= 0; i-- {
		step := &tm.steps[i]
		
		if !step.Executed || step.RollbackRun {
			continue
		}
		
		if step.Rollback != nil {
			if err := step.Rollback(ctx); err != nil {
				rollbackErrors = append(rollbackErrors, 
					fmt.Errorf("rollback failed for step '%s': %w", step.Name, err))
			}
		}
		
		step.RollbackRun = true
	}
	
	if len(rollbackErrors) > 0 {
		return fmt.Errorf("multiple rollback errors: %v", rollbackErrors)
	}
	
	return nil
}

// CompensatingTransaction implements the Saga pattern for distributed transactions
type CompensatingTransaction struct {
	actions []CompensatingAction
}

// CompensatingAction represents an action with its compensation
type CompensatingAction struct {
	Name        string
	Action      func(ctx context.Context) (interface{}, error)
	Compensate  func(ctx context.Context, result interface{}) error
	Result      interface{}
	Completed   bool
	Compensated bool
}

// NewCompensatingTransaction creates a new compensating transaction
func NewCompensatingTransaction() *CompensatingTransaction {
	return &CompensatingTransaction{
		actions: make([]CompensatingAction, 0),
	}
}

// AddAction adds an action with its compensation to the transaction
func (ct *CompensatingTransaction) AddAction(
	name string,
	action func(ctx context.Context) (interface{}, error),
	compensate func(ctx context.Context, result interface{}) error,
) {
	ct.actions = append(ct.actions, CompensatingAction{
		Name:       name,
		Action:     action,
		Compensate: compensate,
	})
}

// Execute runs all actions with compensation on failure
func (ct *CompensatingTransaction) Execute(ctx context.Context) error {
	// Execute all actions
	for i := range ct.actions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		action := &ct.actions[i]
		result, err := action.Action(ctx)
		
		if err != nil {
			// Compensate all completed actions
			compensateErr := ct.compensate(ctx, i-1)
			if compensateErr != nil {
				return NewTransactionError(
					fmt.Sprintf("action '%s' failed and compensation failed", action.Name),
					fmt.Errorf("original error: %w, compensation error: %v", err, compensateErr),
				)
			}
			
			return NewTransactionError(
				fmt.Sprintf("action '%s' failed", action.Name),
				err,
			)
		}
		
		action.Result = result
		action.Completed = true
	}
	
	return nil
}

// compensate runs compensation actions for all completed actions in reverse order
func (ct *CompensatingTransaction) compensate(ctx context.Context, lastCompletedIndex int) error {
	var compensationErrors []error
	
	// Compensate in reverse order
	for i := lastCompletedIndex; i >= 0; i-- {
		action := &ct.actions[i]
		
		if !action.Completed || action.Compensated {
			continue
		}
		
		if action.Compensate != nil {
			if err := action.Compensate(ctx, action.Result); err != nil {
				compensationErrors = append(compensationErrors, 
					fmt.Errorf("compensation failed for action '%s': %w", action.Name, err))
			}
		}
		
		action.Compensated = true
	}
	
	if len(compensationErrors) > 0 {
		return fmt.Errorf("multiple compensation errors: %v", compensationErrors)
	}
	
	return nil
}

// BatchOperation represents a batch operation with partial failure handling
type BatchOperation struct {
	items   []BatchItem
	options BatchOptions
}

// BatchItem represents a single item in a batch operation
type BatchItem struct {
	ID        string
	Operation func(ctx context.Context) error
	Completed bool
	Error     error
}

// BatchOptions configures batch operation behavior
type BatchOptions struct {
	MaxConcurrency int           // Maximum number of concurrent operations
	Timeout        time.Duration // Timeout for each operation
	StopOnError    bool          // Whether to stop on first error
	RetryFailures  bool          // Whether to retry failed operations
}

// DefaultBatchOptions returns default batch options
func DefaultBatchOptions() BatchOptions {
	return BatchOptions{
		MaxConcurrency: 10,
		Timeout:        30 * time.Second,
		StopOnError:    false,
		RetryFailures:  true,
	}
}

// NewBatchOperation creates a new batch operation
func NewBatchOperation(options BatchOptions) *BatchOperation {
	return &BatchOperation{
		items:   make([]BatchItem, 0),
		options: options,
	}
}

// AddItem adds an item to the batch
func (bo *BatchOperation) AddItem(id string, operation func(ctx context.Context) error) {
	bo.items = append(bo.items, BatchItem{
		ID:        id,
		Operation: operation,
	})
}

// Execute runs all batch items with partial failure handling
func (bo *BatchOperation) Execute(ctx context.Context) error {
	if len(bo.items) == 0 {
		return nil
	}
	
	// Create a semaphore to limit concurrency
	semaphore := make(chan struct{}, bo.options.MaxConcurrency)
	results := make(chan int, len(bo.items))
	
	// Start workers
	for i := range bo.items {
		go func(index int) {
			defer func() { results <- index }()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Create timeout context
			operationCtx, cancel := context.WithTimeout(ctx, bo.options.Timeout)
			defer cancel()
			
			// Execute operation
			item := &bo.items[index]
			err := item.Operation(operationCtx)
			
			item.Error = err
			item.Completed = err == nil
			
			if err != nil && bo.options.StopOnError {
				// Cancel parent context on error if StopOnError is true
				if ctxCancel := ctx.Value("cancel"); ctxCancel != nil {
					if cancel, ok := ctxCancel.(context.CancelFunc); ok {
						cancel()
					}
				}
			}
		}(i)
	}
	
	// Wait for all operations to complete
	for i := 0; i < len(bo.items); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-results:
			// Operation completed
		}
	}
	
	// Check results
	var errors []error
	successCount := 0
	
	for _, item := range bo.items {
		if item.Error != nil {
			errors = append(errors, fmt.Errorf("item '%s': %w", item.ID, item.Error))
		} else {
			successCount++
		}
	}
	
	if len(errors) > 0 {
		return NewRepositoryErrorWithDetails(
			ErrCodeOperationFailed,
			fmt.Sprintf("batch operation failed: %d/%d items succeeded", successCount, len(bo.items)),
			map[string]interface{}{
				"total_items":    len(bo.items),
				"success_count":  successCount,
				"failure_count":  len(errors),
				"errors":         errors,
			},
			nil,
		)
	}
	
	return nil
}

// GetFailedItems returns items that failed during execution
func (bo *BatchOperation) GetFailedItems() []BatchItem {
	var failed []BatchItem
	for _, item := range bo.items {
		if item.Error != nil {
			failed = append(failed, item)
		}
	}
	return failed
}

// GetSuccessfulItems returns items that completed successfully
func (bo *BatchOperation) GetSuccessfulItems() []BatchItem {
	var successful []BatchItem
	for _, item := range bo.items {
		if item.Completed && item.Error == nil {
			successful = append(successful, item)
		}
	}
	return successful
}

// ConsistencyChecker validates data consistency across operations
type ConsistencyChecker struct {
	checks []ConsistencyCheck
}

// ConsistencyCheck represents a single consistency validation
type ConsistencyCheck struct {
	Name     string
	Validate func(ctx context.Context) error
}

// NewConsistencyChecker creates a new consistency checker
func NewConsistencyChecker() *ConsistencyChecker {
	return &ConsistencyChecker{
		checks: make([]ConsistencyCheck, 0),
	}
}

// AddCheck adds a consistency check
func (cc *ConsistencyChecker) AddCheck(name string, validate func(ctx context.Context) error) {
	cc.checks = append(cc.checks, ConsistencyCheck{
		Name:     name,
		Validate: validate,
	})
}

// Validate runs all consistency checks
func (cc *ConsistencyChecker) Validate(ctx context.Context) error {
	var errors []error
	
	for _, check := range cc.checks {
		if err := check.Validate(ctx); err != nil {
			errors = append(errors, fmt.Errorf("consistency check '%s' failed: %w", check.Name, err))
		}
	}
	
	if len(errors) > 0 {
		return NewRepositoryErrorWithDetails(
			ErrCodeInconsistentState,
			fmt.Sprintf("consistency validation failed: %d checks failed", len(errors)),
			map[string]interface{}{
				"failed_checks": len(errors),
				"total_checks":  len(cc.checks),
				"errors":        errors,
			},
			nil,
		)
	}
	
	return nil
}

// Helper functions for creating common transaction operations

// CreateNodeWithRollback creates a node operation with rollback capability
func CreateNodeWithRollback(repo Repository, node domain.Node) (func(ctx context.Context) error, func(ctx context.Context) error) {
	execute := func(ctx context.Context) error {
		return repo.CreateNodeAndKeywords(ctx, node)
	}
	
	rollback := func(ctx context.Context) error {
		return repo.DeleteNode(ctx, node.UserID, node.ID)
	}
	
	return execute, rollback
}

// CreateEdgesWithRollback creates edges operation with rollback capability
func CreateEdgesWithRollback(repo Repository, userID, sourceNodeID string, relatedNodeIDs []string) (func(ctx context.Context) error, func(ctx context.Context) error) {
	execute := func(ctx context.Context) error {
		return repo.CreateEdges(ctx, userID, sourceNodeID, relatedNodeIDs)
	}
	
	rollback := func(ctx context.Context) error {
		// Note: This would require a method to delete specific edges
		// For now, we'll clear all connections for the source node
		return repo.DeleteNode(ctx, userID, sourceNodeID)
	}
	
	return execute, rollback
}