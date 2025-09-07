package valueobjects

import (
	"errors"
	"github.com/google/uuid"
)

// NodeID is a value object representing a unique node identifier
// Value objects are immutable and have no identity beyond their value
type NodeID struct {
	value string
}

// NewNodeID creates a new random NodeID
func NewNodeID() NodeID {
	return NodeID{value: uuid.New().String()}
}

// NewNodeIDFromString creates a NodeID from an existing string
func NewNodeIDFromString(id string) (NodeID, error) {
	if id == "" {
		return NodeID{}, errors.New("node ID cannot be empty")
	}
	if !isValidUUID(id) {
		return NodeID{}, errors.New("node ID must be a valid UUID")
	}
	return NodeID{value: id}, nil
}

// String returns the string representation of the NodeID
func (id NodeID) String() string {
	return id.value
}

// Equals checks if two NodeIDs are equal
func (id NodeID) Equals(other NodeID) bool {
	return id.value == other.value
}

// IsZero checks if the NodeID is the zero value
func (id NodeID) IsZero() bool {
	return id.value == ""
}

// MarshalJSON implements json.Marshaler
func (id NodeID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.value + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (id *NodeID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.New("NodeID must be a string")
	}
	id.value = string(data[1 : len(data)-1])
	return nil
}

// isValidUUID validates if a string is a valid UUID
func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}