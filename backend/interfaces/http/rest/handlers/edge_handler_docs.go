package handlers

// This file contains OpenAPI/Swagger documentation for EdgeHandler endpoints

// CreateEdge creates a new edge between nodes
// @Summary Create an edge
// @Description Creates a new edge connecting two nodes with optional weight and metadata
// @Tags edges
// @Accept json
// @Produce json
// @Param request body docs.CreateEdgeRequest true "Edge creation request"
// @Success 201 {object} docs.EdgeResponse "Edge created successfully"
// @Failure 400 {object} docs.ErrorResponse "Invalid request (e.g., self-reference)"
// @Failure 404 {object} docs.ErrorResponse "Source or target node not found"
// @Failure 409 {object} docs.ErrorResponse "Edge already exists"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /edges [post]

// GetEdge retrieves an edge by ID
// @Summary Get edge by ID
// @Description Retrieves edge information including weight and metadata
// @Tags edges
// @Accept json
// @Produce json
// @Param id path string true "Edge ID" example:"EDGE#node1#node2"
// @Success 200 {object} docs.EdgeResponse "Edge details"
// @Failure 404 {object} docs.ErrorResponse "Edge not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /edges/{id} [get]

// UpdateEdge updates edge properties
// @Summary Update an edge
// @Description Updates edge weight and metadata
// @Tags edges
// @Accept json
// @Produce json
// @Param id path string true "Edge ID"
// @Param request body docs.UpdateEdgeRequest true "Update request"
// @Success 200 {object} docs.EdgeResponse "Updated edge"
// @Failure 400 {object} docs.ErrorResponse "Invalid request"
// @Failure 404 {object} docs.ErrorResponse "Edge not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /edges/{id} [put]

// DeleteEdge deletes an edge
// @Summary Delete an edge
// @Description Removes an edge connection between two nodes
// @Tags edges
// @Accept json
// @Produce json
// @Param id path string true "Edge ID"
// @Success 204 "Edge deleted successfully"
// @Failure 404 {object} docs.ErrorResponse "Edge not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /edges/{id} [delete]

// GetNodeEdges retrieves all edges for a node
// @Summary Get edges for a node
// @Description Retrieves all incoming and outgoing edges for a specific node
// @Tags edges
// @Accept json
// @Produce json
// @Param node_id path string true "Node ID"
// @Param direction query string false "Edge direction (in/out/both)" default:"both"
// @Param type query string false "Filter by edge type"
// @Param page query int false "Page number" default:"1"
// @Param per_page query int false "Items per page" default:"20"
// @Success 200 {object} docs.PaginatedResponse "List of edges"
// @Failure 404 {object} docs.ErrorResponse "Node not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /nodes/{node_id}/edges [get]

// DiscoverEdges triggers edge discovery for a node
// @Summary Discover potential edges
// @Description Analyzes node content to discover and suggest potential edges
// @Tags edges
// @Accept json
// @Produce json
// @Param node_id path string true "Node ID"
// @Param request body docs.DiscoverEdgesRequest true "Discovery parameters"
// @Success 200 {object} docs.EdgeDiscoveryResponse "Discovered edge candidates"
// @Failure 404 {object} docs.ErrorResponse "Node not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /nodes/{node_id}/discover-edges [post]