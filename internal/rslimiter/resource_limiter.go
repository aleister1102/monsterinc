package rslimiter

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// ResourceLimiter manages memory, CPU, and goroutine limits
type ResourceLimiter struct {
	config           ResourceLimiterConfig
	logger           zerolog.Logger
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	memoryThreshold  int64
	goroutineWarning int
	isRunning        bool
	mu               sync.RWMutex
	shutdownCallback func() // Callback function to trigger graceful shutdown
}

// NewResourceLimiter creates a new resource limiter
func NewResourceLimiter(config ResourceLimiterConfig, logger zerolog.Logger) *ResourceLimiter {
	ctx, cancel := context.WithCancel(context.Background())

	// Apply default values for any zero-value fields in the config
	if config.CheckInterval == 0 {
		config.CheckInterval = 30 * time.Second
	}
	if config.MemoryThreshold == 0 {
		config.MemoryThreshold = 0.8
	}
	if config.GoroutineWarning == 0 {
		config.GoroutineWarning = 0.8
	}
	if config.SystemMemThreshold == 0 {
		config.SystemMemThreshold = 0.9
	}
	if config.CPUThreshold == 0 {
		config.CPUThreshold = 0.9
	}

	rl := &ResourceLimiter{
		config:           config,
		logger:           logger.With().Str("component", "ResourceLimiter").Logger(),
		ctx:              ctx,
		cancel:           cancel,
		memoryThreshold:  int64(float64(config.MaxMemoryMB) * config.MemoryThreshold),
		goroutineWarning: int(float64(config.MaxGoroutines) * config.GoroutineWarning),
	}

	return rl
}

// SetShutdownCallback sets the callback function for graceful shutdown
func (rl *ResourceLimiter) SetShutdownCallback(callback func()) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.shutdownCallback = callback
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
		Int64("max_memory_mb", rl.config.MaxMemoryMB).
		Int("max_goroutines", rl.config.MaxGoroutines).
		Dur("check_interval", rl.config.CheckInterval).
		Float64("system_mem_threshold", rl.config.SystemMemThreshold).
		Float64("cpu_threshold", rl.config.CPUThreshold).
		Bool("auto_shutdown_enabled", rl.config.EnableAutoShutdown).
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

	if currentMB > rl.config.MaxMemoryMB {
		return fmt.Errorf("memory limit exceeded: current %dMB > limit %dMB", currentMB, rl.config.MaxMemoryMB)
	}

	return nil
}

// CheckSystemMemoryLimit checks if system memory usage exceeds threshold
func (rl *ResourceLimiter) CheckSystemMemoryLimit() (bool, error) {
	if !rl.config.EnableAutoShutdown {
		return false, nil
	}

	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return false, fmt.Errorf("failed to get system memory stats: %w", err)
	}

	usedPercent := vmStat.UsedPercent / 100.0 // Convert percentage to decimal

	if usedPercent > rl.config.SystemMemThreshold {
		rl.logger.Warn().
			Float64("used_percent", usedPercent*100).
			Float64("threshold_percent", rl.config.SystemMemThreshold*100).
			Uint64("used_mb", vmStat.Used/1024/1024).
			Uint64("total_mb", vmStat.Total/1024/1024).
			Msg("System memory usage exceeded threshold")
		return true, nil
	}

	return false, nil
}

// CheckCPULimit checks if CPU usage exceeds threshold
func (rl *ResourceLimiter) CheckCPULimit() (bool, error) {
	if !rl.config.EnableAutoShutdown {
		return false, nil
	}

	cpuPercents, err := cpu.Percent(time.Second, false)
	if err != nil {
		return false, fmt.Errorf("failed to get CPU usage: %w", err)
	}

	if len(cpuPercents) == 0 {
		return false, fmt.Errorf("no CPU usage data available")
	}

	cpuUsage := cpuPercents[0] / 100.0 // Convert percentage to decimal

	if cpuUsage > rl.config.CPUThreshold {
		rl.logger.Warn().
			Float64("cpu_usage_percent", cpuUsage*100).
			Float64("threshold_percent", rl.config.CPUThreshold*100).
			Msg("CPU usage exceeded threshold")
		return true, nil
	}

	return false, nil
}

// CheckGoroutineLimit checks if current goroutine count exceeds limit
func (rl *ResourceLimiter) CheckGoroutineLimit() error {
	current := runtime.NumGoroutine()

	if current > rl.config.MaxGoroutines {
		return fmt.Errorf("goroutine limit exceeded: current %d > limit %d", current, rl.config.MaxGoroutines)
	}

	return nil
}

