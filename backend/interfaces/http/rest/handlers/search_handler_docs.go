package handlers

// This file contains OpenAPI/Swagger documentation for SearchHandler endpoints

// SearchNodes performs full-text search across nodes
// @Summary Search nodes
// @Description Performs full-text search across node titles and content with relevance scoring
// @Tags search
// @Accept json
// @Produce json
// @Param request body docs.SearchRequest true "Search parameters"
// @Success 200 {object} docs.SearchResponse "Search results with relevance scores"
// @Failure 400 {object} docs.ErrorResponse "Invalid search parameters"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /search/nodes [post]

// SearchByTags searches nodes by tags
// @Summary Search by tags
// @Description Finds nodes that match specified tags with AND/OR logic
// @Tags search
// @Accept json
// @Produce json
// @Param tags query []string true "Tags to search for" collectionFormat:"multi"
// @Param match query string false "Match logic (all/any)" default:"any"
// @Param graph_id query string false "Limit to specific graph"
// @Param page query int false "Page number" default:"1"
// @Param per_page query int false "Items per page" default:"20"
// @Success 200 {object} docs.PaginatedResponse "Nodes matching tags"
// @Failure 400 {object} docs.ErrorResponse "Invalid parameters"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /search/tags [get]

// SimilarNodes finds nodes similar to a given node
// @Summary Find similar nodes
// @Description Finds nodes with similar content using semantic analysis
// @Tags search
// @Accept json
// @Produce json
// @Param node_id path string true "Reference node ID"
// @Param limit query int false "Maximum results" default:"10"
// @Param threshold query number false "Similarity threshold (0.0-1.0)" default:"0.7"
// @Success 200 {object} docs.SimilarityResponse "Similar nodes with scores"
// @Failure 404 {object} docs.ErrorResponse "Node not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /nodes/{node_id}/similar [get]

// SearchGraphPatterns searches for graph patterns
// @Summary Search graph patterns
// @Description Finds subgraphs matching specific patterns or structures
// @Tags search
// @Accept json
// @Produce json
// @Param request body docs.PatternSearchRequest true "Pattern search parameters"
// @Success 200 {object} docs.PatternSearchResponse "Matching subgraphs"
// @Failure 400 {object} docs.ErrorResponse "Invalid pattern"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /search/patterns [post]

// GetRecentNodes retrieves recently created or updated nodes
// @Summary Get recent nodes
// @Description Retrieves nodes ordered by creation or update time
// @Tags search
// @Accept json
// @Produce json
// @Param graph_id query string false "Filter by graph"
// @Param order_by query string false "Order by field (created_at/updated_at)" default:"created_at"
// @Param limit query int false "Maximum results" default:"20"
// @Success 200 {object} docs.NodeListResponse "Recent nodes"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /search/recent [get]