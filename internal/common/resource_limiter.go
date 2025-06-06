package common

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ResourceLimiter manages memory and goroutine limits
type ResourceLimiter struct {
	maxMemoryMB      int64
	maxGoroutines    int
	checkInterval    time.Duration
	logger           zerolog.Logger
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	memoryThreshold  int64
	goroutineWarning int
	isRunning        bool
	mu               sync.RWMutex
}

// ResourceLimiterConfig holds configuration for the resource limiter
type ResourceLimiterConfig struct {
	MaxMemoryMB      int64         // Maximum memory in MB before triggering GC
	MaxGoroutines    int           // Maximum number of goroutines
	CheckInterval    time.Duration // How often to check resource usage
	MemoryThreshold  float64       // Percentage of max memory to trigger warning (0.8 = 80%)
	GoroutineWarning float64       // Percentage of max goroutines to trigger warning
}

// DefaultResourceLimiterConfig returns default configuration
func DefaultResourceLimiterConfig() ResourceLimiterConfig {
	return ResourceLimiterConfig{
		MaxMemoryMB:      2048,  // 2GB
		MaxGoroutines:    10000, // 10k goroutines
		CheckInterval:    30 * time.Second,
		MemoryThreshold:  0.8, // 80%
		GoroutineWarning: 0.7, // 70%
	}
}

// NewResourceLimiter creates a new resource limiter
func NewResourceLimiter(config ResourceLimiterConfig, logger zerolog.Logger) *ResourceLimiter {
	ctx, cancel := context.WithCancel(context.Background())

	rl := &ResourceLimiter{
		maxMemoryMB:      config.MaxMemoryMB,
		maxGoroutines:    config.MaxGoroutines,
		checkInterval:    config.CheckInterval,
		logger:           logger.With().Str("component", "ResourceLimiter").Logger(),
		ctx:              ctx,
		cancel:           cancel,
		memoryThreshold:  int64(float64(config.MaxMemoryMB) * config.MemoryThreshold),
		goroutineWarning: int(float64(config.MaxGoroutines) * config.GoroutineWarning),
	}

	return rl
}

// Start begins monitoring resource usage
func (rl *ResourceLimiter) Start() {
	rl.mu.Lock()
	if rl.isRunning {
		rl.mu.Unlock()
		return
	}
	rl.isRunning = true
	rl.mu.Unlock()

	rl.wg.Add(1)
	go rl.monitorResources()

	rl.logger.Info().
		Int64("max_memory_mb", rl.maxMemoryMB).
		Int("max_goroutines", rl.maxGoroutines).
		Dur("check_interval", rl.checkInterval).
		Msg("Resource limiter started")
}

// Stop stops the resource monitor
func (rl *ResourceLimiter) Stop() {
	rl.mu.Lock()
	if !rl.isRunning {
		rl.mu.Unlock()
		return
	}
	rl.isRunning = false
	rl.mu.Unlock()

	rl.cancel()
	rl.wg.Wait()
	rl.logger.Info().Msg("Resource limiter stopped")
}

// CheckMemoryLimit checks if current memory usage exceeds limit
func (rl *ResourceLimiter) CheckMemoryLimit() error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	currentMB := int64(m.Alloc / 1024 / 1024)

	if currentMB > rl.maxMemoryMB {
		return fmt.Errorf("memory limit exceeded: current %dMB > limit %dMB", currentMB, rl.maxMemoryMB)
	}

	return nil
}

// CheckGoroutineLimit checks if current goroutine count exceeds limit
func (rl *ResourceLimiter) CheckGoroutineLimit() error {
	current := runtime.NumGoroutine()

	if current > rl.maxGoroutines {
		return fmt.Errorf("goroutine limit exceeded: current %d > limit %d", current, rl.maxGoroutines)
	}

	return nil
}

// ForceGC forces garbage collection if memory usage is high
func (rl *ResourceLimiter) ForceGC() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	beforeMB := int64(m.Alloc / 1024 / 1024)

	runtime.GC()
	runtime.ReadMemStats(&m)

	afterMB := int64(m.Alloc / 1024 / 1024)

	rl.logger.Info().
		Int64("before_mb", beforeMB).
		Int64("after_mb", afterMB).
		Int64("freed_mb", beforeMB-afterMB).
		Msg("Forced garbage collection")
}

// GetResourceUsage returns current resource usage
func (rl *ResourceLimiter) GetResourceUsage() ResourceUsage {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return ResourceUsage{
		AllocMB:    int64(m.Alloc / 1024 / 1024),
		SysMB:      int64(m.Sys / 1024 / 1024),
		Goroutines: runtime.NumGoroutine(),
		GCCount:    int64(m.NumGC),
		NextGCMB:   int64(m.NextGC / 1024 / 1024),
	}
}

// ResourceUsage holds current resource usage stats
type ResourceUsage struct {
	AllocMB    int64 // Currently allocated memory
	SysMB      int64 // System memory
	Goroutines int   // Number of goroutines
	GCCount    int64 // Number of GC cycles
	NextGCMB   int64 // Next GC target
}

// Private methods

func (rl *ResourceLimiter) monitorResources() {
	defer rl.wg.Done()

	ticker := time.NewTicker(rl.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.ctx.Done():
			return
		case <-ticker.C:
			rl.checkAndLogResourceUsage()
		}
	}
}

func (rl *ResourceLimiter) checkAndLogResourceUsage() {
	usage := rl.GetResourceUsage()

	// Check memory usage
	if usage.AllocMB > rl.memoryThreshold {
		rl.logger.Warn().
			Int64("current_mb", usage.AllocMB).
			Int64("threshold_mb", rl.memoryThreshold).
			Int64("limit_mb", rl.maxMemoryMB).
			Msg("Memory usage above threshold, forcing GC")
		rl.ForceGC()
	}

	// Check goroutine count
	if usage.Goroutines > rl.goroutineWarning {
		rl.logger.Warn().
			Int("current", usage.Goroutines).
			Int("warning_threshold", rl.goroutineWarning).
			Int("limit", rl.maxGoroutines).
			Msg("High number of goroutines detected")
	}

	// Log periodic stats
	rl.logger.Debug().
		Int64("alloc_mb", usage.AllocMB).
		Int64("sys_mb", usage.SysMB).
		Int("goroutines", usage.Goroutines).
		Int64("gc_count", usage.GCCount).
		Msg("Resource usage stats")
}

// Global resource limiter instance
var globalResourceLimiter *ResourceLimiter
var globalLimiterOnce sync.Once

// GetGlobalResourceLimiter returns the global resource limiter instance
func GetGlobalResourceLimiter(logger zerolog.Logger) *ResourceLimiter {
	globalLimiterOnce.Do(func() {
		config := DefaultResourceLimiterConfig()
		globalResourceLimiter = NewResourceLimiter(config, logger)
	})
	return globalResourceLimiter
}

// StartGlobalResourceLimiter starts the global resource limiter
func StartGlobalResourceLimiter(logger zerolog.Logger) {
	limiter := GetGlobalResourceLimiter(logger)
	limiter.Start()
}

// StopGlobalResourceLimiter stops the global resource limiter
func StopGlobalResourceLimiter() {
	if globalResourceLimiter != nil {
		globalResourceLimiter.Stop()
	}
}
