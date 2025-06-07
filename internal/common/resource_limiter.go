package common

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
	// System resource monitoring
	systemMemThreshold float64 // Percentage of system memory to trigger shutdown (0.5 = 50%)
	cpuThreshold       float64 // Percentage of CPU usage to trigger actions (0.5 = 50%)
	enableAutoShutdown bool    // Whether to enable auto-shutdown on resource limits
	shutdownCallback   func()  // Callback function to trigger graceful shutdown
}

// NewResourceLimiter creates a new resource limiter
func NewResourceLimiter(config ResourceLimiterConfig, logger zerolog.Logger) *ResourceLimiter {
	ctx, cancel := context.WithCancel(context.Background())

	rl := &ResourceLimiter{
		maxMemoryMB:        config.MaxMemoryMB,
		maxGoroutines:      config.MaxGoroutines,
		checkInterval:      config.CheckInterval,
		logger:             logger.With().Str("component", "ResourceLimiter").Logger(),
		ctx:                ctx,
		cancel:             cancel,
		memoryThreshold:    int64(float64(config.MaxMemoryMB) * config.MemoryThreshold),
		goroutineWarning:   int(float64(config.MaxGoroutines) * config.GoroutineWarning),
		systemMemThreshold: config.SystemMemThreshold,
		cpuThreshold:       config.CPUThreshold,
		enableAutoShutdown: config.EnableAutoShutdown,
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
		Int64("max_memory_mb", rl.maxMemoryMB).
		Int("max_goroutines", rl.maxGoroutines).
		Dur("check_interval", rl.checkInterval).
		Float64("system_mem_threshold", rl.systemMemThreshold).
		Float64("cpu_threshold", rl.cpuThreshold).
		Bool("auto_shutdown_enabled", rl.enableAutoShutdown).
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

// CheckSystemMemoryLimit checks if system memory usage exceeds threshold
func (rl *ResourceLimiter) CheckSystemMemoryLimit() (bool, error) {
	if !rl.enableAutoShutdown {
		return false, nil
	}

	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return false, fmt.Errorf("failed to get system memory stats: %w", err)
	}

	usedPercent := vmStat.UsedPercent / 100.0 // Convert percentage to decimal

	if usedPercent > rl.systemMemThreshold {
		rl.logger.Warn().
			Float64("used_percent", usedPercent*100).
			Float64("threshold_percent", rl.systemMemThreshold*100).
			Uint64("used_mb", vmStat.Used/1024/1024).
			Uint64("total_mb", vmStat.Total/1024/1024).
			Msg("System memory usage exceeded threshold")
		return true, nil
	}

	return false, nil
}

// CheckCPULimit checks if CPU usage exceeds threshold
func (rl *ResourceLimiter) CheckCPULimit() (bool, error) {
	if !rl.enableAutoShutdown {
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

	if cpuUsage > rl.cpuThreshold {
		rl.logger.Warn().
			Float64("cpu_usage_percent", cpuUsage*100).
			Float64("threshold_percent", rl.cpuThreshold*100).
			Msg("CPU usage exceeded threshold")
		return true, nil
	}

	return false, nil
}

// CheckGoroutineLimit checks if current goroutine count exceeds limit
func (rl *ResourceLimiter) CheckGoroutineLimit() error {
	current := runtime.NumGoroutine()

	if current > rl.maxGoroutines {
		return fmt.Errorf("goroutine limit exceeded: current %d > limit %d", current, rl.maxGoroutines)
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

// GetResourceUsage returns current resource usage statistics
func (rl *ResourceLimiter) GetResourceUsage() ResourceUsage {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	usage := ResourceUsage{
		AllocMB:    int64(m.Alloc / 1024 / 1024),
		SysMB:      int64(m.Sys / 1024 / 1024),
		Goroutines: runtime.NumGoroutine(),
		GCCount:    int64(m.NumGC),
		NextGCMB:   int64(m.NextGC / 1024 / 1024),
	}

	// Get system memory stats
	if vmStat, err := mem.VirtualMemory(); err == nil {
		usage.SystemMemUsedMB = int64(vmStat.Used / 1024 / 1024)
		usage.SystemMemTotalMB = int64(vmStat.Total / 1024 / 1024)
		usage.SystemMemUsedPercent = vmStat.UsedPercent
	}

	// Get CPU usage
	if cpuPercents, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(cpuPercents) > 0 {
		usage.CPUUsagePercent = cpuPercents[0]
	}

	return usage
}

// monitorResources runs the resource monitoring loop
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

// checkAndLogResourceUsage checks current resource usage and logs warnings/errors
func (rl *ResourceLimiter) checkAndLogResourceUsage() {
	usage := rl.GetResourceUsage()

	// Check memory threshold warning
	if usage.AllocMB > rl.memoryThreshold {
		rl.logger.Warn().
			Int64("current_mb", usage.AllocMB).
			Int64("threshold_mb", rl.memoryThreshold).
			Int64("limit_mb", rl.maxMemoryMB).
			Msg("Memory usage approaching limit")
	}

	// Check goroutine warning
	if usage.Goroutines > rl.goroutineWarning {
		rl.logger.Warn().
			Int("current", usage.Goroutines).
			Int("warning_threshold", rl.goroutineWarning).
			Int("limit", rl.maxGoroutines).
			Msg("Goroutine count approaching limit")
	}

	// Check for resource limit violations that trigger shutdown
	if rl.enableAutoShutdown {
		shouldShutdown := false
		var shutdownReason string

		// Check system memory
		if exceeded, err := rl.CheckSystemMemoryLimit(); err == nil && exceeded {
			shouldShutdown = true
			shutdownReason = "System memory threshold exceeded"
		}

		// Check CPU usage
		if !shouldShutdown {
			if exceeded, err := rl.CheckCPULimit(); err == nil && exceeded {
				shouldShutdown = true
				shutdownReason = "CPU usage threshold exceeded"
			}
		}

		// Check application memory limit
		if !shouldShutdown {
			if err := rl.CheckMemoryLimit(); err != nil {
				shouldShutdown = true
				shutdownReason = fmt.Sprintf("Application memory limit exceeded: %v", err)
			}
		}

		// Check goroutine limit
		if !shouldShutdown {
			if err := rl.CheckGoroutineLimit(); err != nil {
				shouldShutdown = true
				shutdownReason = fmt.Sprintf("Goroutine limit exceeded: %v", err)
			}
		}

		if shouldShutdown {
			rl.logger.Error().
				Str("reason", shutdownReason).
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
