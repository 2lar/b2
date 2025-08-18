package shared

import (
	"fmt"
	"strings"
)

// GraphID represents a unique identifier for a graph
type GraphID string

// NewGraphID creates a new graph ID from a user ID and graph name
func NewGraphID(userID, graphName string) GraphID {
	return GraphID(fmt.Sprintf("%s:%s", userID, graphName))
}

// ParseGraphID parses a graph ID string into components
func ParseGraphID(id string) (userID, graphName string, err error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid graph ID format: %s", id)
	}
	return parts[0], parts[1], nil
}

// String returns the string representation of the graph ID
func (g GraphID) String() string {
	return string(g)
}

// UserID extracts the user ID from the graph ID
func (g GraphID) UserID() string {
	userID, _, _ := ParseGraphID(string(g))
	return userID
}

// GraphName extracts the graph name from the graph ID
func (g GraphID) GraphName() string {
	_, graphName, _ := ParseGraphID(string(g))
	return graphName
}

// IsValid checks if the graph ID has a valid format
func (g GraphID) IsValid() bool {
	_, _, err := ParseGraphID(string(g))
	return err == nil
}