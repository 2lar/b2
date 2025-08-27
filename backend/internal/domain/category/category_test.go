package category

import (
	"testing"
	"time"

	"brain2-backend/internal/domain/shared"
)

func TestCategory_UpdateTitle(t *testing.T) {
	userID, _ := shared.NewUserID("user123")
	category, _ := NewCategory(userID, "Initial Title", "Description")
	
	// Clear initial events
	category.MarkEventsAsCommitted()

	tests := []struct {
		name      string
		newTitle  string
		wantErr   bool
		wantEvent bool
	}{
		{
			name:      "valid title update",
			newTitle:  "Updated Title",
			wantErr:   false,
			wantEvent: true,
		},
		{
			name:      "same title no update",
			newTitle:  "Updated Title", // Same as previous
			wantErr:   false,
			wantEvent: false,
		},
		{
			name:      "empty title",
			newTitle:  "",
			wantErr:   true,
			wantEvent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear any previous events
			category.MarkEventsAsCommitted()
			
			err := category.UpdateTitle(tt.newTitle)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateTitle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			events := category.GetUncommittedEvents()
			hasEvent := len(events) > 0
			
			if hasEvent != tt.wantEvent {
				t.Errorf("Event generation = %v, wantEvent %v", hasEvent, tt.wantEvent)
			}
			
			if !tt.wantErr && tt.wantEvent {
				// Verify title was updated
				if category.Title != tt.newTitle {
					t.Errorf("Title = %v, want %v", category.Title, tt.newTitle)
				}
				
				// Verify event type
				if len(events) > 0 {
					if _, ok := events[0].(*CategoryUpdatedEvent); !ok {
						t.Error("Expected CategoryUpdatedEvent")
					}
				}
			}
		})
	}
}

func TestCategory_UpdateDescription(t *testing.T) {
	userID, _ := shared.NewUserID("user123")
	category, _ := NewCategory(userID, "Title", "Initial Description")
	
	// Clear initial events
	category.MarkEventsAsCommitted()

	tests := []struct {
		name           string
		newDescription string
		wantEvent      bool
	}{
		{
			name:           "valid description update",
			newDescription: "Updated Description",
			wantEvent:      true,
		},
		{
			name:           "same description no update",
			newDescription: "Updated Description", // Same as previous
			wantEvent:      false,
		},
		{
			name:           "empty description",
			newDescription: "",
			wantEvent:      true, // Empty is valid for description
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear any previous events
			category.MarkEventsAsCommitted()
			
			err := category.UpdateDescription(tt.newDescription)
			if err != nil {
				t.Errorf("UpdateDescription() unexpected error: %v", err)
			}
			
			events := category.GetUncommittedEvents()
			hasEvent := len(events) > 0
			
			if hasEvent != tt.wantEvent {
				t.Errorf("Event generation = %v, wantEvent %v", hasEvent, tt.wantEvent)
			}
			
			if tt.wantEvent {
				// Verify description was updated
				if category.Description != tt.newDescription {
					t.Errorf("Description = %v, want %v", category.Description, tt.newDescription)
				}
			}
		})
	}
}

func TestCategory_SetColor(t *testing.T) {
	userID, _ := shared.NewUserID("user123")
	category, _ := NewCategory(userID, "Title", "Description")
	
	// Clear initial events
	category.MarkEventsAsCommitted()

	t.Run("set new color", func(t *testing.T) {
		err := category.SetColor("#FF0000")
		if err != nil {
			t.Errorf("SetColor() unexpected error: %v", err)
		}
		
		if category.Color == nil || *category.Color != "#FF0000" {
			t.Error("Color not properly set")
		}
		
		events := category.GetUncommittedEvents()
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
	})

	t.Run("same color no update", func(t *testing.T) {
		category.MarkEventsAsCommitted()
		
		err := category.SetColor("#FF0000") // Same color
		if err != nil {
			t.Errorf("SetColor() unexpected error: %v", err)
		}
		
		events := category.GetUncommittedEvents()
		if len(events) != 0 {
			t.Error("Should not generate event for same color")
		}
	})
}

