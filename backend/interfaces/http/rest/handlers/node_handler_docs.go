package handlers

// This file contains OpenAPI/Swagger documentation for NodeHandler endpoints

// CreateNode creates a new node with automatic edge discovery
// @Summary Create a new knowledge node
// @Description Creates a new node in the graph with automatic edge discovery based on content similarity
// @Tags nodes
// @Accept json
// @Produce json
// @Param request body docs.CreateNodeRequest true "Node creation request"
// @Success 201 {object} docs.CreateNodeResponse "Node created successfully"
// @Failure 400 {object} docs.ErrorResponse "Invalid request parameters"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /nodes [post]

// GetNode retrieves a node by ID
// @Summary Get node by ID
// @Description Retrieves complete node information including metadata and edge counts
// @Tags nodes
// @Accept json
// @Produce json
// @Param id path string true "Node ID" example:"550e8400-e29b-41d4-a716-446655440000"
// @Success 200 {object} docs.NodeResponse "Node details"
// @Failure 404 {object} docs.ErrorResponse "Node not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /nodes/{id} [get]

// UpdateNode updates an existing node
// @Summary Update a node
// @Description Updates node properties including title, content, tags, and position
// @Tags nodes
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param request body docs.UpdateNodeRequest true "Update request"
// @Success 200 {object} docs.NodeResponse "Updated node"
// @Failure 400 {object} docs.ErrorResponse "Invalid request"
// @Failure 404 {object} docs.ErrorResponse "Node not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /nodes/{id} [put]

// DeleteNode deletes a node and its edges
// @Summary Delete a node
// @Description Deletes a node and optionally its associated edges
// @Tags nodes
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param delete_edges query bool false "Delete associated edges" default:"true"
// @Success 204 "Node deleted successfully"
// @Failure 404 {object} docs.ErrorResponse "Node not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /nodes/{id} [delete]

// BulkDeleteNodes deletes multiple nodes
// @Summary Bulk delete nodes
// @Description Deletes multiple nodes in a single operation
// @Tags nodes
// @Accept json
// @Produce json
// @Param request body docs.BulkDeleteRequest true "Bulk delete request"
// @Success 200 {object} docs.BulkDeleteResponse "Deletion results"
// @Failure 400 {object} docs.ErrorResponse "Invalid request"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /nodes/bulk-delete [post]

// GetNodesByGraph retrieves all nodes in a graph
// @Summary Get nodes by graph
// @Description Retrieves paginated list of nodes in a specific graph
// @Tags nodes
// @Accept json
// @Produce json
// @Param graph_id query string true "Graph ID"
// @Param page query int false "Page number" default:"1"
// @Param per_page query int false "Items per page" default:"20"
// @Param sort_by query string false "Sort field" default:"created_at"
// @Param sort_order query string false "Sort order (asc/desc)" default:"desc"
// @Success 200 {object} docs.PaginatedResponse "Paginated nodes list"
// @Failure 400 {object} docs.ErrorResponse "Invalid parameters"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /nodes [get]