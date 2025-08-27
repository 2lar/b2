package edge

import (
	"testing"
	"time"

	"brain2-backend/internal/domain/shared"
)

func TestEdge_UpdateWeight(t *testing.T) {
	// Setup
	sourceID := shared.NewNodeID()
	targetID := shared.NewNodeID()
	userID, _ := shared.NewUserID("user123")
	initialWeight := 0.5

	edge, err := NewEdge(sourceID, targetID, userID, initialWeight)
	if err != nil {
		t.Fatalf("Failed to create edge: %v", err)
	}

	// Clear initial events
	edge.MarkEventsAsCommitted()

	tests := []struct {
		name      string
		newWeight float64
		wantErr   bool
		wantEvent bool
	}{
		{
			name:      "valid weight update",
			newWeight: 0.8,
			wantErr:   false,
			wantEvent: true,
		},
		{
			name:      "same weight no update",
			newWeight: 0.8, // Same as previous
			wantErr:   false,
			wantEvent: false,
		},
		{
			name:      "invalid weight negative",
			newWeight: -0.1,
			wantErr:   true,
			wantEvent: false,
		},
		{
			name:      "invalid weight too high",
			newWeight: 1.5,
			wantErr:   true,
			wantEvent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear any previous events
			edge.MarkEventsAsCommitted()
			
			err := edge.UpdateWeight(tt.newWeight)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateWeight() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			events := edge.GetUncommittedEvents()
			hasEvent := len(events) > 0
			
			if hasEvent != tt.wantEvent {
				t.Errorf("Event generation = %v, wantEvent %v", hasEvent, tt.wantEvent)
			}
			
			if !tt.wantErr && tt.wantEvent {
				// Verify weight was updated
				if edge.Weight() != tt.newWeight {
					t.Errorf("Weight = %v, want %v", edge.Weight(), tt.newWeight)
				}
				
				// Verify event details
				if len(events) > 0 {
					if event, ok := events[0].(*shared.EdgeWeightUpdatedEvent); ok {
						if event.NewWeight != tt.newWeight {
							t.Errorf("Event NewWeight = %v, want %v", event.NewWeight, tt.newWeight)
						}
					} else {
						t.Error("Expected EdgeWeightUpdatedEvent")
					}
				}
			}
		})
	}
}

func TestEdge_ValidateInvariants(t *testing.T) {
	sourceID := shared.NewNodeID()
	targetID := shared.NewNodeID()
	userID, _ := shared.NewUserID("user123")

	t.Run("valid edge", func(t *testing.T) {
		edge, _ := NewEdge(sourceID, targetID, userID, 0.5)
		if err := edge.ValidateInvariants(); err != nil {
			t.Errorf("Valid edge should pass invariants: %v", err)
		}
	})

	t.Run("self-connection", func(t *testing.T) {
		edge := &Edge{
			BaseAggregateRoot: shared.NewBaseAggregateRoot("test"),
			id:        sourceID,
			sourceID:  sourceID,
			targetID:  sourceID, // Same as source
			userID:    userID,
			weight:    shared.Weight{}, // Will be invalid but we're testing self-connection
			metadata:  shared.NewEdgeMetadata(),
			createdAt: time.Now(),
			updatedAt: time.Now(),
			version:   shared.NewVersion(),
		}
		
		err := edge.ValidateInvariants()
		if err == nil {
			t.Error("Self-connection should fail invariants")
		}
	})

	t.Run("timestamps", func(t *testing.T) {
		edge, _ := NewEdge(sourceID, targetID, userID, 0.5)
		// Force invalid timestamp
		edge.updatedAt = edge.createdAt.Add(-time.Hour)
		
		err := edge.ValidateInvariants()
		if err == nil {
			t.Error("UpdatedAt before CreatedAt should fail invariants")
		}
	})
}

func TestEdge_Metadata(t *testing.T) {
	sourceID := shared.NewNodeID()
	targetID := shared.NewNodeID()
	userID, _ := shared.NewUserID("user123")

	edge, _ := NewEdge(sourceID, targetID, userID, 0.5)

	t.Run("initial metadata is empty", func(t *testing.T) {
		metadata := edge.Metadata()
		if !metadata.IsEmpty() {
			t.Error("Initial metadata should be empty")
		}
	})

	t.Run("set and get metadata", func(t *testing.T) {
		metadata := shared.NewEdgeMetadata()
		metadata = metadata.Set("reason", "similar_content")
		metadata = metadata.Set("confidence", 0.95)
		
		edge.SetMetadata(metadata)
		
		retrievedMetadata := edge.Metadata()
		reason, _ := retrievedMetadata.Get("reason")
		confidence, _ := retrievedMetadata.Get("confidence")
		
		if reason != "similar_content" || confidence != 0.95 {
			t.Error("Metadata not properly set")
		}
	})
}

func TestEdge_BaseAggregateRoot(t *testing.T) {
	sourceID := shared.NewNodeID()
	targetID := shared.NewNodeID()
	userID, _ := shared.NewUserID("user123")

	edge, _ := NewEdge(sourceID, targetID, userID, 0.5)

	t.Run("GetID", func(t *testing.T) {
		id := edge.GetID()
		if id == "" {
			t.Error("GetID should return valid ID")
		}
	})

	t.Run("GetVersion", func(t *testing.T) {
		version := edge.GetVersion()
		if version != 0 {
			t.Errorf("Initial version should be 0, got %d", version)
		}
	})

	t.Run("IncrementVersion", func(t *testing.T) {
		initialVersion := edge.GetVersion()
		edge.IncrementVersion()
		newVersion := edge.GetVersion()
		
		if newVersion != initialVersion+1 {
			t.Errorf("Version should increment by 1, got %d -> %d", initialVersion, newVersion)
		}
	})
}

func TestEdge_Delete(t *testing.T) {
	sourceID := shared.NewNodeID()
	targetID := shared.NewNodeID()
	userID, _ := shared.NewUserID("user123")

	edge, _ := NewEdge(sourceID, targetID, userID, 0.5)
	edge.MarkEventsAsCommitted()

	initialVersion := edge.GetVersion()
	edge.Delete()

	events := edge.GetUncommittedEvents()
	if len(events) != 1 {
		t.Errorf("Expected 1 delete event, got %d", len(events))
	}

	if _, ok := events[0].(*shared.EdgeDeletedEvent); !ok {
		t.Error("Expected EdgeDeletedEvent")
	}

	if edge.GetVersion() <= initialVersion {
		t.Error("Version should be incremented after delete")
	}
}

func TestEdge_ClassificationMethods(t *testing.T) {
	sourceID := shared.NewNodeID()
	targetID := shared.NewNodeID()
	userID, _ := shared.NewUserID("user123")

	tests := []struct {
		weight   float64
		isStrong bool
		isWeak   bool
	}{
		{weight: 0.8, isStrong: true, isWeak: false},
		{weight: 0.2, isStrong: false, isWeak: true},
		{weight: 0.5, isStrong: false, isWeak: false},
	}

	for _, tt := range tests {
		edge, _ := NewEdge(sourceID, targetID, userID, tt.weight)
		
		if edge.IsStrongConnection() != tt.isStrong {
			t.Errorf("Weight %v: IsStrongConnection = %v, want %v", tt.weight, edge.IsStrongConnection(), tt.isStrong)
		}
		
		if edge.IsWeakConnection() != tt.isWeak {
			t.Errorf("Weight %v: IsWeakConnection = %v, want %v", tt.weight, edge.IsWeakConnection(), tt.isWeak)
		}
	}
}