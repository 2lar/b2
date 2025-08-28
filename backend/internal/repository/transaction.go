package repository

import (
	"context"
	"fmt"
	"time"

	"brain2-backend/internal/domain/node"
)

type TransactionManager struct {
	steps []TransactionStep
}

type TransactionStep struct {
	Name        string
	Execute     func(ctx context.Context) error
	Rollback    func(ctx context.Context) error
	Executed    bool
	RollbackRun bool
}

func NewTransactionManager() *TransactionManager {
	return &TransactionManager{
		steps: make([]TransactionStep, 0),
	}
}

func (tm *TransactionManager) AddStep(name string, execute, rollback func(ctx context.Context) error) {
	tm.steps = append(tm.steps, TransactionStep{
		Name:     name,
		Execute:  execute,
		Rollback: rollback,
	})
}

func (tm *TransactionManager) Execute(ctx context.Context) error {
	for i := range tm.steps {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		step := &tm.steps[i]
		if err := step.Execute(ctx); err != nil {
			step.Executed = true

			rollbackErr := tm.rollback(ctx, i)
			if rollbackErr != nil {
				return fmt.Errorf("transaction error: step '%s' failed and rollback failed: original error: %w, rollback error: %v", 
					step.Name, err, rollbackErr)
			}

			return fmt.Errorf("transaction error: step '%s' failed: %w", step.Name, err)
		}

		step.Executed = true
	}

	return nil
}

func (tm *TransactionManager) rollback(ctx context.Context, failedStepIndex int) error {
	var rollbackErrors []error

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

type CompensatingTransaction struct {
	actions []CompensatingAction
}

type CompensatingAction struct {
	Name        string
	Action      func(ctx context.Context) (interface{}, error)
	Compensate  func(ctx context.Context, result interface{}) error
	Result      interface{}
	Completed   bool
	Compensated bool
}

func NewCompensatingTransaction() *CompensatingTransaction {
	return &CompensatingTransaction{
		actions: make([]CompensatingAction, 0),
	}
}

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

func (ct *CompensatingTransaction) Execute(ctx context.Context) error {
	for i := range ct.actions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		action := &ct.actions[i]
		result, err := action.Action(ctx)

		if err != nil {
			compensateErr := ct.compensate(ctx, i-1)
			if compensateErr != nil {
				return fmt.Errorf("transaction error: action '%s' failed and compensation failed: original error: %w, compensation error: %v",
					action.Name, err, compensateErr)
			}

			return fmt.Errorf("transaction error: action '%s' failed: %w", action.Name, err)
		}

		action.Result = result
		action.Completed = true
	}

	return nil
}

func (ct *CompensatingTransaction) compensate(ctx context.Context, lastCompletedIndex int) error {
	var compensationErrors []error

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

type BatchOperation struct {
	items   []BatchItem
	options BatchOptions
}

type BatchItem struct {
	ID        string
	Operation func(ctx context.Context) error
	Completed bool
	Error     error
}

type BatchOptions struct {
	MaxConcurrency int
	Timeout        time.Duration
	StopOnError    bool
	RetryFailures  bool
}

func DefaultBatchOptions() BatchOptions {
	return BatchOptions{
		MaxConcurrency: 10,
		Timeout:        30 * time.Second,
		StopOnError:    false,
		RetryFailures:  true,
	}
}

func NewBatchOperation(options BatchOptions) *BatchOperation {
	return &BatchOperation{
		items:   make([]BatchItem, 0),
		options: options,
	}
}

func (bo *BatchOperation) AddItem(id string, operation func(ctx context.Context) error) {
	bo.items = append(bo.items, BatchItem{
		ID:        id,
		Operation: operation,
	})
}

func (bo *BatchOperation) Execute(ctx context.Context) error {
	if len(bo.items) == 0 {
		return nil
	}

	semaphore := make(chan struct{}, bo.options.MaxConcurrency)
	results := make(chan int, len(bo.items))

	for i := range bo.items {
		go func(index int) {
			defer func() { results <- index }()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			operationCtx, cancel := context.WithTimeout(ctx, bo.options.Timeout)
			defer cancel()

			item := &bo.items[index]
			err := item.Operation(operationCtx)

			item.Error = err
			item.Completed = err == nil

			if err != nil && bo.options.StopOnError {
				if ctxCancel := ctx.Value("cancel"); ctxCancel != nil {
					if cancel, ok := ctxCancel.(context.CancelFunc); ok {
						cancel()
					}
				}
			}
		}(i)
	}

	for i := 0; i < len(bo.items); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-results:
		}
	}

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
		return fmt.Errorf("batch operation failed: %d/%d items succeeded, errors: %v", 
			successCount, len(bo.items), errors)
	}

	return nil
}

func (bo *BatchOperation) GetFailedItems() []BatchItem {
	var failed []BatchItem
	for _, item := range bo.items {
		if item.Error != nil {
			failed = append(failed, item)
		}
	}
	return failed
}

func (bo *BatchOperation) GetSuccessfulItems() []BatchItem {
	var successful []BatchItem
	for _, item := range bo.items {
		if item.Completed && item.Error == nil {
			successful = append(successful, item)
		}
	}
	return successful
}

type ConsistencyChecker struct {
	checks []ConsistencyCheck
}

type ConsistencyCheck struct {
	Name     string
	Validate func(ctx context.Context) error
}

func NewConsistencyChecker() *ConsistencyChecker {
	return &ConsistencyChecker{
		checks: make([]ConsistencyCheck, 0),
	}
}

func (cc *ConsistencyChecker) AddCheck(name string, validate func(ctx context.Context) error) {
	cc.checks = append(cc.checks, ConsistencyCheck{
		Name:     name,
		Validate: validate,
	})
}

func (cc *ConsistencyChecker) Validate(ctx context.Context) error {
	var errors []error

	for _, check := range cc.checks {
		if err := check.Validate(ctx); err != nil {
			errors = append(errors, fmt.Errorf("consistency check '%s' failed: %w", check.Name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("consistency validation failed: %d checks failed out of %d, errors: %v",
			len(errors), len(cc.checks), errors)
	}

	return nil
}

func CreateNodeWithRollback(repo Repository, node *node.Node) (func(ctx context.Context) error, func(ctx context.Context) error) {
	execute := func(ctx context.Context) error {
		return repo.CreateNodeAndKeywords(ctx, node)
	}

	rollback := func(ctx context.Context) error {
		return repo.DeleteNode(ctx, node.UserID().String(), node.ID().String())
	}

	return execute, rollback
}

func CreateEdgesWithRollback(repo Repository, userID, sourceNodeID string, relatedNodeIDs []string) (func(ctx context.Context) error, func(ctx context.Context) error) {
	execute := func(ctx context.Context) error {
		return repo.CreateEdges(ctx, userID, sourceNodeID, relatedNodeIDs)
	}

	rollback := func(ctx context.Context) error {
		return repo.DeleteNode(ctx, userID, sourceNodeID)
	}

	return execute, rollback
}
