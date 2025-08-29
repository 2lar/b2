package config

import "time"

// EnvironmentConcurrency is a simplified version for pool manager compatibility
// This matches what the pool_manager.go expects
type EnvironmentConcurrency struct {
	MaxWorkers    int           `yaml:"max_workers" json:"max_workers"`
	BatchSize     int           `yaml:"batch_size" json:"batch_size"`
	QueueSize     int           `yaml:"queue_size" json:"queue_size"`
	TimeoutBuffer time.Duration `yaml:"timeout_buffer" json:"timeout_buffer"`
	EnableMetrics bool          `yaml:"enable_metrics" json:"enable_metrics"`
}

// GetLambdaConcurrency returns Lambda config as EnvironmentConcurrency
func (c *Config) GetLambdaConcurrency() *EnvironmentConcurrency {
	if c.Concurrency.Lambda.MaxWorkers == 0 {
		// Return defaults if not configured
		return &EnvironmentConcurrency{
			MaxWorkers:    4,
			BatchSize:     25,
			QueueSize:     100,
			TimeoutBuffer: c.Concurrency.Lambda.TimeoutBuffer,
			EnableMetrics: true,
		}
	}
	
	return &EnvironmentConcurrency{
		MaxWorkers:    c.Concurrency.Lambda.MaxWorkers,
		BatchSize:     c.Concurrency.Lambda.BatchSize,
		QueueSize:     c.Concurrency.Lambda.QueueSize,
		TimeoutBuffer: c.Concurrency.Lambda.TimeoutBuffer,
		EnableMetrics: true,
	}
}

// GetECSConcurrency returns ECS config as EnvironmentConcurrency
func (c *Config) GetECSConcurrency() *EnvironmentConcurrency {
	if c.Concurrency.ECS.MaxWorkers == 0 {
		// Return defaults if not configured
		return &EnvironmentConcurrency{
			MaxWorkers:    20,
			BatchSize:     100,
			QueueSize:     1000,
			TimeoutBuffer: 30 * time.Second,
			EnableMetrics: true,
		}
	}
	
	return &EnvironmentConcurrency{
		MaxWorkers:    c.Concurrency.ECS.MaxWorkers,
		BatchSize:     c.Concurrency.ECS.BatchSize,
		QueueSize:     c.Concurrency.ECS.QueueSize,
		TimeoutBuffer: 30 * time.Second,
		EnableMetrics: true,
	}
}

// GetLocalConcurrency returns Local config as EnvironmentConcurrency
func (c *Config) GetLocalConcurrency() *EnvironmentConcurrency {
	if c.Concurrency.Local.MaxWorkers == 0 {
		// Return defaults if not configured
		return &EnvironmentConcurrency{
			MaxWorkers:    10,
			BatchSize:     50,
			QueueSize:     500,
			TimeoutBuffer: 60 * time.Second,
			EnableMetrics: false,
		}
	}
	
	return &EnvironmentConcurrency{
		MaxWorkers:    c.Concurrency.Local.MaxWorkers,
		BatchSize:     c.Concurrency.Local.BatchSize,
		QueueSize:     c.Concurrency.Local.QueueSize,
		TimeoutBuffer: 60 * time.Second,
		EnableMetrics: false,
	}
}