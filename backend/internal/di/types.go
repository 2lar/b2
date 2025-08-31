// Package di provides types for dependency injection.
// This file contains shared types that are used by both Wire and the application containers.
package di

import (
	"context"
	"time"
)

// ColdStartInfoProvider interface for cold start tracking.
type ColdStartInfoProvider interface {
	GetTimeSinceColdStart() time.Duration
	IsPostColdStartRequest() bool
}

// HealthChecker interface for health checks.
type HealthChecker interface {
	Health(ctx context.Context) map[string]string
}