// ForceGC forces garbage collection and logs the results
func (rl *ResourceLimiter) ForceGC() {
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	before := m1.Alloc / 1024 / 1024

	runtime.GC()
	runtime.GC() // Call twice to ensure complete cleanup

	runtime.ReadMemStats(&m2)
	after := m2.Alloc / 1024 / 1024

	rl.logger.Info().
		Uint64("before_mb", before).
		Uint64("after_mb", after).
		Int64("freed_mb", int64(before-after)).
		Msg("Forced garbage collection completed")
}

// monitorResources runs the resource monitoring loop
func (rl *ResourceLimiter) monitorResources() {
	defer rl.wg.Done()

	ticker := time.NewTicker(rl.config.CheckInterval)
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

// checkAndLogResourceUsage checks current resource usage and logs warnings/errors
func (rl *ResourceLimiter) checkAndLogResourceUsage() {
	usage := GetResourceUsage()

	rl.logWarnings(usage)

	if rl.config.EnableAutoShutdown {
		if exceeded, reason := rl.checkShutdownConditions(); exceeded {
			rl.logger.Error().
				Str("reason", reason).
				Int64("alloc_mb", usage.AllocMB).
				Int("goroutines", usage.Goroutines).
				Float64("system_mem_percent", usage.SystemMemUsedPercent).
				Float64("cpu_percent", usage.CPUUsagePercent).
				Msg("Resource limits exceeded, triggering graceful shutdown")

			rl.triggerGracefulShutdown()
			return
		}
	}

	// Log periodic resource usage at debug level
	rl.logger.Debug().
		Int64("alloc_mb", usage.AllocMB).
		Int64("sys_mb", usage.SysMB).
		Int("goroutines", usage.Goroutines).
		Int64("gc_count", usage.GCCount).
		Float64("system_mem_percent", usage.SystemMemUsedPercent).
		Float64("cpu_percent", usage.CPUUsagePercent).
		Msg("Current resource usage")
}

func (rl *ResourceLimiter) logWarnings(usage ResourceUsage) {
	// Check memory threshold warning
	if usage.AllocMB > rl.memoryThreshold {
		rl.logger.Warn().
			Int64("current_mb", usage.AllocMB).
			Int64("threshold_mb", rl.memoryThreshold).
			Int64("limit_mb", rl.config.MaxMemoryMB).
			Msg("Memory usage approaching limit")
	}

	// Check goroutine warning
	if usage.Goroutines > rl.goroutineWarning {
		rl.logger.Warn().
			Int("current", usage.Goroutines).
			Int("warning_threshold", rl.goroutineWarning).
			Int("limit", rl.config.MaxGoroutines).
			Msg("Goroutine count approaching limit")
	}
}

// checkShutdownConditions checks all shutdown conditions and returns if a shutdown is needed
func (rl *ResourceLimiter) checkShutdownConditions() (bool, string) {
	type checkFunc func() (bool, string)

	checks := []checkFunc{
		rl.systemMemoryChecker,
		rl.cpuChecker,
		rl.appMemoryChecker,
		rl.goroutineChecker,
	}

	for _, check := range checks {
		if exceeded, reason := check(); exceeded {
			return true, reason
		}
	}

	return false, ""
}

func (rl *ResourceLimiter) systemMemoryChecker() (bool, string) {
	exceeded, err := rl.CheckSystemMemoryLimit()
	if err != nil {
		rl.logger.Error().Err(err).Msg("Failed to check system memory limit")
		return false, ""
	}
	if exceeded {
		return true, "System memory threshold exceeded"
	}
	return false, ""
}

func (rl *ResourceLimiter) cpuChecker() (bool, string) {
	exceeded, err := rl.CheckCPULimit()
	if err != nil {
		rl.logger.Error().Err(err).Msg("Failed to check CPU limit")
		return false, ""
	}
	if exceeded {
		return true, "CPU usage threshold exceeded"
	}
	return false, ""
}

func (rl *ResourceLimiter) appMemoryChecker() (bool, string) {
	if err := rl.CheckMemoryLimit(); err != nil {
		return true, fmt.Sprintf("Application memory limit exceeded: %v", err)
	}
	return false, ""
}

func (rl *ResourceLimiter) goroutineChecker() (bool, string) {
	if err := rl.CheckGoroutineLimit(); err != nil {
		return true, fmt.Sprintf("Goroutine limit exceeded: %v", err)
	}
	return false, ""
}

// triggerGracefulShutdown calls the shutdown callback if set
func (rl *ResourceLimiter) triggerGracefulShutdown() {
	rl.mu.RLock()
	callback := rl.shutdownCallback
	rl.mu.RUnlock()

	if callback != nil {
		rl.logger.Info().Msg("Calling shutdown callback due to resource limits")
		callback()
	} else {
		rl.logger.Warn().Msg("No shutdown callback set, cannot trigger graceful shutdown")
	}
}
