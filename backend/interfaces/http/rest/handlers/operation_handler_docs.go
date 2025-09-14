package handlers

// This file contains OpenAPI/Swagger documentation for OperationHandler endpoints

// GetOperationStatus retrieves the status of an async operation
// @Summary Get operation status
// @Description Retrieves the current status and progress of an asynchronous operation
// @Tags operations
// @Accept json
// @Produce json
// @Param id path string true "Operation ID" example:"op_123456789"
// @Success 200 {object} docs.OperationStatus "Operation status and progress"
// @Failure 404 {object} docs.ErrorResponse "Operation not found"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /operations/{id} [get]

// ListOperations lists recent operations for the user
// @Summary List user operations
// @Description Retrieves a list of recent operations initiated by the user
// @Tags operations
// @Accept json
// @Produce json
// @Param status query string false "Filter by status (pending/running/completed/failed)"
// @Param type query string false "Filter by operation type"
// @Param page query int false "Page number" default:"1"
// @Param per_page query int false "Items per page" default:"20"
// @Success 200 {object} docs.PaginatedResponse "List of operations"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /operations [get]

// CancelOperation cancels a running operation
// @Summary Cancel operation
// @Description Attempts to cancel a running asynchronous operation
// @Tags operations
// @Accept json
// @Produce json
// @Param id path string true "Operation ID"
// @Success 200 {object} docs.OperationStatus "Updated operation status"
// @Failure 404 {object} docs.ErrorResponse "Operation not found"
// @Failure 409 {object} docs.ErrorResponse "Operation cannot be cancelled"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /operations/{id}/cancel [post]

// RetryOperation retries a failed operation
// @Summary Retry failed operation
// @Description Retries a failed operation with the same parameters
// @Tags operations
// @Accept json
// @Produce json
// @Param id path string true "Operation ID"
// @Success 202 {object} docs.OperationStatus "New operation started"
// @Failure 404 {object} docs.ErrorResponse "Operation not found"
// @Failure 409 {object} docs.ErrorResponse "Operation cannot be retried"
// @Failure 401 {object} docs.ErrorResponse "Unauthorized"
// @Failure 500 {object} docs.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /operations/{id}/retry [post]