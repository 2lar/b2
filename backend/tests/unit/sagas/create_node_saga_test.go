package sagas_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"backend/application/sagas"
	"backend/domain/core/entities"
	"backend/domain/core/valueobjects"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCreateNodeSaga_Success(t *testing.T) {
	// Arrange
	logger := zap.NewNop()
	
	data := &sagas.CreateNodeSagaData{
		UserID:      "test-user-123",
		Title:       "Test Node",
		Content:     "Test content for the node",
		Tags:        []string{"test", "saga"},
		X:           100,
		Y:           200,
		Z:           0,
		OperationID: "op-123",
		StartTime:   time.Now(),
	}

	// Create a simple saga for testing
	saga := sagas.NewSagaBuilder("TestCreateNode", logger).
		WithStep("Step1", func(ctx context.Context, d interface{}) (interface{}, error) {
			sagaData := d.(*sagas.CreateNodeSagaData)
			assert.Equal(t, "test-user-123", sagaData.UserID)
			return sagaData, nil
		}).
		WithStep("Step2", func(ctx context.Context, d interface{}) (interface{}, error) {
			sagaData := d.(*sagas.CreateNodeSagaData)
			// Simulate node creation
			content, _ := valueobjects.NewNodeContent(sagaData.Title, sagaData.Content, valueobjects.FormatMarkdown)
			position, _ := valueobjects.NewPosition3D(sagaData.X, sagaData.Y, sagaData.Z)
			node, _ := entities.NewNode(sagaData.UserID, content, position)
			sagaData.Node = node
			sagaData.NodeCreated = true
			return sagaData, nil
		}).
		Build()

	// Act
	result, err := saga.Execute(context.Background(), data)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	
	resultData := result.(*sagas.CreateNodeSagaData)
	assert.True(t, resultData.NodeCreated)
	assert.NotNil(t, resultData.Node)
	assert.Equal(t, "Test Node", resultData.Node.Content().Title())
	assert.Equal(t, sagas.SagaStateCompleted, saga.GetState())
}

func TestCreateNodeSaga_FailureWithCompensation(t *testing.T) {
	// Arrange
	logger := zap.NewNop()
	
	data := &sagas.CreateNodeSagaData{
		UserID:      "test-user-123",
		Title:       "Test Node",
		OperationID: "op-456",
		StartTime:   time.Now(),
	}

	compensationCalled := false

	// Create saga with a failing step and compensation
	saga := sagas.NewSagaBuilder("TestCreateNodeFailure", logger).
		WithCompensableStep("Step1",
			func(ctx context.Context, d interface{}) (interface{}, error) {
				sagaData := d.(*sagas.CreateNodeSagaData)
				sagaData.NodeCreated = true
				return sagaData, nil
			},
			func(ctx context.Context, d interface{}) error {
				compensationCalled = true
				return nil
			},
		).
		WithStep("Step2", func(ctx context.Context, d interface{}) (interface{}, error) {
			// This step fails
			return nil, errors.New("simulated failure")
		}).
		Build()

	// Act
	result, err := saga.Execute(context.Background(), data)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated failure")
	assert.True(t, compensationCalled, "Compensation should have been called")
	assert.Equal(t, sagas.SagaStateCompensated, saga.GetState())
	assert.Nil(t, result)
}

func TestCreateNodeSaga_RetrySuccess(t *testing.T) {
	// Arrange
	logger := zap.NewNop()
	
	data := &sagas.CreateNodeSagaData{
		UserID:      "test-user-123",
		Title:       "Test Node",
		OperationID: "op-789",
		StartTime:   time.Now(),
	}

	attemptCount := 0

	// Create saga with a step that succeeds on retry
	saga := sagas.NewSagaBuilder("TestCreateNodeRetry", logger).
		WithRetryableStep("RetryableStep",
			func(ctx context.Context, d interface{}) (interface{}, error) {
				attemptCount++
				if attemptCount < 2 {
					return nil, errors.New("temporary failure")
				}
				// Success on second attempt
				return d, nil
			},
			3,                    // max retries
			10*time.Millisecond, // short retry delay for testing
		).
		Build()

	// Act
	result, err := saga.Execute(context.Background(), data)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, attemptCount, "Should have succeeded on second attempt")
	assert.Equal(t, sagas.SagaStateCompleted, saga.GetState())
	assert.NotNil(t, result)
}

