package shared

import (
	"testing"
)

func TestWeight_NewWeight(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{
			name:    "valid weight minimum",
			value:   0.0,
			wantErr: false,
		},
		{
			name:    "valid weight maximum",
			value:   1.0,
			wantErr: false,
		},
		{
			name:    "valid weight middle",
			value:   0.5,
			wantErr: false,
		},
		{
			name:    "invalid weight negative",
			value:   -0.1,
			wantErr: true,
		},
		{
			name:    "invalid weight too high",
			value:   1.1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight, err := NewWeight(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWeight() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && weight.Value() != tt.value {
				t.Errorf("Weight.Value() = %v, want %v", weight.Value(), tt.value)
			}
		})
	}
}

func TestWeight_Classification(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		isStrong bool
		isWeak   bool
		isMedium bool
	}{
		{
			name:     "strong connection",
			value:    0.8,
			isStrong: true,
			isWeak:   false,
			isMedium: false,
		},
		{
			name:     "weak connection",
			value:    0.2,
			isStrong: false,
			isWeak:   true,
			isMedium: false,
		},
		{
			name:     "medium connection",
			value:    0.5,
			isStrong: false,
			isWeak:   false,
			isMedium: true,
		},
		{
			name:     "boundary strong",
			value:    0.7,
			isStrong: true,
			isWeak:   false,
			isMedium: false,
		},
		{
			name:     "boundary weak/medium",
			value:    0.3,
			isStrong: false,
			isWeak:   false,
			isMedium: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight, _ := NewWeight(tt.value)
			
			if got := weight.IsStrong(); got != tt.isStrong {
				t.Errorf("Weight.IsStrong() = %v, want %v", got, tt.isStrong)
			}
			if got := weight.IsWeak(); got != tt.isWeak {
				t.Errorf("Weight.IsWeak() = %v, want %v", got, tt.isWeak)
			}
			if got := weight.IsMedium(); got != tt.isMedium {
				t.Errorf("Weight.IsMedium() = %v, want %v", got, tt.isMedium)
			}
		})
	}
}

func TestWeight_Equals(t *testing.T) {
	weight1, _ := NewWeight(0.5)
	weight2, _ := NewWeight(0.5)
	weight3, _ := NewWeight(0.6)

	if !weight1.Equals(weight2) {
		t.Error("Equal weights should be equal")
	}

	if weight1.Equals(weight3) {
		t.Error("Different weights should not be equal")
	}
}

func TestEdgeMetadata_Operations(t *testing.T) {
	t.Run("new metadata is empty", func(t *testing.T) {
		metadata := NewEdgeMetadata()
		if !metadata.IsEmpty() {
			t.Error("New metadata should be empty")
		}
	})

	t.Run("set and get value", func(t *testing.T) {
		metadata := NewEdgeMetadata()
		metadata = metadata.Set("key1", "value1")
		metadata = metadata.Set("key2", 42)

		val1, exists1 := metadata.Get("key1")
		if !exists1 || val1 != "value1" {
			t.Errorf("Expected key1='value1', got %v", val1)
		}

		val2, exists2 := metadata.Get("key2")
		if !exists2 || val2 != 42 {
			t.Errorf("Expected key2=42, got %v", val2)
		}

		_, exists3 := metadata.Get("nonexistent")
		if exists3 {
			t.Error("Nonexistent key should not exist")
		}
	})

	t.Run("remove value", func(t *testing.T) {
		metadata := NewEdgeMetadata()
		metadata = metadata.Set("key1", "value1")
		metadata = metadata.Set("key2", "value2")
		
		metadata = metadata.Remove("key1")
		
		_, exists := metadata.Get("key1")
		if exists {
			t.Error("Removed key should not exist")
		}
		
		val, exists := metadata.Get("key2")
		if !exists || val != "value2" {
			t.Error("Other keys should remain after removal")
		}
	})

	t.Run("immutability", func(t *testing.T) {
		metadata1 := NewEdgeMetadata()
		metadata1 = metadata1.Set("key1", "value1")
		
		metadata2 := metadata1.Set("key2", "value2")
		
		// metadata1 should not have key2
		_, exists1 := metadata1.Get("key2")
		if exists1 {
			t.Error("Original metadata should not be modified")
		}
		
		// metadata2 should have both keys
		val1, _ := metadata2.Get("key1")
		val2, _ := metadata2.Get("key2")
		if val1 != "value1" || val2 != "value2" {
			t.Error("New metadata should have both keys")
		}
	})

	t.Run("to map", func(t *testing.T) {
		metadata := NewEdgeMetadata()
		metadata = metadata.Set("key1", "value1")
		metadata = metadata.Set("key2", 42)
		
		m := metadata.ToMap()
		if len(m) != 2 {
			t.Errorf("Expected 2 items in map, got %d", len(m))
		}
		
		// Verify map is a copy
		m["key3"] = "value3"
		_, exists := metadata.Get("key3")
		if exists {
			t.Error("Modifying returned map should not affect metadata")
		}
	})

	t.Run("with initial data", func(t *testing.T) {
		initialData := map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		}
		
		metadata := NewEdgeMetadataWithData(initialData)
		
		val1, _ := metadata.Get("key1")
		val2, _ := metadata.Get("key2")
		if val1 != "value1" || val2 != 42 {
			t.Error("Metadata should contain initial data")
		}
		
		// Verify initial data is copied
		initialData["key3"] = "value3"
		_, exists := metadata.Get("key3")
		if exists {
			t.Error("Modifying initial data should not affect metadata")
		}
	})

	t.Run("nil initial data", func(t *testing.T) {
		metadata := NewEdgeMetadataWithData(nil)
		if !metadata.IsEmpty() {
			t.Error("Metadata with nil initial data should be empty")
		}
	})
}