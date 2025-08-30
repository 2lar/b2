package metrics

import (
	"fmt"
	"sync"
	"time"

	"brain2-backend/internal/infrastructure/observability"
	"go.uber.org/zap"
)

// InMemoryMetricsCollector collects metrics in memory
type InMemoryMetricsCollector struct {
	counters map[string]float64
	gauges   map[string]float64
	timings  map[string][]time.Duration
	mu       sync.RWMutex
}

// NewInMemoryMetricsCollector creates a new in-memory metrics collector
func NewInMemoryMetricsCollector(logger *zap.Logger) *observability.Collector {
	// For now, return a real observability.Collector instance
	return observability.NewCollector("brain2")
}

func (m *InMemoryMetricsCollector) IncrementCounter(name string, tags map[string]string) {
	m.IncrementCounterBy(name, 1, tags)
}

func (m *InMemoryMetricsCollector) IncrementCounterBy(name string, value float64, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.buildKey(name, tags)
	m.counters[key] += value
}

func (m *InMemoryMetricsCollector) SetGauge(name string, value float64, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.buildKey(name, tags)
	m.gauges[key] = value
}

func (m *InMemoryMetricsCollector) IncrementGauge(name string, value float64, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.buildKey(name, tags)
	m.gauges[key] += value
}

func (m *InMemoryMetricsCollector) RecordDuration(name string, duration time.Duration, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.buildKey(name, tags)
	m.timings[key] = append(m.timings[key], duration)

	// Keep only last 1000 timings per metric
	if len(m.timings[key]) > 1000 {
		m.timings[key] = m.timings[key][len(m.timings[key])-1000:]
	}
}

func (m *InMemoryMetricsCollector) RecordValue(name string, value float64, tags map[string]string) {
	// For now, treat as gauge
	m.SetGauge(name, value, tags)
}

func (m *InMemoryMetricsCollector) RecordDistribution(name string, value float64, tags map[string]string) {
	// For now, treat as gauge
	m.SetGauge(name, value, tags)
}

func (m *InMemoryMetricsCollector) buildKey(name string, tags map[string]string) string {
	if len(tags) == 0 {
		return name
	}

	// Build key with tags
	key := name
	for k, v := range tags {
		key += fmt.Sprintf(",%s=%s", k, v)
	}
	return key
}