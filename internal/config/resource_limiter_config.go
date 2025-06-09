package config

// ResourceLimiterConfig holds configuration for resource monitoring
type ResourceLimiterConfig struct {
	MaxMemoryMB        int64   `json:"max_memory_mb,omitempty" yaml:"max_memory_mb,omitempty" validate:"omitempty,min=100"`
	MaxGoroutines      int     `json:"max_goroutines,omitempty" yaml:"max_goroutines,omitempty" validate:"omitempty,min=100"`
	CheckIntervalSecs  int     `json:"check_interval_secs,omitempty" yaml:"check_interval_secs,omitempty" validate:"omitempty,min=1"`
	MemoryThreshold    float64 `json:"memory_threshold,omitempty" yaml:"memory_threshold,omitempty" validate:"omitempty,min=0.1,max=1.0"`
	GoroutineWarning   float64 `json:"goroutine_warning,omitempty" yaml:"goroutine_warning,omitempty" validate:"omitempty,min=0.1,max=1.0"`
	SystemMemThreshold float64 `json:"system_mem_threshold,omitempty" yaml:"system_mem_threshold,omitempty" validate:"omitempty,min=0.1,max=1.0"`
	CPUThreshold       float64 `json:"cpu_threshold,omitempty" yaml:"cpu_threshold,omitempty" validate:"omitempty,min=0.1,max=1.0"`
	EnableAutoShutdown bool    `json:"enable_auto_shutdown" yaml:"enable_auto_shutdown"`
}

// NewDefaultResourceLimiterConfig creates default resource limiter configuration
func NewDefaultResourceLimiterConfig() ResourceLimiterConfig {
	return ResourceLimiterConfig{
		MaxMemoryMB:        512,  // Giảm từ 1024 xuống 512MB để trigger sớm hơn
		MaxGoroutines:      5000, // Giảm từ 10000 xuống 5000
		CheckIntervalSecs:  15,   // Giảm từ 30 xuống 15 seconds để check thường xuyên hơn
		MemoryThreshold:    0.7,  // Giảm từ 0.8 xuống 0.7 (70%)
		GoroutineWarning:   0.6,  // Giảm từ 0.7 xuống 0.6 (60%)
		SystemMemThreshold: 0.4,  // Giảm từ 0.5 xuống 0.4 (40% system memory)
		CPUThreshold:       0.4,  // Giảm từ 0.5 xuống 0.4 (40% CPU usage)
		EnableAutoShutdown: true, // Enable auto-shutdown by default
	}
}
