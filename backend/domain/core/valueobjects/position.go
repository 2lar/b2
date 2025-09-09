package valueobjects

import (
	pkgerrors "backend/pkg/errors"
	"math"
)

// Position is a value object representing node coordinates in 2D/3D space
type Position struct {
	x float64
	y float64
	z float64 // For 3D graphs, 0 for 2D
}

// NewPosition2D creates a 2D position with validation
func NewPosition2D(x, y float64) (Position, error) {
	return NewPosition3D(x, y, 0)
}

// NewPosition3D creates a 3D position with validation
func NewPosition3D(x, y, z float64) (Position, error) {
	if !isValidCoordinate(x) || !isValidCoordinate(y) || !isValidCoordinate(z) {
		return Position{}, pkgerrors.NewValidationError("invalid coordinates: must be finite numbers")
	}
	return Position{x: x, y: y, z: z}, nil
}

// X returns the X coordinate
func (p Position) X() float64 {
	return p.x
}

// Y returns the Y coordinate
func (p Position) Y() float64 {
	return p.y
}

// Z returns the Z coordinate
func (p Position) Z() float64 {
	return p.z
}

// Is3D checks if this is a 3D position
func (p Position) Is3D() bool {
	return p.z != 0
}

// DistanceTo calculates the Euclidean distance to another position
func (p Position) DistanceTo(other Position) float64 {
	dx := p.x - other.x
	dy := p.y - other.y
	dz := p.z - other.z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// Equals checks if two positions are equal
func (p Position) Equals(other Position) bool {
	const epsilon = 1e-9
	return math.Abs(p.x-other.x) < epsilon &&
		math.Abs(p.y-other.y) < epsilon &&
		math.Abs(p.z-other.z) < epsilon
}

// Translate moves the position by the given offsets
func (p Position) Translate(dx, dy, dz float64) (Position, error) {
	return NewPosition3D(p.x+dx, p.y+dy, p.z+dz)
}

// Midpoint calculates the midpoint between two positions
func (p Position) Midpoint(other Position) Position {
	return Position{
		x: (p.x + other.x) / 2,
		y: (p.y + other.y) / 2,
		z: (p.z + other.z) / 2,
	}
}

// isValidCoordinate checks if a coordinate is a valid finite number
func isValidCoordinate(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
