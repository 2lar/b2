// Package di provides cold start tracking.
package di

import (
	"time"
)

// ColdStartTracker tracks cold start information.
type ColdStartTracker struct {
	ColdStartTime *time.Time
	IsColdStart   bool
}

// NewColdStartTracker creates a new cold start tracker.
func NewColdStartTracker() *ColdStartTracker {
	coldStartTime := time.Now()
	return &ColdStartTracker{
		ColdStartTime: &coldStartTime,
		IsColdStart:   true,
	}
}

// GetTimeSinceColdStart returns the time since cold start.
func (t *ColdStartTracker) GetTimeSinceColdStart() time.Duration {
	if t.ColdStartTime == nil {
		return 0
	}
	return time.Since(*t.ColdStartTime)
}

// IsPostColdStartRequest returns true if this is after a cold start.
func (t *ColdStartTracker) IsPostColdStartRequest() bool {
	return t.IsColdStart
}

// ProvideColdStartTracker creates a cold start tracker for Wire.
func ProvideColdStartTracker() *ColdStartTracker {
	return NewColdStartTracker()
}