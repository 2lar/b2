// Package api provides standardized helper functions for HTTP API responses.
package api

import (
	"encoding/json"
	"net/http"
)


// Success sends a standardized successful HTTP response with optional JSON data.
func Success(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// Error sends a standardized error response with consistent JSON format.
func Error(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// NodeResponse represents the response format for node operations.
type NodeResponse struct {
	NodeID    string   `json:"nodeId"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags,omitempty"`
	Timestamp string   `json:"timestamp"`
	Version   int      `json:"version"`
}

// NodeDetailsResponse represents the response format for detailed node information.
type NodeDetailsResponse struct {
	NodeID    string   `json:"nodeId"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags,omitempty"`
	Timestamp string   `json:"timestamp"`
	Version   int      `json:"version"`
	Edges     []string `json:"edges,omitempty"`
}
