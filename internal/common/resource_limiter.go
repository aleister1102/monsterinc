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

	// Get system memory stats
	vmStat, err := mem.VirtualMemory()
	var systemMemUsedMB, systemMemTotalMB uint64
	var systemMemUsedPercent float64
	if err == nil {
		systemMemUsedMB = vmStat.Used / 1024 / 1024
		systemMemTotalMB = vmStat.Total / 1024 / 1024
		systemMemUsedPercent = vmStat.UsedPercent
	}

	// Get CPU usage stats
	var cpuUsagePercent float64
	cpuPercents, err := cpu.Percent(100*time.Millisecond, false)
	if err == nil && len(cpuPercents) > 0 {
		cpuUsagePercent = cpuPercents[0]
	}

	return ResourceUsage{
		AllocMB:              int64(m.Alloc / 1024 / 1024),
		SysMB:                int64(m.Sys / 1024 / 1024),
		Goroutines:           runtime.NumGoroutine(),
		GCCount:              int64(m.NumGC),
		NextGCMB:             int64(m.NextGC / 1024 / 1024),
		SystemMemUsedMB:      int64(systemMemUsedMB),
		SystemMemTotalMB:     int64(systemMemTotalMB),
		SystemMemUsedPercent: systemMemUsedPercent,
		CPUUsagePercent:      cpuUsagePercent,
	}
}

// ResourceUsage holds current resource usage stats
type ResourceUsage struct {
	AllocMB              int64   // Currently allocated memory by application
	SysMB                int64   // System memory used by Go runtime
	Goroutines           int     // Number of goroutines
	GCCount              int64   // Number of GC cycles
	NextGCMB             int64   // Next GC target
	SystemMemUsedMB      int64   // System memory used (MB)
	SystemMemTotalMB     int64   // Total system memory (MB)
	SystemMemUsedPercent float64 // System memory used percentage
	CPUUsagePercent      float64 // CPU usage percentage
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

	// Check system memory usage first - this is the critical check
	exceeded, err := rl.CheckSystemMemoryLimit()
	if err != nil {
		rl.logger.Error().Err(err).Msg("Failed to check system memory limit")
	} else if exceeded {
		rl.logger.Error().
			Float64("system_mem_used_percent", usage.SystemMemUsedPercent).
			Float64("threshold_percent", rl.systemMemThreshold*100).
			Int64("system_mem_used_mb", usage.SystemMemUsedMB).
			Int64("system_mem_total_mb", usage.SystemMemTotalMB).
			Msg("CRITICAL: System memory usage exceeded threshold, triggering shutdown")

		// Trigger graceful shutdown
		rl.triggerGracefulShutdown()
		return
	}

	// Check CPU usage - this is also a critical check
	cpuExceeded, err := rl.CheckCPULimit()
	if err != nil {
		rl.logger.Error().Err(err).Msg("Failed to check CPU limit")
	} else if cpuExceeded {
		rl.logger.Error().
			Float64("cpu_usage_percent", usage.CPUUsagePercent).
			Float64("threshold_percent", rl.cpuThreshold*100).
			Msg("CRITICAL: CPU usage exceeded threshold, triggering shutdown")

		// Trigger graceful shutdown
		rl.triggerGracefulShutdown()
		return
	}

	// Check application memory usage
	if usage.AllocMB > rl.memoryThreshold {
		rl.logger.Warn().
			Int64("current_mb", usage.AllocMB).
			Int64("threshold_mb", rl.memoryThreshold).
			Int64("limit_mb", rl.maxMemoryMB).
			Msg("Application memory usage above threshold, forcing GC")
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

	// Log periodic stats including system memory and CPU
	rl.logger.Debug().
		Int64("app_alloc_mb", usage.AllocMB).
		Int64("app_sys_mb", usage.SysMB).
		Int("goroutines", usage.Goroutines).
		Int64("gc_count", usage.GCCount).
		Int64("system_mem_used_mb", usage.SystemMemUsedMB).
		Int64("system_mem_total_mb", usage.SystemMemTotalMB).
		Float64("system_mem_used_percent", usage.SystemMemUsedPercent).
		Float64("cpu_usage_percent", usage.CPUUsagePercent).
		Msg("Resource usage stats")
}

func (rl *ResourceLimiter) triggerGracefulShutdown() {
	rl.mu.RLock()
	callback := rl.shutdownCallback
	rl.mu.RUnlock()

	if callback != nil {
		rl.logger.Info().Msg("Triggering graceful shutdown due to memory limit")
		go callback() // Run in goroutine to avoid blocking the monitor
	} else {
		rl.logger.Error().Msg("No shutdown callback registered, cannot trigger graceful shutdown")
	}
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

// SetGlobalShutdownCallback sets the shutdown callback for the global resource limiter
func SetGlobalShutdownCallback(callback func()) {
	if globalResourceLimiter != nil {
		globalResourceLimiter.SetShutdownCallback(callback)
	}
}
