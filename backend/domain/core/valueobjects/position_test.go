package valueobjects

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPosition3D(t *testing.T) {
	tests := []struct {
		name    string
		x, y, z float64
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid position at origin",
			x:       0,
			y:       0,
			z:       0,
			wantErr: false,
		},
		{
			name:    "valid positive position",
			x:       100.5,
			y:       200.75,
			z:       50.25,
			wantErr: false,
		},
		{
			name:    "valid negative position",
			x:       -100.5,
			y:       -200.75,
			z:       -50.25,
			wantErr: false,
		},
		{
			name:    "very large coordinates",
			x:       1e10,
			y:       -1e10,
			z:       1e10,
			wantErr: false,
		},
		{
			name:    "NaN x coordinate",
			x:       math.NaN(),
			y:       0,
			z:       0,
			wantErr: true,
			errMsg:  "invalid coordinates",
		},
		{
			name:    "NaN y coordinate",
			x:       0,
			y:       math.NaN(),
			z:       0,
			wantErr: true,
			errMsg:  "invalid coordinates",
		},
		{
			name:    "NaN z coordinate",
			x:       0,
			y:       0,
			z:       math.NaN(),
			wantErr: true,
			errMsg:  "invalid coordinates",
		},
		{
			name:    "Infinity x coordinate",
			x:       math.Inf(1),
			y:       0,
			z:       0,
			wantErr: true,
			errMsg:  "invalid coordinates",
		},
		{
			name:    "Negative infinity y coordinate",
			x:       0,
			y:       math.Inf(-1),
			z:       0,
			wantErr: true,
			errMsg:  "invalid coordinates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, err := NewPosition3D(tt.x, tt.y, tt.z)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.x, pos.X())
				assert.Equal(t, tt.y, pos.Y())
				assert.Equal(t, tt.z, pos.Z())
			}
		})
	}
}

func TestNewPosition2D(t *testing.T) {
	tests := []struct {
		name    string
		x, y    float64
		wantErr bool
	}{
		{
			name:    "valid 2D position",
			x:       10.5,
			y:       20.5,
			wantErr: false,
		},
		{
			name:    "invalid 2D position with NaN",
			x:       math.NaN(),
			y:       20.5,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, err := NewPosition2D(tt.x, tt.y)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.x, pos.X())
				assert.Equal(t, tt.y, pos.Y())
				assert.Equal(t, float64(0), pos.Z())
				assert.False(t, pos.Is3D())
			}
		})
	}
}

