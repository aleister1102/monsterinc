package rslimiter

import "time"

// ResourceLimiterConfig holds configuration for the resource limiter
type ResourceLimiterConfig struct {
	MaxMemoryMB        int64         // Maximum memory in MB before triggering GC
	MaxGoroutines      int           // Maximum number of goroutines
	CheckInterval      time.Duration // How often to check resource usage
	MemoryThreshold    float64       // Percentage of max memory to trigger warning (0.8 = 80%)
	GoroutineWarning   float64       // Percentage of max goroutines to trigger warning
	SystemMemThreshold float64       // Percentage of system memory to trigger auto-shutdown (0.5 = 50%)
	CPUThreshold       float64       // Percentage of CPU usage to trigger actions (0.5 = 50%)
	EnableAutoShutdown bool          // Enable auto-shutdown when resource thresholds are exceeded
}

// DefaultResourceLimiterConfig returns default configuration
func DefaultResourceLimiterConfig() ResourceLimiterConfig {
	return ResourceLimiterConfig{
		MaxMemoryMB:        1024,  // 1GB
		MaxGoroutines:      10000, // 10k goroutines
		CheckInterval:      30 * time.Second,
		MemoryThreshold:    0.8,  // 80%
		GoroutineWarning:   0.7,  // 70%
		SystemMemThreshold: 0.5,  // 50% system memory
		CPUThreshold:       0.5,  // 50% CPU usage
		EnableAutoShutdown: true, // Enable auto-shutdown by default
	}
}
