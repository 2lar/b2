package valueobjects

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNodeID(t *testing.T) {
	id := NewNodeID()

	assert.NotEmpty(t, id.String())
	assert.False(t, id.IsZero())

	// Should be a valid UUID
	_, err := uuid.Parse(id.String())
	assert.NoError(t, err)
}

func TestNewNodeIDFromString(t *testing.T) {
	validUUID := uuid.New().String()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid UUID string",
			input:   validUUID,
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "node ID cannot be empty",
		},
		{
			name:    "invalid UUID format",
			input:   "not-a-uuid",
			wantErr: true,
			errMsg:  "node ID must be a valid UUID",
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: true,
			errMsg:  "node ID must be a valid UUID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := NewNodeIDFromString(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.True(t, id.IsZero())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.input, id.String())
				assert.False(t, id.IsZero())
			}
		})
	}
}

func TestNodeID_Equals(t *testing.T) {
	id1 := NewNodeID()
	id2 := NewNodeID()
	id1Copy, _ := NewNodeIDFromString(id1.String())

	tests := []struct {
		name     string
		id       NodeID
		other    NodeID
		expected bool
	}{
		{
			name:     "same ID via copy",
			id:       id1,
			other:    id1Copy,
			expected: true,
		},
		{
			name:     "same ID reference",
			id:       id1,
			other:    id1,
			expected: true,
		},
		{
			name:     "different IDs",
			id:       id1,
			other:    id2,
			expected: false,
		},
		{
			name:     "both zero values",
			id:       NodeID{},
			other:    NodeID{},
			expected: true,
		},
		{
			name:     "one zero value",
			id:       id1,
			other:    NodeID{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.id.Equals(tt.other)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNodeID_IsZero(t *testing.T) {
	tests := []struct {
		name     string
		id       NodeID
		expected bool
	}{
		{
			name:     "new ID is not zero",
			id:       NewNodeID(),
			expected: false,
		},
		{
			name:     "empty struct is zero",
			id:       NodeID{},
			expected: true,
		},
		{
			name:     "ID from valid string is not zero",
			id:       mustNodeIDFromString(uuid.New().String()),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.id.IsZero()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNodeID_MarshalJSON(t *testing.T) {
	id := NewNodeID()

	data, err := id.MarshalJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Should be a quoted string
	str := string(data)
	assert.True(t, len(str) > 2)
	assert.Equal(t, '"', rune(str[0]))
	assert.Equal(t, '"', rune(str[len(str)-1]))

	// The UUID should be inside the quotes
	unquoted := str[1 : len(str)-1]
	assert.Equal(t, id.String(), unquoted)
}

func TestNodeID_UnmarshalJSON(t *testing.T) {
	originalID := NewNodeID()

	// Marshal to JSON
	data, err := originalID.MarshalJSON()
	require.NoError(t, err)

	// Unmarshal back
	var newID NodeID
	err = newID.UnmarshalJSON(data)
	require.NoError(t, err)

	// Should be equal
	assert.True(t, originalID.Equals(newID))
	assert.Equal(t, originalID.String(), newID.String())
}

func TestNodeID_MarshalUnmarshalRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		id   NodeID
	}{
		{
			name: "regular ID",
			id:   NewNodeID(),
		},
		{
			name: "zero value",
			id:   NodeID{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := tt.id.MarshalJSON()
			require.NoError(t, err)

			// Unmarshal
			var newID NodeID
			err = newID.UnmarshalJSON(data)

			if tt.id.IsZero() {
				// Zero value should fail unmarshal with empty string
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, tt.id.Equals(newID))
			}
		})
	}
}

// Helper function for tests
func mustNodeIDFromString(s string) NodeID {
	id, err := NewNodeIDFromString(s)
	if err != nil {
		panic(err)
	}
	return id
}

// Benchmarks
func BenchmarkNewNodeID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewNodeID()
	}
}

func BenchmarkNodeID_String(b *testing.B) {
	id := NewNodeID()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
}

func BenchmarkNodeID_Equals(b *testing.B) {
	id1 := NewNodeID()
	id2 := NewNodeID()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = id1.Equals(id2)
	}
}

func BenchmarkNodeID_MarshalJSON(b *testing.B) {
	id := NewNodeID()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = id.MarshalJSON()
	}
}

func BenchmarkNodeID_UnmarshalJSON(b *testing.B) {
	id := NewNodeID()
	data, _ := id.MarshalJSON()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var newID NodeID
		_ = newID.UnmarshalJSON(data)
	}
}