func TestPosition_Is3D(t *testing.T) {
	tests := []struct {
		name     string
		pos      Position
		expected bool
	}{
		{
			name:     "2D position (z=0)",
			pos:      mustNewPosition(10, 20, 0),
			expected: false,
		},
		{
			name:     "3D position with positive z",
			pos:      mustNewPosition(10, 20, 30),
			expected: true,
		},
		{
			name:     "3D position with negative z",
			pos:      mustNewPosition(10, 20, -30),
			expected: true,
		},
		{
			name:     "2D position created with NewPosition2D",
			pos:      mustNewPosition2D(10, 20),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pos.Is3D()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPosition_DistanceTo(t *testing.T) {
	tests := []struct {
		name     string
		pos1     Position
		pos2     Position
		expected float64
		delta    float64
	}{
		{
			name:     "distance between same points",
			pos1:     mustNewPosition(0, 0, 0),
			pos2:     mustNewPosition(0, 0, 0),
			expected: 0,
			delta:    0.0001,
		},
		{
			name:     "distance along x-axis",
			pos1:     mustNewPosition(0, 0, 0),
			pos2:     mustNewPosition(10, 0, 0),
			expected: 10,
			delta:    0.0001,
		},
		{
			name:     "distance along y-axis",
			pos1:     mustNewPosition(0, 0, 0),
			pos2:     mustNewPosition(0, 10, 0),
			expected: 10,
			delta:    0.0001,
		},
		{
			name:     "distance along z-axis",
			pos1:     mustNewPosition(0, 0, 0),
			pos2:     mustNewPosition(0, 0, 10),
			expected: 10,
			delta:    0.0001,
		},
		{
			name:     "3D diagonal distance (3-4-5 triangle in xy plane)",
			pos1:     mustNewPosition(0, 0, 0),
			pos2:     mustNewPosition(3, 4, 0),
			expected: 5,
			delta:    0.0001,
		},
		{
			name:     "3D space distance",
			pos1:     mustNewPosition(1, 2, 3),
			pos2:     mustNewPosition(4, 6, 8),
			expected: math.Sqrt(9 + 16 + 25), // sqrt(50)
			delta:    0.0001,
		},
		{
			name:     "negative coordinates",
			pos1:     mustNewPosition(-5, -5, -5),
			pos2:     mustNewPosition(5, 5, 5),
			expected: math.Sqrt(300), // sqrt(100 + 100 + 100)
			delta:    0.0001,
		},
		{
			name:     "2D positions distance",
			pos1:     mustNewPosition2D(0, 0),
			pos2:     mustNewPosition2D(3, 4),
			expected: 5,
			delta:    0.0001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distance := tt.pos1.DistanceTo(tt.pos2)
			assert.InDelta(t, tt.expected, distance, tt.delta)

			// Distance should be symmetric
			reverseDistance := tt.pos2.DistanceTo(tt.pos1)
			assert.InDelta(t, distance, reverseDistance, 0.0001)
		})
	}
}

func TestPosition_Equals(t *testing.T) {
	tests := []struct {
		name     string
		pos1     Position
		pos2     Position
		expected bool
	}{
		{
			name:     "same positions",
			pos1:     mustNewPosition(1.5, 2.5, 3.5),
			pos2:     mustNewPosition(1.5, 2.5, 3.5),
			expected: true,
		},
		{
			name:     "different x",
			pos1:     mustNewPosition(1.5, 2.5, 3.5),
			pos2:     mustNewPosition(1.6, 2.5, 3.5),
			expected: false,
		},
		{
			name:     "different y",
			pos1:     mustNewPosition(1.5, 2.5, 3.5),
			pos2:     mustNewPosition(1.5, 2.6, 3.5),
			expected: false,
		},
		{
			name:     "different z",
			pos1:     mustNewPosition(1.5, 2.5, 3.5),
			pos2:     mustNewPosition(1.5, 2.5, 3.6),
			expected: false,
		},
		{
			name:     "zero positions",
			pos1:     mustNewPosition(0, 0, 0),
			pos2:     mustNewPosition(0, 0, 0),
			expected: true,
		},
		{
			name:     "negative positions",
			pos1:     mustNewPosition(-1, -2, -3),
			pos2:     mustNewPosition(-1, -2, -3),
			expected: true,
		},
		{
			name:     "very small difference (within epsilon)",
			pos1:     mustNewPosition(1.0, 2.0, 3.0),
			pos2:     mustNewPosition(1.0+1e-10, 2.0+1e-10, 3.0+1e-10),
			expected: true, // Within epsilon tolerance
		},
		{
			name:     "small difference (outside epsilon)",
			pos1:     mustNewPosition(1.0, 2.0, 3.0),
			pos2:     mustNewPosition(1.0+1e-8, 2.0, 3.0),
			expected: false, // Outside epsilon tolerance
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pos1.Equals(tt.pos2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPosition_Translate(t *testing.T) {
	tests := []struct {
		name        string
		initial     Position
		dx, dy, dz  float64
		wantErr     bool
		expectedPos Position
		errMsg      string
	}{
		{
			name:        "translate from origin",
			initial:     mustNewPosition(0, 0, 0),
			dx:          10,
			dy:          20,
			dz:          30,
			wantErr:     false,
			expectedPos: mustNewPosition(10, 20, 30),
		},
		{
			name:        "translate with negative deltas",
			initial:     mustNewPosition(100, 100, 100),
			dx:          -50,
			dy:          -25,
			dz:          -75,
			wantErr:     false,
			expectedPos: mustNewPosition(50, 75, 25),
		},
		{
			name:        "no translation",
			initial:     mustNewPosition(100, 200, 300),
			dx:          0,
			dy:          0,
			dz:          0,
			wantErr:     false,
			expectedPos: mustNewPosition(100, 200, 300),
		},
		{
			name:        "translate 2D position",
			initial:     mustNewPosition2D(10, 20),
			dx:          5,
			dy:          10,
			dz:          0,
			wantErr:     false,
			expectedPos: mustNewPosition(15, 30, 0),
		},
		{
			name:        "translate resulting in NaN",
			initial:     mustNewPosition(1e308, 0, 0),
			dx:          1e308, // Would overflow to Inf
			dy:          0,
			dz:          0,
			wantErr:     true,
			errMsg:      "invalid coordinates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newPos, err := tt.initial.Translate(tt.dx, tt.dy, tt.dz)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.True(t, newPos.Equals(tt.expectedPos))
			}
		})
	}
}

func TestPosition_Midpoint(t *testing.T) {
	tests := []struct {
		name     string
		pos1     Position
		pos2     Position
		expected Position
	}{
		{
			name:     "midpoint of same points",
			pos1:     mustNewPosition(10, 20, 30),
			pos2:     mustNewPosition(10, 20, 30),
			expected: mustNewPosition(10, 20, 30),
		},
		{
			name:     "midpoint along x-axis",
			pos1:     mustNewPosition(0, 0, 0),
			pos2:     mustNewPosition(10, 0, 0),
			expected: mustNewPosition(5, 0, 0),
		},
		{
			name:     "midpoint along y-axis",
			pos1:     mustNewPosition(0, 0, 0),
			pos2:     mustNewPosition(0, 10, 0),
			expected: mustNewPosition(0, 5, 0),
		},
		{
			name:     "midpoint along z-axis",
			pos1:     mustNewPosition(0, 0, 0),
			pos2:     mustNewPosition(0, 0, 10),
			expected: mustNewPosition(0, 0, 5),
		},
		{
			name:     "midpoint in 3D space",
			pos1:     mustNewPosition(2, 4, 6),
			pos2:     mustNewPosition(8, 12, 18),
			expected: mustNewPosition(5, 8, 12),
		},
		{
			name:     "midpoint with negative coordinates",
			pos1:     mustNewPosition(-10, -20, -30),
			pos2:     mustNewPosition(10, 20, 30),
			expected: mustNewPosition(0, 0, 0),
		},
		{
			name:     "midpoint of 2D positions",
			pos1:     mustNewPosition2D(0, 0),
			pos2:     mustNewPosition2D(10, 10),
			expected: mustNewPosition(5, 5, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			midpoint := tt.pos1.Midpoint(tt.pos2)
			assert.True(t, midpoint.Equals(tt.expected))

			// Midpoint should be symmetric
			reverseMidpoint := tt.pos2.Midpoint(tt.pos1)
			assert.True(t, midpoint.Equals(reverseMidpoint))
		})
	}
}

// Helper functions for tests
func mustNewPosition(x, y, z float64) Position {
	pos, err := NewPosition3D(x, y, z)
	if err != nil {
		panic(err)
	}
	return pos
}

func mustNewPosition2D(x, y float64) Position {
	pos, err := NewPosition2D(x, y)
	if err != nil {
		panic(err)
	}
	return pos
}

// Benchmarks
func BenchmarkNewPosition3D(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewPosition3D(100, 200, 300)
	}
}

func BenchmarkPosition_DistanceTo(b *testing.B) {
	pos1 := mustNewPosition(0, 0, 0)
	pos2 := mustNewPosition(100, 200, 300)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = pos1.DistanceTo(pos2)
	}
}

func BenchmarkPosition_Equals(b *testing.B) {
	pos1 := mustNewPosition(100, 200, 300)
	pos2 := mustNewPosition(100, 200, 300)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = pos1.Equals(pos2)
	}
}

func BenchmarkPosition_Translate(b *testing.B) {
	pos := mustNewPosition(100, 200, 300)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = pos.Translate(10, 20, 30)
	}
}

func BenchmarkPosition_Midpoint(b *testing.B) {
	pos1 := mustNewPosition(0, 0, 0)
	pos2 := mustNewPosition(100, 200, 300)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = pos1.Midpoint(pos2)
	}
}