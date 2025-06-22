// Package api defines the contracts for API requests and responses.
// It decouples the API structure from the internal domain models.
package api

import (
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
)

// CreateNodeRequest is the expected body for a POST /nodes request.
type CreateNodeRequest struct {
	Content string `json:"content"`
}

// UpdateNodeRequest is the expected body for a PUT /nodes/{nodeId} request.
type UpdateNodeRequest struct {
	Content string `json:"content"`
}

// NodeResponse is the API representation of a single node.
type NodeResponse struct {
	NodeID    string `json:"nodeId"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Version   int    `json:"version"`
}

// NodeDetailsResponse includes a node and its direct connections.
type NodeDetailsResponse struct {
	NodeID    string   `json:"nodeId"`
	Content   string   `json:"content"`
	Timestamp string   `json:"timestamp"`
	Version   int      `json:"version"`
	Edges     []string `json:"edges"`
}

// GraphDataResponse is the structure for the Cytoscape.js graph visualization.
type GraphDataResponse struct {
	Elements []interface{} `json:"elements"`
}

type GraphNode struct {
	Data NodeData `json:"data"`
}

type NodeData struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type GraphEdge struct {
	Data EdgeData `json:"data"`
}

type EdgeData struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

// ErrorResponse is a standardized error message for API responses.
type ErrorResponse struct {
	Error string `json:"error"`
}

// GatewayResponse is a helper to create a valid APIGatewayProxyResponse.
func GatewayResponse(statusCode int, body string) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}, nil
}

// Success formats a successful JSON response.
func Success(statusCode int, data interface{}) (events.APIGatewayProxyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return Error(500, "Internal server error"), err
	}
	return GatewayResponse(statusCode, string(body))
}

// Error formats a JSON error response.
func Error(statusCode int, message string) events.APIGatewayProxyResponse {
	body, _ := json.Marshal(ErrorResponse{Error: message})
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}
}
