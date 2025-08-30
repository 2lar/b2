//go:build wireinject
// +build wireinject

// Package di implements compile-time dependency injection using Google Wire.
//
// WHY WIRE FOR LAMBDA ENVIRONMENTS:
//
// Traditional DI frameworks use reflection at runtime, which adds latency during
// Lambda cold starts. Wire generates all dependency wiring code at compile time,
// resulting in zero runtime overhead and faster cold starts.
//
// WIRE BUILD TAGS PATTERN:
//   • wire.go (this file): Has `//go:build wireinject` tag
//     - Only compiled when running `wire` command
//     - Contains dependency graph specification
//     - Wire replaces function bodies with generated code
//
//   • wire_gen.go: Has `//go:build !wireinject` tag  
//     - Compiled in normal builds (not when running `wire`)
//     - Contains generated constructor code
//     - Prevents "redeclared function" errors
//
// This build tag pattern allows the same function signature to exist in both
// files without conflicts, enabling Wire's code generation workflow.
//
// DEPENDENCY GRAPH APPROACH:
// Instead of manually wiring hundreds of dependencies, Wire analyzes provider
// functions and generates the optimal initialization order automatically.
// This eliminates dependency ordering bugs and reduces boilerplate.

package di

//go:generate wire

import (
	"github.com/google/wire"
)

// InitializeContainer wires together all dependencies using Wire.
// This function signature tells Wire what to generate by analyzing SuperSet.
//
// Wire Process:
//   1. Analyzes all provider functions in SuperSet
//   2. Builds dependency graph ensuring proper initialization order
//   3. Generates wire_gen.go with concrete implementation
//   4. Catches circular dependencies and missing providers at compile time
//
// The generated code is equivalent to manually calling all provider functions
// in the correct order, but without the maintenance burden.
func InitializeContainer() (*Container, error) {
	// Wire will replace this implementation with generated code
	// The SuperSet contains all provider functions Wire needs to analyze
	wire.Build(SuperSet)
	return nil, nil // Wire replaces this entire function body
}

// InitializeApplicationContainer wires together the new clean container architecture.
// This function uses the CleanSuperSet to create an ApplicationContainer with
// all the focused sub-containers properly initialized and wired.
func InitializeApplicationContainer() (*ApplicationContainer, error) {
	// Wire will replace this implementation with generated code
	// The CleanSuperSet contains the new focused container providers
	wire.Build(CleanSuperSet)
	return nil, nil // Wire replaces this entire function body
}