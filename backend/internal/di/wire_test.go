package di

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeContainerIntegration(t *testing.T) {
	container, err := InitializeContainer()

	require.NoError(t, err)
	require.NotNil(t, container)
	
	router := container.GetRouter()
	require.NotNil(t, router)

	// Create a new HTTP request to the /health endpoint
	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Serve the HTTP request to the router
	router.ServeHTTP(rr, req)

	// Check the status code is what we expect
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body is what we expect
	assert.JSONEq(t, `{"status":"ok"}`, rr.Body.String())
	
	// Clean up
	container.Shutdown(req.Context())
}
