//go:build wireinject
// +build wireinject

package di

//go:generate wire

import (
	"github.com/google/wire"
)

// InitializeContainer wires together all dependencies using Wire.
// This function signature tells Wire what to generate.
func InitializeContainer() (*Container, error) {
	// Wire will generate the implementation using SuperSet
	wire.Build(SuperSet)
	return nil, nil // Wire replaces this
}