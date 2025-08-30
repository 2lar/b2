package dto

import (
	"fmt"
	"time"
)

// ReconstructCreateNodeResult reconstructs a CreateNodeResult from a map[string]interface{}
// This is needed when the idempotency store returns a JSON-deserialized object
func ReconstructCreateNodeResult(data map[string]interface{}) (*CreateNodeResult, error) {
	result := &CreateNodeResult{}
	
	// Reconstruct the node
	if nodeData, ok := data["Node"].(map[string]interface{}); ok {
		result.Node = ReconstructNodeDTO(nodeData)
	}
	
	// Validate that Node was reconstructed successfully
	if result.Node == nil {
		return nil, fmt.Errorf("failed to reconstruct node from cached data")
	}
	
	// Reconstruct created edges
	if edges, ok := data["CreatedEdges"].([]interface{}); ok {
		result.CreatedEdges = make([]*EdgeDTO, 0, len(edges))
		for _, edge := range edges {
			if edgeMap, ok := edge.(map[string]interface{}); ok {
				result.CreatedEdges = append(result.CreatedEdges, ReconstructEdgeDTO(edgeMap))
			}
		}
	}
	
	return result, nil
}

// ReconstructNodeDTO reconstructs a NodeDTO from a map
func ReconstructNodeDTO(data map[string]interface{}) *NodeDTO {
	dto := &NodeDTO{}
	
	if id, ok := data["ID"].(string); ok {
		dto.ID = id
	}
	if userID, ok := data["UserID"].(string); ok {
		dto.UserID = userID
	}
	if content, ok := data["Content"].(string); ok {
		dto.Content = content
	}
	if title, ok := data["Title"].(string); ok {
		dto.Title = title
	}
	if keywords, ok := data["Keywords"].([]interface{}); ok {
		dto.Keywords = make([]string, 0, len(keywords))
		for _, k := range keywords {
			if str, ok := k.(string); ok {
				dto.Keywords = append(dto.Keywords, str)
			}
		}
	}
	if tags, ok := data["Tags"].([]interface{}); ok {
		dto.Tags = make([]string, 0, len(tags))
		for _, t := range tags {
			if str, ok := t.(string); ok {
				dto.Tags = append(dto.Tags, str)
			}
		}
	}
	if version, ok := data["Version"].(float64); ok {
		dto.Version = int(version)
	}
	
	// Parse timestamps - they might be strings in JSON
	if createdAt, ok := data["CreatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			dto.CreatedAt = t
		}
	}
	if updatedAt, ok := data["UpdatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			dto.UpdatedAt = t
		}
	}
	
	return dto
}

// ReconstructNodeView reconstructs a NodeView from a map
func ReconstructNodeView(data map[string]interface{}) *NodeView {
	view := &NodeView{}
	
	if id, ok := data["ID"].(string); ok {
		view.ID = id
	}
	if userID, ok := data["UserID"].(string); ok {
		view.UserID = userID
	}
	if content, ok := data["Content"].(string); ok {
		view.Content = content
	}
	if keywords, ok := data["Keywords"].([]interface{}); ok {
		view.Keywords = make([]string, 0, len(keywords))
		for _, k := range keywords {
			if str, ok := k.(string); ok {
				view.Keywords = append(view.Keywords, str)
			}
		}
	}
	if tags, ok := data["Tags"].([]interface{}); ok {
		view.Tags = make([]string, 0, len(tags))
		for _, t := range tags {
			if str, ok := t.(string); ok {
				view.Tags = append(view.Tags, str)
			}
		}
	}
	if version, ok := data["Version"].(float64); ok {
		view.Version = int(version)
	}
	
	// Parse timestamps - they might be strings in JSON
	if createdAt, ok := data["CreatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			view.CreatedAt = t
		}
	}
	if updatedAt, ok := data["UpdatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			view.UpdatedAt = t
		}
	}
	
	return view
}

// ReconstructEdgeDTO reconstructs an EdgeDTO from a map
func ReconstructEdgeDTO(data map[string]interface{}) *EdgeDTO {
	dto := &EdgeDTO{}
	
	if id, ok := data["ID"].(string); ok {
		dto.ID = id
	}
	if sourceNodeID, ok := data["SourceNodeID"].(string); ok {
		dto.SourceNodeID = sourceNodeID
	}
	if targetNodeID, ok := data["TargetNodeID"].(string); ok {
		dto.TargetNodeID = targetNodeID
	}
	if weight, ok := data["Weight"].(float64); ok {
		dto.Weight = weight
	}
	if userID, ok := data["UserID"].(string); ok {
		dto.UserID = userID
	}
	
	// Parse timestamps - they might be strings in JSON
	if createdAt, ok := data["CreatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			dto.CreatedAt = t
		}
	}
	if updatedAt, ok := data["UpdatedAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			dto.UpdatedAt = t
		}
	}
	
	return dto
}

// ReconstructConnectionView reconstructs a ConnectionView from a map
func ReconstructConnectionView(data map[string]interface{}) *ConnectionView {
	view := &ConnectionView{}
	
	// Map the correct field names from ConnectionView struct
	if id, ok := data["id"].(string); ok {
		view.ID = id
	}
	if sourceNodeID, ok := data["source_node_id"].(string); ok {
		view.SourceNodeID = sourceNodeID
	}
	if targetNodeID, ok := data["target_node_id"].(string); ok {
		view.TargetNodeID = targetNodeID
	}
	if strength, ok := data["strength"].(float64); ok {
		view.Strength = strength
	}
	if createdAt, ok := data["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			view.CreatedAt = t
		}
	}
	
	return view
}