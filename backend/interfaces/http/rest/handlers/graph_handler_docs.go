package handlers

// This file contains OpenAPI/Swagger documentation for GraphHandler endpoints

// CreateGraph creates a new graph
// @Summary Create a new graph
// @Description Creates a new graph for organizing knowledge nodes
// @Tags graphs
// @Accept json
// @Produce json
// @Param request body docs.CreateGraphRequest true "Graph creation request"
// @Success 201 {object} docs.GraphResponse "Graph created successfully"
// @Failure 400 {object} docs.ErrorResponse "Invalid request"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /graphs [post]

// GetGraph retrieves a graph by ID
// @Summary Get graph by ID
// @Description Retrieves graph information including metadata and statistics
// @Tags graphs
// @Accept json
// @Produce json
// @Param id path string true "Graph ID" example:"GRAPH#user123#default"
// @Param include_nodes query bool false "Include nodes in response" default:"false"
// @Param include_edges query bool false "Include edges in response" default:"false"
// @Success 200 {object} docs.GraphResponse "Graph details"
// @Failure 404 {object} docs.ErrorResponse "Graph not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /graphs/{id} [get]

// GetUserGraphs retrieves all graphs for a user
// @Summary Get user's graphs
// @Description Retrieves all graphs owned by the authenticated user
// @Tags graphs
// @Accept json
// @Produce json
// @Param page query int false "Page number" default:"1"
// @Param per_page query int false "Items per page" default:"10"
// @Success 200 {object} docs.PaginatedResponse "List of user's graphs"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /graphs [get]

// UpdateGraph updates graph properties
// @Summary Update a graph
// @Description Updates graph name and metadata
// @Tags graphs
// @Accept json
// @Produce json
// @Param id path string true "Graph ID"
// @Param request body docs.UpdateGraphRequest true "Update request"
// @Success 200 {object} docs.GraphResponse "Updated graph"
// @Failure 400 {object} docs.ErrorResponse "Invalid request"
// @Failure 404 {object} docs.ErrorResponse "Graph not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /graphs/{id} [put]

// DeleteGraph deletes a graph and all its contents
// @Summary Delete a graph
// @Description Deletes a graph including all nodes and edges
// @Tags graphs
// @Accept json
// @Produce json
// @Param id path string true "Graph ID"
// @Success 204 "Graph deleted successfully"
// @Failure 404 {object} docs.ErrorResponse "Graph not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /graphs/{id} [delete]

// GetGraphStatistics retrieves graph statistics
// @Summary Get graph statistics
// @Description Retrieves detailed statistics and metrics for a graph
// @Tags graphs
// @Accept json
// @Produce json
// @Param id path string true "Graph ID"
// @Success 200 {object} docs.GraphMetadata "Graph statistics"
// @Failure 404 {object} docs.ErrorResponse "Graph not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /graphs/{id}/stats [get]