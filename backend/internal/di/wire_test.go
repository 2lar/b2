package di

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeContainer(t *testing.T) {
	container, err := InitializeContainer()
	
	require.NoError(t, err)
	require.NotNil(t, container)
	
	// Test that all services are properly initialized
	assert.NotNil(t, container.Config)
	assert.NotNil(t, container.DynamoDBClient)
	assert.NotNil(t, container.EventBridgeClient)
	assert.NotNil(t, container.Repository)
	assert.NotNil(t, container.MemoryService)
	assert.NotNil(t, container.CategoryService)
	assert.NotNil(t, container.LLMService)
	assert.NotNil(t, container.Router)
	assert.NotNil(t, container.ChiLambda)
	
	// Test that services are properly configured
	assert.NotEmpty(t, container.Config.Region)
	assert.NotEmpty(t, container.Config.TableName)
	
	// Test that the LLM service is available (using mock provider)
	assert.True(t, container.LLMService.IsAvailable())
}