// Package api provides API request and response types for the Brain2 system.
package api

// ConnectNodesRequest represents a request to connect two memory nodes.
// The source node ID is provided in the URL path, and the target node ID
// is provided in the request body.
type ConnectNodesRequest struct {
	// TargetNodeID is the ID of the node to connect to
	TargetNodeID string `json:"targetNodeId" validate:"required" example:"node-456"`
}

// ConnectionResponse represents the response after successfully connecting two nodes.
type ConnectionResponse struct {
	// Message provides a human-readable confirmation
	Message string `json:"message" example:"Nodes connected successfully"`
	// EdgeID is the unique identifier for the created edge
	EdgeID string `json:"edgeId" example:"node-123-node-456"`
}