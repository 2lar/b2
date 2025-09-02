// Package metrics provides a simple metrics adapter implementation
package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"brain2-backend/internal/core/application/ports"
)

// SimpleMetrics provides a basic in-memory metrics implementation
// In production, this could be replaced with Prometheus, CloudWatch, etc.
type SimpleMetrics struct {
	counters   map[string]*int64
	gauges     map[string]*float64
	histograms map[string][]float64
	mu         sync.RWMutex
	logger     ports.Logger
}

// NewSimpleMetrics creates a new simple metrics adapter
func NewSimpleMetrics(logger ports.Logger) *SimpleMetrics {
	return &SimpleMetrics{
		counters:   make(map[string]*int64),
		gauges:     make(map[string]*float64),
		histograms: make(map[string][]float64),
		logger:     logger,
	}
}

// IncrementCounter increments a counter metric
func (m *SimpleMetrics) IncrementCounter(name string, tags ...ports.Tag) {
	key := m.buildKey(name, tags)
	
	m.mu.Lock()
	counter, exists := m.counters[key]
	if !exists {
		var c int64
		m.counters[key] = &c
		counter = &c
	}
	m.mu.Unlock()
	
	atomic.AddInt64(counter, 1)
	
	if m.logger != nil {
		m.logger.Debug("Counter incremented",
			ports.Field{Key: "metric", Value: key},
			ports.Field{Key: "value", Value: atomic.LoadInt64(counter)})
	}
}

// RecordGauge records a gauge value
func (m *SimpleMetrics) RecordGauge(name string, value float64, tags ...ports.Tag) {
	key := m.buildKey(name, tags)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.gauges[key]; !exists {
		m.gauges[key] = new(float64)
	}
	*m.gauges[key] = value
	
	if m.logger != nil {
		m.logger.Debug("Gauge recorded",
			ports.Field{Key: "metric", Value: key},
			ports.Field{Key: "value", Value: value})
	}
}

// RecordHistogram records a histogram value
func (m *SimpleMetrics) RecordHistogram(name string, value float64, tags ...ports.Tag) {
	key := m.buildKey(name, tags)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.histograms[key]; !exists {
		m.histograms[key] = make([]float64, 0, 100)
	}
	m.histograms[key] = append(m.histograms[key], value)
	
	// Keep only last 1000 values to prevent unbounded growth
	if len(m.histograms[key]) > 1000 {
		m.histograms[key] = m.histograms[key][1:]
	}
	
	if m.logger != nil {
		m.logger.Debug("Histogram value recorded",
			ports.Field{Key: "metric", Value: key},
			ports.Field{Key: "value", Value: value})
	}
}

// RecordDuration records a duration as a histogram
func (m *SimpleMetrics) RecordDuration(name string, duration time.Duration, tags ...ports.Tag) {
	m.RecordHistogram(name, float64(duration.Milliseconds()), tags...)
}

// StartTimer starts a timing operation
func (m *SimpleMetrics) StartTimer(name string, tags ...ports.Tag) ports.Timer {
	return &simpleTimer{
		metrics:   m,
		name:      name,
		tags:      tags,
		startTime: time.Now(),
	}
}

// GetCounter returns the current value of a counter
func (m *SimpleMetrics) GetCounter(name string, tags ...ports.Tag) int64 {
	key := m.buildKey(name, tags)
	
	m.mu.RLock()
	counter, exists := m.counters[key]
	m.mu.RUnlock()
	
	if !exists {
		return 0
	}
	return atomic.LoadInt64(counter)
}

// GetGauge returns the current value of a gauge
func (m *SimpleMetrics) GetGauge(name string, tags ...ports.Tag) float64 {
	key := m.buildKey(name, tags)
	
	m.mu.RLock()
	gauge, exists := m.gauges[key]
	m.mu.RUnlock()
	
	if !exists {
		return 0
	}
	return *gauge
}

// GetHistogramStats returns basic statistics for a histogram
func (m *SimpleMetrics) GetHistogramStats(name string, tags ...ports.Tag) map[string]float64 {
	key := m.buildKey(name, tags)
	
	m.mu.RLock()
	values, exists := m.histograms[key]
	m.mu.RUnlock()
	
	if !exists || len(values) == 0 {
		return map[string]float64{
			"count": 0,
			"min":   0,
			"max":   0,
			"avg":   0,
		}
	}
	
	// Calculate basic stats
	var sum, min, max float64
	min = values[0]
	max = values[0]
	
	for _, v := range values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	
	return map[string]float64{
		"count": float64(len(values)),
		"min":   min,
		"max":   max,
		"avg":   sum / float64(len(values)),
	}
}

// Reset clears all metrics (useful for testing)
func (m *SimpleMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.counters = make(map[string]*int64)
	m.gauges = make(map[string]*float64)
	m.histograms = make(map[string][]float64)
}

// buildKey creates a metric key from name and tags
func (m *SimpleMetrics) buildKey(name string, tags []ports.Tag) string {
	if len(tags) == 0 {
		return name
	}
	
	key := name
	for _, tag := range tags {
		key = fmt.Sprintf("%s.%s_%v", key, tag.Key, tag.Value)
	}
	return key
}

// simpleTimer implements ports.Timer
type simpleTimer struct {
	metrics   *SimpleMetrics
	name      string
	tags      []ports.Tag
	startTime time.Time
}

// Stop stops the timer and records the duration
func (t *simpleTimer) Stop() {
	duration := time.Since(t.startTime)
	t.metrics.RecordDuration(t.name, duration, t.tags...)
}