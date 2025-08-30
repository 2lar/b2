package metrics

import (
	"time"

	"brain2-backend/internal/infrastructure/observability"
)

// NoOpMetricsCollector is a simple metrics collector that does nothing
type NoOpMetricsCollector struct{}

// NewNoOpMetricsCollector creates a new no-op metrics collector
func NewNoOpMetricsCollector() *observability.Collector {
	return nil // Return nil for no-op case
}

func (m *NoOpMetricsCollector) IncrementCounter(name string, tags map[string]string) {}

func (m *NoOpMetricsCollector) IncrementCounterBy(name string, value float64, tags map[string]string) {
}

func (m *NoOpMetricsCollector) SetGauge(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) IncrementGauge(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) RecordDuration(name string, duration time.Duration, tags map[string]string) {
}

func (m *NoOpMetricsCollector) RecordValue(name string, value float64, tags map[string]string) {}

func (m *NoOpMetricsCollector) RecordDistribution(name string, value float64, tags map[string]string) {
}