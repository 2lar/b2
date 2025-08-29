package handlers

import (
	"encoding/json"
	"net/http"
	
	"brain2-backend/pkg/api"
)

// HealthHandler handles health check endpoints
type HealthHandler struct{}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Check handles GET /health requests
// @Summary Basic health check
// @Description Returns the basic health status of the application
// @Tags System
// @Produce json
// @Success 200 {object} api.HealthResponse "Application is healthy"
// @Router /health [get]
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(api.HealthResponse{
		Status: "ok",
	})
}

// Ready handles GET /ready requests for readiness checks
// @Summary Application readiness check
// @Description Returns the readiness status of the application for load balancer health checks
// @Tags System
// @Produce json
// @Success 200 {object} api.HealthResponse "Application is ready to serve requests"
// @Router /ready [get]
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(api.HealthResponse{
		Status: "ready",
	})
}