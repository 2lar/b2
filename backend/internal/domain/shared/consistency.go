package shared

import (
	"fmt"
)

// ConsistencyBoundary defines rules for aggregate consistency
type ConsistencyBoundary struct {
	aggregateType string
	rules         []ConsistencyRule
}

// ConsistencyRule defines a business rule that must be satisfied
type ConsistencyRule interface {
	Validate(aggregate AggregateRoot) error
	Name() string
}

// NewConsistencyBoundary creates a new consistency boundary for an aggregate type
func NewConsistencyBoundary(aggregateType string) *ConsistencyBoundary {
	return &ConsistencyBoundary{
		aggregateType: aggregateType,
		rules:         []ConsistencyRule{},
	}
}

// AddRule adds a consistency rule to the boundary
func (cb *ConsistencyBoundary) AddRule(rule ConsistencyRule) {
	cb.rules = append(cb.rules, rule)
}

// Validate checks all consistency rules for the aggregate
func (cb *ConsistencyBoundary) Validate(aggregate AggregateRoot) error {
	// First validate the aggregate's own invariants
	if err := aggregate.ValidateInvariants(); err != nil {
		return err
	}
	
	// Then validate all boundary rules
	for _, rule := range cb.rules {
		if err := rule.Validate(aggregate); err != nil {
			return NewDomainError(
				"consistency_violation",
				fmt.Sprintf("Consistency rule '%s' violated for %s", rule.Name(), cb.aggregateType),
				err,
			)
		}
	}
	
	return nil
}

// Common Consistency Rules

// SingleAggregatePerTransactionRule ensures only one aggregate is modified per transaction
type SingleAggregatePerTransactionRule struct {
	modifiedAggregates map[string]bool
}

func NewSingleAggregatePerTransactionRule() *SingleAggregatePerTransactionRule {
	return &SingleAggregatePerTransactionRule{
		modifiedAggregates: make(map[string]bool),
	}
}

func (r *SingleAggregatePerTransactionRule) Validate(aggregate AggregateRoot) error {
	if len(r.modifiedAggregates) > 0 && !r.modifiedAggregates[aggregate.GetID()] {
		return fmt.Errorf("cannot modify multiple aggregates in a single transaction")
	}
	r.modifiedAggregates[aggregate.GetID()] = true
	return nil
}

func (r *SingleAggregatePerTransactionRule) Name() string {
	return "SingleAggregatePerTransaction"
}

// VersionConsistencyRule ensures optimistic locking is respected
type VersionConsistencyRule struct {
	expectedVersions map[string]int
}

func NewVersionConsistencyRule() *VersionConsistencyRule {
	return &VersionConsistencyRule{
		expectedVersions: make(map[string]int),
	}
}

func (r *VersionConsistencyRule) SetExpectedVersion(aggregateID string, version int) {
	r.expectedVersions[aggregateID] = version
}

func (r *VersionConsistencyRule) Validate(aggregate AggregateRoot) error {
	expectedVersion, exists := r.expectedVersions[aggregate.GetID()]
	if exists && aggregate.GetVersion() != expectedVersion {
		return fmt.Errorf("version mismatch: expected %d, got %d", expectedVersion, aggregate.GetVersion())
	}
	return nil
}

func (r *VersionConsistencyRule) Name() string {
	return "VersionConsistency"
}

// EventCountRule ensures aggregates don't generate too many events in one operation
type EventCountRule struct {
	maxEvents int
}

func NewEventCountRule(maxEvents int) *EventCountRule {
	return &EventCountRule{maxEvents: maxEvents}
}

func (r *EventCountRule) Validate(aggregate AggregateRoot) error {
	events := aggregate.GetUncommittedEvents()
	if len(events) > r.maxEvents {
		return fmt.Errorf("too many events generated: %d (max: %d)", len(events), r.maxEvents)
	}
	return nil
}

func (r *EventCountRule) Name() string {
	return "EventCount"
}

// AggregateStateRule validates aggregate-specific state constraints
type AggregateStateRule struct {
	name      string
	validator func(aggregate AggregateRoot) error
}

func NewAggregateStateRule(name string, validator func(AggregateRoot) error) *AggregateStateRule {
	return &AggregateStateRule{
		name:      name,
		validator: validator,
	}
}

func (r *AggregateStateRule) Validate(aggregate AggregateRoot) error {
	return r.validator(aggregate)
}

func (r *AggregateStateRule) Name() string {
	return r.name
}