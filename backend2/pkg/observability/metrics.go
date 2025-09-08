package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// Metrics handles application metrics and monitoring
type Metrics struct {
	namespace string
	client    *cloudwatch.Client
}

// NewMetrics creates a new metrics instance
func NewMetrics(namespace string, client *cloudwatch.Client) *Metrics {
	return &Metrics{
		namespace: namespace,
		client:    client,
	}
}

// RecordCommandExecution records metrics for command execution
func (m *Metrics) RecordCommandExecution(ctx context.Context, commandName string, duration time.Duration, err error) {
	if m.client == nil {
		return // Skip if no client configured
	}

	status := "success"
	if err != nil {
		status = "failure"
	}

	// Create metric data
	metricData := []types.MetricDatum{
		{
			MetricName: aws.String("CommandExecution"),
			Dimensions: []types.Dimension{
				{
					Name:  aws.String("CommandName"),
					Value: aws.String(commandName),
				},
				{
					Name:  aws.String("Status"),
					Value: aws.String(status),
				},
			},
			Value:     aws.Float64(float64(duration.Milliseconds())),
			Unit:      types.StandardUnitMilliseconds,
			Timestamp: aws.Time(time.Now()),
		},
		{
			MetricName: aws.String("CommandCount"),
			Dimensions: []types.Dimension{
				{
					Name:  aws.String("CommandName"),
					Value: aws.String(commandName),
				},
				{
					Name:  aws.String("Status"),
					Value: aws.String(status),
				},
			},
			Value:     aws.Float64(1),
			Unit:      types.StandardUnitCount,
			Timestamp: aws.Time(time.Now()),
		},
	}

	// Send metrics to CloudWatch
	input := &cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(m.namespace),
		MetricData: metricData,
	}

	if _, err := m.client.PutMetricData(ctx, input); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Failed to send metrics: %v\n", err)
	}
}

// RecordLatency records latency for any operation
func (m *Metrics) RecordLatency(ctx context.Context, operation string, latency time.Duration) {
	if m.client == nil {
		return
	}

	metricData := []types.MetricDatum{
		{
			MetricName: aws.String("OperationLatency"),
			Dimensions: []types.Dimension{
				{
					Name:  aws.String("Operation"),
					Value: aws.String(operation),
				},
			},
			Value:     aws.Float64(float64(latency.Milliseconds())),
			Unit:      types.StandardUnitMilliseconds,
			Timestamp: aws.Time(time.Now()),
		},
	}

	input := &cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(m.namespace),
		MetricData: metricData,
	}

	m.client.PutMetricData(ctx, input)
}

// RecordError records error occurrences
func (m *Metrics) RecordError(ctx context.Context, errorType string, errorCode string) {
	if m.client == nil {
		return
	}

	metricData := []types.MetricDatum{
		{
			MetricName: aws.String("Errors"),
			Dimensions: []types.Dimension{
				{
					Name:  aws.String("ErrorType"),
					Value: aws.String(errorType),
				},
				{
					Name:  aws.String("ErrorCode"),
					Value: aws.String(errorCode),
				},
			},
			Value:     aws.Float64(1),
			Unit:      types.StandardUnitCount,
			Timestamp: aws.Time(time.Now()),
		},
	}

	input := &cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(m.namespace),
		MetricData: metricData,
	}

	m.client.PutMetricData(ctx, input)
}

// RecordBusinessMetric records custom business metrics
func (m *Metrics) RecordBusinessMetric(ctx context.Context, metricName string, value float64, unit types.StandardUnit, dimensions map[string]string) {
	if m.client == nil {
		return
	}

	// Convert dimensions map to CloudWatch format
	var cwDimensions []types.Dimension
	for name, val := range dimensions {
		cwDimensions = append(cwDimensions, types.Dimension{
			Name:  aws.String(name),
			Value: aws.String(val),
		})
	}

	metricData := []types.MetricDatum{
		{
			MetricName: aws.String(metricName),
			Dimensions: cwDimensions,
			Value:      aws.Float64(value),
			Unit:       unit,
			Timestamp:  aws.Time(time.Now()),
		},
	}

	input := &cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(m.namespace),
		MetricData: metricData,
	}

	m.client.PutMetricData(ctx, input)
}
