package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-gonic/gin"
)

// HealthHandler provides health check endpoints for monitoring and load balancing.
// This handler follows best practices for health check implementation.
type HealthHandler struct {
	dynamoClient *dynamodb.Client
	tableName    string
	// Add other dependencies for health checks (cache, external services, etc.)
}

// NewHealthHandler creates a new HealthHandler with required dependencies.
func NewHealthHandler(dynamoClient *dynamodb.Client, tableName string) *HealthHandler {
	return &HealthHandler{
		dynamoClient: dynamoClient,
		tableName:    tableName,
	}
}

// HealthResponse represents the health check response structure.
type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
	Uptime    string                 `json:"uptime,omitempty"`
	Checks    map[string]HealthCheck `json:"checks,omitempty"`
}

// HealthCheck represents an individual component health check.
type HealthCheck struct {
	Status      string        `json:"status"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	LastChecked time.Time     `json:"last_checked"`
}

const (
	StatusHealthy   = "healthy"
	StatusUnhealthy = "unhealthy"
	StatusDegraded  = "degraded"
)

// LivenessHandler implements the liveness probe endpoint.
// This endpoint indicates whether the application is alive and should restart if it fails.
//
// @Summary Application liveness check
// @Description Returns 200 if the application is alive, 503 if it should be restarted
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse "Application is alive"
// @Failure 503 {object} HealthResponse "Application should be restarted"
// @Router /health/live [get]
func (h *HealthHandler) LivenessHandler(c *gin.Context) {
	startTime := time.Now()
	
	// Liveness check should be minimal - just verify the application can respond
	healthResp := &HealthResponse{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
		Version:   "1.0.0", // This could come from build info
	}

	duration := time.Since(startTime)
	
	// Log the health check for monitoring
	c.Header("X-Health-Check-Duration", duration.String())
	
	c.JSON(http.StatusOK, healthResp)
}

// ReadinessHandler implements the readiness probe endpoint.
// This endpoint indicates whether the application is ready to accept traffic.
//
// @Summary Application readiness check
// @Description Returns 200 if ready to serve traffic, 503 if not ready
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse "Application is ready"
// @Failure 503 {object} HealthResponse "Application is not ready"
// @Router /health/ready [get]
func (h *HealthHandler) ReadinessHandler(c *gin.Context) {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	
	checks := make(map[string]HealthCheck)
	overallStatus := StatusHealthy
	
	// Check DynamoDB connectivity
	dbCheck := h.checkDynamoDB(ctx)
	checks["database"] = dbCheck
	if dbCheck.Status != StatusHealthy {
		overallStatus = StatusUnhealthy
	}
	
	// Add other dependency checks here
	// cacheCheck := h.checkCache(ctx)
	// checks["cache"] = cacheCheck
	
	// externalServiceCheck := h.checkExternalServices(ctx)
	// checks["external_services"] = externalServiceCheck
	
	healthResp := &HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Checks:    checks,
	}
	
	duration := time.Since(startTime)
	c.Header("X-Health-Check-Duration", duration.String())
	
	// Return 503 if not ready, 200 if ready
	statusCode := http.StatusOK
	if overallStatus != StatusHealthy {
		statusCode = http.StatusServiceUnavailable
	}
	
	c.JSON(statusCode, healthResp)
}

// HealthHandler implements a general health endpoint with detailed information.
// This is useful for monitoring dashboards and detailed health inspection.
//
// @Summary Detailed health check
// @Description Returns detailed health information about all components
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse "Detailed health information"
// @Failure 503 {object} HealthResponse "One or more components unhealthy"
// @Router /health [get]
func (h *HealthHandler) HealthHandler(c *gin.Context) {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	
	checks := make(map[string]HealthCheck)
	overallStatus := StatusHealthy
	
	// Perform all health checks
	dbCheck := h.checkDynamoDB(ctx)
	checks["database"] = dbCheck
	
	// Add performance metrics check
	performanceCheck := h.checkPerformance(ctx)
	checks["performance"] = performanceCheck
	
	// Add memory usage check
	memoryCheck := h.checkMemoryUsage(ctx)
	checks["memory"] = memoryCheck
	
	// Determine overall status
	if dbCheck.Status != StatusHealthy {
		overallStatus = StatusUnhealthy
	} else if performanceCheck.Status == StatusDegraded || memoryCheck.Status == StatusDegraded {
		overallStatus = StatusDegraded
	}
	
	healthResp := &HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    h.getUptime(),
		Checks:    checks,
	}
	
	duration := time.Since(startTime)
	c.Header("X-Health-Check-Duration", duration.String())
	
	// Return appropriate status code
	statusCode := http.StatusOK
	if overallStatus == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}
	
	c.JSON(statusCode, healthResp)
}

// checkDynamoDB verifies DynamoDB connectivity and basic functionality.
func (h *HealthHandler) checkDynamoDB(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		LastChecked: start,
	}
	
	// Perform a simple describe table operation to verify connectivity
	_, err := h.dynamoClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: &h.tableName,
	})
	
	check.Duration = time.Since(start)
	
	if err != nil {
		check.Status = StatusUnhealthy
		check.Error = "DynamoDB connection failed: " + err.Error()
	} else if check.Duration > 5*time.Second {
		check.Status = StatusDegraded
		check.Error = "DynamoDB response time is high"
	} else {
		check.Status = StatusHealthy
	}
	
	return check
}

// checkPerformance performs basic performance checks.
func (h *HealthHandler) checkPerformance(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		LastChecked: start,
		Status:      StatusHealthy,
	}
	
	// Simulate some work to measure performance
	time.Sleep(1 * time.Millisecond)
	
	check.Duration = time.Since(start)
	
	// Check if response time is acceptable
	if check.Duration > 100*time.Millisecond {
		check.Status = StatusDegraded
		check.Error = "Performance degraded - high response time"
	}
	
	return check
}

// checkMemoryUsage checks memory usage and reports if it's too high.
func (h *HealthHandler) checkMemoryUsage(ctx context.Context) HealthCheck {
	start := time.Now()
	check := HealthCheck{
		LastChecked: start,
		Status:      StatusHealthy,
		Duration:    time.Since(start),
	}
	
	// In a real implementation, you would check actual memory usage
	// For now, we'll just return healthy
	// runtime.ReadMemStats(&m)
	// if m.Alloc > threshold { check.Status = StatusDegraded }
	
	return check
}

// getUptime returns the application uptime.
func (h *HealthHandler) getUptime() string {
	// In a real implementation, you would track the application start time
	// For now, return a placeholder
	return "unknown"
}

// RegisterRoutes registers all health check routes with the Gin router.
func (h *HealthHandler) RegisterRoutes(router *gin.Engine) {
	healthGroup := router.Group("/health")
	{
		healthGroup.GET("", h.HealthHandler)
		healthGroup.GET("/", h.HealthHandler)
		healthGroup.GET("/live", h.LivenessHandler)
		healthGroup.GET("/ready", h.ReadinessHandler)
	}
}

// Example middleware for health check routes
func HealthCheckMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Add health check specific headers
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	})
}