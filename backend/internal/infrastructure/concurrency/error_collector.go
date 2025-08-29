package concurrency

import (
	"fmt"
	"strings"
	"sync"
)

// ErrorCollector safely collects errors from concurrent operations
type ErrorCollector struct {
	mu           sync.RWMutex
	errors       map[string]error
	errorOrder   []string
	maxErrors    int
	stopOnError  bool
	stopped      bool
}

// ErrorSummary provides a summary of collected errors
type ErrorSummary struct {
	TotalErrors  int
	Errors       map[string]error
	FirstError   error
	ErrorMessage string
}

// NewErrorCollector creates a new error collector
func NewErrorCollector(maxErrors int, stopOnError bool) *ErrorCollector {
	if maxErrors <= 0 {
		maxErrors = 100 // Default max errors to track
	}
	
	return &ErrorCollector{
		errors:      make(map[string]error),
		errorOrder:  make([]string, 0),
		maxErrors:   maxErrors,
		stopOnError: stopOnError,
		stopped:     false,
	}
}

// Add adds an error to the collector
func (ec *ErrorCollector) Add(id string, err error) {
	if err == nil {
		return
	}
	
	ec.mu.Lock()
	defer ec.mu.Unlock()
	
	// Check if we should stop collecting
	if ec.stopped || (ec.stopOnError && len(ec.errors) > 0) {
		ec.stopped = true
		return
	}
	
	// Check if we've reached max errors
	if len(ec.errors) >= ec.maxErrors {
		// Keep only the first maxErrors
		return
	}
	
	// Add error if not already present
	if _, exists := ec.errors[id]; !exists {
		ec.errors[id] = err
		ec.errorOrder = append(ec.errorOrder, id)
		
		// Mark as stopped if stopOnError is enabled
		if ec.stopOnError {
			ec.stopped = true
		}
	}
}

// AddBulk adds multiple errors at once
func (ec *ErrorCollector) AddBulk(errors map[string]error) {
	for id, err := range errors {
		ec.Add(id, err)
		
		// Check if we should stop
		ec.mu.RLock()
		stopped := ec.stopped
		ec.mu.RUnlock()
		
		if stopped {
			break
		}
	}
}

// HasErrors returns true if any errors have been collected
func (ec *ErrorCollector) HasErrors() bool {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return len(ec.errors) > 0
}

// GetErrorCount returns the number of errors collected
func (ec *ErrorCollector) GetErrorCount() int {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return len(ec.errors)
}

// GetErrors returns a copy of all collected errors
func (ec *ErrorCollector) GetErrors() map[string]error {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	
	// Return a copy to prevent external modification
	errorsCopy := make(map[string]error, len(ec.errors))
	for k, v := range ec.errors {
		errorsCopy[k] = v
	}
	return errorsCopy
}

// GetFirstError returns the first error that was collected
func (ec *ErrorCollector) GetFirstError() error {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	
	if len(ec.errorOrder) == 0 {
		return nil
	}
	
	firstID := ec.errorOrder[0]
	return ec.errors[firstID]
}

// GetErrorsByID returns errors for specific IDs
func (ec *ErrorCollector) GetErrorsByID(ids []string) map[string]error {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	
	result := make(map[string]error)
	for _, id := range ids {
		if err, exists := ec.errors[id]; exists {
			result[id] = err
		}
	}
	return result
}

// GetSummary returns a summary of all collected errors
func (ec *ErrorCollector) GetSummary() *ErrorSummary {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	
	summary := &ErrorSummary{
		TotalErrors: len(ec.errors),
		Errors:      make(map[string]error, len(ec.errors)),
	}
	
	// Copy errors
	for k, v := range ec.errors {
		summary.Errors[k] = v
	}
	
	// Set first error
	if len(ec.errorOrder) > 0 {
		summary.FirstError = ec.errors[ec.errorOrder[0]]
	}
	
	// Create error message
	summary.ErrorMessage = ec.buildErrorMessage()
	
	return summary
}

// buildErrorMessage creates a formatted error message
func (ec *ErrorCollector) buildErrorMessage() string {
	if len(ec.errors) == 0 {
		return ""
	}
	
	if len(ec.errors) == 1 {
		for id, err := range ec.errors {
			return fmt.Sprintf("error processing %s: %v", id, err)
		}
	}
	
	// Multiple errors - create summary
	var messages []string
	maxDisplay := 5 // Show first 5 errors in message
	
	displayed := 0
	for _, id := range ec.errorOrder {
		if displayed >= maxDisplay {
			break
		}
		if err, exists := ec.errors[id]; exists {
			messages = append(messages, fmt.Sprintf("%s: %v", id, err))
			displayed++
		}
	}
	
	if len(ec.errors) > maxDisplay {
		messages = append(messages, fmt.Sprintf("... and %d more errors", len(ec.errors)-maxDisplay))
	}
	
	return fmt.Sprintf("%d errors occurred:\n%s", len(ec.errors), strings.Join(messages, "\n"))
}

// Clear resets the error collector
func (ec *ErrorCollector) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	
	ec.errors = make(map[string]error)
	ec.errorOrder = make([]string, 0)
	ec.stopped = false
}

// IsStopped returns true if collection has been stopped
func (ec *ErrorCollector) IsStopped() bool {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.stopped
}

// Stop stops error collection
func (ec *ErrorCollector) Stop() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.stopped = true
}

// Merge merges errors from another collector
func (ec *ErrorCollector) Merge(other *ErrorCollector) {
	if other == nil {
		return
	}
	
	otherErrors := other.GetErrors()
	ec.AddBulk(otherErrors)
}

// ToError converts the collector to a single error
func (ec *ErrorCollector) ToError() error {
	summary := ec.GetSummary()
	
	if summary.TotalErrors == 0 {
		return nil
	}
	
	if summary.TotalErrors == 1 {
		return summary.FirstError
	}
	
	return fmt.Errorf(summary.ErrorMessage)
}

// ErrorGroup provides a way to group related errors
type ErrorGroup struct {
	name      string
	collector *ErrorCollector
}

// NewErrorGroup creates a new error group
func NewErrorGroup(name string) *ErrorGroup {
	return &ErrorGroup{
		name:      name,
		collector: NewErrorCollector(100, false),
	}
}

// Add adds an error to the group
func (eg *ErrorGroup) Add(err error) {
	if err != nil {
		eg.collector.Add(eg.name, err)
	}
}

// AddWithID adds an error with a specific ID
func (eg *ErrorGroup) AddWithID(id string, err error) {
	if err != nil {
		eg.collector.Add(fmt.Sprintf("%s.%s", eg.name, id), err)
	}
}

// HasErrors returns true if the group has errors
func (eg *ErrorGroup) HasErrors() bool {
	return eg.collector.HasErrors()
}

// ToError returns the group's errors as a single error
func (eg *ErrorGroup) ToError() error {
	return eg.collector.ToError()
}