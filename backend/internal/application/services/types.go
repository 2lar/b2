// Package services provides application services for the Brain2 backend.
package services

// NOTE: This file previously contained view models and query result types.
// All view models have been moved to internal/application/dto/ for better CQRS separation.
// Command types have been moved to internal/application/commands/
//
// For view models, use:
//   - internal/application/dto for Data Transfer Objects and view models
//   - internal/application/queries for query services and query types