func TestCreateNodeSaga_ValidationFailure(t *testing.T) {
	// Arrange
	logger := zap.NewNop()
	
	// Invalid data (empty user ID)
	data := &sagas.CreateNodeSagaData{
		UserID:      "", // Invalid
		Title:       "Test Node",
		OperationID: "op-invalid",
		StartTime:   time.Now(),
	}

	// Create saga with validation step
	saga := sagas.NewSagaBuilder("TestCreateNodeValidation", logger).
		WithStep("Validate", func(ctx context.Context, d interface{}) (interface{}, error) {
			sagaData := d.(*sagas.CreateNodeSagaData)
			if sagaData.UserID == "" {
				return nil, errors.New("user ID is required")
			}
			return sagaData, nil
		}).
		WithStep("CreateNode", func(ctx context.Context, d interface{}) (interface{}, error) {
			// This should not be reached
			t.Fatal("CreateNode step should not be executed after validation failure")
			return d, nil
		}).
		Build()

	// Act
	result, err := saga.Execute(context.Background(), data)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user ID is required")
	assert.Equal(t, sagas.SagaStateFailed, saga.GetState())
	assert.Nil(t, result)
}

func TestCreateNodeSaga_ComplexCompensation(t *testing.T) {
	// Arrange
	logger := zap.NewNop()
	
	data := &sagas.CreateNodeSagaData{
		UserID:      "test-user-123",
		Title:       "Test Node",
		OperationID: "op-complex",
		StartTime:   time.Now(),
	}

	// Track compensation order
	compensationOrder := []string{}

	// Create saga with multiple compensable steps
	saga := sagas.NewSagaBuilder("TestComplexCompensation", logger).
		WithCompensableStep("Step1",
			func(ctx context.Context, d interface{}) (interface{}, error) {
				return d, nil
			},
			func(ctx context.Context, d interface{}) error {
				compensationOrder = append(compensationOrder, "compensate-1")
				return nil
			},
		).
		WithCompensableStep("Step2",
			func(ctx context.Context, d interface{}) (interface{}, error) {
				return d, nil
			},
			func(ctx context.Context, d interface{}) error {
				compensationOrder = append(compensationOrder, "compensate-2")
				return nil
			},
		).
		WithCompensableStep("Step3",
			func(ctx context.Context, d interface{}) (interface{}, error) {
				return d, nil
			},
			func(ctx context.Context, d interface{}) error {
				compensationOrder = append(compensationOrder, "compensate-3")
				return nil
			},
		).
		WithStep("FailingStep", func(ctx context.Context, d interface{}) (interface{}, error) {
			return nil, errors.New("trigger compensation")
		}).
		Build()

	// Act
	result, err := saga.Execute(context.Background(), data)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trigger compensation")
	
	// Verify compensation was called in reverse order
	require.Len(t, compensationOrder, 3)
	assert.Equal(t, []string{"compensate-3", "compensate-2", "compensate-1"}, compensationOrder,
		"Compensation should be executed in reverse order")
	
	assert.Equal(t, sagas.SagaStateCompensated, saga.GetState())
	assert.Nil(t, result)
}

func TestCreateNodeSaga_PartialCompensationFailure(t *testing.T) {
	// Arrange
	logger := zap.NewNop()
	
	data := &sagas.CreateNodeSagaData{
		UserID:      "test-user-123",
		Title:       "Test Node",
		OperationID: "op-partial",
		StartTime:   time.Now(),
	}

	compensationAttempts := []string{}

	// Create saga where one compensation fails
	saga := sagas.NewSagaBuilder("TestPartialCompensation", logger).
		WithCompensableStep("Step1",
			func(ctx context.Context, d interface{}) (interface{}, error) {
				return d, nil
			},
			func(ctx context.Context, d interface{}) error {
				compensationAttempts = append(compensationAttempts, "compensate-1")
				return nil
			},
		).
		WithCompensableStep("Step2",
			func(ctx context.Context, d interface{}) (interface{}, error) {
				return d, nil
			},
			func(ctx context.Context, d interface{}) error {
				compensationAttempts = append(compensationAttempts, "compensate-2-failed")
				return errors.New("compensation failed")
			},
		).
		WithCompensableStep("Step3",
			func(ctx context.Context, d interface{}) (interface{}, error) {
				return d, nil
			},
			func(ctx context.Context, d interface{}) error {
				compensationAttempts = append(compensationAttempts, "compensate-3")
				return nil
			},
		).
		WithStep("FailingStep", func(ctx context.Context, d interface{}) (interface{}, error) {
			return nil, errors.New("trigger compensation")
		}).
		Build()

	// Act
	result, err := saga.Execute(context.Background(), data)

	// Assert
	require.Error(t, err)
	
	// All compensations should be attempted despite one failing
	require.Len(t, compensationAttempts, 3)
	assert.Contains(t, compensationAttempts, "compensate-3")
	assert.Contains(t, compensationAttempts, "compensate-2-failed")
	assert.Contains(t, compensationAttempts, "compensate-1")
	
	assert.Equal(t, sagas.SagaStateCompensated, saga.GetState())
	assert.Nil(t, result)
}