func TestCategory_ValidateInvariants(t *testing.T) {
	userID, _ := shared.NewUserID("user123")

	t.Run("valid category", func(t *testing.T) {
		category, _ := NewCategory(userID, "Title", "Description")
		if err := category.ValidateInvariants(); err != nil {
			t.Errorf("Valid category should pass invariants: %v", err)
		}
	})

	t.Run("empty title", func(t *testing.T) {
		category, _ := NewCategory(userID, "Title", "Description")
		category.Title = ""
		
		err := category.ValidateInvariants()
		if err == nil {
			t.Error("Empty title should fail invariants")
		}
	})

	t.Run("empty user ID", func(t *testing.T) {
		category, _ := NewCategory(userID, "Title", "Description")
		category.UserID = ""
		
		err := category.ValidateInvariants()
		if err == nil {
			t.Error("Empty user ID should fail invariants")
		}
	})

	t.Run("negative level", func(t *testing.T) {
		category, _ := NewCategory(userID, "Title", "Description")
		category.Level = -1
		
		err := category.ValidateInvariants()
		if err == nil {
			t.Error("Negative level should fail invariants")
		}
	})

	t.Run("negative note count", func(t *testing.T) {
		category, _ := NewCategory(userID, "Title", "Description")
		category.NoteCount = -1
		
		err := category.ValidateInvariants()
		if err == nil {
			t.Error("Negative note count should fail invariants")
		}
	})

	t.Run("self-parent", func(t *testing.T) {
		category, _ := NewCategory(userID, "Title", "Description")
		category.ParentID = &category.ID
		
		err := category.ValidateInvariants()
		if err == nil {
			t.Error("Self-parent should fail invariants")
		}
	})

	t.Run("invalid timestamps", func(t *testing.T) {
		category, _ := NewCategory(userID, "Title", "Description")
		category.UpdatedAt = category.CreatedAt.Add(-time.Hour)
		
		err := category.ValidateInvariants()
		if err == nil {
			t.Error("UpdatedAt before CreatedAt should fail invariants")
		}
	})
}

func TestCategory_SetParent(t *testing.T) {
	userID, _ := shared.NewUserID("user123")
	category, _ := NewCategory(userID, "Child", "Description")
	parentID := shared.CategoryID("parent-123")
	
	// Clear initial events
	category.MarkEventsAsCommitted()

	t.Run("set valid parent", func(t *testing.T) {
		err := category.SetParent(&parentID, 0)
		if err != nil {
			t.Errorf("SetParent() unexpected error: %v", err)
		}
		
		if category.ParentID == nil || *category.ParentID != parentID {
			t.Error("Parent not properly set")
		}
		
		if category.Level != 1 {
			t.Errorf("Level = %d, want 1", category.Level)
		}
		
		events := category.GetUncommittedEvents()
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
	})

	t.Run("set parent to nil (make root)", func(t *testing.T) {
		category.MarkEventsAsCommitted()
		
		err := category.SetParent(nil, 0)
		if err != nil {
			t.Errorf("SetParent(nil) unexpected error: %v", err)
		}
		
		if category.ParentID != nil {
			t.Error("ParentID should be nil")
		}
		
		if category.Level != 0 {
			t.Errorf("Level = %d, want 0", category.Level)
		}
	})

	t.Run("self-parent error", func(t *testing.T) {
		err := category.SetParent(&category.ID, 0)
		if err == nil {
			t.Error("Setting self as parent should return error")
		}
	})
}

func TestCategory_BaseAggregateRoot(t *testing.T) {
	userID, _ := shared.NewUserID("user123")
	category, _ := NewCategory(userID, "Title", "Description")

	t.Run("GetID", func(t *testing.T) {
		id := category.GetID()
		if id == "" {
			t.Error("GetID should return valid ID")
		}
		if id != string(category.ID) {
			t.Errorf("GetID = %v, want %v", id, string(category.ID))
		}
	})

	t.Run("GetVersion", func(t *testing.T) {
		version := category.GetVersion()
		if version != 0 {
			t.Errorf("Initial version should be 0, got %d", version)
		}
	})

	t.Run("IncrementVersion", func(t *testing.T) {
		initialVersion := category.GetVersion()
		category.IncrementVersion()
		newVersion := category.GetVersion()
		
		if newVersion != initialVersion+1 {
			t.Errorf("Version should increment by 1, got %d -> %d", initialVersion, newVersion)
		}
	})
}

func TestCategory_EventGeneration(t *testing.T) {
	userID, _ := shared.NewUserID("user123")
	
	t.Run("creation event", func(t *testing.T) {
		category, _ := NewCategory(userID, "Title", "Description")
		
		events := category.GetUncommittedEvents()
		if len(events) != 1 {
			t.Errorf("Expected 1 creation event, got %d", len(events))
		}
		
		if _, ok := events[0].(*shared.CategoryCreatedEvent); !ok {
			t.Error("Expected CategoryCreatedEvent")
		}
	})

	t.Run("multiple updates single transaction", func(t *testing.T) {
		category, _ := NewCategory(userID, "Title", "Description")
		category.MarkEventsAsCommitted()
		
		// Multiple updates
		category.UpdateTitle("New Title")
		category.UpdateDescription("New Description")
		category.SetColor("#00FF00")
		
		events := category.GetUncommittedEvents()
		if len(events) != 3 {
			t.Errorf("Expected 3 update events, got %d", len(events))
		}
		
		// All should be update events
		for _, event := range events {
			if _, ok := event.(*CategoryUpdatedEvent); !ok {
				t.Error("Expected CategoryUpdatedEvent")
			}
		}
	})
}